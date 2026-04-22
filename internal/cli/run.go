package cli

import (
	"fmt"
	"os"
)

func Run(args []string) {
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}
	switch args[0] {
	case "query":
		runQuery(args[1:])
	case "agent":
		runAgent(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprint(os.Stderr, `lm-bridge — local LLM helper for Claude Code

Usage:
  lm-bridge                              open dashboard
  lm-bridge query [--think] [--stream] <prompt>   single prompt (stdin also accepted)
  lm-bridge agent [--think] [--dir DIR] <task>
                                         agentic loop with file tools

`)
}
