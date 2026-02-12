# push

Git workflow automation: add, commit, tag, push.

## Usage

```bash
push 'commit message'              # Specific message (required)
push 'commit message' 'v1.0.0'     # Specific message and tag
```

## What it does

1. `git add .`
2. `git commit -m "message"`
3. Creates or uses tag (auto-increments patch version)
4. Intelligent `git push`: If rejection occurs (non-fast-forward), it automatically runs `git pull --rebase` and retries.
5. Sets upstream if needed

```mermaid
graph TD
    A[Start push] --> B[Git Add .]
    B --> C{Changes found?}
    C -- No --> D[Check Latest Tag]
    C -- Yes --> E[Git Commit]
    E --> F[Generate Next Tag]
    D --> F
    F --> G[Create Tag]
    G --> H{Tag Exists?}
    H -- Yes --> I[Increment Tag & Retry]
    I --> G
    H -- No --> J[Git Push]
    J --> K[Git Push Tag]
    K --> L[✅ Done]
```

## Output

```
✅ Tag: v1.0.1, ✅ Pushed ok
```
**Auto-recovered from remote changes:**
```
✅ Tag: v1.0.1, 🔄 Pulled remote changes, ✅ Pushed ok
```

**Tag already exists:**
```
Tag warning: tag v1.0.1 already exists, ✅ Pushed ok
```

## Tag auto-generation

- Finds latest tag (e.g., `v1.0.5`)
- Increments patch: `v1.0.6`
- If no tags exist: `v0.0.1`

## Exit codes

- `0` - Success
- `1` - Git operation failed

## Note: Special characters

Use **single quotes** for messages with backticks or `$`:
```bash
push 'feat: Add `afterLine` parameter'
```
