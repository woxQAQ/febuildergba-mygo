package game

import (
	"slices"
)

// PortraitEntry identifies the shared portrait-table record, including class
// cards. Character/class fields are uses of this resource, not its owner.
type PortraitEntry struct {
	ID, Address, Stride int
	ClassCard, Empty    bool
	Name                string
	Uses                []AssetUse
	Err                 error
}

// PortraitEntries follows the live table pointer. Null holes are retained,
// trailing null records are trimmed, and discovery is bounded as in the legacy
// pointer-table reader rather than assuming the original number of portraits.
func (s *Session) PortraitEntries() ([]PortraitEntry, error) {
	s.Refresh()
	if s.portraitEntries != nil {
		return clonePortraitEntries(s.portraitEntries), nil
	}
	format, ok := s.Adapter.(gameFormat)
	if !ok {
		return nil, ErrVersion
	}
	f := format.portraits
	base, valid := pointer(s.image.Data, f.pointer)
	if !valid || base < 0xC0 {
		return nil, ErrImage
	}
	uses, names := s.portraitUses()
	var entries []PortraitEntry
	nulls, last := 0, 0
	for id := range 4096 {
		at := base + id*f.stride
		if !within(s.image.Data, at, f.stride) {
			break
		}
		invalid := false
		for offset := 0; offset < 12; offset += 4 {
			if u32(s.image.Data, at+offset) == 0 {
				continue
			}
			target, ok := pointer(s.image.Data, at+offset)
			if !ok || target < 0xC0 {
				invalid = true
				break
			}
		}
		if invalid && id > 0 && len(uses[id]) == 0 &&
			!portraitRecordReadable(s.image.Data, at+f.stride, f.stride) {
			break
		}
		entry := PortraitEntry{
			ID:      id,
			Address: at,
			Stride:  f.stride,
			Uses:    uses[id],
			Name:    names[id],
		}
		if invalid {
			entry.Err = ErrImage
		}
		entry.Empty = u32(s.image.Data, at) == 0 &&
			u32(s.image.Data, at+4) == 0 &&
			u32(s.image.Data, at+8) == 0
		entry.ClassCard = !entry.Empty &&
			u32(s.image.Data, at+f.cardMarker) == 0
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
	if !within(data, at, stride) || u32(data, at+8) == 0 {
		return false
	}
	for offset := 0; offset < 12; offset += 4 {
		if u32(data, at+offset) == 0 {
			continue
		}
		target, ok := pointer(data, at+offset)
		if !ok || target < 0xC0 {
			return false
		}
	}
	return true
}

func clonePortraitEntries(entries []PortraitEntry) []PortraitEntry {
	result := slices.Clone(entries)
	for index := range result {
		result[index].Uses = slices.Clone(result[index].Uses)
	}
	return result
}

func (s *Session) portraitUses() (map[int][]AssetUse, map[int]string) {
	uses := make(map[int][]AssetUse)
	names := make(map[int]string)
	for _, table := range s.Tables {
		if table.Err != nil {
			continue
		}
		for _, field := range table.Fields {
			if field.Asset != PortraitAsset {
				continue
			}
			for _, row := range table.Rows {
				id := int(field.Value(s.image.Data, row.Address))
				if id == 0 {
					continue
				}
				uses[id] = append(
					uses[id],
					AssetUse{Kind: table.Kind, ID: row.ID, Field: field.ID},
				)
				if names[id] == "" {
					names[id] = row.Name
				}
			}
		}
	}
	return uses, names
}

func (s *Session) PortraitEntry(id int) (PortraitEntry, error) {
	entries, err := s.PortraitEntries()
	if err != nil {
		return PortraitEntry{}, err
	}
	for _, entry := range entries {
		if entry.ID == id {
			return entry, nil
		}
	}
	return PortraitEntry{}, ErrImage
}
