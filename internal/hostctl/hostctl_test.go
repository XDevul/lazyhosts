package hostctl

import (
	"strings"
	"testing"
)

func TestParseJSON(t *testing.T) {
	input := `[{"Profile":"default","Status":"on","IP":"127.0.0.1","Host":"localhost"},{"Profile":"default","Status":"on","IP":"::1","Host":"localhost"},{"Profile":"dev","Status":"on","IP":"10.0.0.1","Host":"myapp.dev"},{"Profile":"dev","Status":"on","IP":"10.0.0.2","Host":"api.dev"},{"Profile":"prod","Status":"off","IP":"10.0.1.1","Host":"myapp.prod"}]`

	profiles := parseJSON([]byte(input))

	if len(profiles) != 3 {
		t.Fatalf("expected 3 profiles, got %d", len(profiles))
	}

	tests := []struct {
		name    string
		enabled bool
		entries int
	}{
		{"default", true, 2},
		{"dev", true, 2},
		{"prod", false, 1},
	}

	for i, tt := range tests {
		if profiles[i].Name != tt.name {
			t.Errorf("profile[%d].Name = %q, want %q", i, profiles[i].Name, tt.name)
		}
		if profiles[i].Enabled != tt.enabled {
			t.Errorf("profile[%d].Enabled = %v, want %v", i, profiles[i].Enabled, tt.enabled)
		}
		if profiles[i].Entries != tt.entries {
			t.Errorf("profile[%d].Entries = %d, want %d", i, profiles[i].Entries, tt.entries)
		}
	}
}

func TestParseJSONEmpty(t *testing.T) {
	profiles := parseJSON([]byte("[]"))
	if len(profiles) != 0 {
		t.Fatalf("expected 0 profiles, got %d", len(profiles))
	}
}

func TestParseJSONInvalid(t *testing.T) {
	profiles := parseJSON([]byte("not json"))
	if profiles != nil {
		t.Fatalf("expected nil, got %v", profiles)
	}
}

func TestValidateName(t *testing.T) {
	valid := []string{"dev", "sit", "prod", "my-profile", "test_123", "a", "a1-b2_c3"}
	for _, name := range valid {
		if err := validateName(name); err != nil {
			t.Errorf("validateName(%q) should pass, got: %v", name, err)
		}
	}

	invalid := []string{
		"",
		"dev;rm -rf /",
		"my profile",
		"test|cat",
		"../etc",
		"name&cmd",
		"foo\nbar",
		"$(whoami)",
		"`id`",
	}
	for _, name := range invalid {
		if err := validateName(name); err == nil {
			t.Errorf("validateName(%q) should fail, but passed", name)
		}
	}
}

