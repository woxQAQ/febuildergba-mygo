//go:build integration

package integration

import (
	"bytes"
	"encoding/binary"
	"image"
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
)

func TestRealMovementPaths(t *testing.T) {
	samples := map[string]struct{ site, address int }{
		"FE8U": {0x9E8664, 0x8B3C14},
		"FE8J": {0xA5A788, 0x90819C},
	}
	nativeSteps := [][]byte{
		{0x0D, 0x82, 1, 0, 0xFF, 0xFF, 0, 0},
		{0x0A, 0x82, 1, 0, 0xFF, 0xFF, 0, 0},
		{0xCA, 0x81, 1, 0, 0xFF, 0xFF, 0, 0},
		{0xC9, 0x81, 1, 0, 0xFF, 0xFF, 0, 0},
		{0x49, 0x81, 1, 0, 0xFF, 0xFF, 0, 0},
		{0x48, 0x81, 1, 0, 0xFF, 0xFF, 0, 0},
	}
	for _, fixture := range realROMFixtures {
		sample, exists := samples[fixture.id]
		if !exists {
			continue
		}
		t.Run(fixture.id, func(t *testing.T) {
			s := openRealROM(t, fixture.filename)
			d, err := s.NewDeploymentDraft(0, sample.site)
			if err != nil {
				t.Fatal(err)
			}
			if d.Address() != sample.address || d.Count() != 2 ||
				len(d.Path(0)) != 6 ||
				d.Units()[0].Start != image.Pt(13, 9) ||
				d.Units()[0].End != image.Pt(8, 5) {
				t.Fatal("fixed native deployment path")
			}
			for index, step := range d.Path(0) {
				if !bytes.Equal(step.Raw, nativeSteps[index]) ||
					step.Config != 24 ||
					step.SyncStep != 0xFFFF {
					t.Fatalf("fixed native REDA %d: %+v", index, step)
				}
			}
			before := s.Document.Snapshot()
			if err := d.MovePathStep(0, 5, 1); err != nil {
				t.Fatal(err)
			}
			if err := d.DuplicatePathStep(0, 2); err != nil {
				t.Fatal(err)
			}
			if err := d.RemovePathStep(0, 3); err != nil {
				t.Fatal(err)
			}
			if err := d.AddPathStep(0); err != nil {
				t.Fatal(err)
			}
			for field, value := range map[string]int64{game.MoveX: 7, game.MoveY: 6, game.MoveConfig: 3, game.MoveSyncUnit: 2, game.MoveSyncStep: 0, game.MoveDelay: 22} {
				if err := d.SetPathField(0, 6, field, value); err != nil {
					t.Fatal(err)
				}
			}
			if err := d.SetPathField(0, 0, game.MoveDelay, 12); err != nil {
				t.Fatal(err)
			}
			plan, err := s.PlanDeploymentEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.AddPathStep(0); err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyDeploymentEdit(plan); err != nil {
				t.Fatal(err)
			}
			actual, err := s.NewDeploymentDraft(0, sample.site)
			if err != nil {
				t.Fatal(err)
			}
			steps := actual.Path(0)
			if len(steps) != 7 || actual.Units()[0].End != image.Pt(7, 6) {
				t.Fatal("immutable native path plan")
			}
			for index, source := range []int{0, 5, 1, 2, 3, 4} {
				expected := bytes.Clone(nativeSteps[source])
				if index == 0 {
					expected[6] = 12
				}
				if !bytes.Equal(steps[index].Raw, expected) {
					t.Fatalf("preserved native segment %d", index)
				}
			}
			if !bytes.Equal(
				steps[6].Raw,
				[]byte{0x87, 0x31, 0, 2, 0, 0, 22, 0},
			) {
				t.Fatalf(
					"new absolute target and synchronization: % X",
					steps[6].Raw,
				)
			}
			independent, err := s.NewDeploymentDraft(0, sample.site+4)
			if err != nil {
				t.Fatal(err)
			}
			if independent.Address() != sample.address ||
				len(independent.Path(0)) != 6 ||
				!bytes.Equal(independent.Path(0)[0].Raw, nativeSteps[0]) {
				t.Fatal("independent pointer alias changed")
			}
			after := s.Document.Snapshot()
			prefix := bytes.Clone(after.Data[:len(before.Data)])
			copy(
				prefix[sample.site:sample.site+4],
				before.Data[sample.site:sample.site+4],
			)
			if !bytes.Equal(prefix, before.Data) {
				t.Fatal(
					"original path or independent native resource overwritten",
				)
			}
			if !s.Document.Undo() ||
				!bytes.Equal(s.Document.Snapshot().Data, before.Data) {
				t.Fatal("atomic path undo")
			}
			if !s.Document.Redo() ||
				!bytes.Equal(s.Document.Snapshot().Data, after.Data) {
				t.Fatal("atomic path redo")
			}
			output := filepath.Join(t.TempDir(), "movement.gba")
			if err := s.Document.SaveAs(output, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(output)
			if err != nil {
				t.Fatal(err)
			}
			edit, err := reopened.NewDeploymentDraft(0, sample.site)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(edit.Path(0)[6].Raw, steps[6].Raw) {
				t.Fatal("save and reopen")
			}
			for range 7 {
				if err := edit.RemovePathStep(0, 0); err != nil {
					t.Fatal(err)
				}
			}
			empty, err := reopened.PlanDeploymentEdit(edit)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.ApplyDeploymentEdit(empty); err != nil {
				t.Fatal(err)
			}
			data := reopened.Document.Snapshot().Data
			if data[empty.Destination()+7] != 0 ||
				binary.LittleEndian.Uint32(data[empty.Destination()+8:]) != 0 {
				t.Fatal("empty path count/pointer")
			}
		})
	}
}
