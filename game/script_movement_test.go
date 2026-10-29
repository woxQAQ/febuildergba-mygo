package game

import (
	"bytes"
	"errors"
	"path/filepath"
	"slices"
	"testing"

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
			s := openFixture(t, f)
			root, site, path := f.GetPosition(
				"script_root",
			), f.GetPosition(
				"script_site",
			), f.GetPosition(
				"movement",
			)
			before := s.Document.Snapshot()
			d, err := s.NewScriptDraft(0, site)
			if err != nil {
				t.Fatal(err)
			}
			steps, err := d.Movement(0, ScriptMovementPointer)
			if err != nil ||
				!slices.Equal(
					steps,
					[]ScriptMovementStep{
						{1, 0},
						{1, 0},
						{9, 4},
						{2, 0},
						{13, 0},
						{8, 0},
						{14, 0},
						{4, 0},
					},
				) {
				t.Fatal("native movement boundaries", steps, err)
			}
			replacement := []ScriptMovementStep{{0, 0}, {9, 7}, {1, 0}, {4, 0}}
			if err := d.SetMovement(
				0,
				ScriptMovementPointer,
				replacement,
			); err != nil {
				t.Fatal(err)
			}
			replacement[0].Command = 3
			plan, err := s.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("movement preview wrote ROM")
			}
			if err := d.SetMovement(
				0,
				ScriptMovementPointer,
				[]ScriptMovementStep{{4, 0}},
			); err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			target, ok := pointer(after.Data, root+8)
			if !ok || target != len(before.Data) ||
				len(after.Data) != len(before.Data)+64 ||
				u32(after.Data, root+20) != uint32(0x08000000+path) {
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
			d, err = s.NewScriptDraft(0, site)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.SetMovement(
				1,
				ScriptMovementPointer,
				[]ScriptMovementStep{{3, 0}, {4, 0}},
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
				ScriptMovementPointer,
				[]ScriptMovementStep{{2, 0}, {4, 0}},
			); err != nil {
				t.Fatal(err)
			}
			plan, err = s.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyScriptEdit(plan); err != nil {
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
			s, err = Open(output)
			if err != nil {
				t.Fatal(err)
			}
			d, err = s.NewScriptDraft(0, site)
			if err != nil {
				t.Fatal(err)
			}
			for index, command := range []byte{2, 0, 3} {
				steps, err := d.Movement(index, ScriptMovementPointer)
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
			plan, err = s.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestScriptMovementNewFactionResources(t *testing.T) {
	for _, version := range []string{"FE7J", "FE7U"} {
		t.Run(version, func(t *testing.T) {
			f := movementFixture(version)
			s := openFixture(t, f)
			d, err := s.NewScriptDraft(0, f.GetPosition("script_site"))
			if err != nil {
				t.Fatal(err)
			}
			original, err := d.Movement(0, ScriptMovementPointer)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.SetMovement(
				0,
				ScriptMovementPointer,
				[]ScriptMovementStep{{4, 0}},
			); err != nil {
				t.Fatal(err)
			}
			if err := d.SetMovement(
				0,
				ScriptMovementPointer,
				original,
			); err != nil ||
				d.Modified() {
				t.Fatal("restoring movement left a dirty draft", err)
			}
			if err := d.Insert(0, ScriptMoveFactionPaths); err != nil {
				t.Fatal(err)
			}
			player := append(
				slices.Repeat([]ScriptMovementStep{{1, 0}}, 63),
				ScriptMovementStep{4, 0},
			)
			if err := d.SetMovement(
				0,
				ScriptMovementPlayer,
				player,
			); err != nil {
				t.Fatal(err)
			}
			before := s.Document.Snapshot()
			if _, err := s.PlanScriptEdit(
				d,
			); !errors.Is(err, ErrScriptStructure) ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("missing alternate path accepted", err)
			}
			if err := d.SetMovement(
				0,
				ScriptMovementOther,
				[]ScriptMovementStep{{3, 0}, {4, 0}},
			); err != nil {
				t.Fatal(err)
			}
			plan, err := s.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if plan.MovementCount() != 2 || !plan.Relocated() {
				t.Fatal("new movement resources not in one plan")
			}
			if err := s.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			d, err = s.NewScriptDraft(0, f.GetPosition("script_site"))
			if err != nil {
				t.Fatal(err)
			}
			got, err := d.Movement(0, ScriptMovementPlayer)
			if err != nil || !slices.Equal(player, got) {
				t.Fatal("64-byte player path", err)
			}
			got, err = d.Movement(0, ScriptMovementOther)
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
			s := openFixture(t, f)
			if scenario == "capacity" {
				if err := s.Document.ApplyEdit(
					s.Document.State().Revision,
					"capacity",
					rom.Edit{Size: rom.MaxSize},
				); err != nil {
					t.Fatal(err)
				}
			}
			d, err := s.NewScriptDraft(0, f.GetPosition("script_site"))
			if err != nil {
				t.Fatal(err)
			}
			steps := []ScriptMovementStep{{1, 0}, {4, 0}}
			switch scenario {
			case "unknown", "unterminated", "ff end":
				if _, err := d.Movement(
					0,
					ScriptMovementPointer,
				); !errors.Is(
					err,
					ErrScriptStructure,
				) {
					t.Fatal("unproven path exposed", err)
				}
				if err := d.Insert(0, ScriptWait); err != nil {
					t.Fatal(err)
				}
				if _, err := s.PlanScriptEdit(
					d,
				); !errors.Is(
					err,
					ErrScriptStructure,
				) {
					t.Fatal("unproven path relocated", err)
				}
				d, err = s.NewScriptDraft(0, f.GetPosition("script_site"))
				if err != nil {
					t.Fatal(err)
				}
			case "length":
				steps = append(
					slices.Repeat([]ScriptMovementStep{{1, 0}}, 64),
					ScriptMovementStep{4, 0},
				)
			case "sleep":
				steps = []ScriptMovementStep{{9, 128}, {4, 0}}
			case "speed":
				steps = []ScriptMovementStep{{12, 4}, {4, 0}}
			case "missing halt":
				steps = []ScriptMovementStep{{1, 0}}
			}
			before := s.Document.Snapshot()
			err = d.SetMovement(0, ScriptMovementPointer, steps)
			switch scenario {
			case "length", "sleep", "speed", "missing halt":
				if !errors.Is(err, ErrValue) || d.Modified() {
					t.Fatal("invalid movement modified draft", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			plan, err := s.PlanScriptEdit(d)
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
				if err := s.Set(Chapters, []int{0}, "weather", 2); err != nil {
					t.Fatal(err)
				}
				before = s.Document.Snapshot()
				if err := s.ApplyScriptEdit(
					plan,
				); !errors.Is(err, rom.ErrStale) ||
					!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
					t.Fatal("stale movement applied", err)
				}
			} else if err := s.ApplyScriptEdit(plan); err != nil {
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
				s := openFixture(t, f)
				before := s.Document.Snapshot()
				d, err := s.NewScriptDraft(0, f.GetPosition("script_site"))
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
				if err := d.Insert(0, ScriptWait); err != nil {
					t.Fatal(err)
				}
				plan, err := s.PlanScriptEdit(d)
				if end == 0 {
					if !errors.Is(err, ErrScript) {
						t.Fatal("label after END accepted", err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if err := s.ApplyScriptEdit(plan); err != nil {
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
