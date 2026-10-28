//go:build integration

package integration

import (
	"bytes"
	"encoding/binary"
	"path/filepath"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
)

func TestRealChapterObjectives(t *testing.T) {
	samples := map[string]struct{ chapter, pointer, address, count, index, kind, flag, trigger, script, ending, rout, defeat int }{
		"FE8U": {
			2,
			0x9E8940,
			0x9E8890,
			2,
			0,
			1,
			3,
			6,
			0x9F04B4,
			0x9F04B4,
			6,
			0x592104,
		},
		"FE8J": {
			2,
			0xA5AA64,
			0xA5A9B4,
			2,
			0,
			1,
			3,
			6,
			0xA625EC,
			0xA625EC,
			6,
			0x5B9EE8,
		},
		"FE7U": {
			1,
			0xCA0980,
			0xCA0904,
			9,
			0,
			14,
			0,
			0x08079FE9,
			0xCA8F54,
			0xCA8F54,
			0x08079FE9,
			0xCA749C,
		},
		"FE7J": {
			1,
			0xD68950,
			0xD688D4,
			9,
			0,
			14,
			0,
			0x0807AA5D,
			0xD711D8,
			0xD711D8,
			0x0807AA5D,
			0xD6F71C,
		},
		"FE6": {
			1,
			0x6677AC,
			0x667790,
			1,
			0,
			1,
			0,
			0x65,
			0x66AC6C,
			0x66B944,
			0x0806C2F9,
			0x66AC6C,
		},
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
				t.Fatalf("fixed objective table: %+v", native)
			}
			event := native.Events[sample.index]
			if event.Kind != sample.kind || event.Flag != sample.flag ||
				event.Script != sample.script ||
				binary.LittleEndian.Uint32(
					event.Raw[8:],
				) != uint32(
					sample.trigger,
				) {
				t.Fatalf("fixed native condition: %+v", event)
			}
			before := s.Document.Snapshot()
			d, err := s.NewEventDraft(sample.chapter, game.AlwaysEvents)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.ContainsFunc(
				d.Scripts(),
				func(ref game.EventScriptRef) bool {
					return ref.Kind == game.ChapterEndingScript &&
						ref.Address == sample.ending
				},
			) {
				t.Fatal("fixed chapter ending")
			}
			if err := d.Set(sample.index, game.EventFlag, 0x5E); err != nil {
				t.Fatal(err)
			}
			for _, template := range []game.ObjectiveTemplate{game.BossVictory, game.RoutVictory, game.FlagFailure} {
				if err := d.AddObjective(s, template); err != nil {
					t.Fatal(err)
				}
			}
			added := sample.count
			if err := d.Set(added+2, game.EventTrigger, 0x5D); err != nil {
				t.Fatal(err)
			}
			plan, err := s.PlanEventEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyEventEdit(plan); err != nil {
				t.Fatal(err)
			}
			result, err := s.ChapterEvents(sample.chapter, game.AlwaysEvents)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Events) != sample.count+3 ||
				result.Address != plan.Destination() {
				t.Fatal("new complete general-condition table")
			}
			for i, old := range native.Events {
				want := bytes.Clone(old.Raw)
				if i == sample.index {
					want[2], want[3] = 0x5E, 0
				}
				if !bytes.Equal(want, result.Events[i].Raw) {
					t.Fatalf("native bytes changed: %d", i)
				}
			}
			boss, rout, failure := result.Events[added], result.Events[added+1], result.Events[added+2]
			if boss.Kind != 1 || boss.Flag != 3 || boss.TriggerFlag != 2 ||
				boss.Script != sample.ending ||
				rout.Flag != 3 ||
				rout.Script != sample.ending ||
				binary.LittleEndian.Uint32(
					rout.Raw[8:],
				) != uint32(
					sample.rout,
				) ||
				failure.Kind != 1 ||
				failure.Flag != 0 ||
				failure.TriggerFlag != 0x5D ||
				failure.Script != sample.defeat {
				t.Fatal("native objective templates")
			}
			after := s.Document.Snapshot()
			prefix := bytes.Clone(after.Data[:len(before.Data)])
			copy(
				prefix[sample.pointer:sample.pointer+4],
				before.Data[sample.pointer:sample.pointer+4],
			)
			if !bytes.Equal(prefix, before.Data) {
				t.Fatal("original scripts, headers or tables overwritten")
			}
			if !s.Document.Undo() ||
				!bytes.Equal(s.Document.Snapshot().Data, before.Data) {
				t.Fatal("objective atomic undo")
			}
			if !s.Document.Redo() ||
				!bytes.Equal(s.Document.Snapshot().Data, after.Data) {
				t.Fatal("objective atomic redo")
			}
			output := filepath.Join(t.TempDir(), "objectives.gba")
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
			if err != nil || !bytes.Equal(again.Events[added+1].Raw, rout.Raw) {
				t.Fatal("save and reopen")
			}
			edit, err := reopened.NewEventDraft(
				sample.chapter,
				game.AlwaysEvents,
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := edit.SetEventScript(reopened, added, 1); err != nil {
				t.Fatal(err)
			}
			if err := edit.Remove(added + 2); err != nil {
				t.Fatal(err)
			}
			next, err := reopened.PlanEventEdit(edit)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.ApplyEventEdit(next); err != nil {
				t.Fatal(err)
			}
			final, err := reopened.ChapterEvents(
				sample.chapter,
				game.AlwaysEvents,
			)
			if err != nil || len(final.Events) != sample.count+2 ||
				final.Events[added].Script != 1 ||
				len(final.Events[added].Warnings) != 0 {
				t.Fatal("continued native condition editing")
			}
		})
	}
}
