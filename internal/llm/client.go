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
	"strings"
	"time"
)

const DefaultBaseURL = "http://localhost:1234/v1"

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func New(baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		baseURL:    baseURL,
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
		req.Model = "local-model"
	}

	// Pre-flight: check if LM Studio is reachable and idle before sending a heavy request.
	// A fast /v1/models call reveals whether the server responds at all.
	// If it hangs too — model is mid-generation, abort immediately to avoid disrupting it.
	if err := c.checkIdle(ctx); err != nil {
		return nil, err
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

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("LM Studio unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("LM Studio error %d: %s", resp.StatusCode, string(b))
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
	req.Stream = true
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("LM Studio unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("LM Studio error %d: %s", resp.StatusCode, string(b))
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

// ModelStatus returns whether LM Studio is reachable and the loaded model name.
func (c *Client) ModelStatus(ctx context.Context) (online bool, modelName string, err error) {
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
