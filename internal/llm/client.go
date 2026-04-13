package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	APIKey     string
	BaseURL    string
	Model      string
	Timeout    time.Duration
	HTTPClient *http.Client
}

func (c *Client) Chat(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return c.doChat(ctx, systemPrompt, userPrompt, false, nil)
}

func (c *Client) ChatStream(ctx context.Context, systemPrompt, userPrompt string, onDelta func(string) error) (string, error) {
	return c.doChat(ctx, systemPrompt, userPrompt, true, onDelta)
}

func (c *Client) doChat(ctx context.Context, systemPrompt, userPrompt string, stream bool, onDelta func(string) error) (string, error) {
	if strings.TrimSpace(c.APIKey) == "" {
		return "", fmt.Errorf("Qwen API key is not configured")
	}
	if strings.TrimSpace(c.BaseURL) == "" {
		return "", fmt.Errorf("Qwen base URL is not configured")
	}
	if strings.TrimSpace(c.Model) == "" {
		return "", fmt.Errorf("Qwen model is not configured")
	}

	body, err := json.Marshal(map[string]any{
		"model": c.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.2,
		"stream":      stream,
	})
	if err != nil {
		return "", fmt.Errorf("marshal llm request: %w", err)
	}

	endpoint := strings.TrimRight(c.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build llm request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: c.Timeout}
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("call qwen api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return "", fmt.Errorf("qwen api returned %d", resp.StatusCode)
		}
		return "", fmt.Errorf("qwen api returned %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	if stream {
		return readStream(resp.Body, onDelta)
	}
	return readFull(resp.Body)
}

func readFull(body io.Reader) (string, error) {
	respBody, err := io.ReadAll(body)
	if err != nil {
		return "", fmt.Errorf("read llm response: %w", err)
	}

	var payload struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return "", fmt.Errorf("decode llm response: %w", err)
	}
	if len(payload.Choices) == 0 {
		return "", fmt.Errorf("qwen api returned no choices")
	}

	return strings.TrimSpace(payload.Choices[0].Message.Content), nil
}

func readStream(body io.Reader, onDelta func(string) error) (string, error) {
	reader := bufio.NewReader(body)
	var output strings.Builder

	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", fmt.Errorf("read llm stream: %w", err)
		}

		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			payload := strings.TrimPrefix(line, "data: ")
			if payload == "[DONE]" {
				return strings.TrimSpace(output.String()), nil
			}

			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if unmarshalErr := json.Unmarshal([]byte(payload), &chunk); unmarshalErr == nil && len(chunk.Choices) > 0 {
				text := chunk.Choices[0].Delta.Content
				if text != "" {
					output.WriteString(text)
					if onDelta != nil {
						if callbackErr := onDelta(text); callbackErr != nil {
							return "", callbackErr
						}
					}
				}
			}
		}

		if err == io.EOF {
			break
		}
	}

	return strings.TrimSpace(output.String()), nil
}
