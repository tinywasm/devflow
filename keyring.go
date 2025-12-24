package devflow

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/zalando/go-keyring"
)

// Keyring service name for storing secrets
const keyringService = "devflow"

// Keyring provides secure credential storage using the system keyring
type Keyring struct {
	log func(...any)
}

// NewKeyring creates a keyring handler and ensures dependencies are installed
func NewKeyring() (*Keyring, error) {
	k := &Keyring{
		log: func(...any) {},
	}
	if err := k.ensureKeyringAvailable(); err != nil {
		return nil, err
	}
	return k, nil
}

// SetLog sets the logging function for the keyring handler
func (k *Keyring) SetLog(fn func(...any)) {
	if fn != nil {
		k.log = fn
	}
}

// Set stores a secret in the keyring
func (k *Keyring) Set(key, value string) error {
	return keyring.Set(keyringService, key, value)
}

// Get retrieves a secret from the keyring
func (k *Keyring) Get(key string) (string, error) {
	return keyring.Get(keyringService, key)
}

// Delete removes a secret from the keyring
func (k *Keyring) Delete(key string) error {
	return keyring.Delete(keyringService, key)
}

// ensureKeyringAvailable checks if keyring is working and installs dependencies if needed
func (k *Keyring) ensureKeyringAvailable() error {
	// Test if keyring is working
	testKey := "devflow_keyring_test"
	err := keyring.Set(keyringService, testKey, "test")
	if err == nil {
		keyring.Delete(keyringService, testKey)
		return nil
	}

	// Keyring failed - try to install on Linux only
	if runtime.GOOS != "linux" {
		return fmt.Errorf("keyring unavailable: %w", err)
	}

	k.log("⚙️  Installing keyring dependencies...")

	if !k.tryInstallKeyring() {
		return fmt.Errorf("could not install keyring. Install manually:\n  Debian/Ubuntu: sudo apt install gnome-keyring libsecret-1-0\n  Fedora: sudo dnf install gnome-keyring libsecret\n  Arch: sudo pacman -S gnome-keyring libsecret")
	}

	k.startKeyringService()

	// Test again
	err = keyring.Set(keyringService, testKey, "test")
	if err == nil {
		keyring.Delete(keyringService, testKey)
		k.log("✅ Keyring installed successfully")
		return nil
	}

	return fmt.Errorf("keyring installation failed: %w", err)
}

// tryInstallKeyring attempts to install keyring using available package manager
func (k *Keyring) tryInstallKeyring() bool {
	type pkgManager struct {
		cmd  string
		args []string
	}

	managers := []pkgManager{
		{"apt", []string{"sudo", "apt", "install", "-y", "gnome-keyring", "libsecret-1-0"}},
		{"dnf", []string{"sudo", "dnf", "install", "-y", "gnome-keyring", "libsecret"}},
		{"pacman", []string{"sudo", "pacman", "-S", "--noconfirm", "gnome-keyring", "libsecret"}},
	}

	for _, m := range managers {
		if _, err := exec.LookPath(m.cmd); err == nil {
			k.log(fmt.Sprintf("   Installing via %s...", m.cmd))
			cmd := exec.Command(m.args[0], m.args[1:]...)
			// We don't pipe to os.Stdout anymore to keep it quiet unless logged
			if cmd.Run() == nil {
				return true
			}
		}
	}
	return false
}

// startKeyringService starts gnome-keyring-daemon if not running
func (k *Keyring) startKeyringService() {
	if _, err := exec.LookPath("gnome-keyring-daemon"); err != nil {
		return
	}

	cmd := exec.Command("gnome-keyring-daemon", "--start", "--components=secrets")
	output, err := cmd.Output()
	if err == nil {
		for _, line := range strings.Split(string(output), "\n") {
			if parts := strings.SplitN(line, "=", 2); len(parts) == 2 {
				os.Setenv(parts[0], parts[1])
			}
		}
	}
}
