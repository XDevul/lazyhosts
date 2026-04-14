package hostctl

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// validProfileName only allows alphanumeric, underscore, and hyphen.
var validProfileName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Profile represents a hostctl profile with its status.
type Profile struct {
	Name    string
	Enabled bool
	Entries int
}

// HostEntry represents a single row from hostctl JSON output.
type HostEntry struct {
	Profile string `json:"Profile"`
	Status  string `json:"Status"`
	IP      string `json:"IP"`
	Host    string `json:"Host"`
}

// Result holds the outcome of a hostctl command.
type Result struct {
	Profiles   []Profile
	Output     string
	Error      error
	ExecutedAt time.Time
}

// validateName checks that a profile name contains only safe characters.
func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}
	if !validProfileName.MatchString(name) {
		return fmt.Errorf("invalid profile name %q: only [a-zA-Z0-9_-] allowed", name)
	}
	return nil
}

// validateFilePath checks that a file path does not contain path traversal.
func validateFilePath(path string) error {
	if path == "" {
		return fmt.Errorf("file path cannot be empty")
	}
	// Reject if the raw input contains ".." components
	if strings.Contains(path, "..") {
		return fmt.Errorf("path traversal not allowed: %q", path)
	}
	// Must be an absolute path
	if !filepath.IsAbs(path) {
		return fmt.Errorf("must be an absolute path: %q", path)
	}
	return nil
}

// runSudoHostctl executes a hostctl subcommand with sudo -n, handling
// common sudo permission errors consistently.
func runSudoHostctl(args ...string) Result {
	cmdArgs := append([]string{"-n", "hostctl"}, args...)
	out, err := exec.Command("sudo", cmdArgs...).CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "sudo") || strings.Contains(string(out), "password") {
			return Result{
				Error:      fmt.Errorf("sudo required — run 'sudo -v' first, then retry"),
				ExecutedAt: time.Now(),
			}
		}
		return Result{
			Error:      fmt.Errorf("hostctl %s failed: %w\n%s", args[0], err, string(out)),
			ExecutedAt: time.Now(),
		}
	}
	return Result{Output: string(out), ExecutedAt: time.Now()}
}

