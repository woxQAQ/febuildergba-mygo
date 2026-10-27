package game

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func TestDeploymentEditingWorkflow(t *testing.T) {
	for _, version := range []string{"FE8U", "FE8J", "FE7U", "FE7J", "FE6"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewDeploymentFixture(version)
			s := openFixture(t, f)
			groups, err := s.ChapterDeployments(0)
			if err != nil || len(groups.Warnings) != 0 ||
				len(groups.Groups) < 3 {
				t.Fatalf("deployment entries: %+v, %v", groups, err)
			}
			group := groups.Groups[0]
			first := DeploymentUnit{
				Character: 1,
				Class:     2,
				Level:     13,
				Faction:   2,
				Start:     image.Pt(1, 2),
				End:       image.Pt(4, 8),
			}
			if group.Stride == 20 {
				first.MovementCount = 2
			}
			if group.Err != nil ||
				group.Pointer != f.GetPosition("deploy_first") ||
				group.Address != f.GetPosition("deploy_units") ||
				len(group.Units) != 2 ||
				group.Units[0] != first {
				t.Fatalf("wrong native deployment record: %+v", group)
			}
			foundScript := slices.ContainsFunc(
				groups.Groups,
				func(group DeploymentGroup) bool {
					return group.Pointer == f.GetPosition(
						"deploy_script_site",
					) &&
						len(group.Units) == 2
				},
			)
			if !foundScript {
				t.Fatal("opening or first-turn typed load was not discovered")
			}
			before := s.Document.Snapshot()
			draft, err := s.NewDeploymentDraft(0, group.Pointer)
			if err != nil {
				t.Fatal(err)
			}
			for _, edit := range []struct {
				field string
				value int64
			}{
				{"deploy_class", 5},
				{"deploy_leader", 7},
				{"deploy_level", 17},
				{"deploy_faction", 1},
				{"deploy_autolevel", 0},
				{"deploy_start_x", 5},
				{"deploy_start_y", 6},
				{"deploy_end_x", 7},
				{"deploy_end_y", 9},
				{"deploy_item_1", 0x32},
				{"deploy_ai_1", 0xAB},
			} {
				if err := draft.Set(0, edit.field, edit.value); err != nil {
					t.Fatal(err)
				}
			}
			if group.Stride == 20 {
				if err := draft.Set(0, "deploy_position_flags", 3); err != nil {
					t.Fatal(err)
				}
			}
			if err := draft.Add(); err != nil {
				t.Fatal(err)
			}
			if err := draft.Duplicate(0); err != nil {
				t.Fatal(err)
			}
			if err := draft.Remove(1); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				before.Revision != s.Document.State().Revision {
				t.Fatal("draft changed the document before application")
			}
			preview, err := s.PlanDeploymentEdit(draft)
			if err != nil || preview.Before() != 2 || preview.After() != 3 ||
				preview.Destination() != len(before.Data) ||
				len(preview.Uses()) != 2 {
				t.Fatalf("append/reference preview: %+v, %v", preview, err)
			}
			if err := draft.Set(0, "deploy_level", 20); err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyDeploymentEdit(preview); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			expected := bytes.Clone(before.Data)
			binary.LittleEndian.PutUint32(
				expected[group.Pointer:],
				uint32(0x08000000+preview.Destination()),
			)
			if !bytes.Equal(expected, after.Data[:len(expected)]) {
				t.Fatal(
					"deployment edit overwrote bytes outside the selected pointer",
				)
			}
			var record []byte
			if group.Stride == 20 {
				record = []byte{
					1,
					5,
					7,
					0x8A,
					0x85,
					0xB1,
					0xA7,
					2,
					0,
					0,
					0,
					0,
					0x32,
					0x22,
					0x33,
					0x44,
					0xAB,
					0x66,
					0x77,
					0x88,
				}
				binary.LittleEndian.PutUint32(
					record[8:],
					uint32(0x08000000+preview.Destination()+64),
				)
				path := []byte{
					1,
					0xA1,
					0x40,
					0,
					0xDE,
					0xAD,
					0x34,
					0x12,
					0x47,
					0xB2,
					0x50,
					0,
					0xBE,
					0xEF,
					0x78,
					0x56,
				}
				if !bytes.Equal(
					path,
					after.Data[preview.Destination()+64:preview.Destination()+80],
				) ||
					!bytes.Equal(
						path,
						after.Data[preview.Destination()+80:preview.Destination()+96],
					) {
					t.Fatal(
						"last-step edit lost earlier coordinates, packed flags, speed, target, delay or unknown movement bytes",
					)
				}
			} else {
				record = []byte{
					1,
					5,
					7,
					0x8A,
					5,
					6,
					7,
					9,
					0x32,
					0x22,
					0x33,
					0x44,
					0xAB,
					0x66,
					0x77,
					0x88,
				}
			}
			if !bytes.Equal(
				record,
				after.Data[preview.Destination():preview.Destination()+group.Stride],
			) ||
				!bytes.Equal(
					[]byte{0, 0xD1, 0xE2, 0xF3},
					after.Data[preview.Destination()+3*group.Stride:preview.Destination()+3*group.Stride+4],
				) {
				t.Fatal(
					"fixed native record bytes or terminator did not survive the edit",
				)
			}
			for _, chapter := range []int{0, 1} {
				result, err := s.ChapterDeployments(chapter)
				if err != nil ||
					result.Groups[0].Address != preview.Destination() ||
					len(result.Groups[0].Units) != 3 ||
					result.Groups[0].Units[0].Level != 17 ||
					result.Groups[1].Address != group.Address ||
					len(result.Groups[1].Units) != 2 {
					t.Fatalf(
						"shared site or independent aliases: %+v, %v",
						result,
						err,
					)
				}
			}
			if err := s.ApplyDeploymentEdit(
				preview,
			); !errors.Is(
				err,
				rom.ErrStale,
			) {
				t.Fatalf("reused preview: %v", err)
			}
			if _, err := s.PlanDeploymentEdit(
				draft,
			); !errors.Is(
				err,
				rom.ErrStale,
			) {
				t.Fatalf("stale draft: %v", err)
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				!s.Document.Redo() ||
				!bytes.Equal(after.Data, s.Document.Snapshot().Data) {
				t.Fatal(
					"deployment records, pointers, paths and ROM length were not one history operation",
				)
			}
			s.Refresh()
			if err := s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(s.Document.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			result, err := reopened.ChapterDeployments(0)
			if err != nil ||
				result.Groups[0].Address != preview.Destination() ||
				len(result.Groups[0].Units) != 3 ||
				result.Groups[0].Units[0].Start != image.Pt(5, 6) ||
				result.Groups[0].Units[0].End != image.Pt(7, 9) {
				t.Fatalf("deployment did not reopen: %+v, %v", result, err)
			}
			next, err := reopened.NewDeploymentDraft(0, group.Pointer)
			if err != nil {
				t.Fatal(err)
			}
			for next.Count() > 0 {
				if err := next.Remove(0); err != nil {
					t.Fatal(err)
				}
			}
			empty, err := reopened.PlanDeploymentEdit(next)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.ApplyDeploymentEdit(empty); err != nil {
				t.Fatal(err)
			}
			result, err = reopened.ChapterDeployments(0)
			if err != nil || len(result.Groups[0].Units) != 0 {
				t.Fatalf("empty deployment list: %+v, %v", result, err)
			}
		})
	}
}

