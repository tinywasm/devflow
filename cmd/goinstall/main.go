package main

import (
	"fmt"
	"os"

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
	if err := goHandler.Install(""); err != nil {
		fmt.Println("Install failed:", err)
		os.Exit(1)
	}
}
