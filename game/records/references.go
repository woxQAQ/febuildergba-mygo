package records

import (
	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func (s *Editor) TextReferences(data []byte, id uint16) text.References {
	definition := s.source(data)
	inspection := s.Inspect(data)
	var result text.References
	for _, table := range inspection.Tables {
		if table.Err != nil {
			continue
		}
		if table.Manual {
			result.Manual = append(
				result.Manual,
				space.Range{
					Start: table.Address,
					End:   table.Address + table.Stride*table.Count,
				},
			)
		}
		for _, row := range table.Rows {
			for _, field := range table.Fields {
				if field.Text != text.NoRole && field.Width == 2 &&
					gba.Uint16(data, row.Address+field.Offset) == id {
					result.Uses = append(
						result.Uses,
						text.Use{
							Kind:  string(table.Kind),
							ID:    row.ID,
							Field: field.ID,
						},
					)
				}
			}
		}
	}
	result.Ranges = definition.Protected(data)
	for _, rule := range definition.Rules {
		result.Sites = append(result.Sites, rule.Pointers...)
		table := rule.Read(
			data,
			Layout{},
			false,
			func(uint16) (string, error) { return "", nil },
		)
		if region, err := rule.Extent(data, table); err == nil {
			result.Ranges = append(result.Ranges, region)
		}
	}
	result.Aliases = definition.TextAliases
	return result
}

func (s *Editor) ImageReferences(
	data []byte,
	ref graphics.Ref,
) graphics.References {
	definition := s.source(data)
	result := graphics.References{Ranges: definition.Protected(data)}
	for _, rule := range definition.Rules {
		result.Sites = append(result.Sites, rule.Pointers...)
	}
	for _, table := range s.Inspect(data).Tables {
		if table.Err != nil {
			continue
		}
		result.Ranges = append(
			result.Ranges,
			space.Range{
				Start: table.Address,
				End:   table.Address + table.Stride*table.Count,
			},
		)
		for _, row := range table.Rows {
			for _, field := range table.Fields {
				if field.Asset == ref.Kind &&
					field.Value(data, row.Address) == int64(ref.ID) {
					result.Uses = append(
						result.Uses,
						graphics.Use{
							Kind:  string(table.Kind),
							ID:    row.ID,
							Field: field.ID,
						},
					)
				}
			}
		}
	}
	return result
}
