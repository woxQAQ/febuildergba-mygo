package event

const (
	ScriptWMZoomTo          ScriptOp = "script_wm_zoom_to"
	ScriptWMZoomBack        ScriptOp = "script_wm_zoom_back"
	ScriptWMMovePortrait    ScriptOp = "script_wm_move_portrait"
	ScriptWMArrow           ScriptOp = "script_wm_arrow"
	ScriptWMHighlight       ScriptOp = "script_wm_highlight"
	ScriptWMClearHighlight  ScriptOp = "script_wm_clear_highlight"
	ScriptWMClearHighlights ScriptOp = "script_wm_clear_highlights"
	ScriptWMDot             ScriptOp = "script_wm_dot"
	ScriptWMClearDot        ScriptOp = "script_wm_clear_dot"
	ScriptWMFlag            ScriptOp = "script_wm_flag"
	ScriptWMClearFlag       ScriptOp = "script_wm_clear_flag"
	ScriptWMMapLabel        ScriptOp = "script_wm_map_label"
	ScriptWMClearMapLabel   ScriptOp = "script_wm_clear_map_label"
)

const (
	ScriptWMMarkerSlot  = "script_wm_marker_slot"
	ScriptWMFaceY       = "script_wm_face_y"
	ScriptWMGraphicID   = "script_wm_graphic_id"
	ScriptWMPaletteBank = "script_wm_palette_bank"
)

// FE6 reads word operands after the opcode. Display-object slots are unrelated
// to chapter units or FE8 travel nodes; unused operands retain their bytes.
func fe6WorldMapDefinitions() []scriptDefinition {
	var definitions []scriptDefinition
	add := func(op ScriptOp, opcode byte, size int, help string, fields ...ScriptField) {
		definition := wmCommand(op, opcode, size, help, fields...)
		definition.raw[0], definition.raw[1] = opcode, 0
		definitions = append(definitions, definition)
	}
	operand := wmOperand
	coordinate := func(id string, offset int) ScriptField {
		field := operand(id, offset, 2, -1, 0x7FFF)
		field.Help = "script_fe6_wm_coordinate_help"
		return field
	}
	slot := func(id string, offset, width int, max int64) ScriptField { return operand(id, offset, width, 0, max) }
	portrait := operand(ScriptWMPortrait, 12, 2, 1, 0x7FFF)
	portrait.Target, portrait.Hex = ScriptPortraitTarget, true
	text := operand(ScriptTextID, 4, 4, 1, 0xFFFF)
	text.Target, text.Hex = ScriptTextTarget, true
	add(ScriptWMInitScene, 0x4D, 8, "script_fe6_wm_init_help")
	add(ScriptWMEndScene, 0x4E, 4, "script_fe6_wm_end_help")
	add(
		ScriptWMZoomTo,
		0x4F,
		8,
		"script_fe6_wm_zoom_help",
		coordinate(ScriptWMX, 4),
		coordinate(ScriptWMY, 6),
	)
	add(ScriptWMZoomBack, 0x50, 4, "script_fe6_wm_zoom_back_help")
	add(
		ScriptWMShowPortrait,
		0x51,
		16,
		"script_fe6_wm_show_portrait_help",
		slot(ScriptWMFaceSlot, 4, 2, 3),
		coordinate(ScriptWMFaceX, 8),
		coordinate(ScriptWMFaceY, 10),
		portrait,
	)
	add(
		ScriptWMClearPortrait,
		0x52,
		8,
		"script_fe6_wm_clear_portrait_help",
		slot(ScriptWMFaceSlot, 4, 4, 3),
	)
	// The move process stores Y but changes only the displayed X coordinate.
	add(
		ScriptWMMovePortrait,
		0x53,
		12,
		"script_fe6_wm_move_portrait_help",
		slot(ScriptWMFaceSlot, 4, 2, 3),
		coordinate(ScriptWMFaceX, 8),
	)
	add(ScriptWMText, 0x56, 8, "script_fe6_wm_text_help", text)
	add(ScriptWMTextBottom, 0x57, 4, "script_wm_text_bottom_help")
	add(ScriptWMTextTop, 0x58, 4, "script_wm_text_top_help")
	add(ScriptWMTextHide, 0x59, 4, "script_wm_text_hide_help")
	// Arrow operands select a native path, not a pair of map coordinates. The
	// palette halfword's signed conversion is left intact with the original data.
	add(
		ScriptWMArrow,
		0x5A,
		8,
		"script_fe6_wm_arrow_help",
		slot(ScriptWMGraphicID, 4, 2, 46),
	)
	add(
		ScriptWMHighlight,
		0x5B,
		8,
		"script_fe6_wm_highlight_help",
		slot(ScriptWMMarkerSlot, 4, 2, 1),
		slot(ScriptWMGraphicID, 6, 2, 6),
	)
	add(
		ScriptWMClearHighlight,
		0x5C,
		8,
		"script_fe6_wm_clear_highlight_help",
		slot(ScriptWMMarkerSlot, 4, 4, 1),
	)
	add(ScriptWMClearHighlights, 0x5D, 4, "script_fe6_wm_clear_highlights_help")
	palette := operand(ScriptWMPaletteBank, 12, 4, 0, 15)
	palette.bits = 4
	add(
		ScriptWMDot,
		0x5E,
		16,
		"script_fe6_wm_dot_help",
		slot(ScriptWMMarkerSlot, 4, 4, 1),
		coordinate(ScriptWMX, 8),
		coordinate(ScriptWMY, 10),
		palette,
	)
	add(
		ScriptWMClearDot,
		0x5F,
		8,
		"script_fe6_wm_clear_dot_help",
		slot(ScriptWMMarkerSlot, 4, 4, 1),
	)
	add(
		ScriptWMFlag,
		0x60,
		16,
		"script_fe6_wm_flag_help",
		slot(ScriptWMMarkerSlot, 4, 4, 3),
		coordinate(ScriptWMX, 8),
		coordinate(ScriptWMY, 10),
		palette,
	)
	add(
		ScriptWMClearFlag,
		0x61,
		8,
		"script_fe6_wm_clear_flag_help",
		slot(ScriptWMMarkerSlot, 4, 4, 3),
	)
	add(
		ScriptWMMapLabel,
		0x62,
		20,
		"script_fe6_wm_map_label_help",
		slot(ScriptWMMarkerSlot, 4, 4, 1),
		coordinate(ScriptWMX, 8),
		coordinate(ScriptWMY, 10),
		slot(ScriptWMGraphicID, 12, 2, 34),
	)
	add(
		ScriptWMClearMapLabel,
		0x63,
		8,
		"script_fe6_wm_clear_map_label_help",
		slot(ScriptWMMarkerSlot, 4, 4, 1),
	)
	return definitions
}
