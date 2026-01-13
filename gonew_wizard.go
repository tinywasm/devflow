package devflow

import (
	"path/filepath"

	"github.com/tinywasm/context"
	"github.com/tinywasm/wizard"
)

// Name returns module identifier
func (gn *GoNew) Name() string {
	return "GoNew"
}

// GetSteps returns the sequence of steps to create a new Go project
func (gn *GoNew) GetSteps() []*wizard.Step {
	return []*wizard.Step{
		// Step 1: Project Name
		{
			LabelText: "Project Name",
			DefaultFn: func(ctx *context.Context) string { return "" },
			OnInputFn: func(in string, ctx *context.Context) (bool, error) {
				if in == "" {
					return false, nil // Wait for valid input
				}
				err := ctx.Set("project_name", in)
				return true, err
			},
		},

		// Step 2: Project Location
		{
			LabelText: "Project Location",
			DefaultFn: func(ctx *context.Context) string {
				abs, _ := filepath.Abs(".")
				return filepath.Join(abs, ctx.Value("project_name"))
			},
			OnInputFn: func(in string, ctx *context.Context) (bool, error) {
				if in == "" {
					return false, nil
				}
				err := ctx.Set("project_dir", in)
				return true, err
			},
		},

		// Step 3: Create Execution
		{
			LabelText: "Create Project",
			DefaultFn: func(ctx *context.Context) string { return "Press Enter to Create" },
			OnInputFn: func(in string, ctx *context.Context) (bool, error) {
				name := ctx.Value("project_name")
				dir := ctx.Value("project_dir")

				opts := NewProjectOptions{
					Name:        name,
					Directory:   dir,
					Description: "Created via TinyWasm Wizard",
					Visibility:  "public",
					License:     "MIT",
				}

				gn.log("[...", "Creating project")
				summary, err := gn.Create(opts)
				if err != nil {
					gn.log("...]", "Error: "+err.Error())
					return false, err
				}

				gn.log("...]", summary)
				err = ctx.Set("creation_summary", summary)
				return true, err
			},
		},
	}
}
