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

const (
	openRouterURL    = "https://openrouter.ai/api/v1/chat/completions"
	defaultModel     = "anthropic/claude-sonnet-4"
	classifyBatchSize = 50
	appGenBatchSize   = 20
)

// OpenRouterClient handles API calls to OpenRouter.
type OpenRouterClient struct {
	APIKey  string
	Model   string
	Verbose bool
}

// NewOpenRouterClient creates a new client from environment variables.
func NewOpenRouterClient(model string, verbose bool) (*OpenRouterClient, error) {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENROUTER_API_KEY environment variable not set")
	}

	if model == "" {
		model = os.Getenv("OPENROUTER_MODEL")
		if model == "" {
			model = defaultModel
		}
	}

	return &OpenRouterClient{
		APIKey:  apiKey,
		Model:   model,
		Verbose: verbose,
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

	client := &http.Client{Timeout: 120 * time.Second}
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
	prompt := fmt.Sprintf(`You are classifying Nerd Font icon names for a Linux desktop notification system.

For each icon glyph name, determine:
- type: "app" if it represents a specific application (Discord, Firefox, Spotify), "category" if it's generic (email, browser, folder, music), or "skip" if not useful for app icons (arrows, shapes, abstract symbols)
- name: the canonical lowercase name extracted from the glyph (e.g., "md-discord" → "discord", "fa-envelope" → "email", "md-folder" → "folder")

Focus on icons that would be useful for matching Linux desktop applications and notification sources.

Icons to classify:
%s

Respond with JSON matching the schema.`, strings.Join(glyphNames, "\n"))

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

	prompt := fmt.Sprintf(`You are generating Linux application identifiers for icon mappings in a desktop notification system.

For each icon, list all Linux apps that would use this icon. Include:
- Package names (apt, pacman, dnf, etc.): discord, firefox, thunderbird
- Desktop file base names: org.mozilla.firefox, com.discordapp.Discord
- Flatpak application IDs: com.discordapp.Discord, org.mozilla.firefox
- Common variants and forks: discord-canary, firefox-esr, firefox-nightly, librewolf

Confidence scoring guidelines:
- 0.9-1.0: Official/primary app that exactly matches the icon (discord for discord icon)
- 0.7-0.9: Well-known official variants (discord-canary, firefox-esr)
- 0.5-0.7: Popular third-party clients or forks (vesktop, librewolf, evolution for email)
- 0.3-0.5: Less common alternatives or inferred matches

For "category" type icons (email, browser, file-manager, music), list the most popular Linux applications in that category.

Icons to map (format: name (type)):
%s

Be comprehensive but accurate. Only include apps you're confident exist on Linux.

Respond with JSON matching the schema.`, strings.Join(iconList, "\n"))

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

	fmt.Printf("Classifying %d glyphs in batches of %d...\n", len(glyphNames), classifyBatchSize)

	// Phase 1: Classify icons in batches
	var classified []struct {
		Glyph string
		Type  string
		Name  string
	}

	for i := 0; i < len(glyphNames); i += classifyBatchSize {
		end := i + classifyBatchSize
		if end > len(glyphNames) {
			end = len(glyphNames)
		}
		batch := glyphNames[i:end]

		if c.Verbose {
			fmt.Printf("  Classifying batch %d-%d of %d...\n", i+1, end, len(glyphNames))
		}

		result, err := c.ClassifyIcons(batch)
		if err != nil {
			return nil, fmt.Errorf("classify batch %d: %w", i/classifyBatchSize+1, err)
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

	for i := 0; i < len(classified); i += appGenBatchSize {
		end := i + appGenBatchSize
		if end > len(classified) {
			end = len(classified)
		}
		batch := classified[i:end]

		if c.Verbose {
			fmt.Printf("  Generating apps for batch %d-%d of %d...\n", i+1, end, len(classified))
		}

		var icons []struct{ Name, Type string }
		for _, c := range batch {
			icons = append(icons, struct{ Name, Type string }{c.Name, c.Type})
		}

		result, err := c.GenerateAppNames(icons)
		if err != nil {
			return nil, fmt.Errorf("generate apps batch %d: %w", i/appGenBatchSize+1, err)
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
