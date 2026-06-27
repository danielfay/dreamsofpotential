// Package debuglog writes structured JSON-lines to logs/debug.log when enabled
// via the -debug flag. It is a no-op when disabled so Emit calls are safe in
// hot simulation paths.
//
// Usage:
//
//	go run . -debug=workers,economy
//
// Each Emit call appends one JSON line. The log is capped at maxLines and then
// frozen (no rotation) — restart the game to get a fresh log.
package debuglog

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
)

// F is a shorthand for the field map passed to Emit.
type F = map[string]any

const maxLines = 10_000

var (
	mu        sync.Mutex
	file      *os.File
	cats      map[string]bool
	lineCount int
	capped    bool
)

// Init opens path for writing and enables the listed categories.
// Overwrites any existing file. Calling Init again replaces the previous session.
func Init(categories, path string) error {
	mu.Lock()
	defer mu.Unlock()

	if file != nil {
		file.Close()
		file = nil
	}

	cats = make(map[string]bool)
	for _, c := range strings.Split(categories, ",") {
		if c = strings.TrimSpace(c); c != "" {
			cats[c] = true
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	file = f
	lineCount = 0
	capped = false
	return nil
}

// Emit writes one JSON line if category is enabled and the cap has not been hit.
// Fields are written as-is; "cat" is injected automatically.
// Safe to call when disabled — returns immediately with no allocations on the
// hot path (nil file check before acquiring the mutex).
func Emit(category string, fields F) {
	if file == nil {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if !cats[category] || capped {
		return
	}
	if lineCount >= maxLines {
		capped = true
		capLine, _ := json.Marshal(F{"cap": "reached", "lines": maxLines})
		file.Write(capLine)
		file.Write([]byte("\n"))
		return
	}
	fields["cat"] = category
	b, err := json.Marshal(fields)
	if err != nil {
		return
	}
	file.Write(b)
	file.Write([]byte("\n"))
	lineCount++
}

// Close flushes and closes the log file.
func Close() {
	mu.Lock()
	defer mu.Unlock()
	if file != nil {
		file.Close()
		file = nil
	}
}

// Enabled reports whether category is currently active.
func Enabled(category string) bool {
	if file == nil {
		return false
	}
	mu.Lock()
	defer mu.Unlock()
	return cats[category]
}
