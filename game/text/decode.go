package text

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/japanese"

	"github.com/woxQAQ/febuildergba-mygo/gba"
)

// textStream retains Huffman leaf boundaries, including 16-bit control
// arguments.
type textStream struct {
	words []uint16
	plain bool
}

// readName preserves the ROM encoding; aliases affect display and search only.
func (f Format) ReadName(
	data []byte,
	id uint16,
	aliases map[byte]string,
) (string, error) {
	stream, err := f.readStream(data, id)
	if err != nil {
		return "", err
	}
	var raw []byte
	for _, word := range stream.words {
		raw = append(raw, byte(word&0xFF))
		if word>>8 != 0 {
			raw = append(raw, byte(word>>8))
		}
	}
	return f.displayText(raw, aliases), nil
}

func (f Format) readStream(data []byte, id uint16) (textStream, error) {
	if id == 0 {
		return textStream{}, nil
	}
	if id >= 0x7fff {
		return textStream{}, ErrText
	}
	table, ok := gba.Pointer(data, f.Pointer)
	if !ok {
		table = f.Recovery
	}
	entry := table + int(id)*4
	if !gba.Within(data, entry, 4) {
		return textStream{}, ErrText
	}
	p := gba.Uint32(data, entry)
	if p == 0 {
		return textStream{}, nil
	}
	if p >= 0x88000000 && p < 0x8A000000 {
		at := int(p - 0x88000000)
		var out []uint16
		for i := 0; i < 4096 && gba.Within(data, at+i, 1); i++ {
			b := data[at+i]
			if b == 0 {
				return textStream{words: out, plain: true}, nil
			}
			out = append(out, uint16(b))
		}
		return textStream{}, ErrText
	}
	at, ok := gba.Pointer(data, entry)
	if !ok {
		return textStream{}, ErrText
	}
	tree, ok := gba.Pointer(data, f.TreePointer)
	if !ok {
		return textStream{}, ErrText
	}
	rootPointer, ok := gba.Pointer(data, f.RootPointer)
	if !ok {
		return textStream{}, ErrText
	}
	root, ok := gba.Pointer(data, rootPointer)
	if !ok {
		return textStream{}, ErrText
	}
	node := root
	size := 0
	var out []uint16
	// A bound also terminates cyclic trees and streams without an end symbol.
	for bit := range 65536 {
		if !gba.Within(data, at+bit/8, 1) {
			return textStream{}, ErrText
		}
		branch := int((data[at+bit/8] >> (bit % 8)) & 1)
		if !gba.Within(data, node+branch*2, 2) {
			return textStream{}, ErrText
		}
		node = tree + int(gba.Uint16(data, node+branch*2))*4
		if !gba.Within(data, node, 4) {
			return textStream{}, ErrText
		}
		word := gba.Uint32(data, node)
		if word&0x80000000 == 0 {
			continue
		}
		lo, hi := byte(word&0xFF), byte((word>>8)&0xFF)
		if lo == 0 {
			return textStream{words: out}, nil
		}
		out = append(out, uint16(word&0xFFFF))
		size++
		if hi != 0 {
			size++
		}
		if size > 4096 {
			return textStream{}, ErrText
		}
		node = root
	}
	return textStream{}, ErrText
}

func (f Format) displayText(data []byte, aliases map[byte]string) string {
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
		case f.ShiftJIS && ((b >= 0xA1 && b <= 0xDF) || shiftJISPair(data[i:])):
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
	if f.Names != nil {
		name = f.Names.display.Replace(name)
	}
	return strings.TrimSpace(name)
}

func shiftJISPair(data []byte) bool {
	return len(data) >= 2 &&
		((data[0] >= 0x81 && data[0] <= 0x9F) || (data[0] >= 0xE0 && data[0] <= 0xFC)) &&
		data[1] >= 0x40 &&
		data[1] <= 0xFC &&
		data[1] != 0x7F
}

// nameSymbols keeps display names and their game glyph sequences in one table.
type Symbols struct {
	display, storage *strings.Replacer
}

func newNameSymbols(pairs ...string) *Symbols {
	reverse := make([]string, 0, len(pairs))
	for i := 0; i < len(pairs); i += 2 {
		reverse = append(reverse, pairs[i+1], pairs[i])
	}
	return &Symbols{
		display: strings.NewReplacer(pairs...),
		storage: strings.NewReplacer(reverse...),
	}
}

// These sequences denote compact class-name glyphs, not their Unicode symbols.
// Their names follow FETextEncode's multi-byte game mappings.
var FE8JNames = newNameSymbols(
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

var FE7JNames = newNameSymbols(
	"⑮⑯⑰⑱⑲⑳α", "(ｻﾝﾀﾞｰｽﾄｰﾑ)",
	"⑧⑨⑩⑪⑫⑬⑭", "(ﾌｧﾙｺﾝﾅｲﾄ)",
	"①②③④⑤⑥⑦", "(ﾄﾞﾗｺﾞﾝﾏｽﾀｰ)",
)

var FE6Names = newNameSymbols(
	"①②③④⑤⑥⑦", "(ｻﾝﾀﾞｰｽﾄｰﾑ)",
	"ⅠⅡⅢⅣⅤⅥⅦ", "(ﾌｧﾙｺﾝﾅｲﾄ)",
	"⑪⑫⑬⑭⑮⑯⑰", "(ﾄﾞﾗｺﾞﾝﾏｽﾀｰ)",
)
