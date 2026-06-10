//go:build !js

package game

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const saveDirName = "dreamsofpotential"
const saveFileName = "save.json"

func savePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, saveDirName, saveFileName), nil
}

// Save serialises w to disk atomically.
func Save(w *World) error {
	path, err := savePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// SaveTo serialises w to an explicit path atomically.
func SaveTo(w *World, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ClearSave deletes the save file. Errors (e.g. file already missing) are ignored.
func ClearSave() {
	path, err := savePath()
	if err != nil {
		return
	}
	_ = os.Remove(path)
}

// Load deserialises the save file and returns the world.
// Returns os.ErrNotExist (wrapped) if no save file exists or if the save is
// from a different version (treated as missing so the caller starts fresh).
func Load() (*World, error) {
	path, err := savePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var w World
	if err := json.Unmarshal(data, &w); err != nil {
		return nil, err
	}
	if w.Version != SaveVersion {
		return nil, os.ErrNotExist
	}
	return &w, nil
}
