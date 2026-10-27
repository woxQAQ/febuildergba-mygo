//go:build integration

package integration

import (
	"bytes"
	"image"
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
)

func TestRealAreaReinforcements(t *testing.T) {
	samples := map[string]struct{ chapter, pointer, address, count, index, flag, x1, y1, x2, y2, script int }{
		"FE8U": {1, 0x9E87A8, 0x9E872C, 3, 1, 0xA, 0, 0, 7, 9, 0x9EFBCC},
		"FE8J": {1, 0xA5A8CC, 0xA5A850, 3, 1, 0xA, 0, 0, 7, 9, 0xA61D04},
		"FE7U": {6, 0xCA127C, 0xCA120C, 8, 5, 0x10, 8, 1, 8, 1, 0xCAC6CC},
		"FE7J": {6, 0xD6924C, 0xD691DC, 8, 5, 0x10, 8, 1, 8, 1, 0xD748BC},
		"FE6":  {5, 0x667BC8, 0x667BA0, 2, 1, 5, 7, 16, 7, 17, 0x66C948},
	}
	for _, fixture := range realROMFixtures {
		t.Run(fixture.id, func(t *testing.T) {
			sample := samples[fixture.id]
			s := openRealROM(t, fixture.filename)
			native, err := s.ChapterEvents(sample.chapter, game.AlwaysEvents)
			if err != nil {
				t.Fatal(err)
			}
			if native.Pointer != sample.pointer ||
				native.Address != sample.address ||
				len(native.Events) != sample.count {
				t.Fatalf("fixed area table: %+v", native)
			}
			e := native.Events[sample.index]
			if e.Kind != 11 || e.Flag != sample.flag || e.X1 != sample.x1 ||
				e.Y1 != sample.y1 ||
				e.X2 != sample.x2 ||
				e.Y2 != sample.y2 ||
				e.Script != sample.script {
				t.Fatalf("fixed area condition: %+v", e)
			}
			before := s.Document.Snapshot()
			d, err := s.NewEventDraft(sample.chapter, game.AlwaysEvents)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Set(
				sample.index,
				game.AreaX1,
				int64(sample.x1+1),
			); err != nil {
				t.Fatal(err)
			}
			if err := d.Set(
				sample.index,
				game.AreaX2,
				int64(sample.x2+1),
			); err != nil {
				t.Fatal(err)
			}
			if err := d.Add(s); err != nil {
				t.Fatal(err)
			}
			added := len(d.Events()) - 1
			for field, value := range map[string]int64{game.AreaX1: 2, game.AreaY1: 3, game.AreaX2: 4, game.AreaY2: 5, game.EventFlag: 0x60} {
				if err := d.Set(added, field, value); err != nil {
					t.Fatal(err)
				}
			}
			units := d.Deployment(added)
			for field, value := range map[string]int64{"deploy_character": 1, "deploy_class": 2, "deploy_level": 5, "deploy_start_x": 2, "deploy_start_y": 3, "deploy_end_x": 4, "deploy_end_y": 5, "deploy_item_1": 1, "deploy_ai_1": 3} {
				if err := units.Set(0, field, value); err != nil {
					t.Fatal(err)
				}
			}
			if err := units.Duplicate(0); err != nil {
				t.Fatal(err)
			}
			if err := units.Set(1, "deploy_start_x", 3); err != nil {
				t.Fatal(err)
			}
			plan, err := s.PlanEventEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyEventEdit(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			result, err := s.ChapterEvents(sample.chapter, game.AlwaysEvents)
			if err != nil {
				t.Fatal(err)
			}
			if result.Address != plan.Destination() ||
				len(result.Events) != sample.count+1 {
				t.Fatal("new general condition table")
			}
			for i, old := range native.Events {
				expected := bytes.Clone(old.Raw)
				if i == sample.index {
					expected[8]++
					expected[10]++
				}
				if !bytes.Equal(expected, result.Events[i].Raw) {
					t.Fatalf("unrelated native condition bytes changed: %d", i)
				}
			}
			event := result.Events[added]
			if event.Kind != 11 || event.Flag != 0x60 ||
				event.Area() != image.Rect(2, 3, 5, 6) ||
				len(event.Groups) != 1 ||
				len(event.Warnings) != 0 {
				t.Fatalf("generated area: %+v", event)
			}
			group := event.Groups[0]
			if group.Err != nil || len(group.Units) != 2 ||
				group.Units[0].Level != 5 ||
				group.Units[0].Faction != 2 ||
				group.Units[0].End != image.Pt(4, 5) {
				t.Fatalf("generated area deployment: %+v", group)
			}
			prefix := bytes.Clone(after.Data[:len(before.Data)])
			copy(
				prefix[sample.pointer:sample.pointer+4],
				before.Data[sample.pointer:sample.pointer+4],
			)
			if !bytes.Equal(prefix, before.Data) {
				t.Fatal(
					"original ROM data changed outside selected general pointer",
				)
			}
			if !s.Document.Undo() ||
				!bytes.Equal(s.Document.Snapshot().Data, before.Data) {
				t.Fatal("area atomic undo")
			}
			if !s.Document.Redo() ||
				!bytes.Equal(s.Document.Snapshot().Data, after.Data) {
				t.Fatal("area atomic redo")
			}
			output := filepath.Join(t.TempDir(), "area-reinforcements.gba")
			if err := s.Document.SaveAs(output, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(output)
			if err != nil {
				t.Fatal(err)
			}
			again, err := reopened.ChapterEvents(
				sample.chapter,
				game.AlwaysEvents,
			)
			if err != nil || !bytes.Equal(again.Events[added].Raw, event.Raw) ||
				again.Events[added].Groups[0].Units[1].Start.X != 3 {
				t.Fatal("save and reopen")
			}
			groupDraft, err := reopened.NewDeploymentDraft(
				sample.chapter,
				group.Pointer,
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := groupDraft.Set(0, "deploy_level", 6); err != nil {
				t.Fatal(err)
			}
			edit, err := reopened.PlanDeploymentEdit(groupDraft)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.ApplyDeploymentEdit(edit); err != nil {
				t.Fatal(err)
			}
			removal, err := reopened.NewEventDraft(
				sample.chapter,
				game.AlwaysEvents,
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := removal.Remove(added); err != nil {
				t.Fatal(err)
			}
			deletion, err := reopened.PlanEventEdit(removal)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.ApplyEventEdit(deletion); err != nil {
				t.Fatal(err)
			}
			remaining, err := reopened.ChapterEvents(
				sample.chapter,
				game.AlwaysEvents,
			)
			if err != nil || len(remaining.Events) != sample.count {
				t.Fatal("remove saved area reinforcement")
			}
		})
	}
}
