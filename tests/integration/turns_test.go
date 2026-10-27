//go:build integration

package integration

import (
	"bytes"
	"encoding/binary"
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
)

func TestRealTurnReinforcements(t *testing.T) {
	samples := map[string]struct {
		chapter, pointer, address, count, index, start, end, phase, script int
	}{
		"FE8U": {9, 0x9E8FE4, 0x9E8EC0, 3, 2, 5, 0, 0, 0x9F3268},
		"FE8J": {9, 0xA5B108, 0xA5AFE4, 3, 2, 5, 0, 0, 0xA6539C},
		"FE7U": {9, 0xCA1538, 0xCA14AC, 3, 2, 3, 4, 0x80, 0xCAD664},
		"FE7J": {9, 0xD69508, 0xD6947C, 3, 2, 3, 4, 0x80, 0xD7582C},
		"FE6":  {5, 0x667BBC, 0x667AF8, 6, 1, 12, 15, 0x80, 0x67AF3C},
	}
	for _, fixture := range realROMFixtures {
		t.Run(fixture.id, func(t *testing.T) {
			sample := samples[fixture.id]
			s := openRealROM(t, fixture.filename)
			native, err := s.ChapterEvents(sample.chapter, game.TurnEvents)
			if err != nil {
				t.Fatal(err)
			}
			if native.Pointer != sample.pointer ||
				native.Address != sample.address ||
				len(native.Events) != sample.count {
				t.Fatalf("fixed native turn table: %+v", native)
			}
			event := native.Events[sample.index]
			if event.Kind != 2 || event.Start != sample.start ||
				event.End != sample.end ||
				event.Phase != sample.phase ||
				event.Script != sample.script ||
				len(event.Groups) != 1 ||
				event.Groups[0].Err != nil {
				t.Fatalf("fixed reinforcement condition: %+v", event)
			}
			before := s.Document.Snapshot()
			d, err := s.NewEventDraft(sample.chapter, game.TurnEvents)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Set(sample.index, game.TurnStart, 8); err != nil {
				t.Fatal(err)
			}
			if err := d.Set(sample.index, game.TurnEnd, 0); err != nil {
				t.Fatal(err)
			}
			if err := d.Add(s); err != nil {
				t.Fatal(err)
			}
			added := len(d.Events()) - 1
			group := d.Deployment(added)
			for field, value := range map[string]int64{"deploy_character": 1, "deploy_class": 2, "deploy_level": 5, "deploy_start_x": 2, "deploy_start_y": 3, "deploy_end_x": 4, "deploy_end_y": 5, "deploy_item_1": 1, "deploy_ai_1": 3} {
				if err := group.Set(0, field, value); err != nil {
					t.Fatal(err)
				}
			}
			if err := group.Duplicate(0); err != nil {
				t.Fatal(err)
			}
			if err := group.Set(1, "deploy_start_x", 3); err != nil {
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
			result, err := s.ChapterEvents(sample.chapter, game.TurnEvents)
			if err != nil {
				t.Fatal(err)
			}
			if result.Address != plan.Destination() ||
				len(result.Events) != sample.count+1 {
				t.Fatal("new turn table")
			}
			for i, old := range native.Events {
				expected := bytes.Clone(old.Raw)
				if i == sample.index {
					expected[8], expected[9] = 8, 0
				}
				if !bytes.Equal(expected, result.Events[i].Raw) {
					t.Fatalf(
						"native record %d changed outside target fields",
						i,
					)
				}
			}
			created := result.Events[added]
			if created.Kind != 2 || created.Start != 3 || created.End != 0 ||
				created.Phase != 0x80 ||
				created.Flag != 0 ||
				len(created.Groups) != 1 ||
				len(created.Warnings) != 0 {
				t.Fatalf("new event: %+v", created)
			}
			deployed := created.Groups[0]
			if deployed.Err != nil || len(deployed.Units) != 2 ||
				deployed.Units[0].Level != 5 ||
				deployed.Units[0].Faction != 2 ||
				deployed.Units[0].Start.X != 2 ||
				deployed.Units[0].End.Y != 5 {
				t.Fatalf("new reinforcement group: %+v", deployed)
			}
			if int(
				binary.LittleEndian.Uint32(
					after.Data[created.Script+4:],
				)&0x01FFFFFF,
			) != deployed.Address {
				t.Fatal("unit operand")
			}
			prefix := bytes.Clone(after.Data[:len(before.Data)])
			copy(
				prefix[sample.pointer:sample.pointer+4],
				before.Data[sample.pointer:sample.pointer+4],
			)
			if !bytes.Equal(prefix, before.Data) {
				t.Fatal("old ROM data changed")
			}
			if !s.Document.Undo() ||
				!bytes.Equal(s.Document.Snapshot().Data, before.Data) {
				t.Fatal("undo condition/script/group allocation")
			}
			if !s.Document.Redo() ||
				!bytes.Equal(s.Document.Snapshot().Data, after.Data) {
				t.Fatal("redo condition/script/group allocation")
			}
			s.Refresh()
			output := filepath.Join(t.TempDir(), "turn-reinforcements.gba")
			if err := s.Document.SaveAs(output, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(output)
			if err != nil {
				t.Fatal(err)
			}
			again, err := reopened.ChapterEvents(
				sample.chapter,
				game.TurnEvents,
			)
			if err != nil ||
				!bytes.Equal(again.Events[added].Raw, created.Raw) ||
				again.Events[added].Groups[0].Units[1].Start.X != 3 {
				t.Fatal("save/reopen")
			}
			units, err := reopened.NewDeploymentDraft(
				sample.chapter,
				deployed.Pointer,
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := units.Set(0, "deploy_level", 6); err != nil {
				t.Fatal(err)
			}
			unitPlan, err := reopened.PlanDeploymentEdit(units)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.ApplyDeploymentEdit(unitPlan); err != nil {
				t.Fatal(err)
			}
			removal, err := reopened.NewEventDraft(
				sample.chapter,
				game.TurnEvents,
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := removal.Remove(added); err != nil {
				t.Fatal(err)
			}
			removePlan, err := reopened.PlanEventEdit(removal)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.ApplyEventEdit(removePlan); err != nil {
				t.Fatal(err)
			}
			remaining, err := reopened.ChapterEvents(
				sample.chapter,
				game.TurnEvents,
			)
			if err != nil || len(remaining.Events) != sample.count {
				t.Fatal("remove saved reinforcement")
			}
		})
	}
}