func TestNormalizeName(t *testing.T) {
	cases := map[string]string{
		"EAS":     "eas",
		"EAS-PMO": "eas-pmo",
		"  dev  ": "dev",
		"dev":     "dev",
	}
	for in, want := range cases {
		if got := NormalizeName(in); got != want {
			t.Errorf("NormalizeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateFilePath(t *testing.T) {
	valid := []string{"/tmp/hosts.txt", "/home/user/hosts", "/etc/hosts.bak"}
	for _, p := range valid {
		if err := validateFilePath(p); err != nil {
			t.Errorf("validateFilePath(%q) should pass, got: %v", p, err)
		}
	}

	invalid := []string{
		"",                                  // empty
		"/tmp/../etc/shadow",                // path traversal with ..
		"../../etc/passwd",                  // relative + traversal
		"/home/user/../../root/.ssh/id_rsa", // traversal in middle
		"hosts.txt",                         // relative path (not absolute)
	}
	for _, p := range invalid {
		if err := validateFilePath(p); err == nil {
			t.Errorf("validateFilePath(%q) should fail, but passed", p)
		}
	}
}

func TestBatchReplaceIPLogic(t *testing.T) {
	tests := []struct {
		name     string
		entries  string
		newIP    string
		expected string
	}{
		{
			name:     "single entry",
			entries:  "10.0.0.1 myapp.dev",
			newIP:    "192.168.1.1",
			expected: "192.168.1.1 myapp.dev",
		},
		{
			name:     "multiple entries",
			entries:  "10.0.0.1 myapp.dev\n10.0.0.2 api.dev\n10.0.0.3 db.dev",
			newIP:    "127.0.0.1",
			expected: "127.0.0.1 myapp.dev\n127.0.0.1 api.dev\n127.0.0.1 db.dev",
		},
		{
			name:     "entries with extra spaces",
			entries:  "  10.0.0.1   myapp.dev  \n  10.0.0.2   api.dev  ",
			newIP:    "1.2.3.4",
			expected: "1.2.3.4 myapp.dev\n1.2.3.4 api.dev",
		},
		{
			name:     "skip empty lines",
			entries:  "10.0.0.1 myapp.dev\n\n\n10.0.0.2 api.dev",
			newIP:    "5.5.5.5",
			expected: "5.5.5.5 myapp.dev\n5.5.5.5 api.dev",
		},
		{
			name:     "skip single-field lines",
			entries:  "10.0.0.1 myapp.dev\njunk\n10.0.0.2 api.dev",
			newIP:    "9.9.9.9",
			expected: "9.9.9.9 myapp.dev\n9.9.9.9 api.dev",
		},
		{
			name:     "entry with multiple hosts",
			entries:  "10.0.0.1 myapp.dev www.myapp.dev",
			newIP:    "8.8.8.8",
			expected: "8.8.8.8 myapp.dev www.myapp.dev",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := batchReplaceIP(tt.entries, tt.newIP)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestBatchReplaceIPEmpty(t *testing.T) {
	result := batchReplaceIP("", "1.2.3.4")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}

	result = batchReplaceIP("\n\n\n", "1.2.3.4")
	if result != "" {
		t.Errorf("expected empty string for blank lines, got %q", result)
	}
}

func TestListProfilesIntegration(t *testing.T) {
	if !IsInstalled() {
		t.Skip("hostctl not installed")
	}
	result := ListProfiles()
	if result.Error != nil {
		t.Fatalf("ListProfiles failed: %v", result.Error)
	}
	if len(result.Profiles) == 0 {
		t.Fatal("expected at least 1 profile (default)")
	}
	if result.Profiles[0].Name != "default" {
		t.Errorf("first profile should be 'default', got %q", result.Profiles[0].Name)
	}
}

func TestHostsPreview(t *testing.T) {
	preview, err := HostsPreview(5)
	if err != nil {
		t.Fatalf("HostsPreview failed: %v", err)
	}
	if preview == "" {
		t.Fatal("expected non-empty preview")
	}
}

func TestCopyProfileIntegration(t *testing.T) {
	if !IsInstalled() {
		t.Skip("hostctl not installed")
	}
	if !HasElevatedPrivilege() {
		t.Skip("sudo not available")
	}

	// Setup: create a source profile
	src := "test-copy-src"
	dst := "test-copy-dst"
	AddProfile(src, "10.0.0.1 copy-test.dev\n10.0.0.2 copy-test2.dev")
	defer RemoveProfile(src)
	defer RemoveProfile(dst)

	// Execute copy
	result := CopyProfile(src, dst)
	if result.Error != nil {
		t.Fatalf("CopyProfile failed: %v", result.Error)
	}

	// Verify: dst should have same entries
	dstEntries, err := GetProfileEntries(dst)
	if err != nil {
		t.Fatalf("GetProfileEntries(dst) failed: %v", err)
	}
	srcEntries, err := GetProfileEntries(src)
	if err != nil {
		t.Fatalf("GetProfileEntries(src) failed: %v", err)
	}
	if dstEntries != srcEntries {
		t.Errorf("entries mismatch:\n  src: %q\n  dst: %q", srcEntries, dstEntries)
	}
}

func TestBatchChangeIPIntegration(t *testing.T) {
	if !IsInstalled() {
		t.Skip("hostctl not installed")
	}
	if !HasElevatedPrivilege() {
		t.Skip("sudo not available")
	}

	// Setup: create a test profile
	name := "test-batch-ip"
	AddProfile(name, "10.0.0.1 batch-test.dev\n10.0.0.2 batch-test2.dev")
	defer RemoveProfile(name)

	// Execute batch change
	result := BatchChangeIP(name, "192.168.1.100")
	if result.Error != nil {
		t.Fatalf("BatchChangeIP failed: %v", result.Error)
	}

	// Verify: all IPs should be changed
	entries, err := GetProfileEntries(name)
	if err != nil {
		t.Fatalf("GetProfileEntries failed: %v", err)
	}
	for _, line := range strings.Split(entries, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] != "192.168.1.100" {
			t.Errorf("expected IP 192.168.1.100, got %q in line %q", fields[0], line)
		}
	}
}
