package game

import (
	"bytes"
	"errors"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func TestDeploymentChapterContext(t *testing.T) {
	for _, version := range []string{"FE8U", "FE8J", "FE7U", "FE7J", "FE6"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewDeploymentFixture(version)
			const manual, otherHeader, otherUnits = 0x244000, 0x244400, 0x244800
			chapterStride, eventOffset, firstOffset := 148, 116, 40
			switch version {
			case "FE7U":
				chapterStride, eventOffset, firstOffset = 152, 120, 24
			case "FE7J":
				firstOffset = 24
			case "FE6":
				chapterStride, eventOffset, firstOffset = 68, 58, 16
			}
			raw := f.Bytes()
			f.WriteAt(
				manual,
				raw[f.GetPosition("chapters"):f.GetPosition("chapters")+chapterStride],
			)
			f.WriteU8(manual+eventOffset, 3)
			f.WritePointer(f.GetPosition("deploy_list")+12, otherHeader)
			f.WritePointer(otherHeader+firstOffset, otherUnits)
			f.WriteAt(otherUnits, []byte{9, 0, 0, 8})
			s := openFixture(t, f)
			original, err := s.NewDeploymentDraft(
				0,
				f.GetPosition("deploy_first"),
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := original.Set(0, "deploy_level", 5); err != nil {
				t.Fatal(err)
			}
			move, err := s.PlanTableMove(Chapters, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyTableMove(move); err != nil {
				t.Fatal(err)
			}
			if original.Current(s) {
				t.Fatal(
					"relocated chapter table left a stale deployment draft active",
				)
			}
			result, err := s.ChapterDeployments(0)
			if err != nil ||
				result.Groups[0].Pointer != f.GetPosition("deploy_first") ||
				result.Groups[0].Units[0].Character != 1 {
				t.Fatalf(
					"relocated chapter lost deployment context: %+v, %v",
					result,
					err,
				)
			}
			if err := s.UseLayout(
				Chapters,
				Layout{Address: manual, Stride: chapterStride, Count: 1},
			); err != nil {
				t.Fatal(err)
			}
			result, err = s.ChapterDeployments(0)
			if err != nil ||
				result.Groups[0].Pointer != otherHeader+firstOffset ||
				len(result.Groups[0].Units) != 1 ||
				result.Groups[0].Units[0].Character != 9 {
				t.Fatalf(
					"manual chapter layout was not followed: %+v, %v",
					result,
					err,
				)
			}
			draft, err := s.NewDeploymentDraft(0, result.Groups[0].Pointer)
			if err != nil {
				t.Fatal(err)
			}
			if err := draft.Set(0, "deploy_level", 8); err != nil {
				t.Fatal(err)
			}
			plan, err := s.PlanDeploymentEdit(draft)
			if err != nil {
				t.Fatal(err)
			}
			s.UseAutomaticLayout(Chapters)
			if err := s.ApplyDeploymentEdit(
				plan,
			); !errors.Is(
				err,
				rom.ErrStale,
			) {
				t.Fatalf(
					"manual-layout preview applied to a different chapter context: %v",
					err,
				)
			}
			result, err = s.ChapterDeployments(0)
			if err != nil || result.Groups[0].Units[0].Character != 1 {
				t.Fatalf(
					"automatic chapter context did not recover: %+v, %v",
					result,
					err,
				)
			}
		})
	}
}

func TestDeploymentDeclaredSpaceProtection(t *testing.T) {
	f := testutil.NewDeploymentFixture("FE8U")
	s := openFixture(t, f)
	before := s.Document.Snapshot()
	for _, address := range []int{f.GetPosition("deploy_first"), f.GetPosition("deploy_units"), f.GetPosition("deploy_path")} {
		candidate := space.Range{Start: address, End: address + 0x400}
		_, err := s.PlanTableMove(
			Units,
			[]space.FreeRange{
				{
					Range:  candidate,
					Source: "explicitly declared candidate",
				},
			},
		)
		if !errors.Is(err, space.ErrRange) {
			t.Fatalf(
				"live deployment bytes accepted as reusable space at %X: %v",
				address,
				err,
			)
		}
	}
	if before.Revision != s.Document.State().Revision ||
		!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
		t.Fatal("rejected resource move changed live deployment data")
	}
}

func TestCreateDeploymentInEmptyEntry(t *testing.T) {
	f := testutil.NewDeploymentFixture("FE8U")
	s := openFixture(t, f)
	result, err := s.ChapterDeployments(0)
	if err != nil {
		t.Fatal(err)
	}
	index := slices.IndexFunc(
		result.Groups,
		func(group DeploymentGroup) bool { return group.Address == 0 && group.Err == nil },
	)
	if index < 0 {
		t.Fatal("fixture has no empty entry")
	}
	group := result.Groups[index]
	before := s.Document.Snapshot()
	draft, err := s.NewDeploymentDraft(0, group.Pointer)
	if err != nil {
		t.Fatal(err)
	}
	if err := draft.Add(); err != nil {
		t.Fatal(err)
	}
	if err := draft.Set(0, "deploy_character", 7); err != nil {
		t.Fatal(err)
	}
	plan, err := s.PlanDeploymentEdit(draft)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyDeploymentEdit(plan); err != nil {
		t.Fatal(err)
	}
	result, err = s.ChapterDeployments(0)
	if err != nil || len(result.Groups[index].Units) != 1 ||
		result.Groups[index].Units[0].Character != 7 {
		t.Fatalf("new deployment entry: %+v, %v", result, err)
	}
	if !s.Document.Undo() ||
		!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
		t.Fatal("creation did not undo to a null pointer and original length")
	}
}
