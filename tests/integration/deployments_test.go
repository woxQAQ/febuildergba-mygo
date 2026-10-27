//go:build integration

package integration

import (
	"bytes"
	"encoding/binary"
	"image"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
)

func TestRealInitialDeployments(t *testing.T) {
	// Sites, native counts and first records are fixed independently of discovery.
	samples := map[string]struct {
		group, site, address, count, eventSite, eventCount int
		first                                              game.DeploymentUnit
	}{
		"FE8U": {
			0,
			0x9E87C4,
			0x8B40A0,
			2,
			0x9EF850,
			7,
			game.DeploymentUnit{
				Character:     1,
				Class:         2,
				Level:         1,
				Faction:       0,
				Start:         image.Pt(14, 9),
				End:           image.Pt(12, 9),
				MovementCount: 1,
			},
		},
		"FE8J": {
			0,
			0xA5A8E8,
			0x908628,
			2,
			0xA61988,
			7,
			game.DeploymentUnit{
				Character:     1,
				Class:         2,
				Level:         1,
				Faction:       0,
				Start:         image.Pt(14, 9),
				End:           image.Pt(12, 9),
				MovementCount: 1,
			},
		},
		"FE7U": {
			0,
			0xCA098C,
			0xCC5BF0,
			1,
			0xCA8394,
			4,
			game.DeploymentUnit{
				Character: 0x89,
				Class:     0x39,
				Level:     4,
				Faction:   2,
				Start:     image.Pt(13, 1),
				End:       image.Pt(13, 1),
			},
		},
		"FE7J": {
			0,
			0xD6895C,
			0xD8E184,
			1,
			0xD705F8,
			4,
			game.DeploymentUnit{
				Character: 0x89,
				Class:     0x39,
				Level:     4,
				Faction:   2,
				Start:     image.Pt(13, 1),
				End:       image.Pt(13, 1),
			},
		},
		"FE6": {
			1,
			0x6677B4,
			0x679B40,
			2,
			0x66B600,
			5,
			game.DeploymentUnit{
				Character: 1,
				Class:     0,
				Level:     1,
				Faction:   0,
				Start:     image.Pt(1, 20),
				End:       image.Pt(2, 18),
			},
		},
	}
	for _, fixture := range realROMFixtures {
		t.Run(fixture.id, func(t *testing.T) {
			sample := samples[fixture.id]
			s := openRealROM(t, fixture.filename)
			const chapter = 1
			result, err := s.ChapterDeployments(chapter)
			if err != nil || len(result.Groups) <= sample.group {
				t.Fatalf("native deployment discovery: %+v, %v", result, err)
			}
			group := result.Groups[sample.group]
			if group.Err != nil || group.Pointer != sample.site ||
				group.Address != sample.address ||
				len(group.Units) != sample.count ||
				group.Units[0] != sample.first {
				t.Fatalf("fixed initial record: %+v", group)
			}
			if !slices.ContainsFunc(
				result.Groups,
				func(group game.DeploymentGroup) bool {
					return group.Pointer == sample.eventSite &&
						group.Err == nil &&
						len(group.Units) == sample.eventCount
				},
			) {
				t.Fatalf("fixed event-loaded army was missed: %+v", result)
			}
			before := s.Document.Snapshot()
			draft, err := s.NewDeploymentDraft(chapter, sample.site)
			if err != nil {
				t.Fatal(err)
			}
			item, err := draft.Value(0, "deploy_item_1")
			if err != nil {
				t.Fatal(err)
			}
			for _, edit := range []struct {
				field string
				value int64
			}{
				{"deploy_level", int64(sample.first.Level + 1)},
				{"deploy_start_x", int64(sample.first.Start.X + 1)},
				{"deploy_end_y", int64(sample.first.End.Y + 1)},
				{"deploy_item_1", (item + 1) & 0xFF},
				{"deploy_ai_4", 3},
			} {
				if err := draft.Set(0, edit.field, edit.value); err != nil {
					t.Fatal(err)
				}
			}
			if err := draft.Duplicate(0); err != nil {
				t.Fatal(err)
			}
			if err := draft.Add(); err != nil {
				t.Fatal(err)
			}
			plan, err := s.PlanDeploymentEdit(draft)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("native preview wrote before confirmation")
			}
			if err := s.ApplyDeploymentEdit(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			expected := bytes.Clone(before.Data)
			binary.LittleEndian.PutUint32(
				expected[sample.site:],
				uint32(0x08000000+plan.Destination()),
			)
			if !bytes.Equal(expected, after.Data[:len(before.Data)]) {
				t.Fatal(
					"native edit changed old resources or independent aliases",
				)
			}
			raw := after.Data[plan.Destination() : plan.Destination()+group.Stride]
			if raw[3] != before.Data[sample.address+3]+8 {
				t.Fatal("native packed level damaged faction or growth flags")
			}
			if group.Stride == 20 {
				oldPath := int(
					binary.LittleEndian.Uint32(before.Data[sample.address+8:]),
				) - 0x08000000
				newPath := int(binary.LittleEndian.Uint32(raw[8:])) - 0x08000000
				length := int(raw[7]) * 8
				path := bytes.Clone(before.Data[oldPath : oldPath+length])
				// Y occupies bits 6..11. The fixed sample increments Y without
				// crossing its packed range, leaving every other path byte intact.
				word := binary.LittleEndian.Uint16(path[length-8:])
				binary.LittleEndian.PutUint16(path[length-8:], word+64)
				if !bytes.Equal(path, after.Data[newPath:newPath+length]) ||
					raw[6] != before.Data[sample.address+6] {
					t.Fatal("native movement or unknown unit flags were lost")
				}
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				!s.Document.Redo() ||
				!bytes.Equal(after.Data, s.Document.Snapshot().Data) {
				t.Fatal("native deployment edit was not atomic history")
			}
			s.Refresh()
			if err := s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(s.Document.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			result, err = reopened.ChapterDeployments(chapter)
			if err != nil {
				t.Fatal(err)
			}
			group = result.Groups[sample.group]
			if group.Address != plan.Destination() ||
				len(group.Units) != sample.count+2 ||
				group.Units[0].Level != sample.first.Level+1 ||
				group.Units[0].Start.X != sample.first.Start.X+1 ||
				group.Units[0].End.Y != sample.first.End.Y+1 {
				t.Fatalf("saved deployment did not reopen: %+v", group)
			}
			next, err := reopened.NewDeploymentDraft(chapter, sample.site)
			if err != nil {
				t.Fatal(err)
			}
			if err := next.Remove(next.Count() - 1); err != nil {
				t.Fatal(err)
			}
			remove, err := reopened.PlanDeploymentEdit(next)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.ApplyDeploymentEdit(remove); err != nil {
				t.Fatal(err)
			}
			result, err = reopened.ChapterDeployments(chapter)
			if err != nil ||
				len(result.Groups[sample.group].Units) != sample.count+1 {
				t.Fatalf(
					"continue editing reopened deployment: %+v, %v",
					result,
					err,
				)
			}
			t.Logf(
				"chapter %02X, entry %08X, %d native units, %d event-loaded units, %d known uses",
				chapter,
				sample.site,
				sample.count,
				sample.eventCount,
				len(plan.Uses()),
			)
		})
	}
}
