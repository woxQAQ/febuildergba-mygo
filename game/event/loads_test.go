package event_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/deployment"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

const (
	loadUnitsA = 0x258000
	loadUnitsB = 0x259000
)

func loadSceneFixture(
	t *testing.T,
	version string,
) (*testutil.FixtureBuilder, []byte) {
	t.Helper()
	f := testutil.NewScriptFixture(version)
	// Two independently located groups make FIFO order and literal ownership
	// observable. Native bytes do not come from the editor's definitions.
	units := f.GetPosition("objective_units")
	f.WriteAt(loadUnitsA, f.Bytes()[units:units+44])
	f.WriteAt(loadUnitsB, f.Bytes()[units:units+44])
	raw, err := hex.DecodeString(strings.Join(strings.Fields(`
  40050D0000000000 4005010000802508 21070000
  4005010000902508 20070100 22070200 412C0100FFFFFFFF
  20300000 21260100 213B0100 200E0500
  4005010078563412 22070200 422C000000000080
  20300000 201A0000 201B0100 201D0000 221B0000
  402C020000802508 20300000 22340100
  402F04000100050A 20300000 4005030001000000 20370100
  223B0000 20010000`), ""))
	if err != nil {
		t.Fatal(err)
	}
	f.WriteAt(f.GetPosition("script_root"), raw)
	return f, raw
}

