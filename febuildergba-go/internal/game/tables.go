package game

import "fmt"

type tableRule struct {
	kind                                Kind
	pointers                            []int
	stride, firstID, count, probe, skip int
	pointerFields                       []int
	fields                              []Field
}

func (rule tableRule) read(data []byte, layout Layout, manual bool, nameOf func(uint16) (string, error)) Table {
	t := Table{Kind: rule.kind, Pointer: -1, FirstID: rule.firstID, Fields: rule.fields, Manual: manual}
	if manual {
		t.Layout = layout
		if !validLayout(data, layout, rule.firstID, rule.fields) {
			t.Err = fmt.Errorf("%w: %s", ErrLayout, rule.kind)
			return t
		}
	} else {
		t.Layout = Layout{Stride: rule.stride, Count: rule.count}
		t.Pointer, t.Address = rule.locate(data)
		if t.Pointer < 0 {
			t.Err = fmt.Errorf("%w: %s", ErrLayout, rule.kind)
			return t
		}
	}
	for i := 0; i < t.Count; i++ {
		at := t.Address + i*t.Stride
		if !within(data, at, t.Stride) {
			break
		}
		if !manual && rule.ends(data, at, i) {
			break
		}
		nameID := u16(data, at)
		name, err := nameOf(nameID)
		if err != nil {
			name = fmt.Sprintf("[text 0x%04X]", nameID)
		}
		t.Rows = append(t.Rows, Row{ID: rule.firstID + i, Address: at, Name: name, NameID: nameID, TextError: err})
	}
	if len(t.Rows) == 0 {
		t.Err = fmt.Errorf("%w: %s", ErrLayout, rule.kind)
	}
	t.Count = len(t.Rows)
	return t
}

func (rule tableRule) ends(data []byte, at, index int) bool {
	if rule.kind == Classes && index > 0 && data[at+4] == 0 {
		return true
	}
	for _, offset := range rule.pointerFields {
		p := u32(data, at+offset)
		if p != 0 && (p < 0x08000000 || p >= 0x0A000000) {
			return true
		}
	}
	return false
}

// locate prefers the legacy structural probe, then accepts a bounded relocated table.
func (rule tableRule) locate(data []byte) (int, int) {
	for pass := range 2 {
		for _, p := range rule.pointers {
			base, ok := pointer(data, p)
			base += rule.skip
			if !ok || base%4 != 0 || !within(data, base, rule.stride) {
				continue
			}
			if pass == 0 && rule.probe >= 0 {
				if _, valid := pointer(data, base+rule.probe); !valid {
					continue
				}
			}
			if rule.kind == Units && !within(data, base, rule.stride*rule.count) {
				continue
			}
			return p, base
		}
	}
	return -1, 0
}

func validLayout(data []byte, layout Layout, firstID int, fields []Field) bool {
	if layout.Address < 0 || layout.Address%4 != 0 || layout.Address >= len(data) ||
		layout.Stride < 4 || layout.Stride%4 != 0 || layout.Count < 1 || layout.Count > 256-firstID {
		return false
	}
	// Division checks the entire extent without overflowing address + count * stride.
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
