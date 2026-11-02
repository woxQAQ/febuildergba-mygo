package event

import (
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/worldmap"
)

const (
	ScriptWMRevealRoute     ScriptOp = "script_wm_reveal_route"
	ScriptWMOpenRoute       ScriptOp = "script_wm_open_route"
	ScriptWMCloseRoute      ScriptOp = "script_wm_close_route"
	ScriptWMEnableNode      ScriptOp = "script_wm_enable_node"
	ScriptWMDisableNode     ScriptOp = "script_wm_disable_node"
	ScriptWMRevealNode      ScriptOp = "script_wm_reveal_node"
	ScriptWMRevealRouteNode ScriptOp = "script_wm_reveal_route_node"
	ScriptWMNextStoryNode   ScriptOp = "script_wm_next_story_node"
	ScriptWMRouteStoryNode  ScriptOp = "script_wm_route_story_node"
	ScriptWMWaitNode        ScriptOp = "script_wm_wait_node"
	ScriptWMStoryNode       ScriptOp = "script_wm_story_node"
	ScriptWMMarkStoryNode   ScriptOp = "script_wm_mark_story_node"
	ScriptWMNextStorySilent ScriptOp = "script_wm_next_story_silent"
	ScriptWMClearStoryNode  ScriptOp = "script_wm_clear_story_node"
	ScriptWMClassUnit       ScriptOp = "script_wm_class_unit"
	ScriptWMCharacterUnit   ScriptOp = "script_wm_character_unit"
	ScriptWMRemoveUnit      ScriptOp = "script_wm_remove_unit"
	ScriptWMShowUnit        ScriptOp = "script_wm_show_unit"
	ScriptWMHideUnit        ScriptOp = "script_wm_hide_unit"
	ScriptWMFadeInUnit      ScriptOp = "script_wm_fade_in_unit"
	ScriptWMFadeOutUnit     ScriptOp = "script_wm_fade_out_unit"
	ScriptWMWaitUnitFade    ScriptOp = "script_wm_wait_unit_fade"
	ScriptWMUnitNode        ScriptOp = "script_wm_unit_node"
	ScriptWMUnitPosition    ScriptOp = "script_wm_unit_position"
	ScriptWMFreeMove        ScriptOp = "script_wm_free_move"
	ScriptWMRouteMove       ScriptOp = "script_wm_route_move"
	ScriptWMPauseMove       ScriptOp = "script_wm_pause_move"
	ScriptWMResumeMove      ScriptOp = "script_wm_resume_move"
	ScriptWMWaitMove        ScriptOp = "script_wm_wait_move"
	ScriptWMSpawnLord       ScriptOp = "script_wm_spawn_lord"
	ScriptWMCameraPosition  ScriptOp = "script_wm_camera_position"
	ScriptWMCameraNode      ScriptOp = "script_wm_camera_node"
	ScriptWMCameraUnit      ScriptOp = "script_wm_camera_unit"
	ScriptWMScrollCamera    ScriptOp = "script_wm_scroll_camera"
	ScriptWMScrollNode      ScriptOp = "script_wm_scroll_node"
	ScriptWMScrollUnit      ScriptOp = "script_wm_scroll_unit"
	ScriptWMWaitCamera      ScriptOp = "script_wm_wait_camera"
)

const (
	ScriptWMNode         = "script_wm_node"
	ScriptWMRoute        = "script_wm_route"
	ScriptWMUnit         = "script_wm_unit"
	ScriptWMCharacter    = "script_wm_character"
	ScriptWMClass        = "script_wm_class"
	ScriptWMFaction      = "script_wm_faction"
	ScriptWMFromNode     = "script_wm_from_node"
	ScriptWMToNode       = "script_wm_to_node"
	ScriptWMDuration     = "script_wm_duration"
	ScriptWMDelay        = "script_wm_delay"
	ScriptWMFlags        = "script_wm_flags"
	ScriptWMAcceleration = "script_wm_acceleration"
	ScriptWMX            = "script_wm_x"
	ScriptWMY            = "script_wm_y"
	ScriptWMStartX       = "script_wm_start_x"
	ScriptWMStartY       = "script_wm_start_y"
	ScriptWMEndX         = "script_wm_end_x"
	ScriptWMEndY         = "script_wm_end_y"
)

