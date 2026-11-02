//go:build integration

package integration

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
)

func editWorldMapEvent(t *testing.T, s *game.Session) {
	if !s.Events.WorldMapEventsAvailable() {
		t.Skip("no native world-map event directory")
	}
	entries, err := s.Events.ScriptEntries(event.WorldMapScripts)
	if err != nil {
		t.Skipf("world-map event directory unavailable: %v", err)
	}
	routes, err := s.WorldMap.Routes()
	if err != nil || len(routes) < 2 {
		t.Skip("no confirmed native route choices")
	}
	for _, entry := range entries {
		if entry.Err != nil || entry.Address == 0 || !entry.WorldMap {
			continue
		}
		draft, err := s.Events.NewScriptDraft(
			event.WorldMapScripts,
			entry.Pointer,
		)
		if err != nil || !draft.Complete() {
			continue
		}
		for index, row := range draft.Instructions() {
			if !row.Editable {
				continue
			}
			for _, field := range draft.Fields(index) {
				if field.Target != event.ScriptWorldMapRouteTarget {
					continue
				}
				original, err := draft.Value(index, field.ID)
				if err != nil {
					t.Fatal(err)
				}
				changed := (original + 1) % int64(len(routes))
				if err = draft.Set(index, field.ID, changed); err != nil {
					t.Fatal(err)
				}
				before := s.Document.Snapshot()
				plan, err := s.Events.PlanScriptEdit(draft)
				checkPlan(t, s, before, err)
				if plan.Relocated() {
					t.Fatal("parameter edit unexpectedly relocated")
				}
				if err = s.Events.ApplyScriptEdit(plan); err != nil {
					t.Fatal(err)
				}
				// Native route operands occupy the full argument word. All
				// other bytes, including unused header bytes, are retained.
				want := bytes.Clone(before.Data)
				binary.LittleEndian.PutUint32(
					want[row.Address+4:],
					uint32(changed),
				)
				if !bytes.Equal(want, s.Document.Snapshot().Data) {
					t.Fatal("event edit changed unrelated bytes or aliases")
				}
				reopened := historyAndReopen(t, s, before)
				fresh, err := reopened.Events.NewScriptDraft(
					event.WorldMapScripts,
					entry.Pointer,
				)
				if err != nil {
					t.Fatal(err)
				}
				value, err := fresh.Value(index, field.ID)
				if err != nil || value != changed {
					t.Fatal("event operand after reopen", value, err)
				}
				uses, complete, err := reopened.Events.WorldMapUses(
					event.ScriptWorldMapRouteTarget,
					int(changed),
				)
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, use := range uses {
					if use.Entry.Pointer == entry.Pointer {
						found = true
					}
				}
				if !found {
					t.Fatal("edited operand missing from confirmed references")
				}
				t.Logf(
					"source=%s event=%02X site=0x%X route=%d shared=%d reference_search_complete=%v",
					entry.Source,
					entry.EventID,
					entry.Pointer,
					changed,
					len(plan.Uses()),
					complete,
				)
				return
			}
		}
	}
	t.Skip("no complete world-map script with a confirmed route operand")
}
