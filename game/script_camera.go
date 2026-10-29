package game

const (
	ScriptCameraPosition       ScriptOp = "script_camera_position"
	ScriptCameraCenterPosition ScriptOp = "script_camera_center_position"
	ScriptCameraUnit           ScriptOp = "script_camera_unit"
	ScriptCameraCenterUnit     ScriptOp = "script_camera_center_unit"
	ScriptCursorPosition       ScriptOp = "script_cursor_position"
	ScriptCursorUnit           ScriptOp = "script_cursor_unit"
	ScriptCursorGlowPosition   ScriptOp = "script_cursor_glow_position"
	ScriptCursorGlowUnit       ScriptOp = "script_cursor_glow_unit"
	ScriptCursorFlashPosition  ScriptOp = "script_cursor_flash_position"
	ScriptCursorFlashUnit      ScriptOp = "script_cursor_flash_unit"
	ScriptCursorClear          ScriptOp = "script_cursor_clear"
)

func scriptFE8Character(offset int) ScriptField {
	field := sceneOperand(ScriptCharacter, offset, 2, -3, 0xFF)
	field.Help = "script_fe8_character_help"
	return field
}

// Camera centering and cursor lifetime are native command semantics. In
// particular, FE8's camera subcode does not select instantaneous movement.
func (f deploymentFormat) cameraDefinitions() []scriptDefinition {
	var result []scriptDefinition
	add := func(op ScriptOp, raw []byte, help string, fields ...ScriptField) {
		result = append(
			result,
			scriptDefinition{op: op, raw: raw, fields: fields, help: help},
		)
	}
	if f.kind == eventFE8 {
		x := sceneOperand(ScriptX, 2, 1, -0x80, 0x7F)
		y := sceneOperand(ScriptY, 3, 1, -0x80, 0x7F)
		x.Help, y.Help = "script_slot_b_coordinates_help", "script_slot_b_coordinates_help"
		add(
			ScriptCameraPosition,
			[]byte{0x20, 0x26, 0, 0},
			"script_camera_visible_help",
			x,
			y,
		)
		add(
			ScriptCameraCenterPosition,
			[]byte{0x28, 0x26, 0, 0},
			"script_camera_center_help",
			x,
			y,
		)
		add(
			ScriptCameraUnit,
			[]byte{0x21, 0x26, 1, 0},
			"script_camera_visible_help",
			scriptFE8Character(2),
		)
		add(
			ScriptCameraCenterUnit,
			[]byte{0x29, 0x26, 1, 0},
			"script_camera_center_help",
			scriptFE8Character(2),
		)
		add(
			ScriptCursorPosition,
			[]byte{0x20, 0x3B, 0, 0},
			"script_cursor_persistent_help",
			x,
			y,
		)
		add(
			ScriptCursorUnit,
			[]byte{0x21, 0x3B, 1, 0},
			"script_cursor_persistent_help",
			scriptFE8Character(2),
		)
		add(
			ScriptCursorGlowPosition,
			[]byte{0x24, 0x3B, 0, 0},
			"script_cursor_persistent_help",
			x,
			y,
		)
		add(
			ScriptCursorGlowUnit,
			[]byte{0x25, 0x3B, 1, 0},
			"script_cursor_persistent_help",
			scriptFE8Character(2),
		)
		add(ScriptCursorClear, []byte{0x22, 0x3B, 0, 0}, "")
		return result
	}

	x := sceneOperand(ScriptX, 4, 2, -0x8000, 0x7FFF)
	y := sceneOperand(ScriptY, 6, 2, -0x8000, 0x7FFF)
	x.Help, y.Help = scriptWordCoordinateHelp, scriptWordCoordinateHelp
	character := sceneOperand(ScriptCharacter, 4, 2, 0, 0xFF)
	camera, flash := uint32(0x1D), uint32(0x64)
	if f.kind == eventFE7U {
		camera, flash = 0x1F, 0x66
	}
	if f.kind == eventFE6 {
		flash = 0x2C
		add(
			ScriptCameraPosition,
			scriptWords(0x0B, 0),
			"script_camera_visible_help",
			x,
			y,
		)
		add(
			ScriptCameraUnit,
			scriptWords(0x0C, 1),
			"script_camera_visible_help",
			character,
		)
	} else {
		cameraX := sceneOperand(ScriptX, 2, 1, 0, 0xFF)
		cameraY := sceneOperand(ScriptY, 3, 1, 0, 0xFF)
		add(
			ScriptCameraPosition,
			scriptWords(camera),
			"script_camera_visible_help",
			cameraX,
			cameraY,
		)
		add(
			ScriptCameraCenterPosition,
			scriptWords(camera+3),
			"script_camera_center_help",
			cameraX,
			cameraY,
		)
		add(
			ScriptCameraUnit,
			scriptWords(camera+1|1<<16),
			"script_camera_visible_help",
			sceneOperand(ScriptCharacter, 2, 2, 0, 0xFF),
		)
	}
	add(
		ScriptCursorFlashPosition,
		scriptWords(flash, 0),
		"script_cursor_flash_help",
		x,
		y,
	)
	add(
		ScriptCursorFlashUnit,
		scriptWords(flash+1, 1),
		"script_cursor_flash_help",
		character,
	)
	add(
		ScriptCursorPosition,
		scriptWords(flash+2, 0),
		"script_cursor_persistent_help",
		x,
		y,
	)
	add(ScriptCursorClear, scriptWords(flash+3), "")
	return result
}
