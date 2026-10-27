package game

import (
	"bytes"
	"maps"

	"github.com/woxQAQ/febuildergba-mygo/rom"
)

// UseLayout changes interpretation for this session, without rewriting pointers or ROM bytes.
func (s *Session) UseLayout(kind Kind, layout Layout) error {
	s.Refresh()
	if !s.Adapter.Match(s.image.Data) {
		return ErrVersion
	}
	if s.Table(kind) == nil {
		return ErrLayout
	}
	layouts := maps.Clone(s.layouts)
	if layouts == nil {
		layouts = make(map[Kind]Layout)
	}
	layouts[kind] = layout
	inspection := s.Adapter.Inspect(s.image.Data, layouts)
	for _, table := range inspection.Tables {
		if table.Kind != kind {
			continue
		}
		if table.Err != nil {
			return table.Err
		}
		s.layouts, s.Tables, s.Patches = layouts, inspection.Tables, inspection.Patches
		return nil
	}
	return ErrLayout
}

func (s *Session) UseAutomaticLayout(kind Kind) {
	delete(s.layouts, kind)
	s.refresh(s.Document.Snapshot())
}

// RawRegion records the document and revision whose bytes were shown for editing.
// Its original bytes cannot be changed through the UI's draft buffer.
type RawRegion struct {
	document *rom.Document
	revision uint64
	address  int
	original []byte
}

func (r *RawRegion) Address() int     { return r.address }
func (r *RawRegion) Revision() uint64 { return r.revision }
func (r *RawRegion) Bytes() []byte    { return bytes.Clone(r.original) }

func (s *Session) ReadRegion(address, length int) (*RawRegion, error) {
	s.Refresh()
	if length < 1 || !within(s.image.Data, address, length) {
		return nil, rom.ErrRange
	}
	return &RawRegion{document: s.Document, revision: s.image.Revision, address: address, original: bytes.Clone(s.image.Data[address : address+length])}, nil
}

// ApplyRegion is a fixed-length replacement. A stale draft must be read again explicitly.
func (s *Session) ApplyRegion(region *RawRegion, replacement []byte) error {
	if region == nil || region.document != s.Document {
		return rom.ErrStale
	}
	if len(replacement) != len(region.original) {
		return rom.ErrRange
	}
	if err := s.Document.Apply(region.revision, "raw", []rom.Change{{Offset: region.address, Data: replacement}}); err != nil {
		return err
	}
	s.Refresh()
	return nil
}
