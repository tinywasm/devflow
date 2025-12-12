package gitgo

import (
	"os/exec"
	"strings"
)

func GetLatestTag() (tag, err string) {
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0")
	output, er := cmd.Output()
	if er != nil {
		return "", "GetLatestTag error " + er.Error()
	}

	// Convierte el resultado en una cadena y elimina espacios en blanco adicionales
	return strings.TrimSpace(string(output)), ""
}
