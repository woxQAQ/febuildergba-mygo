package event_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/worldmap"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/romwrite"
)

func worldMapScript(
	t *testing.T,
	s *game.Session,
	site int,
) *event.ScriptDraft {
	t.Helper()
	draft, err := s.Events.NewScriptDraft(event.WorldMapScripts, site)
	if err != nil {
		t.Fatal(err)
	}
	return draft
}

func TestWorldMapEventNavigationAndAtomicParameters(t *testing.T) {
	for _, version := range []string{"FE8U", "FE8J"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewWorldMapEventFixture(version)
			s := testutil.Open(t, f)
			directory, err := s.Events.WorldMapEvents()
			if err != nil {
				t.Fatal(err)
			}
			if len(directory) != 120 {
				t.Fatal("native directory size", len(directory))
			}
			for _, chapter := range []struct{ id, site int }{{0, f.GetPosition("wm_site")}, {1, f.GetPosition("wm_alias_site")}} {
				entries, err := s.Events.ScriptEntries(chapter.id)
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, entry := range entries {
					if entry.Pointer == chapter.site {
						found = entry.WorldMap && entry.Chapter == chapter.id
					}
				}
				if !found {
					t.Fatal(
						"chapter ID incorrectly used as world-map event index",
						chapter,
					)
				}
			}
			uses, complete, err := s.Events.WorldMapUses(
				event.ScriptWorldMapNodeTarget,
				5,
			)
			if err != nil || complete || len(uses) != 5 {
				t.Fatal(
					"shared child reachability/opaque search",
					len(uses),
					complete,
					err,
				)
			}
			d := worldMapScript(t, s, f.GetPosition("wm_site"))
			if !d.Complete() || !d.Relocatable() || len(d.Instructions()) != 9 {
				t.Fatal("confirmed script", d.Instructions())
			}
			for _, value := range []struct {
				row   int
				field string
				want  int64
			}{{3, event.ScriptWMDuration, -4}, {4, event.ScriptWMUnit, -1}, {5, event.ScriptWMX, -2}, {6, event.ScriptWMStartX, -1}, {3, event.ScriptWMFlags, 5}, {6, event.ScriptWMAcceleration, 0}} {
				got, err := d.Value(value.row, value.field)
				if err != nil || got != value.want {
					t.Fatal("native operand", value, got, err)
				}
			}
			for _, change := range []struct {
				row   int
				field string
				value int64
			}{{1, event.ScriptWMRoute, 4}, {3, event.ScriptWMFlags, 2}, {4, event.ScriptWMUnit, 6}, {5, event.ScriptWMX, -3}} {
				if err = d.Set(
					change.row,
					change.field,
					change.value,
				); err != nil {
					t.Fatal(err)
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
				t.Fatal("parameter preview")
			}
			if err = d.Set(1, event.ScriptWMRoute, 5); err != nil {
				t.Fatal(err)
			}
			if err = s.Events.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			want := bytes.Clone(before.Data)
			at := f.GetPosition("wm_script")
			want[at+12] = 4
			want[at+34] = 0x82
			want[at+48] = 6
			want[at+49] = 0
			want[at+60] = 0xFD
			if !bytes.Equal(want, s.Document.Snapshot().Data) {
				t.Fatal(
					"changed opcode, padding, reserved flags, pointer or frozen plan",
				)
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("atomic undo")
			}
			if !s.Document.Redo() ||
				!bytes.Equal(want, s.Document.Snapshot().Data) {
				t.Fatal("atomic redo")
			}
			if err = s.Events.ApplyScriptEdit(
				plan,
			); !errors.Is(
				err,
				rom.ErrStale,
			) {
				t.Fatal("replay", err)
			}
		})
	}
}

