package records

import (
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/version"
)

// WeaponSkill retains the exact stored weapon EXP alongside its native rank.
// Character zero inherits the referenced class; a nonzero character value
// replaces its class value, even when it is lower.
type WeaponSkill struct {
	Field, Weapon, Rank, EffectiveRank, ClassRank string
	Value, Effective, ClassValue                  int64
	Inherited                                     bool
	ReferenceError                                error
}

var weaponRanks = []string{"none", "E", "D", "C", "B", "A", "S"}

func (s *Editor) weaponThresholds() []int64 {
	if s.definition.Version == version.FE6 {
		return []int64{0, 1, 51, 101, 151, 201, 251}
	}
	return []int64{0, 1, 31, 71, 121, 181, 251}
}

func (s *Editor) weaponRank(value int64) string {
	index := 0
	for i, minimum := range s.weaponThresholds() {
		if value >= minimum {
			index = i
		}
	}
	return weaponRanks[index]
}

func (s *Editor) WeaponSkills(kind Kind, id int) ([]WeaponSkill, error) {
	s.Refresh()
	if kind != Units && kind != Classes {
		return nil, edit.ErrField
	}
	var result []WeaponSkill
	for _, weapon := range WeaponNames {
		field := "weapon." + weapon
		value, err := s.Value(kind, id, field)
		if err != nil {
			return nil, err
		}
		entry := WeaponSkill{
			Field:     field,
			Weapon:    weapon,
			Value:     value,
			Effective: value,
			Rank:      s.weaponRank(value),
		}
		entry.ClassValue = value
		if kind == Units {
			entry.ClassValue, entry.ReferenceError = s.weaponClassValue(
				id,
				field,
			)
			entry.Inherited = value == 0
			if entry.Inherited {
				entry.Effective = entry.ClassValue
			}
		}
		entry.ClassRank = s.weaponRank(entry.ClassValue)
		entry.EffectiveRank = s.weaponRank(entry.Effective)
		result = append(result, entry)
	}
	return result, nil
}

func (s *Editor) weaponClassValue(id int, field string) (int64, error) {
	class, err := s.Value(Units, id, "class_id")
	if err != nil {
		return 0, err
	}
	if class == 0 {
		return 0, edit.ErrField
	}
	return s.Value(Classes, int(class), field)
}

// SetWeaponRank writes a rank's native minimum only when its stored rank
// changes. Choosing the current rank preserves partial progress within it.
func (s *Editor) SetWeaponRank(kind Kind, id int, field, rank string) error {
	s.Refresh()
	if kind != Units && kind != Classes {
		return edit.ErrField
	}
	if !slices.ContainsFunc(
		WeaponNames,
		func(name string) bool { return field == "weapon."+name },
	) {
		return edit.ErrField
	}
	index := slices.Index(weaponRanks, rank)
	if index < 0 {
		return edit.ErrValue
	}
	value, err := s.Value(kind, id, field)
	if err != nil {
		return err
	}
	if s.weaponRank(value) == rank {
		return nil
	}
	return s.Set(kind, []int{id}, field, s.weaponThresholds()[index])
}
