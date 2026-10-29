package game

const (
	ScriptHideUnit             ScriptOp = "script_hide_unit"
	ScriptRevealUnit           ScriptOp = "script_reveal_unit"
	ScriptSetUnitHP            ScriptOp = "script_set_unit_hp"
	ScriptEndUnitTurn          ScriptOp = "script_end_unit_turn"
	ScriptMarkUnitMoved        ScriptOp = "script_mark_unit_moved"
	ScriptSetUnitDeployment    ScriptOp = "script_set_unit_deployment"
	ScriptRefreshUnit          ScriptOp = "script_refresh_unit"
	ScriptHidePlayerUnits      ScriptOp = "script_hide_player_units"
	ScriptClearNPCUnits        ScriptOp = "script_clear_npc_units"
	ScriptClearEnemyUnits      ScriptOp = "script_clear_enemy_units"
	ScriptUnitDeathFade        ScriptOp = "script_unit_death_fade"
	ScriptRemoveUnitAfterFade  ScriptOp = "script_remove_unit_after_fade"
	ScriptRemoveUnit           ScriptOp = "script_remove_unit"
	ScriptCheckUnitVisitGroup  ScriptOp = "script_check_unit_visit_group"
	ScriptCheckUnitActive      ScriptOp = "script_check_unit_active"
	ScriptCheckUnitCoordinates ScriptOp = "script_check_unit_coordinates"
	ScriptCheckUnitClass       ScriptOp = "script_check_unit_class"
	ScriptCheckUnitLuck        ScriptOp = "script_check_unit_luck"
)

func (f deploymentFormat) unitStateDefinitions() []scriptDefinition {
	if f.kind != eventFE8 {
		return nil
	}
	var result []scriptDefinition
	for _, entry := range []struct {
		op   ScriptOp
		sub  byte
		help string
	}{
		{ScriptHideUnit, 0, "script_hide_unit_help"},
		{ScriptRevealUnit, 1, "script_reveal_unit_help"},
		{ScriptSetUnitHP, 5, "script_set_unit_hp_help"},
		{ScriptEndUnitTurn, 6, "script_end_unit_turn_help"},
		{ScriptMarkUnitMoved, 7, "script_mark_unit_moved_help"},
		{ScriptSetUnitDeployment, 8, "script_set_unit_deployment_help"},
		{ScriptRefreshUnit, 9, "script_refresh_unit_help"},
		{ScriptHidePlayerUnits, 10, "script_hide_player_units_help"},
		{ScriptClearNPCUnits, 11, "script_clear_units_help"},
		{ScriptClearEnemyUnits, 12, "script_clear_units_help"},
		{ScriptUnitDeathFade, 13, "script_unit_death_fade_help"},
		{ScriptRemoveUnitAfterFade, 14, "script_remove_unit_after_fade_help"},
		{ScriptRemoveUnit, 15, "script_remove_unit_help"},
	} {
		raw := []byte{0x20 + entry.sub, 0x34, 1, 0}
		fields := []ScriptField{scriptFE8Character(2)}
		if entry.sub >= 10 && entry.sub <= 12 {
			raw[2] = 0
			fields = nil
		}
		result = append(
			result,
			scriptDefinition{
				op:     entry.op,
				raw:    raw,
				fields: fields,
				help:   entry.help,
			},
		)
	}
	for _, entry := range []struct {
		op   ScriptOp
		sub  byte
		help string
	}{
		{ScriptCheckUnitVisitGroup, 1, "script_check_unit_visit_group_help"},
		{ScriptCheckUnitActive, 4, "script_check_unit_active_help"},
		{ScriptCheckUnitCoordinates, 6, "script_check_unit_coordinates_help"},
		{ScriptCheckUnitClass, 7, "script_check_unit_class_help"},
		{ScriptCheckUnitLuck, 8, "script_check_unit_luck_help"},
	} {
		character := scriptFE8Character(2)
		if entry.op == ScriptCheckUnitActive {
			character.Min = 1
			character.Help = entry.help
		}
		result = append(
			result,
			scriptDefinition{
				op:     entry.op,
				raw:    []byte{0x20 + entry.sub, 0x33, 1, 0},
				fields: []ScriptField{character},
				help:   entry.help,
			},
		)
	}
	return result
}
