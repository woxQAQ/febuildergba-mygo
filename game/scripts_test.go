package game

import (
	"bytes"
	"encoding/binary"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

func TestScriptParameterWorkflow(t *testing.T) {
	for _, fixture := range gameFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			f := testutil.NewScriptFixture(fixture.name)
			s := openFixture(t, f)
			before := s.Document.Snapshot()
			d, err := s.NewScriptDraft(0, f.GetPosition("script_site"))
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
			indexOf := func(op ScriptOp) int {
				for index, row := range d.Instructions() {
					if row.Op == op {
						return index
					}
				}
				t.Fatalf("missing native op %s", op)
				return -1
			}
			for _, change := range []struct {
				op    ScriptOp
				field string
				value int64
			}{{ScriptText, ScriptTextID, 2}, {ScriptWait, ScriptFrames, 9}, {ScriptFlagOn, ScriptFlagID, 0x21}} {
				if err := d.Set(
					indexOf(change.op),
					change.field,
					change.value,
				); err != nil {
					t.Fatal(err)
				}
			}
			plan, err := s.PlanScriptEdit(d)
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
			if err := d.Set(indexOf(ScriptWait), ScriptFrames, 17); err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			if len(after.Data) != len(before.Data) {
				t.Fatal("parameter patch expanded ROM")
			}
			flagOffset, waitOffset, textOffset := 2, 2, 2
			switch fixture.name {
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
			alias, err := s.NewScriptDraft(
				0,
				f.GetPosition("script_alias_site"),
			)
			if err != nil {
				t.Fatal(err)
			}
			if value, _ := alias.Value(
				indexOf(ScriptWait),
				ScriptFrames,
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
			reopened, err := Open(output)
			if err != nil {
				t.Fatal(err)
			}
			again, err := reopened.NewScriptDraft(
				0,
				f.GetPosition("script_site"),
			)
			if err != nil {
				t.Fatal(err)
			}
			if value, _ := again.Value(
				indexOf(ScriptText),
				ScriptTextID,
			); value != 2 {
				t.Fatal("script parameters after save")
			}
		})
	}
}

func TestScriptStructuralWorkflow(t *testing.T) {
	for _, fixture := range gameFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			f := testutil.NewScriptFixture(fixture.name)
			s := openFixture(t, f)
			before := s.Document.Snapshot()
			d, err := s.NewScriptDraft(0, f.GetPosition("script_site"))
			if err != nil {
				t.Fatal(err)
			}
			callOp := ScriptCall
			if fixture.name == "FE6" {
				callOp = ScriptJump
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
				ScriptPointer,
				int64(0x08000000+f.GetPosition("script_label_two")),
			); err != nil {
				t.Fatal(err)
			}
			last := len(d.Instructions()) - 1
			if err := d.Insert(last, ScriptWait); err != nil {
				t.Fatal(err)
			}
			if err := d.Set(last, ScriptFrames, 12); err != nil {
				t.Fatal(err)
			}
			if err := d.Duplicate(last); err != nil {
				t.Fatal(err)
			}
			if err := d.Remove(last + 1); err != nil {
				t.Fatal(err)
			}
			plan, err := s.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if !plan.Relocated() || len(plan.Uses()) != 2 ||
				plan.Destination() < len(before.Data) {
				t.Fatal("pointer-site relocation scope")
			}
			if err := d.Set(last, ScriptFrames, 99); err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyScriptEdit(plan); err != nil {
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
			current, err := s.NewScriptDraft(0, f.GetPosition("script_site"))
			if err != nil {
				t.Fatal(err)
			}
			labelAddress := 0
			for index, row := range current.Instructions() {
				if row.Op == ScriptLabel {
					if value, _ := current.Value(
						index,
						ScriptLabelID,
					); value == 2 {
						labelAddress = row.Address
					}
				}
			}
			if value, _ := current.Value(
				callIndex,
				ScriptPointer,
			); value != int64(
				0x08000000+labelAddress,
			) {
				t.Fatal("local absolute target not relocated")
			}
			if value, _ := current.Value(last, ScriptFrames); value != 12 {
				t.Fatal("mutable structural preview")
			}
			independent, err := s.NewScriptDraft(
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
			reopened, err := Open(output)
			if err != nil {
				t.Fatal(err)
			}
			edit, err := reopened.NewScriptDraft(
				0,
				f.GetPosition("script_site"),
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := edit.Move(last, 0); err != nil {
				t.Fatal(err)
			}
			continued, err := reopened.PlanScriptEdit(edit)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.ApplyScriptEdit(continued); err != nil {
				t.Fatal(err)
			}
			end, err := reopened.Script(0, f.GetPosition("script_site"))
			if err != nil || end.Instructions[0].Op != ScriptWait {
				t.Fatal("continued script editing")
			}
		})
	}
}

