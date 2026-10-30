package deployment_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/deployment"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func TestMovementPathWorkflow(t *testing.T) {
	for _, version := range []string{"FE8U", "FE8J"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewPathFixture(version)
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			d, err := s.Deployments.NewDraft(0, f.GetPosition("deploy_first"))
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
			for field, value := range map[string]int64{deployment.MoveX: 9, deployment.MoveConfig: 18, deployment.MoveSyncUnit: 2, deployment.MoveSyncStep: 0xFFFF, deployment.MoveDelay: 15} {
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
			if err := d.SetPathField(0, 3, deployment.MoveY, 9); err != nil {
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
			if err := d.SetPathField(2, 0, deployment.MoveX, 11); err != nil {
				t.Fatal(err)
			}
			if d.Path(0)[0].Position.X != 6 {
				t.Fatal("duplicate unit path aliases source")
			}
			plan, err := s.Deployments.Plan(d)
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.Uses()) != 2 ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("shared preview or local mutation")
			}
			if err := d.SetPathField(0, 1, deployment.MoveY, 0); err != nil {
				t.Fatal(err)
			}
			if err := s.Deployments.Apply(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			groups, err := s.Events.Deployments(0)
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
			reopened, err := game.Open(output)
			if err != nil {
				t.Fatal(err)
			}
			next, err := reopened.Deployments.NewDraft(0, group.Pointer)
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
			edit, err := reopened.Deployments.Plan(next)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.Deployments.Apply(edit); err != nil {
				t.Fatal(err)
			}
			current := reopened.Document.Snapshot().Data
			if current[edit.Destination()+7] != 0 ||
				binary.LittleEndian.Uint32(
					current[edit.Destination()+8:],
				) != 0 {
				t.Fatal("last path deletion left an unused pointer")
			}
			continued, err := reopened.Deployments.NewDraft(0, group.Pointer)
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
			s := testutil.Open(t, f)
			d, err := s.Deployments.NewDraft(0, f.GetPosition("deploy_first"))
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Set(0, "deploy_level", 5); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Deployments.Plan(d); err != nil {
				t.Fatal("untouched oversized path was not preserved", err)
			}
			if err := d.SetPathField(0, 0, deployment.MoveX, 2); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Deployments.Plan(
				d,
			); !errors.Is(
				err,
				edit.ErrValue,
			) {
				t.Fatal("modified oversized native buffer accepted", err)
			}
			if err := d.RemovePathStep(0, 16); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Deployments.Plan(d); err != nil {
				t.Fatal(err)
			}
			before, generation := d.Path(0), d.Generation()
			if err := d.AddPathStep(0); !errors.Is(err, edit.ErrValue) {
				t.Fatal("17th path step accepted")
			}
			if err := d.DuplicatePathStep(
				0,
				0,
			); !errors.Is(
				err,
				edit.ErrValue,
			) {
				t.Fatal("oversized path duplication accepted")
			}
			if err := d.MovePathStep(
				0,
				-1,
				0,
			); !errors.Is(
				err,
				edit.ErrField,
			) {
				t.Fatal("negative node accepted")
			}
			if err := d.SetPathField(
				0,
				0,
				deployment.MoveConfig,
				64,
			); !errors.Is(
				err,
				edit.ErrValue,
			) {
				t.Fatal("configuration overflow")
			}
			if d.Generation() != generation ||
				!bytes.Equal(before[0].Raw, d.Path(0)[0].Raw) {
				t.Fatal("failed edit mutated path")
			}
			if _, err := s.Records.PlanTableMove(
				records.Items,
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
			s := testutil.Open(t, f)
			d, err := s.Deployments.NewDraft(0, f.GetPosition("deploy_first"))
			if err != nil {
				t.Fatal(err)
			}
			if d.SupportsPaths() || len(d.Path(0)) != 0 ||
				!errors.Is(d.AddPathStep(0), edit.ErrField) {
				t.Fatal("FE8 REDA editing offered for a legacy record")
			}
		})
	}
}
