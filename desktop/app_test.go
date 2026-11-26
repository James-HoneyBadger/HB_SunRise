// SPDX-FileCopyrightText: 2020-2026 Honey Badger Universe
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/James-HoneyBadger/hb-sunrise/pkg/converters/msmoney/synthetic"
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

// TestInspectRowsConcurrentPageWalks runs raw inspection pages in parallel, as
// the window does when a user pages quickly. The inspection Database memoizes
// its pages on first use, so without serialization this races; the test only
// proves it under the race detector, where it must never report a race.
func TestInspectRowsConcurrentPageWalks(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()

	path := filepath.Join(t.TempDir(), "synthetic.mny")

	if err := os.WriteFile(path, synthetic.MoneyFile(), 0o600); err != nil {
		t.Fatalf("write synthetic database: %v", err)
	}

	if _, err := app.Inspect(path); err != nil {
		t.Fatalf("inspect synthetic database: %v", err)
	}

	var group sync.WaitGroup

	for range 8 {
		group.Add(1)

		go func() {
			defer group.Done()

			if _, err := app.InspectRows("TRN", 0, 50); err != nil {
				t.Errorf("inspect rows: %v", err)
			}
		}()
	}

	group.Wait()
}
