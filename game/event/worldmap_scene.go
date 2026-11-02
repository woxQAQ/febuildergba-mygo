package event

import (
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
)

const (
	ScriptWMFadeBlack        ScriptOp = "script_wm_fade_black"
	ScriptWMEndScene         ScriptOp = "script_wm_end_scene"
	ScriptWMDarken           ScriptOp = "script_wm_darken"
	ScriptWMWaitDarken       ScriptOp = "script_wm_wait_darken"
	ScriptWMShowTextBox      ScriptOp = "script_wm_show_text_box"
	ScriptWMWaitTextBox      ScriptOp = "script_wm_wait_text_box"
	ScriptWMRevealScreen     ScriptOp = "script_wm_reveal_screen"
	ScriptWMWaitRevealScreen ScriptOp = "script_wm_wait_reveal_screen"
	ScriptWMInitScene        ScriptOp = "script_wm_init_scene"
	ScriptWMSkipScene        ScriptOp = "script_wm_skip_scene"
	ScriptWMNoFade           ScriptOp = "script_wm_no_fade"
	ScriptWMShowPortrait     ScriptOp = "script_wm_show_portrait"
	ScriptWMClearPortrait    ScriptOp = "script_wm_clear_portrait"
	ScriptWMText             ScriptOp = "script_wm_text"
	ScriptWMTextBottom       ScriptOp = "script_wm_text_bottom"
	ScriptWMTextTop          ScriptOp = "script_wm_text_top"
	ScriptWMTextHide         ScriptOp = "script_wm_text_hide"

	ScriptWMFaceSlot      = "script_wm_face_slot"
	ScriptWMPortrait      = "script_wm_portrait"
	ScriptWMFaceX         = "script_wm_face_x"
	ScriptWMFaceSlide     = "script_wm_face_slide"
	ScriptWMFaceFlip      = "script_wm_face_flip"
	ScriptWMFaceLayer     = "script_wm_face_layer"
	ScriptWMFaceDelay     = "script_wm_face_delay"
	ScriptWMFadeFrames    = "script_wm_fade_frames"
	ScriptWMTextBoxFrames = "script_wm_text_box_frames"
	ScriptWMTextBoxCurve  = "script_wm_text_box_curve"
	ScriptWMRevealStyle   = "script_wm_reveal_style"
)

func fe8WorldMapSceneDefinitions() []scriptDefinition {
	operand := func(id string, offset, width int, minValue, maxValue int64, help string) ScriptField {
		field := wmOperand(id, offset, width, minValue, maxValue)
		field.Help = help
		return field
	}
	faceSlot := func(offset int) ScriptField {
		return operand(
			ScriptWMFaceSlot,
			offset,
			2,
			0,
			1,
			"script_wm_face_slot_help",
		)
	}
	slide := func(offset int) ScriptField {
		field := operand(
			ScriptWMFaceSlide,
			offset,
			2,
			0,
			3,
			"script_wm_face_slide_help",
		)
		field.bits, field.shift = 2, 8
		if offset == 4 {
			field.Help = "script_wm_face_clear_slide_help"
		}
		return field
	}
	faceX := operand(ScriptWMFaceX, 8, 2, 0, 255, "script_wm_face_x_help")
	faceX.bits = 8
	flip := operand(ScriptWMFaceFlip, 8, 2, 0, 1, "script_wm_face_flip_help")
	flip.bits, flip.shift = 1, 10
	layer := operand(ScriptWMFaceLayer, 8, 2, 0, 3, "script_wm_face_layer_help")
	layer.bits, layer.shift = 2, 13
	portrait := operand(
		ScriptWMPortrait,
		6,
		2,
		1,
		0x7FFF,
		"script_wm_portrait_help",
	)
	portrait.Target, portrait.Hex = ScriptPortraitTarget, true
	text := operand(ScriptTextID, 4, 2, 1, 0xFFFF, "script_wm_text_id_help")
	text.Target, text.Hex, text.orOffset = ScriptTextTarget, true, 6
	reveal := operand(
		ScriptWMRevealStyle,
		4,
		4,
		0,
		255,
		"script_wm_reveal_style_help",
	)
	reveal.bits = 8
	return []scriptDefinition{
		wmCommand(ScriptWMFadeBlack, 0x81, 4, "script_wm_fade_black_help"),
		// Ending the world-map process does not end the event interpreter.
		wmCommand(ScriptWMEndScene, 0x82, 4, "script_wm_end_scene_help"),
		wmCommand(
			ScriptWMDarken,
			0xAD,
			8,
			"script_wm_darken_help",
			operand(
				ScriptWMFadeFrames,
				4,
				4,
				-0x80000000,
				0x7FFFFFFF,
				"script_wm_fade_frames_help",
			),
		),
		wmCommand(ScriptWMWaitDarken, 0xAE, 4, "script_wm_darken_help"),
		wmCommand(
			ScriptWMShowTextBox,
			0xAF,
			8,
			"script_wm_show_text_box_help",
			operand(
				ScriptWMTextBoxFrames,
				4,
				2,
				0,
				0xFFFF,
				"script_wm_text_box_frames_help",
			),
			operand(
				ScriptWMTextBoxCurve,
				6,
				2,
				-0x8000,
				0x7FFF,
				"script_wm_text_box_curve_help",
			),
		),
		wmCommand(ScriptWMWaitTextBox, 0xB1, 4, "script_wm_show_text_box_help"),
		wmCommand(
			ScriptWMRevealScreen,
			0xB2,
			8,
			"script_wm_reveal_screen_help",
			reveal,
		),
		wmCommand(
			ScriptWMWaitRevealScreen,
			0xB3,
			4,
			"script_wm_reveal_screen_help",
		),
		wmCommand(ScriptWMInitScene, 0xC0, 4, "script_wm_init_scene_help"),
		wmCommand(ScriptWMSkipScene, 0xC1, 4, "script_wm_skip_scene_help"),
		wmCommand(ScriptWMNoFade, 0xC2, 4, "script_wm_no_fade_help"),
		wmCommand(
			ScriptWMShowPortrait,
			0xC4,
			12,
			"script_wm_show_portrait_help",
			faceSlot(4),
			portrait,
			faceX,
			slide(8),
			flip,
			layer,
			operand(
				ScriptWMFaceDelay,
				10,
				2,
				-0x8000,
				0x7FFF,
				"script_wm_face_show_delay_help",
			),
		),
		wmCommand(
			ScriptWMClearPortrait,
			0xC5,
			8,
			"script_wm_clear_portrait_help",
			faceSlot(
				2,
			),
			slide(4),
			operand(
				ScriptWMFaceDelay,
				6,
				2,
				0,
				0xFFFF,
				"script_wm_face_clear_delay_help",
			),
		),
		wmCommand(ScriptWMText, 0xC6, 8, "script_wm_text_help", text),
		wmCommand(ScriptWMTextBottom, 0xC7, 4, "script_wm_text_bottom_help"),
		wmCommand(ScriptWMTextTop, 0xC8, 4, "script_wm_text_top_help"),
		wmCommand(ScriptWMTextHide, 0xC9, 4, "script_wm_text_hide_help"),
	}
}

func (s *Editor) validateScriptPortrait(id int64) error {
	if s.source.Portraits == nil {
		return edit.ErrLayout
	}
	portrait, err := s.source.Portraits.Read(int(id))
	if err != nil {
		return err
	}
	if portrait.Entry.Empty || portrait.Entry.ClassCard {
		return edit.ErrValue
	}
	if portrait.MainErr != nil {
		return portrait.MainErr
	}
	if portrait.Main == nil {
		return edit.ErrValue
	}
	return nil
}