func TestScriptLoadSceneWorkflow(t *testing.T) {
	for _, version := range []string{"FE8J", "FE8U"} {
		t.Run(version, func(t *testing.T) {
			f, raw := loadSceneFixture(t, version)
			s := testutil.Open(t, f)
			root, site := f.GetPosition(
				"script_root",
			), f.GetPosition(
				"script_site",
			)
			before := s.Document.Snapshot()
			d, err := s.Events.NewScriptDraft(0, site)
			if err != nil {
				t.Fatal(err)
			}
			if !d.Complete() || !d.Relocatable() {
				t.Fatal("load scene is not structurally editable")
			}
			first, second := sceneInstruction(
				t,
				d,
				event.ScriptLoadUnits2,
			), sceneInstruction(
				t,
				d,
				event.ScriptLoadUnits3,
			)
			for _, want := range []struct{ row, source, address int }{{first, root + 12, loadUnitsA}, {second, root + 24, loadUnitsB}} {
				ref, err := d.Reference(want.row, event.ScriptUnitPointer)
				if err != nil || !ref.Indirect || ref.Pointer != want.source ||
					ref.Address != want.address {
					t.Fatalf("FIFO reference: %+v %v", ref, err)
				}
			}
			groups, err := s.Events.Deployments(0)
			if err != nil {
				t.Fatal(err)
			}
			for _, source := range []int{root + 12, root + 24} {
				found := false
				for _, group := range groups.Groups {
					if group.Pointer == source {
						found = group.Err == nil && len(group.Units) == 2
					}
				}
				if !found {
					t.Fatal("indirect deployment was not discovered", source)
				}
			}
			if err := d.Set(first, event.ScriptLoadPolicy, 0); err != nil {
				t.Fatal(err)
			}
			if err := d.Set(second, event.ScriptLoadPolicy, 1); err != nil {
				t.Fatal(err)
			}
			plan, err := s.Events.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				plan.Relocated() ||
				len(plan.Uses()) < 2 {
				t.Fatal("load preview sharing/atomicity")
			}
			if err := d.Set(first, event.ScriptLoadPolicy, 2); err != nil {
				t.Fatal(err)
			}
			if err := s.Events.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			changed := bytes.Clone(raw)
			changed[38], changed[74] = 0, 1
			want := bytes.Clone(before.Data)
			copy(want[root:], changed)
			if !bytes.Equal(want, s.Document.Snapshot().Data) {
				t.Fatal("load policy bytes or immutable preview")
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("load parameter undo")
			}
			if !s.Document.Redo() ||
				!bytes.Equal(want, s.Document.Snapshot().Data) {
				t.Fatal("load parameter redo")
			}

			d, err = s.Events.NewScriptDraft(0, site)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Insert(0, event.ScriptWait); err != nil {
				t.Fatal(err)
			}
			if err := d.Set(0, event.ScriptFrames, 3); err != nil {
				t.Fatal(err)
			}
			last := len(d.Instructions()) - 1
			if err := d.Insert(last, event.ScriptLoadUnits3); err != nil {
				t.Fatal(err)
			}
			if err := d.Set(
				last,
				event.ScriptUnitPointer,
				0x08000000+loadUnitsB,
			); err != nil {
				t.Fatal(err)
			}
			if err := d.Insert(last+1, event.ScriptWaitUnits); err != nil {
				t.Fatal(err)
			}
			plan, err = s.Events.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Events.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			packed := append(
				[]byte{0x20, 0x0E, 3, 0},
				changed[:len(changed)-4]...)
			packed = append(
				packed,
				[]byte{
					0x42,
					0x2C,
					0,
					0,
					0,
					0x90,
					0x25,
					8,
					0x20,
					0x30,
					0,
					0,
					0x20,
					1,
					0,
					0,
				}...)
			after := s.Document.Snapshot()
			if !plan.Relocated() ||
				!bytes.Equal(
					after.Data[plan.Destination():plan.Destination()+plan.Length()],
					packed,
				) {
				t.Fatal("relocation changed slot markers or queue values")
			}
			preserved := bytes.Clone(after.Data[:len(want)])
			copy(preserved[site:site+4], want[site:site+4])
			if !bytes.Equal(preserved, want) {
				t.Fatal(
					"structural load edit changed independent references or resources",
				)
			}
			if !s.Document.Undo() ||
				!bytes.Equal(want, s.Document.Snapshot().Data) {
				t.Fatal("load structure undo")
			}
			if !s.Document.Redo() {
				t.Fatal("load structure redo")
			}
			path := filepath.Join(t.TempDir(), "load-scene.gba")
			if err := s.Document.SaveAs(path, false); err != nil {
				t.Fatal(err)
			}
			s, err = game.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(after.Data, s.Document.Snapshot().Data) {
				t.Fatal("saved load scene changed")
			}
			d, err = s.Events.NewScriptDraft(0, site)
			if err != nil {
				t.Fatal(err)
			}
			ref, err := d.Reference(
				sceneInstruction(t, d, event.ScriptLoadUnits2),
				event.ScriptUnitPointer,
			)
			if err != nil || ref.Pointer != plan.Destination()+16 ||
				ref.Address != loadUnitsA {
				t.Fatalf("relocated source navigation: %+v %v", ref, err)
			}
			// Editing the discovered deployment updates its SVAL owner, not FFFFFFFF
			// in the LOAD or the independent group already queued from another literal.
			unitsDraft, err := s.Deployments.NewDraft(0, ref.Pointer)
			if err != nil {
				t.Fatal(err)
			}
			if err := unitsDraft.Set(0, "deploy_level", 9); err != nil {
				t.Fatal(err)
			}
			unitsPlan, err := s.Deployments.Plan(unitsDraft)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Deployments.Apply(unitsPlan); err != nil {
				t.Fatal(err)
			}
			deployed := s.Document.Snapshot()
			old := bytes.Clone(deployed.Data[:len(after.Data)])
			copy(
				old[ref.Pointer:ref.Pointer+4],
				after.Data[ref.Pointer:ref.Pointer+4],
			)
			if !bytes.Equal(old, after.Data) {
				t.Fatal(
					"deployment edit changed another queue source or old script",
				)
			}
			if !s.Document.Undo() ||
				!bytes.Equal(after.Data, s.Document.Snapshot().Data) {
				t.Fatal("indirect deployment undo")
			}
			if !s.Document.Redo() {
				t.Fatal("indirect deployment redo")
			}
			if err := s.Document.SaveAs(path, false); err != nil {
				t.Fatal(err)
			}
			s, err = game.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			d, err = s.Events.NewScriptDraft(0, site)
			if err != nil {
				t.Fatal(err)
			}
			ref, err = d.Reference(
				sceneInstruction(t, d, event.ScriptLoadUnits2),
				event.ScriptUnitPointer,
			)
			if err != nil || ref.Address != unitsPlan.Destination() {
				t.Fatal("continued load reference after deployment save", err)
			}
			other, err := d.Reference(
				sceneInstruction(t, d, event.ScriptLoadUnits3),
				event.ScriptUnitPointer,
			)
			if err != nil || other.Address != loadUnitsB {
				t.Fatal("other queue item was rewritten")
			}
		})
	}
}

