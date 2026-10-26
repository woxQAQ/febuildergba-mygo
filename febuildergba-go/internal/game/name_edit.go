package game

import (
	"bytes"
	"encoding/binary"
	"maps"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/internal/rom"
	"github.com/woxQAQ/febuildergba-mygo/internal/space"
)

// NameUse identifies a known record sharing a name text ID. Other text users,
// including descriptions and events, also observe edits to that same ID.
type NameUse struct {
	Kind Kind
	ID   int
}

// NameEdit is an immutable preview bound to a document revision and table layouts.
type NameEdit struct {
	document *rom.Document
	revision uint64
	layouts  map[Kind]Layout
	textID   uint16
	name     string
	uses     []NameUse
	plan     space.Plan
}

func (e *NameEdit) TextID() uint16   { return e.textID }
func (e *NameEdit) Name() string     { return e.name }
func (e *NameEdit) Revision() uint64 { return e.revision }
func (e *NameEdit) Destination() int { return e.plan.Destination.Start }
func (e *NameEdit) Length() int      { return e.plan.Destination.End - e.plan.Destination.Start }
func (e *NameEdit) Size() int        { return e.plan.Edit.Size }
func (e *NameEdit) Uses() []NameUse  { return slices.Clone(e.uses) }

func (s *Session) PlanNameEdit(kind Kind, id int, name string) (*NameEdit, error) {
	s.Refresh()
	_, row, _, err := s.locate(kind, id, "name_id")
	if err != nil {
		return nil, err
	}
	if row.TextError != nil {
		return nil, row.TextError
	}
	plan, err := s.Adapter.PlanNameEdit(s.image.Data, row.NameID, name)
	if err != nil {
		return nil, err
	}
	edit := &NameEdit{document: s.Document, revision: s.image.Revision, layouts: maps.Clone(s.layouts), textID: row.NameID, name: name, plan: plan}
	for _, table := range s.Tables {
		if table.Err != nil {
			continue
		}
		for _, change := range plan.Edit.Changes {
			if table.Manual && change.Offset < table.Address+table.Stride*table.Count && table.Address < change.Offset+len(change.Data) {
				return nil, ErrLayout
			}
		}
		for _, candidate := range table.Rows {
			if candidate.NameID == row.NameID {
				edit.uses = append(edit.uses, NameUse{Kind: table.Kind, ID: candidate.ID})
			}
		}
	}
	return edit, nil
}

func (s *Session) ApplyNameEdit(edit *NameEdit) error {
	if edit == nil || edit.document != s.Document || !maps.Equal(edit.layouts, s.layouts) {
		return rom.ErrStale
	}
	if err := s.Document.ApplyEdit(edit.revision, "text.name", edit.plan.Edit); err != nil {
		return err
	}
	s.Refresh()
	return nil
}

func (g gameFormat) PlanNameEdit(data []byte, id uint16, name string) (space.Plan, error) {
	if !g.Match(data) {
		return space.Plan{}, ErrVersion
	}
	entry, err := g.nameEntry(data, id)
	if err != nil {
		return space.Plan{}, err
	}
	if _, err = g.text.readName(data, id, nil); err != nil {
		return space.Plan{}, err
	}
	encoded, err := g.text.encodeName(data, entry, name)
	if err != nil {
		return space.Plan{}, err
	}
	plan, err := space.PlanAppend(len(data), encoded)
	if err != nil {
		return space.Plan{}, err
	}
	value := uint32(0x08000000 + plan.Destination.Start) // #nosec G115 -- PlanAppend bounds addresses by MaxSize.
	plan.Edit.Changes = append(plan.Edit.Changes, rom.Change{Offset: entry, Data: binary.LittleEndian.AppendUint32(nil, value)})
	prospective := bytes.Repeat([]byte{0xFF}, plan.Edit.Size)
	copy(prospective, data)
	for _, change := range plan.Edit.Changes {
		copy(prospective[change.Offset:], change.Data)
	}
	var aliases map[byte]string
	if g.features != nil {
		aliases, _ = g.features(prospective)
	}
	if decoded, err := g.text.readName(prospective, id, aliases); err != nil || decoded != name {
		return space.Plan{}, ErrNameEncoding
	}
	return plan, nil
}

// Writes require a valid primary text-table pointer. The recovery address is a
// read fallback, not evidence that the game will consume edits at that address.
func (g gameFormat) nameEntry(data []byte, id uint16) (int, error) {
	table, ok := pointer(data, g.text.pointer)
	entry := table + int(id)*4
	if id == 0 || id >= 0x7FFF || !ok || table%4 != 0 || table < 0xC0 || !within(data, entry, 4) {
		return 0, ErrText
	}
	protected := []int{g.text.pointer, g.text.treePointer, g.text.rootPointer}
	for _, rule := range g.tables {
		protected = append(protected, rule.pointers...)
		other := rule.read(data, Layout{}, false, func(uint16) (string, error) { return "", nil })
		if region, err := rule.extent(data, other); err == nil && entry < region.End && region.Start < entry+4 {
			return 0, ErrLayout
		}
	}
	for _, address := range protected {
		if entry < address+4 && address < entry+4 {
			return 0, ErrText
		}
	}
	return entry, nil
}
