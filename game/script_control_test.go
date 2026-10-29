package game

import (
	"bytes"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
)

func controlSceneFixture(
	t *testing.T,
	version string,
) (*testutil.FixtureBuilder, []byte) {
	t.Helper()
	f, _ := loadSceneFixture(t, version)
	raw, err := hex.DecodeString(strings.Join(strings.Fields(`
 4005010006000000 4005020003000000
 20061C02 21061C02 22061C02 23061C02 24061C02
 25061C02 26061C02 27061C02 28061C02 29061C02
 4005010000802508 23190000 20061200 412C0100FFFFFFFF
 220E0200 210F0103 230F0100 200F0100
 20340100 21340100 4005010014000000 25340100
 26330100 26340100 27340100 40050100FFFFFFFF
 28340100 2C340000 2B340000 2A340000
 201A0000 2413FF7F 231A0000 201B0100 201C0000 201D0000
 221B0000 20200000 20220000 20150000 21150000 251A0000 21010000
 `), ""))
	if err != nil {
		t.Fatal(err)
	}
	f.WriteAt(f.GetPosition("script_root"), raw)
	return f, raw
}

func TestScriptControlSceneWorkflow(t *testing.T) {
	for _, version := range []string{"FE8J", "FE8U"} {
		t.Run(version, func(t *testing.T) {
			f, raw := controlSceneFixture(t, version)
			s := openFixture(t, f)
			root, site := f.GetPosition(
				"script_root",
			), f.GetPosition(
				"script_site",
			)
			before := s.Document.Snapshot()
			d, err := s.NewScriptDraft(0, site)
			if err != nil {
				t.Fatal(err)
			}
			if !d.Complete() || !d.Relocatable() {
				t.Fatal("control scene was not decoded")
			}
			load := sceneInstruction(t, d, ScriptLoadUnits2)
			ref, err := d.Reference(load, ScriptUnitPointer)
			if err != nil || ref.Pointer != root+60 ||
				ref.Address != loadUnitsA {
				t.Fatalf(
					"copy through zero slot lost ownership: %+v %v",
					ref,
					err,
				)
			}
			multiply := sceneInstruction(t, d, ScriptSlotMultiply)
			for _, id := range []string{ScriptSlot, ScriptSlotA, ScriptSlotB} {
				if err := d.Set(multiply, id, 14); !errors.Is(err, ErrValue) {
					t.Fatal("out-of-range packed slot accepted", err)
				}
			}
			if d.Modified() {
				t.Fatal("invalid packed operand changed draft")
			}
			if err := d.Set(multiply, ScriptSlot, 11); err != nil {
				t.Fatal(err)
			}
			if err := d.Set(multiply, ScriptSlotA, 4); err != nil {
				t.Fatal(err)
			}
			if err := d.Set(multiply, ScriptSlotB, 13); err != nil {
				t.Fatal(err)
			}
			if err := d.Set(
				sceneInstruction(t, d, ScriptCounterSet),
				ScriptCounterValue,
				15,
			); err != nil {
				t.Fatal(err)
			}
			if err := d.Set(
				sceneInstruction(t, d, ScriptMusicFade),
				ScriptSpeed,
				15,
			); err != nil {
				t.Fatal(err)
			}
			plan, err := s.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("control preview wrote ROM")
			}
			if err := s.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			changed := bytes.Clone(raw)
			copy(changed[24:28], []byte{0x22, 6, 0x4B, 0x0D})
			changed[87] = 15
			changed[156] = 0x2F
			want := bytes.Clone(before.Data)
			copy(want[root:], changed)
			if !bytes.Equal(want, s.Document.Snapshot().Data) {
				t.Fatal(
					"packed operands changed neighboring nibbles or instructions",
				)
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("control edit undo")
			}
			if !s.Document.Redo() {
				t.Fatal("control edit redo")
			}
			d, err = s.NewScriptDraft(0, site)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Insert(0, ScriptWait); err != nil {
				t.Fatal(err)
			}
			if err := d.Set(0, ScriptFrames, 0); !errors.Is(err, ErrValue) {
				t.Fatal("non-progressing wait accepted", err)
			}
			plan, err = s.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			packed := append([]byte{0x20, 0x0E, 1, 0}, changed...)
			if !bytes.Equal(
				packed,
				after.Data[plan.Destination():plan.Destination()+plan.Length()],
			) {
				t.Fatal("new wait default or relocated control scene")
			}
			if !bytes.Equal(after.Data[root:root+len(changed)], changed) {
				t.Fatal("independent script entry changed")
			}
			if !s.Document.Undo() ||
				!bytes.Equal(want, s.Document.Snapshot().Data) {
				t.Fatal("control relocation undo")
			}
			if !s.Document.Redo() {
				t.Fatal("control relocation redo")
			}
			path := filepath.Join(t.TempDir(), "control-scene.gba")
			if err := s.Document.SaveAs(path, false); err != nil {
				t.Fatal(err)
			}
			s, err = Open(path)
			if err != nil {
				t.Fatal(err)
			}
			d, err = s.NewScriptDraft(0, site)
			if err != nil {
				t.Fatal(err)
			}
			ref, err = d.Reference(
				sceneInstruction(t, d, ScriptLoadUnits2),
				ScriptUnitPointer,
			)
			if err != nil || ref.Pointer != plan.Destination()+64 ||
				ref.Address != loadUnitsA {
				t.Fatal("saved arithmetic source was not navigable", err)
			}
		})
	}
}

func TestScriptControlPreservesOpaqueBits(t *testing.T) {
	f, raw := controlSceneFixture(t, "FE8U")
	root := f.GetPosition("script_root")
	raw[27] |= 0x80 // Undeclared high nibble in the packed slot operand.
	f.WriteAt(root, raw)
	s := openFixture(t, f)
	before := s.Document.Snapshot()
	d, err := s.NewScriptDraft(0, f.GetPosition("script_site"))
	if err != nil {
		t.Fatal(err)
	}
	if !d.Complete() || d.Relocatable() || d.Instructions()[4].Editable {
		t.Fatal("opaque packed bits were treated as writable")
	}
	if err := d.Insert(0, ScriptWait); !errors.Is(err, ErrScriptStructure) {
		t.Fatal("opaque scene moved", err)
	}
	if err := d.Set(
		sceneInstruction(t, d, ScriptCounterSet),
		ScriptCounterValue,
		4,
	); err != nil {
		t.Fatal(err)
	}
	plan, err := s.PlanScriptEdit(d)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyScriptEdit(plan); err != nil {
		t.Fatal(err)
	}
	want := bytes.Clone(before.Data)
	want[root+87] = 4
	if !bytes.Equal(want, s.Document.Snapshot().Data) {
		t.Fatal("neighbor edit altered opaque packed bits")
	}
}
