//go:build js

package game

import (
	"encoding/json"
	"os"
	"syscall/js"
)

const localStorageKey = "dreamsofpotential.save"

// Save serialises w to localStorage.
func Save(w *World) error {
	data, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		return err
	}
	js.Global().Get("localStorage").Call("setItem", localStorageKey, string(data))
	return nil
}

// ClearSave removes the save from localStorage.
func ClearSave() {
	js.Global().Get("localStorage").Call("removeItem", localStorageKey)
}

// Load deserialises the world from localStorage.
// Returns os.ErrNotExist if no save is present or the save version is stale.
func Load() (*World, error) {
	val := js.Global().Get("localStorage").Call("getItem", localStorageKey)
	if val.IsNull() || val.IsUndefined() {
		return nil, os.ErrNotExist
	}
	var w World
	if err := json.Unmarshal([]byte(val.String()), &w); err != nil {
		return nil, err
	}
	if w.Version != SaveVersion {
		return nil, os.ErrNotExist
	}
	initTransientWorldState(&w)
	return &w, nil
}
