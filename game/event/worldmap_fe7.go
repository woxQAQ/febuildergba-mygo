package event

const ScriptWMPutSprite ScriptOp = "script_wm_put_sprite"

const (
	ScriptWMMapMode = "script_wm_map_mode"
	ScriptWMRegion  = "script_wm_region"
)

const ScriptWMFadeHighlights ScriptOp = "script_wm_fade_highlights"

// FE7 world-map commands are word encoded. Their opcode shift differs from
// ordinary FE7 events; base is AD in FE7J and AC in FE7U.
func fe7WorldMapDefinitions(base byte) []scriptDefinition {
	var definitions []scriptDefinition
	add := func(op ScriptOp, opcode byte, size int, help string, fields ...ScriptField) {
		definition := wmCommand(op, base+(opcode-0xAC), size, help, fields...)
		definition.raw[0], definition.raw[1] = base+(opcode-0xAC), 0
		definitions = append(definitions, definition)
	}
	operand := wmOperand
	coordinate := func(id string, offset int) ScriptField {
		field := operand(id, offset, 2, -1, 0x7FFF)
		field.Help = "script_fe7_wm_coordinate_help"
		return field
	}
	delay := func(offset int) ScriptField {
		field := operand(ScriptWMDelay, offset, 4, -0x80000000, 0x7FFFFFFF)
		field.Help = "script_fe7_wm_delay_help"
		return field
	}
	faceSlot := operand(ScriptWMFaceSlot, 4, 4, 0, 3)
	faceSlot.Help = "script_fe7_wm_face_slot_help"
	portrait := operand(ScriptWMPortrait, 8, 4, 1, 0xFFFF)
	portrait.Target, portrait.Hex = ScriptPortraitTarget, true
	portrait.Help = "script_wm_portrait_help"
	faceBits := func(id string, offset int, bits, shift uint, max int64) ScriptField {
		field := operand(id, offset, 2, 0, max)
		field.bits, field.shift = bits, shift
		return field
	}
	x := faceBits(ScriptWMFaceX, 12, 8, 0, 255)
	x.Help = "script_wm_face_x_help"
	slide := faceBits(ScriptWMFaceSlide, 12, 2, 8, 3)
	slide.Help = "script_wm_face_slide_help"
	flip := faceBits(ScriptWMFaceFlip, 12, 1, 10, 1)
	flip.Help = "script_wm_face_flip_help"
	layer := faceBits(ScriptWMFaceLayer, 12, 2, 13, 3)
	layer.Help = "script_wm_face_layer_help"
	clearSlide := faceBits(ScriptWMFaceSlide, 8, 2, 8, 3)
	clearSlide.Help = "script_fe7_wm_clear_slide_help"
	text := operand(ScriptTextID, 4, 4, 1, 0xFFFF)
	text.Target, text.Hex = ScriptTextTarget, true
	text.Help = "script_fe7_wm_text_help"
	add(
		ScriptWMInitScene,
		0xAC,
		16,
		"script_fe7_wm_init_help",
		operand(
			ScriptWMMapMode,
			4,
			1,
			0,
			1,
		),
		coordinate(ScriptWMX, 8),
		coordinate(ScriptWMY, 10),
	)
	// Fade-in is a native load option; other original option bits stay intact.
	definitions[len(definitions)-1].raw[12] = 4
	add(ScriptWMFadeBlack, 0xAD, 4, "script_wm_fade_black_help")
	add(ScriptWMEndScene, 0xAE, 4, "script_wm_end_scene_help")
	add(
		ScriptWMShowPortrait,
		0xB1,
		20,
		"script_fe7_wm_show_portrait_help",
		faceSlot,
		portrait,
		x,
		slide,
		flip,
		layer,
		delay(16),
	)
	add(
		ScriptWMClearPortrait,
		0xB2,
		16,
		"script_fe7_wm_clear_portrait_help",
		faceSlot,
		clearSlide,
		delay(12),
	)
	add(ScriptWMText, 0xB3, 8, "script_fe7_wm_text_help", text)
	add(ScriptWMTextBottom, 0xB4, 4, "script_wm_text_bottom_help")
	add(ScriptWMTextTop, 0xB5, 4, "script_wm_text_top_help")
	add(ScriptWMTextHide, 0xB6, 4, "script_wm_text_hide_help")
	// The object-list initializer reserves five entries for map sprites.
	unit := operand(ScriptWMUnit, 4, 4, 0, 4)
	unit.Help = "script_fe7_wm_unit_help"
	class := operand(ScriptWMClass, 12, 1, 0, 255)
	class.Optional = true
	class.Help = "script_fe7_wm_class_help"
	faction := operand(ScriptWMFaction, 12, 4, 0, 3)
	faction.bits, faction.shift = 2, 13
	faction.Help = "script_fe7_wm_faction_help"
	add(
		ScriptWMPutSprite,
		0xB7,
		20,
		"script_fe7_wm_put_sprite_help",
		unit,
		coordinate(ScriptWMX, 8),
		coordinate(ScriptWMY, 10),
		class,
		faction,
		delay(16),
	)
	definitions[len(definitions)-1].raw[12] = 1
	add(
		ScriptWMRemoveUnit,
		0xB8,
		12,
		"script_fe7_wm_remove_unit_help",
		unit,
		delay(8),
	)
	highlightSlot := operand(ScriptWMMarkerSlot, 4, 4, 0, 3)
	highlightSlot.Help = "script_fe7_wm_highlight_slot_help"
	dotSlot := operand(ScriptWMMarkerSlot, 4, 4, 0, 4)
	dotSlot.Help = "script_fe7_wm_dot_slot_help"
	region := operand(ScriptWMRegion, 8, 4, 0, 8)
	region.Help = "script_fe7_wm_region_help"
	add(
		ScriptWMHighlight,
		0xB9,
		16,
		"script_fe7_wm_highlight_help",
		highlightSlot,
		region,
		delay(12),
	)
	add(
		ScriptWMClearHighlight,
		0xBA,
		12,
		"script_fe7_wm_clear_highlight_help",
		highlightSlot,
		delay(8),
	)
	add(ScriptWMClearHighlights, 0xBB, 4, "script_fe7_wm_clear_highlights_help")
	// Fade-all consumes no operands, but occupies two words in native dispatch.
	add(ScriptWMFadeHighlights, 0xBC, 8, "script_fe7_wm_fade_highlights_help")
	palette := operand(ScriptWMPaletteBank, 12, 4, 0, 15)
	palette.bits = 4
	palette.Help = "script_fe7_wm_dot_palette_help"
	add(
		ScriptWMDot,
		0xBD,
		20,
		"script_fe7_wm_dot_help",
		dotSlot,
		coordinate(ScriptWMX, 8),
		coordinate(ScriptWMY, 10),
		palette,
		delay(16),
	)
	add(ScriptWMClearDot, 0xBE, 8, "script_fe7_wm_clear_dot_help", dotSlot)
	duration := operand(ScriptWMDuration, 8, 2, 1, 0x7FFF)
	duration.Help = "script_fe7_wm_duration_help"
	add(
		ScriptWMScrollCamera,
		0xC1,
		16,
		"script_fe7_wm_scroll_help",
		coordinate(ScriptWMX, 4),
		coordinate(ScriptWMY, 6),
		duration,
		delay(12),
	)
	return definitions
}
