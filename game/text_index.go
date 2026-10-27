package game

import (
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
)

// TextTable describes the existing contiguous pointer table. Recovery is only
// a browsing fallback; writes still require the game's primary table pointer.
type TextTable struct {
	Address, Count int
	Recovered      bool
}

// TextEntry is a searchable snapshot of an existing text ID. A broken stream
// remains in the list with its error rather than hiding subsequent entries.
type TextEntry struct {
	ID       uint16
	Content  string
	Editable bool
	Err      error
}

func textPointer(value uint32, size int) bool {
	switch {
	case value >= 0x08000000 && value < 0x0A000000:
		return int(value-0x08000000) < size
	case value >= 0x88000000 && value < 0x8A000000:
		return int(value-0x88000000) < size
	case value >= 0x02000000 && value < 0x02040000,
		value >= 0x03000000 && value < 0x03008000:
		// RAM text entries participate in the native table's extent but cannot
		// be decoded or replaced from a ROM-only document.
		return true
	default:
		return false
	}
}

func (g gameFormat) TextTable(data []byte) (TextTable, error) {
	if !g.Match(data) {
		return TextTable{}, ErrVersion
	}
	address, ok := pointer(data, g.text.pointer)
	table := TextTable{Address: address, Recovered: !ok}
	if !ok {
		table.Address = g.text.recovery
	}
	if table.Address < 0xC0 || table.Address%4 != 0 {
		return TextTable{}, ErrText
	}
	for table.Count < 0x7FFF && within(data, table.Address+table.Count*4, 4) {
		if !textPointer(u32(data, table.Address+table.Count*4), len(data)) {
			break
		}
		table.Count++
	}
	if table.Count < 2 {
		return TextTable{}, ErrText
	}
	return table, nil
}

func (s *Session) TextEntries() (entries []TextEntry, err error) {
	defer func() { err = fault.Wrap(err, "game.list_text", fault.Context{Resource: textResource}) }()
	s.Refresh()
	if s.texts != nil {
		return slices.Clone(s.texts), nil
	}
	table, err := s.Adapter.TextTable(s.image.Data)
	if err != nil {
		return nil, err
	}
	s.texts = make([]TextEntry, 0, table.Count-1)
	for id := 1; id < table.Count; id++ {
		textID := uint16(id) // #nosec G115 -- TextTable bounds Count by 0x7FFF.
		value, readErr := s.Adapter.ReadText(s.image.Data, textID)
		s.texts = append(
			s.texts,
			TextEntry{
				ID:       textID,
				Content:  value.Content,
				Editable: value.Editable && !table.Recovered,
				Err:      readErr,
			},
		)
	}
	return slices.Clone(s.texts), nil
}

func (s *Session) TextByID(id uint16) (result TextValue, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.text_by_id",
			fault.Context{Resource: textResource, ResourceID: new(int(id))},
		)
	}()
	s.Refresh()
	table, err := s.Adapter.TextTable(s.image.Data)
	if err != nil {
		return TextValue{}, err
	}
	if id == 0 || int(id) >= table.Count {
		return TextValue{}, ErrText
	}
	value, err := s.Adapter.ReadText(s.image.Data, id)
	if table.Recovered && err == nil {
		value.Editable, value.ReadOnlyReason = false, fault.Wrap(
			ErrText,
			"game.recovered_text_table",
			fault.Context{Resource: textResource, ResourceID: new(int(id))},
		)
	}
	return value, err
}

func (s *Session) PlanTextIDEdit(
	id uint16,
	content string,
) (result *TextEdit, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.plan_text_id",
			fault.Context{Resource: textResource, ResourceID: new(int(id))},
		)
	}()
	value, err := s.TextByID(id)
	if err != nil {
		return nil, err
	}
	if !value.Editable {
		return nil, value.ReadOnlyReason
	}
	return s.planTextEdit(id, "content", content, DescriptionText)
}

func (s *Session) TextUses(id uint16) []TextUse {
	s.Refresh()
	var uses []TextUse
	for _, table := range s.Tables {
		if table.Err != nil {
			continue
		}
		for _, row := range table.Rows {
			for _, field := range table.Fields {
				if field.Text != NoText && field.Width == 2 &&
					u16(s.image.Data, row.Address+field.Offset) == id {
					uses = append(
						uses,
						TextUse{Kind: table.Kind, ID: row.ID, Field: field.ID},
					)
				}
			}
		}
	}
	return uses
}
