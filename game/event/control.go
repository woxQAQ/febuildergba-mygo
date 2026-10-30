package event

import (
	"github.com/woxQAQ/febuildergba-mygo/game/records"
)

const (
	ScriptClearSkip         ScriptOp = "script_clear_skip"
	ScriptEarlyReturn       ScriptOp = "script_early_return"
	ScriptEndAll            ScriptOp = "script_end_all"
	ScriptWaitSkippable     ScriptOp = "script_wait_skippable"
	ScriptWaitFast          ScriptOp = "script_wait_fast"
	ScriptWaitSkippableFast ScriptOp = "script_wait_skippable_fast"
	ScriptCheckEventBit     ScriptOp = "script_check_event_bit"
	ScriptRandom            ScriptOp = "script_random"
	ScriptCounterRead       ScriptOp = "script_counter_read"
	ScriptCounterSet        ScriptOp = "script_counter_set"
	ScriptCounterIncrement  ScriptOp = "script_counter_increment"
	ScriptCounterDecrement  ScriptOp = "script_counter_decrement"
	ScriptExecutionMode     ScriptOp = "script_execution_mode"
	ScriptIgnoreKeys        ScriptOp = "script_ignore_keys"
	ScriptCheckMode         ScriptOp = "script_check_mode"
	ScriptCheckChapter      ScriptOp = "script_check_chapter"
	ScriptCheckHardMode     ScriptOp = "script_check_hard_mode"
	ScriptCheckTurn         ScriptOp = "script_check_turn"
	ScriptCheckEnemies      ScriptOp = "script_check_enemies"
	ScriptCheckNPCs         ScriptOp = "script_check_npcs"
	ScriptCheckMapKind      ScriptOp = "script_check_map_kind"
	ScriptCheckTutorial     ScriptOp = "script_check_tutorial"
	ScriptCheckGold         ScriptOp = "script_check_gold"
	ScriptCheckTrigger      ScriptOp = "script_check_trigger"
	ScriptCheckPostgame     ScriptOp = "script_check_postgame"
)

const (
	ScriptCounterID    = "script_counter_id"
	ScriptCounterValue = "script_counter_value"
	ScriptMaximum      = "script_maximum"
	ScriptMode         = "script_mode"
	ScriptKeyMask      = "script_key_mask"
)

func (f Format) controlDefinitions() []scriptDefinition {
	if f.kind != eventFE8 {
		clearSkip := uint32(0x0A)
		if f.kind == eventFE6 {
			clearSkip = 6
		}
		return []scriptDefinition{
			{
				op:   ScriptClearSkip,
				raw:  scriptWords(clearSkip),
				help: "script_clear_skip_help",
			},
			{
				op:       ScriptEarlyReturn,
				raw:      scriptWords(1),
				terminal: true,
				help:     "script_early_return_help",
			},
		}
	}
	var result []scriptDefinition
	add := func(op ScriptOp, code, sub byte, help string, fields ...ScriptField) {
		result = append(
			result,
			scriptDefinition{
				op:       op,
				raw:      []byte{0x20 + sub, code, 0, 0},
				fields:   fields,
				help:     help,
				terminal: op == ScriptEndAll,
			},
		)
	}
	add(ScriptEndAll, 1, 1, "script_end_all_help")
	for index, op := range []ScriptOp{ScriptWaitSkippable, ScriptWaitFast, ScriptWaitSkippableFast} {
		add(
			op,
			0x0E,
			byte(index+1),
			"script_wait_options_help",
			sceneOperand(ScriptFrames, 2, 2, 1, 0x7FFF),
		)
		result[len(result)-1].raw[2] = 1
	}
	add(
		ScriptCheckEventBit,
		3,
		0,
		"script_check_event_bit_help",
		sceneOperand(ScriptEventBit, 2, 2, -1, 15),
	)
	add(
		ScriptRandom,
		4,
		0,
		"script_random_help",
		scriptOperand(ScriptMaximum, 2, 2, 0xFFFF),
	)
	for index, op := range []ScriptOp{ScriptCounterRead, ScriptCounterSet, ScriptCounterIncrement, ScriptCounterDecrement} {
		fields := []ScriptField{scriptOperand(ScriptCounterID, 2, 1, 7)}
		if op == ScriptCounterSet {
			fields = append(fields, scriptOperand(ScriptCounterValue, 3, 1, 15))
		}
		add(op, 0x0F, byte(index), "script_counter_help", fields...)
	}
	add(
		ScriptExecutionMode,
		0x10,
		0,
		"script_execution_mode_help",
		scriptOperand(ScriptMode, 2, 2, 4),
	)
	add(
		ScriptIgnoreKeys,
		0x11,
		0,
		"script_ignore_keys_help",
		scriptOperand(ScriptKeyMask, 2, 2, 0x3FF),
	)
	for index, op := range []ScriptOp{ScriptCheckMode, ScriptCheckChapter, ScriptCheckHardMode, ScriptCheckTurn, ScriptCheckEnemies, ScriptCheckNPCs, ScriptCheckMapKind, ScriptCheckTutorial, ScriptCheckGold, ScriptCheckTrigger, ScriptCheckPostgame} {
		add(op, 0x19, byte(index), "script_game_check_help")
	}
	return result
}

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

func (f Format) wordConditionDefinitions() []scriptDefinition {
	if f.kind == eventFE8 {
		return nil
	}
	label := scriptOperand(ScriptLabelID, 4, 4, 0xFFFFFFFF)
	character := scriptOperand(ScriptCharacter, 8, 1, 0xFF)
	character.Reference = records.Units
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
