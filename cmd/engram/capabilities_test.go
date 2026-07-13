package main

import (
	"strings"
	"testing"

	versioncheck "github.com/Gentleman-Programming/engram/internal/version"
)

func TestCmdCapabilitiesJSON(t *testing.T) {
	stdout, stderr := captureOutput(t, func() {
		if err := cmdCapabilities([]string{"--json"}); err != nil {
			t.Fatalf("cmdCapabilities: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	want := `{
  "schema_version": 1,
  "features": {
    "atomic_topic_cas": {
      "supported": true,
      "input": {
        "expected_revision": {
          "type": "integer",
          "minimum": 0,
          "requires": "topic_key"
        }
      },
      "success_fields": [
        "id",
        "sync_id",
        "revision_count"
      ],
      "error_codes": [
        "revision_conflict",
        "expected_revision_requires_topic",
        "invalid_expected_revision"
      ]
    }
  }
}
`
	if stdout != want {
		t.Fatalf("stdout contract mismatch\n got: %s\nwant: %s", stdout, want)
	}
}

func TestCmdCapabilitiesParsing(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "human output"},
		{name: "json output", args: []string{"--json"}},
		{name: "unknown flag", args: []string{"--yaml"}, wantErr: true},
		{name: "extra argument", args: []string{"--json", "extra"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, _ := captureOutput(t, func() {
				err := cmdCapabilities(tt.args)
				if (err != nil) != tt.wantErr {
					t.Fatalf("cmdCapabilities(%q) error = %v, wantErr %v", tt.args, err, tt.wantErr)
				}
			})
			if !tt.wantErr && stdout == "" {
				t.Fatal("expected output")
			}
		})
	}
}

func TestMainCapabilitiesIsConfigFree(t *testing.T) {
	oldCheckForUpdates := checkForUpdates
	checkForUpdates = func(string) versioncheck.CheckResult {
		t.Fatal("capabilities must not check for updates")
		return versioncheck.CheckResult{}
	}
	t.Cleanup(func() { checkForUpdates = oldCheckForUpdates })
	withArgs(t, "engram", "capabilities", "--json")

	stdout, stderr := captureOutput(t, func() { main() })
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, `"atomic_topic_cas"`) {
		t.Fatalf("stdout missing CAS capability: %q", stdout)
	}
}
