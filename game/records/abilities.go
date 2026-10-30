package records

import (
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
)

// AbilityValue uses hundredths so linear reference values retain exact decimal
// growth without floating-point accumulation or implied in-game rounding.
type AbilityValue struct {
	Stat       string
	Hundredths int64
}

type AbilityReference struct {
	ClassID           int
	ClassName         string
	GrowthSteps       int
	Values, ClassBase []AbilityValue
}

var abilityOrder = []string{
	StatHP,
	StatPower,
	StatMagic,
	StatSkill,
	StatSpeed,
	StatDefense,
	StatResistance,
	StatLuck,
	StatConstitution,
	StatMovement,
}

// ReferenceAbilities combines stored character and referenced class values.
// With estimate enabled, target is a character's target level or a class's
// growth count. Estimates exclude caps, promotion and actual game state.
func (s *Editor) ReferenceAbilities(
	kind Kind,
	id, target int,
	estimate bool,
) (AbilityReference, error) {
	s.Refresh()
	if kind != Units && kind != Classes {
		return AbilityReference{}, edit.ErrField
	}
	classID := id
	if kind == Units {
		value, err := s.Value(kind, id, "class_id")
		if err != nil {
			return AbilityReference{}, err
		}
		classID = int(value)
		if classID == 0 {
			return AbilityReference{}, edit.ErrField
		}
	}
	table := s.Table(Classes)
	if table == nil || table.Err != nil {
		return AbilityReference{}, edit.ErrLayout
	}
	result := AbilityReference{ClassID: classID}
	found := false
	for _, row := range table.Rows {
		if row.ID == classID {
			result.ClassName = row.Name
			found = true
			break
		}
	}
	if !found {
		return AbilityReference{}, edit.ErrField
	}
	if estimate {
		steps, err := s.referenceSteps(kind, id, target)
		if err != nil {
			return AbilityReference{}, err
		}
		result.GrowthSteps = steps
	}
	for _, stat := range abilityOrder {
		value, present := s.recordAbility(Classes, classID, GroupBase, stat)
		if present {
			result.ClassBase = append(
				result.ClassBase,
				AbilityValue{stat, value * 100},
			)
		}
		if kind == Units {
			personal, exists := s.recordAbility(kind, id, GroupBase, stat)
			if !present && !exists {
				continue
			}
			value += personal
		} else if !present {
			continue
		}
		hundredths := value * 100
		if estimate {
			growth, exists := s.recordAbility(kind, id, GroupGrowth, stat)
			if exists {
				hundredths += growth * int64(result.GrowthSteps)
			}
		}
		result.Values = append(result.Values, AbilityValue{stat, hundredths})
	}
	return result, nil
}

func (s *Editor) referenceSteps(kind Kind, id, target int) (int, error) {
	if target < 0 || target > 255 {
		return 0, edit.ErrValue
	}
	if kind == Classes {
		return target, nil
	}
	level, err := s.Value(kind, id, "level")
	if err != nil {
		return 0, err
	}
	if level < 1 || int64(target) < level {
		return 0, edit.ErrValue
	}
	return target - int(level), nil
}

func (s *Editor) recordAbility(
	kind Kind,
	id int,
	group, stat string,
) (int64, bool) {
	value, err := s.Value(kind, id, group+"."+stat)
	return value, err == nil
}
