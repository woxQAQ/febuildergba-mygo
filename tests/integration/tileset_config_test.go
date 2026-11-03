//go:build integration

package integration

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
)

func editTilesetConfigFile(t *testing.T, s *game.Session) {
	t.Helper()
	chapters := requireTable(t, s, records.Chapters)
	for _, row := range chapters.Rows {
		draft, err := s.Maps.NewTilesetDraft(row.ID)
		if err != nil {
			continue
		}
		source := draft.Config()
		content := source.Bytes()
		before := s.Document.Snapshot()
		// The terrain segment is part of the external raw-file contract.
		// Keep all existing tile words, including custom or unused values.
		content[0x2000] ^= 1
		path := filepath.Join(t.TempDir(), "tiles.mapchip_config")
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
		input, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		imported, err := source.Import(input)
		if err != nil {
			t.Fatal(err)
		}
		plan, err := s.Maps.PlanTileset(imported)
		checkPlan(t, s, before, err)
		if err := s.Maps.ApplyTileset(plan); err != nil {
			t.Fatal(err)
		}
		after := s.Document.Snapshot()
		preserved := bytes.Clone(after.Data[:len(before.Data)])
		copy(
			preserved[plan.Pointer():plan.Pointer()+4],
			before.Data[plan.Pointer():plan.Pointer()+4],
		)
		if !bytes.Equal(preserved, before.Data) {
			t.Fatal("import changed old resources or independent pointers")
		}
		reopened := historyAndReopen(t, s, before)
		next, err := reopened.Maps.NewTilesetDraft(row.ID)
		if err != nil || !bytes.Equal(next.Config().Bytes(), content) {
			t.Fatal("configuration file changed after save/reopen", err)
		}
		t.Logf(
			"chapter=%02X, raw bytes=%d, definitions=%d, confirmed uses=%d",
			row.ID,
			source.Size(),
			imported.Count(),
			len(plan.Uses()),
		)
		return
	}
	t.Skip("no readable native tile configuration")
}
