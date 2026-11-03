package event_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
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

func TestFE6WorldMapNarrationAndHistory(t *testing.T) {
	f := testutil.NewFE6WorldMapFixture(t)
	s := testutil.Open(t, f)
	directory, err := s.Events.WorldMapEvents()
	if err != nil || len(directory) != 4 {
		t.Fatal("chapter-selected entries", directory, err)
	}
	if !slices.Equal(directory[0].Chapters, []int{0, 1}) ||
		directory[1].Entry.EventID != 9 ||
		directory[2].Entry.EventID != 0 ||
		directory[3].Entry.EventID != 255 {
		t.Fatal("shared and zero/last IDs", directory)
	}
	entries, err := s.Events.ScriptEntries(1)
	if err != nil ||
		!slices.ContainsFunc(entries, func(e event.ScriptEntry) bool {
			return e.WorldMap && e.Pointer == f.GetPosition("fe6_wm_site") &&
				e.Chapter == 1
		}) {
		t.Fatal("chapter navigation", err)
	}
	d := worldMapScript(t, s, f.GetPosition("fe6_wm_site"))
	wantOps := []event.ScriptOp{
		event.ScriptWMInitScene,
		event.ScriptWMZoomTo,
		event.ScriptWMShowPortrait,
		event.ScriptWMText,
		event.ScriptWMMovePortrait,
		event.ScriptWMArrow,
		event.ScriptWMHighlight,
		event.ScriptWMDot,
		event.ScriptWMFlag,
		event.ScriptWMMapLabel,
		event.ScriptWMClearPortrait,
		event.ScriptWMClearHighlight,
		event.ScriptWMClearHighlights,
		event.ScriptWMClearDot,
		event.ScriptWMClearFlag,
		event.ScriptWMClearMapLabel,
		event.ScriptWMZoomBack,
		event.ScriptWMTextHide,
		event.ScriptWMEndScene,
		event.ScriptWait,
		event.ScriptClearSkip,
		event.ScriptEnd,
	}
	var ops []event.ScriptOp
	for _, row := range d.Instructions() {
		ops = append(ops, row.Op)
	}
	if !d.Complete() || !d.Relocatable() || !slices.Equal(ops, wantOps) {
		t.Fatal("instruction boundaries", ops)
	}
	changes := []struct {
		row   int
		field string
		value int64
	}{{1, event.ScriptWMX, 120}, {2, event.ScriptWMPortrait, 2}, {2, event.ScriptWMFaceSlot, 2}, {3, event.ScriptTextID, 2}, {4, event.ScriptWMFaceX, 200}, {5, event.ScriptWMGraphicID, 46}, {6, event.ScriptWMGraphicID, 5}, {7, event.ScriptWMPaletteBank, 3}, {8, event.ScriptWMMarkerSlot, 2}, {9, event.ScriptWMGraphicID, 4}}
	for _, change := range changes {
		if err = d.Set(change.row, change.field, change.value); err != nil {
			t.Fatal(change, err)
		}
	}
	before, state := s.Document.Snapshot(), s.Document.State()
	plan, err := s.Events.PlanScriptEdit(d)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Relocated() || len(plan.Uses()) != 3 ||
		!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
		state != s.Document.State() {
		t.Fatal("shared immutable preview", plan.Uses())
	}
	if err = d.Set(3, event.ScriptTextID, 1); err != nil {
		t.Fatal(err)
	}
	if err = s.Events.ApplyScriptEdit(plan); err != nil {
		t.Fatal(err)
	}
	want := bytes.Clone(before.Data)
	put16 := func(name string, offset int, value uint16) {
		binary.LittleEndian.PutUint16(
			want[f.GetPosition("fe6_wm_"+name)+offset:],
			value,
		)
	}
	put16("zoom", 4, 120)
	put16("face", 12, 2)
	put16("face", 4, 2)
	put16("text", 4, 2)
	put16("move_face", 8, 200)
	put16("arrow", 4, 46)
	put16("highlight", 6, 5)
	put16("dot", 12, 0xCCD3)
	put16("flag", 4, 2)
	put16("map_label", 12, 4)
	if !bytes.Equal(want, s.Document.Snapshot().Data) {
		t.Fatal("unknown operands, high halves or aliases changed")
	}
	if !s.Document.Undo() ||
		!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
		!s.Document.Redo() ||
		!bytes.Equal(want, s.Document.Snapshot().Data) {
		t.Fatal("atomic history")
	}
	if err = s.Events.ApplyScriptEdit(plan); !errors.Is(err, rom.ErrStale) {
		t.Fatal("stale plan", err)
	}
	path := filepath.Join(t.TempDir(), "scene.gba")
	if err = s.Document.SaveAs(path, false); err != nil {
		t.Fatal(err)
	}
	reopened, err := game.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	fresh := worldMapScript(t, reopened, f.GetPosition("fe6_wm_site"))
	for _, change := range changes {
		if got, e := fresh.Value(
			change.row,
			change.field,
		); e != nil ||
			got != change.value {
			t.Fatal("reopened field", change, got, e)
		}
	}
}

