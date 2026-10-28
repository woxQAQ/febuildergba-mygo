package game

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func TestMovementPathWorkflow(t *testing.T) {
	for _, version := range []string{"FE8U", "FE8J"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewPathFixture(version)
			s := openFixture(t, f)
			before := s.Document.Snapshot()
			d, err := s.NewDeploymentDraft(0, f.GetPosition("deploy_first"))
			if err != nil {
				t.Fatal(err)
			}
			steps := d.Path(0)
			if len(steps) != 3 || steps[0].Position != image.Pt(2, 3) ||
				steps[0].Config != 5 ||
				steps[0].SyncUnit != 4 ||
				steps[0].SyncStep != 1 ||
				steps[0].Delay != 12 ||
				steps[1].Config != 42 ||
				steps[1].Delay != 0x8000 ||
				d.Units()[0].End != image.Pt(6, 7) {
				t.Fatalf("fixed native fields: %+v", steps)
			}
			steps[0].Raw[0] = 0xFF
			if d.Path(0)[0].Raw[0] != 0xC2 {
				t.Fatal("path snapshot aliases draft")
			}
			for field, value := range map[string]int64{MoveX: 9, MoveConfig: 18, MoveSyncUnit: 2, MoveSyncStep: 0xFFFF, MoveDelay: 15} {
				if err := d.SetPathField(0, 0, field, value); err != nil {
					t.Fatal(err)
				}
			}
			if err := d.DuplicatePathStep(0, 0); err != nil {
				t.Fatal(err)
			}
			if err := d.MovePathStep(0, 3, 0); err != nil {
				t.Fatal(err)
			}
			if err := d.RemovePathStep(0, 2); err != nil {
				t.Fatal(err)
			}
			if err := d.AddPathStep(0); err != nil {
				t.Fatal(err)
			}
			if err := d.SetPathField(0, 3, MoveY, 9); err != nil {
				t.Fatal(err)
			}
			expected := []byte{
				0xC6, 0x01, 0xBD, 1, 0, 0, 60, 0,
				0xC9, 0x20, 0xAD, 2, 0xFF, 0xFF, 15, 0,
				0x44, 0xA1, 0xD6, 0, 0xFF, 0xFF, 0, 0x80,
				0x44, 0x02, 0, 0, 0xFF, 0xFF, 0, 0,
			}
			var raw []byte
			for _, step := range d.Path(0) {
				raw = append(raw, step.Raw...)
			}
			if !bytes.Equal(raw, expected) {
				t.Fatalf("fixed byte preservation and order: % X", raw)
			}
			if err := d.Duplicate(0); err != nil {
				t.Fatal(err)
			}
			if err := d.SetPathField(2, 0, MoveX, 11); err != nil {
				t.Fatal(err)
			}
			if d.Path(0)[0].Position.X != 6 {
				t.Fatal("duplicate unit path aliases source")
			}
			plan, err := s.PlanDeploymentEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.Uses()) != 2 ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("shared preview or local mutation")
			}
			if err := d.SetPathField(0, 1, MoveY, 0); err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyDeploymentEdit(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			groups, err := s.ChapterDeployments(0)
			if err != nil {
				t.Fatal(err)
			}
			group := groups.Groups[0]
			unit := group.Address
			path := int(
				binary.LittleEndian.Uint32(after.Data[unit+8:]),
			) - 0x08000000
			if after.Data[unit+7] != 4 ||
				path == f.GetPosition("deploy_path") ||
				!bytes.Equal(after.Data[path:path+32], expected) {
				t.Fatal("new path packing or immutable plan")
			}
			other := int(
				binary.LittleEndian.Uint32(after.Data[unit+20+8:]),
			) - 0x08000000
			copied := int(
				binary.LittleEndian.Uint32(after.Data[unit+40+8:]),
			) - 0x08000000
			if other != f.GetPosition("deploy_path") || copied == path ||
				after.Data[copied]&63 != 11 {
				t.Fatal("shared path or duplicate ownership")
			}
			prefix := bytes.Clone(after.Data[:len(before.Data)])
			copy(
				prefix[group.Pointer:group.Pointer+4],
				before.Data[group.Pointer:group.Pointer+4],
			)
			if !bytes.Equal(prefix, before.Data) {
				t.Fatal("old native resources changed")
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("atomic path undo")
			}
			if !s.Document.Redo() ||
				!bytes.Equal(after.Data, s.Document.Snapshot().Data) {
				t.Fatal("atomic path redo")
			}
			output := filepath.Join(t.TempDir(), "paths.gba")
			if err := s.Document.SaveAs(output, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(output)
			if err != nil {
				t.Fatal(err)
			}
			next, err := reopened.NewDeploymentDraft(0, group.Pointer)
			if err != nil {
				t.Fatal(err)
			}
			for range 4 {
				if err := next.RemovePathStep(0, 0); err != nil {
					t.Fatal(err)
				}
			}
			if next.Units()[0].MovementCount != 0 ||
				next.Units()[0].End != next.Units()[0].Start {
				t.Fatal("empty path endpoint")
			}
			edit, err := reopened.PlanDeploymentEdit(next)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.ApplyDeploymentEdit(edit); err != nil {
				t.Fatal(err)
			}
			current := reopened.Document.Snapshot().Data
			if current[edit.Destination()+7] != 0 ||
				binary.LittleEndian.Uint32(
					current[edit.Destination()+8:],
				) != 0 {
				t.Fatal("last path deletion left an unused pointer")
			}
			continued, err := reopened.NewDeploymentDraft(0, group.Pointer)
			if err != nil {
				t.Fatal(err)
			}
			if err := continued.AddPathStep(0); err != nil {
				t.Fatal(err)
			}
			node := continued.Path(0)[0]
			if node.Position != continued.Units()[0].Start ||
				node.SyncStep != 0xFFFF ||
				node.Delay != 0 ||
				node.Config != 0 {
				t.Fatal("empty path native defaults")
			}
		})
	}
}

