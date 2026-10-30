package text

import (
	"fmt"
	"strconv"
	"strings"
)

// textCommand describes one complete native instruction. Parameters stay
// separate from printable characters, even when a Huffman leaf packs bytes.
type textCommand struct {
	name     string
	code     uint16
	extended bool
	english  bool
}

// Names follow TextEscape's common FEditor notation. The shared table applies
// to FE6/7/8; opening and closing quotes are English font codes.
var textCommands = []textCommand{
	{"NL", 1, false, false},
	{"Clear", 2, false, false},
	{"A", 3, false, false},
	{"....", 4, false, false},
	{".....", 5, false, false},
	{"......", 6, false, false},
	{".......", 7, false, false},
	{"OpenFarLeft", 8, false, false},
	{"OpenMidLeft", 9, false, false},
	{"OpenLeft", 0xA, false, false},
	{"OpenRight", 0xB, false, false},
	{"OpenMidRight", 0xC, false, false},
	{"OpenFarRight", 0xD, false, false},
	{"OpenFarFarLeft", 0xE, false, false},
	{"OpenFarFarRight", 0xF, false, false},
	{"LoadFace", 0x10, false, false},
	{"ClearFace", 0x11, false, false},
	{"NormalPrint", 0x12, false, false},
	{"FastPrint", 0x13, false, false},
	{"CloseSpeechFast", 0x14, false, false},
	{"CloseSpeechSlow", 0x15, false, false},
	{"ToggleMouthMove", 0x16, false, false},
	{"ToggleSmile", 0x17, false, false},
	{"Yes", 0x18, false, false},
	{"No", 0x19, false, false},
	{"Buy/Sell", 0x1A, false, false},
	{"ShopContinue", 0x1B, false, false},
	{"SendToBack", 0x1C, false, false},
	{"FastPrint2", 0x1D, false, false},
	{".", 0x1F, false, false},
	{"OpenQuote", 0x93, false, true},
	{"CloseQuote", 0x94, false, true},
	{"LoadOverworldFaces", 4, true, false},
	{"G", 5, true, false},
	{"MoveFarLeft", 0xA, true, false},
	{"MoveMidLeft", 0xB, true, false},
	{"MoveLeft", 0xC, true, false},
	{"MoveRight", 0xD, true, false},
	{"MoveMidRight", 0xE, true, false},
	{"MoveFarRight", 0xF, true, false},
	{"MoveFarFarLeft", 0x10, true, false},
	{"MoveFarFarRight", 0x11, true, false},
	{"EnableBlinking", 0x16, true, false},
	{"DelayBlinking", 0x18, true, false},
	{"PauseBlinking", 0x19, true, false},
	{"DisableBlinking", 0x1B, true, false},
	{"OpenEyes", 0x1C, true, false},
	{"CloseEyes", 0x1D, true, false},
	{"HalfCloseEyes", 0x1E, true, false},
	{"Wink", 0x1F, true, false},
	{"Tact", 0x20, true, false},
	{"ToggleRed", 0x21, true, false},
	{"Item", 0x22, true, false},
	{"SetName", 0x23, true, false},
	{"Tact 2", 0x24, true, false},
	{"ToggleColorInvert", 0x25, true, false},
}

func (g Format) Commands() []string {
	var markers []string
	for _, command := range textCommands {
		if command.english && g.ShiftJIS {
			continue
		}
		marker := "[" + command.name + "]"
		if command.code == 0x10 && !command.extended {
			marker += "[0x0101]"
		}
		markers = append(markers, marker)
	}
	return markers
}

func (g Format) command(code uint16, extended bool) (textCommand, bool) {
	for _, command := range textCommands {
		if command.code == code && command.extended == extended &&
			(!command.english || !g.ShiftJIS) {
			return command, true
		}
	}
	return textCommand{}, false
}