// The display process creates seven units, despite the persistent GMData array
// reserving eight slots. Operands accessing the display process use 0..6.
const worldMapUnitCount = 7

func wmOperand(id string, offset, width int, min, max int64) ScriptField {
	field := scriptOperand(id, offset, width, max)
	field.Min, field.Hex = min, false
	switch id {
	case ScriptWMNode, ScriptWMFromNode, ScriptWMToNode:
		field.Target = ScriptWorldMapNodeTarget
		field.Hex = true
	case ScriptWMRoute:
		field.Target = ScriptWorldMapRouteTarget
		field.Hex = true
	case ScriptWMCharacter:
		field.Reference = records.Units
		field.Hex = true
	case ScriptWMClass:
		field.Reference = records.Classes
		field.Hex = true
	case ScriptWMUnit:
		field.Help = "script_wm_unit_help"
	case ScriptWMFlags:
		field.bits = 3
		field.Hex = true
		field.Help = "script_wm_flags_help"
	case ScriptWMAcceleration:
		field.bits = 8
		field.Help = "script_wm_acceleration_help"
	case ScriptWMDuration:
		field.Help = "script_wm_duration_help"
	}
	return field
}

// wmCommand describes scalar world-map operands; unused bytes remain opaque.
// These consumers do not contain ROM pointers or internal script addresses.
func wmCommand(
	op ScriptOp,
	opcode byte,
	size int,
	help string,
	fields ...ScriptField,
) scriptDefinition {
	raw := make([]byte, size)
	raw[0], raw[1] = byte(size/2)<<4, opcode
	preserved := make([]byte, size)
	for i := 2; i < size; i++ {
		preserved[i] = 0xFF
	}
	for _, field := range fields {
		initial := max(field.Min, 0)
		switch field.ID {
		case ScriptWMDuration, ScriptWMFadeFrames, ScriptWMTextBoxFrames:
			initial = 60
		case ScriptWMFaceX:
			initial = 52
		case ScriptWMFaceSlide, ScriptWMFaceFlip:
			initial = 1
		}
		if field.ID == ScriptWMToNode {
			initial = 1
		}
		field.put(raw, initial)
	}
	return scriptDefinition{
		op:        op,
		raw:       raw,
		fields:    fields,
		preserved: preserved,
		help:      help,
	}
}

