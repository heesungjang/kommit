package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const openAIDefaultBaseURL = "https://api.openai.com"

// OpenAIProvider implements Provider using the OpenAI Chat Completions API.
// It also works with any OpenAI-compatible endpoint (Ollama, Groq, DeepSeek,
// LM Studio, Together AI, etc.) by setting a custom baseURL.
type OpenAIProvider struct {
	apiKey  string
	model   string
	baseURL string
}

// openAIRequest is the request body for the OpenAI Chat Completions API.
type openAIRequest struct {
	Model       string          `json:"model"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature"`
	Messages    []openAIMessage `json:"messages"`
}

// openAIMessage is a single message in the OpenAI conversation.
type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openAIResponse is the response from the OpenAI Chat Completions API.
type openAIResponse struct {
	Choices []openAIChoice `json:"choices"`
	Error   *openAIError   `json:"error,omitempty"`
}

// openAIChoice is a single completion choice.
type openAIChoice struct {
	Message openAIMessage `json:"message"`
}

// openAIError is an error returned by the OpenAI API.
type openAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// GenerateCommitMessage implements Provider.
func (p *OpenAIProvider) GenerateCommitMessage(ctx context.Context, diff, stat string) (*CommitMessage, error) {
	userPrompt := buildCommitPrompt(diff, stat)

	reqBody := openAIRequest{
		Model:       p.model,
		MaxTokens:   512,
		Temperature: 0.3,
		Messages: []openAIMessage{
			{Role: "system", Content: commitMessageSystemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := p.baseURL + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp openAIResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error != nil {
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var result openAIResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("API returned empty response")
	}

	return parseCommitMessage(result.Choices[0].Message.Content), nil
}
