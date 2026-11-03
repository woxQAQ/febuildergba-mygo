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

func TestFE7WorldMapNarrationAndHistory(t *testing.T) {
	for _, version := range []string{"FE7U", "FE7J"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewFE7WorldMapFixture(t, version)
			s := testutil.Open(t, f)
			directory, err := s.Events.WorldMapEvents()
			if err != nil || len(directory) != 47 {
				t.Fatal("native directory", len(directory), err)
			}
			if directory[44].Entry.EventID != 44 ||
				directory[45].Entry.WorldMap ||
				directory[46].Entry.WorldMap ||
				!slices.Equal(directory[7].Chapters, []int{0}) {
				t.Fatal("directory roles or chapter association", directory[7])
			}
			entries, err := s.Events.ScriptEntries(0)
			if err != nil ||
				!slices.ContainsFunc(entries, func(e event.ScriptEntry) bool {
					return e.Pointer == f.GetPosition("fe7_wm_site") &&
						e.WorldMap &&
						e.EventID == 7
				}) {
				t.Fatal("chapter world-map navigation", err)
			}
			d := worldMapScript(t, s, f.GetPosition("fe7_wm_site"))
			wantOps := []event.ScriptOp{
				event.ScriptWMInitScene,
				event.ScriptWMShowPortrait,
				event.ScriptWMText,
				event.ScriptWMPutSprite,
				event.ScriptWMScrollCamera,
				event.ScriptWMClearPortrait,
				event.ScriptWMTextHide,
				event.ScriptWMFadeBlack,
				event.ScriptWMEndScene,
				event.ScriptCall,
				event.ScriptWait,
				event.ScriptClearSkip,
				event.ScriptEnd,
			}
			var ops []event.ScriptOp
			for _, row := range d.Instructions() {
				ops = append(ops, row.Op)
			}
			if !d.Complete() || !d.Relocatable() ||
				!slices.Equal(ops, wantOps) {
				t.Fatal("native lengths or scene end", ops)
			}
			for _, v := range []struct {
				row   int
				field string
				want  int64
			}{{1, event.ScriptWMFaceSlot, 3}, {1, event.ScriptWMDelay, -1}, {1, event.ScriptWMFaceLayer, 1}, {3, event.ScriptWMX, -1}, {3, event.ScriptWMFaction, 3}, {4, event.ScriptWMDuration, 60}, {5, event.ScriptWMFaceSlide, 2}} {
				got, e := d.Value(v.row, v.field)
				if e != nil || got != v.want {
					t.Fatal(v, got, e)
				}
			}
			for _, v := range []struct {
				row   int
				field string
				value int64
			}{{1, event.ScriptWMPortrait, 2}, {1, event.ScriptWMFaceX, 188}, {1, event.ScriptWMFaceSlide, 2}, {1, event.ScriptWMFaceFlip, 0}, {1, event.ScriptWMFaceLayer, 3}, {1, event.ScriptWMDelay, -2}, {2, event.ScriptTextID, 2}, {3, event.ScriptWMX, 150}, {3, event.ScriptWMUnit, 4}, {3, event.ScriptWMFaction, 1}, {4, event.ScriptWMX, -1}, {4, event.ScriptWMDuration, 90}, {4, event.ScriptWMDelay, -3}, {5, event.ScriptWMFaceSlide, 0}} {
				if e := d.Set(v.row, v.field, v.value); e != nil {
					t.Fatal(v, e)
				}
			}
			before, state := s.Document.Snapshot(), s.Document.State()
			plan, err := s.Events.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if plan.Relocated() || len(plan.Uses()) < 3 ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				state != s.Document.State() {
				t.Fatal("shared immutable preview")
			}
			if err = d.Set(2, event.ScriptTextID, 1); err != nil {
				t.Fatal(err)
			}
			if err = s.Events.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			want := bytes.Clone(before.Data)
			put16 := func(at int, value uint16) { binary.LittleEndian.PutUint16(want[at:], value) }
			put32 := func(at int, value uint32) { binary.LittleEndian.PutUint32(want[at:], value) }
			face, unit, scroll := f.GetPosition(
				"fe7_wm_face",
			), f.GetPosition(
				"fe7_wm_unit",
			), f.GetPosition(
				"fe7_wm_scroll",
			)
			put32(face+8, 2)
			put16(face+12, 0xE2BC)
			put32(face+16, 0xFFFFFFFE)
			put32(f.GetPosition("fe7_wm_text")+4, 2)
			put32(unit+4, 4)
			put16(unit+8, 150)
			put32(unit+12, 0xA1012001)
			put16(scroll+4, 0xFFFF)
			put16(scroll+8, 90)
			put32(scroll+12, 0xFFFFFFFD)
			put16(f.GetPosition("fe7_wm_clear")+8, 0xD05B)
			if !bytes.Equal(want, s.Document.Snapshot().Data) {
				t.Fatal("unselected bytes, word padding or frozen plan changed")
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				!s.Document.Redo() ||
				!bytes.Equal(want, s.Document.Snapshot().Data) {
				t.Fatal("atomic history")
			}
			if err = s.Events.ApplyScriptEdit(
				plan,
			); !errors.Is(
				err,
				rom.ErrStale,
			) {
				t.Fatal("stale plan", err)
			}
			path := filepath.Join(t.TempDir(), "narration.gba")
			if err = s.Document.SaveAs(path, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			fresh := worldMapScript(t, reopened, f.GetPosition("fe7_wm_site"))
			value, err := fresh.Value(2, event.ScriptTextID)
			if err != nil || value != 2 {
				t.Fatal("reopened narration", value, err)
			}
		})
	}
}

