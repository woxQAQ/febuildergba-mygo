package text

import (
	"github.com/woxQAQ/febuildergba-mygo/gba"
)

type Format struct {
	Pointer, Recovery        int
	TreePointer, RootPointer int
	ShiftJIS                 bool
	Names                    *Symbols
}

type Role uint8

const (
	NoRole Role = iota
	Name
	Description
)

// TextTable describes the existing contiguous pointer table. Recovery is only
// a browsing fallback; writes still require the game's primary table pointer.
type Table struct {
	Address, Count int
	Recovered      bool
}

// TextEntry is a searchable snapshot of an existing text ID. A broken stream
// remains in the list with its error rather than hiding subsequent entries.
type Entry struct {
	ID       uint16
	Content  string
	Editable bool
	Err      error
}

func textPointer(value uint32, size int) bool {
	switch {
	case value >= 0x08000000 && value < 0x0A000000:
		return int(value-0x08000000) < size
	case value >= 0x88000000 && value < 0x8A000000:
		return int(value-0x88000000) < size
	case value >= 0x02000000 && value < 0x02040000,
		value >= 0x03000000 && value < 0x03008000:
		// RAM text entries participate in the native table's extent but cannot
		// be decoded or replaced from a ROM-only document.
		return true
	default:
		return false
	}
}

func (g Format) Table(data []byte) (Table, error) {
	address, ok := gba.Pointer(data, g.Pointer)
	table := Table{Address: address, Recovered: !ok}
	if !ok {
		table.Address = g.Recovery
	}
	if table.Address < 0xC0 || table.Address%4 != 0 {
		return Table{}, ErrText
	}
	for table.Count < 0x7FFF && gba.Within(data, table.Address+table.Count*4, 4) {
		if !textPointer(
			gba.Uint32(data, table.Address+table.Count*4),
			len(data),
		) {
			break
		}
		table.Count++
	}
	if table.Count < 2 {
		return Table{}, ErrText
	}
	return table, nil
}