func TestWorldMapEventRelocationAndCreation(t *testing.T) {
	f := testutil.NewWorldMapEventFixture("FE8U")
	s := testutil.Open(t, f)
	d := worldMapScript(t, s, f.GetPosition("wm_site"))
	if err := d.Insert(1, event.ScriptWMOpenRoute); err != nil {
		t.Fatal(err)
	}
	if err := d.Set(1, event.ScriptWMRoute, 7); err != nil {
		t.Fatal(err)
	}
	before := s.Document.Snapshot()
	plan, err := s.Events.PlanScriptEdit(d)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Relocated() {
		t.Fatal("structural edit not relocated")
	}
	if err = s.Events.ApplyScriptEdit(plan); err != nil {
		t.Fatal(err)
	}
	want := bytes.Clone(before.Data)
	binary.LittleEndian.PutUint32(
		want[f.GetPosition("wm_site"):],
		testutil.PointerWord(len(want)),
	)
	at := f.GetPosition("wm_script")
	want = append(want, before.Data[at:at+8]...)
	want = append(want, 0x40, 0x91, 0, 0, 7, 0, 0, 0)
	want = append(want, before.Data[at+8:at+100]...)
	if !bytes.Equal(want, s.Document.Snapshot().Data) {
		t.Fatal("relocation changed shared/unknown entry, old body or child")
	}
	if !s.Document.Undo() ||
		!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
		t.Fatal("undo")
	}
	// A null native slot is a declared entry and can create an independent script.
	empty := f.GetPosition("wm_intro") + 9*4
	d = worldMapScript(t, s, empty)
	if err = d.Insert(0, event.ScriptEnd); err != nil {
		t.Fatal(err)
	}
	if err = d.Insert(0, event.ScriptWMRevealRoute); err != nil {
		t.Fatal(err)
	}
	if err = d.Set(0, event.ScriptWMRoute, 2); err != nil {
		t.Fatal(err)
	}
	plan, err = s.Events.PlanScriptEdit(d)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Events.ApplyScriptEdit(plan); err != nil {
		t.Fatal(err)
	}
	want = bytes.Clone(before.Data)
	binary.LittleEndian.PutUint32(want[empty:], testutil.PointerWord(len(want)))
	want = append(want, 0x40, 0x90, 0, 0, 2, 0, 0, 0, 0x20, 1, 0, 0)
	if !bytes.Equal(want, s.Document.Snapshot().Data) {
		t.Fatal("null entry creation")
	}
}

func TestWorldMapEventFailuresKeepDocument(t *testing.T) {
	for _, scenario := range []string{"record-overlap", "pointer-overlap", "capacity", "revision", "layout", "document", "missing-route", "zero-duration", "unknown-class", "opaque", "incomplete"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.NewWorldMapEventFixture("FE8J")
			site := f.GetPosition("wm_site")
			if scenario == "capacity" {
				full := testutil.NewFixture("BE8J01", rom.MaxSize)
				full.WriteAt(0, f.Bytes())
				f = full
			}
			if scenario == "opaque" {
				site = f.GetPosition("wm_ending_other")
			}
			if scenario == "incomplete" {
				f.WriteAt(f.GetPosition("wm_script")+96, []byte{0, 0, 0, 0})
			}
			s := testutil.Open(t, f)
			if scenario == "record-overlap" || scenario == "pointer-overlap" {
				address := f.GetPosition("wm_script")
				if scenario == "pointer-overlap" {
					address = site
				}
				if err := s.Records.UseLayout(
					records.Units,
					records.Layout{Address: address, Stride: 52, Count: 1},
				); err != nil {
					t.Fatal(err)
				}
			}
			d := worldMapScript(t, s, site)
			before, state := s.Document.Snapshot(), s.Document.State()
			var plan *event.ScriptPlan
			var err error
			expected := error(rom.ErrStale)
			switch scenario {
			case "record-overlap":
				expected = romwrite.ErrClaim
				err = d.Set(1, event.ScriptWMRoute, 3)
			case "pointer-overlap":
				expected = romwrite.ErrClaim
				err = d.Insert(0, event.ScriptWMWaitNode)
			case "capacity":
				expected = romwrite.ErrCapacity
				err = d.Insert(0, event.ScriptWMWaitNode)
			case "missing-route":
				expected = worldmap.ErrConnections
				err = d.Set(3, event.ScriptWMToNode, 20)
			case "zero-duration":
				expected = edit.ErrValue
				err = d.Set(3, event.ScriptWMDuration, 0)
			case "unknown-class":
				expected = edit.ErrValue
				err = d.Set(2, event.ScriptWMClass, 0x1234)
			case "opaque":
				expected = event.ErrScriptStructure
				err = d.Insert(0, event.ScriptWMWaitNode)
			case "incomplete":
				expected = edit.ErrField
				err = d.Set(1, event.ScriptWMRoute, 3)
			default:
				err = d.Set(1, event.ScriptWMRoute, 3)
			}
			if err == nil {
				plan, err = s.Events.PlanScriptEdit(d)
			}
			if scenario == "revision" || scenario == "layout" ||
				scenario == "document" {
				if err != nil {
					t.Fatal(err)
				}
				switch scenario {
				case "revision":
					err = s.Records.Set(records.Units, []int{1}, "level", 9)
				case "layout":
					table := s.Records.Table(records.Chapters)
					err = s.Records.UseLayout(
						records.Chapters,
						records.Layout{
							Address: table.Address,
							Stride:  table.Stride,
							Count:   1,
						},
					)
				case "document":
					s = testutil.Open(t, f)
				}
				if err != nil {
					t.Fatal(err)
				}
				before, state = s.Document.Snapshot(), s.Document.State()
				err = s.Events.ApplyScriptEdit(plan)
			}
			if !errors.Is(err, expected) {
				t.Fatal("unexpected failure", err, expected)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				state != s.Document.State() {
				t.Fatal("failure changed document/history")
			}
		})
	}
}