func TestFE7WorldMapRelocationAndEmptyEntry(t *testing.T) {
	for _, version := range []string{"FE7U", "FE7J"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewFE7WorldMapFixture(t, version)
			s := testutil.Open(t, f)
			site := f.GetPosition("fe7_wm_site")
			d := worldMapScript(t, s, site)
			if err := d.Insert(3, event.ScriptWMTextTop); err != nil {
				t.Fatal(err)
			}
			before := s.Document.Snapshot()
			plan, err := s.Events.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if !plan.Relocated() || len(plan.Uses()) != 1 {
				t.Fatal("selected pointer isolation", plan.Uses())
			}
			if err = s.Events.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			want := bytes.Clone(before.Data)
			binary.LittleEndian.PutUint32(
				want[site:],
				testutil.PointerWord(plan.Destination()),
			)
			if !bytes.Equal(want, after.Data[:len(want)]) {
				t.Fatal("old code, aliases or endings changed")
			}
			fresh := worldMapScript(t, s, site)
			if len(fresh.Instructions()) != len(d.Instructions()) ||
				fresh.Instructions()[3].Op != event.ScriptWMTextTop {
				t.Fatal("relocated script")
			}
			if got := worldMapScript(
				t,
				s,
				f.GetPosition("fe7_wm_alias_site"),
			).Entry().
				Address; got != f.GetPosition(
				"fe7_wm_scene",
			) {
				t.Fatal("independent alias redirected")
			}
			path := filepath.Join(t.TempDir(), "relocated.gba")
			if err = s.Document.SaveAs(path, false); err != nil {
				t.Fatal(err)
			}
			s, err = game.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			emptySite := f.GetPosition("fe7_wm_table") + 11*4
			empty := worldMapScript(t, s, emptySite)
			for i, op := range []event.ScriptOp{event.ScriptWMInitScene, event.ScriptWMShowPortrait, event.ScriptWMText, event.ScriptWMClearPortrait, event.ScriptWMFadeBlack, event.ScriptWMEndScene, event.ScriptEnd} {
				if err = empty.Insert(i, op); err != nil {
					t.Fatal(err)
				}
			}
			plan, err = s.Events.PlanScriptEdit(empty)
			if err != nil {
				t.Fatal("empty entry plan", err)
			}
			if err = s.Events.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			if !worldMapScript(t, s, emptySite).Complete() {
				t.Fatal("empty entry creation")
			}
		})
	}
}

func TestFE7WorldMapDirectoryClaims(t *testing.T) {
	for _, scenario := range []string{"root-site", "damaged-prefix"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.NewFE7WorldMapFixture(t, "FE7U")
			site := f.GetPosition("fe7_wm_root")
			if scenario == "root-site" {
				f.WritePointer(site, site-7*4)
			} else {
				at := len(f.Bytes()) - 12
				f.WritePointer(site, at)
				site = f.GetPosition("fe7_wm_ending")
				f.WritePointer(site, at)
				f.WriteAt(at, []byte{0xB3, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0})
			}
			s := testutil.Open(t, f)
			d := worldMapScript(t, s, site)
			var err error
			if scenario == "root-site" {
				err = d.Insert(0, event.ScriptWMTextTop)
			} else {
				err = d.Set(0, event.ScriptTextID, 2)
			}
			if err != nil {
				t.Fatal(err)
			}
			before, state := s.Document.Snapshot(), s.Document.State()
			if _, err = s.Events.PlanScriptEdit(
				d,
			); !errors.Is(
				err,
				space.ErrClaim,
			) {
				t.Fatal("directory ownership", err)
			}
			if state != s.Document.State() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("claim failure changed document")
			}
		})
	}
}