func TestOpaqueScriptParameters(t *testing.T) {
	for _, fixture := range gameFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			f := testutil.NewScriptFixture(fixture.name)
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
			switch fixture.name {
			case "FE7U", "FE7J":
				wait = []byte{2, 0, 5, 0}
				opaque = []byte{0x3C, 0, 0, 0, 1, 1, 0, 8}
				if fixture.name == "FE7U" {
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
			s := openFixture(t, f)
			before := s.Document.Snapshot()
			d, err := s.NewScriptDraft(0, f.GetPosition("script_site"))
			if err != nil {
				t.Fatal(err)
			}
			if !d.Complete() || d.Relocatable() ||
				d.Instructions()[1].Editable || d.Instructions()[1].Op != ScriptASM {
				t.Fatal("opaque script classification")
			}
			if err := d.Insert(
				0,
				ScriptWait,
			); !errors.Is(
				err,
				ErrScriptStructure,
			) {
				t.Fatal("opaque stream moved")
			}
			if err := d.Set(0, ScriptFrames, 6); err != nil {
				t.Fatal(err)
			}
			plan, err := s.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyScriptEdit(plan); err != nil {
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
			s := openFixture(t, f)
			d, err := s.NewScriptDraft(0, f.GetPosition("script_site"))
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "partial" {
				if d.Complete() || len(d.Fields(0)) != 0 ||
					!errors.Is(d.Insert(0, ScriptEnd), ErrScriptStructure) {
					t.Fatal("unknown boundary editable")
				}
				return
			}
			for index, row := range d.Instructions() {
				switch {
				case scenario == "dangling label" && row.Op == ScriptGoto:
					err = d.Set(index, ScriptLabelID, 999)
				case scenario == "bad text" && row.Op == ScriptText:
					err = d.Set(index, ScriptTextID, 3)
				case scenario == "bad call" && row.Op == ScriptCall:
					err = d.Set(index, ScriptPointer, 0x02000000)
				case scenario == "bad units" && row.Op == ScriptLoadUnits:
					err = d.Set(index, ScriptUnitPointer, 0)
				case (scenario == "foreign" || scenario == "stale" || scenario == "layout") && row.Op == ScriptWait:
					err = d.Set(index, ScriptFrames, 6)
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "foreign" {
				s = openFixture(t, f)
			}
			if scenario == "stale" {
				if err := s.Set(Chapters, []int{0}, "weather", 2); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "layout" {
				table := s.Table(Chapters)
				if err := s.UseLayout(
					Chapters,
					Layout{
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
				_, err = s.NewScriptDraft(0, 0x180)
			} else {
				_, err = s.PlanScriptEdit(d)
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
		s := openFixture(t, f)
		entries, err := s.ScriptEntries(0)
		if err != nil || len(entries) > 512 {
			t.Fatal("unbounded recursive discovery")
		}
		if !slices.ContainsFunc(
			entries,
			func(entry ScriptEntry) bool { return entry.Pointer == f.GetPosition("script_call")+4 },
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
