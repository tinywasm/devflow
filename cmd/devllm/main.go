package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/tinywasm/devflow"
)

func main() {
	fs := flag.NewFlagSet("devllm", flag.ExitOnError)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `devllm - Sync LLM configuration files

Usage:
    devllm              Sync all installed LLMs
    devllm -l claude    Sync only Claude
    devllm -f           Force overwrite (with backup)
    devllm -h           Show this help

Detects LLMs by directory:
    ~/.claude/CLAUDE.md
    ~/.gemini/GEMINI.md

Master template: devflow/DEFAULT_GLOBAL_LLM_SKILLS.md
`)
	}

	llmFlag := fs.String("l", "", "Sync specific LLM (claude, gemini)")
	fs.StringVar(llmFlag, "llm", "", "Sync specific LLM (alias)")
	forceFlag := fs.Bool("f", false, "Force overwrite with backup")
	fs.BoolVar(forceFlag, "force", false, "Force overwrite with backup (alias)")
	helpFlag := fs.Bool("h", false, "Show help")
	fs.BoolVar(helpFlag, "help", false, "Show help")

	fs.Parse(os.Args[1:])

	if *helpFlag {
		fs.Usage()
		os.Exit(0)
	}

	llm := devflow.NewLLM()

	summary, err := llm.Sync(*llmFlag, *forceFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if summary != "" {
		fmt.Println(summary)
	}
}
