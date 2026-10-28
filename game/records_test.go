package game

import (
	"bytes"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
)

func TestRecordAbilityWorkflow(t *testing.T) {
	for _, fixture := range gameFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			f := testutil.NewRecordFixture(fixture.name)
			s := openFixture(t, f)
			before := s.Document.Snapshot()
			initial, err := s.ReferenceAbilities(Units, 1, 0, false)
			if err != nil {
				t.Fatal(err)
			}
			expected := []AbilityValue{
				{StatHP, 1700},
				{StatPower, 400},
				{StatSkill, 0},
				{StatSpeed, 0},
				{StatDefense, 0},
				{StatResistance, 0},
				{StatLuck, 700},
				{StatConstitution, 400},
				{StatMovement, 600},
			}
			if initial.ClassID != 1 || !slices.Equal(initial.Values, expected) {
				t.Fatalf("native reference: %+v", initial)
			}
			estimated, err := s.ReferenceAbilities(Units, 1, 5, true)
			if err != nil {
				t.Fatal(err)
			}
			if estimated.GrowthSteps != 2 ||
				estimated.Values[0].Hundredths != 1850 {
				t.Fatal("exact growth reference")
			}
			class, err := s.ReferenceAbilities(Classes, 1, 3, true)
			if err != nil {
				t.Fatal(err)
			}
			if class.Values[0].Hundredths != 1985 ||
				slices.ContainsFunc(
					class.Values,
					func(value AbilityValue) bool { return value.Stat == StatLuck },
				) {
				t.Fatal("signed class growth or absent base")
			}
			for _, target := range []int{2, 256, -1} {
				if _, err := s.ReferenceAbilities(
					Units,
					1,
					target,
					true,
				); !errors.Is(
					err,
					ErrValue,
				) {
					t.Fatal("invalid level estimate", target, err)
				}
			}
			if !bytes.Equal(s.Document.Snapshot().Data, before.Data) ||
				s.Document.State().Dirty {
				t.Fatal("reference created ROM history")
			}
			for _, change := range []struct {
				kind   Kind
				field  string
				value  int64
				offset int
				raw    byte
			}{
				{Classes, "base.power", -128, f.GetPosition("record_class") + 12, 0x80},
				{Classes, "maximum.hp", 127, f.GetPosition("record_class") + 19, 0x7F},
				{Classes, "growth.hp", -10, f.GetPosition("record_class") + 27, 0xF6},
				{Classes, "experience", -128, f.GetPosition("record_class") + 26, 0x80},
				{Units, "level", -1, f.GetPosition("record_unit") + 11, 0xFF},
			} {
				if err := s.Set(
					change.kind,
					[]int{1},
					change.field,
					change.value,
				); err != nil {
					t.Fatal(err)
				}
				if s.Document.Snapshot().Data[change.offset] != change.raw {
					t.Fatal("native signed field encoding")
				}
				if !s.Document.Undo() {
					t.Fatal("signed undo")
				}
				s.Refresh()
			}
			switch fixture.name {
			case fe6ID:
				if err := s.Set(
					Classes,
					[]int{1},
					"promotion.hp",
					200,
				); !errors.Is(
					err,
					ErrField,
				) {
					t.Fatal("FE6 promotion field invented")
				}
			default:
				value, err := s.Value(Classes, 1, "promotion.hp")
				if err != nil || value != 200 {
					t.Fatal("unsigned promotion byte")
				}
				if err := s.Set(
					Classes,
					[]int{1},
					"promotion.hp",
					255,
				); err != nil {
					t.Fatal(err)
				}
				if s.Document.Snapshot().Data[f.GetPosition("record_class")+34] != 0xFF {
					t.Fatal("promotion byte encoding")
				}
				s.Document.Undo()
				s.Refresh()
			}
			if err := s.Set(Units, []int{1}, "class_id", 2); err != nil {
				t.Fatal(err)
			}
			changed, err := s.ReferenceAbilities(Units, 1, 0, false)
			if err != nil || changed.Values[0].Hundredths != 2700 ||
				changed.ClassID != 2 {
				t.Fatal("reference did not follow class")
			}
			output := filepath.Join(t.TempDir(), "record-reference.gba")
			if err := s.Document.SaveAs(output, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(output)
			if err != nil {
				t.Fatal(err)
			}
			again, err := reopened.ReferenceAbilities(Units, 1, 5, true)
			if err != nil || again.Values[0].Hundredths != 2850 {
				t.Fatal("reference after save/reopen")
			}
		})
	}
}

