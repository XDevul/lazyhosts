package hostctl

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
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

// runElevatedHostctl executes a hostctl subcommand with elevated privileges.
// On Unix, uses sudo -n. On Windows, runs hostctl directly (requires admin terminal).
func runElevatedHostctl(args ...string) Result {
	var out []byte
	var err error

	if runtime.GOOS == "windows" {
		out, err = exec.Command("hostctl", args...).CombinedOutput()
		if err != nil {
			outStr := string(out)
			if strings.Contains(outStr, "Access is denied") || strings.Contains(outStr, "requires elevation") {
				return Result{
					Error:      fmt.Errorf("administrator required — please run this terminal as Administrator"),
					ExecutedAt: time.Now(),
				}
			}
			return Result{
				Error:      fmt.Errorf("hostctl %s failed: %w\n%s", args[0], err, outStr),
				ExecutedAt: time.Now(),
			}
		}
	} else {
		cmdArgs := append([]string{"-n", "hostctl"}, args...)
		out, err = exec.Command("sudo", cmdArgs...).CombinedOutput()
		if err != nil {
			outStr := string(out)
			if strings.Contains(outStr, "sudo") || strings.Contains(outStr, "password") {
				return Result{
					Error:      fmt.Errorf("sudo required — run 'sudo -v' first, then retry"),
					ExecutedAt: time.Now(),
				}
			}
			return Result{
				Error:      fmt.Errorf("hostctl %s failed: %w\n%s", args[0], err, outStr),
				ExecutedAt: time.Now(),
			}
		}
	}

	return Result{Output: string(out), ExecutedAt: time.Now()}
}

// hostsFilePath returns the OS-specific hosts file path.
func hostsFilePath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("SystemRoot"), "System32", "drivers", "etc", "hosts")
	}
	return "/etc/hosts"
}

// HostsPreview reads the first n lines of the hosts file.
func HostsPreview(n int) (string, error) {
	hostsPath := hostsFilePath()
	f, err := os.Open(hostsPath)
	if err != nil {
		return "", fmt.Errorf("cannot read %s: %w", hostsPath, err)
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
	return runElevatedHostctl("enable", name)
}

// DisableProfile disables a profile via hostctl. Requires sudo.
func DisableProfile(name string) Result {
	if err := validateName(name); err != nil {
		return Result{Error: err, ExecutedAt: time.Now()}
	}
	return runElevatedHostctl("disable", name)
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

	return runElevatedHostctl("add", name, "--from", tmpFile.Name())
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

	return runElevatedHostctl("add", name, "--from", filePath)
}

// RemoveProfile removes a profile entirely.
func RemoveProfile(name string) Result {
	if err := validateName(name); err != nil {
		return Result{Error: err, ExecutedAt: time.Now()}
	}
	return runElevatedHostctl("remove", name)
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

// CopyProfile duplicates a profile's entries under a new name.
func CopyProfile(srcName, newName string) Result {
	if err := validateName(srcName); err != nil {
		return Result{Error: err, ExecutedAt: time.Now()}
	}
	if err := validateName(newName); err != nil {
		return Result{Error: err, ExecutedAt: time.Now()}
	}

	entries, err := GetProfileEntries(srcName)
	if err != nil {
		return Result{Error: fmt.Errorf("copy failed: %w", err), ExecutedAt: time.Now()}
	}

	return AddProfile(newName, entries)
}

// BatchChangeIP replaces all IPs in a profile with the given new IP.
func BatchChangeIP(name string, newIP string) Result {
	if err := validateName(name); err != nil {
		return Result{Error: err, ExecutedAt: time.Now()}
	}
	newIP = strings.TrimSpace(newIP)
	if newIP == "" {
		return Result{Error: fmt.Errorf("IP cannot be empty"), ExecutedAt: time.Now()}
	}

	entries, err := GetProfileEntries(name)
	if err != nil {
		return Result{Error: fmt.Errorf("batch change IP failed: %w", err), ExecutedAt: time.Now()}
	}

	var newLines []string
	for _, line := range strings.Split(entries, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			fields[0] = newIP
			newLines = append(newLines, strings.Join(fields, " "))
		}
	}

	if len(newLines) == 0 {
		return Result{Error: fmt.Errorf("no entries to update"), ExecutedAt: time.Now()}
	}

	return UpdateProfile(name, strings.Join(newLines, "\n"))
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

// NeedsSudo returns true if the current OS requires sudo for privilege elevation.
func NeedsSudo() bool {
	return runtime.GOOS != "windows"
}

// HasElevatedPrivilege checks if we have the required privileges.
// On Unix: checks sudo credential cache. On Windows: checks if running as Administrator.
func HasElevatedPrivilege() bool {
	if runtime.GOOS == "windows" {
		// Try writing to a protected path to detect admin privileges.
		f, err := os.CreateTemp(filepath.Join(os.Getenv("SystemRoot"), "System32"), "lazyhosts-check-*.tmp")
		if err != nil {
			return false
		}
		name := f.Name()
		f.Close()
		os.Remove(name)
		return true
	}
	err := exec.Command("sudo", "-n", "true").Run()
	return err == nil
}

// AcquireSudo runs `sudo -v` interactively so the user can enter their password.
// This must be called BEFORE the TUI starts (before AltScreen takes over).
// On Windows this is a no-op (admin must be granted when opening the terminal).
func AcquireSudo() error {
	if runtime.GOOS == "windows" {
		return nil
	}
	cmd := exec.Command("sudo", "-v")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// SudoKeepalive starts a background goroutine that refreshes sudo credentials
// every 2 minutes. Returns a stop function to cancel the keepalive.
// On Windows this is a no-op.
func SudoKeepalive() (stop func()) {
	done := make(chan struct{})
	if runtime.GOOS == "windows" {
		return func() { close(done) }
	}
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