func TestFE6WorldMapSharedSiteRelocationAndCreation(t *testing.T) {
	f := testutil.NewFE6WorldMapFixture(t)
	s := testutil.Open(t, f)
	site := f.GetPosition("fe6_wm_site")
	d := worldMapScript(t, s, site)
	if err := d.Insert(3, event.ScriptWMTextTop); err != nil {
		t.Fatal(err)
	}
	before := s.Document.Snapshot()
	plan, err := s.Events.PlanScriptEdit(d)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Relocated() || len(plan.Uses()) != 2 ||
		plan.Uses()[0].Chapter != 0 ||
		plan.Uses()[1].Chapter != 1 {
		t.Fatal("shared pointer consumers", plan.Uses())
	}
	if err = s.Events.ApplyScriptEdit(plan); err != nil {
		t.Fatal(err)
	}
	after := s.Document.Snapshot()
	prefix := bytes.Clone(before.Data)
	binary.LittleEndian.PutUint32(
		prefix[site:],
		testutil.PointerWord(plan.Destination()),
	)
	if !bytes.Equal(prefix, after.Data[:len(prefix)]) {
		t.Fatal("independent PLIST and unknown aliases changed")
	}
	if !s.Document.Undo() ||
		!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
		!s.Document.Redo() ||
		!bytes.Equal(after.Data, s.Document.Snapshot().Data) {
		t.Fatal("relocation history")
	}
	path := filepath.Join(t.TempDir(), "relocated.gba")
	if err = s.Document.SaveAs(path, false); err != nil {
		t.Fatal(err)
	}
	s, err = game.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(worldMapScript(t, s, site).Instructions()) != 23 ||
		len(
			worldMapScript(
				t,
				s,
				f.GetPosition("fe6_wm_alias_site"),
			).Instructions(),
		) != 22 {
		t.Fatal("saved independent entries")
	}
	// Index zero is interpreted by the native loader just like other PLIST IDs.
	empty := worldMapScript(t, s, f.GetPosition("fe6_wm_empty"))
	if len(empty.Instructions()) != 0 {
		t.Fatal("empty entry")
	}
	for i, op := range []event.ScriptOp{event.ScriptWMInitScene, event.ScriptWMTextTop, event.ScriptWMText, event.ScriptWMEndScene, event.ScriptEnd} {
		if err = empty.Insert(i, op); err != nil {
			t.Fatal(err)
		}
	}
	before = s.Document.Snapshot()
	plan, err = s.Events.PlanScriptEdit(empty)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Uses()) != 1 || plan.Uses()[0].Chapter != 3 {
		t.Fatal("empty shared preview", plan.Uses())
	}
	if err = s.Events.ApplyScriptEdit(plan); err != nil {
		t.Fatal(err)
	}
	if !worldMapScript(t, s, f.GetPosition("fe6_wm_empty")).Complete() ||
		!s.Document.Undo() ||
		!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
		!s.Document.Redo() {
		t.Fatal("empty creation history")
	}
	if err = s.Document.SaveAs(
		filepath.Join(t.TempDir(), "created.gba"),
		false,
	); err != nil {
		t.Fatal(err)
	}
	saved, err := game.Open(s.Document.State().Path)
	if err != nil {
		t.Fatal(err)
	}
	if len(
		worldMapScript(t, saved, f.GetPosition("fe6_wm_empty")).Instructions(),
	) != 5 {
		t.Fatal("saved zero-index creation")
	}
}