func TestWorldMapEventsIndependentContextAndClaims(t *testing.T) {
	t.Run("battle-map-context", func(t *testing.T) {
		f := testutil.NewWorldMapEventFixture("FE8U")
		at := f.GetPosition("wm_script")
		f.WriteAt(at, []byte{0x20, 0x27, 0, 0, 0x20, 1, 0, 0})
		s := testutil.Open(t, f)
		d, err := s.Events.NewScriptDraft(0, f.GetPosition("wm_site"))
		if err != nil {
			t.Fatal(err)
		}
		if !d.Entry().WorldMap {
			t.Fatal("lost world-map context")
		}
		// An explicit chapter map load is required before map-change interpretation.
		if _, err = s.Events.MapChanges(
			d,
			0,
		); !errors.Is(
			err,
			event.ErrMapChangeContext,
		) {
			t.Fatal("world map inherited a battle map", err)
		}
	})
	t.Run("opaque-parameters", func(t *testing.T) {
		f := testutil.NewWorldMapEventFixture("FE8U")
		s := testutil.Open(t, f)
		d := worldMapScript(t, s, f.GetPosition("wm_ending_other"))
		if !d.Complete() || d.Relocatable() {
			t.Fatal("opaque script capability")
		}
		if err := d.Set(1, event.ScriptWMRoute, 4); err != nil {
			t.Fatal(err)
		}
		before := s.Document.Snapshot()
		plan, err := s.Events.PlanScriptEdit(d)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.Events.ApplyScriptEdit(plan); err != nil {
			t.Fatal(err)
		}
		want := bytes.Clone(before.Data)
		want[f.GetPosition("wm_opaque")+8] = 4
		if !bytes.Equal(want, s.Document.Snapshot().Data) {
			t.Fatal("opaque instruction overwritten")
		}
	})
	t.Run("independent-directory", func(t *testing.T) {
		f := testutil.NewWorldMapEventFixture("FE8U")
		f.WriteU32(f.GetPosition("chapters_pointer"), 0)
		s := testutil.Open(t, f)
		d := worldMapScript(t, s, f.GetPosition("wm_site"))
		if err := d.Set(1, event.ScriptWMRoute, 3); err != nil {
			t.Fatal(err)
		}
		plan, err := s.Events.PlanScriptEdit(d)
		if err != nil {
			t.Fatal("chapter table blocked independent event", err)
		}
		if len(plan.Uses()) < 3 {
			t.Fatal("lost independent shared roots")
		}
		if err = s.Events.ApplyScriptEdit(plan); err != nil {
			t.Fatal(err)
		}
	})
	for _, position := range []string{"wm_site", "wm_script", "wm_child", "wm_beginning_root", "wm_ending"} {
		t.Run(position, func(t *testing.T) {
			f := testutil.NewWorldMapEventFixture("FE8U")
			f.WritePointer(
				f.GetPosition("worldmap_nodes_pointer"),
				f.GetPosition(position)-28*32,
			)
			s := testutil.Open(t, f)
			d, err := s.WorldMap.NewDraft(28)
			if err != nil {
				t.Fatal(err)
			}
			if err = d.Set(worldmap.X, 9); err != nil {
				t.Fatal(err)
			}
			before := s.Document.Snapshot()
			if _, err = s.WorldMap.Plan(d); !errors.Is(err, romwrite.ErrClaim) {
				t.Fatal("node write ignored world-map event claim", err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("claim failure changed bytes")
			}
		})
	}
	t.Run("relocated-directory", func(t *testing.T) {
		f := testutil.NewWorldMapEventFixture("FE8U")
		const relocated = 0x28F000
		base := f.GetPosition("wm_beginning")
		f.WriteAt(relocated, f.Bytes()[base:base+59*4])
		f.WritePointer(f.GetPosition("wm_beginning_root"), relocated)
		s := testutil.Open(t, f)
		entries, err := s.Events.ScriptEntries(0)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.ContainsFunc(
			entries,
			func(e event.ScriptEntry) bool { return e.Pointer == relocated+7*4 && e.WorldMap },
		) {
			t.Fatal("directory root was treated as a fixed table")
		}
		d := worldMapScript(t, s, relocated+7*4)
		if err = d.Insert(0, event.ScriptWMWaitNode); err != nil {
			t.Fatal(err)
		}
		plan, err := s.Events.PlanScriptEdit(d)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.Events.ApplyScriptEdit(plan); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(
			f.Bytes()[base:base+59*4],
			s.Document.Snapshot().Data[base:base+59*4],
		) {
			t.Fatal("unselected old directory changed")
		}
	})
	t.Run("shared-directory", func(t *testing.T) {
		f := testutil.NewWorldMapEventFixture("FE8U")
		f.WritePointer(
			f.GetPosition("wm_intro_root"),
			f.GetPosition("wm_beginning"),
		)
		s := testutil.Open(t, f)
		d := worldMapScript(t, s, f.GetPosition("wm_site"))
		if err := d.Insert(0, event.ScriptWMWaitNode); err != nil {
			t.Fatal(err)
		}
		plan, err := s.Events.PlanScriptEdit(d)
		if err != nil {
			t.Fatal(err)
		}
		if len(plan.Uses()) != 2 ||
			plan.Uses()[0].Source == plan.Uses()[1].Source {
			t.Fatal("physical entry lost semantic aliases", plan.Uses())
		}
		if err = s.Events.ApplyScriptEdit(plan); err != nil {
			t.Fatal(err)
		}
	})
	for _, broken := range []string{"directory", "script"} {
		t.Run("damaged-"+broken, func(t *testing.T) {
			f := testutil.NewWorldMapEventFixture("FE8U")
			at := len(f.Bytes()) - 104
			if broken == "directory" {
				f.WritePointer(f.GetPosition("wm_intro_root"), at)
			} else {
				at = f.GetPosition("wm_script")
				f.WriteAt(at, []byte{0, 0, 0, 0})
			}
			f.WritePointer(f.GetPosition("worldmap_nodes_pointer"), at-28*32)
			s := testutil.Open(t, f)
			d, err := s.WorldMap.NewDraft(28)
			if err != nil {
				t.Fatal(err)
			}
			if err = d.Set(worldmap.X, 9); err != nil {
				t.Fatal(err)
			}
			before := s.Document.Snapshot()
			if _, err = s.WorldMap.Plan(d); !errors.Is(err, romwrite.ErrClaim) {
				t.Fatal("damaged confirmed prefix was unprotected", err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("failed protected write changed bytes")
			}
		})
	}
	t.Run("limits", func(t *testing.T) {
		f := testutil.NewWorldMapEventFixture("FE8U")
		s := testutil.Open(t, f)
		d := worldMapScript(t, s, f.GetPosition("wm_site"))
		for _, bad := range []struct {
			index int
			field string
			value int64
		}{{1, event.ScriptWMRoute, 32}, {2, event.ScriptWMUnit, 7}, {2, event.ScriptWMNode, 29}, {6, event.ScriptWMDuration, -1}} {
			if err := d.Set(
				bad.index,
				bad.field,
				bad.value,
			); !errors.Is(
				err,
				edit.ErrValue,
			) {
				t.Fatal("native bound", bad, err)
			}
		}
		if d.Modified() {
			t.Fatal("rejected values changed draft")
		}
		entries, err := s.Events.ScriptEntries(event.WorldMapScripts)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.ContainsFunc(
			entries,
			func(e event.ScriptEntry) bool { return e.Source == "script_call_entry" && e.WorldMap },
		) {
			t.Fatal("missing call navigation")
		}
	})
}
