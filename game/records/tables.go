package records

import (
	"fmt"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

type Rule struct {
	Kind                                Kind
	Pointers                            []int
	Stride, FirstID, Count, Probe, Skip int
	PointerFields                       []int
	NameOffset                          int
	Accepts                             func([]byte, int) bool
	Fields                              []Field
}

func (rule Rule) Read(
	data []byte,
	layout Layout,
	manual bool,
	nameOf func(uint16) (string, error),
) Table {
	t := Table{
		Kind:    rule.Kind,
		Pointer: -1,
		FirstID: rule.FirstID,
		Fields:  rule.Fields,
		Manual:  manual,
	}
	if manual {
		t.Layout = layout
		if !validLayout(data, layout, rule.FirstID, rule.Fields) {
			t.Err = fault.Wrap(
				edit.ErrLayout,
				"game.validate_layout",
				fault.Context{
					Table: string(rule.Kind),
					Range: &fault.Range{
						Offset: t.Address,
						Length: t.Stride,
						Size:   len(data),
					},
				},
			)
			return t
		}
	} else {
		t.Layout = Layout{Stride: rule.Stride, Count: rule.Count}
		t.Pointer, t.Address = rule.locate(data)
		if t.Pointer < 0 {
			t.Err = fault.Wrap(
				edit.ErrLayout,
				"game.locate_table",
				fault.Context{
					Table: string(rule.Kind),
					Range: &fault.Range{
						Offset: t.Address,
						Length: t.Stride,
						Size:   len(data),
					},
				},
			)
			return t
		}
	}
	for i := range t.Count {
		at := t.Address + i*t.Stride
		if !gba.Within(data, at, t.Stride) {
			break
		}
		if !manual && rule.ends(data, at, i) {
			break
		}
		nameID := gba.Uint16(data, at+rule.NameOffset)
		name, err := nameOf(nameID)
		if err != nil {
			name = fmt.Sprintf("[text 0x%04X]", nameID)
		}
		t.Rows = append(
			t.Rows,
			Row{
				ID:      rule.FirstID + i,
				Address: at,
				Name:    name,
				NameID:  nameID,
				TextError: fault.Wrap(
					err,
					"game.read_name",
					fault.Context{
						Table:      string(rule.Kind),
						RecordID:   new(rule.FirstID + i),
						Resource:   text.Resource,
						ResourceID: new(int(nameID)),
					},
				),
			},
		)
	}
	if len(t.Rows) == 0 {
		t.Err = fault.Wrap(
			edit.ErrLayout,
			"game.read_table",
			fault.Context{
				Table: string(rule.Kind),
				Range: &fault.Range{
					Offset: t.Address,
					Length: t.Stride,
					Size:   len(data),
				},
			},
		)
	}
	t.Count = len(t.Rows)
	return t
}

func (rule Rule) ends(data []byte, at, index int) bool {
	if rule.Accepts != nil {
		return !rule.Accepts(data, at)
	}
	if rule.Kind == Classes && index > 0 && data[at+4] == 0 {
		return true
	}
	for _, offset := range rule.PointerFields {
		p := gba.Uint32(data, at+offset)
		if p != 0 && (p < 0x08000000 || p >= 0x0A000000) {
			return true
		}
	}
	return false
}

// locate prefers the legacy structural probe, then accepts a bounded relocated
// table.
func (rule Rule) locate(data []byte) (int, int) {
	for pass := range 2 {
		for _, p := range rule.Pointers {
			base, ok := gba.Pointer(data, p)
			base += rule.Skip
			if !ok || base%4 != 0 || !gba.Within(data, base, rule.Stride) {
				continue
			}
			if rule.Accepts != nil && !rule.Accepts(data, base) {
				continue
			}
			if pass == 0 && rule.Probe >= 0 {
				if _, valid := gba.Pointer(data, base+rule.Probe); !valid {
					continue
				}
			}
			if rule.Kind == Units &&
				!gba.Within(data, base, rule.Stride*rule.Count) {
				continue
			}
			return p, base
		}
	}
	return -1, 0
}

func validLayout(data []byte, layout Layout, firstID int, fields []Field) bool {
	if layout.Address < 0 || layout.Address%4 != 0 ||
		layout.Address >= len(data) ||
		layout.Stride < 4 ||
		layout.Stride%4 != 0 ||
		layout.Count < 1 ||
		layout.Count > 256-firstID {
		return false
	}
	// Division checks the entire extent without overflowing address + count *
	// stride.
	if layout.Count > (len(data)-layout.Address)/layout.Stride {
		return false
	}
	for _, f := range fields {
		if f.Offset < 0 || f.Width < 1 || f.Offset > layout.Stride-f.Width {
			return false
		}
	}
	return true
}

// extent includes FE6's hidden ID 0 and the record that stops automatic
// parsing.
func (rule Rule) Extent(data []byte, table Table) (space.Range, error) {
	if table.Err != nil || table.Manual || table.Stride != rule.Stride ||
		table.Count < 1 ||
		table.Count > rule.Count {
		return space.Range{}, edit.ErrLayout
	}
	start := table.Address - rule.Skip
	length := rule.Skip + table.Count*rule.Stride
	if table.Count < rule.Count {
		end := start + length
		if !gba.Within(data, end, rule.Stride) ||
			!rule.ends(data, end, table.Count) {
			return space.Range{}, fault.Wrap(
				edit.ErrLayout,
				"game.table_terminator",
				fault.Context{
					Table: string(rule.Kind),
					Range: &fault.Range{
						Offset: end,
						Length: rule.Stride,
						Size:   len(data),
					},
				},
			)
		}
		length += rule.Stride
	}
	if !gba.Within(data, start, length) {
		return space.Range{}, edit.ErrLayout
	}
	return space.Range{Start: start, End: start + length}, nil
}
