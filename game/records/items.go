package records

import (
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/game/edit"
)

// ItemChoice names one confirmed native value. Other values remain editable;
// listing choices never normalizes an existing custom value.
type ItemChoice struct {
	Value int64
	Name  string
	Help  string
}

// ItemIconUses groups native item users by their stored icon reference.
// Item zero is the no-item sentinel; icon zero is a normal image slot.
func (s *Editor) ItemIconUses() (map[int][]Row, error) {
	s.Refresh()
	table, _, field, err := s.Resolve(Items, 0, "icon_id")
	if err != nil {
		return nil, err
	}
	if table.Err != nil {
		return nil, table.Err
	}
	uses := make(map[int][]Row)
	for _, row := range table.Rows {
		if row.ID == 0 {
			continue
		}
		id := int(field.Value(s.image.Data, row.Address))
		uses[id] = append(uses[id], row)
	}
	return uses, nil
}

// ItemChoices supplies concrete native values for item fields. Effect
// definitions and rank minima follow the current generation.
func (s *Editor) ItemChoices(field string) ([]ItemChoice, error) {
	s.Refresh()
	switch field {
	case "weapon_type":
		choices := make([]ItemChoice, len(WeaponNames))
		for index, name := range WeaponNames {
			choices[index] = ItemChoice{Value: int64(index), Name: name}
		}
		return append(
			choices,
			ItemChoice{Value: 9, Name: "item_type_item"},
		), nil
	case "weapon_rank":
		thresholds, err := s.weaponThresholds()
		if err != nil {
			return nil, err
		}
		choices := make([]ItemChoice, len(thresholds))
		for index, value := range thresholds {
			name := weaponRanks[index]
			if index == 0 {
				name = "item_rank_none"
			}
			choices[index] = ItemChoice{Value: value, Name: name}
		}
		return choices, nil
	case "use_effect":
		return slices.Clone(s.definition.ItemUseEffects), nil
	case "weapon_effect":
		return slices.Clone(s.definition.ItemWeaponEffects), nil
	case "combat.range":
		return []ItemChoice{
			{Value: 0, Name: "item_range_none"},
			{Value: 0x11, Name: "1"},
			{Value: 0x12, Name: "1–2"},
			{Value: 0x13, Name: "1–3"},
			{Value: 0x22, Name: "2"},
			{Value: 0x23, Name: "2–3"},
			{Value: 0x3A, Name: "3–10"},
			{Value: 0x3F, Name: "3–15"},
			{Value: 0x10, Name: "item_range_magic"},
			{Value: 0xFF, Name: "item_range_all"},
		}, nil
	default:
		return nil, edit.ErrField
	}
}
