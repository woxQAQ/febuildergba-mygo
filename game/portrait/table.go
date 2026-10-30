package portrait

import (
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/gba"
)

// PortraitEntry identifies the shared portrait-table record, including class
// cards. Character/class fields are uses of this resource, not its owner.
type Entry struct {
	ID, Address, Stride int
	ClassCard, Empty    bool
	Name                string
	Uses                []graphics.Use
	Err                 error
}

// PortraitEntries follows the live table pointer. Null holes are retained,
// trailing null records are trimmed, and discovery is bounded as in the legacy
// pointer-table reader rather than assuming the original number of portraits.
func (s *Editor) Entries() ([]Entry, error) {
	s.Refresh()
	if s.portraitEntries != nil {
		return clonePortraitEntries(s.portraitEntries), nil
	}
	f := s.format
	base, valid := gba.Pointer(s.image.Data, f.Pointer)
	if !valid || base < 0xC0 {
		return nil, graphics.ErrImage
	}
	uses, names := s.portraitUses()
	var entries []Entry
	nulls, last := 0, 0
	for id := range 4096 {
		at := base + id*f.Stride
		if !gba.Within(s.image.Data, at, f.Stride) {
			break
		}
		invalid := false
		for offset := 0; offset < 12; offset += 4 {
			if gba.Uint32(s.image.Data, at+offset) == 0 {
				continue
			}
			target, ok := gba.Pointer(s.image.Data, at+offset)
			if !ok || target < 0xC0 {
				invalid = true
				break
			}
		}
		if invalid && id > 0 && len(uses[id]) == 0 &&
			!portraitRecordReadable(s.image.Data, at+f.Stride, f.Stride) {
			break
		}
		entry := Entry{
			ID:      id,
			Address: at,
			Stride:  f.Stride,
			Uses:    uses[id],
			Name:    names[id],
		}
		if invalid {
			entry.Err = graphics.ErrImage
		}
		entry.Empty = gba.Uint32(s.image.Data, at) == 0 &&
			gba.Uint32(s.image.Data, at+4) == 0 &&
			gba.Uint32(s.image.Data, at+8) == 0
		entry.ClassCard = !entry.Empty &&
			gba.Uint32(s.image.Data, at+f.CardMarker) == 0
		entries = append(entries, entry)
		if entry.Empty {
			nulls++
		} else {
			nulls = 0
			last = len(entries)
		}
		if nulls >= 1000 {
			break
		}
	}
	if last == 0 {
		last = min(1, len(entries))
	}
	s.portraitEntries = entries[:last]
	return clonePortraitEntries(s.portraitEntries), nil
}

func portraitRecordReadable(data []byte, at, stride int) bool {
	if !gba.Within(data, at, stride) || gba.Uint32(data, at+8) == 0 {
		return false
	}
	for offset := 0; offset < 12; offset += 4 {
		if gba.Uint32(data, at+offset) == 0 {
			continue
		}
		target, ok := gba.Pointer(data, at+offset)
		if !ok || target < 0xC0 {
			return false
		}
	}
	return true
}

func clonePortraitEntries(entries []Entry) []Entry {
	result := slices.Clone(entries)
	for index := range result {
		result[index].Uses = slices.Clone(result[index].Uses)
	}
	return result
}

func (s *Editor) portraitUses() (map[int][]graphics.Use, map[int]string) {
	uses := make(map[int][]graphics.Use)
	names := make(map[int]string)
	for _, table := range s.records.Tables() {
		if table.Err != nil {
			continue
		}
		for _, field := range table.Fields {
			if field.Asset != graphics.Portrait {
				continue
			}
			for _, row := range table.Rows {
				id := int(field.Value(s.image.Data, row.Address))
				if id == 0 {
					continue
				}
				uses[id] = append(
					uses[id],
					graphics.Use{
						Kind:  string(table.Kind),
						ID:    row.ID,
						Field: field.ID,
					},
				)
				if names[id] == "" {
					names[id] = row.Name
				}
			}
		}
	}
	return uses, names
}

func (s *Editor) Entry(id int) (Entry, error) {
	entries, err := s.Entries()
	if err != nil {
		return Entry{}, err
	}
	for _, entry := range entries {
		if entry.ID == id {
			return entry, nil
		}
	}
	return Entry{}, graphics.ErrImage
}
