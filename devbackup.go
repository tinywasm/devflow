package devflow

import (
	"fmt"
	"os"
)

const (
	backupEnvVar = "DEV_BACKUP"
)

// DevBackup handles backup operations
type DevBackup struct {
	bashrc *Bashrc
}

// NewDevBackup creates a new DevBackup instance
func NewDevBackup() *DevBackup {
	return &DevBackup{
		bashrc: NewBashrc(),
	}
}

// SetCommand sets the backup command in .bashrc and current environment
func (d *DevBackup) SetCommand(command string) error {
	// Save to .bashrc for persistence
	if err := d.bashrc.Set(backupEnvVar, command); err != nil {
		return err
	}

	// Update current process environment for immediate use
	if command == "" {
		os.Unsetenv(backupEnvVar)
	} else {
		os.Setenv(backupEnvVar, command)
	}

	return nil
}

// GetCommand retrieves the backup command
// First checks environment variable, then falls back to .bashrc
func (d *DevBackup) GetCommand() (string, error) {
	// Try environment variable first (current session)
	if envCmd := os.Getenv(backupEnvVar); envCmd != "" {
		return envCmd, nil
	}

	// Fallback to .bashrc
	return d.bashrc.Get(backupEnvVar)
}

// Run executes the backup command asynchronously
// Returns a message for the summary or empty string if not configured
func (d *DevBackup) Run() (string, error) {
	command, err := d.GetCommand()
	if err != nil {
		// Not configured, silent skip
		return "", nil
	}

	if command == "" {
		return "", nil
	}

	// Execute asynchronously at OS level
	if err := RunShellCommandAsync(command); err != nil {
		return "", fmt.Errorf("failed to start backup: %w", err)
	}

	return "✅ Backup started", nil
}
