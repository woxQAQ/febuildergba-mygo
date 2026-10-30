package text

import (
	"bytes"
	"encoding/binary"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

type Format struct {
	Pointer, Recovery        int
	TreePointer, RootPointer int
	ShiftJIS                 bool
	Names                    *Symbols
}

type Role uint8

const (
	NoRole Role = iota
	Name
	Description
)

// TextTable describes the existing contiguous pointer table. Recovery is only
// a browsing fallback; writes still require the game's primary table pointer.
type Table struct {
	Address, Count int
	Recovered      bool
}

// TextEntry is a searchable snapshot of an existing text ID. A broken stream
// remains in the list with its error rather than hiding subsequent entries.
type Entry struct {
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

func (g Format) Table(data []byte) (Table, error) {
	address, ok := gba.Pointer(data, g.Pointer)
	table := Table{Address: address, Recovered: !ok}
	if !ok {
		table.Address = g.Recovery
	}
	if table.Address < 0xC0 || table.Address%4 != 0 {
		return Table{}, ErrText
	}
	for table.Count < 0x7FFF && gba.Within(data, table.Address+table.Count*4, 4) {
		if !textPointer(
			gba.Uint32(data, table.Address+table.Count*4),
			len(data),
		) {
			break
		}
		table.Count++
	}
	if table.Count < 2 {
		return Table{}, ErrText
	}
	return table, nil
}

func (g Format) Read(
	data []byte,
	id uint16,
) (result Value, err error) {
	defer func() {
		ctx := fault.Context{Resource: Resource, ResourceID: new(int(id))}
		err = fault.Wrap(err, "game.read_text", ctx)
		result.ReadOnlyReason = fault.Wrap(
			result.ReadOnlyReason,
			"game.edit_text",
			ctx,
		)
	}()
	stream, err := g.readStream(data, id)
	if err != nil {
		return Value{}, err
	}
	value := g.descriptionValue(stream)
	value.ID = id
	if id == 0 {
		value.Editable, value.ReadOnlyReason = false, ErrText
	}
	return value, nil
}

func (g Format) encodeText(
	data []byte,
	entry int,
	id uint16,
	content string,
	role Role,
) ([]byte, string, error) {
	switch role {
	case Name:
		if _, err := g.ReadName(data, id, nil); err != nil {
			return nil, "", err
		}
		encoded, err := g.encodeName(data, entry, content)
		return encoded, content, err
	case Description:
		original, err := g.Read(data, id)
		if err != nil {
			return nil, "", err
		}
		if !original.Editable {
			return nil, "", fault.Wrap(
				original.ReadOnlyReason,
				"game.encode_text",
				fault.Context{Resource: Resource, ResourceID: new(int(id))},
			)
		}
		value, err := g.parseDescription(content)
		if err != nil {
			return nil, "", fault.Wrap(
				err,
				"game.parse_text",
				fault.Context{Resource: Resource, ResourceID: new(int(id))},
			)
		}
		if original.Content == value.Content {
			return nil, "", edit.ErrValue
		}
		encoded, err := g.encodeTextUnits(data, entry, value.units, false)
		return encoded, value.Content, err
	default:
		return nil, "", edit.ErrField
	}
}

func (g Format) plan(
	data []byte,
	id uint16,
	content string,
	role Role,
	entry int,
	aliases map[byte]string,
) (space.Plan, error) {
	encoded, expected, err := g.encodeText(data, entry, id, content, role)
	if err != nil {
		return space.Plan{}, err
	}
	plan, err := space.PlanAppend(len(data), encoded)
	if err != nil {
		return space.Plan{}, err
	}
	value := uint32(
		0x08000000 + plan.Destination.Start,
	) // #nosec G115 -- PlanAppend bounds addresses by MaxSize.
	plan.Edit.Changes = append(
		plan.Edit.Changes,
		rom.Change{
			Offset: entry,
			Data:   binary.LittleEndian.AppendUint32(nil, value),
		},
	)
	prospective := bytes.Repeat([]byte{0xFF}, plan.Edit.Size)
	copy(prospective, data)
	for _, change := range plan.Edit.Changes {
		copy(prospective[change.Offset:], change.Data)
	}
	if role == Description {
		decoded, decodeErr := g.Read(prospective, id)
		if decodeErr != nil || !decoded.Editable ||
			decoded.Content != expected {
			return space.Plan{}, ErrTextEncoding
		}
	} else {
		if decoded, decodeErr := g.ReadName(
			prospective,
			id,
			aliases,
		); decodeErr != nil ||
			decoded != expected {
			return space.Plan{}, ErrNameEncoding
		}
	}
	return plan, nil
}
