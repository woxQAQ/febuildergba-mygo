//go:build integration

package integration

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/internal/game"
)

const romDir = "../../.rom"

type realROMFixture struct {
	id, filename, name, compact, descriptionSuffix string
	descriptions                                   [4]uint16
	useRecord                                      int
	emptyItemDescription                           bool
}

var realROMFixtures = []realROMFixture{
	{
		id: "FE8U", filename: "Fire Emblem - The Sacred Stones (USA, Australia).gba",
		name: "Eirika A", descriptionSuffix: "\nDescription.",
		descriptions: [4]uint16{0x026E, 0x0309, 0x0404, 0x04AF}, useRecord: 0x4B,
	},
	{
		id: "FE8J", filename: "Fire Emblem - Seima no Kouseki (Japan).gba",
		name: "エイリークエイリーク", compact: "(ﾄﾞﾗｺﾞﾝﾏｽﾀｰ)", descriptionSuffix: "\nリン",
		descriptions: [4]uint16{0x01F5, 0x0290, 0x038B, 0x0437}, useRecord: 0x4B,
	},
	{
		id: "FE7U", filename: "Fire Emblem (USA, Australia).gba",
		name: "Eliwood A", descriptionSuffix: "\nDescription.",
		descriptions: [4]uint16{0x01AF, 0x022C, 0x0279, 0x0310}, useRecord: 0x4A, emptyItemDescription: true,
	},
	{
		id: "FE7J", filename: "Fire Emblem - Rekka no Ken (Japan).gba",
		name: "リンリン", compact: "(ﾄﾞﾗｺﾞﾝﾏｽﾀｰ)", descriptionSuffix: "\nリン",
		descriptions: [4]uint16{0x01B2, 0x022F, 0x027C, 0x0313}, useRecord: 0x4A, emptyItemDescription: true,
	},
	{
		id: "FE6", filename: "Fire Emblem - Fuuin no Tsurugi (Japan).gba",
		name: "ロイロイ", compact: "(ﾄﾞﾗｺﾞﾝﾏｽﾀｰ)", descriptionSuffix: "\nロイ",
		descriptions: [4]uint16{0x04B9, 0x0569, 0x05AD, 0x066E}, useRecord: 0x43,
	},
}

func openRealROM(t *testing.T, filename string) *game.Session {
	t.Helper()
	sourcePath, err := filepath.Abs(filepath.Join(romDir, filename))
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
	session, err := game.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func TestRealDescriptions(t *testing.T) {
	for _, fixture := range realROMFixtures {
		for index, sample := range []struct {
			kind  game.Kind
			field string
			id    int
			empty bool
		}{
			{game.Units, "description_id", 1, false},
			{game.Classes, "description_id", 1, false},
			{game.Items, "description_id", 1, fixture.emptyItemDescription},
			{game.Items, "use_description_id", fixture.useRecord, false},
		} {
			t.Run(fixture.id+"/"+string(sample.kind)+"/"+sample.field, func(t *testing.T) {
				s := openRealROM(t, fixture.filename)
				id := sample.id
				original, err := s.Text(sample.kind, id, sample.field)
				if err != nil || !original.Editable || original.ID != fixture.descriptions[index] || (original.Content == "") != sample.empty {
					t.Fatalf("cannot read the expected native description (record 0x%02X, text ID 0x%04X, editable=%t, empty=%t): %v", id, fixture.descriptions[index], original.Editable, original.Content == "", err)
				}
				move, err := s.PlanTableMove(sample.kind, nil)
				if err != nil {
					t.Fatal(err)
				}
				if err := s.ApplyTableMove(move); err != nil {
					t.Fatal(err)
				}
				before := s.Document.Snapshot()
				content := original.Content + fixture.descriptionSuffix
				edit, err := s.PlanTextEdit(sample.kind, id, sample.field, content)
				if err != nil {
					t.Fatal(err)
				}
				if err := s.ApplyTextEdit(edit); err != nil {
					t.Fatal(err)
				}
				value, err := s.Text(sample.kind, id, sample.field)
				if err != nil || value.Content != content || !slices.Equal(value.ControlCodes(), original.ControlCodes()) {
					t.Fatalf("description or control arguments changed unexpectedly: %v", err)
				}
				if !s.Document.Undo() || !bytes.Equal(before.Data, s.Document.Snapshot().Data) || !s.Document.Redo() {
					t.Fatal("description history lost bytes or file length")
				}
				if err := s.Document.Save(); err != nil {
					t.Fatal(err)
				}
				reopened, err := game.Open(s.Document.State().Path)
				if err != nil {
					t.Fatal(err)
				}
				saved, err := reopened.Text(sample.kind, id, sample.field)
				if err != nil || saved.Content != content || !slices.Equal(saved.ControlCodes(), original.ControlCodes()) {
					t.Fatalf("saved description did not reopen losslessly: %v", err)
				}
				continued, err := reopened.PlanTextEdit(sample.kind, id, sample.field, content+"\n")
				if err != nil {
					t.Fatal(err)
				}
				if err := reopened.ApplyTextEdit(continued); err != nil {
					t.Fatal(err)
				}
				value, err = reopened.Text(sample.kind, id, sample.field)
				if err != nil || value.Content != content+"\n" {
					t.Fatalf("continued description editing failed: %v", err)
				}
				t.Logf("record 0x%02X, text ID 0x%04X, preserved control words: %d", id, original.ID, len(original.ControlCodes()))
			})
		}
	}
}

// Missing local ROMs are the only skipped cases in this optional suite.
func TestRealROMs(t *testing.T) {
	for _, fixture := range realROMFixtures {
		t.Run(fixture.id, func(t *testing.T) {
			s := openRealROM(t, fixture.filename)
			path := s.Document.State().Path
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
			edit, err := s.PlanTextEdit(game.Units, 1, "name_id", fixture.name)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyTextEdit(edit); err != nil {
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
				compactEdit, err := s.PlanTextEdit(game.Classes, compactID, "name_id", fixture.compact)
				if err != nil {
					t.Fatal(err)
				}
				if err := s.ApplyTextEdit(compactEdit); err != nil {
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
			continued, err := reopened.PlanTextEdit(game.Units, 1, "name_id", continuedName)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.ApplyTextEdit(continued); err != nil {
				t.Fatal(err)
			}
			if reopened.Table(game.Units).Rows[0].Name != continuedName {
				t.Fatal("could not continue editing the saved name")
			}
		})
	}
}
