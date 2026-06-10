package main

import (
	"encoding/json"
	"io/fs"
	"strings"
	"testing"

	"github.com/danielfay/dreamsofpotential/internal/game"
)

// TestPresetsRoundTrip verifies that every embedded preset file parses cleanly
// and produces a valid world without error. Guards against preset/struct drift.
func TestPresetsRoundTrip(t *testing.T) {
	err := fs.WalkDir(presetFS, "presets", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".json") {
			return err
		}
		t.Run(path, func(t *testing.T) {
			data, err := presetFS.ReadFile(path)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			var p game.QAPreset
			if err := json.Unmarshal(data, &p); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if _, err := game.BuildQAWorld(p); err != nil {
				t.Fatalf("BuildQAWorld: %v", err)
			}
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walking presets: %v", err)
	}
}
