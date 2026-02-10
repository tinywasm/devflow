package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	cmdDir := filepath.Join(".", "cmd")
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		fmt.Println("Error: no cmd/ directory found")
		os.Exit(1)
	}

	var commands []string
	for _, entry := range entries {
		if entry.IsDir() {
			commands = append(commands, entry.Name())
		}
	}

	if len(commands) == 0 {
		fmt.Println("Error: no commands found in cmd/")
		os.Exit(1)
	}

	for _, cmd := range commands {
		pkg := "./cmd/" + cmd
		install := exec.Command("go", "install", pkg)
		install.Stdout = os.Stdout
		install.Stderr = os.Stderr
		if err := install.Run(); err != nil {
			fmt.Printf("❌ %s\n", cmd)
			os.Exit(1)
		}
		fmt.Printf("✅ %s\n", cmd)
	}
}
