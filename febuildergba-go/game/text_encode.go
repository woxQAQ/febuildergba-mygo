package game

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/encoding/japanese"
)

// nameSymbol removes the name-padding control from a leaf's visible output.
// A zero low byte terminates text; a padding-only leaf does not.
func nameSymbol(word uint32) (uint16, bool) {
	lo, hi := uint16(word&0xFF), uint16((word>>8)&0xFF)
	if lo == 0 {
		return 0, true
	}
	if hi == 0x1F {
		hi = 0
	}
	if lo == 0x1F {
		lo, hi = hi, 0
	}
	return lo | hi<<8, lo != 0
}

// textCodes validates the reachable tree and records low-bit-first paths. The
// pinned encoder uses 32-bit codes; deeper trees are outside this writer's scope.
// A text entry may not overwrite any node used by its own decoder.
func (f textFormat) textCodes(data []byte, entry int, names bool) (map[uint16][]byte, error) {
	tree, treeOK := pointer(data, f.treePointer)
	indirect, indirectOK := pointer(data, f.rootPointer)
	root, rootOK := pointer(data, indirect)
	if !treeOK || !indirectOK || !rootOK || tree%4 != 0 || indirect%4 != 0 || entry == indirect {
		return nil, ErrText
	}
	codes := make(map[uint16][]byte)
	state := make(map[int]byte)
	var visit func(int, []byte) error
	visit = func(at int, path []byte) error {
		if !within(data, at, 4) || at < tree || (at-tree)%4 != 0 || (at-tree)/4 > 0xFFFF ||
			len(path) > 32 || state[at] == 1 || (entry < at+4 && at < entry+4) {
			return ErrText
		}
		if state[at] == 2 {
			return nil
		}
		state[at] = 1
		word := u32(data, at)
		if word&0x80000000 == 0 {
			for branch := range 2 {
				next := tree + int(u16(data, at+branch*2))*4
				if err := visit(next, append(path, byte(branch))); err != nil {
					return err
				}
			}
			state[at] = 2
			return nil
		}
		if len(path) == 0 {
			return ErrText
		}
		symbol, visible := uint16(word&0xFFFF), true
		if names {
			symbol, visible = nameSymbol(word)
		} else if symbol&0xFF == 0 {
			symbol = 0
		}
		if visible {
			if previous, exists := codes[symbol]; !exists || len(path) < len(previous) {
				codes[symbol] = slices.Clone(path)
			}
		}
		state[at] = 2
		return nil
	}
	if err := visit(root, nil); err != nil {
		return nil, err
	}
	if len(codes[0]) == 0 {
		return nil, ErrText
	}
	return codes, nil
}

// characterUnits encodes a plain span without splitting Shift-JIS characters.
// Display aliases are reversed before encoding; lossy mappings are rejected.
func (f textFormat) characterUnits(content string) ([]textUnit, error) {
	if !utf8.ValidString(content) {
		return nil, ErrTextEncoding
	}
	for _, character := range content {
		if unicode.IsControl(character) || (!f.shiftJIS && (character < 0x20 || character > 0x7E)) {
			return nil, ErrTextEncoding
		}
	}
	storage := content
	if f.names != nil {
		storage = f.names.storage.Replace(storage)
	}
	if f.shiftJIS {
		var err error
		storage, err = japanese.ShiftJIS.NewEncoder().String(storage)
		if err != nil {
			return nil, ErrTextEncoding
		}
	}
	data := []byte(storage)
	decoded, valid := f.decodeCharacters(data)
	if len(data) > 4096 || !valid || decoded != content {
		return nil, ErrTextEncoding
	}
	var units []textUnit
	for i := 0; i < len(data); i++ {
		character := uint16(data[i])
		if f.shiftJIS && shiftJISPair(data[i:]) {
			character |= uint16(data[i+1]) << 8
			i++
		}
		units = append(units, textUnit{symbol: character, pair: true})
	}
	return units, nil
}

func (f textFormat) encodeName(data []byte, entry int, name string) ([]byte, error) {
	if strings.TrimSpace(name) != name {
		return nil, ErrNameEncoding
	}
	units, err := f.characterUnits(name)
	if err != nil {
		return nil, ErrNameEncoding
	}
	encoded, err := f.encodeTextUnits(data, entry, units, true)
	if err == ErrTextEncoding {
		return nil, ErrNameEncoding
	}
	return encoded, err
}

func (f textFormat) encodeTextUnits(data []byte, entry int, units []textUnit, names bool) ([]byte, error) {
	codes, err := f.textCodes(data, entry, names)
	if err != nil {
		return nil, err
	}
	// A pair chosen greedily can leave an unencodable suffix. Work backwards
	// over game characters; only two single-byte characters may be combined.
	cost := make([]int, len(units)+1)
	width := make([]int, len(units))
	for i := range cost {
		cost[i] = 65537
	}
	endBits := 7
	if f.shiftJIS {
		endBits = 8
	}
	cost[len(units)] = max(endBits, len(codes[0]))
	for i := len(units) - 1; i >= 0; i-- {
		for size := 1; size <= 2 && i+size <= len(units); size++ {
			symbol := units[i].symbol
			if size == 2 {
				extended := symbol == 0x80 && units[i].control && units[i+1].control
				if (!extended && (!units[i].pair || !units[i+1].pair)) || symbol > 0xFF || units[i+1].symbol > 0xFF {
					continue
				}
				symbol |= units[i+1].symbol << 8
			}
			if code, exists := codes[symbol]; exists && len(code)+cost[i+size] <= cost[i] {
				cost[i], width[i] = len(code)+cost[i+size], size
			}
		}
	}
	if cost[0] > 65536 {
		return nil, ErrTextEncoding
	}
	// The pinned decoder reads a 32-bit window at each byte. Keep three
	// trailing bytes so the final symbol remains readable at the ROM end.
	encoded := make([]byte, (cost[0]+7)/8+3)
	bit := 0
	write := func(code []byte) {
		for _, branch := range code {
			encoded[bit/8] |= branch << (bit % 8)
			bit++
		}
	}
	for i := 0; i < len(units); {
		symbol := units[i].symbol
		if width[i] == 2 {
			symbol |= units[i+1].symbol << 8
		}
		write(codes[symbol])
		i += width[i]
	}
	// Match the legacy minimum end space: seven bits for English and eight
	// for Japanese. Zero padding preserves the original end code.
	write(codes[0])
	return encoded, nil
}
