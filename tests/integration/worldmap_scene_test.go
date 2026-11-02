//go:build integration

package integration

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
)

func editWorldMapScene(t *testing.T, s *game.Session) {
	if !s.Events.WorldMapEventsAvailable() {
		t.Skip("no native world-map event directory")
	}
	entries, err := s.Events.ScriptEntries(event.WorldMapScripts)
	if err != nil {
		t.Skipf("world-map events unavailable: %v", err)
	}
	routes, err := s.WorldMap.Routes()
	if err != nil || len(routes) == 0 {
		t.Skip("no confirmed native routes")
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
		if !entry.WorldMap || entry.Err != nil || entry.Address == 0 {
			continue
		}
		draft, err := s.Events.NewScriptDraft(
			event.WorldMapScripts,
			entry.Pointer,
		)
		if err != nil || !draft.Complete() || !draft.Relocatable() {
			continue
		}
		rows := draft.Instructions()
		textRow, faceRow := -1, -1
		length := 0
		for index, row := range rows {
			length += len(row.Raw)
			// Compound text encodings are exercised by independent synthetic fixtures.
			if row.Op == event.ScriptWMText &&
				binary.LittleEndian.Uint16(row.Raw[6:]) == 0 {
				textRow = index
			}
			if row.Op == event.ScriptWMShowPortrait {
				faceRow = index
			}
		}
		if textRow < 0 || faceRow < 0 {
			continue
		}
		direct := true
		for index := range rows {
			for _, field := range draft.Fields(index) {
				value, _ := draft.Value(index, field.ID)
				if field.Target == event.ScriptEventTarget ||
					field.Target == event.ScriptDeploymentTarget ||
					field.Target == event.ScriptMovementTarget {
					if (value < 0x080000C0 || value >= 0x0A000000) &&
						!(field.Optional && value == 0) {
						direct = false
					}
				}
				// Keep this workflow's expected byte layout independent of pointer rebasing.
				if value >= int64(0x08000000+entry.Address) &&
					value < int64(0x08000000+entry.Address+length) {
					direct = false
				}
			}
		}
		if !direct {
			continue
		}
		originalText, _ := draft.Value(textRow, event.ScriptTextID)
		originalFace, _ := draft.Value(faceRow, event.ScriptWMPortrait)
		textID, faceID := 0, 0
		for _, entry := range texts {
			if entry.ID == 0 || int64(entry.ID) == originalText ||
				entry.Err != nil {
				continue
			}
			if _, err := s.Events.DialogueText(entry.ID); err == nil {
				textID = int(entry.ID)
				break
			}
		}
		for _, entry := range portraits {
			if entry.ID == 0 || int64(entry.ID) == originalFace ||
				entry.Empty ||
				entry.ClassCard ||
				entry.Err != nil {
				continue
			}
			portrait, err := s.Portraits.Read(entry.ID)
			if err == nil && portrait.MainErr == nil && portrait.Main != nil {
				faceID = entry.ID
				break
			}
		}
		if textID == 0 || faceID == 0 {
			continue
		}
		if err = draft.Set(
			textRow,
			event.ScriptTextID,
			int64(textID),
		); err != nil {
			t.Fatal(err)
		}
		if err = draft.Set(
			faceRow,
			event.ScriptWMPortrait,
			int64(faceID),
		); err != nil {
			t.Fatal(err)
		}
		if err = draft.Insert(textRow, event.ScriptWMOpenRoute); err != nil {
			t.Fatal(err)
		}
		if err = draft.Set(
			textRow,
			event.ScriptWMRoute,
			int64(routes[0].ID),
		); err != nil {
			t.Fatal(err)
		}
		before := s.Document.Snapshot()
		plan, err := s.Events.PlanScriptEdit(draft)
		checkPlan(t, s, before, err)
		if !plan.Relocated() {
			t.Fatal("complete scene was not relocated")
		}
		for _, use := range plan.Uses() {
			if use.Pointer != entry.Pointer {
				t.Fatal("independent pointer included in scene redirect", use)
			}
		}
		if err = s.Events.ApplyScriptEdit(plan); err != nil {
			t.Fatal(err)
		}
		body := bytes.Clone(before.Data[entry.Address : entry.Address+length])
		binary.LittleEndian.PutUint16(
			body[rows[textRow].Address-entry.Address+4:],
			uint16(textID),
		)
		binary.LittleEndian.PutUint16(
			body[rows[faceRow].Address-entry.Address+6:],
			uint16(faceID),
		)
		split := rows[textRow].Address - entry.Address
		command := []byte{0x40, 0x91, 0, 0, 0, 0, 0, 0}
		binary.LittleEndian.PutUint32(command[4:], uint32(routes[0].ID))
		want := bytes.Clone(before.Data)
		binary.LittleEndian.PutUint32(
			want[entry.Pointer:],
			uint32(plan.Destination())+0x08000000,
		)
		for len(want) < plan.Destination() {
			want = append(want, 0xFF)
		}
		want = append(want, body[:split]...)
		want = append(want, command...)
		want = append(want, body[split:]...)
		if !bytes.Equal(want, s.Document.Snapshot().Data) {
			t.Fatal(
				"scene relocation changed old code, aliases, padding or resources",
			)
		}
		reopened := historyAndReopen(t, s, before)
		fresh, err := reopened.Events.NewScriptDraft(
			event.WorldMapScripts,
			entry.Pointer,
		)
		if err != nil || !fresh.Complete() || !fresh.Relocatable() {
			t.Fatal("scene after reopen", err)
		}
		if value, _ := fresh.Value(
			textRow+1,
			event.ScriptTextID,
		); value != int64(
			textID,
		) {
			t.Fatal("narration after reopen", value)
		}
		if faceRow >= textRow {
			faceRow++
		}
		if value, _ := fresh.Value(
			faceRow,
			event.ScriptWMPortrait,
		); value != int64(
			faceID,
		) {
			t.Fatal("portrait after reopen", value)
		}
		t.Logf(
			"source=%s event=%02X instructions=%d→%d text=%04X portrait=%04X shared=%d",
			entry.Source,
			entry.EventID,
			len(rows),
			len(fresh.Instructions()),
			textID,
			faceID,
			len(plan.Uses()),
		)
		return
	}
	t.Skip(
		"no relocatable world-map narration scene with readable replacement resources",
	)
}