func (g Format) marker(
	content string,
	pending uint16,
) ([]textUnit, uint16, int, error) {
	end := strings.IndexByte(content, ']')
	if end < 0 {
		return nil, 0, 0, ErrTextEncoding
	}
	name := content[1:end]
	if strings.HasPrefix(name, "0x") {
		if len(name) != 6 {
			return nil, 0, 0, ErrTextEncoding
		}
		code, err := strconv.ParseUint(name[2:], 16, 16)
		if err != nil {
			return nil, 0, 0, ErrTextEncoding
		}
		units, next, err := g.rawControl(uint16(code), pending)
		return units, next, end + 1, err
	}
	if pending != 0 {
		return nil, 0, 0, ErrTextArgument
	}
	if name == "2NL" {
		name = "Clear"
	}
	if name == "@" {
		return []textUnit{{symbol: '@', pair: true}}, 0, end + 1, nil
	}
	for _, command := range textCommands {
		if command.name == name && (!command.english || !g.ShiftJIS) {
			if command.extended {
				return []textUnit{
					{symbol: 0x80, pair: true, control: true},
					{symbol: command.code, control: true},
				}, 0, end + 1, nil
			}
			units, next, err := g.rawControl(command.code, 0)
			return units, next, end + 1, err
		}
	}
	return nil, 0, 0, ErrTextControl
}

func (g Format) rawControl(
	code, pending uint16,
) ([]textUnit, uint16, error) {
	if pending != 0 {
		if code&0xFF == 0 {
			return nil, 0, ErrTextArgument
		}
		if pending == 0x80 {
			if _, ok := g.command(code, true); !ok {
				return nil, 0, ErrTextControl
			}
		}
		return []textUnit{{symbol: code, control: true}}, 0, nil
	}
	if code == 0x10 || code == 0x80 {
		return []textUnit{{symbol: code, pair: true, control: true}}, code, nil
	}
	if code&0xFF == 0x80 && code > 0xFF {
		if _, ok := g.command(code>>8, true); ok {
			return []textUnit{
				{symbol: 0x80, pair: true, control: true},
				{symbol: code >> 8, control: true},
			}, 0, nil
		}
	}
	if _, ok := g.command(code, false); !ok {
		return nil, 0, ErrTextControl
	}
	return []textUnit{{symbol: code, pair: true, control: code != 1}}, 0, nil
}

func (g Format) readableControl(units []textUnit) (string, int) {
	code := units[0].symbol
	if code == 0x80 && len(units) > 1 {
		if command, ok := g.command(units[1].symbol, true); ok {
			return "[" + command.name + "]", 2
		}
	}
	command, ok := g.command(code, false)
	if !ok {
		return fmt.Sprintf("[0x%04X]", code), 1
	}
	if code == 0x10 && len(units) > 1 {
		return fmt.Sprintf("[%s][0x%04X]", command.name, units[1].symbol), 2
	}
	return "[" + command.name + "]", 1
}

// renderText retains native control units rather than flattening arguments
// into printable bytes. Raw mode keeps each portrait argument 16 bits wide.
func (g Format) renderText(units []textUnit, readable bool) (string, bool) {
	var out strings.Builder
	var span []byte
	valid := true
	flush := func() {
		content, ok := g.decodeCharacters(span)
		if ok {
			out.WriteString(escapeText.Replace(content))
		} else {
			valid = false
			for _, b := range span {
				fmt.Fprintf(&out, "[0x%04X]", b)
			}
		}
		span = nil
	}
	for i := 0; i < len(units); i++ {
		unit := units[i]
		if !unit.control && unit.symbol != 1 {
			span = append(span, byte(unit.symbol&0xFF))
			if unit.symbol > 0xFF {
				span = append(span, byte(unit.symbol>>8))
			}
			continue
		}
		flush()
		if !unit.control {
			out.WriteByte('\n')
			continue
		}
		if readable {
			marker, consumed := g.readableControl(units[i:])
			out.WriteString(marker)
			i += consumed - 1
		} else {
			fmt.Fprintf(&out, "[0x%04X]", unit.symbol)
		}
	}
	flush()
	return out.String(), valid
}
