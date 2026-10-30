package event_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

func TestScriptParameterWorkflow(t *testing.T) {
	for _, fixture := range testutil.Games() {
		t.Run(fixture.Name, func(t *testing.T) {
			f := testutil.NewScriptFixture(fixture.Name)
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			d, err := s.Events.NewScriptDraft(0, f.GetPosition("script_site"))
			if err != nil {
				t.Fatal(err)
			}
			if !d.Complete() || !d.Relocatable() {
				t.Fatal("fixed native script not fully decoded")
			}
			rows := d.Instructions()
			rows[0].Raw[0] = 0xFF
			if d.Instructions()[0].Raw[0] == 0xFF {
				t.Fatal("instruction snapshot aliases draft")
			}
			indexOf := func(op event.ScriptOp) int {
				for index, row := range d.Instructions() {
					if row.Op == op {
						return index
					}
				}
				t.Fatalf("missing native op %s", op)
				return -1
			}
			for _, change := range []struct {
				op    event.ScriptOp
				field string
				value int64
			}{{event.ScriptText, event.ScriptTextID, 2}, {event.ScriptWait, event.ScriptFrames, 9}, {event.ScriptFlagOn, event.ScriptFlagID, 0x21}} {
				if err := d.Set(
					indexOf(change.op),
					change.field,
					change.value,
				); err != nil {
					t.Fatal(err)
				}
			}
			plan, err := s.Events.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if plan.Relocated() ||
				plan.Destination() != f.GetPosition("script_root") ||
				len(plan.Uses()) < 4 {
				t.Fatal("shared in-place preview")
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("preview mutated ROM")
			}
			if err := d.Set(
				indexOf(event.ScriptWait),
				event.ScriptFrames,
				17,
			); err != nil {
				t.Fatal(err)
			}
			if err := s.Events.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			if len(after.Data) != len(before.Data) {
				t.Fatal("parameter patch expanded ROM")
			}
			flagOffset, waitOffset, textOffset := 2, 2, 2
			switch fixture.Name {
			case "FE6":
				flagOffset, waitOffset, textOffset = 4, 4, 4
			case "FE7U", "FE7J":
				textOffset = 4
			}
			expected := bytes.Clone(before.Data)
			expected[f.GetPosition("script_flag")+flagOffset] = 0x21
			expected[f.GetPosition("script_wait")+waitOffset] = 9
			expected[f.GetPosition("script_text")+textOffset] = 2
			if !bytes.Equal(expected, after.Data) {
				t.Fatal("fixed parameter bytes or immutable preview")
			}
			alias, err := s.Events.NewScriptDraft(
				0,
				f.GetPosition("script_alias_site"),
			)
			if err != nil {
				t.Fatal(err)
			}
			if value, _ := alias.Value(
				indexOf(event.ScriptWait),
				event.ScriptFrames,
			); value != 9 {
				t.Fatal(
					"same-address alias did not observe shared parameter edit",
				)
			}
			if !s.Document.Undo() ||
				!bytes.Equal(s.Document.Snapshot().Data, before.Data) {
				t.Fatal("script undo")
			}
			if !s.Document.Redo() ||
				!bytes.Equal(s.Document.Snapshot().Data, after.Data) {
				t.Fatal("script redo")
			}
			output := filepath.Join(t.TempDir(), "script-parameters.gba")
			if err := s.Document.SaveAs(output, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(output)
			if err != nil {
				t.Fatal(err)
			}
			again, err := reopened.Events.NewScriptDraft(
				0,
				f.GetPosition("script_site"),
			)
			if err != nil {
				t.Fatal(err)
			}
			if value, _ := again.Value(
				indexOf(event.ScriptText),
				event.ScriptTextID,
			); value != 2 {
				t.Fatal("script parameters after save")
			}
		})
	}
}

func TestScriptStructuralWorkflow(t *testing.T) {
	for _, fixture := range testutil.Games() {
		t.Run(fixture.Name, func(t *testing.T) {
			f := testutil.NewScriptFixture(fixture.Name)
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			d, err := s.Events.NewScriptDraft(0, f.GetPosition("script_site"))
			if err != nil {
				t.Fatal(err)
			}
			callOp := event.ScriptCall
			if fixture.Name == "FE6" {
				callOp = event.ScriptJump
			}
			callIndex := -1
			for index, row := range d.Instructions() {
				if row.Op == callOp {
					callIndex = index
				}
			}
			if callIndex < 0 {
				t.Fatal("native script link")
			}
			if err := d.Set(
				callIndex,
				event.ScriptPointer,
				int64(0x08000000+f.GetPosition("script_label_two")),
			); err != nil {
				t.Fatal(err)
			}
			last := len(d.Instructions()) - 1
			if err := d.Insert(last, event.ScriptWait); err != nil {
				t.Fatal(err)
			}
			if err := d.Set(last, event.ScriptFrames, 12); err != nil {
				t.Fatal(err)
			}
			if err := d.Duplicate(last); err != nil {
				t.Fatal(err)
			}
			if err := d.Remove(last + 1); err != nil {
				t.Fatal(err)
			}
			plan, err := s.Events.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if !plan.Relocated() || len(plan.Uses()) != 2 ||
				plan.Destination() < len(before.Data) {
				t.Fatal("pointer-site relocation scope")
			}
			if err := d.Set(last, event.ScriptFrames, 99); err != nil {
				t.Fatal(err)
			}
			if err := s.Events.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			prefix := bytes.Clone(after.Data[:len(before.Data)])
			copy(
				prefix[f.GetPosition("script_site"):f.GetPosition("script_site")+4],
				before.Data[f.GetPosition("script_site"):f.GetPosition("script_site")+4],
			)
			if !bytes.Equal(prefix, before.Data) {
				t.Fatal("old script or unrelated pointer overwritten")
			}
			current, err := s.Events.NewScriptDraft(
				0,
				f.GetPosition("script_site"),
			)
			if err != nil {
				t.Fatal(err)
			}
			labelAddress := 0
			for index, row := range current.Instructions() {
				if row.Op == event.ScriptLabel {
					if value, _ := current.Value(
						index,
						event.ScriptLabelID,
					); value == 2 {
						labelAddress = row.Address
					}
				}
			}
			if value, _ := current.Value(
				callIndex,
				event.ScriptPointer,
			); value != int64(
				0x08000000+labelAddress,
			) {
				t.Fatal("local absolute target not relocated")
			}
			if value, _ := current.Value(
				last,
				event.ScriptFrames,
			); value != 12 {
				t.Fatal("mutable structural preview")
			}
			independent, err := s.Events.NewScriptDraft(
				0,
				f.GetPosition("script_alias_site"),
			)
			if err != nil {
				t.Fatal(err)
			}
			if independent.Entry().Address != f.GetPosition("script_root") ||
				len(independent.Instructions()) != plan.Before() {
				t.Fatal("independent pointer alias changed")
			}
			if !s.Document.Undo() ||
				!bytes.Equal(s.Document.Snapshot().Data, before.Data) {
				t.Fatal("structural undo")
			}
			if !s.Document.Redo() ||
				!bytes.Equal(s.Document.Snapshot().Data, after.Data) {
				t.Fatal("structural redo")
			}
			output := filepath.Join(t.TempDir(), "script-structure.gba")
			if err := s.Document.SaveAs(output, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(output)
			if err != nil {
				t.Fatal(err)
			}
			edit, err := reopened.Events.NewScriptDraft(
				0,
				f.GetPosition("script_site"),
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := edit.Move(last, 0); err != nil {
				t.Fatal(err)
			}
			continued, err := reopened.Events.PlanScriptEdit(edit)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.Events.ApplyScriptEdit(continued); err != nil {
				t.Fatal(err)
			}
			end, err := reopened.Events.Script(0, f.GetPosition("script_site"))
			if err != nil || end.Instructions[0].Op != event.ScriptWait {
				t.Fatal("continued script editing")
			}
		})
	}
}

func TestOpaqueScriptParameters(t *testing.T) {
	for _, fixture := range testutil.Games() {
		t.Run(fixture.Name, func(t *testing.T) {
			f := testutil.NewScriptFixture(fixture.Name)
			root := f.GetPosition("script_root")
			wait, opaque, end := []byte{
				0x20,
				0x0E,
				5,
				0,
			}, []byte{
				0x40,
				0x0D,
				0,
				0,
				1,
				1,
				0,
				8,
			}, []byte{
				0x20,
				1,
				0,
				0,
			}
			offset := 2
			switch fixture.Name {
			case "FE7U", "FE7J":
				wait = []byte{2, 0, 5, 0}
				opaque = []byte{0x3C, 0, 0, 0, 1, 1, 0, 8}
				if fixture.Name == "FE7U" {
					opaque[0] = 0x3E
				}
				end = []byte{0x0A, 0, 0, 0, 0, 0, 0, 0}
			case "FE6":
				wait = []byte{2, 0, 0, 0, 5, 0, 0, 0}
				opaque = []byte{0x17, 0, 0, 0, 1, 1, 0, 8}
				end = []byte{6, 0, 0, 0, 0, 0, 0, 0}
				offset = 4
			}
			raw := append(append(slices.Clone(wait), opaque...), end...)
			f.WriteAt(root, raw)
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			d, err := s.Events.NewScriptDraft(0, f.GetPosition("script_site"))
			if err != nil {
				t.Fatal(err)
			}
			if !d.Complete() || d.Relocatable() ||
				d.Instructions()[1].Editable || d.Instructions()[1].Op != event.ScriptASM {
				t.Fatal("opaque script classification")
			}
			if err := d.Insert(
				0,
				event.ScriptWait,
			); !errors.Is(
				err,
				event.ErrScriptStructure,
			) {
				t.Fatal("opaque stream moved")
			}
			if err := d.Set(0, event.ScriptFrames, 6); err != nil {
				t.Fatal(err)
			}
			plan, err := s.Events.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Events.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			expected := bytes.Clone(before.Data)
			expected[root+offset] = 6
			if !bytes.Equal(s.Document.Snapshot().Data, expected) {
				t.Fatal("opaque native bytes changed")
			}
		})
	}
}

func TestScriptGuards(t *testing.T) {
	for _, scenario := range []string{"foreign", "stale", "layout", "dangling label", "bad text", "bad call", "bad units", "partial", "unowned site"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.NewScriptFixture("FE8U")
			if scenario == "partial" {
				f.WriteAt(
					f.GetPosition("script_root"),
					[]byte{0xFF, 0xFF, 0xFF, 0xFF},
				)
			}
			s := testutil.Open(t, f)
			d, err := s.Events.NewScriptDraft(0, f.GetPosition("script_site"))
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "partial" {
				if d.Complete() || len(d.Fields(0)) != 0 ||
					!errors.Is(
						d.Insert(0, event.ScriptEnd),
						event.ErrScriptStructure,
					) {
					t.Fatal("unknown boundary editable")
				}
				return
			}
			for index, row := range d.Instructions() {
				switch {
				case scenario == "dangling label" && row.Op == event.ScriptGoto:
					err = d.Set(index, event.ScriptLabelID, 999)
				case scenario == "bad text" && row.Op == event.ScriptText:
					err = d.Set(index, event.ScriptTextID, 3)
				case scenario == "bad call" && row.Op == event.ScriptCall:
					err = d.Set(index, event.ScriptPointer, 0x02000000)
				case scenario == "bad units" && row.Op == event.ScriptLoadUnits:
					err = d.Set(index, event.ScriptUnitPointer, 0)
				case (scenario == "foreign" || scenario == "stale" || scenario == "layout") && row.Op == event.ScriptWait:
					err = d.Set(index, event.ScriptFrames, 6)
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "foreign" {
				s = testutil.Open(t, f)
			}
			if scenario == "stale" {
				if err := s.Records.Set(
					records.Chapters,
					[]int{0},
					"weather",
					2,
				); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "layout" {
				table := s.Records.Table(records.Chapters)
				if err := s.Records.UseLayout(
					records.Chapters,
					records.Layout{
						Address: table.Address,
						Stride:  table.Stride,
						Count:   table.Count,
					},
				); err != nil {
					t.Fatal(err)
				}
			}
			before := s.Document.Snapshot()
			if scenario == "unowned site" {
				_, err = s.Events.NewScriptDraft(0, 0x180)
			} else {
				_, err = s.Events.PlanScriptEdit(d)
			}
			if err == nil ||
				!bytes.Equal(s.Document.Snapshot().Data, before.Data) {
				t.Fatal("unsafe script operation accepted")
			}
			if (scenario == "foreign" || scenario == "stale" || scenario == "layout") &&
				!errors.Is(err, rom.ErrStale) {
				t.Fatal("missing stale identity")
			}
		})
	}
	t.Run("recursive call index", func(t *testing.T) {
		f := testutil.NewScriptFixture("FE8U")
		f.WritePointer(
			f.GetPosition("script_call")+4,
			f.GetPosition("script_root"),
		)
		s := testutil.Open(t, f)
		entries, err := s.Events.ScriptEntries(0)
		if err != nil || len(entries) > 512 {
			t.Fatal("unbounded recursive discovery")
		}
		if !slices.ContainsFunc(
			entries,
			func(entry event.ScriptEntry) bool { return entry.Pointer == f.GetPosition("script_call")+4 },
		) {
			t.Fatal("call operand not exposed as entry")
		}
		raw := s.Document.Snapshot().Data
		if binary.LittleEndian.Uint32(
			raw[f.GetPosition("script_call")+4:],
		) != uint32(
			0x08000000+f.GetPosition("script_root"),
		) {
			t.Fatal("discovery changed bytes")
		}
	})
}
