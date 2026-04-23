package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lm-bridge/internal/db"
	"lm-bridge/internal/llm"
)

const maxFileBytes = 12 * 1024 // 12 KB cap per file

var agentTools = []llm.Tool{
	{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "read_file",
			Description: "Read the contents of a file. Returns truncated content if file exceeds 12 KB.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "File path relative to the working directory",
					},
				},
				"required": []string{"path"},
			},
		},
	},
	{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "list_dir",
			Description: "List files and subdirectories under a path (recursive, relative paths).",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Directory path relative to the working directory (use '.' for root)",
					},
				},
				"required": []string{"path"},
			},
		},
	},
}

func runAgent(args []string) {
	fs := flag.NewFlagSet("agent", flag.ExitOnError)
	think := fs.Bool("think", false, "enable chain-of-thought reasoning")
	dir := fs.String("dir", ".", "working directory — file access is sandboxed to this path")
	fs.Parse(args)

	task := strings.Join(fs.Args(), " ")
	if strings.TrimSpace(task) == "" {
		fmt.Fprintln(os.Stderr, "error: no task provided")
		printUsage()
		os.Exit(1)
	}

	workDir, err := filepath.Abs(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error resolving dir:", err)
		os.Exit(1)
	}

	store, _ := db.Open()
	client := newClient(store)

	if store != nil {
		store.SetActiveTask(os.Getpid(), "agent", trunc(task, 120))
		defer store.ClearActiveTask()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	llm.StreamProgress(ctx, store)

	messages := []llm.Message{
		{
			Role: "system",
			Content: "You are a helpful assistant that completes coding tasks. " +
				"Use the provided tools to read files and explore the directory. " +
				"Return a concise, structured result. Do not include unnecessary commentary.",
		},
		{Role: "user", Content: task},
	}

	start := time.Now()
	totalTokens := 0
	var finalResult string

	for i := 0; i < 15; i++ {
		resp, err := client.Chat(ctx, llm.ChatRequest{
			Messages: messages,
			Tools:    agentTools,
			Think:    *think,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		totalTokens += resp.Usage.TotalTokens

		msg := resp.Choices[0].Message
		messages = append(messages, msg)

		if len(msg.ToolCalls) == 0 {
			finalResult = msg.Content
			break
		}

		for _, tc := range msg.ToolCalls {
			result := execTool(tc, workDir)
			messages = append(messages, llm.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	elapsed := time.Since(start)
	fmt.Println(finalResult)

	if store != nil {
		store.SaveCall(db.Call{
			Mode:      "agent",
			Provider:  client.ProviderLabel(),
			Model:     client.ModelLabel(),
			Task:      trunc(task, 200),
			Result:    trunc(finalResult, 500),
			Tokens:    totalTokens,
			LatencyMs: elapsed.Milliseconds(),
		})
	}
}

func execTool(tc llm.ToolCall, workDir string) string {
	var params map[string]string
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
		return "error: invalid tool arguments"
	}

	switch tc.Function.Name {
	case "read_file":
		abs := safePath(params["path"], workDir)
		data, err := os.ReadFile(abs)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		if len(data) > maxFileBytes {
			data = append(data[:maxFileBytes], []byte("\n\n[truncated — file exceeds 12 KB]")...)
		}
		return string(data)

	case "list_dir":
		abs := safePath(params["path"], workDir)
		var lines []string
		filepath.WalkDir(abs, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(workDir, p)
			if d.IsDir() {
				lines = append(lines, rel+"/")
			} else {
				lines = append(lines, rel)
			}
			return nil
		})
		if len(lines) == 0 {
			return "(empty)"
		}
		return strings.Join(lines, "\n")
	}
	return "error: unknown tool " + tc.Function.Name
}

// safePath resolves a relative path inside workDir, preventing traversal.
func safePath(rel, workDir string) string {
	abs := filepath.Join(workDir, rel)
	if !strings.HasPrefix(abs, workDir+string(filepath.Separator)) && abs != workDir {
		return workDir
	}
	return abs
}
