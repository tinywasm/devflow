# GitHub Actions Runner Best Practices

This document outlines the technical strategy for selecting GitHub-hosted runners, focusing on efficiency, compatibility, and the recent introduction of `ubuntu-slim`.

## 1. The Alpine Limitation: `runs-on: alpine`
GitHub does not provide a native `runs-on: alpine` runner. GitHub-hosted runners are virtual machines based on **Ubuntu**, **Windows**, and **macOS**.

### The `musl` vs. `glibc` Gap
The primary technical reason for the lack of native Alpine support is the C standard library:
* **Ubuntu/Debian**: Uses `glibc` (GNU C Library).
* **Alpine**: Uses `musl libc`.

**Implications:**
- **Incompatibility**: Binaries compiled for `glibc` (like the Node.js runtime used by GitHub Actions) fail on Alpine with "file not found" errors unless a compatibility layer (`gcompat` or `libc6-compat`) is installed.
- **Dependency Issues**: Python "wheels" and Node.js native addons are almost exclusively built for `glibc`. Installing them on Alpine often forces a compilation from source, drastically increasing build times.

## 2. Why `ubuntu-slim`?
Launched in early 2026, `ubuntu-slim` is the recommended choice for lightweight automation tasks (linting, API orchestration, simple scripts).

### Rationale for Preference
1. **Native Compatibility**: Unlike Alpine, `ubuntu-slim` uses `glibc`. This ensures that standard actions (e.g., `actions/checkout`, `actions/setup-*`) work out of the box without hacks.
2. **Resource Optimization**: 
   - **Specs**: 1 vCPU / 5GB RAM (vs 2vCPU / 8GB in `ubuntu-latest`).
   - **Cost**: Optimized for lower resource consumption, making it ideal for non-intensive tasks.
3. **Execution Speed**: Being a first-class runner image, it initializes faster than spinning up a custom Docker container on top of a standard runner.
4. **Security**: Reduced footprint compared to the full `ubuntu-latest` image, decreasing the attack surface.

## 3. Implementation Strategies

### Case A: Lightweight Automation (Recommended)
Use `ubuntu-slim` for tasks that don't require heavy compilation.
```yaml
jobs:
  lint:
    runs-on: ubuntu-slim
    steps:
      - uses: actions/checkout@v4
      - run: npm run lint
```

### Case B: When Alpine is Mandatory (Production Parity)
If your production environment is Alpine and you need to test `musl`-specific behavior (DNS resolution, memory allocation), use a container on top of a standard runner.
```yaml
jobs:
  test-alpine:
    runs-on: ubuntu-latest
    container:
      image: alpine:latest
    steps:
      - name: Install Compatibility Layer
        run: apk add --no-cache gcompat
      - uses: actions/checkout@v4
      - run: ./test.sh
```

### Case C: Static Linking (Go/Rust)
For languages like Go or Rust, the best practice is to:
1. **Build** on `ubuntu-latest` (Faster compilation, better toolchain support).
2. **Link Statically** (Disable CGo in Go: `CGO_ENABLED=0`).
3. **Deploy** to a `scratch` or `alpine` image.

## 4. Summary Table (2026)

| Runner Label | vCPU | RAM | Typical Use Case |
| :--- | :--- | :--- | :--- |
| `ubuntu-latest` | 2 | 8 GB | Heavy builds, C++/Java compilation. |
| `ubuntu-slim` | 1 | 5 GB | **Linter, API calls, simple Python/JS scripts.** |
| `ubuntu-*-arm` | 2 | 8 GB | Native ARM64 builds. |

## Conclusion
While Alpine is excellent for production runtimes, `ubuntu-slim` provides the best balance of **lightweight footprint** and **native compatibility** for the CI/CD pipeline itself.
