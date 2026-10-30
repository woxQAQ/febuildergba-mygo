package event

const (
	ScriptCheckUnitExists      ScriptOp = "script_check_unit_exists"
	ScriptCheckUnitAlive       ScriptOp = "script_check_unit_alive"
	ScriptCheckUnitDeployed    ScriptOp = "script_check_unit_deployed"
	ScriptCheckUnitFaction     ScriptOp = "script_check_unit_faction"
	ScriptBranchNoAliveAlly    ScriptOp = "script_branch_no_alive_ally"
	ScriptBranchNoDeployedAlly ScriptOp = "script_branch_no_deployed_ally"
)

// FE8 queries write slot C without branching. FE6/FE7 instead branch when the
// named character is absent from the living or deployed player-unit roster.
func (f Format) unitConditionDefinitions() []scriptDefinition {
	if f.kind == eventFE8 {
		var result []scriptDefinition
		for _, entry := range []struct {
			op   ScriptOp
			code byte
			help string
		}{
			{ScriptCheckUnitExists, 0x20, "script_check_unit_exists_help"},
			{ScriptCheckUnitAlive, 0x22, "script_check_unit_alive_help"},
			{ScriptCheckUnitDeployed, 0x23, "script_check_unit_deployed_help"},
			{ScriptCheckUnitFaction, 0x25, "script_check_unit_faction_help"},
		} {
			result = append(result, scriptDefinition{
				op: entry.op, raw: []byte{entry.code, 0x33, 1, 0},
				fields: []ScriptField{scriptFE8Character(2)}, help: entry.help,
			})
		}
		return result
	}
	code := uint32(0x44)
	if f.kind == eventFE7U {
		code = 0x46
	}
	if f.kind == eventFE6 {
		code = 0x1D
	}
	fields := []ScriptField{
		scriptOperand(ScriptLabelID, 4, 4, 0xFFFFFFFF),
		sceneOperand(ScriptCharacter, 8, 2, 0, 0xFF),
	}
	return []scriptDefinition{
		{
			op:     ScriptBranchNoAliveAlly,
			raw:    scriptWords(code, 0, 1),
			fields: fields,
			branch: true,
			help:   "script_branch_no_alive_ally_help",
		},
		{
			op:     ScriptBranchNoDeployedAlly,
			raw:    scriptWords(code+1, 0, 1),
			fields: fields,
			branch: true,
			help:   "script_branch_no_deployed_ally_help",
		},
	}
}

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

func (f Format) unitStateDefinitions() []scriptDefinition {
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
