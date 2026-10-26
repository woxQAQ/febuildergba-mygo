package game

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/japanese"
)

type textFormat struct {
	pointer, recovery int
	shiftJIS          bool
	names             *strings.Replacer
}

// readName preserves the ROM encoding; aliases affect display and search only.
func (f textFormat) readName(data []byte, id uint16, aliases map[byte]string) (string, error) {
	if id == 0 {
		return "", nil
	}
	if id >= 0x7fff {
		return "", ErrText
	}
	table, ok := pointer(data, f.pointer)
	if !ok {
		table = f.recovery
	}
	entry := table + int(id)*4
	if !within(data, entry, 4) {
		return "", ErrText
	}
	p := u32(data, entry)
	if p == 0 {
		return "", nil
	}
	if p >= 0x88000000 && p < 0x8A000000 {
		at := int(p - 0x88000000)
		var out []byte
		for i := 0; i < 4096 && within(data, at+i, 1); i++ {
			b := data[at+i]
			if b == 0 {
				return f.displayText(out, aliases), nil
			}
			out = append(out, b)
		}
		return "", ErrText
	}
	at, ok := pointer(data, entry)
	if !ok {
		return "", ErrText
	}
	tree, ok := pointer(data, 0x6E0)
	if !ok {
		return "", ErrText
	}
	rootPointer, ok := pointer(data, 0x6DC)
	if !ok {
		return "", ErrText
	}
	root, ok := pointer(data, rootPointer)
	if !ok {
		return "", ErrText
	}
	node := root
	var out []byte
	// A bound also terminates cyclic trees and streams without an end symbol.
	for bit := range 65536 {
		if !within(data, at+bit/8, 1) {
			return "", ErrText
		}
		branch := int((data[at+bit/8] >> (bit % 8)) & 1)
		if !within(data, node+branch*2, 2) {
			return "", ErrText
		}
		node = tree + int(u16(data, node+branch*2))*4
		if !within(data, node, 4) {
			return "", ErrText
		}
		word := u32(data, node)
		if word&0x80000000 == 0 {
			continue
		}
		lo, hi := byte(word&0xFF), byte((word>>8)&0xFF)
		if lo == 0 {
			return f.displayText(out, aliases), nil
		}
		out = append(out, lo)
		if hi != 0 {
			out = append(out, hi)
		}
		if len(out) > 4096 {
			return "", ErrText
		}
		node = root
	}
	return "", ErrText
}

func (f textFormat) displayText(data []byte, aliases map[byte]string) string {
	var out strings.Builder
	for i := 0; i < len(data); {
		b := data[i]
		// TextForm.Direct removes the FE name-padding control before display.
		if b == 0x1F {
			i++
			continue
		}
		text, alias := aliases[b]
		switch {
		case alias:
			out.WriteString(text)
		case b >= 32 && b < 127:
			out.WriteByte(b)
		case f.shiftJIS && ((b >= 0xA1 && b <= 0xDF) || shiftJISPair(data[i:])):
			width := 1
			if shiftJISPair(data[i:]) {
				width = 2
			}
			encoded := data[i : i+width]
			text, err := japanese.ShiftJIS.NewDecoder().String(string(encoded))
			if err == nil && !strings.ContainsRune(text, utf8.RuneError) {
				out.WriteString(text)
			} else {
				fmt.Fprintf(&out, "[0x%X]", encoded)
			}
			i += width
			continue
		default:
			fmt.Fprintf(&out, "[0x%02X]", b)
		}
		i++
	}
	name := out.String()
	if f.names != nil {
		name = f.names.Replace(name)
	}
	return strings.TrimSpace(name)
}

func shiftJISPair(data []byte) bool {
	return len(data) >= 2 && ((data[0] >= 0x81 && data[0] <= 0x9F) || (data[0] >= 0xE0 && data[0] <= 0xFC)) &&
		data[1] >= 0x40 && data[1] <= 0xFC && data[1] != 0x7F
}

// These sequences denote compact class-name glyphs, not their Unicode symbols.
// Their names follow FETextEncode's multi-byte game mappings.
var fe8JNameSymbols = strings.NewReplacer(
	"нопр", "(ｻﾝﾀﾞｰｽﾄｰﾑ)",
	"⊂⊃┌┐", "(ﾌｫﾚｽﾄﾅｲﾄ)",
	"абвг", "(ﾄﾞﾗｺﾞﾝﾏｽﾀｰ)",
	"шщъы", "(ﾜｲﾊﾞｰﾝﾅｲﾄ)",
	"жзий", "(ﾌｧﾙｺﾝﾅｲﾄ)",
	"∞┴∠∧", "(ﾓｰｻﾄﾞｩｰｸﾞ)",
	"∇∈∋├", "(ﾃﾞｽｶﾞｰｺﾞｲﾙ)",
	"≠≡≦≧", "(ﾄﾞﾗｺﾞﾝｿﾞﾝﾋﾞ)",
	"∪∫∬∴", "(ｺﾞｰｺﾞﾝの卵)",
)

var fe6NameSymbols = strings.NewReplacer(
	"①②③④⑤⑥⑦", "(ｻﾝﾀﾞｰｽﾄｰﾑ)",
	"ⅠⅡⅢⅣⅤⅥⅦ", "(ﾌｧﾙｺﾝﾅｲﾄ)",
	"⑪⑫⑬⑭⑮⑯⑰", "(ﾄﾞﾗｺﾞﾝﾏｽﾀｰ)",
)