func TestDeploymentRecoveryAndIsolation(t *testing.T) {
	f := testutil.NewDeploymentFixture("FE8U")
	f.WriteU32(f.GetPosition("deploy_first"), 0x02000000)
	s := openFixture(t, f)
	groups, err := s.ChapterDeployments(0)
	if err != nil || !errors.Is(groups.Groups[0].Err, ErrDeployment) ||
		groups.Groups[1].Err != nil {
		t.Fatalf(
			"damaged group blocked independent entries: %+v, %v",
			groups,
			err,
		)
	}
	if _, err := s.NewDeploymentDraft(
		0,
		groups.Groups[0].Pointer,
	); !errors.Is(
		err,
		ErrDeployment,
	) {
		t.Fatalf("damaged group was editable: %v", err)
	}
	draft, err := s.NewDeploymentDraft(0, groups.Groups[1].Pointer)
	if err != nil {
		t.Fatal(err)
	}
	stable := s.Document.Snapshot()
	for _, edit := range []struct {
		index int
		field string
		value int64
	}{
		{0, "deploy_character", 0},
		{0, "deploy_level", 32},
		{0, "deploy_faction", 4},
		{0, "deploy_start_x", 64},
		{0, "deploy_item_1", 256},
		{0, "not_a_field", 0},
		{99, "deploy_level", 1},
	} {
		if err := draft.Set(edit.index, edit.field, edit.value); err == nil {
			t.Fatalf("invalid draft edit accepted: %+v", edit)
		}
	}
	if draft.Modified() ||
		!bytes.Equal(stable.Data, s.Document.Snapshot().Data) {
		t.Fatal("invalid input mutated source or draft")
	}
	if err := draft.Set(0, "deploy_level", 9); err != nil {
		t.Fatal(err)
	}
	preview, err := s.PlanDeploymentEdit(draft)
	if err != nil {
		t.Fatal(err)
	}
	foreign := openFixture(t, testutil.NewDeploymentFixture("FE8U"))
	if err := foreign.ApplyDeploymentEdit(
		preview,
	); !errors.Is(
		err,
		rom.ErrStale,
	) {
		t.Fatalf("foreign preview: %v", err)
	}
	layout := s.Table(Classes).Layout
	if err := s.UseLayout(Classes, layout); err != nil {
		t.Fatal(err)
	}
	if draft.Current(s) {
		t.Fatal("manual interpretation did not invalidate the deployment draft")
	}
	if err := s.ApplyDeploymentEdit(preview); !errors.Is(err, rom.ErrStale) {
		t.Fatalf("layout changed without a document revision: %v", err)
	}
	s.UseAutomaticLayout(Classes)
	if err := s.Set(Units, []int{1}, "base.hp", 20); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PlanDeploymentEdit(draft); !errors.Is(err, rom.ErrStale) {
		t.Fatalf("changed ROM did not invalidate draft: %v", err)
	}
	reloaded, err := s.NewDeploymentDraft(0, groups.Groups[1].Pointer)
	if err != nil {
		t.Fatal(err)
	}
	if err := reloaded.Set(1, "deploy_end_x", 6); err != nil {
		t.Fatal(err)
	}
	if err := reloaded.Set(1, "deploy_end_y", 7); err != nil {
		t.Fatal(err)
	}
	preview, err = s.PlanDeploymentEdit(reloaded)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyDeploymentEdit(preview); err != nil {
		t.Fatal(err)
	}
	groups, err = s.ChapterDeployments(0)
	if err != nil || groups.Groups[1].Units[1].End != image.Pt(6, 7) ||
		groups.Groups[1].Units[1].MovementCount != 1 {
		t.Fatalf(
			"zero-path unit did not receive a valid native path: %+v, %v",
			groups,
			err,
		)
	}
	data := s.Document.Snapshot().Data
	unit := groups.Groups[1].Address + 20
	path := int(binary.LittleEndian.Uint32(data[unit+8:])) - 0x08000000
	if !bytes.Equal(
		[]byte{0xC6, 1, 0, 0, 0xFF, 0xFF, 0, 0},
		data[path:path+8],
	) {
		t.Fatal("new FE8 movement step has incorrect native defaults")
	}
}

