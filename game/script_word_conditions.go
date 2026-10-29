package game

const (
	ScriptBranchSkipped         ScriptOp = "script_branch_skipped"
	ScriptBranchTextSkipped     ScriptOp = "script_branch_text_skipped"
	ScriptBranchActive          ScriptOp = "script_branch_active"
	ScriptBranchNotActive       ScriptOp = "script_branch_not_active"
	ScriptBranchEliwood         ScriptOp = "script_branch_eliwood"
	ScriptBranchHector          ScriptOp = "script_branch_hector"
	ScriptBranchNormal          ScriptOp = "script_branch_normal"
	ScriptBranchHard            ScriptOp = "script_branch_hard"
	ScriptBranchNotYes          ScriptOp = "script_branch_not_yes"
	ScriptBranchNotYesAlternate ScriptOp = "script_branch_not_yes_alternate"
	ScriptBranchNoTutorial      ScriptOp = "script_branch_no_tutorial"
	ScriptBranchDeathOnce       ScriptOp = "script_branch_death_once"
	ScriptBranchTurnReached     ScriptOp = "script_branch_turn_reached"
	ScriptBranchDeployed        ScriptOp = "script_branch_deployed"
	ScriptBranchNotDeployed     ScriptOp = "script_branch_not_deployed"
	ScriptTurn                           = "script_turn"
)

func (f deploymentFormat) wordConditionDefinitions() []scriptDefinition {
	if f.kind == eventFE8 {
		return nil
	}
	label := scriptOperand(ScriptLabelID, 4, 4, 0xFFFFFFFF)
	character := scriptOperand(ScriptCharacter, 8, 1, 0xFF)
	character.Reference = Units
	if f.kind == eventFE6 {
		character.width = 4
	}
	var result []scriptDefinition
	add := func(op ScriptOp, code uint32, help string, fields ...ScriptField) {
		size := 8
		for _, field := range fields {
			size = max(size, field.offset+field.width)
		}
		raw := make([]byte, (size+3)&^3)
		copy(raw, scriptWords(code))
		result = append(
			result,
			scriptDefinition{
				op:     op,
				raw:    raw,
				fields: append([]ScriptField{label}, fields...),
				branch: true,
				help:   help,
			},
		)
	}
	if f.kind == eventFE6 {
		add(ScriptBranchSkipped, 0x21, "script_branch_skipped_help")
		add(ScriptBranchActive, 0x24, "script_branch_active_help", character)
		return result
	}
	shift := uint32(0)
	if f.kind == eventFE7U {
		shift = 2
	}
	for _, entry := range []struct {
		op   ScriptOp
		code uint32
		help string
	}{
		{ScriptBranchSkipped, 0x48, "script_branch_skipped_help"},
		{ScriptBranchTextSkipped, 0x49, "script_branch_text_skipped_help"},
		{ScriptBranchEliwood, 0x4D, "script_branch_route_help"},
		{ScriptBranchHector, 0x4E, "script_branch_route_help"},
		{ScriptBranchNormal, 0x4F, "script_branch_difficulty_help"},
		{ScriptBranchHard, 0x1004F, "script_branch_difficulty_help"},
		{ScriptBranchNotYes, 0x50, "script_branch_choice_help"},
		{ScriptBranchNotYesAlternate, 0x51, "script_branch_choice_help"},
		{ScriptBranchNoTutorial, 0x52, "script_branch_no_tutorial_help"},
	} {
		add(entry.op, entry.code+shift, entry.help)
	}
	for _, entry := range []struct {
		op   ScriptOp
		code uint32
		help string
	}{
		{ScriptBranchActive, 0x1004C, "script_branch_active_help"},
		{ScriptBranchNotActive, 0x4C, "script_branch_active_help"},
		{ScriptBranchDeployed, 0x10055, "script_branch_deployed_help"},
		{ScriptBranchNotDeployed, 0x55, "script_branch_deployed_help"},
	} {
		add(entry.op, entry.code+shift, entry.help, character)
	}
	add(
		ScriptBranchDeathOnce,
		0x53+shift,
		"script_branch_death_once_help",
		character,
		scriptOperand(ScriptFlagID, 12, 4, 0xFFFF),
	)
	add(
		ScriptBranchTurnReached,
		0x54+shift,
		"script_branch_turn_help",
		scriptOperand(ScriptTurn, 2, 2, 0xFFFF),
	)
	return result
}
