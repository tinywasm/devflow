package devflow

type PublishAction int

const (
	ActionNone     PublishAction = iota // no objection: full publication (tag + cascade)
	ActionDepsOnly                      // commit go.mod/go.sum, push without tag, no cascade
	ActionSkip                          // do not touch the repo at all
)

const (
	ObjectionCodejobSession = "codejob session active"
	ObjectionOtherReplaces  = "other replaces exist"
	ObjectionPlanPending    = "docs/PLAN.md pending"
	ObjectionDirtyTree      = "dirty tree"
)

type PublishContext struct {
	RepoDir     string   // dependent repo being evaluated
	ModulePaths []string // upstream module paths being updated in this wave
}

type PublishObjector interface {
	ObjectsToPublish(ctx PublishContext) (PublishAction, string) // action + readable reason
}

// ResolvePublishAction returns the strongest action any objector requires
// (Skip > DepsOnly > None) and the reason of the objector that set it.
func ResolvePublishAction(objectors []PublishObjector, ctx PublishContext) (PublishAction, string) {
	action, reason := ActionNone, ""
	for _, o := range objectors {
		if o == nil {
			continue
		}
		a, r := o.ObjectsToPublish(ctx)
		if a > action { // ActionSkip(2) > ActionDepsOnly(1) > ActionNone(0)
			action, reason = a, r
		}
	}
	return action, reason
}

var publishAllowedDirtyFiles = []string{"go.mod", "go.sum"}
