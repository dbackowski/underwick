//go:build !js

package main

import (
	"os"
	"path/filepath"
)

// inBrowser is whether this is the browser build, see store_js.go.
const inBrowser = false

// dataDir holds the save and the high scores, in the user's config folder. Tests point it elsewhere.
var dataDir = func() string {
	d, err := os.UserConfigDir()
	if err != nil {
		return "."
	}
	return filepath.Join(d, "underwick")
}()

// hasData, readData, writeData and removeData keep the save and the high scores, by name, as files in
// dataDir.
func hasData(name string) bool {
	_, err := os.Stat(filepath.Join(dataDir, name))
	return err == nil
}

func readData(name string) ([]byte, error) { return os.ReadFile(filepath.Join(dataDir, name)) }

func writeData(name string, data []byte) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dataDir, name), data, 0o644)
}

func removeData(name string) { os.Remove(filepath.Join(dataDir, name)) }
