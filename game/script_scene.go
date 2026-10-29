package game

import "encoding/binary"

const scriptSlot2Help = "script_slot2_help"

const (
	ScriptMoveUnit       ScriptOp = "script_move_unit"
	ScriptMoveUnitSpeed  ScriptOp = "script_move_unit_speed"
	ScriptJoinPlayer     ScriptOp = "script_join_player"
	ScriptJoinNPC        ScriptOp = "script_join_npc"
	ScriptJoinEnemy      ScriptOp = "script_join_enemy"
	ScriptGiveItem       ScriptOp = "script_give_item"
	ScriptGiveActiveItem ScriptOp = "script_give_active_item"
	ScriptGiveItemSlot   ScriptOp = "script_give_item_slot"
	ScriptGiveGoldSlot   ScriptOp = "script_give_gold_slot"
	ScriptMusic          ScriptOp = "script_music"
	ScriptMusicOverride  ScriptOp = "script_music_override"
	ScriptMusicRestore   ScriptOp = "script_music_restore"
	ScriptMusicFadeOut   ScriptOp = "script_music_fade_out"
	ScriptSound          ScriptOp = "script_sound"
	ScriptFadeToBlack    ScriptOp = "script_fade_to_black"
	ScriptFadeFromBlack  ScriptOp = "script_fade_from_black"
	ScriptFadeToWhite    ScriptOp = "script_fade_to_white"
	ScriptFadeFromWhite  ScriptOp = "script_fade_from_white"
	ScriptBackground     ScriptOp = "script_background"
	ScriptHideMap        ScriptOp = "script_hide_map"
	ScriptShowMap        ScriptOp = "script_show_map"
	ScriptEventBitOn     ScriptOp = "script_event_bit_on"
	ScriptEventBitOff    ScriptOp = "script_event_bit_off"
)

const (
	ScriptCharacter    = "script_character"
	ScriptItem         = "script_item"
	ScriptX            = "script_x"
	ScriptY            = "script_y"
	ScriptSpeed        = "script_speed"
	ScriptSong         = "script_song"
	ScriptSoundID      = "script_sound_id"
	ScriptBackgroundID = "script_background_id"
	ScriptEventBit     = "script_event_bit"
)

const scriptWordCoordinateHelp = "script_word_coordinate_help"

func scriptWords(values ...uint32) []byte {
	var result []byte
	for _, value := range values {
		result = binary.LittleEndian.AppendUint32(result, value)
	}
	return result
}

func sceneOperand(
	id string,
	offset, width int,
	minimum, maximum int64,
) ScriptField {
	field := scriptOperand(id, offset, width, maximum)
	field.Min = minimum
	field.Hex = id != ScriptX && id != ScriptY && id != ScriptSpeed &&
		id != ScriptFrames
	switch id {
	case ScriptCharacter:
		field.Reference = Units
	case ScriptItem:
		field.Reference = Items
	}
	return field
}

