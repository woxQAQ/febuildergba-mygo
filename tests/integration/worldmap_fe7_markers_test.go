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

func editFE7WorldMapMarkers(t *testing.T, s *game.Session) {
	if !s.Events.WorldMapEventsAvailable() ||
		!slices.Contains(
			s.Events.ScriptCommands(),
			event.ScriptWMFadeHighlights,
		) {
		t.Skip("no FE7 world-map marker workflow")
	}
	entries, err := s.Events.ScriptEntries(event.WorldMapScripts)
	if err != nil {
		t.Skipf("world-map directory unavailable: %v", err)
	}
	for _, entry := range entries {
		if !entry.WorldMap || entry.Address == 0 || entry.Err != nil {
			continue
		}
		d, err := s.Events.NewScriptDraft(event.WorldMapScripts, entry.Pointer)
		if err != nil || !d.Complete() {
			continue
		}
		highlight, dot := -1, -1
		rows := d.Instructions()
		for index, row := range rows {
			if row.Op == event.ScriptWMHighlight && highlight < 0 {
				highlight = index
			}
			if row.Op == event.ScriptWMDot && dot < 0 {
				dot = index
			}
		}
		if highlight < 0 || dot < 0 {
			continue
		}
		region, _ := d.Value(highlight, event.ScriptWMRegion)
		region = (region + 1) % 9
		delay, _ := d.Value(highlight, event.ScriptWMDelay)
		if delay == -1 {
			delay = 0
		} else {
			delay = -1
		}
		x, _ := d.Value(dot, event.ScriptWMX)
		x = max(0, (x+1)%32768)
		palette, _ := d.Value(dot, event.ScriptWMPaletteBank)
		palette = (palette + 1) % 16
		changes := []struct {
			row   int
			field string
			value int64
		}{{highlight, event.ScriptWMRegion, region}, {highlight, event.ScriptWMDelay, delay}, {dot, event.ScriptWMX, x}, {dot, event.ScriptWMPaletteBank, palette}}
		for _, change := range changes {
			if err = d.Set(change.row, change.field, change.value); err != nil {
				t.Fatal(err)
			}
		}
		before := s.Document.Snapshot()
		plan, err := s.Events.PlanScriptEdit(d)
		checkPlan(t, s, before, err)
		if plan.Relocated() {
			t.Fatal("marker parameters relocated")
		}
		if err = s.Events.ApplyScriptEdit(plan); err != nil {
			t.Fatal(err)
		}
		want := bytes.Clone(before.Data)
		binary.LittleEndian.PutUint32(
			want[rows[highlight].Address+8:],
			uint32(region),
		)
		binary.LittleEndian.PutUint32(
			want[rows[highlight].Address+12:],
			uint32(delay),
		)
		binary.LittleEndian.PutUint16(want[rows[dot].Address+8:], uint16(x))
		paletteWord := binary.LittleEndian.Uint32(want[rows[dot].Address+12:])
		binary.LittleEndian.PutUint32(
			want[rows[dot].Address+12:],
			paletteWord&^15|uint32(palette),
		)
		if !bytes.Equal(want, s.Document.Snapshot().Data) {
			t.Fatal("marker edit changed unrelated bytes")
		}
		reopened := historyAndReopen(t, s, before)
		fresh, err := reopened.Events.NewScriptDraft(
			event.WorldMapScripts,
			entry.Pointer,
		)
		if err != nil {
			t.Fatal(err)
		}
		for _, change := range changes {
			if value, err := fresh.Value(
				change.row,
				change.field,
			); err != nil ||
				value != change.value {
				t.Fatal("saved marker parameter", change, value, err)
			}
		}
		t.Logf(
			"world-map %02X: region, delay, location and palette; %d shared consumers; structural editing=%v",
			entry.EventID,
			len(plan.Uses()),
			fresh.Relocatable(),
		)
		return
	}
	t.Skip(
		"no complete world-map scene with region highlights and location dots",
	)
}
