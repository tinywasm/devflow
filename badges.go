package devflow

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cdvelop/badges"
	"github.com/cdvelop/mdgo"
)

func getGoVersion() string {
	out, err := RunCommand("go", "version")
	if err != nil {
		return "unknown"
	}
	parts := strings.Fields(out)
	if len(parts) >= 3 {
		return strings.TrimPrefix(parts[2], "go")
	}
	return "unknown"
}

func getBadgeColor(typ, value string) string {
	switch typ {
	case "license", "go":
		return "#007acc"
	case "tests":
		if value == "Passing" {
			return "#4c1"
		}
		return "#e05d44"
	case "coverage":
		// value is string like "85"
		val, _ := strconv.ParseFloat(value, 64)
		if val >= 80 {
			return "#4c1"
		} else if val >= 60 {
			return "#dfb317"
		} else if val > 0 {
			return "#fe7d37"
		}
		return "#e05d44"
	case "race":
		if value == "Clean" {
			return "#4c1"
		}
		return "#e05d44"
	case "vet":
		if value == "OK" {
			return "#4c1"
		}
		return "#e05d44"
	}
	return "#007acc"
}

func getModuleName(dir string) (string, error) {
	f, err := os.Open(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimPrefix(line, "module "), nil
		}
	}
	return "", fmt.Errorf("module name not found in go.mod")
}

func checkFileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func updateBadges(readmeFile, licenseType, goVer, testStatus, coveragePercent, raceStatus, vetStatus string, quiet bool) error {
	// Colors
	licenseColor := getBadgeColor("license", licenseType)
	goColor := getBadgeColor("go", goVer)
	testColor := getBadgeColor("tests", testStatus)
	coverageColor := getBadgeColor("coverage", coveragePercent)
	raceColor := getBadgeColor("race", raceStatus)
	vetColor := getBadgeColor("vet", vetStatus)

	// Format: Label:Value:Color
	// Plus readmefile arg
	badgeArgs := []string{
		"readmefile:" + readmeFile,
		fmt.Sprintf("License:%s:%s", licenseType, licenseColor),
		fmt.Sprintf("Go:%s:%s", goVer, goColor),
		fmt.Sprintf("Tests:%s:%s", testStatus, testColor),
		fmt.Sprintf("Coverage:%s%%:%s", coveragePercent, coverageColor),
		fmt.Sprintf("Race:%s:%s", raceStatus, raceColor),
		fmt.Sprintf("Vet:%s:%s", vetStatus, vetColor),
	}

	sectionArgs, err := badges.NewBadgeHandler(badgeArgs...).BuildBadges()
	if err != nil {
		return fmt.Errorf("error building badges: %w", err)
	}

	// Update README using mdgo
	if len(sectionArgs) >= 4 {
		// sectionArgs: sectionID, afterLine(unused), content, readmeFile
		sectionID := sectionArgs[0]
		content := sectionArgs[2]
		readmeFile := sectionArgs[3]

		// Use mdgo to update
		m := mdgo.New(".", ".", func(name string, data []byte) error {
			return os.WriteFile(name, data, 0644)
		})
		m.InputPath(readmeFile, func(name string) ([]byte, error) {
			return os.ReadFile(name)
		})

		if err := m.UpdateSection(sectionID, content); err != nil {
			return fmt.Errorf("error updating README with mdgo: %w", err)
		}

		if !quiet {
			fmt.Printf("Updated section %s in %s\n", sectionID, readmeFile)
		}
	}

	return nil
}
