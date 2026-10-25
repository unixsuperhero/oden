package oden

import (
	"bytes"
	"runtime/debug"
	"strings"
	"testing"
)

func TestVersionReportsEmbeddedBuildProvenance(t *testing.T) {
	const sha = "1234567890abcdef1234567890abcdef12345678"
	info := &debug.BuildInfo{
		GoVersion: "go1.26.0",
		Main:      debug.Module{Path: "example.com/tool", Version: "v1.2.3"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: sha},
			{Key: "vcs.time", Value: "2026-10-25T12:34:56Z"},
			{Key: "vcs.modified", Value: "true"},
		},
	}
	var out bytes.Buffer
	if err := writeVersion(&out, "tool", info); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"tool", "v1.2.3", sha, "2026-10-25T12:34:56Z", "Modified: true", "go1.26.0", "example.com/tool"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing build provenance %q: %s", want, &out)
		}
	}
}

func TestVersionDoesNotInventMissingProvenance(t *testing.T) {
	for _, info := range []*debug.BuildInfo{nil, {Main: debug.Module{Version: "(devel)"}}} {
		var out bytes.Buffer
		if err := writeVersion(&out, "tool", info); err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{"Commit", "Commit time", "Modified"} {
			if !strings.Contains(out.String(), field+": unknown") {
				t.Fatalf("missing provenance must remain unknown: %s", &out)
			}
		}
		if info != nil && !strings.Contains(out.String(), "(devel)") {
			t.Fatalf("development build version was lost: %s", &out)
		}
	}
}
