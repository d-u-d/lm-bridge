package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

const DefaultBaseURL = "http://localhost:1234/v1"
const OpenRouterBaseURL = "https://openrouter.ai/api/v1"

type Client struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

// New creates a client. Pass empty strings to use defaults.
// apiKey non-empty → OpenRouter mode (baseURL defaults to OpenRouterBaseURL).
// model non-empty → overrides the model name in every request.
func New(baseURL, apiKey, model string) *Client {
	if baseURL == "" {
		if apiKey != "" {
			baseURL = OpenRouterBaseURL
		} else {
			baseURL = DefaultBaseURL
		}
	}
	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{Timeout: 1800 * time.Second},
	}
}

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`           // no omitempty — LM Studio requires field to always be present
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

var ErrLoopDetected = errors.New("loop detected: model is repeating output")

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Tools    []Tool    `json:"tools,omitempty"`
	Think    bool      `json:"think,omitempty"`
	Stream   bool      `json:"stream,omitempty"`
}

type ChatResponse struct {
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ModelList struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

func (c *Client) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if req.Model == "" {
		if c.model != "" {
			req.Model = c.model
		} else {
			req.Model = "local-model"
		}
	}

	// Pre-flight idle check only for local LM Studio (no API key).
	if c.apiKey == "" {
		if err := c.checkIdle(ctx); err != nil {
			return nil, err
		}
	}

	return c.doChat(ctx, req)
}

// checkIdle does a quick /v1/models probe with a short timeout.
// Returns an error if LM Studio is unreachable or appears busy (no response within 5s).
func (c *Client) checkIdle(ctx context.Context) error {
	probe, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(probe, "GET", c.baseURL+"/models", nil)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return fmt.Errorf("LM Studio is not running (connection refused)")
		}
		return fmt.Errorf("LM Studio is busy (another generation is running) — try again later")
	}
	resp.Body.Close()
	return nil
}

func (c *Client) doChat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("provider unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("provider error %d: %s", resp.StatusCode, string(b))
	}

	var result ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("empty response from model")
	}
	return &result, nil
}

// ChatStream streams response tokens to out, returns ErrLoopDetected if repetition found.
func (c *Client) ChatStream(ctx context.Context, req ChatRequest, out io.Writer) error {
	if req.Model == "" {
		if c.model != "" {
			req.Model = c.model
		} else {
			req.Model = "local-model"
		}
	}
	req.Stream = true
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("provider unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("provider error %d: %s", resp.StatusCode, string(b))
	}

	var buf strings.Builder
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
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		token := chunk.Choices[0].Delta.Content
		fmt.Fprint(out, token)
		buf.WriteString(token)

		if detectLoop(buf.String()) {
			fmt.Fprintln(out) // newline before error
			return ErrLoopDetected
		}
	}
	return scanner.Err()
}

// detectLoop returns true if the tail of s contains a repeating pattern — sign of a stuck model.
func detectLoop(s string) bool {
	const window = 120
	if len(s) < window*3 {
		return false
	}
	tail := s[len(s)-window:]
	prev := s[len(s)-window*2 : len(s)-window]
	t := strings.TrimSpace(tail)
	p := strings.TrimSpace(prev)
	return len(t) > 15 && t == p
}

// ModelStatus returns whether the provider is reachable and the active model name.
func (c *Client) ModelStatus(ctx context.Context) (online bool, modelName string, err error) {
	if c.apiKey != "" {
		// API-based provider: report configured model directly.
		return true, c.model, nil
	}
	// LM Studio: probe /models endpoint.
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/models", nil)
	if err != nil {
		return false, "", err
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return false, "", nil
	}
	defer resp.Body.Close()

	var list ModelList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return true, "", nil
	}
	if len(list.Data) > 0 {
		return true, list.Data[0].ID, nil
	}
	return true, "", nil
}

// ProviderLabel returns "openrouter" or "lmstudio" for use in call records.
func (c *Client) ProviderLabel() string {
	if c.apiKey != "" {
		return "openrouter"
	}
	return "lmstudio"
}

// ModelLabel returns the configured model name (may be empty for LM Studio).
func (c *Client) ModelLabel() string {
	return c.model
}

// openRouterModel is used when fetching the free-model list.
type openRouterModel struct {
	ID      string `json:"id"`
	Pricing struct {
		Prompt     string `json:"prompt"`
		Completion string `json:"completion"`
	} `json:"pricing"`
}

// FetchFreeModels returns the IDs of all OpenRouter models with zero pricing.
func FetchFreeModels(ctx context.Context, apiKey string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", OpenRouterBaseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	c := &http.Client{Timeout: 15 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenRouter error %d: %s", resp.StatusCode, string(b))
	}

	var result struct {
		Data []openRouterModel `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var free []string
	for _, m := range result.Data {
		if m.Pricing.Prompt == "0" && m.Pricing.Completion == "0" {
			free = append(free, m.ID)
		}
	}
	sort.Strings(free)
	return free, nil
}
