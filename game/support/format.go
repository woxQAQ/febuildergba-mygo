// Package support edits native relationships, dialog and affinity bonuses.
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

// Layout describes the native support record and talk row structure shared
// by one generation. Version files supply the confirmed consumer addresses.
type Layout struct {
	Capacity, Size, TalkSize, TextWidth int
	WideUnits                           bool
}

func FE6Layout() Layout {
	return Layout{Capacity: 10, Size: 32, TalkSize: 16, TextWidth: 4}
}

func FE7Layout() Layout {
	return Layout{Capacity: 7, Size: 24, TalkSize: 20, TextWidth: 4}
}

func FE8Layout() Layout {
	return Layout{
		Capacity:  7,
		Size:      24,
		TalkSize:  16,
		TextWidth: 2,
		WideUnits: true,
	}
}

// Format contains confirmed native consumers, including the support viewer and
// save routines. No pointer-shaped byte scan participates in writes.
type Format struct {
	Layout
	TalkPointers []int
	BonusPointer int
}

// Unit is a character record's support entry. Host names the containing
// record, so the session can tell an owner's own pointer from damage.
type Unit struct {
	ID, Pointer int
	Affinity    uint8
	Name        string
	Host        space.Owner
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
	if !valid || at < 0xC0 || at%4 != 0 || !gba.Within(data, at, f.Size) {
		r.Err = ErrData
		return
	}
	r.Address, r.Raw = at, bytes.Clone(data[at:at+f.Size])
	if int(r.Raw[f.Capacity*3]) > f.Capacity {
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
		address := at + i*f.TalkSize
		if !gba.Within(data, address, f.TalkSize) {
			break
		}
		raw := data[address : address+f.TalkSize]
		ended := raw[0] == 0
		if f.WideUnits {
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
		if f.WideUnits {
			r.Unit, r.Partner = gba.Uint16(raw, 0), gba.Uint16(raw, 2)
		}
		for rank := range r.Texts {
			if f.TextWidth == 2 {
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
	t := f.talksAt(data, f.TalkPointers[0])
	if t.err == nil {
		for _, site := range f.TalkPointers {
			if at, valid := gba.Pointer(data, site); !valid || at != t.address {
				t.err = ErrReferences
				break
			}
		}
	}
	return t
}

// Claims declares every byte this format owns: each character's support
// pointer (hosted by the character record) and payload, and the shared talk
// table with its native consumer pointers. The talk table uses the zero ID,
// which no character record can hold.
func (f Format) Claims(data []byte, units []Unit) []space.Claim {
	result := f.bonusClaims(data)
	for _, unit := range units {
		owner := space.Owner{Resource: "support", ID: unit.ID}
		result = append(
			result,
			space.Claim{
				Range: space.Range{Start: unit.Pointer, End: unit.Pointer + 4},
				Owner: owner,
				Host:  unit.Host,
				Site:  true,
			},
		)
		if r := f.Read(data, unit); r.Address != 0 {
			result = append(
				result,
				space.Claim{
					Range: space.Range{
						Start: r.Address,
						End:   r.Address + f.Size,
					},
					Owner: owner,
				},
			)
		}
	}
	for _, site := range f.TalkPointers {
		owner := space.Owner{Resource: "support"}
		result = append(
			result,
			space.Claim{
				Range: space.Range{Start: site, End: site + 4},
				Owner: owner,
				Site:  true,
			},
		)
		t := f.talksAt(data, site)
		length := len(t.rows)*f.TalkSize + len(t.terminator)
		if t.address != 0 && length > 0 {
			result = append(
				result,
				space.Claim{
					Range: space.Range{
						Start: t.address,
						End:   t.address + length,
					},
					Owner: owner,
				},
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
