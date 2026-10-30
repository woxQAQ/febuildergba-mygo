package testutil

import (
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
)

// Games enumerates the native versions exercised by shared workflows.
func Games() []struct {
	Name  string
	Build func() *FixtureBuilder
} {
	return []struct {
		Name  string
		Build func() *FixtureBuilder
	}{
		{"FE8U", NewMinimalFE8U},
		{"FE8J", NewMinimalFE8J},
		{"FE7U", NewMinimalFE7U},
		{"FE7J", NewMinimalFE7J},
		{"FE6", NewMinimalFE6},
	}
}

// Open writes a synthetic ROM to a test-owned directory and opens a real session.
func Open(t *testing.T, fixture *FixtureBuilder) *game.Session {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.gba")
	if err := fixture.Save(path); err != nil {
		t.Fatal(err)
	}
	s, err := game.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
