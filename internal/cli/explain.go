package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lm-bridge/internal/db"
	"lm-bridge/internal/llm"
)

const explainSystemPrompt = `You are a senior engineer explaining code to a teammate. Be clear and concise.

Structure your response as:
1. **Purpose** — what this file/code does in 2-3 sentences
2. **Key parts** — the most important functions, types, or sections with a brief note on each
3. **How it fits** — how it connects to the rest of the system (if apparent)
4. **Gotchas** — anything non-obvious, tricky, or worth paying attention to

Skip boilerplate. Focus on what's actually interesting or important.`

func runExplain(args []string) {
	fs := flag.NewFlagSet("explain", flag.ExitOnError)
	think := fs.Bool("think", false, "enable chain-of-thought reasoning")
	fs.Parse(args)

	var content string
	var label string

	if fs.NArg() > 0 {
		// File path provided
		path := fs.Arg(0)
		b, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error reading file:", err)
			os.Exit(1)
		}
		content = string(b)
		label = filepath.Base(path)
	} else {
		// Try stdin
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			fmt.Fprintln(os.Stderr, "usage: lm-bridge explain <file>")
			fmt.Fprintln(os.Stderr, "       cat file.go | lm-bridge explain")
			os.Exit(1)
		}
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error reading stdin:", err)
			os.Exit(1)
		}
		content = string(b)
		label = "stdin"
	}

	content = strings.TrimSpace(content)
	if content == "" {
		fmt.Fprintln(os.Stderr, "error: file is empty")
		os.Exit(1)
	}

	// Trim large files
	const maxContent = 12000
	if len(content) > maxContent {
		content = content[:maxContent] + "\n\n[truncated — file too large]"
	}

	prompt := "Explain this file (" + label + "):\n\n```\n" + content + "\n```"

	store, _ := db.Open()
	client := newClient(store)

	if store != nil {
		store.SetActiveTask(os.Getpid(), "explain", label)
		defer store.ClearActiveTask()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	llm.StreamProgress(ctx, store)

	fmt.Fprintf(os.Stderr, "[lm] Explaining %s...\n", label)
	start := time.Now()

	resp, err := client.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: explainSystemPrompt},
			{Role: "user", Content: prompt},
		},
		Think: *think,
	})
	elapsed := time.Since(start)

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		if store != nil {
			store.SaveCall(db.Call{Mode: "explain", Provider: client.ProviderLabel(), Model: client.ModelLabel(), Task: label, Error: err.Error(), LatencyMs: elapsed.Milliseconds()})
		}
		os.Exit(1)
	}

	result := resp.Choices[0].Message.Content
	fmt.Println(result)

	if store != nil {
		store.SaveCall(db.Call{
			Mode:      "explain",
			Provider:  client.ProviderLabel(),
			Model:     client.ModelLabel(),
			Task:      label,
			Result:    trunc(result, 500),
			Tokens:    resp.Usage.TotalTokens,
			LatencyMs: elapsed.Milliseconds(),
		})
	}
}
