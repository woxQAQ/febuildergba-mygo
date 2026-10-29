package game

import (
	"bytes"
	"encoding/hex"
	"errors"
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
)

func wordVariantFixture(
	t *testing.T,
	version string,
) (*testutil.FixtureBuilder, []byte) {
	t.Helper()
	f := testutil.NewScriptFixture(version)
	units := f.GetPosition("objective_units")
	f.WriteAt(loadUnitsA, f.Bytes()[units:units+36])
	f.WriteAt(loadUnitsB, f.Bytes()[units:units+36])
	var raw []byte
	for _, part := range [][2]string{
		{"320000000200010000802508", "340000000200010000802508"},
		{"3300000000802508000000000090250800802508", "3500000000802508000000000090250800802508"},
		{"0F0000000100000002000000", "0F0000000100000002000000"},
		{"120000000200000001000000", "120000000200000001000000"},
		{"3400000000902508", "3600000000902508"},
		{"3100000000802508", "3300000000802508"},
		{"350000000300000000902508", "370000000300000000902508"},
		{"3600000000802508009025080080250800902508", "3800000000802508009025080080250800902508"},
		{"0A00000000000000", "0A00000000000000"},
	} {
		text := part[0]
		if version == "FE7U" {
			text = part[1]
		}
		decoded, err := hex.DecodeString(text)
		if err != nil {
			t.Fatal(err)
		}
		raw = append(raw, decoded...)
	}
	f.WriteAt(f.GetPosition("script_root"), raw)
	return f, raw
}

func TestScriptRouteResourcesWorkflow(t *testing.T) {
	for _, version := range []string{"FE7J", "FE7U"} {
		t.Run(version, func(t *testing.T) {
			f, raw := wordVariantFixture(t, version)
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
			if !d.Complete() || !d.Relocatable() ||
				d.Instructions()[1].Address != root+12 {
				t.Fatal("hard-only filter broke instruction boundaries")
			}
			mode := sceneInstruction(t, d, ScriptLoadUnitsByRoute)
			ref, err := d.Reference(mode, ScriptUnitsHectorNormal)
			if err != nil || ref.Pointer != root+24 ||
				ref.Address != loadUnitsB {
				t.Fatalf("named deployment operand: %+v %v", ref, err)
			}
			if _, err := s.NewDeploymentDraft(0, root+8); err != nil {
				t.Fatal("hard-only deployment was not discovered", err)
			}
			if err := d.Set(0, ScriptRoute, 3); err != nil {
				t.Fatal(err)
			}
			if err := d.Set(0, ScriptHardOnly, 0); err != nil {
				t.Fatal(err)
			}
			if err := d.Set(mode, ScriptUnitsHectorNormal, 0); err != nil {
				t.Fatal(err)
			}
			talk := sceneInstruction(t, d, ScriptTextByRoute)
			if err := d.Set(talk, ScriptTextPrimary, 2); err != nil {
				t.Fatal(err)
			}
			if err := d.Set(talk, ScriptTextHector, 1); err != nil {
				t.Fatal(err)
			}
			plan, err := s.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("route resource preview wrote ROM")
			}
			if err := s.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			changed := bytes.Clone(raw)
			changed[4], changed[6], changed[36], changed[40] = 3, 0, 2, 1
			clear(changed[24:28])
			want := bytes.Clone(before.Data)
			copy(want[root:], changed)
			if !bytes.Equal(want, s.Document.Snapshot().Data) {
				t.Fatal("route edit changed another operand or reserved bytes")
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("route resource undo")
			}
			if !s.Document.Redo() {
				t.Fatal("route resource redo")
			}
			d, err = s.NewScriptDraft(0, site)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Insert(0, ScriptWait); err != nil {
				t.Fatal(err)
			}
			if err := d.Set(0, ScriptFrames, 1); err != nil {
				t.Fatal(err)
			}
			plan, err = s.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			packed := append([]byte{2, 0, 1, 0}, changed...)
			if !bytes.Equal(
				after.Data[plan.Destination():plan.Destination()+plan.Length()],
				packed,
			) ||
				!bytes.Equal(after.Data[root:root+len(raw)], changed) {
				t.Fatal(
					"route relocation changed native references or independent entry",
				)
			}
			path := filepath.Join(t.TempDir(), "routes.gba")
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
			mode = sceneInstruction(t, d, ScriptLoadUnitsByRoute)
			if _, err := d.Reference(
				mode,
				ScriptUnitsHectorNormal,
			); !errors.Is(
				err,
				ErrValue,
			) {
				t.Fatal("null route target exposed as resource", err)
			}
			ref, err = d.Reference(mode, ScriptUnitsHectorHard)
			if err != nil || ref.Pointer != plan.Destination()+32 ||
				ref.Address != loadUnitsA {
				t.Fatal("saved route target lost", err)
			}
			if _, err := s.NewDeploymentDraft(0, ref.Pointer); err != nil {
				t.Fatal("saved named target not navigable", err)
			}
		})
	}
}

func TestScriptRouteResourceFailures(t *testing.T) {
	for _, scenario := range []string{"text", "deployment", "party null"} {
		t.Run(scenario, func(t *testing.T) {
			f, _ := wordVariantFixture(t, "FE7U")
			s := openFixture(t, f)
			before := s.Document.Snapshot()
			d, err := s.NewScriptDraft(0, f.GetPosition("script_site"))
			if err != nil {
				t.Fatal(err)
			}
			op, field, value, want := ScriptTextByRoute, ScriptTextHector, int64(
				3,
			), error(
				ErrText,
			)
			switch scenario {
			case "deployment":
				op, field, value, want = ScriptLoadUnitsByRoute, ScriptUnitsHectorHard, 0x09FFFFFC, ErrDeployment
			case "party null":
				op, field, value, want = ScriptLoadPartyByRoute, ScriptUnitsHectorHard, 0, ErrDeployment
			}
			if err := d.Set(
				sceneInstruction(t, d, op),
				field,
				value,
			); err != nil {
				t.Fatal(err)
			}
			if _, err := s.PlanScriptEdit(
				d,
			); !errors.Is(err, want) ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatalf("named resource bypassed validation: %v", err)
			}
		})
	}
}
