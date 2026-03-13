# Symlink Skills to LLM Directories

Replace text-based reference line in LLM config files with filesystem symlinks. Each LLM gets a `skills/` symlink in its config directory pointing to the shared `~/skills/`.

> `~/skills/` is a shared directory. Other tools and the user can add their own skills there. `llmskill` only installs/updates its own skill subdirectories without touching anything else.

## Development Rules

- **Testing Runner:** Use `gotest` CLI (not `go test`).
- **Standard Library Only:** No external assertion libraries.
- **Thin Main / Fat Library:** Business logic in library.

## Current Behavior

`Sync()` writes `"Skills location: ~/tinywasm/skills/"` as text into `CLAUDE.md`/`GEMINI.md`.

## Target Behavior

`Sync()` creates symlinks:
```
~/.claude/skills/ → ~/skills/
~/.gemini/skills/ → ~/skills/
```

LLMs discover skills via their native directory structure (Agent Skills spec) without any text line in config files.

## Steps

### 1. Add `linkSkills()` method

In [llm_skill.go](llm_skill.go):

```go
// linkSkills creates a symlink from the LLM's skills dir to the shared skills location.
// Falls back to copying if symlink fails (Windows without Developer Mode).
func (l *LLM) linkSkills(llmDir, skillsSource string) (bool, error) {
    target := filepath.Join(llmDir, "skills")

    // Already correct symlink?
    if dest, err := os.Readlink(target); err == nil {
        if dest == skillsSource {
            return false, nil // already linked
        }
        os.Remove(target) // stale symlink
    }

    // Remove if regular dir exists (leftover from old copy approach)
    if info, err := os.Lstat(target); err == nil && info.IsDir() {
        os.RemoveAll(target)
    }

    // Try symlink
    if err := os.Symlink(skillsSource, target); err == nil {
        return true, nil
    }

    // Fallback: copy only our own skills (not the whole dir)
    return true, copyDir(skillsSource, target)
}
```

### 2. Update `InstallSkills()` path

Change destination from `~/tinywasm/skills/` to `~/skills/`:

```diff
-destRoot := filepath.Join(home, "tinywasm", "skills")
+destRoot := filepath.Join(home, "skills")
```

`InstallSkills()` only writes its own skill subdirectories. It MUST NOT delete or modify other entries in `~/skills/` that belong to the user or other tools.

### 3. Update `Sync()` method

Replace `ensureReferenceLine()` calls with `linkSkills()` calls:

```diff
 for _, llm := range installed {
-    configPath := filepath.Join(llm.Dir, llm.ConfigFile)
-    changed, err := l.ensureReferenceLine(configPath, master, force)
+    changed, err := l.linkSkills(llm.Dir, destRoot)
 }
```

### 4. Remove unused code

- Remove `ensureReferenceLine()`
- Remove `GetMasterContent()`
- Remove `ForceUpdate()` (or simplify to just reinstall + relink)
- Remove `DefaultSkillsReference` constant
- Remove `"strings"` import if no longer used

> **Note:** The user's global rule (`Skills location: ~/tinywasm/skills/`) should also be updated to `~/skills/`.

### 5. Add `copyDir()` helper

For Windows fallback when `os.Symlink()` fails:

```go
func copyDir(src, dst string) error {
    return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            return err
        }
        rel, _ := filepath.Rel(src, path)
        target := filepath.Join(dst, rel)
        if d.IsDir() {
            return os.MkdirAll(target, 0755)
        }
        data, err := os.ReadFile(path)
        if err != nil {
            return err
        }
        return os.WriteFile(target, data, 0644)
    })
}
```

### 6. Update tests

In [llm_skill_test.go](test/llm_skill_test.go):

- `TestLLM_Sync`: Verify symlink is created at `~/.claude/skills/` → `~/skills/`
- `TestLLM_Sync` idempotent: Second run skips (symlink already correct)
- `TestLLM_Sync_Fallback`: If symlink fails, verify files are copied instead
- Remove `TestLLM_GetMasterContent` (function removed)
- Remove config file text-line assertions

### 7. Update LLMSKILL.md

Replace section about "reference line" with symlink behavior.

## Cross-Platform Summary

| OS | Strategy | Why |
|---|---|---|
| Linux/macOS | `os.Symlink()` | Native support, no restrictions |
| Windows (Dev Mode) | `os.Symlink()` | Works with Developer Mode enabled |
| Windows (no Dev Mode) | `copyDir()` fallback | Symlink fails, copy files instead |

## Verification

```bash
gotest -run TestLLM
```
