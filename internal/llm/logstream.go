package llm

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"lm-bridge/internal/db"
)

// latestServerLog returns the most recently modified log file in
// ~/.lmstudio/server-logs/YYYY-MM/ for today's month.
func latestServerLog() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	now := time.Now()
	dir := filepath.Join(home, ".lmstudio", "server-logs", now.Format("2006-01"))
	entries, err := filepath.Glob(filepath.Join(dir, "*.log"))
	if err != nil || len(entries) == 0 {
		return "", fmt.Errorf("no server logs found in %s", dir)
	}
	// Sort by modification time, pick newest.
	sort.Slice(entries, func(i, j int) bool {
		fi, _ := os.Stat(entries[i])
		fj, _ := os.Stat(entries[j])
		if fi == nil || fj == nil {
			return false
		}
		return fi.ModTime().After(fj.ModTime())
	})
	return entries[0], nil
}

// StreamProgress tails the latest LM Studio server log file and prints
// prompt-processing progress to stderr, updating the DB for the dashboard.
func StreamProgress(ctx context.Context, store *db.Store) {
	logFile, err := latestServerLog()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[lm] logstream: %v\n", err)
		return
	}

	cmd := exec.CommandContext(ctx, "tail", "-n", "0", "-f", logFile)
	out, err := cmd.StdoutPipe()
	cmd.Stderr = nil
	if err != nil {
		return
	}
	if err := cmd.Start(); err != nil {
		return
	}

	go func() {
		defer cmd.Wait()
		scanner := bufio.NewScanner(out)
		lastPct := -1.0
		for scanner.Scan() {
			line := scanner.Text()
			// Format: [2026-04-22 12:38:06][INFO][model] Prompt processing progress: 69.2%
			if !strings.Contains(line, "Prompt processing progress:") {
				continue
			}
			pct, ok := parsePercent(line)
			if !ok {
				continue
			}
			if pct == lastPct {
				continue
			}
			lastPct = pct
			fmt.Fprintf(os.Stderr, "[lm] Prompt processing progress: %.1f%%\n", pct)

			if store != nil {
				store.UpdateTaskProgress(pct)
			}
		}
	}()
}

// parsePercent extracts a percentage from a line ending in "progress: 69.2%"
func parsePercent(line string) (float64, bool) {
	idx := strings.LastIndex(line, ": ")
	if idx < 0 {
		return 0, false
	}
	s := strings.TrimSuffix(strings.TrimSpace(line[idx+2:]), "%")
	pct, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return pct, true
}
