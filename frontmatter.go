package devflow

import (
	"errors"
	"os"
	"strings"
)

// PlanMeta holds the parsed frontmatter of a docs/PLAN.md file.
type PlanMeta struct {
	Message string // required: commit message used when closing the loop
	Tag     string // optional: explicit version tag (e.g. "v0.1.0")
}

var (
	ErrFrontmatterMissing   = errors.New("plan frontmatter: file must start with a '---' line")
	ErrFrontmatterUnclosed  = errors.New("plan frontmatter: opening '---' has no matching closing '---'")
	ErrFrontmatterNoMessage = errors.New("plan frontmatter: missing required 'message:' field")
)

const FrontmatterFence = "---"

// ParseFrontmatter parses the leading YAML-style frontmatter block of content.
// Rules: must start at byte 0 with a "---" line, close at the next "---" line;
// between them "key: value" pairs (split on first ':'); unknown keys ignored;
// surrounding single/double quotes stripped from values. Requires 'message'.
func ParseFrontmatter(content string) (PlanMeta, error) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return PlanMeta{}, ErrFrontmatterMissing
	}

	firstLine := strings.TrimSpace(strings.TrimSuffix(lines[0], "\r"))
	if firstLine != FrontmatterFence {
		return PlanMeta{}, ErrFrontmatterMissing
	}

	var meta PlanMeta
	foundEnd := false
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSuffix(lines[i], "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == FrontmatterFence {
			foundEnd = true
			break
		}

		if trimmed == "" {
			continue
		}

		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, "\"'")

		switch key {
		case "message":
			meta.Message = value
		case "tag":
			meta.Tag = value
		}
	}

	if !foundEnd {
		return PlanMeta{}, ErrFrontmatterUnclosed
	}

	if meta.Message == "" {
		return PlanMeta{}, ErrFrontmatterNoMessage
	}

	return meta, nil
}

// ReadPlanMeta reads and validates the frontmatter of a plan file at path.
func ReadPlanMeta(path string) (PlanMeta, error) {
	return NewMarkDown(".", "", nil).InputPath(path, os.ReadFile).Frontmatter()
}

var ErrNoCloseLoopMessage = errors.New("no close-loop commit message: pass one on the CLI or add 'message:' to the plan frontmatter")

// ResolvePublishMessage picks the effective close-loop commit message and tag:
// an explicit CLI value wins; otherwise the plan frontmatter is used.
func ResolvePublishMessage(cliMessage, cliTag string, meta PlanMeta) (message, tag string, err error) {
	message = cliMessage
	if message == "" {
		message = meta.Message
	}
	if message == "" {
		return "", "", ErrNoCloseLoopMessage
	}
	tag = cliTag
	if tag == "" {
		tag = meta.Tag
	}
	return message, tag, nil
}
