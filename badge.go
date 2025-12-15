package devflow

import (
	"fmt"
	"strings"
)

// Badge represents a single badge with a label, value, and color.
// This is the primary struct used to define a badge's appearance and content.
//
// For example, to create a "Go version" badge, you might use:
//
//	b := Badge{
//	  Label: "Go",
//	  Value: "1.18",
//	  Color: "#007d9c",
//	}
type Badge struct {
	Label string // The text displayed on the left side of the badge.
	Value string // The text displayed on the right side of the badge.
	Color string // The background color for the value part of the badge (e.g., "#4c1" or "green").
}

// parseBadge parses a string in the format "label:value:color" into a Badge struct.
// This function is unexported and used internally by the package.
//
// It returns an error if the string format is invalid or if it encounters
// special command strings like "output_svgfile:" or "readmefile:".
func parseBadge(s string) (Badge, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return Badge{}, fmt.Errorf("invalid badge format: %s", s)
	}
	if parts[0] == "output_svgfile" || parts[0] == "readmefile" {
		return Badge{}, fmt.Errorf("special command")
	}
	if parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return Badge{}, fmt.Errorf("empty fields in badge: %s", s)
	}
	return Badge{Label: parts[0], Value: parts[1], Color: parts[2]}, nil
}
