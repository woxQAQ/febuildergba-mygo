package game

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/woxQAQ/febuildergba-mygo/internal/rom"
	"github.com/woxQAQ/febuildergba-mygo/internal/space"
)

var (
	ErrVersion = errors.New("game.unsupported_version")
	ErrLayout  = errors.New("game.invalid_layout")
	ErrField   = errors.New("game.invalid_field")
	ErrValue   = errors.New("game.invalid_value")
	ErrText    = errors.New("game.invalid_text")
	ErrImage   = errors.New("game.invalid_image")
)

type Kind string

const (
	Units   Kind = "units"
	Classes Kind = "classes"
	Items   Kind = "items"
)

var Kinds = []Kind{Units, Classes, Items}

type Field struct {
	ID, Group     string
	Offset, Width int
	Signed, Hex   bool
	Reference     Kind
	Asset         AssetKind
}

func (f Field) Bounds() (int64, int64) {
	if f.Signed {
		return -(1 << (f.Width*8 - 1)), (1 << (f.Width*8 - 1)) - 1
	}
	return 0, (1 << (f.Width * 8)) - 1
}

func (f Field) Value(data []byte, address int) int64 {
	var v uint32
	for i := 0; i < f.Width; i++ {
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

type Adapter interface {
	ID() string
	Match([]byte) bool
	Inspect([]byte, map[Kind]Layout) Inspection
	TableMove([]byte, Table) (space.Move, error)
	Preview([]byte, AssetRef) (Preview, error)
}

// Session combines a document with a game adapter. The UI sees schema and records, not ROM offsets.
type Session struct {
	Document *rom.Document
	Adapter  Adapter
	Tables   []Table
	Patches  []string
	image    rom.Snapshot
	layouts  map[Kind]Layout
}

func Open(path string) (*Session, error) {
	doc, err := rom.Open(path)
	if err != nil {
		return nil, err
	}
	image := doc.Snapshot()
	for _, adapter := range []Adapter{fe8uFormat(), fe8jFormat(), fe7uFormat(), fe7jFormat(), fe6Format()} {
		if adapter.Match(image.Data) {
			s := &Session{Document: doc, Adapter: adapter}
			s.refresh(image)
			return s, nil
		}
	}
	return nil, ErrVersion
}

func (s *Session) Refresh() {
	if s.Document.State().Revision != s.image.Revision {
		s.refresh(s.Document.Snapshot())
	}
}

func (s *Session) refresh(image rom.Snapshot) {
	s.image = image
	inspection := s.Adapter.Inspect(image.Data, s.layouts)
	s.Tables, s.Patches = inspection.Tables, inspection.Patches
	if !s.Adapter.Match(image.Data) {
		for i := range s.Tables {
			s.Tables[i].Err = ErrVersion
		}
	}
}

func (s *Session) Table(kind Kind) *Table {
	for i := range s.Tables {
		if s.Tables[i].Kind == kind {
			return &s.Tables[i]
		}
	}
	return nil
}

func (s *Session) Value(kind Kind, id int, fieldID string) (int64, error) {
	table, row, field, err := s.locate(kind, id, fieldID)
	if err != nil {
		return 0, err
	}
	if table.Err != nil {
		return 0, table.Err
	}
	return field.Value(s.image.Data, row.Address), nil
}

func (s *Session) locate(kind Kind, id int, fieldID string) (*Table, Row, Field, error) {
	t := s.Table(kind)
	if t == nil {
		return nil, Row{}, Field{}, ErrLayout
	}
	if t.Err != nil {
		return nil, Row{}, Field{}, t.Err
	}
	for _, row := range t.Rows {
		if row.ID != id {
			continue
		}
		for _, f := range t.Fields {
			if f.ID == fieldID {
				return t, row, f, nil
			}
		}
	}
	return nil, Row{}, Field{}, ErrField
}

// Set applies one field to one or more records as a single undoable operation.
func (s *Session) Set(kind Kind, ids []int, fieldID string, value int64) error {
	var changes []rom.Change
	for _, id := range ids {
		_, row, field, err := s.locate(kind, id, fieldID)
		if err != nil {
			return err
		}
		low, high := field.Bounds()
		if value < low || value > high {
			return fmt.Errorf("%w: %s (%d…%d)", ErrValue, fieldID, low, high)
		}
		data := make([]byte, field.Width)
		for i := range data {
			data[i] = byte((value >> (i * 8)) & 0xFF)
		}
		changes = append(changes, rom.Change{Offset: row.Address + field.Offset, Data: data})
	}
	if err := s.Document.Apply(s.image.Revision, string(kind)+"."+fieldID, changes); err != nil {
		return err
	}
	s.Refresh()
	return nil
}

func (s *Session) Raw(address, length int) ([]byte, error) {
	if !within(s.image.Data, address, length) {
		return nil, rom.ErrRange
	}
	return bytes.Clone(s.image.Data[address : address+length]), nil
}

func within(data []byte, offset, size int) bool {
	return offset >= 0 && size >= 0 && offset <= len(data) && size <= len(data)-offset
}
func u16(data []byte, offset int) uint16 { return binary.LittleEndian.Uint16(data[offset:]) }
func u32(data []byte, offset int) uint32 { return binary.LittleEndian.Uint32(data[offset:]) }
func pointer(data []byte, at int) (int, bool) {
	if !within(data, at, 4) {
		return 0, false
	}
	p := u32(data, at)
	if p < 0x08000000 || p >= 0x0A000000 {
		return 0, false
	}
	offset := int(p - 0x08000000)
	return offset, offset < len(data)
}
