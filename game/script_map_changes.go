package game

const (
	ScriptMapChange           ScriptOp = "script_map_change"
	ScriptMapRevert           ScriptOp = "script_map_revert"
	ScriptMapChangePosition   ScriptOp = "script_map_change_position"
	ScriptMapChangeInstant    ScriptOp = "script_map_change_instant"
	ScriptMapChangeQuiet      ScriptOp = "script_map_change_quiet"
	ScriptMapChangeWater      ScriptOp = "script_map_change_water"
	ScriptMapChangeParameter  ScriptOp = "script_map_change_parameter"
	ScriptMapInstantParameter ScriptOp = "script_map_instant_parameter"
	ScriptMapQuietParameter   ScriptOp = "script_map_quiet_parameter"
	ScriptMapWaterParameter   ScriptOp = "script_map_water_parameter"
	ScriptMapRefresh          ScriptOp = "script_map_refresh"
)

const (
	ScriptMapChangeID       = "script_map_change_id"
	ScriptMapRemovePrevious = "script_map_remove_previous"
	ScriptMapWaterShadows   = "script_map_water_shadows"
)

func (f deploymentFormat) mapChangeDefinitions() []scriptDefinition {
	var result []scriptDefinition
	add := func(op ScriptOp, raw []byte, help string, fields ...ScriptField) {
		result = append(
			result,
			scriptDefinition{op: op, raw: raw, help: help, fields: fields},
		)
	}
	id := func(offset, width int, minimum int64) ScriptField {
		field := sceneOperand(ScriptMapChangeID, offset, width, minimum, 127)
		field.Target = ScriptMapChangeTarget
		return field
	}
	if f.kind == eventFE8 {
		add(
			ScriptMapChange,
			[]byte{0x20, 0x27, 0, 0},
			"script_fe8_map_change_help",
			id(2, 2, -3),
		)
		add(
			ScriptMapRevert,
			[]byte{0x21, 0x27, 0, 0},
			"script_fe8_map_revert_help",
			id(2, 2, -3),
		)
		return result
	}
	if f.kind == eventFE6 {
		add(
			ScriptMapChange,
			scriptWords(0x29, 0),
			"script_fe6_map_change_help",
			id(4, 4, -1),
		)
		add(
			ScriptMapChangePosition,
			scriptWords(0x2A, 0),
			"script_word_map_position_help",
			sceneOperand(ScriptX, 4, 1, 0, 255),
			sceneOperand(ScriptY, 5, 1, 0, 255),
		)
		return result
	}
	shift := uint32(0)
	if f.kind == eventFE7U {
		shift = 2
	}
	for _, command := range []struct {
		op, parameter ScriptOp
		code          uint32
		flag          bool
		help          string
	}{
		{ScriptMapChange, ScriptMapChangeParameter, 0x5D, true, "script_fe7_map_change_help"},
		{ScriptMapChangeInstant, ScriptMapInstantParameter, 0x5F, false, "script_fe7_map_instant_help"},
		{ScriptMapChangeQuiet, ScriptMapQuietParameter, 0x60, true, "script_fe7_map_quiet_help"},
		{ScriptMapChangeWater, ScriptMapWaterParameter, 0x62, true, "script_fe7_map_water_help"},
	} {
		code := command.code + shift
		add(
			command.parameter,
			scriptWords(code|0xFFFF0000),
			"script_map_parameter_help",
		)
		field := id(2, 2, 0)
		// IDs occupy 0..127. Other low-half bits are not a new writable contract.
		field.bits = 7
		fields := []ScriptField{field}
		if command.flag {
			remove := sceneOperand(ScriptMapRemovePrevious, 2, 2, 0, 1)
			remove.bits, remove.shift = 1, 15
			fields = append(fields, remove)
		}
		add(command.op, scriptWords(code), command.help, fields...)
	}
	add(
		ScriptMapChangePosition,
		scriptWords(0x5E+shift),
		"script_word_map_position_help",
		sceneOperand(ScriptX, 2, 1, 0, 255),
		sceneOperand(ScriptY, 3, 1, 0, 255),
	)
	add(
		ScriptMapRefresh,
		scriptWords(0x61+shift),
		"script_map_refresh_help",
		sceneOperand(ScriptMapWaterShadows, 2, 2, 0, 1),
	)
	return result
}

func (d *ScriptDraft) IsMapChange(index int) bool {
	return index >= 0 && index < len(d.rows) &&
		mapChangeOperation(d.rows[index].instruction.Op)
}

func mapChangeOperation(op ScriptOp) bool {
	switch op {
	case ScriptMapChange,
		ScriptMapRevert,
		ScriptMapChangePosition,
		ScriptMapChangeInstant,
		ScriptMapChangeQuiet,
		ScriptMapChangeWater,
		ScriptMapChangeParameter,
		ScriptMapInstantParameter,
		ScriptMapQuietParameter,
		ScriptMapWaterParameter:
		return true
	}
	return false
}