func TestFE6WorldMapProtectsMapWrites(t *testing.T) {
	for _, mode := range []string{"shared-site", "script-payload"} {
		t.Run(mode, func(t *testing.T) {
			f := testutil.NewMapFixture("FE6")
			f.WritePointer(0x9310C, 0x289000)
			f.WriteU8(f.GetPosition("chapters")+59, 7)
			f.WritePointer(0x289000+7*4, 0x28A000)
			f.WriteAt(0x28A000, []byte{0x4D, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
			if mode == "shared-site" {
				f.WritePointer(0x9310C, f.GetPosition("map_layout_list"))
				f.WriteU8(f.GetPosition("chapters")+59, 4)
			} else {
				f.WritePointer(f.GetPosition("map_layout_pointer"), 0x28A004-16)
				f.WritePointer(0x28A004, f.GetPosition("map_layout"))
			}
			s := testutil.Open(t, f)
			d, err := s.Maps.NewDraft(0)
			if err != nil {
				t.Fatal(err)
			}
			if err = d.Stroke([]image.Point{{}}, 4); err != nil {
				t.Fatal(err)
			}
			before, state := s.Document.Snapshot(), s.Document.State()
			if _, err = s.Maps.Plan(d); !errors.Is(err, space.ErrClaim) {
				t.Fatal("script ownership", err)
			}
			if state != s.Document.State() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("map failure changed document")
			}
		})
	}
}

func TestFE6WorldMapBoundsAndFailures(t *testing.T) {
	for _, name := range []string{"text", "portrait", "stale", "layout", "capacity", "record-overlap", "map-site", "map-payload", "bad-root", "bad-entry", "bad-chapters", "broken-header", "relocated-root", "opaque", "callback", "manual-chapters"} {
		t.Run(name, func(t *testing.T) {
			f := testutil.NewFE6WorldMapFixture(t)
			site, scene := f.GetPosition(
				"fe6_wm_site",
			), f.GetPosition(
				"fe6_wm_scene",
			)
			switch name {
			case "capacity":
				full := testutil.NewFixture("AFEJ01", rom.MaxSize)
				full.WriteAt(0, f.Bytes())
				f = full
			case "map-site":
				f.WritePointer(0x18B18, f.GetPosition("fe6_wm_list"))
				f.WriteU8(f.GetPosition("chapters")+6, 7)
			case "map-payload":
				f.WritePointer(0x18B18, 0x28F000)
				f.WritePointer(0x28F000+7*4, scene)
				f.WriteU8(f.GetPosition("chapters")+6, 7)
			case "bad-root":
				f.WritePointer(f.GetPosition("fe6_wm_root"), len(f.Bytes())-4)
			case "bad-entry":
				f.WriteU32(site, 0x12345678)
			case "bad-chapters":
				f.WriteU32(f.GetPosition("chapters_pointer"), 0)
			case "broken-header":
				f.WriteU32(0x2BBCC, 0)
			case "relocated-root":
				f.WriteAt(
					0x28F000,
					f.Bytes()[f.GetPosition("fe6_wm_list"):f.GetPosition("fe6_wm_list")+1024],
				)
				f.WritePointer(f.GetPosition("fe6_wm_root"), 0x28F000)
				site = 0x28F000 + 7*4
			case "opaque":
				f.WriteU32(f.GetPosition("fe6_wm_face"), 0x54)
			case "callback":
				f.WriteU32(f.GetPosition("fe6_wm_init"), 0x4B)
			}
			s := testutil.Open(t, f)
			if name == "record-overlap" {
				if err := s.Records.UseLayout(
					records.Units,
					records.Layout{Address: scene, Stride: 48, Count: 1},
				); err != nil {
					t.Fatal(err)
				}
			}
			if name == "bad-root" || name == "bad-entry" ||
				name == "bad-chapters" {
				directory, err := s.Events.WorldMapEvents()
				if err != nil ||
					!slices.ContainsFunc(
						directory,
						func(e event.WorldMapEvent) bool { return e.Entry.Err != nil },
					) {
					t.Fatal("missing bounded diagnostic", directory, err)
				}
				return
			}
			if name == "manual-chapters" {
				if err := s.Records.UseLayout(
					records.Chapters,
					records.Layout{
						Address: f.GetPosition("chapters") + 68,
						Stride:  68,
						Count:   2,
					},
				); err != nil {
					t.Fatal(err)
				}
			}
			d := worldMapScript(t, s, site)
			row, field, value := 3, event.ScriptTextID, int64(2)
			if name == "text" {
				value = 0x7FFF
			}
			if name == "portrait" {
				row, field, value = 2, event.ScriptWMPortrait, 3
			}
			if err := d.Set(row, field, value); err != nil {
				t.Fatal(err)
			}
			if name == "capacity" || name == "map-site" {
				if err := d.Insert(0, event.ScriptWMTextTop); err != nil {
					t.Fatal(err)
				}
			}
			if name == "stale" {
				if err := s.Document.Apply(
					s.Document.Snapshot().Revision,
					"other",
					[]rom.Change{{Offset: 0x28E000, Data: []byte{1}}},
				); err != nil {
					t.Fatal(err)
				}
			}
			if name == "layout" {
				if err := s.Records.UseLayout(
					records.Chapters,
					records.Layout{
						Address: f.GetPosition("chapters"),
						Stride:  68,
						Count:   4,
					},
				); err != nil {
					t.Fatal(err)
				}
			}
			before, state := s.Document.Snapshot(), s.Document.State()
			plan, err := s.Events.PlanScriptEdit(d)
			if name == "broken-header" || name == "relocated-root" ||
				name == "opaque" ||
				name == "callback" ||
				name == "manual-chapters" {
				if err != nil {
					t.Fatal("independent source", err)
				}
				if (name == "opaque" || name == "callback") &&
					(!d.Complete() || d.Relocatable() || !errors.Is(d.Insert(0, event.ScriptWMTextTop), event.ErrScriptStructure)) {
					t.Fatal("opaque structural boundary")
				}
				if name == "callback" &&
					d.Instructions()[0].Op != event.ScriptASM {
					t.Fatal("skip callback classification")
				}
				if name == "broken-header" {
					entries, e := s.Events.ScriptEntries(0)
					if e != nil ||
						!slices.ContainsFunc(
							entries,
							func(entry event.ScriptEntry) bool { return entry.WorldMap && entry.Pointer == site },
						) {
						t.Fatal(
							"world-map hidden by chapter header",
							entries,
							e,
						)
					}
				}
				if err = s.Events.ApplyScriptEdit(plan); err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil {
				t.Fatal("invalid edit accepted")
			}
			if name == "capacity" && !errors.Is(err, space.ErrCapacity) ||
				(name == "stale" || name == "layout") &&
					!errors.Is(err, rom.ErrStale) ||
				(name == "map-site" || name == "map-payload" || name == "record-overlap") &&
					!errors.Is(err, space.ErrClaim) {
				t.Fatal("failure category", err)
			}
			if state != s.Document.State() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("failure changed document")
			}
		})
	}
	f := testutil.NewFE6WorldMapFixture(t)
	s := testutil.Open(t, f)
	d := worldMapScript(t, s, f.GetPosition("fe6_wm_site"))
	for _, v := range []struct {
		row   int
		field string
		value int64
	}{{2, event.ScriptWMFaceSlot, 4}, {4, event.ScriptWMFaceX, -2}, {5, event.ScriptWMGraphicID, 47}, {6, event.ScriptWMMarkerSlot, 2}, {6, event.ScriptWMGraphicID, 7}, {7, event.ScriptWMMarkerSlot, 2}, {8, event.ScriptWMMarkerSlot, 4}, {9, event.ScriptWMGraphicID, 35}} {
		if err := d.Set(
			v.row,
			v.field,
			v.value,
		); !errors.Is(err, edit.ErrValue) ||
			d.Modified() {
			t.Fatal("invalid parameter", v, err)
		}
	}
}
