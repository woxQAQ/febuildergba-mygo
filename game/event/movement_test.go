package event_test

import (
	"bytes"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func movementFixture(version string) *testutil.FixtureBuilder {
	f := testutil.NewScriptFixture(version)
	const path = 0x258000
	op := byte(0x10)
	if version == "FE7J" {
		op = 0x26
	}
	if version == "FE7U" {
		op = 0x28
	}
	raw := []byte{
		op,
		0,
		0,
		0,
		1,
		0,
		0,
		0,
		0,
		0x80,
		0x25,
		8,
		op,
		0,
		0,
		0,
		2,
		0,
		0,
		0,
		0,
		0x80,
		0x25,
		8,
		0,
		0,
		0,
		0,
	}
	f.WriteAt(f.GetPosition("script_root"), raw)
	f.WriteAt(path, bytes.Repeat([]byte{0xA5}, 64))
	f.WriteAt(path, []byte{1, 1, 9, 4, 2, 13, 8, 14, 4})
	f.MarkPosition("movement", path)
	return f
}

func TestScriptMovementWorkflow(t *testing.T) {
	for _, version := range []string{"FE6", "FE7J", "FE7U"} {
		t.Run(version, func(t *testing.T) {
			f := movementFixture(version)
			s := testutil.Open(t, f)
			root, site, path := f.GetPosition(
				"script_root",
			), f.GetPosition(
				"script_site",
			), f.GetPosition(
				"movement",
			)
			before := s.Document.Snapshot()
			d, err := s.Events.NewScriptDraft(0, site)
			if err != nil {
				t.Fatal(err)
			}
			steps, err := d.Movement(0, event.ScriptMovementPointer)
			if err != nil ||
				!slices.Equal(
					steps,
					[]event.ScriptMovementStep{
						{Command: 1, Value: 0},
						{Command: 1, Value: 0},
						{Command: 9, Value: 4},
						{Command: 2, Value: 0},
						{Command: 13, Value: 0},
						{Command: 8, Value: 0},
						{Command: 14, Value: 0},
						{Command: 4, Value: 0},
					},
				) {
				t.Fatal("native movement boundaries", steps, err)
			}
			replacement := []event.ScriptMovementStep{
				{Command: 0, Value: 0},
				{Command: 9, Value: 7},
				{Command: 1, Value: 0},
				{Command: 4, Value: 0},
			}
			if err := d.SetMovement(
				0,
				event.ScriptMovementPointer,
				replacement,
			); err != nil {
				t.Fatal(err)
			}
			replacement[0].Command = 3
			plan, err := s.Events.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("movement preview wrote ROM")
			}
			if err := d.SetMovement(
				0,
				event.ScriptMovementPointer,
				[]event.ScriptMovementStep{{Command: 4, Value: 0}},
			); err != nil {
				t.Fatal(err)
			}
			if err := s.Events.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			target, ok := gba.Pointer(after.Data, root+8)
			if !ok || target != len(before.Data) ||
				len(after.Data) != len(before.Data)+64 ||
				gba.Uint32(after.Data, root+20) != uint32(0x08000000+path) {
				t.Fatal("movement ownership or allocation")
			}
			want := append(
				[]byte{0, 9, 7, 1, 4},
				bytes.Repeat([]byte{4}, 59)...)
			if !bytes.Equal(after.Data[target:target+64], want) {
				t.Fatal("mutable movement preview or incorrect bytes")
			}
			preserved := bytes.Clone(after.Data[:len(before.Data)])
			copy(preserved[root+8:root+12], before.Data[root+8:root+12])
			if !bytes.Equal(preserved, before.Data) {
				t.Fatal(
					"movement edit touched another reference or unknown bytes",
				)
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("movement undo")
			}
			if !s.Document.Redo() {
				t.Fatal("movement redo")
			}
			d, err = s.Events.NewScriptDraft(0, site)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.SetMovement(
				1,
				event.ScriptMovementPointer,
				[]event.ScriptMovementStep{
					{Command: 3, Value: 0},
					{Command: 4, Value: 0},
				},
			); err != nil {
				t.Fatal(err)
			}
			if err := d.Duplicate(1); err != nil {
				t.Fatal(err)
			}
			if err := d.Move(2, 0); err != nil {
				t.Fatal(err)
			}
			if err := d.SetMovement(
				0,
				event.ScriptMovementPointer,
				[]event.ScriptMovementStep{
					{Command: 2, Value: 0},
					{Command: 4, Value: 0},
				},
			); err != nil {
				t.Fatal(err)
			}
			plan, err = s.Events.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Events.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			snapshot := s.Document.Snapshot()
			if !plan.Relocated() ||
				!bytes.Equal(
					snapshot.Data[root:root+28],
					after.Data[root:root+28],
				) {
				t.Fatal("movement/script relocation changed independent entry")
			}
			output := filepath.Join(t.TempDir(), "movement.gba")
			if err := s.Document.SaveAs(output, false); err != nil {
				t.Fatal(err)
			}
			s, err = game.Open(output)
			if err != nil {
				t.Fatal(err)
			}
			d, err = s.Events.NewScriptDraft(0, site)
			if err != nil {
				t.Fatal(err)
			}
			for index, command := range []byte{2, 0, 3} {
				steps, err := d.Movement(index, event.ScriptMovementPointer)
				if err != nil || steps[0].Command != command {
					t.Fatal(
						"movement identity after copy, reorder and save",
						index,
						steps,
						err,
					)
				}
			}
			if err := d.Remove(0); err != nil {
				t.Fatal(err)
			}
			plan, err = s.Events.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Events.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestScriptMovementNewFactionResources(t *testing.T) {
	for _, version := range []string{"FE7J", "FE7U"} {
		t.Run(version, func(t *testing.T) {
			f := movementFixture(version)
			s := testutil.Open(t, f)
			d, err := s.Events.NewScriptDraft(0, f.GetPosition("script_site"))
			if err != nil {
				t.Fatal(err)
			}
			original, err := d.Movement(0, event.ScriptMovementPointer)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.SetMovement(
				0,
				event.ScriptMovementPointer,
				[]event.ScriptMovementStep{{Command: 4, Value: 0}},
			); err != nil {
				t.Fatal(err)
			}
			if err := d.SetMovement(
				0,
				event.ScriptMovementPointer,
				original,
			); err != nil ||
				d.Modified() {
				t.Fatal("restoring movement left a dirty draft", err)
			}
			if err := d.Insert(
				0,
				event.ScriptMoveFactionPaths,
			); err != nil {
				t.Fatal(err)
			}
			player := append(
				slices.Repeat(
					[]event.ScriptMovementStep{{Command: 1, Value: 0}},
					63,
				),
				event.ScriptMovementStep{Command: 4, Value: 0},
			)
			if err := d.SetMovement(
				0,
				event.ScriptMovementPlayer,
				player,
			); err != nil {
				t.Fatal(err)
			}
			before := s.Document.Snapshot()
			if _, err := s.Events.PlanScriptEdit(
				d,
			); !errors.Is(err, event.ErrScriptStructure) ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("missing alternate path accepted", err)
			}
			if err := d.SetMovement(
				0,
				event.ScriptMovementOther,
				[]event.ScriptMovementStep{
					{Command: 3, Value: 0},
					{Command: 4, Value: 0},
				},
			); err != nil {
				t.Fatal(err)
			}
			plan, err := s.Events.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if plan.MovementCount() != 2 || !plan.Relocated() {
				t.Fatal("new movement resources not in one plan")
			}
			if err := s.Events.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			d, err = s.Events.NewScriptDraft(0, f.GetPosition("script_site"))
			if err != nil {
				t.Fatal(err)
			}
			got, err := d.Movement(0, event.ScriptMovementPlayer)
			if err != nil || !slices.Equal(player, got) {
				t.Fatal("64-byte player path", err)
			}
			got, err = d.Movement(0, event.ScriptMovementOther)
			if err != nil || len(got) != 2 || got[0].Command != 3 {
				t.Fatal("independent alternate path", err)
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("multi-resource movement undo")
			}
		})
	}
}

func TestScriptMovementGuards(t *testing.T) {
	for _, scenario := range []string{"unknown", "unterminated", "ff end", "capacity", "stale", "length", "sleep", "speed", "missing halt"} {
		t.Run(scenario, func(t *testing.T) {
			f := movementFixture("FE7U")
			switch scenario {
			case "unknown":
				f.WriteAt(f.GetPosition("movement"), []byte{11})
			case "unterminated":
				f.WriteAt(
					f.GetPosition("movement"),
					bytes.Repeat([]byte{1}, 64),
				)
			case "ff end":
				f.WriteAt(f.GetPosition("movement"), []byte{0xFF})
			}
			s := testutil.Open(t, f)
			if scenario == "capacity" {
				if err := s.Document.ApplyEdit(
					s.Document.State().Revision,
					"capacity",
					rom.Edit{Size: rom.MaxSize},
				); err != nil {
					t.Fatal(err)
				}
			}
			d, err := s.Events.NewScriptDraft(0, f.GetPosition("script_site"))
			if err != nil {
				t.Fatal(err)
			}
			steps := []event.ScriptMovementStep{
				{Command: 1, Value: 0},
				{Command: 4, Value: 0},
			}
			switch scenario {
			case "unknown", "unterminated", "ff end":
				if _, err := d.Movement(
					0,
					event.ScriptMovementPointer,
				); !errors.Is(
					err,
					event.ErrScriptStructure,
				) {
					t.Fatal("unproven path exposed", err)
				}
				if err := d.Insert(0, event.ScriptWait); err != nil {
					t.Fatal(err)
				}
				if _, err := s.Events.PlanScriptEdit(
					d,
				); !errors.Is(
					err,
					event.ErrScriptStructure,
				) {
					t.Fatal("unproven path relocated", err)
				}
				d, err = s.Events.NewScriptDraft(
					0,
					f.GetPosition("script_site"),
				)
				if err != nil {
					t.Fatal(err)
				}
			case "length":
				steps = append(
					slices.Repeat(
						[]event.ScriptMovementStep{{Command: 1, Value: 0}},
						64,
					),
					event.ScriptMovementStep{Command: 4, Value: 0},
				)
			case "sleep":
				steps = []event.ScriptMovementStep{
					{Command: 9, Value: 128},
					{Command: 4, Value: 0},
				}
			case "speed":
				steps = []event.ScriptMovementStep{
					{Command: 12, Value: 4},
					{Command: 4, Value: 0},
				}
			case "missing halt":
				steps = []event.ScriptMovementStep{{Command: 1, Value: 0}}
			}
			before := s.Document.Snapshot()
			err = d.SetMovement(0, event.ScriptMovementPointer, steps)
			switch scenario {
			case "length", "sleep", "speed", "missing halt":
				if !errors.Is(err, edit.ErrValue) || d.Modified() {
					t.Fatal("invalid movement modified draft", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			plan, err := s.Events.PlanScriptEdit(d)
			if scenario == "capacity" {
				if !errors.Is(err, space.ErrCapacity) {
					t.Fatal("movement capacity", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("failed or successful preview wrote ROM")
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
				before = s.Document.Snapshot()
				if err := s.Events.ApplyScriptEdit(
					plan,
				); !errors.Is(err, rom.ErrStale) ||
					!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
					t.Fatal("stale movement applied", err)
				}
			} else if err := s.Events.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestScriptWordEndBoundaries(t *testing.T) {
	for _, version := range []string{"FE6", "FE7J", "FE7U"} {
		for _, end := range []byte{0, 1} {
			t.Run(version+string(rune('0'+end)), func(t *testing.T) {
				f := testutil.NewScriptFixture(version)
				branch, label, clearSkip := byte(0x1C), byte(0x1B), byte(6)
				wait := []byte{2, 0, 0, 0, 5, 0, 0, 0}
				if version != "FE6" {
					branch, label, clearSkip = 0x43, 0x42, 0x0A
					wait = []byte{2, 0, 5, 0}
				}
				if version == "FE7U" {
					branch, label = 0x45, 0x44
				}
				raw := []byte{
					branch,
					0,
					0,
					0,
					7,
					0,
					0,
					0,
					end,
					0,
					0,
					0,
					label,
					0,
					0,
					0,
					7,
					0,
					0,
					0,
					clearSkip,
					0,
					0,
					0,
				}
				raw = append(raw, wait...)
				raw = append(raw, 0, 0, 0, 0)
				f.WriteAt(f.GetPosition("script_root"), raw)
				s := testutil.Open(t, f)
				before := s.Document.Snapshot()
				d, err := s.Events.NewScriptDraft(
					0,
					f.GetPosition("script_site"),
				)
				if err != nil {
					t.Fatal(err)
				}
				expected := 2
				if end == 1 {
					expected = 6
				}
				if !d.Complete() || len(d.Instructions()) != expected {
					t.Fatal("native label-search boundary", d.Instructions())
				}
				if err := d.Insert(0, event.ScriptWait); err != nil {
					t.Fatal(err)
				}
				plan, err := s.Events.PlanScriptEdit(d)
				if end == 0 {
					if !errors.Is(err, event.ErrScript) {
						t.Fatal("label after END accepted", err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if err := s.Events.ApplyScriptEdit(plan); err != nil {
					t.Fatal(err)
				}
				if !s.Document.Undo() ||
					!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
					t.Fatal("word control undo")
				}
			})
		}
	}
}
