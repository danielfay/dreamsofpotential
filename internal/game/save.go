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

// Save serialises w to disk atomically. Safe to call from a periodic timer.
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

// Load deserialises the save file and rebuilds internal pointers.
// Returns os.ErrNotExist (wrapped) if no save file exists yet.
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
	// Rebuild Worker.Home back-pointers excluded from JSON.
	for _, b := range w.Buildings {
		for _, wk := range b.Workers {
			wk.Home = b
		}
	}
	return &w, nil
}
