package event

import (
	"bytes"
	"slices"
)

// scriptSet is the selected native instruction set. Editable operands and
// discovery share these definitions; opaque commands retain explicit lengths
// and control-flow facts without becoming editable.
type scriptSet struct {
	definitions           []scriptDefinition
	decode                func(*deploymentScanner, int, deploymentInstruction) (deploymentInstruction, bool)
	words                 [256]byte
	flow                  [256]wordFlow
	asmOffset             int
	asmOpcodes            []byte
	labelStop             []byte
	trackSlots            bool
	positionFallback      bool
	dialog, reinforcement []byte
	textOffset            int
}

// Offsets are relative to the instruction; zero means no operand. Native word
// commands place discovered resource operands after their opcode word.
type wordFlow struct {
	end, chapterEnd, jump      bool
	call, label, branch, mapID int
	loads, texts               []int
}

func instructionDefinitions(groups ...[]scriptDefinition) []scriptDefinition {
	var result []scriptDefinition
	for _, group := range groups {
		result = append(result, group...)
	}
	return result
}

func fe8Scripts() *scriptSet {
	result := &scriptSet{
		definitions: instructionDefinitions(
			fe8WorldMapDefinitions(),
			fe8WorldMapSceneDefinitions(),
			fe8SceneDefinitions(),
			fe8CameraDefinitions(),
			fe8UnitConditionDefinitions(),
			fe8LoadDefinitions(),
			fe8SlotDefinitions(),
			fe8ControlDefinitions(),
			fe8UnitStateDefinitions(),
			fe8PresentationDefinitions(),
			fe8MapChangeDefinitions(),
			fe8CoreDefinitions(),
		),
		decode:    (*deploymentScanner).sizedInstruction,
		asmOffset: 1, asmOpcodes: []byte{0x0D}, trackSlots: true, textOffset: 6,
	}
	for index := range result.definitions {
		for operand := range result.definitions[index].fields {
			field := &result.definitions[index].fields[operand]
			switch field.Target {
			case ScriptTextTarget:
				if result.definitions[index].op != ScriptWMText {
					field.indirect = &operandRange{0x8000, 0xFFFF}
				}
			case ScriptEventTarget:
				field.indirect = &operandRange{0xFFFFFFFF, 0xFFFFFFFF}
			case ScriptDeploymentTarget:
				field.indirect = &operandRange{0x80000000, 0xFFFFFFFF}
			}
		}
	}
	result.dialog = result.sequence(
		ScriptTextStart,
		ScriptText,
		ScriptTextWait,
		ScriptTextClear,
		ScriptEnd,
	)
	result.reinforcement = result.sequence(
		ScriptLoadUnits,
		ScriptWaitUnits,
		ScriptEnd,
	)
	return result
}

func fe6Scripts() *scriptSet {
	result := &scriptSet{
		definitions: instructionDefinitions(
			fe6WorldMapDefinitions(),
			fe6SceneDefinitions(),
			fe6CameraDefinitions(),
			wordUnitConditionDefinitions(0x1D),
			wordControlDefinitions(
				6,
			),
			fe6PartyDefinitions(),
			wordMovementDefinitions(0x10, 0x0E, 0),
			fe6ConditionDefinitions(),
			fe6MapChangeDefinitions(),
			fe6CoreDefinitions(),
		),
		decode: (*deploymentScanner).wordInstruction,
		words:  fe6InstructionWords, asmOpcodes: []byte{0x17, 0x4B},
		labelStop: scriptWords(0), positionFallback: true, textOffset: 4,
	}
	result.flow[0x3F].mapID = 4
	result.flow[0x3D].chapterEnd = true
	for opcode := 0x1C; opcode <= 0x24; opcode++ {
		result.flow[opcode].branch = 4
	}
	result.describeWords()
	return result
}

func fe7Scripts(
	shift, call uint32,
	worldMapBase byte,
	words [256]byte,
) *scriptSet {
	result := &scriptSet{
		definitions: instructionDefinitions(
			fe7WorldMapDefinitions(worldMapBase),
			fe7SceneDefinitions(
				shift,
			),
			fe7CameraDefinitions(0x1D+shift, 0x64+shift),
			wordUnitConditionDefinitions(0x44+shift),
			wordControlDefinitions(
				0x0A,
			),
			fe7RouteDefinitions(shift),
			wordMovementDefinitions(0x26+shift, 0x23+shift, 0x29+shift),
			fe7ConditionDefinitions(
				shift,
			),
			fe7MapChangeDefinitions(shift),
			fe7CoreDefinitions(shift, call),
		),
		decode: (*deploymentScanner).wordInstruction,
		words:  words,
		// The second opcode installs an ASM callback for event skipping.
		asmOpcodes: []byte{
			byte(0x3C + shift),
			byte(0x97 + shift),
		}, // #nosec G115 -- Native opcode variants fit in one byte.
		labelStop:        scriptWords(0),
		positionFallback: true,
		textOffset:       4,
	}
	result.flow[0x1B+shift].mapID = 4
	for opcode := 0x43 + shift; opcode <= 0x55+shift; opcode++ {
		result.flow[opcode].branch = 4
	}
	result.describeWords()
	return result
}

func (set *scriptSet) describeWords() {
	for _, definition := range set.definitions {
		rule := &set.flow[definition.raw[0]]
		// Variants of one opcode have the same resource positions. Replace the
		// lists instead of accumulating duplicate sites for each subcommand.
		rule.loads, rule.texts = nil, nil
		rule.end = definition.op == ScriptEnd ||
			definition.op == ScriptEarlyReturn
		rule.jump = definition.op == ScriptJump
		for _, field := range definition.fields {
			if field.ID == ScriptLabelID {
				if definition.label {
					rule.label = field.offset
				}
				if definition.branch {
					rule.branch = field.offset
				}
			}
			switch field.Target {
			case ScriptEventTarget:
				rule.call = field.offset
			case ScriptDeploymentTarget:
				rule.loads = append(rule.loads, field.offset)
			case ScriptTextTarget:
				rule.texts = append(rule.texts, field.offset)
			}
		}
	}
	set.dialog = set.sequence(
		ScriptText,
		ScriptTextClear,
		ScriptClearSkip,
		ScriptEnd,
	)
	set.reinforcement = set.sequence(
		ScriptLoadUnits,
		ScriptWaitUnits,
		ScriptClearSkip,
		ScriptEnd,
	)
}

func (set *scriptSet) sequence(ops ...ScriptOp) []byte {
	var result []byte
	for _, op := range ops {
		for _, definition := range set.definitions {
			if definition.op == op {
				result = append(result, definition.raw...)
				break
			}
		}
	}
	return result
}

func (f Format) labelSearchEnd(raw []byte) bool {
	return len(f.scripts.labelStop) != 0 &&
		bytes.Equal(raw, f.scripts.labelStop)
}

func (f Format) scriptASM(raw []byte) bool {
	return len(raw) == 8 &&
		slices.Contains(f.scripts.asmOpcodes, raw[f.scripts.asmOffset])
}
