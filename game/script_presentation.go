package game

const (
	ScriptMusicFade             ScriptOp = "script_music_fade"
	ScriptMusicQuieter          ScriptOp = "script_music_quieter"
	ScriptMusicNormalVolume     ScriptOp = "script_music_normal_volume"
	ScriptTextBackgroundMode    ScriptOp = "script_text_background_mode"
	ScriptTextCGMode            ScriptOp = "script_text_cg_mode"
	ScriptTextTutorialMode      ScriptOp = "script_text_tutorial_mode"
	ScriptTextBoxMode           ScriptOp = "script_text_box_mode"
	ScriptTextShortTutorialMode ScriptOp = "script_text_short_tutorial_mode"
	ScriptTextContinue          ScriptOp = "script_text_continue"
	ScriptTextBubbleClear       ScriptOp = "script_text_bubble_clear"
	ScriptRestoreMapScene       ScriptOp = "script_restore_map_scene"
)

func (f deploymentFormat) presentationDefinitions() []scriptDefinition {
	if f.kind != eventFE8 {
		return nil
	}
	var result []scriptDefinition
	for _, entry := range []struct {
		op        ScriptOp
		code, sub byte
		help      string
	}{
		{ScriptMusicQuieter, 0x15, 0, "script_music_volume_help"},
		{ScriptMusicNormalVolume, 0x15, 1, "script_music_volume_help"},
		{ScriptTextBackgroundMode, 0x1A, 1, "script_text_background_mode_help"},
		{ScriptTextCGMode, 0x1A, 2, "script_text_cg_mode_help"},
		{ScriptTextTutorialMode, 0x1A, 3, "script_text_box_position_help"},
		{ScriptTextBoxMode, 0x1A, 4, "script_text_box_position_help"},
		{ScriptTextShortTutorialMode, 0x1A, 5, "script_text_short_tutorial_help"},
		{ScriptTextContinue, 0x1C, 0, "script_text_continue_help"},
		{ScriptTextBubbleClear, 0x20, 0, "script_text_bubble_clear_help"},
		{ScriptRestoreMapScene, 0x22, 0, "script_restore_map_scene_help"},
	} {
		result = append(
			result,
			scriptDefinition{
				op:   entry.op,
				raw:  []byte{0x20 + entry.sub, entry.code, 0, 0},
				help: entry.help,
			},
		)
	}
	speed := sceneOperand(ScriptSpeed, 0, 1, 0, 15)
	speed.bits = 4
	song := sceneOperand(ScriptSong, 2, 2, -0x8000, 0x7FFF)
	song.Help = scriptSlot2Help
	return append(result, scriptDefinition{
		op:     ScriptMusicFade,
		raw:    []byte{0x24, 0x13, 0xFF, 0x7F},
		fields: []ScriptField{speed, song},
		help:   "script_music_fade_help",
	})
}
