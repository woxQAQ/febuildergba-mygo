// Package support edits native character relationships and support dialog.
package support

import (
	"bytes"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

var (
	ErrData = fault.Code{
		ID:   "game.invalid_support",
		Kind: fault.InvalidData,
	}
	ErrReferences = fault.Code{
		ID:   "game.support_reference_conflict",
		Kind: fault.Conflict,
	}
)

const (
	TalkUse  = "support_talk"
	maxTalks = 2048
)

// Format contains confirmed native consumers, including the support viewer and
// save routines. No pointer-shaped byte scan participates in writes.
type Format struct {
	capacity, size, talkSize, textWidth int
	wideUnits                           bool
	pointers                            []int
}

func FE6() Format {
	return Format{10, 32, 16, 4, false, []int{0x6AFE0}}
}

func FE7(japanese bool) Format {
	pointers := []int{
		0x78A94,
		0x78B04,
		0x78B64,
		0x9B164,
		0x9B18C,
		0x9EC2C,
		0x9ECF4,
		0x9EE00,
		0x9EE70,
	}
	if japanese {
		pointers = []int{
			0x79264,
			0x792D4,
			0x79334,
			0x9BB38,
			0x9BB60,
			0x9F668,
			0x9F730,
			0x9F83C,
			0x9F8AC,
		}
	}
	return Format{7, 24, 20, 4, false, pointers}
}

func FE8(japanese bool) Format {
	pointers := []int{0x84784, 0x847FC}
	if japanese {
		pointers = []int{0x86A50, 0x86AC8}
	}
	return Format{7, 24, 16, 2, true, pointers}
}

type Unit struct {
	ID, Pointer int
	Name        string
}

type Relationship struct{ Unit, Initial, Growth uint8 }

type Record struct {
	Unit
	Address int
	Empty   bool
	Err     error
	Raw     []byte
}

func (f Format) Read(data []byte, unit Unit) (r Record) {
	r.Unit = unit
	if !gba.Within(data, unit.Pointer, 4) {
		r.Err = ErrData
		return
	}
	if gba.Uint32(data, unit.Pointer) == 0 {
		r.Empty = true
		return
	}
	at, valid := gba.Pointer(data, unit.Pointer)
	if !valid || at < 0xC0 || at%4 != 0 || !gba.Within(data, at, f.size) {
		r.Err = ErrData
		return
	}
	r.Address, r.Raw = at, bytes.Clone(data[at:at+f.size])
	if int(r.Raw[f.capacity*3]) > f.capacity {
		r.Err = ErrData
	}
	return
}

type Talk struct {
	Index, Address int
	Unit, Partner  uint16
	Texts          [3]uint32
	Raw            []byte
}

type talkTable struct {
	address    int
	rows       []Talk
	terminator []byte
	err        error
}

func (f Format) talksAt(data []byte, site int) (t talkTable) {
	at, valid := gba.Pointer(data, site)
	if !valid || at < 0xC0 || at%4 != 0 {
		t.err = ErrData
		return
	}
	t.address = at
	for i := 0; i <= maxTalks; i++ {
		address := at + i*f.talkSize
		if !gba.Within(data, address, f.talkSize) {
			break
		}
		raw := data[address : address+f.talkSize]
		ended := raw[0] == 0
		if f.wideUnits {
			ended = gba.Uint16(raw, 0) == 0xFFFF
		}
		if ended {
			t.terminator = bytes.Clone(raw)
			return
		}
		if i == maxTalks {
			break
		}
		r := Talk{
			Index:   i,
			Address: address,
			Unit:    uint16(raw[0]),
			Partner: uint16(raw[1]),
			Raw:     bytes.Clone(raw),
		}
		if f.wideUnits {
			r.Unit, r.Partner = gba.Uint16(raw, 0), gba.Uint16(raw, 2)
		}
		for rank := range r.Texts {
			if f.textWidth == 2 {
				r.Texts[rank] = uint32(gba.Uint16(raw, 4+rank*2))
			} else {
				r.Texts[rank] = gba.Uint32(raw, 4+rank*4)
			}
		}
		t.rows = append(t.rows, r)
	}
	t.err = ErrData
	return
}

func (f Format) talks(data []byte) talkTable {
	t := f.talksAt(data, f.pointers[0])
	if t.err == nil {
		for _, site := range f.pointers {
			if at, valid := gba.Pointer(data, site); !valid || at != t.address {
				t.err = ErrReferences
				break
			}
		}
	}
	return t
}

func (f Format) Ranges(data []byte, units []Unit) []space.Range {
	var result []space.Range
	for _, unit := range units {
		result = append(
			result,
			space.Range{Start: unit.Pointer, End: unit.Pointer + 4},
		)
		if r := f.Read(data, unit); r.Address != 0 {
			result = append(
				result,
				space.Range{Start: r.Address, End: r.Address + f.size},
			)
		}
	}
	for _, site := range f.pointers {
		result = append(result, space.Range{Start: site, End: site + 4})
		t := f.talksAt(data, site)
		length := len(t.rows)*f.talkSize + len(t.terminator)
		if t.address != 0 && length > 0 {
			result = append(
				result,
				space.Range{Start: t.address, End: t.address + length},
			)
		}
	}
	return result
}

func (f Format) TextUses(data []byte, id uint16) []text.Use {
	var result []text.Use
	for _, row := range f.talks(data).rows {
		for rank, value := range row.Texts {
			if value == uint32(id) {
				result = append(
					result,
					text.Use{
						Kind:  TalkUse,
						ID:    row.Index,
						Field: []string{"support_c", "support_b", "support_a"}[rank],
					},
				)
			}
		}
	}
	return result
}

func cloneTalks(rows []Talk) []Talk {
	result := slices.Clone(rows)
	for i := range result {
		result[i].Raw = bytes.Clone(result[i].Raw)
	}
	return result
}
