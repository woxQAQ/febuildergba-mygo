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

func TestRealTalkEvents(t *testing.T) {
	samples := map[string]struct {
		chapter, pointer, address, count, kind, flag, initiator, target, script int
		text                                                                    uint16
	}{
		"FE8U": {1, 0x9E87A0, 0x9E86C0, 4, 3, 8, 2, 4, 0x9EFB38, 0x93A},
		"FE8J": {1, 0xA5A8C4, 0xA5A7E4, 4, 3, 8, 2, 4, 0xA61C70, 0x8FA},
		"FE7U": {4, 0xCA0EAC, 0xCA0D7C, 2, 3, 6, 3, 8, 0xCAAA34, 0x8ED},
		"FE7J": {4, 0xD68E7C, 0xD68D4C, 2, 3, 6, 3, 8, 0xD72C64, 0x90E},
		"FE6":  {2, 0x66786C, 0x6677D8, 2, 4, 5, 1, 0x18, 0x66BCA0, 0x56},
	}
	for _, fixture := range realROMFixtures {
		t.Run(fixture.id, func(t *testing.T) {
			sample := samples[fixture.id]
			s := openRealROM(t, fixture.filename)
			native, err := s.ChapterEvents(sample.chapter, game.TalkEvents)
			if err != nil {
				t.Fatal(err)
			}
			if native.Pointer != sample.pointer ||
				native.Address != sample.address ||
				len(native.Events) != sample.count {
				t.Fatalf("fixed native talk table: %+v", native)
			}
			event := native.Events[0]
			if event.Kind != sample.kind || event.Flag != sample.flag ||
				event.Initiator != sample.initiator ||
				event.Target != sample.target ||
				event.TalkMode != 0 ||
				event.TalkFlag != 0 ||
				event.Script != sample.script ||
				!slices.Equal(event.Texts, []uint16{sample.text}) {
				t.Fatalf("fixed directional talk: %+v", event)
			}
			before := s.Document.Snapshot()
			d, err := s.NewEventDraft(sample.chapter, game.TalkEvents)
			if err != nil {
				t.Fatal(err)
			}
			for field, value := range map[string]int64{game.TalkInitiator: int64(sample.target), game.TalkTarget: int64(sample.initiator), game.EventFlag: 0x61} {
				if err := d.Set(0, field, value); err != nil {
					t.Fatal(err)
				}
			}
			if err := d.Add(s); err != nil {
				t.Fatal(err)
			}
			added := len(d.Events()) - 1
			textID := sample.text + 1
			text, err := s.DialogueText(textID)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.SetDialogue(added, textID); err != nil {
				t.Fatal(err)
			}
			if err := d.Set(added, game.EventFlag, 0x62); err != nil {
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
			result, err := s.ChapterEvents(sample.chapter, game.TalkEvents)
			if err != nil {
				t.Fatal(err)
			}
			for i, old := range native.Events {
				expected := bytes.Clone(old.Raw)
				if i == 0 {
					expected[2], expected[3] = 0x61, 0
					expected[8], expected[9] = byte(
						sample.target,
					), byte(
						sample.initiator,
					)
				}
				if !bytes.Equal(expected, result.Events[i].Raw) {
					t.Fatalf("unexpected existing condition change: %d", i)
				}
			}
			created := result.Events[added]
			if result.Address != plan.Destination() ||
				len(result.Events) != sample.count+1 ||
				created.Kind != sample.kind ||
				created.Flag != 0x62 ||
				!slices.Equal(created.Texts, []uint16{textID}) ||
				len(created.Groups) != 0 ||
				len(created.Warnings) != 0 {
				t.Fatalf("generated ordinary dialogue: %+v", created)
			}
			literal := []byte{
				0x0D,
				0,
				0,
				0,
				0,
				0,
				0,
				0,
				0x09,
				0,
				0,
				0,
				0x0A,
				0,
				0,
				0,
				0,
				0,
				0,
				0,
			}
			binary.LittleEndian.PutUint16(literal[4:], textID)
			if fixture.id == "FE6" {
				literal[0], literal[8], literal[12] = 7, 5, 6
			}
			if fixture.id == "FE8U" || fixture.id == "FE8J" {
				literal = []byte{
					0x20,
					0x1A,
					0,
					0,
					0x20,
					0x1B,
					0,
					0,
					0x20,
					0x1D,
					0,
					0,
					0x22,
					0x1B,
					0,
					0,
					0x20,
					1,
					0,
					0,
				}
				binary.LittleEndian.PutUint16(literal[6:], textID)
			}
			if !bytes.Equal(
				after.Data[created.Script:created.Script+len(literal)],
				literal,
			) {
				t.Fatal("native dialogue script differs from fixed literal")
			}
			prefix := bytes.Clone(after.Data[:len(before.Data)])
			copy(
				prefix[sample.pointer:sample.pointer+4],
				before.Data[sample.pointer:sample.pointer+4],
			)
			if !bytes.Equal(prefix, before.Data) {
				t.Fatal("original ROM changed outside selected talk pointer")
			}
			if !s.Document.Undo() ||
				!bytes.Equal(s.Document.Snapshot().Data, before.Data) ||
				!s.Document.Redo() ||
				!bytes.Equal(s.Document.Snapshot().Data, after.Data) {
				t.Fatal("talk edit did not undo/redo atomically")
			}
			output := filepath.Join(t.TempDir(), "talk-events.gba")
			if err := s.Document.SaveAs(output, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(output)
			if err != nil {
				t.Fatal(err)
			}
			again, err := reopened.ChapterEvents(
				sample.chapter,
				game.TalkEvents,
			)
			if err != nil ||
				!bytes.Equal(again.Events[added].Raw, created.Raw) ||
				!slices.Equal(again.Events[added].Texts, []uint16{textID}) {
				t.Fatal("talk did not survive save/reopen")
			}
			value, err := reopened.TextByID(textID)
			if err != nil || value.Content != text.Content {
				t.Fatal("text reference did not survive save/reopen")
			}
			removal, err := reopened.NewEventDraft(
				sample.chapter,
				game.TalkEvents,
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := removal.Set(0, game.EventFlag, 0x63); err != nil {
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
				game.TalkEvents,
			)
			if err != nil || len(remaining.Events) != sample.count ||
				remaining.Events[0].Flag != 0x63 {
				t.Fatal("saved talks could not continue editing or be removed")
			}
		})
	}
}
