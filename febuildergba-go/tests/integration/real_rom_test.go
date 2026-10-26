//go:build integration

package integration

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/internal/game"
)

const romDir = "../../../.rom"

// Missing local ROMs are the only skipped cases in this optional suite.
func TestRealROMs(t *testing.T) {
	for _, fixture := range []struct {
		id, filename string
	}{
		{"FE8U", "Fire Emblem - The Sacred Stones (USA, Australia).gba"},
		{"FE8J", "Fire Emblem - Seima no Kouseki (Japan).gba"},
		{"FE6", "Fire Emblem - Fuuin no Tsurugi (Japan).gba"},
	} {
		t.Run(fixture.id, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(romDir, fixture.filename))
			if errors.Is(err, os.ErrNotExist) {
				t.Skip("local ROM unavailable")
			}
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "copy.gba")
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			s, err := game.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			if s.Adapter.ID() != fixture.id {
				t.Fatalf("identified as %q", s.Adapter.ID())
			}
			for _, kind := range game.Kinds {
				table := s.Table(kind)
				if table == nil || table.Err != nil || table.Count == 0 {
					t.Fatalf("cannot read %s", kind)
				}
			}
			move, err := s.PlanTableMove(game.Items, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyTableMove(move); err != nil {
				t.Fatal(err)
			}
			if !s.Document.Undo() || !bytes.Equal(data, s.Document.Snapshot().Data) || !s.Document.Redo() {
				t.Fatal("table move history lost bytes")
			}
			s.Refresh()
			if err := s.Set(game.Units, []int{1}, "base.hp", 23); err != nil {
				t.Fatal(err)
			}
			if err := s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			if value, err := reopened.Value(game.Units, 1, "base.hp"); err != nil || value != 23 {
				t.Fatalf("saved HP %d: %v", value, err)
			}
			if reopened.Table(game.Items).Address != s.Table(game.Items).Address {
				t.Fatal("saved table did not remain relocated")
			}
		})
	}
}
