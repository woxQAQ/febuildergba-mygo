package game

import (
	"bytes"
	"errors"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func TestTurnChapterContext(t *testing.T) {
	for _, fixture := range gameFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			f := testutil.NewTurnFixture(fixture.name)
			const aliasHeader, manual = 0x244400, 0x244000
			header := f.GetPosition("deploy_header")
			raw := f.Bytes()
			f.WriteAt(aliasHeader, raw[header:header+80])
			f.WritePointer(f.GetPosition("deploy_list")+8, aliasHeader)
			s := openFixture(t, f)
			table := s.Table(Chapters)
			chapter := table.Rows[0]
			current := f.Bytes()
			f.WriteAt(
				manual,
				current[chapter.Address:chapter.Address+table.Stride],
			)
			// The third event entry belongs to the copied manual chapter.
			f.WritePointer(f.GetPosition("deploy_list")+12, aliasHeader)
			eventOffset := 116
			if fixture.name == "FE7U" {
				eventOffset = 120
			}
			if fixture.name == "FE6" {
				eventOffset = 58
			}
			f.WriteU8(manual+eventOffset, 3)
			s = openFixture(t, f)
			d, err := s.NewTurnDraft(0)
			if err != nil {
				t.Fatal(err)
			}
			index := 0
			if fixture.name == "FE7U" || fixture.name == "FE7J" {
				index = 1
			}
			if err := d.Set(index, TurnStart, 9); err != nil {
				t.Fatal(err)
			}
			plan, err := s.PlanTurnEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.Uses()) != 1 || plan.Uses()[0] != 0 {
				t.Fatal("separate header alias treated as a shared edit target")
			}
			if err := s.ApplyTurnEdit(plan); err != nil {
				t.Fatal(err)
			}
			unchanged, err := s.ChapterTurns(1)
			if err != nil ||
				unchanged.Address != f.GetPosition("deploy_turns") ||
				unchanged.Events[index].Start != 3 {
				t.Fatal("separate header alias changed")
			}
			move, err := s.PlanTableMove(Chapters, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyTableMove(move); err != nil {
				t.Fatal(err)
			}
			relocated, err := s.ChapterTurns(0)
			if err != nil || relocated.Events[index].Start != 9 {
				t.Fatal("chapter relocation lost turn context")
			}
			draft, err := s.NewTurnDraft(0)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.UseLayout(
				Chapters,
				Layout{Address: manual, Stride: table.Stride, Count: 1},
			); err != nil {
				t.Fatal(err)
			}
			if draft.Current(s) {
				t.Fatal("manual chapter layout did not invalidate draft")
			}
			manualTurns, err := s.ChapterTurns(0)
			if err != nil || manualTurns.Pointer != aliasHeader ||
				manualTurns.Events[index].Start != 3 {
				t.Fatal(
					"manual chapter layout did not select its actual event header",
				)
			}
			manualDraft, err := s.NewTurnDraft(0)
			if err != nil {
				t.Fatal(err)
			}
			if err := manualDraft.Add(s); err != nil {
				t.Fatal(err)
			}
			manualPlan, err := s.PlanTurnEdit(manualDraft)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyTurnEdit(manualPlan); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTurnResourceRecovery(t *testing.T) {
	for _, name := range []string{"empty pointer", "malformed terminator", "unterminated", "unresolved script", "declared table space", "declared script space", "declared units space"} {
		t.Run(name, func(t *testing.T) {
			f := testutil.NewTurnFixture("FE8U")
			header := f.GetPosition("deploy_header")
			switch name {
			case "empty pointer":
				f.WriteU32(header, 0)
			case "malformed terminator":
				f.WriteU32(f.GetPosition("turn_terminator"), 0x12340000)
			case "unterminated":
				for i := range 257 {
					f.WriteAt(
						f.GetPosition("deploy_turns")+i*12,
						[]byte{2, 0, 0, 0, 0, 0, 0, 0, 3, 0, 0x80, 0},
					)
				}
			case "unresolved script":
				f.WriteU32(f.GetPosition("turn_condition")+4, 0xDEADBEEF)
			}
			s := openFixture(t, f)
			before := s.Document.Snapshot()
			d, err := s.NewTurnDraft(0)
			if name == "malformed terminator" || name == "unterminated" {
				if err == nil || !errors.Is(err, ErrTurnEvents) ||
					!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
					t.Fatalf("invalid condition table: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if name == "unresolved script" {
				if len(d.Events()[0].Warnings) == 0 {
					t.Fatal("unresolved script not diagnosed")
				}
				if err := d.Set(0, TurnStart, 7); err != nil {
					t.Fatal(err)
				}
			}
			if name == "declared table space" ||
				name == "declared script space" ||
				name == "declared units space" {
				at := f.GetPosition("deploy_turns")
				if name == "declared script space" {
					at = f.GetPosition("deploy_script")
				}
				if name == "declared units space" {
					at = f.GetPosition("deploy_units")
				}
				region := space.Range{Start: at, End: at + 0x100}
				_, err := s.PlanTableMove(
					Items,
					[]space.FreeRange{
						{Range: region, Source: "fixture ownership conflict"},
					},
				)
				if !errors.Is(err, space.ErrRange) ||
					!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
					t.Fatalf("live event resource reused: %v", err)
				}
				return
			}
			if err := d.Add(s); err != nil {
				t.Fatal(err)
			}
			plan, err := s.PlanTurnEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyTurnEdit(plan); err != nil {
				t.Fatal(err)
			}
			result, err := s.ChapterTurns(0)
			if err != nil {
				t.Fatal(err)
			}
			if name == "empty pointer" && len(result.Events) != 1 {
				t.Fatal(
					"empty condition list did not accept a new reinforcement",
				)
			}
			if name == "unresolved script" &&
				(!bytes.Equal(result.Events[0].Raw[4:8], []byte{0xEF, 0xBE, 0xAD, 0xDE}) || result.Events[0].Start != 7) {
				t.Fatal(
					"unresolved script was rewritten or hid editable conditions",
				)
			}
		})
	}
}
