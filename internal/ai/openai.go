package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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
	text, err := p.generate(ctx, commitMessageSystemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}
	return parseCommitMessage(text), nil
}

// generate sends a request to the OpenAI Chat Completions API with the given
// system prompt and user prompt, returning the raw response text.
func (p *OpenAIProvider) generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	reqBody := openAIRequest{
		Model:       p.model,
		MaxTokens:   1024,
		Temperature: 0.3,
		Messages: []openAIMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := p.baseURL + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp openAIResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error != nil {
			return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var result openAIResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("API returned empty response")
	}

	return result.Choices[0].Message.Content, nil
}

// openAIStreamRequest includes the stream flag.
type openAIStreamRequest struct {
	Model       string          `json:"model"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature"`
	Messages    []openAIMessage `json:"messages"`
	Stream      bool            `json:"stream"`
}

// openAIStreamChunk represents a single SSE chunk from the streaming API.
type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

// GenerateStream implements StreamingProvider for OpenAI.
func (p *OpenAIProvider) GenerateStream(ctx context.Context, systemPrompt, userPrompt string, onChunk func(chunk string)) (string, error) {
	reqBody := openAIStreamRequest{
		Model:       p.model,
		MaxTokens:   1024,
		Temperature: 0.3,
		Messages: []openAIMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Stream: true,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := p.baseURL + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	// Parse SSE stream
	var full strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				full.WriteString(choice.Delta.Content)
				if onChunk != nil {
					onChunk(choice.Delta.Content)
				}
			}
		}
	}

	return full.String(), scanner.Err()
}
