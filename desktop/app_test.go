// SPDX-FileCopyrightText: 2020-2026 Honey Badger Universe
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"path/filepath"
	"testing"
)

// TestRecentFilesWithoutStoreReturnsEmptyList guards the window against a
// JSON null: it reads the list on first render, and a nil slice would arrive
// as null and crash the component into a blank window.
func TestRecentFilesWithoutStoreReturnsEmptyList(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("AppData", filepath.Join(home, "appdata"))
	t.Setenv("HOME", home)

	recent := NewApp().RecentFiles()

	if recent == nil {
		t.Fatal("RecentFiles returned nil; a nil slice becomes JSON null in the window")
	}

	if len(recent) != 0 {
		t.Fatalf("RecentFiles returned %d entries, want 0", len(recent))
	}
}