// Scene commands contain immediate operands, not resource addresses. Native
// coordinate widths and FE8 unit selectors remain distinct from deployment IDs.
// Encodings follow the fixed legacy catalogs and the native event interpreters.
func (f deploymentFormat) sceneDefinitions() []scriptDefinition {
	var result []scriptDefinition
	add := func(op ScriptOp, raw []byte, fields ...ScriptField) {
		result = append(
			result,
			scriptDefinition{op: op, raw: raw, fields: fields},
		)
	}
	if f.kind == eventFE8 {
		character := scriptFE8Character
		add(
			ScriptMoveUnit,
			[]byte{0x40, 0x2F, 0, 0, 1, 0, 0, 0},
			sceneOperand(ScriptSpeed, 2, 2, -0x8000, 0x7FFF),
			character(4),
			sceneOperand(
				ScriptX,
				6,
				1,
				-0x80,
				0x7F,
			),
			sceneOperand(ScriptY, 7, 1, -0x80, 0x7F),
		)
		result[len(result)-1].help = "script_fe8_move_help"
		for index, op := range []ScriptOp{ScriptJoinPlayer, ScriptJoinNPC, ScriptJoinEnemy} {
			add(op, []byte{0x22 + byte(index), 0x34, 1, 0}, character(2))
		}
		add(ScriptGiveItemSlot, []byte{0x20, 0x37, 0xFF, 0xFF}, character(2))
		result[len(result)-1].help = "script_item_slot_help"
		add(ScriptGiveGoldSlot, []byte{0x21, 0x37, 0xFF, 0xFF}, character(2))
		result[len(result)-1].help = "script_gold_slot_help"
		for _, entry := range []struct {
			op    ScriptOp
			code  byte
			field string
		}{
			{ScriptMusic, 0x12, ScriptSong},
			{ScriptMusicOverride, 0x14, ScriptSong},
			{ScriptSound, 0x16, ScriptSoundID},
		} {
			field := sceneOperand(entry.field, 2, 2, -0x8000, 0x7FFF)
			field.Help = scriptSlot2Help
			add(entry.op, []byte{0x20, entry.code, 0, 0}, field)
		}
		add(
			ScriptMusicRestore,
			[]byte{0x21, 0x14, 6, 0},
			sceneOperand(ScriptSpeed, 2, 2, 0, 0x7FFF),
		)
		for sub, op := range []ScriptOp{ScriptFadeFromBlack, ScriptFadeToBlack, ScriptFadeFromWhite, ScriptFadeToWhite} {
			add(
				op,
				[]byte{0x20 + byte(sub), 0x17, 0x10, 0},
				sceneOperand(ScriptSpeed, 2, 2, 1, 0x7FFF),
			)
		}
		background := sceneOperand(ScriptBackgroundID, 2, 2, -0x8000, 0x7FFF)
		background.Help = scriptSlot2Help
		add(ScriptBackground, []byte{0x40, 0x21, 0, 0, 0, 0, 0, 0}, background)
		add(ScriptHideMap, []byte{0x20, 0x23, 0, 0})
		add(ScriptShowMap, []byte{0x20, 0x24, 0, 0})
		add(
			ScriptEventBitOn,
			[]byte{0x28, 2, 7, 0},
			sceneOperand(ScriptEventBit, 2, 2, 0, 15),
		)
		result[len(result)-1].help = "script_event_bit_help"
		add(
			ScriptEventBitOff,
			[]byte{0x20, 2, 9, 0},
			sceneOperand(ScriptEventBit, 2, 2, 0, 15),
		)
		result[len(result)-1].help = "script_event_bit_help"
		return result
	}

	character := sceneOperand(ScriptCharacter, 4, 2, 0, 0xFF)
	x, y := sceneOperand(
		ScriptX,
		8,
		2,
		-0x8000,
		0x7FFF,
	), sceneOperand(
		ScriptY,
		10,
		2,
		-0x8000,
		0x7FFF,
	)
	x.Help, y.Help = scriptWordCoordinateHelp, scriptWordCoordinateHelp
	move, faction, item, activeItem := uint32(
		0x24,
	), uint32(
		0x63,
	), uint32(
		0x5A,
	), uint32(
		0x59,
	)
	music, override, restore, fadeMusic, sound := uint32(
		0x76,
	), uint32(
		0x77,
	), uint32(
		0x78,
	), uint32(
		0x7A,
	), uint32(
		0x7D,
	)
	fade, hide, show := uint32(0x8C), uint32(0x90), uint32(0x91)
	if f.kind == eventFE7U {
		move, faction, item, activeItem = move+2, faction+2, item+2, activeItem+2
		music, override, restore, fadeMusic, sound = music+2, override+2, restore+2, fadeMusic+2, sound+2
		fade, hide, show = fade+2, hide+2, show+2
	}
	if f.kind == eventFE6 {
		move, faction, item, activeItem = 0x0F, 0x2B, 0x27, 0x26
		music, override, restore, fadeMusic, sound = 0x36, 0x37, 0x38, 0x39, 0x3C
		fade, hide, show = 0x43, 0x47, 0x48
	}
	add(ScriptMoveUnit, scriptWords(move, 1, 0), character, x, y)
	if f.kind != eventFE6 {
		add(ScriptMoveUnitSpeed, scriptWords(move+1, 1, 0, 0), character, x, y,
			sceneOperand(ScriptSpeed, 12, 2, 0, 0xFFFF))
	}
	for index, op := range []ScriptOp{ScriptJoinPlayer, ScriptJoinNPC, ScriptJoinEnemy} {
		add(op, scriptWords(faction, 1, uint32(index)*0x40), character)
	}
	add(
		ScriptGiveItem,
		scriptWords(item, 1, 1),
		character,
		sceneOperand(ScriptItem, 8, 2, 0, 0xFFFF),
	)
	result[len(result)-1].help = "script_item_parameter_help"
	add(
		ScriptGiveActiveItem,
		scriptWords(activeItem, 1),
		sceneOperand(ScriptItem, 4, 2, 0, 0xFFFF),
	)
	result[len(result)-1].help = "script_item_parameter_help"
	for _, entry := range []struct {
		op    ScriptOp
		code  uint32
		field string
	}{
		{ScriptMusic, music, ScriptSong},
		{ScriptMusicOverride, override, ScriptSong},
		{ScriptSound, sound, ScriptSoundID},
		{ScriptMusicFadeOut, fadeMusic, ScriptSpeed},
	} {
		raw, offset := scriptWords(entry.code), 2
		if f.kind == eventFE6 {
			raw, offset = scriptWords(entry.code, 0), 4
		}
		add(entry.op, raw, sceneOperand(entry.field, offset, 2, 0, 0xFFFF))
	}
	if f.kind == eventFE6 {
		add(ScriptMusicRestore, scriptWords(restore))
		add(
			ScriptBackground,
			scriptWords(3, 0),
			sceneOperand(ScriptBackgroundID, 4, 2, 0, 0xFFFF),
		)
	} else {
		add(
			ScriptMusicRestore,
			scriptWords(restore|6<<16),
			sceneOperand(ScriptSpeed, 2, 2, 0, 0xFFFF),
		)
		add(
			ScriptBackground,
			scriptWords(5),
			sceneOperand(ScriptBackgroundID, 2, 2, 0, 0xFFFF),
		)
	}
	for index, op := range []ScriptOp{ScriptFadeToBlack, ScriptFadeFromBlack, ScriptFadeToWhite, ScriptFadeFromWhite} {
		raw, offset := scriptWords(fade+uint32(index)|0x10<<16), 2
		if f.kind == eventFE6 {
			raw, offset = scriptWords(fade+uint32(index), 0x10), 4
		}
		add(op, raw, sceneOperand(ScriptSpeed, offset, 2, 1, 0x7FFF))
	}
	add(ScriptHideMap, scriptWords(hide))
	add(ScriptShowMap, scriptWords(show))
	return result
}
