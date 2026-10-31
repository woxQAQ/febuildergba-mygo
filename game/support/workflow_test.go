package support_test

import (
	"bytes"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/support"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func TestSupportWorkflow(t *testing.T) {
	for _, version := range []string{"FE6", "FE7U", "FE7J", "FE8U", "FE8J"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewSupportFixture(version)
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			d, err := s.Supports.NewDraft(1)
			if err != nil {
				t.Fatal(err)
			}
			if got := d.Relationships(); !slices.Equal(
				got,
				[]support.Relationship{{Unit: 2, Initial: 20, Growth: 3}},
			) {
				t.Fatal(got)
			}
			original := d.Raw()
			if d.Modified() || len(d.Talks()) != 2 {
				t.Fatal("initial draft or native terminator")
			}
			if err = d.Set(
				0,
				support.Relationship{Unit: 2, Initial: 31, Growth: 5},
			); err != nil {
				t.Fatal(err)
			}
			if err = d.SetText(0, 0, 2); err != nil {
				t.Fatal(err)
			}
			plan, err := s.Supports.Plan(d, true)
			if err != nil {
				t.Fatal(err)
			}
			moves := plan.Moves()
			if len(moves) != 3 || len(moves[0].Shared) != 1 ||
				moves[0].Shared[0].ID != 3 ||
				!slices.Equal(
					moves[2].Pointers,
					f.TablePointers("support_talks"),
				) {
				t.Fatal("incomplete preview", moves)
			}
			moves[0].Pointers[0] = 0
			moves[0].Shared[0].ID = 99
			if plan.Moves()[0].Pointers[0] == 0 ||
				plan.Moves()[0].Shared[0].ID != 3 {
				t.Fatal("mutable plan")
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("preview wrote bytes")
			}
			// Changing the draft after preview must not mutate the frozen plan.
			if err = d.Set(
				0,
				support.Relationship{Unit: 2, Initial: 99, Growth: 5},
			); err != nil {
				t.Fatal(err)
			}
			if err = s.Supports.Apply(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			prefix := bytes.Clone(before.Data)
			for _, move := range plan.Moves() {
				for _, site := range move.Pointers {
					at, valid := gba.Pointer(after.Data, site)
					if !valid || at != move.After {
						t.Fatal("consumer not updated", site)
					}
					copy(prefix[site:site+4], after.Data[site:site+4])
				}
			}
			if !bytes.Equal(prefix, after.Data[:len(prefix)]) {
				t.Fatal(
					"changed old payload, independent pointer or unrelated bytes",
				)
			}
			for _, id := range []int{1, 2} {
				saved, err := s.Supports.NewDraft(id)
				if err != nil {
					t.Fatal(err)
				}
				if got := saved.Relationships()[0]; got.Initial != 31 ||
					got.Growth != 5 {
					t.Fatal("reverse or frozen values", got)
				}
				expected := bytes.Clone(original)
				if id == 2 {
					expected[0] = 1
				}
				expected[d.Capacity()], expected[2*d.Capacity()] = 31, 5
				if !bytes.Equal(saved.Raw(), expected) {
					t.Fatal("lost opaque inactive slots or tail")
				}
			}
			independent, err := s.Supports.NewDraft(3)
			if err != nil || !bytes.Equal(independent.Raw(), original) {
				t.Fatal("shared independent record changed", err)
			}
			talks, err := s.Supports.Talks()
			if err != nil {
				t.Fatal(err)
			}
			if len(talks) != 2 || talks[0].Texts != [3]uint32{2, 1, 1} ||
				talks[1].Texts != [3]uint32{1, 1, 1} {
				t.Fatal("dialog values or duplicate order")
			}
			expectedTalk := bytes.Clone(
				before.Data[f.GetPosition("support_talks") : f.GetPosition("support_talks")+3*len(talks[0].Raw)],
			)
			expectedTalk[4] = 2
			movedTalk := plan.Moves()[2].After
			if !bytes.Equal(
				expectedTalk,
				after.Data[movedTalk:movedTalk+len(expectedTalk)],
			) {
				t.Fatal("dialog padding, music or terminator changed")
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				!s.Document.Redo() ||
				!bytes.Equal(after.Data, s.Document.Snapshot().Data) {
				t.Fatal("non-atomic history")
			}
			path := filepath.Join(t.TempDir(), "support.gba")
			if err = s.Document.SaveAs(path, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			saved, err := reopened.Supports.NewDraft(1)
			if err != nil {
				t.Fatal(err)
			}
			if saved.Relationships()[0].Initial != 31 ||
				saved.Talks()[0].Texts[0] != 2 {
				t.Fatal("save/reopen")
			}
			uses := reopened.Texts.Uses(2)
			found := false
			for _, use := range uses {
				if use.Kind == support.TalkUse && use.ID == 0 &&
					use.Field == "support_c" {
					found = true
				}
			}
			if !found {
				t.Fatal("missing text use")
			}
		})
	}
}

func TestSupportStructureAndSynchronization(t *testing.T) {
	for _, version := range []string{"FE6", "FE7U", "FE7J", "FE8U", "FE8J"} {
		t.Run(version, func(t *testing.T) {
			s := testutil.Open(t, testutil.NewSupportFixture(version))
			d, err := s.Supports.NewDraft(4)
			if err != nil {
				t.Fatal(err)
			}
			if err = d.Add(
				support.Relationship{Unit: 1, Initial: 8, Growth: 2},
			); err != nil {
				t.Fatal(err)
			}
			if err = d.Add(support.Relationship{Unit: 2}); err != nil {
				t.Fatal(err)
			}
			if err = d.Remove(1); err != nil {
				t.Fatal(err)
			}
			plan, err := s.Supports.Plan(d, true)
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.Missing()) != 1 || plan.Missing()[0].ID != 1 ||
				len(plan.Moves()) != 1 {
				t.Fatal("missing reverse relationship was invented")
			}
			if err = s.Supports.Apply(plan); err != nil {
				t.Fatal(err)
			}
			d, err = s.Supports.NewDraft(4)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(
				d.Relationships(),
				[]support.Relationship{{Unit: 1, Initial: 8, Growth: 2}},
			) {
				t.Fatal("initialization")
			}
			if err = d.Remove(0); err != nil {
				t.Fatal(err)
			}
			plan, err = s.Supports.Plan(d, true)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.Supports.Apply(plan); err != nil {
				t.Fatal(err)
			}
			d, err = s.Supports.NewDraft(4)
			if err != nil || len(d.Relationships()) != 0 {
				t.Fatal("empty native record", err)
			}
			for i := 0; i < d.Capacity(); i++ {
				if err = d.Add(support.Relationship{Unit: 1}); err != nil {
					t.Fatal(err)
				}
			}
			if err = d.Add(
				support.Relationship{Unit: 1},
			); !errors.Is(
				err,
				space.ErrCapacity,
			) {
				t.Fatal("capacity", err)
			}
			if _, err = s.Supports.Plan(
				d,
				false,
			); !errors.Is(
				err,
				edit.ErrValue,
			) {
				t.Fatal("duplicate partners", err)
			}
		})
	}
}

