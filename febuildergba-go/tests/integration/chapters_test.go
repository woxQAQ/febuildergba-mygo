//go:build integration

package integration

import (
	"bytes"
	"encoding/binary"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
)

func TestRealChapters(t *testing.T) {
	samples := map[string]struct {
		stride, count, name, objective, events, weather, music, musicWidth int
	}{
		"FE8U": {148, 0x4F, 112, 136, 116, 18, 22, 2},
		"FE8J": {148, 0x4F, 112, 136, 116, 18, 22, 2},
		"FE7U": {152, 0x43, 116, 140, 120, 18, 22, 2},
		"FE7J": {148, 0x45, 112, 136, 116, 18, 22, 2},
		"FE6":  {68, 0x2D, 56, 48, 58, 17, 20, 1},
	}
	for _, fixture := range realROMFixtures {
		t.Run(fixture.id, func(t *testing.T) {
			sample := samples[fixture.id]
			s := openRealROM(t, fixture.filename)
			table := s.Table(game.Chapters)
			if table == nil || table.Err != nil || table.Stride != sample.stride || table.Count != sample.count {
				t.Fatalf("native chapter layout: %+v", table)
			}
			const id = 1
			row := table.Rows[id]
			if row.ID != id || row.TextError != nil || row.NameID == 0 || row.Name == "" {
				t.Fatalf("cannot read fixed chapter %02X: %+v", id, row)
			}
			original := s.Document.Snapshot()
			if row.NameID != binary.LittleEndian.Uint16(original.Data[row.Address+sample.name:]) {
				t.Fatal("chapter title was read from the wrong native offset")
			}
			objective := binary.LittleEndian.Uint16(original.Data[row.Address+sample.objective:])
			if value, err := s.Value(game.Chapters, id, "objective_text_id"); err != nil || value != int64(objective) {
				t.Fatalf("native objective text ID: %d, %v", value, err)
			}
			move, err := s.PlanTableMove(game.Chapters, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyTableMove(move); err != nil {
				t.Fatal(err)
			}
			if !s.Document.Undo() || !bytes.Equal(original.Data, s.Document.Snapshot().Data) || !s.Document.Redo() {
				t.Fatal("chapter relocation did not preserve complete history")
			}
			s.Refresh()
			table = s.Table(game.Chapters)
			row = table.Rows[id]
			before := s.Document.Snapshot()
			base := table.Address
			events := before.Data[base+sample.events] // Fixed chapter 0 has an existing event ID.
			mapID := before.Data[base+8]
			music := int64(0x101)
			if sample.musicWidth == 1 {
				music = 0x21
			}
			expected := bytes.Clone(before.Data)
			for _, edit := range []struct {
				field         string
				offset, width int
				value         int64
			}{
				{"weather", sample.weather, 1, 2},
				{"fog_level", 12, 1, 3},
				{"player_music_id", sample.music, sample.musicWidth, music},
				{"chapter_events_id", sample.events, 1, int64(events)},
				{"map_id", 8, 1, int64(mapID)},
			} {
				if err := s.Set(game.Chapters, []int{id}, edit.field, edit.value); err != nil {
					t.Fatal(err)
				}
				for i := range edit.width {
					expected[row.Address+edit.offset+i] = byte(edit.value >> (8 * i))
				}
			}
			if !bytes.Equal(expected, s.Document.Snapshot().Data) {
				t.Fatal("chapter edits changed native bytes outside the fixed field offsets")
			}
			text, err := s.TextByID(row.NameID)
			if err != nil || !text.Editable {
				t.Fatalf("fixed chapter title cannot be edited through its text ID: %v, %v", err, text.ReadOnlyReason)
			}
			content := text.Content + fixture.descriptionSuffix
			preview, err := s.PlanTextIDEdit(row.NameID, content)
			if err != nil || !slices.Contains(preview.Uses(), game.TextUse{Kind: game.Chapters, ID: id, Field: "name_id"}) {
				t.Fatalf("chapter was missing from the linked-text preview: %v", err)
			}
			if err := s.ApplyTextEdit(preview); err != nil {
				t.Fatal(err)
			}
			if !s.Document.Undo() || !bytes.Equal(expected, s.Document.Snapshot().Data) || !s.Document.Redo() {
				t.Fatal("chapter text history lost bytes or file length")
			}
			if err := s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(s.Document.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			if reopened.Table(game.Chapters).Address != table.Address || reopened.Table(game.Chapters).Count != sample.count {
				t.Fatal("saved chapter table did not remain relocated")
			}
			if value, err := reopened.Value(game.Chapters, id, "player_music_id"); err != nil || value != music {
				t.Fatalf("saved chapter music: %d, %v", value, err)
			}
			text, err = reopened.TextByID(row.NameID)
			if err != nil || text.Content != content {
				t.Fatalf("saved chapter title did not reopen: %v", err)
			}
			if err := reopened.Set(game.Chapters, []int{id}, "fog_level", 5); err != nil {
				t.Fatal(err)
			}
			t.Logf("chapter %02X (%s), %d records × %d bytes, title text %04X, %d confirmed move references", id, row.Name, sample.count, sample.stride, row.NameID, move.ReferenceCount())
		})
	}
}