func TestRecordTraitsAndWeapons(t *testing.T) {
	for _, fixture := range gameFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			f := testutil.NewRecordFixture(fixture.name)
			s := openFixture(t, f)
			before := s.Document.Snapshot()
			if err := s.SetRecordTrait(Units, 1, 1, true); err != nil {
				t.Fatal(err)
			}
			expected := bytes.Clone(before.Data)
			expected[f.GetPosition("record_unit")+40] |= 2
			if !bytes.Equal(expected, s.Document.Snapshot().Data) {
				t.Fatal("trait changed unknown or adjacent bits")
			}
			stable := s.Document.Snapshot()
			if err := s.SetRecordTrait(
				Units,
				1,
				23,
				false,
			); !errors.Is(err, ErrField) ||
				!bytes.Equal(stable.Data, s.Document.Snapshot().Data) {
				t.Fatal("unknown trait editable")
			}
			traits := s.RecordTraits(Classes)
			switch fixture.name {
			case "FE6":
				if traits[31].Known ||
					traits[19].Label != "trait_zephiel_lock" {
					t.Fatal("FE6 traits")
				}
			case "FE7U", "FE7J":
				if traits[27].Label != "trait_drop_item" {
					t.Fatal("FE7 traits")
				}
			default:
				if traits[27].Label != "trait_summon" {
					t.Fatal("FE8 traits")
				}
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("trait atomic undo")
			}
			s.Refresh()
			if !s.Document.Redo() {
				t.Fatal("trait redo")
			}
			s.Refresh()
			skills, err := s.WeaponSkills(Units, 1)
			if err != nil {
				t.Fatal(err)
			}
			rank := "C"
			if fixture.name == "FE6" {
				rank = "D"
			}
			if !skills[0].Inherited || skills[0].Value != 0 ||
				skills[0].Effective != 95 ||
				skills[0].EffectiveRank != rank {
				t.Fatal("weapon inheritance")
			}
			if err := s.Set(Units, []int{1}, "weapon.sword", 41); err != nil {
				t.Fatal(err)
			}
			skills, err = s.WeaponSkills(Units, 1)
			if err != nil || skills[0].Inherited || skills[0].Effective != 41 {
				t.Fatal("character EXP did not replace class EXP")
			}
			revision := s.Document.State().Revision
			if err := s.SetWeaponRank(
				Units,
				1,
				"weapon.sword",
				skills[0].Rank,
			); err != nil ||
				s.Document.State().Revision != revision {
				t.Fatal("same rank reset partial progress")
			}
			minimum := int64(181)
			if fixture.name == "FE6" {
				minimum = 201
			}
			if err := s.SetWeaponRank(
				Units,
				1,
				"weapon.sword",
				"A",
			); err != nil {
				t.Fatal(err)
			}
			if value, _ := s.Value(Units, 1, "weapon.sword"); value != minimum {
				t.Fatal("version rank minimum")
			}
			if err := s.SetWeaponRank(
				Units,
				1,
				"weapon.sword",
				"none",
			); err != nil {
				t.Fatal(err)
			}
			skills, _ = s.WeaponSkills(Units, 1)
			if !skills[0].Inherited || skills[0].Effective != 95 {
				t.Fatal("explicit inherit")
			}
			if err := s.Set(Units, []int{1}, "class_id", 255); err != nil {
				t.Fatal(err)
			}
			skills, err = s.WeaponSkills(Units, 1)
			if err != nil || skills[0].ReferenceError == nil {
				t.Fatal("invalid class silently treated as inherited zero")
			}
			if err := s.SetWeaponRank(
				Units,
				1,
				"weapon.sword",
				"D",
			); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(t.TempDir(), "record-weapons.gba")
			if err := s.Document.SaveAs(output, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(output)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.SetRecordTrait(Units, 1, 1, false); err != nil {
				t.Fatal(err)
			}
			if value, _ := reopened.Value(
				Units,
				1,
				"flags",
			); value != 0x80800080 {
				t.Fatal("trait save/reopen unknown bits")
			}
		})
	}
}

func TestRecordLayoutsAndResources(t *testing.T) {
	for _, fixture := range gameFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			f := testutil.NewRecordFixture(fixture.name)
			s := openFixture(t, f)
			links, err := s.RecordResources(Classes, 1)
			if err != nil {
				t.Fatal(err)
			}
			count := 7
			if fixture.name == "FE6" {
				count = 5
			}
			if len(links) != count || links[0].Address != 0x261000 ||
				links[1].Address != 0x262000 ||
				links[0].Err != nil {
				t.Fatal("native resource pointers")
			}
			if !links[2].Empty {
				t.Fatal("zero resource not distinct")
			}
			stable := s.Document.Snapshot()
			if _, err := s.RecordResources(Units, 1); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(stable.Data, s.Document.Snapshot().Data) {
				t.Fatal("resource browsing wrote document")
			}
			move, err := s.PlanTableMove(Classes, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyTableMove(move); err != nil {
				t.Fatal(err)
			}
			after, err := s.RecordResources(Classes, 1)
			if err != nil || after[0].Address != links[0].Address ||
				after[0].Pointer == links[0].Pointer {
				t.Fatal("resources ignored relocated table")
			}
			reference, err := s.ReferenceAbilities(Units, 1, 0, false)
			if err != nil || reference.Values[0].Hundredths != 1700 {
				t.Fatal("reference ignored relocated table")
			}
			table := s.Table(Classes)
			if err := s.UseLayout(
				Classes,
				Layout{
					Address: table.Address + table.Stride,
					Stride:  table.Stride,
					Count:   1,
				},
			); err != nil {
				t.Fatal(err)
			}
			// The selected manual row now has ID 0; the character's class 1 is absent.
			if _, err := s.ReferenceAbilities(Units, 1, 0, false); err == nil {
				t.Fatal("reference bypassed manual class layout")
			}
			if _, err := s.RecordResources(
				Items,
				0,
			); !errors.Is(
				err,
				ErrField,
			) {
				t.Fatal("unsupported resource owner")
			}
		})
	}
}
