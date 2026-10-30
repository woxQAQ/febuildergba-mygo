package records

import (
	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

type Kind string

const (
	Units    Kind = "units"
	Classes  Kind = "classes"
	Items    Kind = "items"
	Chapters Kind = "chapters"
)

var Kinds = []Kind{Units, Classes, Items, Chapters}

type Field struct {
	ID, Group     string
	Offset, Width int
	Signed, Hex   bool
	Reference     Kind
	Asset         graphics.Kind
	Text          text.Role
}

func (f Field) Bounds() (int64, int64) {
	if f.Signed {
		return -(1 << (f.Width*8 - 1)), (1 << (f.Width*8 - 1)) - 1
	}
	return 0, (1 << (f.Width * 8)) - 1
}

func (f Field) Value(data []byte, address int) int64 {
	var v uint32
	for i := range f.Width {
		v |= uint32(data[address+f.Offset+i]) << (i * 8)
	}
	value := int64(v)
	bits := f.Width * 8
	if f.Signed && value&(1<<(bits-1)) != 0 {
		value -= 1 << bits
	}
	return value
}

type Row struct {
	ID, Address int
	Name        string
	NameID      uint16
	TextError   error
}

type Layout struct {
	Address, Stride, Count int
}

type Table struct {
	Layout
	Kind             Kind
	Pointer, FirstID int
	Manual           bool
	Fields           []Field
	Rows             []Row
	Err              error
}

type Inspection struct {
	Tables  []Table
	Patches []string
}

// Format is the resolved record schema and name reader for one ROM snapshot.
// Version selection and chapter resource discovery supply these concrete rules.
type Format struct {
	TextAliases map[byte]string
	Version     string
	Rules       []Rule
	NameOf      func(uint16) (string, error)
	Patches     []string
	Protected   func([]byte) []space.Range
}

func (f Format) Inspect(data []byte, layouts map[Kind]Layout) Inspection {
	result := Inspection{Patches: f.Patches}
	for _, rule := range f.Rules {
		layout, manual := layouts[rule.Kind]
		result.Tables = append(
			result.Tables,
			rule.Read(data, layout, manual, f.NameOf),
		)
	}
	return result
}