func TestDeploymentResourceBoundaries(t *testing.T) {
	t.Run("missing terminator", func(t *testing.T) {
		f := testutil.NewDeploymentFixture("FE7U")
		f.WriteAt(
			f.GetPosition("deploy_units"),
			bytes.Repeat([]byte{0x11}, 257*16),
		)
		s := openFixture(t, f)
		result, err := s.ChapterDeployments(0)
		if err != nil || !errors.Is(result.Groups[0].Err, ErrDeployment) {
			t.Fatalf("unterminated units were accepted: %+v, %v", result, err)
		}
	})
	t.Run("bad movement", func(t *testing.T) {
		f := testutil.NewDeploymentFixture("FE8U")
		f.WriteU32(f.GetPosition("deploy_units")+8, 0x09FFFFFC)
		s := openFixture(t, f)
		result, err := s.ChapterDeployments(0)
		if err != nil || !errors.Is(result.Groups[0].Err, ErrDeployment) {
			t.Fatalf("truncated movement accepted: %+v, %v", result, err)
		}
	})
	t.Run("protected pointer", func(t *testing.T) {
		f := testutil.NewDeploymentFixture("FE8U")
		f.WritePointer(
			f.GetPosition("deploy_list")+4,
			f.GetPosition("units")-40,
		)
		f.WritePointer(f.GetPosition("units"), f.GetPosition("deploy_units"))
		s := openFixture(t, f)
		draft, err := s.NewDeploymentDraft(0, f.GetPosition("units"))
		if err != nil {
			t.Fatal(err)
		}
		if err := draft.Set(0, "deploy_level", 7); err != nil {
			t.Fatal(err)
		}
		if _, err := s.PlanDeploymentEdit(
			draft,
		); !errors.Is(
			err,
			ErrDeployment,
		) {
			t.Fatalf("deployment pointer overwrote a known table: %v", err)
		}
	})
	t.Run("capacity", func(t *testing.T) {
		s := openFixture(t, testutil.NewDeploymentFixture("FE8U"))
		image := s.Document.Snapshot()
		if err := s.Document.ApplyEdit(
			image.Revision,
			"fill",
			rom.Edit{Size: rom.MaxSize},
		); err != nil {
			t.Fatal(err)
		}
		groups, err := s.ChapterDeployments(0)
		if err != nil {
			t.Fatal(err)
		}
		draft, err := s.NewDeploymentDraft(0, groups.Groups[0].Pointer)
		if err != nil {
			t.Fatal(err)
		}
		if err := draft.Set(0, "deploy_level", 5); err != nil {
			t.Fatal(err)
		}
		before := s.Document.Snapshot()
		if _, err := s.PlanDeploymentEdit(
			draft,
		); !errors.Is(
			err,
			space.ErrCapacity,
		) {
			t.Fatalf("full ROM accepted append: %v", err)
		}
		if before.Revision != s.Document.State().Revision ||
			!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
			t.Fatal("capacity failure changed the ROM")
		}
	})
}
