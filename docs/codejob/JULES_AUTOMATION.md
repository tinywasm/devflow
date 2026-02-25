# Jules AI: GitHub Integration and Automated Corrections

> Jules is one implementation of `CodeJobDriver`. For the generic orchestration pattern,
> chain-of-responsibility design, and CLI/library usage, see [CODEJOB.md](../CODEJOB.md).

---

Jules is an AI agent designed for code modernization and technical project management. It operates directly on your codebase to automate refactoring, bug fixes, and documentation updates.

## 1. Connecting Jules to GitHub

To enable Jules to interact with your projects:
*   **App Installation**: Integrate Jules as a GitHub App or via a Personal Access Token (PAT) with read/write permissions.
*   **Indexing**: Jules performs a semantic scan of your repository to understand its architecture, dependencies, and business logic.

## 2. Project Coordination Workflow

Jules operates through **Tasks**. Instead of manual file uploads, you direct Jules to existing code:
*   **Task Definition**: Create a ticket/issue in the Jules interface (e.g., "Migrate React from 16 to 18").
*   **Impact Analysis**: Jules generates an execution plan showing affected files before making changes.
*   **Branch Creation**: Changes are made in a dedicated GitHub branch for isolated development.

## 3. Automated Correction Cycles

Jules uses a feedback-driven cycle for autonomous corrections:
*   **PR-based Fixes**: If CI tests fail on a Jules-generated PR, you can prompt: *"Review CI logs and fix the memory leak."*
*   **Auto-fix Agents**: Jules can listen to static analysis tools (e.g., SonarQube) and automatically generate PRs to address security or style alerts.

### Technical Note: Context Awareness
Unlike simple search-and-replace tools, Jules understands the software's call graph. Changing a function in `file_A.py` will automatically trigger updates in all call sites across the repository.

## 4. Jules REST API

The Jules API allows for full automation of task creation and project coordination.

### Authentication
Use a **Jules API Key** (managed at `jules.google.com` > Settings > API Keys) via the `X-Goog-Api-Key` header.
*   **Base URL**: `https://jules.googleapis.com/v1alpha/`

### Automation Workflow
1.  **Identify Source**: Get the repository ID.
    ```bash
    curl 'https://jules.googleapis.com/v1alpha/sources' -H 'X-Goog-Api-Key: YOUR_API_KEY'
    ```
2.  **Create Session**: Send a natural language prompt.
    ```bash
    curl 'https://jules.googleapis.com/v1alpha/sessions' \
      -X POST \
      -H "Content-Type: application/json" \
      -H 'X-Goog-Api-Key: YOUR_API_KEY' \
      -d '{
        "title": "Fix Login Vulnerability",
        "prompt": "Fix SQL injection in auth.js and add unit tests.",
        "sourceContext": { "source": "sources/github/user/repo", "githubRepoContext": { "startingBranch": "main" } },
        "automationMode": "AUTO_CREATE_PR"
      }'
    ```

## 5. CI/CD Integration (GitHub Actions)

You can trigger Jules automatically upon pushing to `main` by validating an instructions file (e.g., `docs/ISSUE_JULES.md`).

### Strategy: Lightweight Execution
We use the **`ubuntu-slim`** runner for maximum efficiency and native compatibility.
*   **[Technical Justification for `ubuntu-slim`](RUNNER_BEST_PRACTICES.md)**

### Implementation Details
- **[Workflow Diagram](diagrams/JULES_WORKFLOW.md)**
- **[GitHub Actions Example (.yml)](examples/JULES_CI.yml)**

## Key Technical Best Practices

1.  **Referential Prompting**: Point Jules to a file containing instructions rather than sending raw text via API to avoid character limits and preserve formatting.
2.  **Runner Selection**: Use `ubuntu-slim` (1 vCPU / 5GB RAM) for lightweight API orchestration tasks.
3.  **Secrets Management**: Store your `JULES_API_KEY` in GitHub Encrypted Secrets.

---
*For more information on optimizing GitHub Runners, see [RUNNER_BEST_PRACTICES.md](RUNNER_BEST_PRACTICES.md).*