func TestFE7WorldMapIndependentSourcesAndFailures(t *testing.T) {
	for _, name := range []string{"bad-text", "card", "slot", "duration", "stale", "capacity", "overlap", "opaque", "callback", "broken-chapters", "relocated-root", "broken-root"} {
		t.Run(name, func(t *testing.T) {
			f := testutil.NewFE7WorldMapFixture(t, "FE7U")
			site := f.GetPosition("fe7_wm_site")
			switch name {
			case "capacity":
				full := testutil.NewFixture("AE7E01", rom.MaxSize)
				full.WriteAt(0, f.Bytes())
				f = full
			case "opaque", "callback":
				site = f.GetPosition("fe7_wm_other_ending")
				if name == "callback" {
					f.WriteU32(f.GetPosition("fe7_wm_opaque"), 0x99)
					f.WriteU32(f.GetPosition("fe7_wm_opaque")+4, 0x08004401)
				}
			case "broken-chapters":
				f.WriteU32(f.GetPosition("chapters_pointer"), 0)
			case "relocated-root":
				f.WriteAt(
					0x28F000,
					f.Bytes()[f.GetPosition("fe7_wm_table"):f.GetPosition("fe7_wm_table")+45*4],
				)
				f.WritePointer(f.GetPosition("fe7_wm_root"), 0x28F000)
				site = 0x28F000 + 7*4
			case "broken-root":
				f.WritePointer(f.GetPosition("fe7_wm_root"), len(f.Bytes())-4)
				site = f.GetPosition("fe7_wm_ending")
			}
			s := testutil.Open(t, f)
			if name == "overlap" {
				if err := s.Records.UseLayout(
					records.Units,
					records.Layout{
						Address: f.GetPosition("fe7_wm_scene"),
						Stride:  52,
						Count:   1,
					},
				); err != nil {
					t.Fatal(err)
				}
			}
			d := worldMapScript(t, s, site)
			if name == "slot" || name == "duration" {
				row, field, value := 1, event.ScriptWMFaceSlot, int64(4)
				if name == "duration" {
					row, field, value = 4, event.ScriptWMDuration, 0
				}
				if err := d.Set(
					row,
					field,
					value,
				); !errors.Is(err, edit.ErrValue) ||
					d.Modified() {
					t.Fatal("invalid draft edit", err)
				}
				return
			}
			row, field, value := 2, event.ScriptTextID, int64(2)
			switch name {
			case "bad-text":
				value = 0x7FFF
			case "card":
				row, field, value = 1, event.ScriptWMPortrait, 3
			case "opaque", "callback":
				row = 1
			case "overlap":
				row, field, value = 4, event.ScriptWMX, 200
			}
			if err := d.Set(row, field, value); err != nil {
				t.Fatal(err)
			}
			if name == "capacity" {
				if err := d.Insert(0, event.ScriptWMTextTop); err != nil {
					t.Fatal(err)
				}
			}
			if name == "stale" {
				if err := s.Document.Apply(
					s.Document.Snapshot().Revision,
					"other",
					[]rom.Change{{Offset: 0x28D800, Data: []byte{1}}},
				); err != nil {
					t.Fatal(err)
				}
			}
			before, state := s.Document.Snapshot(), s.Document.State()
			plan, err := s.Events.PlanScriptEdit(d)
			success := name == "opaque" || name == "callback" ||
				name == "broken-chapters" ||
				name == "relocated-root" ||
				name == "broken-root"
			if success {
				if err != nil {
					t.Fatal("independent editable source", err)
				}
				if name == "callback" &&
					d.Instructions()[0].Op != event.ScriptASM {
					t.Fatal("skip callback is not opaque ASM")
				}
				if name == "opaque" || name == "callback" {
					if !d.Complete() || d.Relocatable() ||
						!errors.Is(
							d.Insert(0, event.ScriptWMTextTop),
							event.ErrScriptStructure,
						) {
						t.Fatal("opaque structural boundary")
					}
				}
				if err = s.Events.ApplyScriptEdit(plan); err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil {
				t.Fatal("invalid plan accepted")
			}
			if name == "capacity" && !errors.Is(err, space.ErrCapacity) ||
				name == "stale" && !errors.Is(err, rom.ErrStale) ||
				name == "overlap" && !errors.Is(err, space.ErrClaim) {
				t.Fatal("failure category", err)
			}
			if state != s.Document.State() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("failure mutated document")
			}
		})
	}
}
