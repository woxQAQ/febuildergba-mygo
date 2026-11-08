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
	"github.com/woxQAQ/febuildergba-mygo/romwrite"
)

func TestAffinityBonusWorkflow(t *testing.T) {
	for _, version := range testutil.Games() {
		t.Run(version.Name, func(t *testing.T) {
			f := testutil.NewSupportBonusFixture(version.Name)
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			draft, err := s.Supports.NewBonusDraft()
			if err != nil {
				t.Fatal(err)
			}
			rows := draft.Rows()
			if len(rows) != 4 || rows[0].Affinity != 5 ||
				rows[1].Affinity != 1 ||
				rows[2].Effective ||
				rows[3].Affinity != 8 ||
				rows[0].Values != [6]uint8{1, 2, 3, 4, 5, 6} {
				t.Fatal("native row identity or first-match lookup", rows)
			}
			for kind, value := range []int{0, 255, 17, 18, 19, 20} {
				if err = draft.Set(
					0,
					support.BonusKind(kind),
					value,
				); err != nil {
					t.Fatal(err)
				}
			}
			if err = draft.Set(2, support.AttackBonus, 99); err != nil {
				t.Fatal(err)
			}
			if err = draft.Set(3, support.DodgeBonus, 254); err != nil {
				t.Fatal(err)
			}
			plan, err := s.Supports.PlanBonuses(draft)
			if err != nil {
				t.Fatal(err)
			}
			changes := plan.Changes()
			var ids []int
			for _, unit := range changes[0].Units {
				ids = append(ids, unit.ID)
			}
			if len(changes) != 3 || !slices.Equal(ids, []int{1, 3}) ||
				len(changes[1].Units) != 0 ||
				changes[1].Effective ||
				len(changes[2].Units) != 1 ||
				changes[2].Units[0].ID != 4 ||
				plan.UsesError() != nil {
				t.Fatal("shared affinity preview", changes, plan.UsesError())
			}
			changes[0].Units[0].ID = 99
			if plan.Changes()[0].Units[0].ID != 1 {
				t.Fatal("mutable preview")
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("preview changed ROM")
			}
			if err = draft.Set(0, support.AttackBonus, 44); err != nil {
				t.Fatal(err)
			}
			if err = s.Supports.ApplyBonuses(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			pointer := f.TablePointers("support_bonuses")[0]
			at, valid := gba.Pointer(after.Data, pointer)
			if !valid || at != plan.Destination() || at < len(before.Data) ||
				plan.Pointer() != pointer {
				t.Fatal("consumer not relocated")
			}
			expected := []byte{
				5, 0, 255, 17, 18, 19, 20, 0xA1,
				1, 6, 5, 4, 3, 2, 1, 0xA2,
				5, 99, 8, 9, 10, 11, 12, 0xA3,
				8, 2, 4, 6, 8, 10, 254, 0xA4,
				0, 0xB1, 0xB2, 0xB3, 0xB4, 0xB5, 0xB6, 0xB7,
			}
			if !bytes.Equal(after.Data[at:at+len(expected)], expected) {
				t.Fatal("values, padding, terminator or frozen plan")
			}
			prefix := bytes.Clone(before.Data)
			copy(prefix[pointer:pointer+4], after.Data[pointer:pointer+4])
			if !bytes.Equal(prefix, after.Data[:len(prefix)]) {
				t.Fatal(
					"changed original table, independent alias or unrelated bytes",
				)
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				!s.Document.Redo() ||
				!bytes.Equal(after.Data, s.Document.Snapshot().Data) {
				t.Fatal("non-atomic history")
			}
			path := filepath.Join(t.TempDir(), "bonuses.gba")
			if err = s.Document.SaveAs(path, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			saved, err := reopened.Supports.NewBonusDraft()
			if err != nil || !bytes.Equal(saved.Raw(), expected) {
				t.Fatal("save/reopen", err)
			}
		})
	}
}

func TestAffinityBonusFailuresAreAtomic(t *testing.T) {
	for _, scenario := range []string{"revision", "layout", "document", "overlap", "capacity"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.NewSupportBonusFixture("FE8U")
			s := testutil.Open(t, f)
			draft, err := s.Supports.NewBonusDraft()
			if err != nil {
				t.Fatal(err)
			}
			if err = draft.Set(0, support.HitBonus, 9); err != nil {
				t.Fatal(err)
			}
			plan, err := s.Supports.PlanBonuses(draft)
			if err != nil {
				t.Fatal(err)
			}
			want := error(rom.ErrStale)
			switch scenario {
			case "revision":
				err = s.Records.Set(records.Units, []int{1}, "level", 9)
			case "layout":
				table := s.Records.Table(records.Units)
				err = s.Records.UseLayout(
					records.Units,
					records.Layout{
						Address: table.Address,
						Stride:  table.Stride,
						Count:   3,
					},
				)
			case "document":
				s = testutil.Open(t, f)
			case "overlap":
				err = s.Records.UseLayout(
					records.Classes,
					records.Layout{
						Address: f.TablePointers("support_bonuses")[0],
						Stride:  84,
						Count:   1,
					},
				)
				want = edit.ErrLayout
			case "capacity":
				err = s.Document.ApplyEdit(
					s.Document.State().Revision,
					"fill",
					rom.Edit{Size: rom.MaxSize},
				)
				want = romwrite.ErrCapacity
			}
			if err != nil {
				t.Fatal(err)
			}
			before := s.Document.Snapshot()
			if err = s.Supports.ApplyBonuses(
				plan,
			); !errors.Is(
				err,
				rom.ErrStale,
			) {
				t.Fatal("stale plan accepted", err)
			}
			if scenario == "overlap" || scenario == "capacity" {
				draft, err = s.Supports.NewBonusDraft()
				if err != nil {
					t.Fatal(err)
				}
				if err = draft.Set(0, support.HitBonus, 9); err != nil {
					t.Fatal(err)
				}
			}
			if _, err = s.Supports.PlanBonuses(draft); !errors.Is(err, want) {
				t.Fatal("plan rejection", want, err)
			}
			after := s.Document.Snapshot()
			if after.Revision != before.Revision ||
				!bytes.Equal(after.Data, before.Data) {
				t.Fatal("partial write")
			}
		})
	}
}

func TestAffinityBonusBoundsAndIndependentInputs(t *testing.T) {
	for _, scenario := range []string{"null", "unaligned", "truncated", "unterminated", "empty", "characters", "relationships", "dialog"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.NewSupportBonusFixture("FE8U")
			site, at := f.TablePointers("support_bonuses")[0], f.GetPosition(
				"support_bonuses",
			)
			switch scenario {
			case "null":
				f.WriteU32(site, 0)
			case "unaligned":
				f.WritePointer(site, at+1)
			case "truncated":
				f.WritePointer(site, len(f.Bytes())-4)
			case "unterminated":
				f.WriteAt(at, bytes.Repeat([]byte{1}, 257*8))
			case "empty":
				f.WriteU8(at, 0)
			case "characters":
				for _, pointer := range f.TablePointers("units") {
					f.WriteU32(pointer, 1)
				}
			case "relationships":
				f.WriteU32(f.GetPosition("support_pointer_1"), 1)
			case "dialog":
				f.WriteU32(f.TablePointers("support_talks")[0], 1)
			}
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			draft, err := s.Supports.NewBonusDraft()
			switch scenario {
			case "null", "unaligned", "truncated", "unterminated":
				if !errors.Is(err, support.ErrData) {
					t.Fatal("invalid table accepted", err)
				}
				if after := s.Document.Snapshot(); after.Revision != before.Revision ||
					!bytes.Equal(before.Data, after.Data) {
					t.Fatal("read changed ROM")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			raw := draft.Raw()
			for _, input := range []struct {
				index int
				kind  support.BonusKind
				value int
			}{{-1, support.AttackBonus, 1}, {4, support.AttackBonus, 1}, {0, 6, 1}, {0, support.AttackBonus, -1}, {0, support.AttackBonus, 256}} {
				if err = draft.Set(
					input.index,
					input.kind,
					input.value,
				); !errors.Is(
					err,
					edit.ErrValue,
				) {
					t.Fatal("invalid input accepted", err)
				}
			}
			if !bytes.Equal(raw, draft.Raw()) || draft.Modified() {
				t.Fatal("invalid input changed draft")
			}
			if scenario == "empty" {
				if len(draft.Rows()) != 0 {
					t.Fatal("empty table")
				}
				if _, err = s.Supports.PlanBonuses(
					draft,
				); !errors.Is(
					err,
					edit.ErrValue,
				) {
					t.Fatal(err)
				}
				return
			}
			if err = draft.Set(0, support.AttackBonus, 2); err != nil {
				t.Fatal(err)
			}
			plan, err := s.Supports.PlanBonuses(draft)
			if err != nil {
				t.Fatal("independent table disabled", err)
			}
			if (plan.UsesError() != nil) != (scenario == "characters") {
				t.Fatal("incomplete uses not reported", plan.UsesError())
			}
			if err = s.Supports.ApplyBonuses(plan); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAffinityBonusClaimsProtectAgainstOtherWriters(t *testing.T) {
	f := testutil.NewSupportBonusFixture("FE8U")
	at := f.GetPosition("support_bonuses")
	f.WritePointer(at+4, f.GetPosition("support_1"))
	s := testutil.Open(t, f)
	if err := s.Records.UseLayout(
		records.Units,
		records.Layout{Address: at - 40, Stride: 52, Count: 1},
	); err != nil {
		t.Fatal(err)
	}
	draft, err := s.Supports.NewDraft(1)
	if err != nil {
		t.Fatal(err)
	}
	if err = draft.Remove(0); err != nil {
		t.Fatal(err)
	}
	before := s.Document.Snapshot()
	if _, err = s.Supports.Plan(draft, false); !errors.Is(err, edit.ErrLayout) {
		t.Fatal("bonus payload overwritten by character support pointer", err)
	}
	if after := s.Document.Snapshot(); after.Revision != before.Revision ||
		!bytes.Equal(before.Data, after.Data) {
		t.Fatal("conflict changed ROM")
	}
}
