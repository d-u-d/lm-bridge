package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"lm-bridge/internal/db"
	"lm-bridge/internal/llm"
)


func runQuery(args []string) {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	think  := fs.Bool("think", false, "enable chain-of-thought reasoning")
	stream := fs.Bool("stream", false, "stream tokens to stdout in real-time (enables loop detection)")
	fs.Parse(args)

	prompt := strings.Join(fs.Args(), " ")

	// accept piped stdin
	if prompt == "" {
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			b, err := io.ReadAll(os.Stdin)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error reading stdin:", err)
				os.Exit(1)
			}
			prompt = string(b)
		}
	}

	if strings.TrimSpace(prompt) == "" {
		fmt.Fprintln(os.Stderr, "error: no prompt provided")
		printUsage()
		os.Exit(1)
	}

	store, _ := db.Open()
	client := newClient(store)

	if store != nil {
		store.SetActiveTask(os.Getpid(), "query", trunc(prompt, 120))
		defer store.ClearActiveTask()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	llm.StreamProgress(ctx, store)

	start := time.Now()

	if *stream {
		err := client.ChatStream(ctx, llm.ChatRequest{
			Messages: []llm.Message{{Role: "user", Content: prompt}},
			Think:    *think,
		}, os.Stdout)
		elapsed := time.Since(start)
		fmt.Println() // завершающий перевод строки

		errMsg := ""
		if err != nil {
			errMsg = err.Error()
			fmt.Fprintln(os.Stderr, "error:", err)
		}
		if store != nil {
			store.SaveCall(db.Call{
				Mode:      "query",
				Provider:  client.ProviderLabel(),
				Model:     client.ModelLabel(),
				Task:      trunc(prompt, 200),
				Error:     errMsg,
				LatencyMs: elapsed.Milliseconds(),
			})
		}
		if err != nil {
			os.Exit(1)
		}
		return
	}

	// non-stream mode
	resp, err := client.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{{Role: "user", Content: prompt}},
		Think:    *think,
	})
	elapsed := time.Since(start)

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		if store != nil {
			store.SaveCall(db.Call{
				Mode:      "query",
				Provider:  client.ProviderLabel(),
				Model:     client.ModelLabel(),
				Task:      trunc(prompt, 200),
				Error:     err.Error(),
				LatencyMs: elapsed.Milliseconds(),
			})
		}
		os.Exit(1)
	}

	result := resp.Choices[0].Message.Content
	fmt.Println(result)

	if store != nil {
		store.SaveCall(db.Call{
			Mode:      "query",
			Provider:  client.ProviderLabel(),
			Model:     client.ModelLabel(),
			Task:      trunc(prompt, 200),
			Result:    trunc(result, 500),
			Tokens:    resp.Usage.TotalTokens,
			LatencyMs: elapsed.Milliseconds(),
		})
	}
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
