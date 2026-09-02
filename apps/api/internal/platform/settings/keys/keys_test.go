package keys_test

import (
	"strings"
	"testing"

	"api/internal/platform/settings/keys"
)

var goldenNames = []string{
	"auth.verification_code_ttl_minutes",
	"image.upload_enabled",
	"artifact.upload_enabled",
	"artifact.multipart_threshold_bytes",
	"artifact.part_size_bytes",
	"artifact.presign_upload_ttl_seconds",
	"artifact.presign_download_ttl_seconds",
	"artifact.orphan_ttl_hours",
	"artifact.softdelete_ttl_hours",
	"artifact.reclaim_min_idle_seconds",
	"trust.scan_enabled",
	"trust.check_enabled",
	"trust.scan_mode",
	"trust.scan_sample_rate",
	"ai.escalate_threshold",
	"ai.negative_sample_rate",
	"ai.force_escalate",
	"store.link_quota_per_client",
}

func TestLiveRegistryIsWellFormed(t *testing.T) {
	reg := keys.Live()
	entries := reg.Entries()

	got := make(map[string]bool, len(entries))
	for _, e := range entries {
		m := e.Meta()
		if got[m.Name] {
			t.Errorf("duplicate live key %q", m.Name)
		}
		got[m.Name] = true
		if m.DescEN == "" || m.DescZH == "" {
			t.Errorf("%s: missing description", m.Name)
		}
		if m.EnvVar == "" || !strings.HasPrefix(m.EnvVar, "KUN_") {
			t.Errorf("%s: EnvVar %q must start with KUN_", m.Name, m.EnvVar)
		}
		if err := e.Validate(e.Default()); err != nil {
			t.Errorf("%s: default failed Validate: %v", m.Name, err)
		}
		if m.Kind == "enum" {
			found := false
			def, _ := e.Default().(string)
			for _, v := range m.Enum {
				if v == def {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s: default %q is not in Enum %v", m.Name, def, m.Enum)
			}
		}
	}

	want := make(map[string]bool, len(goldenNames))
	for _, name := range goldenNames {
		want[name] = true
		if !got[name] {
			t.Errorf("golden key %q is missing from Live()", name)
		}
	}
	for name := range got {
		if !want[name] {
			t.Errorf("Live() has extra key %q", name)
		}
	}

	envVars := make(map[string]string)
	for _, e := range entries {
		m := e.Meta()
		if other, ok := envVars[m.EnvVar]; ok {
			t.Errorf("EnvVar %q is used by both %q and %q", m.EnvVar, other, m.Name)
		}
		envVars[m.EnvVar] = m.Name
	}

	for _, d := range reg.Domains() {
		prefix := d.Name + "."
		for _, e := range d.Keys {
			if !strings.HasPrefix(e.Meta().Name, prefix) {
				t.Errorf("%s: name %q does not start with %q", d.Name, e.Meta().Name, prefix)
			}
		}
	}
}
