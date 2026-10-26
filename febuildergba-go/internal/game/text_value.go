package game

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// textUnit keeps control arguments atomic while allowing adjacent byte-sized
// characters to share a Huffman leaf. Control markers survive text edits.
type textUnit struct {
	symbol  uint16
	pair    bool
	control bool
}

// TextValue is a lossless editable representation of a description. Newlines
// are literal; other controls use [0xNNNN]. Literal brackets and backslashes
// are escaped so that text cannot accidentally become a control marker.
type TextValue struct {
	ID       uint16
	Content  string
	Editable bool
	units    []textUnit
}

func (v TextValue) ControlCodes() []uint16 {
	var codes []uint16
	for _, unit := range v.units {
		if unit.control {
			codes = append(codes, unit.symbol)
		}
	}
	return codes
}

func textValue(units []textUnit, editable bool) TextValue {
	var out strings.Builder
	for _, unit := range units {
		switch {
		case unit.control:
			fmt.Fprintf(&out, "[0x%04X]", unit.symbol)
		case unit.symbol == 1:
			out.WriteByte('\n')
		default:
			if unit.symbol == '[' || unit.symbol == '\\' {
				out.WriteByte('\\')
			}
			out.WriteByte(byte(unit.symbol & 0xFF))
		}
	}
	return TextValue{Content: out.String(), Editable: editable, units: units}
}

// Standard extended commands follow text_escape_ALL and FETextDecode. Unknown
// extensions remain visible but are not assumed to have the same argument size.
func standardExtended(code uint16) bool {
	if code >= 0xA && code <= 0x11 {
		return true
	}
	switch code {
	case 4, 5, 0x16, 0x18, 0x19, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F, 0x20, 0x21, 0x22, 0x23, 0x24, 0x25:
		return true
	}
	return false
}

func (s textStream) argument(at int, prefix uint16) (uint16, int, bool) {
	word := s.words[at]
	if !s.plain || prefix != 0x10 {
		return word, at, true
	}
	if at+1 >= len(s.words) {
		return word, at, false
	}
	return word | s.words[at+1]<<8, at + 1, true
}

func descriptionValue(stream textStream) TextValue {
	var units []textUnit
	editable := true
	var pending uint16
	for i := 0; i < len(stream.words); i++ {
		word := stream.words[i]
		if pending != 0 {
			argument, last, complete := stream.argument(i, pending)
			units = append(units, textUnit{symbol: argument, control: true})
			editable = editable && complete && argument&0xFF != 0 && (pending != 0x80 || standardExtended(argument))
			i = last
			pending = 0
			continue
		}
		lo, hi := word&0xFF, word>>8
		if (lo == 0x80 || lo == 0x10) && hi != 0 {
			units = append(units, textUnit{symbol: word, control: true})
			if lo != 0x80 || !standardExtended(hi) {
				editable = false
			}
			continue
		}
		for _, code := range []uint16{lo, hi} {
			if code == 0 {
				continue
			}
			control := code != 1 && (code < 0x20 || code >= 0x7F)
			units = append(units, textUnit{symbol: code, pair: true, control: control})
			if code == 0x10 || code == 0x80 {
				pending = code
			}
			if code == 0x1E {
				editable = false
			}
		}
	}
	return textValue(units, editable && pending == 0)
}

func parseDescription(content string) (TextValue, error) {
	if !utf8.ValidString(content) {
		return TextValue{}, ErrTextEncoding
	}
	var units []textUnit
	var pending uint16
	size := 0
	for i := 0; i < len(content); {
		unit := textUnit{pair: true}
		switch b := content[i]; {
		case b == '[':
			if len(content)-i < 8 || content[i:i+3] != "[0x" || content[i+7] != ']' {
				return TextValue{}, ErrTextEncoding
			}
			code, err := strconv.ParseUint(content[i+3:i+7], 16, 16)
			if err != nil || code&0xFF == 0 {
				return TextValue{}, ErrTextEncoding
			}
			unit.symbol = uint16(code)
			unit.control = pending != 0 || code != 1
			unit.pair = pending == 0 && code <= 0xFF
			if pending != 0 {
				if pending == 0x80 && !standardExtended(unit.symbol) {
					return TextValue{}, ErrTextEncoding
				}
				pending = 0
			} else if code == 0x10 || code == 0x80 {
				pending = unit.symbol
			}
			i += 8
		case pending != 0:
			return TextValue{}, ErrTextEncoding
		case b == '\\':
			if i+1 >= len(content) || (content[i+1] != '[' && content[i+1] != '\\') {
				return TextValue{}, ErrTextEncoding
			}
			unit.symbol = uint16(content[i+1])
			i += 2
		case b == '\n':
			unit.symbol = 1
			i++
		case b == '\r' && i+1 < len(content) && content[i+1] == '\n':
			unit.symbol = 1
			i += 2
		case b >= 0x20 && b <= 0x7E:
			unit.symbol = uint16(b)
			i++
		default:
			return TextValue{}, ErrTextEncoding
		}
		size++
		if unit.symbol > 0xFF {
			size++
		}
		if size > 4096 {
			return TextValue{}, ErrTextEncoding
		}
		units = append(units, unit)
	}
	if pending != 0 {
		return TextValue{}, ErrTextEncoding
	}
	return textValue(units, true), nil
}
