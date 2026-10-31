package quote

import "github.com/woxQAQ/febuildergba-mygo/game/edit"

// Participants describes FE8's matching roles. A zero side is unspecified;
// the populated side keeps its attacker/defender role during lookup.
type Participants uint8

const (
	Pair Participants = iota
	Attacker
	Defender
	Neither
)

type BattleCondition struct {
	Participants Participants
	Triangle     bool
}

func (d *Draft) BattleCondition(index int) (BattleCondition, error) {
	if d.version != 8 || d.Kind() != Battle {
		return BattleCondition{}, edit.ErrField
	}
	unit, err := d.Value(index, Unit)
	if err != nil {
		return BattleCondition{}, err
	}
	opponent, _ := d.Value(index, Opponent)
	chapter, _ := d.Value(index, Chapter)
	condition := BattleCondition{Triangle: chapter == 0xFE}
	switch {
	case unit == 0 && opponent == 0:
		condition.Participants = Neither
	case unit == 0:
		condition.Participants = Defender
	case opponent == 0:
		condition.Participants = Attacker
	default:
		condition.Participants = Pair
	}
	return condition, nil
}

// AddBattle creates a native FE8 record for a visible matching condition.
// Pair/role and triangle conditions remain independent in the same table.
func (d *Draft) AddBattle(condition BattleCondition) error {
	if d.version != 8 || d.Kind() != Battle {
		return edit.ErrField
	}
	if condition.Participants > Defender {
		return edit.ErrValue
	}
	if err := d.Add(); err != nil {
		return err
	}
	raw := d.rows[len(d.rows)-1].Raw
	for _, field := range d.table.rule.fields {
		switch {
		case field.ID == Unit && condition.Participants == Defender,
			field.ID == Opponent && condition.Participants == Attacker:
			field.put(raw, 0)
		case field.ID == Chapter && condition.Triangle:
			field.put(raw, 0xFE)
		}
	}
	return nil
}
