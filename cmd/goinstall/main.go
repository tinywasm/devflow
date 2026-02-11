package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/tinywasm/devflow"
)

func main() {
	git, err := devflow.NewGit()
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	goHandler, err := devflow.NewGo(git)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	// standalone install defaults to "dev" or current version
	summary, err := goHandler.Install("")
	if err != nil {
		fmt.Println("Install failed:", err)
		os.Exit(1)
	}

	if summary != "" {
		// Postprocess summary to match previous output style (one per line)
		parts := strings.Split(summary, ", ")
		for _, p := range parts {
			fmt.Println(p)
		}
	}
}
