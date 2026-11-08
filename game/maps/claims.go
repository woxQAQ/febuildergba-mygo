package maps

import (
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/romwrite"
)

func resourceOwner(field string, site int) romwrite.Owner {
	// Both loading roles consume the same kind of graphics PLIST entry.
	if field == GraphicsAltField {
		field = GraphicsField
	}
	return romwrite.Owner{Resource: "maps." + field, ID: site}
}

// Claims protects concrete map PLIST consumers and their bounded payloads.
// Other kinds in a shared PLIST retain distinct ownership even at the same site.
func (f Format) Claims(data []byte, chapters *records.Table) []romwrite.Claim {
	var claims []romwrite.Claim
	add := func(start, length int, owner romwrite.Owner, site bool) {
		if start >= 0xC0 && start < len(data) && length > 0 {
			claims = append(
				claims,
				romwrite.Claim{
					Start: start,
					End:   min(start+length, len(data)),
					Owner: owner,
					Site:  site,
				},
			)
		}
	}
	seen := make(map[romwrite.Owner]bool)
	for _, binding := range f.bindings(nil) {
		if binding.Root == 0 {
			continue
		}
		add(
			binding.Root,
			4,
			romwrite.Owner{Resource: "maps.root", ID: binding.Root},
			true,
		)
		if chapters == nil || chapters.Err != nil {
			continue
		}
		base, ok := gba.Pointer(data, binding.Root)
		if !ok || base < 0xC0 || base%4 != 0 {
			continue
		}
		for _, field := range chapters.Fields {
			if field.ID != binding.Field {
				continue
			}
			for _, row := range chapters.Rows {
				id := field.Value(data, row.Address)
				if id < 0 || id > 255 || id == 0 && binding.Optional {
					continue
				}
				site := base + int(id)*4
				owner := resourceOwner(binding.Field, site)
				if seen[owner] {
					continue
				}
				seen[owner] = true
				add(site, 4, owner, true)
				address, ok := gba.Pointer(data, site)
				if !ok || address < 0xC0 {
					continue
				}
				length := max(4, binding.Length)
				if binding.CompressedLimit > 0 {
					if _, consumed, err := gba.DecompressLZ77Stream(
						data[address:],
						binding.CompressedLimit,
					); err == nil {
						length = consumed
					}
				}
				if binding.Field == ChangesField {
					if table, err := readMapChanges(
						data,
						binding.Root,
						id,
					); err == nil &&
						table.address != 0 {
						length = len(table.rows)*12 + len(table.terminator)
						for _, change := range table.rows {
							payload := int(
								gba.Uint32(change.header[:], 8) & 0x01FFFFFF,
							)
							add(payload, len(change.tiles), owner, false)
						}
					}
				}
				add(address, length, owner, false)
			}
		}
	}
	return claims
}
