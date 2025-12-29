package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const openRouterURL = "https://openrouter.ai/api/v1/chat/completions"

// Recommended models for structured output (as of December 2025):
//
// Cost-Effective:
//   - openai/gpt-4o-mini           - Good balance of cost/quality, native structured output
//   - google/gemini-2.0-flash      - Fast and cheap, good for bulk operations
//
// High Quality:
//   - anthropic/claude-sonnet-4.5  - Best for coding/agentic tasks, structured via tool use
//   - anthropic/claude-sonnet-4    - Excellent balance of speed and quality
//   - openai/gpt-5.1               - Latest GPT with native structured output
//   - google/gemini-3-pro-preview  - Google's latest flagship
//
// Maximum Quality:
//   - anthropic/claude-opus-4.5    - Most capable, best reasoning
//
// Web Search:
//   Append ":online" to any model to enable real-time web search (e.g., "anthropic/claude-sonnet-4:online")
//   This adds current package data, new apps, and 2025 Linux ecosystem information.
//   Native search for Anthropic/OpenAI/Perplexity/xAI; Exa-powered for others ($0.02/request).
//
// See: https://openrouter.ai/models and https://openrouter.ai/docs/features/web-search

const defaultModel = "anthropic/claude-sonnet-4"

// OpenRouterClient handles API calls to OpenRouter.
type OpenRouterClient struct {
	APIKey    string
	Model     string
	WebSearch bool    // Enable web search for current data
	UseCache  bool    // Enable caching of API responses
	Verbose   bool
	Config    *Config // Configuration with prompts

	// Cache tracking
	cacheHits   int
	apiCalls    int
}

// NewOpenRouterClient creates a new client from environment variables and config.
func NewOpenRouterClient(model string, webSearch, useCache bool, config *Config, verbose bool) (*OpenRouterClient, error) {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENROUTER_API_KEY environment variable not set")
	}

	// Use config defaults if not specified
	if config == nil {
		config = DefaultConfig()
	}

	if model == "" {
		model = os.Getenv("OPENROUTER_MODEL")
		if model == "" {
			model = config.OpenRouter.DefaultModel
		}
	}

	// Use config's web search setting if not explicitly set
	if !webSearch {
		webSearch = config.OpenRouter.WebSearch
	}

	// Append :online suffix for web search if not already present
	if webSearch && !strings.HasSuffix(model, ":online") {
		model = model + ":online"
	}

	return &OpenRouterClient{
		APIKey:    apiKey,
		Model:     model,
		WebSearch: webSearch,
		UseCache:  useCache,
		Verbose:   verbose,
		Config:    config,
	}, nil
}

// APIDebugInfo captures request/response details for debugging.
type APIDebugInfo struct {
	Timestamp         string            `json:"timestamp"`
	Model             string            `json:"model"`
	RequestURL        string            `json:"request_url"`
	RequestHeaders    map[string]string `json:"request_headers"`
	RequestBody       json.RawMessage   `json:"request_body"`
	StatusCode        int               `json:"status_code"`
	StatusText        string            `json:"status_text"`
	ResponseHeaders   map[string]string `json:"response_headers"`
	ResponseBody      string            `json:"response_body"`
	ResponseBodyLen   int               `json:"response_body_len"`
	ContentLength     int64             `json:"content_length"`      // From Content-Length header (-1 if not set)
	TransferEncoding  []string          `json:"transfer_encoding"`   // chunked, etc.
	ConnectionClosed  bool              `json:"connection_closed"`   // Was Connection: close sent?
	Duration          string            `json:"duration"`
	ReadError         string            `json:"read_error,omitempty"`
	Error             string            `json:"error,omitempty"`
}

// saveDebugResponse saves a failed API response to a debug file for investigation.
func saveDebugResponse(prefix string, info *APIDebugInfo) string {
	debugDir := ".debug"
	if err := os.MkdirAll(debugDir, 0755); err != nil {
		return ""
	}

	filename := fmt.Sprintf("%s-%d.json", prefix, time.Now().Unix())
	path := filepath.Join(debugDir, filename)

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return ""
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return ""
	}

	return path
}

