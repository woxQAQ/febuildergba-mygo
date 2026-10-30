package event

import (
	"image"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
)

// MapTrace describes native map operands and the context available at each
// instruction. Resource lookup belongs to the chapter coordinator.
type MapTrace struct {
	Address, Instruction, Chapter         int
	IsTarget, Uncertain, RuntimePositions bool
	Targets                               []maps.Target
	Err                                   error
}

func (p Script) MapTrace() []MapTrace {
	states := mapOperandStates(p.Entry, p.format, p.rows)
	result := make([]MapTrace, len(p.rows))
	for index, row := range p.rows {
		step := MapTrace{
			Address:     row.instruction.Address,
			Instruction: index,
			Chapter:     states[index].chapter,
			IsTarget:    mapChangeOperation(row.instruction.Op),
			Uncertain: row.definition == nil || row.call >= 0 ||
				row.branch >= 0,
			RuntimePositions: p.format.kind != eventFE8,
		}
		if step.IsTarget {
			step.Targets, step.Err = mapOperands(
				row,
				states[index],
				p.rows,
				p.format,
			)
		}
		result[index] = step
	}
	return result
}

// Coordinate is a confirmed position or an uncertainty encountered while
// decoding one instruction. Path positions retain native movement-step indexes.
type Coordinate struct {
	Position image.Point
	Step     int
	Path     bool
	Err      error
}

type CoordinateTrace struct {
	Address, Instruction int
	Coordinates          []Coordinate
}

// Coordinates interprets only the straight-line prefix in the entry's known
// chapter. It stops at map changes, calls, branches or unknown instructions.
func (p Script) Coordinates(data []byte) []CoordinateTrace {
	scanner := deploymentScanner{format: p.format, data: data}
	var result []CoordinateTrace
	for index, row := range p.rows {
		command, valid := scanner.instruction(row.instruction.Address)
		if valid &&
			(command.chapterEnd || (command.mapID >= 0 && command.mapID != p.Entry.Chapter)) {
			break
		}
		step := CoordinateTrace{
			Address:     row.instruction.Address,
			Instruction: index,
		}
		if !valid || row.definition == nil || row.call >= 0 || row.branch >= 0 {
			step.Coordinates = []Coordinate{{Err: ErrMapImpactContext}}
			result = append(result, step)
			break
		}
		step.Coordinates = scriptCoordinates(data, row)
		result = append(result, step)
	}
	return result
}

func scriptCoordinates(data []byte, row scriptRow) []Coordinate {
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
		return []Coordinate{{Path: true, Err: ErrMapImpactContext}}
	}
	if !coordinates {
		return nil
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
		return []Coordinate{{Err: ErrMapImpactContext}}
	}
	point := image.Pt(int(x), int(y))
	result := []Coordinate{{Position: point, Step: -1}}
	if row.instruction.Op == ScriptMoveFactionPositionPath {
		result = append(
			result,
			Coordinate{Path: true, Err: ErrMapImpactContext},
		)
	}
	if row.instruction.Op != ScriptMovePositionPath {
		return result
	}
	for _, field := range row.definition.fields {
		if field.Target != ScriptMovementTarget {
			continue
		}
		value := field.value(row.instruction.Raw)
		if value < 0x080000C0 || value >= 0x0A000000 {
			return append(
				result,
				Coordinate{Path: true, Err: ErrMapImpactContext},
			)
		}
		steps, err := readScriptMovement(data, int(value)-0x08000000)
		if err != nil {
			return append(result, Coordinate{Path: true, Err: err})
		}
		result = append(result, scriptPathCoordinates(point, steps)...)
	}
	return result
}

func scriptPathCoordinates(
	point image.Point,
	steps []ScriptMovementStep,
) []Coordinate {
	var result []Coordinate
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
			return result
		case 12:
			// The native destination scanner can reinterpret a low speed operand as
			// another command; decoded movement steps cannot establish that path.
			if command.Value < 16 {
				return append(
					result,
					Coordinate{Path: true, Err: ErrMapImpactContext},
				)
			}
			continue
		default:
			continue
		}
		result = append(
			result,
			Coordinate{Position: point, Step: step, Path: true},
		)
	}
	return result
}

var ErrMapImpactContext = fault.Code{
	ID:   "game.map_impact_context",
	Kind: fault.Unsupported,
}
