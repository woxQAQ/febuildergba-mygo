package game

import (
	"bytes"
	"errors"
	"testing"
)

// Independent native fragments extend the conversation/recruitment fixture with
// focus effects and a conditional exit beyond its first terminator.
func cameraSceneParts(version string) (before, after, tail string) {
	switch version {
	case "FE8J", "FE8U":
		return `20330200 4005020000000000 400C09000C000200
			22330200 400C09000C000200 23330200 25330100
			2026050A 2826050A 21260100 29260100
			203B050A 213B0100 243B050A 253B0100 223B0000`,
			`20330000 4005020000000000 400C09000C000200
			2233FDFF 400C09000C000200 2333FEFF 2533FFFF
			2026FF0A 2826050A 21260200 29260100
			203B050A 213BFEFF 243B050A 253B0100 223B0000`,
			`20080900 223B0000 20010000`
	case "FE7J":
		return `440000000900000002000000 450000000900000002000000
			1D00050A 2000050A 1E000100
			6400000005000A00 6500000001000000 6600000005000A00 67000000`,
			`440000000900000003000000 450000000900000002000000
			1D00FF0A 2000050A 1E000200
			6400000005000A00 6500000002000000 6600000005000A00 67000000`,
			`4200000009000000 67000000 0A00000000000000`
	case "FE7U":
		return `460000000900000002000000 470000000900000002000000
			1F00050A 2200050A 20000100
			6600000005000A00 6700000001000000 6800000005000A00 69000000`,
			`460000000900000003000000 470000000900000002000000
			1F00FF0A 2200050A 20000200
			6600000005000A00 6700000002000000 6800000005000A00 69000000`,
			`4400000009000000 69000000 0A00000000000000`
	case "FE6":
		return `1D0000000900000002000000 1E0000000900000002000000
			0B00000005000A00 0C00000001000000
			2C00000005000A00 2D00000001000000 2E00000005000A00 2F000000`,
			`1D0000000900000003000000 1E0000000900000002000000
			0B000000FFFF0A00 0C00000002000000
			2C00000005000A00 2D00000002000000 2E00000005000A00 2F000000`,
			`1B00000009000000 2F000000 0600000000000000`
	}
	panic("unsupported scene version")
}

func TestScriptCameraConditionsAndStructure(t *testing.T) {
	for _, fixture := range gameFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			f, original, _ := sceneFixture(t, fixture.name)
			s := openFixture(t, f)
			site := f.GetPosition("script_site")
			before := s.Document.Snapshot()
			d, err := s.NewScriptDraft(0, site)
			if err != nil {
				t.Fatal(err)
			}
			camera := sceneInstruction(t, d, ScriptCameraPosition)
			for _, value := range []int64{-0x8001, 0x10000} {
				if err := d.Set(
					camera,
					ScriptX,
					value,
				); !errors.Is(
					err,
					ErrValue,
				) {
					t.Fatalf("coordinate range: %v", err)
				}
			}
			if d.Modified() {
				t.Fatal("invalid coordinates modified draft")
			}
			// The conditional exit must remain visible past the first end.
			label := sceneInstruction(t, d, ScriptLabel)
			if err := d.Remove(label); err != nil {
				t.Fatal(err)
			}
			if _, err := s.PlanScriptEdit(d); !errors.Is(err, ErrScript) {
				t.Fatalf("deleting a referenced label: %v", err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("invalid branch plan wrote document")
			}
			d, err = s.NewScriptDraft(0, site)
			if err != nil {
				t.Fatal(err)
			}
			branchOp := ScriptBranchNoAliveAlly
			conditionLength := 12
			cameraRaw := []byte{0x0C, 0, 0, 0, 4, 0, 0, 0}
			switch fixture.name {
			case "FE8J", "FE8U":
				branchOp, conditionLength = ScriptCheckUnitExists, 4
				cameraRaw = []byte{0x21, 0x26, 4, 0}
			case "FE7J":
				cameraRaw = []byte{0x1E, 0, 4, 0}
			case "FE7U":
				cameraRaw = []byte{0x20, 0, 4, 0}
			}
			if err := d.Insert(0, branchOp); err != nil {
				t.Fatal(err)
			}
			if err := d.Set(0, ScriptCharacter, 2); err != nil {
				t.Fatal(err)
			}
			if branchOp == ScriptBranchNoAliveAlly {
				if err := d.Set(0, ScriptLabelID, 10); err != nil {
					t.Fatal(err)
				}
				if _, err := s.PlanScriptEdit(d); !errors.Is(err, ErrScript) {
					t.Fatalf("new branch must validate its label: %v", err)
				}
				if err := d.Set(0, ScriptLabelID, 9); err != nil {
					t.Fatal(err)
				}
			}
			if err := d.Insert(1, ScriptCameraUnit); err != nil {
				t.Fatal(err)
			}
			if err := d.Set(1, ScriptCharacter, 3); err != nil {
				t.Fatal(err)
			}
			if err := d.Duplicate(1); err != nil {
				t.Fatal(err)
			}
			if err := d.Set(2, ScriptCharacter, 4); err != nil {
				t.Fatal(err)
			}
			if err := d.Move(2, 1); err != nil {
				t.Fatal(err)
			}
			if err := d.Remove(2); err != nil {
				t.Fatal(err)
			}
			plan, err := s.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			want := append(
				bytes.Clone(original[:conditionLength]),
				cameraRaw...)
			want = append(want, original...)
			data := s.Document.Snapshot().Data
			if !plan.Relocated() ||
				!bytes.Equal(
					data[plan.Destination():plan.Destination()+plan.Length()],
					want,
				) {
				t.Fatal(
					"insert/copy/reorder/delete changed native focus or condition encoding",
				)
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("focus structure did not undo atomically")
			}
		})
	}
}