func fe8WorldMapDefinitions() []scriptDefinition {
	var definitions []scriptDefinition
	add := func(op ScriptOp, opcode byte, size int, help string, fields ...ScriptField) {
		definitions = append(
			definitions,
			wmCommand(op, opcode, size, help, fields...),
		)
	}
	node := func(id string, offset, width int) ScriptField {
		return wmOperand(id, offset, width, 0, worldmap.NodeCount-1)
	}
	unit := func(offset, width int, all bool) ScriptField {
		minimum := int64(0)
		if all {
			minimum = -1
		}
		return wmOperand(
			ScriptWMUnit,
			offset,
			width,
			minimum,
			worldMapUnitCount-1,
		)
	}
	coords := func(x, y string, offset int) []ScriptField {
		return []ScriptField{
			wmOperand(x, offset, 2, -32768, 32767),
			wmOperand(y, offset+2, 2, -32768, 32767),
		}
	}
	time := func(offset int) []ScriptField {
		return []ScriptField{
			wmOperand(ScriptWMDuration, offset, 2, 1, 32767),
			wmOperand(ScriptWMDelay, offset+2, 2, 0, 32767),
		}
	}
	for _, command := range []struct {
		op   ScriptOp
		code byte
	}{
		{ScriptWMRevealRoute, 0x90}, {ScriptWMOpenRoute, 0x91}, {ScriptWMCloseRoute, 0x92}, {ScriptWMRevealRouteNode, 0x96}, {ScriptWMRouteStoryNode, 0x98},
	} {
		add(
			command.op,
			command.code,
			8,
			"script_wm_route_help",
			wmOperand(ScriptWMRoute, 4, 4, 0, worldmap.RouteCount-1),
		)
	}
	for _, command := range []struct {
		op   ScriptOp
		code byte
	}{
		{ScriptWMEnableNode, 0x93}, {ScriptWMDisableNode, 0x94}, {ScriptWMRevealNode, 0x95}, {ScriptWMStoryNode, 0x9A}, {ScriptWMMarkStoryNode, 0x9B}, {ScriptWMClearStoryNode, 0x9D},
	} {
		add(
			command.op,
			command.code,
			8,
			"script_wm_node_help",
			node(ScriptWMNode, 4, 4),
		)
	}
	for _, command := range []struct {
		op   ScriptOp
		code byte
	}{
		{ScriptWMNextStoryNode, 0x97}, {ScriptWMWaitNode, 0x99}, {ScriptWMNextStorySilent, 0x9C}, {ScriptWMWaitUnitFade, 0xA5}, {ScriptWMWaitCamera, 0x89},
	} {
		add(command.op, command.code, 4, "script_wm_context_help")
	}
	for _, command := range []struct {
		op   ScriptOp
		code byte
		id   string
	}{
		{ScriptWMClassUnit, 0x9E, ScriptWMClass}, {ScriptWMCharacterUnit, 0x9F, ScriptWMCharacter},
	} {
		add(
			command.op,
			command.code,
			12,
			"script_wm_place_help",
			unit(4, 2, false),
			wmOperand(command.id, 6, 2, 1, 0xFFFF),
			wmOperand(ScriptWMFaction, 8, 2, 0, 2),
			node(ScriptWMNode, 10, 2),
		)
	}
	add(
		ScriptWMRemoveUnit,
		0xA0,
		8,
		"script_wm_context_help",
		unit(4, 4, false),
	)
	add(ScriptWMShowUnit, 0xA1, 8, "script_wm_context_help", unit(4, 4, true))
	add(ScriptWMHideUnit, 0xA2, 8, "script_wm_context_help", unit(4, 4, true))
	for _, command := range []struct {
		op   ScriptOp
		code byte
	}{{ScriptWMFadeInUnit, 0xA3}, {ScriptWMFadeOutUnit, 0xA4}} {
		add(
			command.op,
			command.code,
			8,
			"script_wm_context_help",
			unit(4, 2, false),
			wmOperand(ScriptWMDuration, 6, 2, 1, 32767),
		)
	}
	add(
		ScriptWMUnitNode,
		0xA6,
		8,
		"script_wm_context_help",
		unit(4, 2, false),
		node(ScriptWMNode, 6, 2),
	)
	add(
		ScriptWMUnitPosition,
		0xA7,
		12,
		"script_wm_position_help",
		append(
			[]ScriptField{unit(4, 2, false)},
			coords(ScriptWMX, ScriptWMY, 8)...)...)
	free := []ScriptField{
		unit(4, 2, false),
		wmOperand(ScriptWMAcceleration, 6, 2, 0, 255),
	}
	free = append(free, coords(ScriptWMStartX, ScriptWMStartY, 8)...)
	free = append(free, coords(ScriptWMEndX, ScriptWMEndY, 12)...)
	free = append(
		free,
		wmOperand(ScriptWMDuration, 16, 2, 1, 32767),
		wmOperand(ScriptWMFlags, 18, 2, 0, 7),
		wmOperand(ScriptWMDelay, 20, 2, 0, 32767),
	)
	add(ScriptWMFreeMove, 0xA8, 24, "script_wm_free_move_help", free...)
	add(
		ScriptWMRouteMove,
		0xA9,
		16,
		"script_wm_route_move_help",
		unit(4, 2, false),
		wmOperand(ScriptWMFlags, 6, 2, 0, 7),
		node(ScriptWMFromNode, 8, 2),
		node(ScriptWMToNode, 10, 2),
		wmOperand(ScriptWMDuration, 12, 2, -32768, 32767),
		wmOperand(ScriptWMDelay, 14, 2, 0, 32767),
	)
	add(ScriptWMPauseMove, 0xAA, 8, "script_wm_context_help", unit(4, 2, false))
	add(
		ScriptWMResumeMove,
		0xAB,
		8,
		"script_wm_context_help",
		unit(4, 2, false),
	)
	add(ScriptWMWaitMove, 0xAC, 8, "script_wm_wait_move_help", unit(4, 2, true))
	add(
		ScriptWMSpawnLord,
		0xC3,
		12,
		"script_wm_spawn_help",
		unit(4, 2, false),
		wmOperand(ScriptWMCharacter, 6, 2, 0, 0xFFFF),
		node(ScriptWMNode, 8, 4),
	)
	camera := coords(ScriptWMX, ScriptWMY, 4)
	// The direct camera setter consumes unsigned halfwords; scroll starts use
	// signed -1 as the current camera coordinate.
	camera[0].Min, camera[0].Max = 0, 65535
	camera[1].Min, camera[1].Max = 0, 65535
	add(ScriptWMCameraPosition, 0x83, 8, "script_wm_camera_help", camera...)
	add(
		ScriptWMCameraNode,
		0x84,
		8,
		"script_wm_camera_help",
		node(ScriptWMNode, 4, 4),
	)
	add(ScriptWMCameraUnit, 0x85, 8, "script_wm_camera_help", unit(4, 4, false))
	scroll := append(
		coords(ScriptWMStartX, ScriptWMStartY, 4),
		coords(ScriptWMEndX, ScriptWMEndY, 8)...)
	scroll = append(scroll, time(12)...)
	add(ScriptWMScrollCamera, 0x86, 16, "script_wm_camera_help", scroll...)
	scroll = append(
		coords(ScriptWMStartX, ScriptWMStartY, 4),
		node(ScriptWMNode, 8, 2),
	)
	scroll = append(scroll, time(10)...)
	add(ScriptWMScrollNode, 0x87, 16, "script_wm_camera_help", scroll...)
	scroll = append(
		coords(ScriptWMStartX, ScriptWMStartY, 4),
		unit(8, 2, false),
	)
	scroll = append(scroll, time(10)...)
	add(ScriptWMScrollUnit, 0x88, 16, "script_wm_camera_help", scroll...)
	return definitions
}

