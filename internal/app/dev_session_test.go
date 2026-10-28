package app

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestDeveloperROMRestoreWorkflow(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "cache", "dev-session.json")
	first := filepath.Join(root, "first ROM.gba")
	second := filepath.Join(root, "second.gba")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(
			path,
			[]byte("untouched ROM"),
			0o600,
		); err != nil {
			t.Fatal(err)
		}
	}
	session := newDevelopmentSession(cache, true)
	if paths, err := session.startupPaths(nil); err != nil || len(paths) != 0 {
		t.Fatal("first launch restore", paths, err)
	}
	snapshot := []string{second, first, first}
	session.remember(snapshot)
	snapshot[0] = "changed caller slice"
	session.close()
	reopened := newDevelopmentSession(cache, true)
	paths, err := reopened.startupPaths(nil)
	if err != nil || !slices.Equal(paths, []string{first, second}) {
		t.Fatal("rebuild lost remembered paths", paths, err)
	}
	explicit := []string{second}
	chosen, err := reopened.startupPaths(explicit)
	if err != nil || !slices.Equal(chosen, explicit) {
		t.Fatal("explicit ROM did not override restore")
	}
	for range 30 {
		reopened.remember([]string{first})
	}
	reopened.remember([]string{second})
	reopened.close()
	latest := newDevelopmentSession(cache, true)
	paths, err = latest.startupPaths(nil)
	if err != nil || !slices.Equal(paths, []string{second}) {
		t.Fatal("writer did not retain latest snapshot", paths, err)
	}
	latest.close()
	for _, path := range []string{first, second} {
		content, err := os.ReadFile(path)
		if err != nil || string(content) != "untouched ROM" {
			t.Fatal("remembering paths wrote ROM")
		}
	}
	productionPath := filepath.Join(root, "production", "dev-session.json")
	disabled := newDevelopmentSession(productionPath, false)
	disabled.remember([]string{first})
	if paths, err := disabled.startupPaths(nil); err != nil || len(paths) != 0 {
		t.Fatal("production restored dev state")
	}
	if paths, err := disabled.startupPaths(
		explicit,
	); err != nil ||
		!slices.Equal(paths, explicit) {
		t.Fatal("production explicit paths")
	}
	disabled.close()
	if _, err := os.Stat(productionPath); !os.IsNotExist(err) {
		t.Fatal("production wrote developer cache")
	}
	if err := os.WriteFile(
		cache,
		[]byte("incomplete JSON"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	damaged := newDevelopmentSession(cache, true)
	if _, err := damaged.startupPaths(nil); err == nil {
		t.Fatal("damaged session silently accepted")
	}
	if paths, err := damaged.startupPaths(
		explicit,
	); err != nil ||
		!slices.Equal(paths, explicit) {
		t.Fatal("damaged cache prevented explicit open")
	}
	damaged.remember(explicit)
	damaged.close()
	recovered := newDevelopmentSession(cache, true)
	paths, err = recovered.startupPaths(nil)
	recovered.close()
	if err != nil || !slices.Equal(paths, explicit) {
		t.Fatal("successful open did not replace damaged cache")
	}
}
