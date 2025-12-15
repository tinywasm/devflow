package devflow

import (
	"fmt"
	"strings"
)

// GenerateSVG creates an SVG image from the configured badges.
//
// It returns the SVG content as a byte slice, the number of badges included,
// and an error if the generation fails. This method is typically called
// by BuildBadges, but it can be used directly if you only need the SVG data.
//
// Example of a generated SVG for two badges ("License:MIT:blue" and "Go:1.22:blue"):
//
// <?xml version="1.0" encoding="UTF-8"?>
// <svg xmlns="http://www.w3.org/2000/svg" width="168" height="20" viewBox="0 0 168 20">
//
//	<!-- Badge: License -->
//	<g transform="translate(0, 0)">
//	  <rect x="0" y="0" width="58" height="20" fill="#6c757d"/>
//	  <rect x="58" y="0" width="46" height="20" fill="blue"/>
//	  <text x="29" y="14" text-anchor="middle" font-family="sans-serif" font-size="11" fill="white">License</text>
//	  <text x="81" y="14" text-anchor="middle" font-family="sans-serif" font-size="11" fill="white">MIT</text>
//	</g>
//	<!-- Badge: Go -->
//	<g transform="translate(109, 0)">
//	  <rect x="0" y="0" width="34" height="20" fill="#6c757d"/>
//	  <rect x="34" y="0" width="25" height="20" fill="blue"/>
//	  <text x="17" y="14" text-anchor="middle" font-family="sans-serif" font-size="11" fill="white">Go</text>
//	  <text x="46" y="14" text-anchor="middle" font-family="sans-serif" font-size="11" fill="white">1.22</text>
//	</g>
//
// </svg>
func (h *Badges) GenerateSVG() ([]byte, int, error) {
	svg, count, _, err := h.generateSVGFromParams(h.args)
	if err != nil {
		return nil, 0, err
	}
	return []byte(svg), count, nil
}

// private: parse inputs and produce svg; mirrors behavior of badges.sh but without filesystem/README changes
func (h *Badges) generateSVGFromParams(params []string) (string, int, []string, error) {
	var parsed []Badge
	var warnings []string
	for _, p := range params {
		b, err := parseBadge(p)
		if err != nil {
			// special command -> ignore
			if strings.Contains(err.Error(), "special command") {
				continue
			}
			// collect warnings but continue
			warnings = append(warnings, err.Error())
			continue
		}
		parsed = append(parsed, b)
	}

	if len(parsed) == 0 {
		return "", 0, warnings, fmt.Errorf("no valid badges to generate")
	}

	svg, err := h.generateSVG(parsed)
	if err != nil {
		return "", 0, warnings, err
	}
	return svg, len(parsed), warnings, nil
}

// calcTextWidth approximates text width similarly to the bash helper
func (h *Badges) calcTextWidth(text string) int {
	return len(text) * h.fontSize * 6 / 10
}

// generateBadgeSVG produces the <g> element for a single badge (label+value)
func (h *Badges) generateBadgeSVG(label, value, color string, xOffset, labelWidth, valueWidth int) string {
	// text positions (y) use the same vertical centering as bash: BADGE_HEIGHT/2 + 4
	labelX := labelWidth / 2
	valueX := labelWidth + valueWidth/2
	textY := h.badgeHeight/2 + 4

	return fmt.Sprintf("    <!-- Badge: %s -->\n    <g transform=\"translate(%d, 0)\">\n        <!-- Label background -->\n        <rect x=\"0\" y=\"0\" width=\"%d\" height=\"%d\" fill=\"%s\"/>\n        <!-- Value background -->\n        <rect x=\"%d\" y=\"0\" width=\"%d\" height=\"%d\" fill=\"%s\"/>\n        <!-- Label text -->\n        <text x=\"%d\" y=\"%d\" \n              text-anchor=\"middle\" font-family=\"sans-serif\" font-size=\"%d\" fill=\"white\">%s</text>\n        <!-- Value text -->\n        <text x=\"%d\" y=\"%d\" \n              text-anchor=\"middle\" font-family=\"sans-serif\" font-size=\"%d\" fill=\"white\">%s</text>\n    </g>",
		label, xOffset,
		labelWidth, h.badgeHeight, h.labelBg,
		labelWidth, valueWidth, h.badgeHeight, color,
		labelX, textY, h.fontSize, label,
		valueX, textY, h.fontSize, value,
	)
}

// generateSVG generates a visual SVG for provided badges, matching the bash layout
func (h *Badges) generateSVG(badges []Badge) (string, error) {
	if len(badges) == 0 {
		return "", fmt.Errorf("no badges to generate")
	}

	var parts []string

	currentX := 0
	totalWidth := 0

	// Generate inner groups and compute widths
	for _, b := range badges {
		lw := h.calcTextWidth(b.Label)
		vw := h.calcTextWidth(b.Value)

		lw = lw + h.labelPadding*2
		vw = vw + h.valuePadding*2

		badgeWidth := lw + vw

		parts = append(parts, h.generateBadgeSVG(b.Label, b.Value, b.Color, currentX, lw, vw))

		currentX = currentX + badgeWidth + h.badgeSpacing
		totalWidth = currentX
	}

	if totalWidth > 0 {
		totalWidth = totalWidth - h.badgeSpacing
	}

	if totalWidth == 0 {
		return "", fmt.Errorf("no badges to generate")
	}

	// Build final SVG with header and viewBox/size
	header := fmt.Sprintf("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<!-- %s -->\n<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"%d\" height=\"%d\" viewBox=\"0 0 %d %d\">", h.svgInfo, totalWidth, h.svgHeight, totalWidth, h.svgHeight)

	all := []string{header}
	all = append(all, parts...)
	// add an empty line before closing to match expected formatting
	all = append(all, "", "</svg>")

	return strings.Join(all, "\n"), nil
}

// GenerateSVGFromParams is the public helper that accepts raw badge parameter strings
// and returns the generated SVG, the number of badges processed, any warnings, and an error.
// Note: badge parsing + generation helpers are used by package consumers.
// The higher-level API (handler-based) is implemented in badges.go.
