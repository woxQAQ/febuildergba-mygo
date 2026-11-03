package event_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func TestFE7WorldMapMarkersSharedEditAndHistory(t *testing.T) {
	for _, version := range []string{"FE7U", "FE7J"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewFE7WorldMapMarkersFixture(t, version)
			s := testutil.Open(t, f)
			site := f.GetPosition("fe7_wm_site")
			d := worldMapScript(t, s, site)
			wantOps := []event.ScriptOp{
				event.ScriptWMHighlight, event.ScriptWMClearHighlight,
				event.ScriptWMClearHighlights, event.ScriptWMFadeHighlights,
				event.ScriptWMDot, event.ScriptWMClearDot,
				event.ScriptWait, event.ScriptClearSkip, event.ScriptEnd,
			}
			var ops []event.ScriptOp
			for _, row := range d.Instructions() {
				ops = append(ops, row.Op)
			}
			if !d.Complete() || !d.Relocatable() ||
				!slices.Equal(ops, wantOps) {
				t.Fatal("native marker boundaries", ops)
			}
			for _, check := range []struct {
				row   int
				field string
				want  int64
			}{{0, event.ScriptWMRegion, 8}, {0, event.ScriptWMDelay, -2}, {4, event.ScriptWMMarkerSlot, 4}, {4, event.ScriptWMX, -1}, {4, event.ScriptWMY, 32767}, {4, event.ScriptWMPaletteBank, 2}, {4, event.ScriptWMDelay, -1}} {
				if value, err := d.Value(
					check.row,
					check.field,
				); err != nil ||
					value != check.want {
					t.Fatal("native operand", check, value, err)
				}
			}
			changes := []struct {
				row   int
				field string
				value int64
			}{{0, event.ScriptWMMarkerSlot, 1}, {0, event.ScriptWMRegion, 6}, {0, event.ScriptWMDelay, -3}, {1, event.ScriptWMMarkerSlot, 3}, {1, event.ScriptWMDelay, 0}, {4, event.ScriptWMMarkerSlot, 0}, {4, event.ScriptWMX, 123}, {4, event.ScriptWMY, -1}, {4, event.ScriptWMPaletteBank, 15}, {4, event.ScriptWMDelay, 0x08001234}, {5, event.ScriptWMMarkerSlot, 4}}
			for _, change := range changes {
				if err := d.Set(
					change.row,
					change.field,
					change.value,
				); err != nil {
					t.Fatal(change, err)
				}
			}
			before, state := s.Document.Snapshot(), s.Document.State()
			plan, err := s.Events.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if plan.Relocated() || len(plan.Uses()) != 3 ||
				state != s.Document.State() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("shared immutable preview", plan.Uses())
			}
			// The preview owns its bytes even if editing continues afterward.
			if err = d.Set(0, event.ScriptWMRegion, 0); err != nil {
				t.Fatal(err)
			}
			if err = s.Events.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			want := bytes.Clone(before.Data)
			highlight, clear, dot := f.GetPosition(
				"fe7_wm_highlight",
			), f.GetPosition(
				"fe7_wm_clear_highlight",
			), f.GetPosition(
				"fe7_wm_dot",
			)
			binary.LittleEndian.PutUint32(want[highlight+4:], 1)
			binary.LittleEndian.PutUint32(want[highlight+8:], 6)
			binary.LittleEndian.PutUint32(want[highlight+12:], 0xFFFFFFFD)
			binary.LittleEndian.PutUint32(want[clear+4:], 3)
			binary.LittleEndian.PutUint32(want[clear+8:], 0)
			binary.LittleEndian.PutUint32(want[dot+4:], 0)
			binary.LittleEndian.PutUint32(want[dot+8:], 0xFFFF007B)
			binary.LittleEndian.PutUint32(want[dot+12:], 0xAABBCCDF)
			binary.LittleEndian.PutUint32(want[dot+16:], 0x08001234)
			binary.LittleEndian.PutUint32(
				want[f.GetPosition("fe7_wm_clear_dot")+4:],
				4,
			)
			if !bytes.Equal(want, s.Document.Snapshot().Data) {
				t.Fatal(
					"marker edit changed opcode padding, fade operand, palette high bits or independent data",
				)
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				!s.Document.Redo() ||
				!bytes.Equal(want, s.Document.Snapshot().Data) {
				t.Fatal("atomic marker history")
			}
			if err = s.Events.ApplyScriptEdit(
				plan,
			); !errors.Is(
				err,
				rom.ErrStale,
			) {
				t.Fatal("reused marker plan", err)
			}
			path := filepath.Join(t.TempDir(), "markers.gba")
			if err = s.Document.SaveAs(path, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			fresh := worldMapScript(
				t,
				reopened,
				f.GetPosition("fe7_wm_alias_site"),
			)
			for _, change := range changes {
				if value, err := fresh.Value(
					change.row,
					change.field,
				); err != nil ||
					value != change.value {
					t.Fatal("saved shared marker", change, value, err)
				}
			}
		})
	}
}

