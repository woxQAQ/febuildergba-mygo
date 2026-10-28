//go:build integration

package integration

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
)

func TestRealEventScripts(t *testing.T) {
	samples := map[string]struct {
		chapter, site, address, index, frames, count, opAddress int
		raw                                                     []byte
	}{
		"FE8U": {
			0,
			0x9E8684,
			0x9EEBE8,
			13,
			60,
			37,
			0x9EEC38,
			[]byte{0x20, 0x0E, 60, 0},
		},
		"FE8J": {
			0,
			0xA5A7A8,
			0xA60D20,
			13,
			60,
			37,
			0xA60D70,
			[]byte{0x20, 0x0E, 60, 0},
		},
		"FE7U": {
			0,
			0xCA0758,
			0xCA7A40,
			14,
			60,
			47,
			0xCA7A8C,
			[]byte{2, 0, 60, 0},
		},
		"FE7J": {
			0,
			0xD68728,
			0xD6FCC0,
			14,
			60,
			47,
			0xD6FD0C,
			[]byte{2, 0, 60, 0},
		},
		"FE6": {
			0,
			0x667708,
			0x66B5A0,
			8,
			64,
			10,
			0x66B5D4,
			[]byte{2, 0, 0, 0, 64, 0, 0, 0},
		},
	}
	for _, fixture := range realROMFixtures {
		t.Run(fixture.id, func(t *testing.T) {
			sample := samples[fixture.id]
			s := openRealROM(t, fixture.filename)
			d, err := s.NewScriptDraft(sample.chapter, sample.site)
			if err != nil {
				t.Fatal(err)
			}
			rows := d.Instructions()
			if !d.Complete() || d.Entry().Address != sample.address ||
				len(rows) != sample.count ||
				rows[sample.index].Address != sample.opAddress ||
				rows[sample.index].Op != game.ScriptWait ||
				!bytes.Equal(rows[sample.index].Raw, sample.raw) {
				t.Fatal("fixed native script and instruction bytes")
			}
			if value, _ := d.Value(
				sample.index,
				game.ScriptFrames,
			); value != int64(
				sample.frames,
			) {
				t.Fatal("native wait operand")
			}
			before := s.Document.Snapshot()
			if err := d.Set(
				sample.index,
				game.ScriptFrames,
				int64(sample.frames+1),
			); err != nil {
				t.Fatal(err)
			}
			plan, err := s.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if plan.Relocated() || plan.Destination() != sample.address {
				t.Fatal("parameter edit relocation")
			}
			if err := s.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			expected := bytes.Clone(before.Data)
			offset := 2
			if fixture.id == "FE6" {
				offset = 4
			}
			expected[sample.opAddress+offset]++
			if !bytes.Equal(after.Data, expected) {
				t.Fatal("native parameter patch touched other bytes")
			}
			if !s.Document.Undo() ||
				!bytes.Equal(s.Document.Snapshot().Data, before.Data) {
				t.Fatal("native parameter undo")
			}
			if !s.Document.Redo() ||
				!bytes.Equal(s.Document.Snapshot().Data, after.Data) {
				t.Fatal("native parameter redo")
			}
			if !s.Document.Undo() {
				t.Fatal("restore parameter check")
			}
			chapter, site := 4, 0xCA0D38
			switch fixture.id {
			case "FE7J":
				site = 0xD68D18
			case "FE6":
				chapter, site = 3, 0x6678A0
			case "FE8U", "FE8J":
				talks, readErr := s.NewEventDraft(1, game.TalkEvents)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if err := talks.Add(s); err != nil {
					t.Fatal(err)
				}
				added := len(talks.Events()) - 1
				if err := talks.SetDialogue(added, 0x900); err != nil {
					t.Fatal(err)
				}
				generated, planErr := s.PlanEventEdit(talks)
				if planErr != nil {
					t.Fatal(planErr)
				}
				if err := s.ApplyEventEdit(generated); err != nil {
					t.Fatal(err)
				}
				events, readErr := s.ChapterEvents(1, game.TalkEvents)
				if readErr != nil {
					t.Fatal(readErr)
				}
				chapter, site = 1, events.Events[added].Address+4
			}
			structure, err := s.NewScriptDraft(chapter, site)
			if err != nil {
				t.Fatal(err)
			}
			if !structure.Relocatable() {
				t.Fatal("fixed native minimal script is not relocatable")
			}
			original := s.Document.Snapshot()
			count := len(structure.Instructions())
			if err := structure.Insert(0, game.ScriptWait); err != nil {
				t.Fatal(err)
			}
			if err := structure.Set(0, game.ScriptFrames, 7); err != nil {
				t.Fatal(err)
			}
			move, err := s.PlanScriptEdit(structure)
			if err != nil {
				t.Fatal(err)
			}
			if !move.Relocated() {
				t.Fatal("structural edit did not append")
			}
			if err := s.ApplyScriptEdit(move); err != nil {
				t.Fatal(err)
			}
			current, err := s.NewScriptDraft(chapter, site)
			if err != nil {
				t.Fatal(err)
			}
			if current.Entry().Address != move.Destination() ||
				len(current.Instructions()) != count+1 {
				t.Fatal("structural script after apply")
			}
			data := s.Document.Snapshot()
			prefix := bytes.Clone(data.Data[:len(original.Data)])
			copy(prefix[site:site+4], original.Data[site:site+4])
			if !bytes.Equal(prefix, original.Data) {
				t.Fatal("old native script or independent references changed")
			}
			output := filepath.Join(t.TempDir(), "scripts.gba")
			if err := s.Document.SaveAs(output, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(output)
			if err != nil {
				t.Fatal(err)
			}
			next, err := reopened.NewScriptDraft(chapter, site)
			if err != nil {
				t.Fatal(err)
			}
			if value, _ := next.Value(0, game.ScriptFrames); value != 7 {
				t.Fatal("native script save/reopen")
			}
			if err := next.Duplicate(0); err != nil {
				t.Fatal(err)
			}
			if err := next.Remove(1); err != nil {
				t.Fatal(err)
			}
			if err := next.Set(0, game.ScriptFrames, 8); err != nil {
				t.Fatal(err)
			}
			continued, err := reopened.PlanScriptEdit(next)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.ApplyScriptEdit(continued); err != nil {
				t.Fatal(err)
			}
		})
	}
}