func TestSupportConflictsAreAtomic(t *testing.T) {
	for _, scenario := range []string{"revision", "layout", "document", "text", "consumers", "count", "overlap", "capacity"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.NewSupportFixture("FE8U")
			if scenario == "consumers" {
				f.WritePointer(f.TablePointers("support_talks")[1], 0x282000)
			}
			if scenario == "count" {
				f.WriteU8(f.GetPosition("support_1")+21, 8)
			}
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			d, err := s.Supports.NewDraft(1)
			if scenario == "count" {
				if !errors.Is(err, support.ErrData) {
					t.Fatal(err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = d.Set(
				0,
				support.Relationship{Unit: 2, Initial: 35, Growth: 4},
			); err != nil {
				t.Fatal(err)
			}
			if scenario == "consumers" {
				if err = d.SetText(
					0,
					0,
					2,
				); !errors.Is(
					err,
					support.ErrReferences,
				) {
					t.Fatal(err)
				}
				// A broken global dialog consumer does not disable independent relationships.
				if _, err = s.Supports.Plan(d, false); err != nil {
					t.Fatal(err)
				}
				return
			}
			if scenario == "text" {
				if err = d.SetText(0, 0, 65535); err != nil {
					t.Fatal(err)
				}
			}
			expected := error(rom.ErrStale)
			switch scenario {
			case "document":
				s = testutil.Open(t, f)
			case "revision":
				if err = s.Records.Set(
					records.Units,
					[]int{1},
					"level",
					7,
				); err != nil {
					t.Fatal(err)
				}
			case "layout":
				table := s.Records.Table(records.Units)
				if err = s.Records.UseLayout(
					records.Units,
					records.Layout{
						Address: table.Address,
						Stride:  table.Stride,
						Count:   3,
					},
				); err != nil {
					t.Fatal(err)
				}
			case "text":
				expected = nil
			case "overlap":
				table := s.Records.Table(records.Classes)
				if err = s.Records.UseLayout(
					records.Classes,
					records.Layout{
						Address: f.GetPosition("support_pointer_1"),
						Stride:  table.Stride,
						Count:   1,
					},
				); err != nil {
					t.Fatal(err)
				}
				d, err = s.Supports.NewDraft(1)
				if err != nil {
					t.Fatal(err)
				}
				d.Set(0, support.Relationship{Unit: 2, Initial: 35, Growth: 4})
				expected = edit.ErrLayout
			case "capacity":
				if err = s.Document.ApplyEdit(
					before.Revision,
					"fill",
					rom.Edit{Size: rom.MaxSize},
				); err != nil {
					t.Fatal(err)
				}
				d, err = s.Supports.NewDraft(1)
				if err != nil {
					t.Fatal(err)
				}
				d.Set(0, support.Relationship{Unit: 2, Initial: 35, Growth: 4})
				expected = space.ErrCapacity
			}
			before = s.Document.Snapshot()
			_, err = s.Supports.Plan(d, true)
			if err == nil || expected != nil && !errors.Is(err, expected) {
				t.Fatal("expected rejection", expected, err)
			}
			if after := s.Document.Snapshot(); after.Revision != before.Revision ||
				!bytes.Equal(after.Data, before.Data) {
				t.Fatal("partial write")
			}
		})
	}
}