func TestScriptUnitConditionFindsLaterDeployment(t *testing.T) {
	for _, fixture := range gameFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			f, original, _ := sceneFixture(t, fixture.name)
			endLength := 8
			load := []byte{0x12, 0, 0, 0, 0, 0, 0, 0}
			switch fixture.name {
			case "FE8J", "FE8U":
				endLength, load = 4, []byte{0x40, 0x2C, 1, 0, 0, 0, 0, 0}
			case "FE7J":
				load[0] = 0x30
			case "FE7U":
				load[0] = 0x32
			}
			at := f.GetPosition("script_root") + len(original) - endLength
			f.WriteAt(at, append(load, original[len(original)-endLength:]...))
			f.WritePointer(at+4, f.GetPosition("objective_units"))
			s := openFixture(t, f)
			groups, err := s.ChapterDeployments(0)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, group := range groups.Groups {
				if group.Pointer == at+4 {
					found = group.Err == nil && len(group.Units) > 0
				}
			}
			if !found {
				t.Fatal(
					"unit-condition branch hid deployment after an early end",
				)
			}
		})
	}
}

func TestScriptCameraWithOpaqueInstruction(t *testing.T) {
	for _, fixture := range gameFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			f, original, _ := sceneFixture(t, fixture.name)
			// Nonzero reserved operands do not become editable simply because
			// the same opcode has a confirmed zero-operand form.
			opaque := []byte{0x22, 0x3B, 0xA5, 0x5A}
			offset := 2
			switch fixture.name {
			case "FE6":
				opaque, offset = []byte{0x2F, 0, 0xA5, 0x5A}, 4
			case "FE7J":
				opaque = []byte{0x67, 0, 0xA5, 0x5A}
			case "FE7U":
				opaque = []byte{0x69, 0, 0xA5, 0x5A}
			}
			f.WriteAt(f.GetPosition("script_root"), append(opaque, original...))
			s := openFixture(t, f)
			before := s.Document.Snapshot()
			d, err := s.NewScriptDraft(0, f.GetPosition("script_site"))
			if err != nil {
				t.Fatal(err)
			}
			if !d.Complete() || d.Relocatable() ||
				d.Instructions()[0].Editable {
				t.Fatal("opaque instruction lost its editing boundary")
			}
			if err := d.Insert(
				0,
				ScriptCursorClear,
			); !errors.Is(
				err,
				ErrScriptStructure,
			) {
				t.Fatalf("opaque script insertion: %v", err)
			}
			index := sceneInstruction(t, d, ScriptCameraUnit)
			if err := d.Set(index, ScriptCharacter, 2); err != nil {
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
			want[d.Instructions()[index].Address+offset] = 2
			if plan.Relocated() ||
				!bytes.Equal(want, s.Document.Snapshot().Data) {
				t.Fatal(
					"camera operand edit changed opaque or neighboring bytes",
				)
			}
		})
	}
}
