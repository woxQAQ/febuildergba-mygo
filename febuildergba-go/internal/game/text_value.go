package game

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/japanese"
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

func (f textFormat) decodeCharacters(raw []byte) (string, bool) {
	content := string(raw)
	if f.shiftJIS {
		decoded, err := japanese.ShiftJIS.NewDecoder().String(content)
		if err != nil || strings.ContainsRune(decoded, utf8.RuneError) {
			return "", false
		}
		content = decoded
	}
	if f.names != nil {
		content = f.names.display.Replace(content)
	}
	return content, true
}

var escapeText = strings.NewReplacer("\\", "\\\\", "[", "\\[")

func (f textFormat) textValue(units []textUnit, editable bool) TextValue {
	var out strings.Builder
	var span []textUnit
	flush := func() {
		var raw []byte
		for _, unit := range span {
			raw = append(raw, byte(unit.symbol&0xFF))
			if unit.symbol > 0xFF {
				raw = append(raw, byte(unit.symbol>>8))
			}
		}
		content, valid := f.decodeCharacters(raw)
		if valid {
			out.WriteString(escapeText.Replace(content))
		} else {
			editable = false
			for _, unit := range span {
				fmt.Fprintf(&out, "[0x%04X]", unit.symbol)
			}
		}
		span = nil
	}
	for _, unit := range units {
		if !unit.control && unit.symbol != 1 {
			span = append(span, unit)
			continue
		}
		flush()
		if unit.control {
			fmt.Fprintf(&out, "[0x%04X]", unit.symbol)
		} else {
			out.WriteByte('\n')
		}
	}
	flush()
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

// descriptionGlyph recognizes characters before their trail bytes can be
// mistaken for commands. Command arguments are handled before this lookup.
func (f textFormat) descriptionGlyph(stream textStream, at int) (uint16, int, bool) {
	if !f.shiftJIS {
		return 0, at, false
	}
	word := stream.words[at]
	lo, hi := word&0xFF, word>>8
	if hi == 0 && at+1 < len(stream.words) && stream.words[at+1] <= 0xFF &&
		shiftJISPair([]byte{byte(lo), byte(stream.words[at+1])}) {
		word |= stream.words[at+1] << 8
		hi = word >> 8
		at++
	}
	return word, at, shiftJISPair([]byte{byte(lo), byte(hi)})
}

func (f textFormat) descriptionValue(stream textStream) TextValue {
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
		if glyph, last, valid := f.descriptionGlyph(stream, i); valid {
			units = append(units, textUnit{symbol: glyph, pair: true})
			i = last
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
			halfWidth := f.shiftJIS && code >= 0xA1 && code <= 0xDF
			control := code != 1 && (code < 0x20 || (code >= 0x7F && !halfWidth))
			units = append(units, textUnit{symbol: code, pair: true, control: control})
			if code == 0x10 || code == 0x80 {
				pending = code
			}
			if code == 0x1E || (f.shiftJIS && ((code >= 0x81 && code <= 0x9F) || (code >= 0xE0 && code <= 0xFC))) {
				editable = false
			}
		}
	}
	return f.textValue(units, editable && pending == 0)
}

func parseTextControl(content string, pending uint16) (textUnit, uint16, error) {
	if len(content) < 8 || content[:3] != "[0x" || content[7] != ']' {
		return textUnit{}, 0, ErrTextEncoding
	}
	code, err := strconv.ParseUint(content[3:7], 16, 16)
	if err != nil || code&0xFF == 0 {
		return textUnit{}, 0, ErrTextEncoding
	}
	unit := textUnit{symbol: uint16(code), control: pending != 0 || code != 1, pair: pending == 0 && code <= 0xFF}
	if pending != 0 {
		if pending == 0x80 && !standardExtended(unit.symbol) {
			return textUnit{}, 0, ErrTextEncoding
		}
		return unit, 0, nil
	}
	if code == 0x10 || code == 0x80 {
		return unit, unit.symbol, nil
	}
	return unit, 0, nil
}

func (f textFormat) parseDescription(content string) (TextValue, error) {
	if !utf8.ValidString(content) {
		return TextValue{}, ErrTextEncoding
	}
	var units []textUnit
	var span strings.Builder
	var pending uint16
	size := 0
	appendUnits := func(next ...textUnit) error {
		for _, unit := range next {
			size++
			if unit.symbol > 0xFF {
				size++
			}
		}
		if size > 4096 {
			return ErrTextEncoding
		}
		units = append(units, next...)
		return nil
	}
	flush := func() error {
		characters, err := f.characterUnits(span.String())
		if err != nil {
			return err
		}
		span.Reset()
		return appendUnits(characters...)
	}
	for i := 0; i < len(content); {
		unit := textUnit{pair: true}
		switch b := content[i]; {
		case b == '[':
			if err := flush(); err != nil {
				return TextValue{}, err
			}
			next, nextPending, err := parseTextControl(content[i:], pending)
			if err != nil {
				return TextValue{}, err
			}
			unit, pending = next, nextPending
			i += 8
		case pending != 0:
			return TextValue{}, ErrTextEncoding
		case b == '\\':
			if i+1 >= len(content) || (content[i+1] != '[' && content[i+1] != '\\') {
				return TextValue{}, ErrTextEncoding
			}
			span.WriteByte(content[i+1])
			i += 2
			continue
		case b == '\n', b == '\r' && i+1 < len(content) && content[i+1] == '\n':
			if err := flush(); err != nil {
				return TextValue{}, err
			}
			unit.symbol = 1
			if b == '\r' {
				i++
			}
			i++
		case b >= 0x20 && b != 0x7F:
			span.WriteByte(b)
			i++
			continue
		default:
			return TextValue{}, ErrTextEncoding
		}
		if err := appendUnits(unit); err != nil {
			return TextValue{}, err
		}
	}
	if err := flush(); err != nil {
		return TextValue{}, err
	}
	if pending != 0 {
		return TextValue{}, ErrTextEncoding
	}
	return f.textValue(units, true), nil
}
