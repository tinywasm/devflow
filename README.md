# GitGo

GitGo is a minimalist Go library and CLI toolset designed to automate Git workflows and Go module updates. It adheres to a zero-dependency policy, using only the Go standard library.

## Features

- **Automated Workflows:** Simplify Git push, tag, and release processes.
- **Go Module Management:** Automate module versioning and dependency updates across projects.
- **Handlers:** Use `Git` and `Go` handlers to build custom workflows.

## Installation

```bash
go get github.com/cdvelop/gitgo
```

To install the CLI tools:

```bash
go install github.com/cdvelop/gitgo/cmd/push@latest
go install github.com/cdvelop/gitgo/cmd/gopu@latest
```

## Usage

### CLI

**Push:**

```bash
push "commit message" [tag]
```

**Go Project Update (gopu):**

```bash
gopu "commit message" [tag]
```

### Library

GitGo provides `Git` and `Go` handlers for programmatic usage.

```go
package main

import (
	"fmt"
	"log"

	"github.com/cdvelop/gitgo"
)

func main() {
	// Initialize Git handler
	git := gitgo.NewGit()

	// Execute Git Push Workflow
	summary, err := git.Push("feat: new feature", "v1.0.0")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(summary)

	// Initialize Go handler (depends on Git handler)
	goHandler := gitgo.NewGo(git)

	// Execute Go Project Update Workflow
	// (Verifies mod, runs tests, pushes, updates dependents)
	summary, err = goHandler.Push("fix: bug", "", false, false, "..")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(summary)
}
```

## Workflows

### Git Push

1.  `git add .`
2.  `git commit -m "message"`
3.  Generate or use provided tag.
4.  `git push` and `git push origin <tag>`.

### Go Update (GoPU)

1.  `go mod verify`
2.  `go test ./...` (optional skip)
3.  `go test -race ./...` (optional skip)
4.  Execute **Git Push** workflow.
5.  Search for dependent modules in the specified path (default `..`).
6.  Update dependents (`go get -u module@version` && `go mod tidy`).

## License

MIT
