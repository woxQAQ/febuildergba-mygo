package game

import "image"

// Only a straight-line prefix with a known chapter context is inspected.
// Calls, branches, ASM and unknown instructions require runtime context and
// are reported as incomplete, rather than assigning their coordinates to a map.
func (s *Session) mapScriptImpacts(
	format gameFormat,
	chapter int,
	report *mapImpactReport,
) {
	entries, err := s.ScriptEntries(chapter)
	if err != nil {
		report.warn(chapter, 0, MapScriptPosition, err)
		return
	}
	seen := make(map[int]bool)
	for _, entry := range entries {
		if entry.Err != nil {
			report.warn(chapter, entry.Pointer, MapScriptPosition, entry.Err)
			continue
		}
		if entry.Address == 0 || seen[entry.Address] {
			continue
		}
		if entry.Source == "script_call_entry" {
			report.warn(
				chapter,
				entry.Pointer,
				MapScriptPosition,
				ErrMapImpactContext,
			)
			continue
		}
		seen[entry.Address] = true
		program, err := s.Script(chapter, entry.Pointer)
		if err != nil {
			report.warn(chapter, entry.Pointer, MapScriptPosition, err)
			continue
		}
		s.mapScriptPrefix(format, program, report)
	}
}

func (s *Session) mapScriptPrefix(
	format gameFormat,
	program Script,
	report *mapImpactReport,
) {
	entry := program.Entry
	if !program.Complete {
		report.warn(
			entry.Chapter,
			entry.Address,
			MapScriptPosition,
			ErrMapImpactContext,
		)
	}
	scanner := deploymentScanner{format: format.deployments, data: s.image.Data}
	for index, row := range program.rows {
		if report.full() {
			report.warn(
				entry.Chapter,
				row.instruction.Address,
				MapScriptPosition,
				ErrMapImpactLimit,
			)
			return
		}
		command, valid := scanner.instruction(row.instruction.Address)
		if valid &&
			(command.chapterEnd || (command.mapID >= 0 && command.mapID != entry.Chapter)) {
			return
		}
		if !valid || row.definition == nil || row.call >= 0 || row.branch >= 0 {
			report.warn(
				entry.Chapter,
				row.instruction.Address,
				MapScriptPosition,
				ErrMapImpactContext,
			)
			return
		}
		s.mapScriptCoordinates(entry, row, index, report)
	}
}

func (s *Session) mapScriptCoordinates(
	entry ScriptEntry,
	row scriptRow,
	index int,
	report *mapImpactReport,
) {
	var coordinates bool
	switch row.instruction.Op {
	case ScriptMoveUnit,
		ScriptMoveUnitSpeed,
		ScriptMovePositionPath,
		ScriptMoveFactionPositionPath,
		ScriptCameraPosition,
		ScriptCameraCenterPosition,
		ScriptCursorPosition,
		ScriptCursorGlowPosition,
		ScriptCursorFlashPosition:
		coordinates = true
	case ScriptMoveUnitPath, ScriptMoveFactionPaths:
		report.warn(
			entry.Chapter,
			row.instruction.Address,
			MapScriptPath,
			ErrMapImpactContext,
		)
	}
	if !coordinates {
		return
	}
	x, y := int64(-1), int64(-1)
	for _, field := range row.definition.fields {
		switch field.ID {
		case ScriptX:
			x = field.value(row.instruction.Raw)
		case ScriptY:
			y = field.value(row.instruction.Raw)
		}
	}
	if x < 0 || y < 0 ||
		(row.instruction.Op == ScriptMoveFactionPositionPath && x == 99) {
		report.warn(
			entry.Chapter,
			row.instruction.Address,
			MapScriptPosition,
			ErrMapImpactContext,
		)
		return
	}
	point := image.Pt(int(x), int(y))
	report.point(
		MapScriptPosition,
		entry.Chapter,
		entry.Pointer,
		row.instruction.Address,
		index,
		-1,
		point,
	)
	if row.instruction.Op == ScriptMoveFactionPositionPath {
		report.warn(
			entry.Chapter,
			row.instruction.Address,
			MapScriptPath,
			ErrMapImpactContext,
		)
	}
	if row.instruction.Op != ScriptMovePositionPath {
		return
	}
	for _, field := range row.definition.fields {
		if field.Target != ScriptMovementTarget {
			continue
		}
		value := field.value(row.instruction.Raw)
		if value < 0x080000C0 || value >= 0x0A000000 {
			report.warn(
				entry.Chapter,
				row.instruction.Address,
				MapScriptPath,
				ErrMapImpactContext,
			)
			return
		}
		steps, err := readScriptMovement(s.image.Data, int(value)-0x08000000)
		if err != nil {
			report.warn(
				entry.Chapter,
				row.instruction.Address,
				MapScriptPath,
				err,
			)
			return
		}
		report.scriptPath(entry, row.instruction.Address, index, point, steps)
	}
}

func (r *mapImpactReport) scriptPath(
	entry ScriptEntry,
	address, index int,
	point image.Point,
	steps []ScriptMovementStep,
) {
	for step, command := range steps {
		switch command.Command {
		case 0:
			point.X--
		case 1:
			point.X++
		case 2:
			point.Y++
		case 3:
			point.Y--
		case 4:
			return
		case 12:
			// The native destination scanner can reinterpret a low speed operand
			// as a command. Its path cannot be summarized by decoded steps alone.
			if command.Value < 16 {
				r.warn(
					entry.Chapter,
					address,
					MapScriptPath,
					ErrMapImpactContext,
				)
				return
			}
			continue
		default:
			continue
		}
		r.point(
			MapScriptPath,
			entry.Chapter,
			entry.Pointer,
			address,
			index,
			step,
			point,
		)
	}
}
