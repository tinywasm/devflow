package devflow_test

import (
	"github.com/tinywasm/devflow"
	"testing"
)

type fakeObjector struct {
	action devflow.PublishAction
	reason string
}

func (f fakeObjector) ObjectsToPublish(ctx devflow.PublishContext) (devflow.PublishAction, string) {
	return f.action, f.reason
}

func TestResolvePublishAction(t *testing.T) {
	tests := []struct {
		name       string
		objectors  []devflow.PublishObjector
		wantAction devflow.PublishAction
		wantReason string
	}{
		{
			name:       "none",
			objectors:  nil,
			wantAction: devflow.ActionNone,
			wantReason: "",
		},
		{
			name: "single skip",
			objectors: []devflow.PublishObjector{
				fakeObjector{devflow.ActionSkip, "skip it"},
			},
			wantAction: devflow.ActionSkip,
			wantReason: "skip it",
		},
		{
			name: "skip beats deps",
			objectors: []devflow.PublishObjector{
				fakeObjector{devflow.ActionDepsOnly, "deps"},
				fakeObjector{devflow.ActionSkip, "skip"},
			},
			wantAction: devflow.ActionSkip,
			wantReason: "skip",
		},
		{
			name: "deps beats none",
			objectors: []devflow.PublishObjector{
				fakeObjector{devflow.ActionNone, ""},
				fakeObjector{devflow.ActionDepsOnly, "deps"},
			},
			wantAction: devflow.ActionDepsOnly,
			wantReason: "deps",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action, reason := devflow.ResolvePublishAction(tt.objectors, devflow.PublishContext{})
			if action != tt.wantAction {
				t.Errorf("action = %v, want %v", action, tt.wantAction)
			}
			if reason != tt.wantReason {
				t.Errorf("reason = %q, want %q", reason, tt.wantReason)
			}
		})
	}
}