func TestMovementNativeLimits(t *testing.T) {
	for _, version := range []string{"FE8U", "FE8J"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewPathFixture(version)
			path := f.GetPosition("deploy_path")
			f.WriteAt(
				path,
				bytes.Repeat([]byte{1, 0, 0xA8, 0, 0xFF, 0xFF, 0, 0}, 17),
			)
			f.WriteU8(f.GetPosition("deploy_units")+7, 17)
			s := openFixture(t, f)
			d, err := s.NewDeploymentDraft(0, f.GetPosition("deploy_first"))
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Set(0, "deploy_level", 5); err != nil {
				t.Fatal(err)
			}
			if _, err := s.PlanDeploymentEdit(d); err != nil {
				t.Fatal("untouched oversized path was not preserved", err)
			}
			if err := d.SetPathField(0, 0, MoveX, 2); err != nil {
				t.Fatal(err)
			}
			if _, err := s.PlanDeploymentEdit(d); !errors.Is(err, ErrValue) {
				t.Fatal("modified oversized native buffer accepted", err)
			}
			if err := d.RemovePathStep(0, 16); err != nil {
				t.Fatal(err)
			}
			if _, err := s.PlanDeploymentEdit(d); err != nil {
				t.Fatal(err)
			}
			before, generation := d.Path(0), d.Generation()
			if err := d.AddPathStep(0); !errors.Is(err, ErrValue) {
				t.Fatal("17th path step accepted")
			}
			if err := d.DuplicatePathStep(0, 0); !errors.Is(err, ErrValue) {
				t.Fatal("oversized path duplication accepted")
			}
			if err := d.MovePathStep(0, -1, 0); !errors.Is(err, ErrField) {
				t.Fatal("negative node accepted")
			}
			if err := d.SetPathField(
				0,
				0,
				MoveConfig,
				64,
			); !errors.Is(
				err,
				ErrValue,
			) {
				t.Fatal("configuration overflow")
			}
			if d.Generation() != generation ||
				!bytes.Equal(before[0].Raw, d.Path(0)[0].Raw) {
				t.Fatal("failed edit mutated path")
			}
			if _, err := s.PlanTableMove(
				Items,
				[]space.FreeRange{
					{Start: path, End: path + 0x100, Source: "live REDA"},
				},
			); !errors.Is(
				err,
				space.ErrRange,
			) {
				t.Fatal("live path reused")
			}
		})
	}
	for _, version := range []string{"FE7U", "FE7J", "FE6"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewDeploymentFixture(version)
			s := openFixture(t, f)
			d, err := s.NewDeploymentDraft(0, f.GetPosition("deploy_first"))
			if err != nil {
				t.Fatal(err)
			}
			if d.SupportsPaths() || len(d.Path(0)) != 0 ||
				!errors.Is(d.AddPathStep(0), ErrField) {
				t.Fatal("FE8 REDA editing offered for a legacy record")
			}
		})
	}
}

func TestReinforcementPathPlan(t *testing.T) {
	for _, version := range []string{"FE8U", "FE8J"} {
		t.Run(version, func(t *testing.T) {
			s := openFixture(t, testutil.NewAreaFixture(version))
			d, err := s.NewEventDraft(0, TurnEvents)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Add(s); err != nil {
				t.Fatal(err)
			}
			index := len(d.Events()) - 1
			units := d.Deployment(index)
			for range 2 {
				if err := units.AddPathStep(0); err != nil {
					t.Fatal(err)
				}
			}
			for field, value := range map[string]int64{MoveX: 4, MoveY: 5, MoveSyncUnit: 2, MoveDelay: 30} {
				if err := units.SetPathField(0, 1, field, value); err != nil {
					t.Fatal(err)
				}
			}
			generation := units.Generation()
			plan, err := s.PlanEventEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if err := units.SetPathField(0, 1, MoveX, 7); err != nil {
				t.Fatal(err)
			}
			if generation == units.Generation() {
				t.Fatal("path edit did not change deployment generation")
			}
			if err := s.ApplyEventEdit(plan); err != nil {
				t.Fatal(err)
			}
			events, err := s.ChapterEvents(0, TurnEvents)
			if err != nil {
				t.Fatal(err)
			}
			group := events.Events[index].Groups[0]
			if group.Units[0].End != image.Pt(4, 5) ||
				group.Units[0].MovementCount != 2 {
				t.Fatal("event plan path was mutable")
			}
			edit, err := s.NewDeploymentDraft(0, group.Pointer)
			if err != nil {
				t.Fatal(err)
			}
			if step := edit.Path(0)[1]; step.SyncUnit != 2 || step.Delay != 30 {
				t.Fatal("reinforcement REDA fields changed")
			}
		})
	}
}
