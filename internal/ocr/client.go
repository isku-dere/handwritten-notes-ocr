package ocr

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Line struct {
	Text  string      `json:"text"`
	Score float64     `json:"score"`
	Box   [][]float64 `json:"box"`
}

type Result struct {
	FileName string `json:"fileName"`
	Lines    []Line `json:"lines"`
	Markdown string `json:"markdown"`
	RawText  string `json:"rawText"`
}

type Client struct {
	PythonBin    string
	ScriptPath   string
	Language     string
	Timeout      time.Duration
	OnlineAPIURL string
	APIToken     string
	HTTPClient   *http.Client
}

func (c *Client) Run(ctx context.Context, imagePath string) (*Result, error) {
	if c.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}

	if c.OnlineAPIURL != "" {
		result, err := c.runOnline(ctx, imagePath)
		if err == nil {
			return result, nil
		}
		if c.PythonBin == "" || c.ScriptPath == "" {
			return nil, err
		}
	}

	cmd := exec.CommandContext(ctx, c.PythonBin, c.ScriptPath, "--image", imagePath, "--lang", c.Language)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("run paddleocr script: %w: %s", err, strings.TrimSpace(string(out)))
	}

	var result Result
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("decode ocr result: %w", err)
	}

	if result.FileName == "" {
		result.FileName = filepath.Base(imagePath)
	}

	return &result, nil
}

func (c *Client) runOnline(ctx context.Context, imagePath string) (*Result, error) {
	imageBytes, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("read image for online ocr: %w", err)
	}

	payload := map[string]any{
		"file":                      base64.StdEncoding.EncodeToString(imageBytes),
		"fileType":                  1,
		"useDocOrientationClassify": false,
		"useDocUnwarping":           false,
		"useChartRecognition":       false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode online ocr payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.OnlineAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build online ocr request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIToken != "" {
		req.Header.Set("Authorization", "token "+c.APIToken)
	}

	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: c.Timeout}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call online ocr api: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read online ocr response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("online ocr api returned %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var onlineResult struct {
		Result struct {
			LayoutParsingResults []struct {
				Markdown struct {
					Text   string            `json:"text"`
					Images map[string]string `json:"images"`
				} `json:"markdown"`
			} `json:"layoutParsingResults"`
		} `json:"result"`
	}
	if err := json.Unmarshal(respBody, &onlineResult); err != nil {
		return nil, fmt.Errorf("decode online ocr result: %w", err)
	}

	var markdownParts []string
	for _, item := range onlineResult.Result.LayoutParsingResults {
		if text := strings.TrimSpace(item.Markdown.Text); text != "" {
			markdownParts = append(markdownParts, text)
		}
	}
	if len(markdownParts) == 0 {
		return nil, fmt.Errorf("online ocr result did not include markdown text")
	}

	markdownText := strings.Join(markdownParts, "\n\n")
	return &Result{
		FileName: filepath.Base(imagePath),
		Markdown: markdownText,
		RawText:  markdownText,
	}, nil
}