// HostsPreview reads the first n lines of /etc/hosts.
func HostsPreview(n int) (string, error) {
	f, err := os.Open("/etc/hosts")
	if err != nil {
		return "", fmt.Errorf("cannot read /etc/hosts: %w", err)
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for i := 0; i < n && scanner.Scan(); i++ {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return strings.Join(lines, "\n"), nil
}

// ListProfiles uses `hostctl list -o json` for reliable parsing.
func ListProfiles() Result {
	out, err := exec.Command("hostctl", "list", "-o", "json").CombinedOutput()
	if err != nil {
		return Result{Error: fmt.Errorf("hostctl list failed: %w\n%s", err, string(out)), ExecutedAt: time.Now()}
	}

	profiles := parseJSON(out)
	return Result{Profiles: profiles, Output: "", ExecutedAt: time.Now()}
}

// EnableProfile enables a profile via hostctl. Requires sudo.
func EnableProfile(name string) Result {
	if err := validateName(name); err != nil {
		return Result{Error: err, ExecutedAt: time.Now()}
	}
	return runSudoHostctl("enable", name)
}

// DisableProfile disables a profile via hostctl. Requires sudo.
func DisableProfile(name string) Result {
	if err := validateName(name); err != nil {
		return Result{Error: err, ExecutedAt: time.Now()}
	}
	return runSudoHostctl("disable", name)
}

// AddProfile creates a new profile with the given host entries.
// entries should be lines of "IP DOMAIN" format.
func AddProfile(name string, entries string) Result {
	if err := validateName(name); err != nil {
		return Result{Error: err, ExecutedAt: time.Now()}
	}

	tmpFile, err := os.CreateTemp("", "lazyhosts-*.txt")
	if err != nil {
		return Result{Error: fmt.Errorf("failed to create temp file: %w", err), ExecutedAt: time.Now()}
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(entries); err != nil {
		tmpFile.Close()
		return Result{Error: fmt.Errorf("failed to write temp file: %w", err), ExecutedAt: time.Now()}
	}
	tmpFile.Close()

	return runSudoHostctl("add", name, "--from", tmpFile.Name())
}

// ImportProfile creates a new profile from an existing file.
func ImportProfile(name string, filePath string) Result {
	if err := validateName(name); err != nil {
		return Result{Error: err, ExecutedAt: time.Now()}
	}
	if err := validateFilePath(filePath); err != nil {
		return Result{Error: err, ExecutedAt: time.Now()}
	}
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return Result{Error: fmt.Errorf("file not found: %s", filePath), ExecutedAt: time.Now()}
	}

	return runSudoHostctl("add", name, "--from", filePath)
}

// RemoveProfile removes a profile entirely.
func RemoveProfile(name string) Result {
	if err := validateName(name); err != nil {
		return Result{Error: err, ExecutedAt: time.Now()}
	}
	return runSudoHostctl("remove", name)
}

// RenameProfile renames a profile by reading its entries, removing the old, and adding with the new name.
func RenameProfile(oldName, newName string) Result {
	if err := validateName(oldName); err != nil {
		return Result{Error: err, ExecutedAt: time.Now()}
	}
	if err := validateName(newName); err != nil {
		return Result{Error: err, ExecutedAt: time.Now()}
	}

	// Read existing entries
	entries, err := GetProfileEntries(oldName)
	if err != nil {
		return Result{Error: fmt.Errorf("rename failed: %w", err), ExecutedAt: time.Now()}
	}

	// Remove old profile
	removeResult := RemoveProfile(oldName)
	if removeResult.Error != nil {
		return removeResult
	}

	// Add with new name
	addResult := AddProfile(newName, entries)
	if addResult.Error != nil {
		// Attempt to restore old profile on failure
		AddProfile(oldName, entries)
		return addResult
	}

	return Result{Output: addResult.Output, ExecutedAt: time.Now()}
}

// UpdateProfile replaces a profile's entries by removing then re-adding.
func UpdateProfile(name string, entries string) Result {
	if err := validateName(name); err != nil {
		return Result{Error: err, ExecutedAt: time.Now()}
	}
	removeResult := RemoveProfile(name)
	if removeResult.Error != nil {
		return removeResult
	}
	return AddProfile(name, entries)
}

// GetProfileEntries returns the raw host entries for a profile as "IP DOMAIN" lines.
func GetProfileEntries(name string) (string, error) {
	if err := validateName(name); err != nil {
		return "", err
	}

	out, err := exec.Command("hostctl", "list", name, "-o", "json").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get profile entries: %w\n%s", err, string(out))
	}

	var entries []HostEntry
	if err := json.Unmarshal(out, &entries); err != nil {
		return "", fmt.Errorf("failed to parse profile entries: %w", err)
	}

	var lines []string
	for _, e := range entries {
		lines = append(lines, fmt.Sprintf("%s %s", e.IP, e.Host))
	}
	return strings.Join(lines, "\n"), nil
}

// ShowProfile returns a compact detail of a specific profile (IP → DOMAIN).
func ShowProfile(name string) Result {
	if err := validateName(name); err != nil {
		return Result{Error: err, ExecutedAt: time.Now()}
	}

	out, err := exec.Command("hostctl", "list", name, "-o", "json").CombinedOutput()
	if err != nil {
		return Result{Error: fmt.Errorf("show profile failed: %w\n%s", err, string(out)), ExecutedAt: time.Now()}
	}

	var entries []HostEntry
	if err := json.Unmarshal(out, &entries); err != nil {
		return Result{Error: fmt.Errorf("parse profile failed: %w", err), ExecutedAt: time.Now()}
	}

	var lines []string
	for _, e := range entries {
		lines = append(lines, fmt.Sprintf("%-15s  %s", e.IP, e.Host))
	}
	return Result{Output: strings.Join(lines, "\n"), ExecutedAt: time.Now()}
}

// parseJSON parses hostctl JSON output, deduplicating profiles.
func parseJSON(data []byte) []Profile {
	var entries []HostEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil
	}

	type profileInfo struct {
		enabled bool
		count   int
	}
	seen := make(map[string]*profileInfo)
	var order []string

	for _, e := range entries {
		info, exists := seen[e.Profile]
		if !exists {
			info = &profileInfo{enabled: strings.EqualFold(e.Status, "on")}
			seen[e.Profile] = info
			order = append(order, e.Profile)
		}
		info.count++
	}

	var profiles []Profile
	for _, name := range order {
		info := seen[name]
		profiles = append(profiles, Profile{
			Name:    name,
			Enabled: info.enabled,
			Entries: info.count,
		})
	}

	return profiles
}

// IsInstalled checks if hostctl is available on the system.
func IsInstalled() bool {
	_, err := exec.LookPath("hostctl")
	return err == nil
}

// HasSudo checks if we can run sudo non-interactively.
func HasSudo() bool {
	err := exec.Command("sudo", "-n", "true").Run()
	return err == nil
}

// AcquireSudo runs `sudo -v` interactively so the user can enter their password.
// This must be called BEFORE the TUI starts (before AltScreen takes over).
func AcquireSudo() error {
	cmd := exec.Command("sudo", "-v")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// SudoKeepalive starts a background goroutine that refreshes sudo credentials
// every 2 minutes. Returns a stop function to cancel the keepalive.
func SudoKeepalive() (stop func()) {
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				exec.Command("sudo", "-v", "-n").Run()
			case <-done:
				return
			}
		}
	}()
	return func() { close(done) }
}