func TestScriptLoadSourceChanges(t *testing.T) {
	for _, scenario := range []string{"invalid address", "lost assignment", "lost pop", "internal address", "stale plan"} {
		t.Run(scenario, func(t *testing.T) {
			f, _ := loadSceneFixture(t, "FE8U")
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			d, err := s.Events.NewScriptDraft(0, f.GetPosition("script_site"))
			if err != nil {
				t.Fatal(err)
			}
			field, value := event.ScriptSlotValue, int64(0x09FFFFFC)
			row := 1 // The first queued deployment's SVAL, not its later LOAD.
			expected := error(deployment.ErrDeployment)
			switch scenario {
			case "lost assignment":
				field, value, expected = event.ScriptSlot, 3, event.ErrScriptStructure
			case "lost pop":
				row, field, value, expected = 5, event.ScriptSlot, 1, event.ErrScriptStructure
			case "internal address":
				value, expected = int64(
					0x08000000+f.GetPosition("script_root"),
				), event.ErrScriptStructure
			case "stale plan":
				value = int64(0x08000000 + loadUnitsB)
			}
			if err := d.Set(row, field, value); err != nil {
				t.Fatal(err)
			}
			if scenario == "internal address" {
				if err := d.Insert(0, event.ScriptWait); err != nil {
					t.Fatal(err)
				}
			}
			plan, err := s.Events.PlanScriptEdit(d)
			if scenario != "stale plan" {
				if !errors.Is(err, expected) ||
					!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
					t.Fatalf("failed source edit was not atomic: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Document.ApplyEdit(
				before.Revision,
				"external edit",
				rom.Edit{
					Size:    len(before.Data),
					Changes: []rom.Change{{Offset: 0x100, Data: []byte{0x55}}},
				},
			); err != nil {
				t.Fatal(err)
			}
			current := s.Document.Snapshot()
			if err := s.Events.ApplyScriptEdit(
				plan,
			); !errors.Is(err, rom.ErrStale) ||
				!bytes.Equal(current.Data, s.Document.Snapshot().Data) {
				t.Fatal("stale source plan was applied", err)
			}
		})
	}
}

func TestScriptLoadUnprovenSources(t *testing.T) {
	prefixes := map[string]string{
		"entry slot":                    "",
		"unknown queue":                 "4005010000802508 21070000 22070200",
		"unknown push after assignment": "4005020000802508 4005010000902508 21070000",
		"empty queue":                   "40050D0000000000 22070200",
		"counter reset":                 "40050D0000000000 4005010000802508 21070000 40050D0001000000 22070200",
		"slot zero":                     "40050D0000000000 4005000000802508 20070000 22070200",
		"query overwrites C":            "40050D0000000000 40050C0000802508 22330100 20070C00 22070200",
		"label join":                    "4005020000802508 20090100 4005020000902508 20080100",
		"ASM":                           "4005020000802508 400D000001010008",
		"call":                          "4005020000802508 400A000000102508",
		"arithmetic":                    "4005020000802508 20062202",
		"unknown command":               "4005020000802508 207F0000",
	}
	prefixes["queue overflow"] = "40050D0000000000 4005010000802508 " + strings.Repeat(
		"21070000 ",
		31, // One more than the native 30-entry event queue.
	) + "22070200"
	for name, prefix := range prefixes {
		t.Run(name, func(t *testing.T) {
			f, _ := loadSceneFixture(t, "FE8U")
			raw, err := hex.DecodeString(
				strings.Join(
					strings.Fields(prefix+" 412C0100FFFFFFFF 20010000"),
					"",
				),
			)
			if err != nil {
				t.Fatal(err)
			}
			root := f.GetPosition("script_root")
			f.WriteAt(root, raw)
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			d, err := s.Events.NewScriptDraft(0, f.GetPosition("script_site"))
			if err != nil {
				t.Fatal(err)
			}
			index := sceneInstruction(t, d, event.ScriptLoadUnits2)
			if _, err := d.Reference(
				index,
				event.ScriptUnitPointer,
			); !errors.Is(
				err,
				event.ErrScriptStructure,
			) {
				t.Fatal("unproven value treated as reference", err)
			}
			groups, err := s.Events.Deployments(0)
			if err != nil {
				t.Fatal(err)
			}
			for _, group := range groups.Groups {
				if group.Pointer >= root && group.Pointer < root+len(raw) {
					t.Fatal("unproven source exposed as editable deployment")
				}
			}
			if d.Relocatable() {
				if err := d.Insert(0, event.ScriptWait); err != nil {
					t.Fatal(err)
				}
				if _, err := s.Events.PlanScriptEdit(
					d,
				); !errors.Is(
					err,
					event.ErrScriptStructure,
				) {
					t.Fatal("dynamic load moved", err)
				}
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("failed source inference wrote ROM")
			}
			// Choosing an explicit existing deployment fixes this LOAD alone.
			d, err = s.Events.NewScriptDraft(0, f.GetPosition("script_site"))
			if err != nil {
				t.Fatal(err)
			}
			index = sceneInstruction(t, d, event.ScriptLoadUnits2)
			if err := d.Set(
				index,
				event.ScriptUnitPointer,
				0x08000000+loadUnitsA,
			); err != nil {
				t.Fatal(err)
			}
			plan, err := s.Events.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Events.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			want := bytes.Clone(before.Data)
			copy(want[root+len(raw)-8:], []byte{0, 0x80, 0x25, 8})
			if !bytes.Equal(want, s.Document.Snapshot().Data) {
				t.Fatal(
					"explicit load choice changed its source or unrelated bytes",
				)
			}
		})
	}
}
