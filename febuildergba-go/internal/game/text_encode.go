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

// nameCodes validates the reachable tree and records low-bit-first paths. The
// pinned encoder uses 32-bit codes; deeper trees are outside this writer's scope.
// A text entry may not overwrite any node used by its own decoder.
func (f textFormat) nameCodes(data []byte, entry int) (map[uint16][]byte, error) {
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
		symbol, visible := nameSymbol(word)
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

// nameCharacters returns game characters without splitting Shift-JIS pairs.
// Display aliases are reversed before encoding, and lossy Unicode mappings are
// rejected rather than silently changing the name visible after reopening.
func (f textFormat) nameCharacters(name string) ([]uint16, error) {
	if !utf8.ValidString(name) || strings.TrimSpace(name) != name {
		return nil, ErrNameEncoding
	}
	for _, character := range name {
		if unicode.IsControl(character) || (!f.shiftJIS && (character < 0x20 || character > 0x7E)) {
			return nil, ErrNameEncoding
		}
	}
	storage := name
	if f.names != nil {
		storage = f.names.storage.Replace(storage)
	}
	if f.shiftJIS {
		var err error
		storage, err = japanese.ShiftJIS.NewEncoder().String(storage)
		if err != nil {
			return nil, ErrNameEncoding
		}
	}
	data := []byte(storage)
	if len(data) > 4096 || f.displayText(data, nil) != name {
		return nil, ErrNameEncoding
	}
	var characters []uint16
	for i := 0; i < len(data); i++ {
		character := uint16(data[i])
		if f.shiftJIS && shiftJISPair(data[i:]) {
			character |= uint16(data[i+1]) << 8
			i++
		}
		characters = append(characters, character)
	}
	return characters, nil
}

func (f textFormat) encodeName(data []byte, entry int, name string) ([]byte, error) {
	characters, err := f.nameCharacters(name)
	if err != nil {
		return nil, err
	}
	codes, err := f.nameCodes(data, entry)
	if err != nil {
		return nil, err
	}
	// A pair chosen greedily can leave an unencodable suffix. Work backwards
	// over game characters; only two single-byte characters may be combined.
	cost := make([]int, len(characters)+1)
	width := make([]int, len(characters))
	for i := range cost {
		cost[i] = 65537
	}
	endBits := 7
	if f.shiftJIS {
		endBits = 8
	}
	cost[len(characters)] = max(endBits, len(codes[0]))
	for i := len(characters) - 1; i >= 0; i-- {
		for size := 1; size <= 2 && i+size <= len(characters); size++ {
			symbol := characters[i]
			if size == 2 {
				if symbol > 0xFF || characters[i+1] > 0xFF {
					continue
				}
				symbol |= characters[i+1] << 8
			}
			if code, exists := codes[symbol]; exists && len(code)+cost[i+size] <= cost[i] {
				cost[i], width[i] = len(code)+cost[i+size], size
			}
		}
	}
	if cost[0] > 65536 {
		return nil, ErrNameEncoding
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
	for i := 0; i < len(characters); {
		symbol := characters[i]
		if width[i] == 2 {
			symbol |= characters[i+1] << 8
		}
		write(codes[symbol])
		i += width[i]
	}
	// Match the legacy minimum end space: seven bits for English and eight
	// for Japanese. Zero padding preserves the original end code.
	write(codes[0])
	return encoded, nil
}
