package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
	Verbose   bool
	Config    *Config // Configuration with prompts
}

// NewOpenRouterClient creates a new client from environment variables and config.
func NewOpenRouterClient(model string, webSearch bool, config *Config, verbose bool) (*OpenRouterClient, error) {
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
		Verbose:   verbose,
		Config:    config,
	}, nil
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
func (c *OpenRouterClient) call(req ChatRequest) (string, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", openRouterURL, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	httpReq.Header.Set("HTTP-Referer", "https://github.com/jmylchreest/histui")
	httpReq.Header.Set("X-Title", "histui icon-aliases generator")

	timeout := time.Duration(c.Config.OpenRouter.RequestTimeout) * time.Second
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("API call failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("API error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no response choices")
	}

	return chatResp.Choices[0].Message.Content, nil
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
func (c *OpenRouterClient) ClassifyIcons(glyphNames []string) (*ClassifyResult, error) {
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

	content, err := c.call(req)
	if err != nil {
		return nil, err
	}

	var result ClassifyResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("parse classification result: %w", err)
	}

	return &result, nil
}

// GenerateAppNames generates Linux app identifiers for icons.
func (c *OpenRouterClient) GenerateAppNames(icons []struct{ Name, Type string }) (*AppGenResult, error) {
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

	content, err := c.call(req)
	if err != nil {
		return nil, err
	}

	var result AppGenResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("parse app generation result: %w", err)
	}

	return &result, nil
}

// GenerateKnowledgeBase generates the full AI knowledge base from Nerd Font glyphs.
func (c *OpenRouterClient) GenerateKnowledgeBase(glyphs map[string]GlyphInfo) (*KnowledgeBase, error) {
	// Filter to app-related glyphs
	appGlyphs := filterAppGlyphs(glyphs)

	// Get all glyph names
	var glyphNames []string
	for name := range appGlyphs {
		glyphNames = append(glyphNames, name)
	}

	classifyBatch := c.Config.OpenRouter.ClassifyBatchSize
	appGenBatch := c.Config.OpenRouter.AppGenBatchSize

	fmt.Printf("Classifying %d glyphs in batches of %d...\n", len(glyphNames), classifyBatch)

	// Phase 1: Classify icons in batches
	var classified []struct {
		Glyph string
		Type  string
		Name  string
	}

	for i := 0; i < len(glyphNames); i += classifyBatch {
		end := i + classifyBatch
		if end > len(glyphNames) {
			end = len(glyphNames)
		}
		batch := glyphNames[i:end]

		if c.Verbose {
			fmt.Printf("  Classifying batch %d-%d of %d...\n", i+1, end, len(glyphNames))
		}

		result, err := c.ClassifyIcons(batch)
		if err != nil {
			return nil, fmt.Errorf("classify batch %d: %w", i/classifyBatch+1, err)
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

		// Rate limiting
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Printf("Classified %d relevant icons (app + category)\n", len(classified))

	// Phase 2: Generate app names in batches
	kb := &KnowledgeBase{
		Version:     1,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Model:       c.Model,
		Icons:       make(map[string]KBIcon),
	}

	for i := 0; i < len(classified); i += appGenBatch {
		end := i + appGenBatch
		if end > len(classified) {
			end = len(classified)
		}
		batch := classified[i:end]

		if c.Verbose {
			fmt.Printf("  Generating apps for batch %d-%d of %d...\n", i+1, end, len(classified))
		}

		var icons []struct{ Name, Type string }
		for _, cl := range batch {
			icons = append(icons, struct{ Name, Type string }{cl.Name, cl.Type})
		}

		result, err := c.GenerateAppNames(icons)
		if err != nil {
			return nil, fmt.Errorf("generate apps batch %d: %w", i/appGenBatch+1, err)
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

		// Rate limiting
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Printf("Generated knowledge base with %d icon mappings\n", len(kb.Icons))
	return kb, nil
}