func TestFE7WorldMapMarkersCreationAndRelocation(t *testing.T) {
	for _, version := range []string{"FE7U", "FE7J"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewFE7WorldMapMarkersFixture(t, version)
			s := testutil.Open(t, f)
			site := f.GetPosition("fe7_wm_site")
			d := worldMapScript(t, s, site)
			if err := d.Duplicate(4); err != nil {
				t.Fatal(err)
			}
			if err := d.Move(5, 0); err != nil {
				t.Fatal(err)
			}
			if err := d.Remove(0); err != nil {
				t.Fatal(err)
			}
			if err := d.Insert(2, event.ScriptWMFadeHighlights); err != nil {
				t.Fatal(err)
			}
			before := s.Document.Snapshot()
			plan, err := s.Events.PlanScriptEdit(d)
			if err != nil || !plan.Relocated() || len(plan.Uses()) != 1 {
				t.Fatal("selected marker entry relocation", err)
			}
			if err = s.Events.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			want := bytes.Clone(before.Data)
			binary.LittleEndian.PutUint32(
				want[site:],
				testutil.PointerWord(plan.Destination()),
			)
			if !bytes.Equal(want, s.Document.Snapshot().Data[:len(want)]) {
				t.Fatal(
					"marker relocation changed old code or independent aliases",
				)
			}
			fresh := worldMapScript(t, s, site)
			if !fresh.Complete() || len(fresh.Instructions()) != 10 ||
				fresh.Instructions()[2].Op != event.ScriptWMFadeHighlights ||
				!bytes.Equal(
					fresh.Instructions()[4].Raw[4:],
					[]byte{0xDE, 0xC0, 0xAD, 0xDE},
				) {
				t.Fatal("relocated native fade size or retained operand")
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				!s.Document.Redo() {
				t.Fatal("relocation history")
			}
			emptySite := f.GetPosition("fe7_wm_table") + 11*4
			empty := worldMapScript(t, s, emptySite)
			for index, op := range []event.ScriptOp{event.ScriptWMHighlight, event.ScriptWMClearHighlight, event.ScriptWMClearHighlights, event.ScriptWMFadeHighlights, event.ScriptWMDot, event.ScriptWMClearDot, event.ScriptEnd} {
				if err = empty.Insert(index, op); err != nil {
					t.Fatal(err)
				}
			}
			plan, err = s.Events.PlanScriptEdit(empty)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.Events.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			// Default words are independent of the command definitions.
			words := []uint32{
				0xB9,
				0,
				0,
				0,
				0xBA,
				0,
				0,
				0xBB,
				0xBC,
				0,
				0xBD,
				0,
				0,
				0,
				0,
				0xBE,
				0,
				0,
			}
			if version == "FE7J" {
				for _, index := range []int{0, 4, 7, 8, 10, 15} {
					words[index]++
				}
			}
			encoded := make([]byte, len(words)*4)
			for index, word := range words {
				binary.LittleEndian.PutUint32(encoded[index*4:], word)
			}
			if !bytes.Equal(
				encoded,
				s.Document.Snapshot().Data[plan.Destination():plan.Destination()+len(encoded)],
			) {
				t.Fatal("new marker scene has incorrect native bytes")
			}
			path := filepath.Join(t.TempDir(), "created.gba")
			if err = s.Document.SaveAs(path, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			if fresh := worldMapScript(
				t,
				reopened,
				emptySite,
			); !fresh.Complete() || !fresh.Relocatable() ||
				len(fresh.Instructions()) != 7 {
				t.Fatal("saved new marker scene")
			}
		})
	}
}

func TestFE7WorldMapMarkersInvalidEditsAndConflicts(t *testing.T) {
	for _, version := range []string{"FE7U", "FE7J"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewFE7WorldMapMarkersFixture(t, version)
			s := testutil.Open(t, f)
			d := worldMapScript(t, s, f.GetPosition("fe7_wm_site"))
			before, state := s.Document.Snapshot(), s.Document.State()
			for _, invalid := range []struct {
				row   int
				field string
				value int64
			}{{0, event.ScriptWMMarkerSlot, 4}, {0, event.ScriptWMMarkerSlot, -1}, {0, event.ScriptWMRegion, 9}, {1, event.ScriptWMMarkerSlot, 4}, {4, event.ScriptWMMarkerSlot, 5}, {4, event.ScriptWMX, -2}, {4, event.ScriptWMY, 32768}, {4, event.ScriptWMPaletteBank, 16}, {4, event.ScriptWMDelay, 0x80000000}, {5, event.ScriptWMMarkerSlot, 5}} {
				if err := d.Set(
					invalid.row,
					invalid.field,
					invalid.value,
				); !errors.Is(err, edit.ErrValue) ||
					d.Modified() {
					t.Fatal("invalid marker edit changed draft", invalid, err)
				}
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				state != s.Document.State() {
				t.Fatal("invalid edits changed document")
			}
			if err := s.Records.UseLayout(
				records.Units,
				records.Layout{
					Address: f.GetPosition("fe7_wm_dot"),
					Stride:  52,
					Count:   1,
				},
			); err != nil {
				t.Fatal(err)
			}
			d = worldMapScript(t, s, f.GetPosition("fe7_wm_site"))
			if err := d.Set(4, event.ScriptWMPaletteBank, 3); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Events.PlanScriptEdit(
				d,
			); !errors.Is(
				err,
				space.ErrClaim,
			) {
				t.Fatal("overlapping marker write", err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				state != s.Document.State() {
				t.Fatal("failed marker plan changed document")
			}
		})
	}
}
