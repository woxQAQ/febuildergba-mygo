// Package shop edits native two-byte inventories independently of their references.
package shop

import (
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/gba"
)

// Native loaders copy 21 halfwords, including the terminator after 20 items.
const (
	MaxItems    = 20
	PayloadSize = (MaxItems + 1) * 2
)

var ErrInventory = fault.Code{
	ID:   "game.invalid_shop_inventory",
	Kind: fault.InvalidData,
}

// Item retains the second byte without assuming a durability or skill patch.
type Item struct {
	ID, Extra byte
}

// Draft owns inventory bytes only. The referring editor binds it to a document
// and commits its payload together with the selected references.
type Draft struct {
	address         int
	original, items []Item
	terminator      byte
}

func New() *Draft { return &Draft{} }

func Read(data []byte, address int) (result *Draft, err error) {
	defer func() {
		err = fault.Wrap(err, "shop.read_inventory", fault.Context{
			Resource: "shop_inventory",
			Range: &fault.Range{
				Offset: address,
				Length: PayloadSize,
				Size:   len(data),
			},
		})
	}()
	if address < 0xC0 || address%2 != 0 ||
		!gba.Within(data, address, PayloadSize) {
		return nil, ErrInventory
	}
	d := &Draft{address: address}
	for i := 0; i <= MaxItems; i++ {
		at := address + i*2
		if data[at] == 0 {
			d.terminator = data[at+1]
			d.original = slices.Clone(d.items)
			return d, nil
		}
		if i == MaxItems {
			break
		}
		d.items = append(d.items, Item{ID: data[at], Extra: data[at+1]})
	}
	return nil, ErrInventory
}

func (d *Draft) Address() int  { return d.address }
func (d *Draft) Items() []Item { return slices.Clone(d.items) }
func (d *Draft) Count() int    { return len(d.items) }

func (d *Draft) Modified() bool { return d.address == 0 || !slices.Equal(d.original, d.items) }

func (d *Draft) Clone() *Draft {
	result := *d
	result.original, result.items = slices.Clone(
		d.original,
	), slices.Clone(
		d.items,
	)
	return &result
}

func (d *Draft) Add(id int) error {
	if id < 1 || id > 255 || len(d.items) >= MaxItems {
		return edit.ErrValue
	}
	d.items = append(
		d.items,
		Item{ID: byte(id)},
	) // #nosec G115 -- Item IDs are bounded above.
	return nil
}

func (d *Draft) SetItem(index, id int) error {
	if index < 0 || index >= len(d.items) {
		return edit.ErrField
	}
	if id < 1 || id > 255 {
		return edit.ErrValue
	}
	d.items[index].ID = byte(id) // #nosec G115 -- Item IDs are bounded above.
	return nil
}

func (d *Draft) SetExtra(index, value int) error {
	if index < 0 || index >= len(d.items) {
		return edit.ErrField
	}
	if value < 0 || value > 255 {
		return edit.ErrValue
	}
	d.items[index].Extra = byte(
		value,
	) // #nosec G115 -- Raw bytes are bounded above.
	return nil
}

func (d *Draft) Remove(index int) error {
	if index < 0 || index >= len(d.items) {
		return edit.ErrField
	}
	d.items = slices.Delete(d.items, index, index+1)
	return nil
}

func (d *Draft) Move(from, to int) error {
	if from < 0 || from >= len(d.items) || to < 0 || to >= len(d.items) {
		return edit.ErrField
	}
	item := d.items[from]
	d.items = slices.Delete(d.items, from, from+1)
	d.items = slices.Insert(d.items, to, item)
	return nil
}

// Payload pads the new allocation for the native fixed-length copy. Bytes after
// the old terminator belong to the original ROM and are never reclaimed.
func (d *Draft) Payload() []byte {
	result := make([]byte, PayloadSize)
	for i, item := range d.items {
		result[i*2], result[i*2+1] = item.ID, item.Extra
	}
	result[len(d.items)*2+1] = d.terminator
	return result
}
