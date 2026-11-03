//go:build integration

package integration

import (
	"bytes"
	"encoding/binary"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
)

func editFE6WorldMapScene(t *testing.T, s *game.Session) {
	if !s.Events.WorldMapEventsAvailable() ||
		!slices.Contains(s.Events.ScriptCommands(), event.ScriptWMZoomTo) {
		t.Skip("no chapter-PLIST world-map zoom workflow")
	}
	entries, err := s.Events.ScriptEntries(event.WorldMapScripts)
	if err != nil {
		t.Skipf("world-map directory unavailable: %v", err)
	}
	portraits, err := s.Portraits.Entries()
	if err != nil {
		t.Skipf("portraits unavailable: %v", err)
	}
	texts, err := s.Texts.Entries()
	if err != nil {
		t.Skipf("texts unavailable: %v", err)
	}
	for _, entry := range entries {
		if !entry.WorldMap || entry.Address == 0 || entry.Err != nil {
			continue
		}
		d, err := s.Events.NewScriptDraft(event.WorldMapScripts, entry.Pointer)
		if err != nil || !d.Complete() {
			continue
		}
		textRow, faceRow, scrollRow := -1, -1, -1
		rows := d.Instructions()
		for index, row := range rows {
			switch row.Op {
			case event.ScriptWMText:
				textRow = index
			case event.ScriptWMShowPortrait:
				faceRow = index
			case event.ScriptWMZoomTo:
				scrollRow = index
			}
		}
		if textRow < 0 || faceRow < 0 || scrollRow < 0 {
			continue
		}
		oldText, _ := d.Value(textRow, event.ScriptTextID)
		oldFace, _ := d.Value(faceRow, event.ScriptWMPortrait)
		textID, faceID := 0, 0
		for _, candidate := range texts {
			if candidate.ID == 0 || int64(candidate.ID) == oldText ||
				candidate.Err != nil {
				continue
			}
			if _, err = s.Events.DialogueText(candidate.ID); err == nil {
				textID = int(candidate.ID)
				break
			}
		}
		for _, candidate := range portraits {
			if candidate.ID == 0 || int64(candidate.ID) == oldFace ||
				candidate.Empty ||
				candidate.ClassCard ||
				candidate.Err != nil {
				continue
			}
			source, e := s.Portraits.Read(candidate.ID)
			if e == nil && source.MainErr == nil && source.Main != nil {
				faceID = candidate.ID
				break
			}
		}
		if textID == 0 || faceID == 0 {
			continue
		}
		cameraX, _ := d.Value(scrollRow, event.ScriptWMX)
		cameraX = max(0, (cameraX+1)%32768)
		for _, change := range []struct {
			row   int
			field string
			value int64
		}{{textRow, event.ScriptTextID, int64(textID)}, {faceRow, event.ScriptWMPortrait, int64(faceID)}, {scrollRow, event.ScriptWMX, cameraX}} {
			if err = d.Set(change.row, change.field, change.value); err != nil {
				t.Fatal(err)
			}
		}
		before := s.Document.Snapshot()
		plan, err := s.Events.PlanScriptEdit(d)
		checkPlan(t, s, before, err)
		if plan.Relocated() {
			t.Fatal("parameter edit relocated")
		}
		if err = s.Events.ApplyScriptEdit(plan); err != nil {
			t.Fatal(err)
		}
		want := bytes.Clone(before.Data)
		binary.LittleEndian.PutUint32(
			want[rows[textRow].Address+4:],
			uint32(textID),
		)
		binary.LittleEndian.PutUint16(
			want[rows[faceRow].Address+12:],
			uint16(faceID),
		)
		binary.LittleEndian.PutUint16(
			want[rows[scrollRow].Address+4:],
			uint16(cameraX),
		)
		if !bytes.Equal(want, s.Document.Snapshot().Data) {
			t.Fatal("world-map operands changed unrelated bytes")
		}
		reopened := historyAndReopen(t, s, before)
		fresh, err := reopened.Events.NewScriptDraft(
			event.WorldMapScripts,
			entry.Pointer,
		)
		if err != nil {
			t.Fatal(err)
		}
		for _, change := range []struct {
			row   int
			field string
			value int64
		}{{textRow, event.ScriptTextID, int64(textID)}, {faceRow, event.ScriptWMPortrait, int64(faceID)}, {scrollRow, event.ScriptWMX, cameraX}} {
			value, e := fresh.Value(change.row, change.field)
			if e != nil || value != change.value {
				t.Fatal("reopened operand", change, value, e)
			}
		}
		t.Logf(
			"world-map %02X: narration, portrait and camera; %d shared consumers; structural editing=%v",
			entry.EventID,
			len(plan.Uses()),
			fresh.Relocatable(),
		)
		if fresh.Relocatable() {
			before = reopened.Document.Snapshot()
			if err = fresh.Insert(textRow, event.ScriptWMTextTop); err != nil {
				t.Fatal(err)
			}
			plan, err = reopened.Events.PlanScriptEdit(fresh)
			checkPlan(t, reopened, before, err)
			if !plan.Relocated() {
				t.Fatal("structure did not relocate")
			}
			if err = reopened.Events.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			prefix := bytes.Clone(before.Data)
			binary.LittleEndian.PutUint32(
				prefix[entry.Pointer:],
				uint32(plan.Destination())|0x08000000,
			)
			if !bytes.Equal(
				prefix,
				reopened.Document.Snapshot().Data[:len(prefix)],
			) {
				t.Fatal(
					"relocation changed old data or independent PLIST sites",
				)
			}
			saved := historyAndReopen(t, reopened, before)
			actual, e := saved.Events.NewScriptDraft(
				event.WorldMapScripts,
				entry.Pointer,
			)
			if e != nil || len(actual.Instructions()) != len(rows)+1 {
				t.Fatal("reopened structure", e)
			}
			t.Logf(
				"relocated scene: %d→%d commands, %d consumers of the selected PLIST site",
				len(rows),
				len(actual.Instructions()),
				len(plan.Uses()),
			)
		}
		return
	}
	t.Skip(
		"no complete world-map scene with readable narration, portrait and camera operands",
	)
}
