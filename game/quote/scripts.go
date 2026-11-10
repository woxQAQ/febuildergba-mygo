package quote

import "github.com/woxQAQ/febuildergba-mygo/gba"

// ScriptReference identifies a native table record's script pointer. Record -1
// reports an unreadable table consumer while other confirmed records remain usable.
type ScriptReference struct {
	Kind    Kind
	Record  int
	Pointer int
	Err     error
}

func (f Format) HasScripts() bool {
	for _, rule := range f.Rules {
		for _, field := range rule.Fields {
			if field.ID == Script {
				return true
			}
		}
	}
	return false
}

// ScriptReferences reads every declared consumer independently. Split consumers
// leave partial results; shared table addresses collapse within the same role,
// while different table roles and independent pointer sites remain distinct.
func (f Format) ScriptReferences(data []byte) ([]ScriptReference, bool) {
	var result []ScriptReference
	complete := true
	for _, rule := range f.Rules {
		for _, field := range rule.Fields {
			if field.ID != Script {
				continue
			}
			seen := make(map[int]bool)
			for _, site := range rule.Pointers {
				table := readTable(data, rule, site)
				if table.Err != nil {
					complete = false
					result = append(result, ScriptReference{
						Kind:    rule.Kind,
						Record:  -1,
						Pointer: site,
						Err:     table.Err,
					})
				}
				if table.Address == 0 || seen[table.Address] {
					continue
				}
				if len(seen) > 0 {
					complete = false
				}
				seen[table.Address] = true
				for index, entry := range table.Entries {
					if gba.Uint32(entry.Raw, field.Offset) == 0 {
						continue
					}
					result = append(result, ScriptReference{
						Kind: rule.Kind, Record: index,
						Pointer: entry.Address + field.Offset,
					})
				}
			}
		}
	}
	return result, complete
}
