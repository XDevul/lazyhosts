package hostctl

import (
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
	valid := []string{"dev", "sit", "prod", "my-profile", "test_123", "A", "a1-b2_c3"}
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

func TestValidateFilePath(t *testing.T) {
	valid := []string{"/tmp/hosts.txt", "/home/user/hosts", "/etc/hosts.bak"}
	for _, p := range valid {
		if err := validateFilePath(p); err != nil {
			t.Errorf("validateFilePath(%q) should pass, got: %v", p, err)
		}
	}

	invalid := []string{
		"",                                   // empty
		"/tmp/../etc/shadow",                 // path traversal with ..
		"../../etc/passwd",                   // relative + traversal
		"/home/user/../../root/.ssh/id_rsa",  // traversal in middle
		"hosts.txt",                          // relative path (not absolute)
	}
	for _, p := range invalid {
		if err := validateFilePath(p); err == nil {
			t.Errorf("validateFilePath(%q) should fail, but passed", p)
		}
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
