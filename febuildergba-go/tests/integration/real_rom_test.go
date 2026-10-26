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

const romDir = "../../.rom"

// Missing local ROMs are the only skipped cases in this optional suite.
func TestRealROMs(t *testing.T) {
	for _, fixture := range []struct {
		id, filename, name, compact string
	}{
		{"FE8U", "Fire Emblem - The Sacred Stones (USA, Australia).gba", "Eirika A", ""},
		{"FE8J", "Fire Emblem - Seima no Kouseki (Japan).gba", "エイリークエイリーク", "(ﾄﾞﾗｺﾞﾝﾏｽﾀｰ)"},
		{"FE7U", "Fire Emblem (USA, Australia).gba", "Eliwood A", ""},
		{"FE7J", "Fire Emblem - Rekka no Ken (Japan).gba", "リンリン", "(ﾄﾞﾗｺﾞﾝﾏｽﾀｰ)"},
		{"FE6", "Fire Emblem - Fuuin no Tsurugi (Japan).gba", "ロイロイ", "(ﾄﾞﾗｺﾞﾝﾏｽﾀｰ)"},
	} {
		t.Run(fixture.id, func(t *testing.T) {
			sourcePath, err := filepath.Abs(filepath.Join(romDir, fixture.filename))
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(sourcePath)
			if errors.Is(err, os.ErrNotExist) {
				t.Skipf("local ROM unavailable: %s", sourcePath)
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
			for _, kind := range []game.Kind{game.Items, game.Units} {
				beforeMove := s.Document.Snapshot()
				move, err := s.PlanTableMove(kind, nil)
				if err != nil {
					t.Fatal(err)
				}
				if err := s.ApplyTableMove(move); err != nil {
					t.Fatal(err)
				}
				if !s.Document.Undo() || !bytes.Equal(beforeMove.Data, s.Document.Snapshot().Data) || !s.Document.Redo() {
					t.Fatal("table move history lost bytes")
				}
				s.Refresh()
			}
			if err := s.Set(game.Units, []int{1}, "base.hp", 23); err != nil {
				t.Fatal(err)
			}
			beforeName := s.Document.Snapshot()
			edit, err := s.PlanNameEdit(game.Units, 1, fixture.name)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyNameEdit(edit); err != nil {
				t.Fatal(err)
			}
			if !s.Document.Undo() || !bytes.Equal(beforeName.Data, s.Document.Snapshot().Data) || !s.Document.Redo() {
				t.Fatal("name edit history lost bytes")
			}
			s.Refresh()
			compactID := -1
			if fixture.compact != "" {
				for _, row := range s.Table(game.Classes).Rows {
					if row.NameID != 0 && row.TextError == nil && row.Name != fixture.compact {
						compactID = row.ID
						break
					}
				}
				compactEdit, err := s.PlanNameEdit(game.Classes, compactID, fixture.compact)
				if err != nil {
					t.Fatal(err)
				}
				if err := s.ApplyNameEdit(compactEdit); err != nil {
					t.Fatal(err)
				}
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
			if reopened.Table(game.Units).Rows[0].Name != fixture.name {
				t.Fatal("saved name did not reopen")
			}
			if compactID >= 0 {
				found := false
				for _, row := range reopened.Table(game.Classes).Rows {
					if row.ID == compactID && row.Name == fixture.compact && row.TextError == nil {
						found = true
					}
				}
				if !found {
					t.Fatal("compact class name did not survive saving")
				}
			}
			for _, kind := range []game.Kind{game.Items, game.Units} {
				if reopened.Table(kind).Address != s.Table(kind).Address {
					t.Fatal("saved table did not remain relocated")
				}
			}
			continuedName := fixture.name + fixture.name
			continued, err := reopened.PlanNameEdit(game.Units, 1, continuedName)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.ApplyNameEdit(continued); err != nil {
				t.Fatal(err)
			}
			if reopened.Table(game.Units).Rows[0].Name != continuedName {
				t.Fatal("could not continue editing the saved name")
			}
		})
	}
}
