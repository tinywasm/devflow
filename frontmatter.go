package devflow

import (
	"errors"
	"os"

	"github.com/tinywasm/markdown"
)

// PlanMeta holds the parsed frontmatter of a docs/PLAN.md file.
type PlanMeta struct {
	Message string // required: commit message used when closing the loop
	Tag     string // optional: explicit version tag (e.g. "v0.1.0")
}

// frontmatterHelp is appended to every frontmatter error so the fix is obvious from the
// terminal alone — nobody should need to open the docs to unblock a dispatch.
const frontmatterHelp = `

docs/PLAN.md must OPEN with a frontmatter block — the very first line is '---':

    ---
    message: "feat: what this plan implements"
    tag: v0.2.0
    ---

    # Plan — ...

  message  REQUIRED. The commit message used when the loop is closed.
  tag      optional. Explicit version (e.g. v0.2.0); omitted = auto-bump.`

var (
	ErrFrontmatterMissing   = errors.New("plan frontmatter: file must start with a '---' line" + frontmatterHelp)
	ErrFrontmatterUnclosed  = errors.New("plan frontmatter: opening '---' has no matching closing '---'" + frontmatterHelp)
	ErrFrontmatterNoMessage = errors.New("plan frontmatter: missing required 'message:' field" + frontmatterHelp)
)

// wrapStructuralErr translates markdown's structural frontmatter errors into
// devflow's own (which carry frontmatterHelp); anything else passes through.
func wrapStructuralErr(err error) error {
	switch {
	case errors.Is(err, markdown.ErrFrontmatterMissing):
		return ErrFrontmatterMissing
	case errors.Is(err, markdown.ErrFrontmatterUnclosed):
		return ErrFrontmatterUnclosed
	default:
		return err
	}
}

// metaFromMap maps generic frontmatter key/values to PlanMeta, enforcing the
// devflow-specific rule that 'message' is required.
func metaFromMap(kv map[string]string) (PlanMeta, error) {
	message := kv["message"]
	if message == "" {
		return PlanMeta{}, ErrFrontmatterNoMessage
	}
	return PlanMeta{Message: message, Tag: kv["tag"]}, nil
}

// ParseFrontmatter parses the leading frontmatter block of content and maps it
// to PlanMeta, requiring 'message'. Structural parsing is delegated to
// tinywasm/markdown; devflow only owns the "which keys are required" rule.
func ParseFrontmatter(content string) (PlanMeta, error) {
	kv, err := markdown.ParseFrontmatter(content)
	if err != nil {
		return PlanMeta{}, wrapStructuralErr(err)
	}
	return metaFromMap(kv)
}

// ReadPlanMeta reads and validates the frontmatter of a plan file at path.
func ReadPlanMeta(path string) (PlanMeta, error) {
	kv, err := markdown.New(".", "", nil).InputPath(path, os.ReadFile).Frontmatter()
	if err != nil {
		return PlanMeta{}, wrapStructuralErr(err)
	}
	return metaFromMap(kv)
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
