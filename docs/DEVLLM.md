# devllm - LLM Configuration Sync

Synchronizes LLM configuration files from a master template.

## Installation

```bash
go install github.com/tinywasm/devflow/cmd/devllm@latest
```

## Usage

### Sync all installed LLMs
```bash
devllm
```

### Sync specific LLM
```bash
devllm -l claude
devllm --llm gemini
```

### Force overwrite (creates backup)
```bash
devllm -f
devllm --force
```

### Show help
```bash
devllm -h
devllm --help
```

## How it works

1. **Detects installed LLMs** by checking directories:
   - `~/.claude/CLAUDE.md`
   - `~/.gemini/GEMINI.md`

2. **Smart merge** using HTML section markers:
   - Updates standard sections from master template
   - Preserves `USER_CUSTOM` section with your personal configs
   - Skips files that are already up-to-date

3. **Master template**: Embedded in devflow as `DEFAULT_GLOBAL_LLM_SKILLS.md`

## Configuration

The master template is divided into sections:

- `CORE_PRINCIPLES`: Development principles (SRP, Framework-less, CSS-First, etc.)
- `TESTING`: Testing guidelines (using gotest)
- `PROTOCOLS`: Language and workflow protocols
- `WASM`: WebAssembly specific rules
- `USER_CUSTOM`: Your personal customizations (preserved during sync)

When you run `devllm`, it updates the standard sections while keeping your custom content intact.

### Adding Custom Rules

You can add LLM-specific custom rules in the `USER_CUSTOM` section:

```markdown
<!-- START_SECTION:USER_CUSTOM -->
- **My Custom Rule:** Always use tabs instead of spaces
- **Project-Specific:** Never use external dependencies for X
<!-- END_SECTION:USER_CUSTOM -->
```

This section will be preserved across all syncs.

## Library Usage

```go
import "github.com/tinywasm/devflow"

llm := devflow.NewLLM()

// Sync all installed LLMs
summary, err := llm.Sync("", false)
if err != nil {
    log.Fatal(err)
}
fmt.Println(summary)

// Sync specific LLM
summary, err = llm.Sync("claude", false)

// Force overwrite with backup
summary, err = llm.Sync("", true)

// Enable logging
llm.SetLog(log.Println)
```

### Advanced Usage

```go
// Detect which LLMs are installed
llm := devflow.NewLLM()
installed := llm.DetectInstalledLLMs()
for _, config := range installed {
    fmt.Printf("Found: %s at %s\n", config.Name, config.Dir)
}

// Get master template content
masterContent, err := llm.GetMasterContent()
if err != nil {
    log.Fatal(err)
}
fmt.Println(masterContent)

// Get list of supported LLMs (installed or not)
supported := llm.GetSupportedLLMs()
for _, config := range supported {
    fmt.Printf("Supported: %s -> %s/%s\n",
        config.Name, config.Dir, config.ConfigFile)
}
```

## Examples

### First time setup
Creates config files with master template:
```bash
$ devllm
✅ Updated: [claude gemini]
```

### Subsequent runs (no changes)
```bash
$ devllm
⏭️  Skipped (up-to-date): [claude gemini]
```

### After master template update
When you update devflow and the master template changes:
```bash
$ devllm
✅ Updated: [claude gemini]
```

### Sync only Claude
```bash
$ devllm -l claude
✅ Updated: [claude]
```

### Force overwrite with backup
```bash
$ devllm -f
✅ Updated: [claude gemini]
```

This creates backups:
- `~/.claude/CLAUDE.md.bak`
- `~/.gemini/GEMINI.md.bak`

### Migration from legacy format
If your config files don't have section markers (created before devllm):
```bash
$ devllm
✅ Updated: [claude gemini]
```

Automatic backups are created:
- `~/.claude/CLAUDE.md.bak` (your old content)
- `~/.gemini/GEMINI.md.bak` (your old content)

## Behavior Details

### Smart Sync (default)

1. **New file**: Creates file with master template
2. **Identical content**: Skips (no changes needed)
3. **Section changes**: Updates only changed sections
4. **Legacy format**: Creates backup and converts to sectioned format
5. **USER_CUSTOM section**: Always preserved

### Force Mode (-f flag)

1. Creates backup (.bak) of existing file
2. Completely overwrites with master template
3. Useful for:
   - Resetting to defaults
   - Fixing corrupted configs
   - Testing new template versions

## Integration

### With gopush

You can integrate `devllm` into your workflow:

```bash
# Sync LLM configs before pushing
devllm && gopush "feat: new feature"
```

### In Go Projects

```go
package main

import (
    "log"
    "github.com/tinywasm/devflow"
)

func main() {
    // Ensure LLM configs are up-to-date
    llm := devflow.NewLLM()
    if _, err := llm.Sync("", false); err != nil {
        log.Printf("Warning: LLM sync failed: %v", err)
    }

    // Continue with your main logic
}
```

## Troubleshooting

### "No LLMs detected"

Neither `~/.claude` nor `~/.gemini` directories exist. Install Claude Code or Gemini AI Studio first.

### "LLM 'xyz' not found or not installed"

The specified LLM doesn't have a config directory. Check available LLMs:
```bash
ls -la ~ | grep -E '\.(claude|gemini)'
```

### Section not updating

If a section isn't updating as expected:
1. Check that section markers are intact in your file
2. Use `--force` to reset:
   ```bash
   devllm -f
   ```
3. Check backup file (.bak) if needed

### Backup files accumulating

Backup files (`.bak`) are only created when:
- Force mode is used
- Legacy format is converted

To clean them up:
```bash
rm ~/.claude/CLAUDE.md.bak ~/.gemini/GEMINI.md.bak
```

## Supported LLMs

Currently supported:
- **Claude Code** (`~/.claude/CLAUDE.md`)
- **Gemini** (`~/.gemini/GEMINI.md`)

Want to add more? Open an issue or PR on GitHub.

## See Also

- [gotest](GOTEST.md) - Run tests with coverage and badges
- [push](PUSH.md) - Git workflow automation
- [gopush](GOPUSH.md) - Complete workflow: test + push + update