// ChatRequest represents an OpenRouter API request.
type ChatRequest struct {
	Model          string         `json:"model"`
	Messages       []ChatMessage  `json:"messages"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
	Temperature    float64        `json:"temperature,omitempty"`
}

// ChatMessage represents a message in the conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ResponseFormat specifies structured output format.
type ResponseFormat struct {
	Type       string      `json:"type"`
	JSONSchema *JSONSchema `json:"json_schema,omitempty"`
}

// JSONSchema defines the expected JSON structure.
type JSONSchema struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

// ChatResponse represents the API response.
type ChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// ClassifyResult is the structured output for icon classification.
type ClassifyResult struct {
	Icons []struct {
		Glyph string `json:"glyph"`
		Type  string `json:"type"` // "app", "category", "skip"
		Name  string `json:"name"` // canonical name
	} `json:"icons"`
}

// AppGenResult is the structured output for app name generation.
type AppGenResult struct {
	Mappings []struct {
		Icon string `json:"icon"`
		Apps []struct {
			ID         string  `json:"id"`
			Confidence float64 `json:"confidence"`
			Source     string  `json:"source"` // "package", "desktop", "flatpak", "snap", "inferred"
		} `json:"apps"`
	} `json:"mappings"`
}

// call makes a request to the OpenRouter API.
// Returns content, debug info, and error.
func (c *OpenRouterClient) call(req ChatRequest) (string, *APIDebugInfo, error) {
	start := time.Now()

	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", openRouterURL, bytes.NewReader(reqBody))
	if err != nil {
		return "", nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	httpReq.Header.Set("HTTP-Referer", "https://github.com/jmylchreest/histui")
	httpReq.Header.Set("X-Title", "histui icon-aliases generator")

	// Build debug info (mask API key)
	debugInfo := &APIDebugInfo{
		Timestamp:  start.UTC().Format(time.RFC3339),
		Model:      c.Model,
		RequestURL: openRouterURL,
		RequestHeaders: map[string]string{
			"Content-Type":  "application/json",
			"Authorization": "Bearer [REDACTED]",
			"HTTP-Referer":  httpReq.Header.Get("HTTP-Referer"),
			"X-Title":       httpReq.Header.Get("X-Title"),
		},
		RequestBody: reqBody,
	}

	timeout := time.Duration(c.Config.OpenRouter.RequestTimeout) * time.Second
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		debugInfo.Duration = time.Since(start).String()
		debugInfo.Error = err.Error()
		return "", debugInfo, fmt.Errorf("API call failed: %w", err)
	}
	defer resp.Body.Close()

	// Capture connection-level details before reading body
	debugInfo.StatusCode = resp.StatusCode
	debugInfo.StatusText = resp.Status
	debugInfo.ContentLength = resp.ContentLength
	debugInfo.TransferEncoding = resp.TransferEncoding
	debugInfo.ConnectionClosed = resp.Close

	// Capture response headers
	debugInfo.ResponseHeaders = make(map[string]string)
	for key, values := range resp.Header {
		debugInfo.ResponseHeaders[key] = strings.Join(values, ", ")
	}

	body, err := io.ReadAll(resp.Body)
	debugInfo.Duration = time.Since(start).String()
	debugInfo.ResponseBodyLen = len(body)
	debugInfo.ResponseBody = string(body)

	if err != nil {
		debugInfo.ReadError = err.Error()
		debugInfo.Error = fmt.Sprintf("read body error: %v", err)
		return "", debugInfo, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Show truncated body for debugging
		bodyStr := string(body)
		if len(bodyStr) > 1000 {
			bodyStr = bodyStr[:1000] + "... [truncated]"
		}
		fmt.Fprintf(os.Stderr, "\n[ERROR] API returned HTTP %d:\n%s\n", resp.StatusCode, bodyStr)
		debugInfo.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return "", debugInfo, fmt.Errorf("API error (HTTP %d)", resp.StatusCode)
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		// Show truncated body for debugging
		bodyStr := string(body)
		if len(bodyStr) > 500 {
			bodyStr = bodyStr[:500] + "... [truncated]"
		}
		fmt.Fprintf(os.Stderr, "\n[ERROR] Failed to parse API response:\n%s\n", bodyStr)
		debugInfo.Error = fmt.Sprintf("JSON parse error: %v", err)
		return "", debugInfo, fmt.Errorf("parse response: %w", err)
	}

	if chatResp.Error != nil {
		fmt.Fprintf(os.Stderr, "\n[ERROR] API error: %s\n", chatResp.Error.Message)
		debugInfo.Error = chatResp.Error.Message
		return "", debugInfo, fmt.Errorf("API error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		fmt.Fprintf(os.Stderr, "\n[ERROR] API returned no response choices\n")
		fmt.Fprintf(os.Stderr, "[ERROR] Full response: %s\n", string(body))
		debugInfo.Error = "no response choices"
		return "", debugInfo, fmt.Errorf("no response choices")
	}

	return chatResp.Choices[0].Message.Content, debugInfo, nil
}

// classifyIconsSchema returns the JSON schema for icon classification.
func classifyIconsSchema() *JSONSchema {
	return &JSONSchema{
		Name:   "icon_classification",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"icons": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"glyph": map[string]any{"type": "string"},
							"type":  map[string]any{"type": "string", "enum": []string{"app", "category", "skip"}},
							"name":  map[string]any{"type": "string"},
						},
						"required":             []string{"glyph", "type", "name"},
						"additionalProperties": false,
					},
				},
			},
			"required":             []string{"icons"},
			"additionalProperties": false,
		},
	}
}

// appGenSchema returns the JSON schema for app name generation.
func appGenSchema() *JSONSchema {
	return &JSONSchema{
		Name:   "app_generation",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"mappings": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"icon": map[string]any{"type": "string"},
							"apps": map[string]any{
								"type": "array",
								"items": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"id":         map[string]any{"type": "string"},
										"confidence": map[string]any{"type": "number"},
										"source":     map[string]any{"type": "string", "enum": []string{"package", "desktop", "flatpak", "snap", "inferred"}},
									},
									"required":             []string{"id", "confidence", "source"},
									"additionalProperties": false,
								},
							},
						},
						"required":             []string{"icon", "apps"},
						"additionalProperties": false,
					},
				},
			},
			"required":             []string{"mappings"},
			"additionalProperties": false,
		},
	}
}

// ClassifyIcons classifies a batch of glyph names.
func (c *OpenRouterClient) ClassifyIcons(glyphNames []string, useCache bool) (*ClassifyResult, error) {
	// Check cache first
	cacheKey := ClassifyCacheKey(glyphNames)
	if useCache {
		if cached, ok := loadCache("classify", cacheKey, c.Model); ok {
			var result ClassifyResult
			if err := json.Unmarshal([]byte(cached), &result); err == nil {
				c.cacheHits++
				fmt.Printf("    [CACHE HIT] classification batch\n")
				return &result, nil
			}
		}
	}

	c.apiCalls++
	fmt.Printf("    [API CALL] classification batch\n")
	prompt, err := c.Config.RenderClassifyPrompt(glyphNames)
	if err != nil {
		return nil, fmt.Errorf("render classify prompt: %w", err)
	}

	req := ChatRequest{
		Model: c.Model,
		Messages: []ChatMessage{
			{Role: "user", Content: prompt},
		},
		ResponseFormat: &ResponseFormat{
			Type:       "json_schema",
			JSONSchema: classifyIconsSchema(),
		},
		Temperature: 0.1,
	}

	content, debugInfo, err := c.call(req)
	if err != nil {
		if debugInfo != nil {
			debugPath := saveDebugResponse("classify-error", debugInfo)
			if debugPath != "" {
				fmt.Fprintf(os.Stderr, "[ERROR] Debug info saved to: %s\n", debugPath)
			}
		}
		return nil, err
	}

	// Save to cache
	if useCache {
		if err := saveCache("classify", cacheKey, c.Model, content); err != nil && c.Verbose {
			fmt.Printf("    (warning: failed to save cache: %v)\n", err)
		}
	}

	var result ClassifyResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		// Save full debug info including headers
		debugInfo.Error = fmt.Sprintf("JSON unmarshal error: %v", err)
		debugPath := saveDebugResponse("classify-parse-error", debugInfo)

		// Show truncated content for debugging
		preview := content
		if len(preview) > 500 {
			preview = preview[:500] + "... [truncated]"
		}
		fmt.Fprintf(os.Stderr, "\n[ERROR] Failed to parse API response:\n%s\n", preview)
		fmt.Fprintf(os.Stderr, "[ERROR] Response length: %d bytes\n", len(content))
		if debugPath != "" {
			fmt.Fprintf(os.Stderr, "[ERROR] Full debug info saved to: %s\n", debugPath)
		}

		// Check for common issues
		if len(content) < 100 {
			fmt.Fprintf(os.Stderr, "[ERROR] Response is suspiciously short - API may have truncated output\n")
		}
		if !strings.HasPrefix(strings.TrimSpace(content), "{") {
			fmt.Fprintf(os.Stderr, "[ERROR] Response does not start with '{' - may not be JSON\n")
		}
		if !strings.HasSuffix(strings.TrimSpace(content), "}") {
			fmt.Fprintf(os.Stderr, "[ERROR] Response does not end with '}' - JSON appears truncated\n")
		}

		return nil, fmt.Errorf("parse classification result: %w", err)
	}

	return &result, nil
}

// GenerateAppNames generates Linux app identifiers for icons.
func (c *OpenRouterClient) GenerateAppNames(icons []struct{ Name, Type string }, useCache bool) (*AppGenResult, error) {
	// Check cache first
	cacheKey := AppGenCacheKey(icons)
	if useCache {
		if cached, ok := loadCache("appgen", cacheKey, c.Model); ok {
			var result AppGenResult
			if err := json.Unmarshal([]byte(cached), &result); err == nil {
				c.cacheHits++
				fmt.Printf("    [CACHE HIT] app generation batch\n")
				return &result, nil
			}
		}
	}

	c.apiCalls++
	fmt.Printf("    [API CALL] app generation batch\n")
	var iconList []string
	for _, icon := range icons {
		iconList = append(iconList, fmt.Sprintf("- %s (%s)", icon.Name, icon.Type))
	}

	prompt, err := c.Config.RenderAppGenPrompt(iconList)
	if err != nil {
		return nil, fmt.Errorf("render app gen prompt: %w", err)
	}

	req := ChatRequest{
		Model: c.Model,
		Messages: []ChatMessage{
			{Role: "user", Content: prompt},
		},
		ResponseFormat: &ResponseFormat{
			Type:       "json_schema",
			JSONSchema: appGenSchema(),
		},
		Temperature: 0.2,
	}

	content, debugInfo, err := c.call(req)
	if err != nil {
		if debugInfo != nil {
			debugPath := saveDebugResponse("appgen-error", debugInfo)
			if debugPath != "" {
				fmt.Fprintf(os.Stderr, "[ERROR] Debug info saved to: %s\n", debugPath)
			}
		}
		return nil, err
	}

	// Save to cache
	if useCache {
		if err := saveCache("appgen", cacheKey, c.Model, content); err != nil && c.Verbose {
			fmt.Printf("    (warning: failed to save cache: %v)\n", err)
		}
	}

	var result AppGenResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		// Save full debug info including headers
		debugInfo.Error = fmt.Sprintf("JSON unmarshal error: %v", err)
		debugPath := saveDebugResponse("appgen-parse-error", debugInfo)

		// Show truncated content for debugging
		preview := content
		if len(preview) > 500 {
			preview = preview[:500] + "... [truncated]"
		}
		fmt.Fprintf(os.Stderr, "\n[ERROR] Failed to parse API response:\n%s\n", preview)
		fmt.Fprintf(os.Stderr, "[ERROR] Response length: %d bytes\n", len(content))
		if debugPath != "" {
			fmt.Fprintf(os.Stderr, "[ERROR] Full debug info saved to: %s\n", debugPath)
		}

		// Check for common issues
		if len(content) < 100 {
			fmt.Fprintf(os.Stderr, "[ERROR] Response is suspiciously short - API may have truncated output\n")
		}
		if !strings.HasPrefix(strings.TrimSpace(content), "{") {
			fmt.Fprintf(os.Stderr, "[ERROR] Response does not start with '{' - may not be JSON\n")
		}
		if !strings.HasSuffix(strings.TrimSpace(content), "}") {
			fmt.Fprintf(os.Stderr, "[ERROR] Response does not end with '}' - JSON appears truncated\n")
		}

		return nil, fmt.Errorf("parse app generation result: %w", err)
	}

	return &result, nil
}

// progressTicker starts a goroutine that prints progress updates.
// Returns a stop function to call when the operation completes.
func progressTicker(phase string, batchNum, totalBatches int) func() {
	start := time.Now()
	done := make(chan struct{})

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				elapsed := time.Since(start).Round(time.Second)
				fmt.Printf("\r  [%s] Batch %d/%d - waiting %s...   ", phase, batchNum, totalBatches, elapsed)
			}
		}
	}()

	return func() {
		close(done)
		elapsed := time.Since(start).Round(time.Second)
		fmt.Printf("\r  [%s] Batch %d/%d - completed in %s\n", phase, batchNum, totalBatches, elapsed)
	}
}

// GenerateKnowledgeBase generates the full AI knowledge base from Nerd Font glyphs.
func (c *OpenRouterClient) GenerateKnowledgeBase(glyphs map[string]GlyphInfo) (*KnowledgeBase, error) {
	// Filter to app-related glyphs
	appGlyphs := filterAppGlyphs(glyphs, c.Config.Filter)

	// Get all glyph names (sorted for deterministic batching and cache hits)
	var glyphNames []string
	for name := range appGlyphs {
		glyphNames = append(glyphNames, name)
	}
	sort.Strings(glyphNames)

	classifyBatch := c.Config.OpenRouter.ClassifyBatchSize
	appGenBatch := c.Config.OpenRouter.AppGenBatchSize

	// Calculate total batches
	classifyBatches := (len(glyphNames) + classifyBatch - 1) / classifyBatch
	fmt.Printf("Classifying %d glyphs in %d batches (batch size: %d)...\n", len(glyphNames), classifyBatches, classifyBatch)

	// Phase 1: Classify icons in batches
	var classified []struct {
		Glyph string
		Type  string
		Name  string
	}

	batchNum := 0
	for i := 0; i < len(glyphNames); i += classifyBatch {
		batchNum++
		end := i + classifyBatch
		if end > len(glyphNames) {
			end = len(glyphNames)
		}
		batch := glyphNames[i:end]

		stop := progressTicker("classify", batchNum, classifyBatches)
		result, err := c.ClassifyIcons(batch, c.UseCache)
		stop()

		if err != nil {
			return nil, fmt.Errorf("classify batch %d: %w", batchNum, err)
		}

		for _, icon := range result.Icons {
			if icon.Type != "skip" {
				classified = append(classified, struct {
					Glyph string
					Type  string
					Name  string
				}{icon.Glyph, icon.Type, icon.Name})
			}
		}

		// Rate limiting between batches
		if i+classifyBatch < len(glyphNames) {
			time.Sleep(500 * time.Millisecond)
		}
	}

	fmt.Printf("Classified %d relevant icons (app + category)\n", len(classified))

	// Phase 2: Generate app names in batches
	kb := &KnowledgeBase{
		Version:     1,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Model:       c.Model,
		Icons:       make(map[string]KBIcon),
	}

	appGenBatches := (len(classified) + appGenBatch - 1) / appGenBatch
	fmt.Printf("Generating app names for %d icons in %d batches (batch size: %d)...\n", len(classified), appGenBatches, appGenBatch)

	batchNum = 0
	for i := 0; i < len(classified); i += appGenBatch {
		batchNum++
		end := i + appGenBatch
		if end > len(classified) {
			end = len(classified)
		}
		batch := classified[i:end]

		var icons []struct{ Name, Type string }
		for _, cl := range batch {
			icons = append(icons, struct{ Name, Type string }{cl.Name, cl.Type})
		}

		stop := progressTicker("app-gen", batchNum, appGenBatches)
		result, err := c.GenerateAppNames(icons, c.UseCache)
		stop()

		if err != nil {
			return nil, fmt.Errorf("generate apps batch %d: %w", batchNum, err)
		}

		// Map back to glyphs and store
		glyphMap := make(map[string]string) // name -> glyph
		typeMap := make(map[string]string)  // name -> type
		for _, c := range batch {
			glyphMap[c.Name] = c.Glyph
			typeMap[c.Name] = c.Type
		}

		for _, mapping := range result.Mappings {
			glyph, ok := glyphMap[mapping.Icon]
			if !ok {
				continue
			}

			var apps []KBApp
			for _, app := range mapping.Apps {
				apps = append(apps, KBApp{
					ID:         app.ID,
					Confidence: app.Confidence,
					Source:     app.Source,
				})
			}

			kb.Icons[mapping.Icon] = KBIcon{
				Type:  typeMap[mapping.Icon],
				Glyph: glyph,
				Apps:  apps,
			}
		}

		// Rate limiting between batches
		if i+appGenBatch < len(classified) {
			time.Sleep(500 * time.Millisecond)
		}
	}

	// Print cache summary
	total := c.cacheHits + c.apiCalls
	if total > 0 {
		if c.apiCalls == 0 {
			fmt.Printf("\n*** ALL %d requests served from cache - no API calls made ***\n", c.cacheHits)
			fmt.Println("*** Use --no-cache to force fresh API calls ***")
		} else {
			fmt.Printf("\nCache: %d hits, %d API calls (%.0f%% cache hit rate)\n",
				c.cacheHits, c.apiCalls, float64(c.cacheHits)/float64(total)*100)
		}
	}

	fmt.Printf("Generated knowledge base with %d icon mappings\n", len(kb.Icons))
	return kb, nil
}