func (s *Editor) validateWorldMapOperand(field ScriptField, value int64) error {
	if field.ID == ScriptWMCharacter || field.ID == ScriptWMClass {
		if field.ID == ScriptWMCharacter && value == 0 {
			return nil
		}
		table := s.records.Table(field.Reference)
		if table == nil || table.Err != nil {
			return edit.ErrLayout
		}
		for _, row := range table.Rows {
			if int64(row.ID) == value {
				return nil
			}
		}
		return edit.ErrValue
	}
	if field.Target != ScriptWorldMapNodeTarget &&
		field.Target != ScriptWorldMapRouteTarget {
		return nil
	}
	if s.source.WorldMap == nil {
		return edit.ErrLayout
	}
	if field.Target == ScriptWorldMapNodeTarget {
		nodes, err := s.source.WorldMap.Nodes()
		if err != nil {
			return err
		}
		if value < 0 || value >= int64(len(nodes)) {
			return edit.ErrValue
		}
	} else {
		routes, err := s.source.WorldMap.Routes()
		if err != nil {
			return err
		}
		if value < 0 || value >= int64(len(routes)) {
			return edit.ErrValue
		}
	}
	return nil
}

func (s *Editor) validateWorldMapMovement(
	row, old scriptRow,
	found bool,
) error {
	if row.instruction.Op != ScriptWMRouteMove {
		return nil
	}
	changed := func(id string) bool {
		if !found {
			return true
		}
		for _, field := range row.definition.fields {
			if field.ID == id {
				return field.value(
					row.instruction.Raw,
				) != field.value(
					old.instruction.Raw,
				)
			}
		}
		return false
	}
	values := make(map[string]int64)
	for _, field := range row.definition.fields {
		values[field.ID] = field.value(row.instruction.Raw)
	}
	if changed(ScriptWMDuration) && values[ScriptWMDuration] == 0 {
		return edit.ErrValue
	}
	if !changed(ScriptWMFromNode) && !changed(ScriptWMToNode) {
		return nil
	}
	if s.source.WorldMap == nil {
		return edit.ErrLayout
	}
	routes, err := s.source.WorldMap.Routes()
	if err != nil {
		return err
	}
	from, to := int(values[ScriptWMFromNode]), int(values[ScriptWMToNode])
	for _, route := range routes {
		if route.Nodes == [2]int{from, to} || route.Nodes == [2]int{to, from} {
			return route.MovementErr
		}
	}
	return worldmap.ErrConnections
}
