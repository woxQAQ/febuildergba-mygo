package game

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/japanese"
)

// textUnit keeps control arguments atomic while allowing adjacent byte-sized
// characters to share a Huffman leaf.
type textUnit struct {
	symbol  uint16
	pair    bool
	control bool
}

// TextValue retains native units alongside readable commands and raw markers.
// Newlines are literal; literal brackets and backslashes are escaped.
type TextValue struct {
	ID             uint16
	Content        string
	Raw            string
	Editable       bool
	ReadOnlyReason error
	units          []textUnit
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

func (f textFormat) textValue(units []textUnit, reason error) TextValue {
	content, valid := f.renderText(units, true)
	raw, _ := f.renderText(units, false)
	if !valid {
		reason = ErrTextEncoding
	}
	return TextValue{Content: content, Raw: raw, Editable: reason == nil, ReadOnlyReason: reason, units: units}
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
	var reason error
	var pending uint16
	for i := 0; i < len(stream.words); i++ {
		word := stream.words[i]
		if pending != 0 {
			argument, last, complete := stream.argument(i, pending)
			units = append(units, textUnit{symbol: argument, control: true})
			if !complete || argument&0xFF == 0 {
				reason = ErrTextArgument
			} else if pending == 0x80 {
				if _, ok := f.command(argument, true); !ok {
					reason = ErrTextControl
				}
			}
			i, pending = last, 0
			continue
		}
		if glyph, last, valid := f.descriptionGlyph(stream, i); valid {
			units = append(units, textUnit{symbol: glyph, pair: true})
			i = last
			continue
		}
		lo, hi := word&0xFF, word>>8
		if (lo == 0x80 || lo == 0x10) && hi != 0 {
			if lo == 0x80 {
				units = append(units, textUnit{symbol: lo, control: true}, textUnit{symbol: hi, control: true})
				if _, ok := f.command(hi, true); !ok {
					reason = ErrTextControl
				}
			} else {
				units = append(units, textUnit{symbol: word, control: true})
				reason = ErrTextArgument
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
			} else if control {
				if _, ok := f.command(code, false); !ok {
					reason = ErrTextControl
				}
			}
		}
	}
	if pending != 0 {
		reason = ErrTextArgument
	}
	return f.textValue(units, reason)
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
		switch b := content[i]; {
		case b == '[':
			if err := flush(); err != nil {
				return TextValue{}, err
			}
			next, nextPending, consumed, err := f.marker(content[i:], pending)
			if err != nil {
				return TextValue{}, err
			}
			if err := appendUnits(next...); err != nil {
				return TextValue{}, err
			}
			pending = nextPending
			i += consumed
		case pending != 0:
			return TextValue{}, ErrTextArgument
		case b == '\\':
			if i+1 >= len(content) || (content[i+1] != '[' && content[i+1] != '\\') {
				return TextValue{}, ErrTextEncoding
			}
			span.WriteByte(content[i+1])
			i += 2
		case b == '\n', b == '\r' && i+1 < len(content) && content[i+1] == '\n':
			if err := flush(); err != nil {
				return TextValue{}, err
			}
			if err := appendUnits(textUnit{symbol: 1, pair: true}); err != nil {
				return TextValue{}, err
			}
			if b == '\r' {
				i++
			}
			i++
		case b >= 0x20 && b != 0x7F:
			span.WriteByte(b)
			i++
		default:
			return TextValue{}, ErrTextEncoding
		}
	}
	if err := flush(); err != nil {
		return TextValue{}, err
	}
	if pending != 0 {
		return TextValue{}, ErrTextArgument
	}
	return f.textValue(units, nil), nil
}
