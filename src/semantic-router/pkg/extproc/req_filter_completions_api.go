package extproc

import (
	"encoding/json"
	"fmt"

	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/observability/logging"
)

// CompletionsAPIContext holds context for a /v1/completions request during processing.
type CompletionsAPIContext struct {
	// IsCompletionsRequest indicates this is a /v1/completions request
	IsCompletionsRequest bool

	// OriginalRequestBody is the original /v1/completions request body
	OriginalRequestBody []byte

	// TranslatedBody is the Chat Completions request body after translation
	TranslatedBody []byte
}

// CompletionRequest represents a /v1/completions request structure
type CompletionRequest struct {
	Model       interface{} `json:"model"`
	Prompt      interface{} `json:"prompt"` // Can be string or []string
	MaxTokens   int         `json:"max_tokens,omitempty"`
	Temperature float32     `json:"temperature,omitempty"`
	TopP        float32     `json:"top_p,omitempty"`
	N           int         `json:"n,omitempty"`
	Stream      bool        `json:"stream,omitempty"`
	Stop        interface{} `json:"stop,omitempty"`
	// Add other fields as needed
}

// TranslateCompletionsRequest translates a /v1/completions request to /v1/chat/completions format.
// This enables auto model routing for text completion requests.
func TranslateCompletionsRequest(body []byte) (*CompletionsAPIContext, []byte, error) {
	// Parse the completions request
	var req CompletionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, nil, fmt.Errorf("failed to parse completions request: %w", err)
	}

	if req.Prompt == nil {
		return nil, nil, fmt.Errorf("completions request missing required 'prompt' field")
	}

	var promptText string
	switch p := req.Prompt.(type) {
	case string:
		promptText = p
	case []interface{}:
		if len(p) > 0 {
			if str, ok := p[0].(string); ok {
				promptText = str
			}
		}
	default:
		return nil, nil, fmt.Errorf("invalid prompt format: expected string or array")
	}

	if promptText == "" {
		return nil, nil, fmt.Errorf("prompt is empty")
	}

	// Build chat completion request as JSON map
	chatReqMap := make(map[string]interface{})

	chatReqMap["model"] = req.Model

	chatReqMap["messages"] = []map[string]string{
		{
			"role":    "user",
			"content": promptText,
		},
	}

	if req.MaxTokens > 0 {
		chatReqMap["max_tokens"] = req.MaxTokens
	}
	if req.Temperature > 0 {
		chatReqMap["temperature"] = req.Temperature
	}
	if req.TopP > 0 {
		chatReqMap["top_p"] = req.TopP
	}
	if req.N > 0 {
		chatReqMap["n"] = req.N
	}
	if req.Stream {
		chatReqMap["stream"] = true
	}
	if req.Stop != nil {
		chatReqMap["stop"] = req.Stop
	}

	translatedBody, err := json.Marshal(chatReqMap)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal chat completion request: %w", err)
	}

	ctx := &CompletionsAPIContext{
		IsCompletionsRequest: true,
		OriginalRequestBody:  body,
		TranslatedBody:       translatedBody,
	}

	logging.Infof("Completions API: Translated /v1/completions to /v1/chat/completions (prompt: %d chars)", len(promptText))

	return ctx, translatedBody, nil
}

// convertChatToCompletionsFormat converts a chat completion request back to completions format
// It extracts the model from chatBody and the other parameters from originalCompletionsBody
func convertChatToCompletionsFormat(chatBody []byte, originalCompletionsBody []byte) ([]byte, error) {
	var chatReq map[string]interface{}
	if err := json.Unmarshal(chatBody, &chatReq); err != nil {
		return nil, fmt.Errorf("failed to parse chat request: %w", err)
	}

	var origReq map[string]interface{}
	if err := json.Unmarshal(originalCompletionsBody, &origReq); err != nil {
		return nil, fmt.Errorf("failed to parse original completions request: %w", err)
	}

	if model, ok := chatReq["model"]; ok {
		origReq["model"] = model
	}

	completionsBody, err := json.Marshal(origReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal completions request: %w", err)
	}

	logging.Infof("Completions API: Converted chat format back to /v1/completions format with model: %v", origReq["model"])

	return completionsBody, nil
}
