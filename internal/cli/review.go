package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"lm-bridge/internal/db"
	"lm-bridge/internal/llm"
)

const reviewSystemPrompt = `You are a senior code reviewer. Analyze the provided git diff and give a concise, actionable review.

Structure your response as:
1. **Summary** — what changed in 1-2 sentences
2. **Issues** — bugs, logic errors, security concerns (if any). Be specific: file and line.
3. **Suggestions** — style, naming, missing edge cases (if any)
4. **Verdict** — one of: ✅ Good to go / ⚠️ Minor issues / ❌ Needs fixes

Be direct and brief. Skip praise. If the diff is clean, say so.`

func runReview(args []string) {
	fs := flag.NewFlagSet("review", flag.ExitOnError)
	think := fs.Bool("think", false, "enable chain-of-thought reasoning")
	staged := fs.Bool("staged", false, "review only staged changes (default: all uncommitted)")
	fs.Parse(args)

	// Get diff — staged or working tree
	var diffBytes []byte
	var err error
	if *staged {
		diffBytes, err = exec.Command("git", "diff", "--cached").Output()
	} else {
		// staged + unstaged
		staged_out, _ := exec.Command("git", "diff", "--cached").Output()
		unstaged_out, _ := exec.Command("git", "diff").Output()
		diffBytes = append(staged_out, unstaged_out...)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error running git diff:", err)
		os.Exit(1)
	}

	// Also allow piped diff
	if len(strings.TrimSpace(string(diffBytes))) == 0 {
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			diffBytes, _ = io.ReadAll(os.Stdin)
		}
	}

	diff := strings.TrimSpace(string(diffBytes))
	if diff == "" {
		fmt.Fprintln(os.Stderr, "nothing to review: no changes found")
		os.Exit(1)
	}

	// Trim very large diffs
	const maxDiff = 12000
	if len(diff) > maxDiff {
		diff = diff[:maxDiff] + "\n\n[diff truncated — too large]"
	}

	prompt := "Review this diff:\n\n```diff\n" + diff + "\n```"

	store, _ := db.Open()
	client := llm.New("")

	if store != nil {
		store.SetActiveTask(os.Getpid(), "review", "code review")
		defer store.ClearActiveTask()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	llm.StreamProgress(ctx, store)

	fmt.Fprintln(os.Stderr, "[lm] Reviewing diff...")
	start := time.Now()

	resp, err := client.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: reviewSystemPrompt},
			{Role: "user", Content: prompt},
		},
		Think: *think,
	})
	elapsed := time.Since(start)

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		if store != nil {
			store.SaveCall(db.Call{Mode: "review", Task: "code review", Error: err.Error(), LatencyMs: elapsed.Milliseconds()})
		}
		os.Exit(1)
	}

	result := resp.Choices[0].Message.Content
	fmt.Println(result)

	if store != nil {
		store.SaveCall(db.Call{
			Mode:      "review",
			Task:      "code review",
			Result:    trunc(result, 500),
			Tokens:    resp.Usage.TotalTokens,
			LatencyMs: elapsed.Milliseconds(),
		})
	}
}
