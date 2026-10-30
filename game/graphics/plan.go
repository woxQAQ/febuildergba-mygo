package graphics

import (
	"encoding/binary"
	"image"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

var (
	ErrFormat = fault.Code{
		ID:   "game.unsupported_asset_format",
		Kind: fault.Unsupported,
	}
	ErrSize = fault.Code{
		ID:   "game.invalid_asset_size",
		Kind: fault.InvalidInput,
	}
	ErrColors = fault.Code{
		ID:   "game.invalid_asset_colors",
		Kind: fault.InvalidInput,
	}
	ErrPixels = fault.Code{
		ID:   "game.unmapped_asset_pixels",
		Kind: fault.InvalidInput,
	}
)

// MaxAssetPNG bounds both file reads and decoding; sheets are at most 160x160.
const MaxPNG = 1 << 20

type Use struct {
	Kind  string
	ID    int
	Field string
}

// AssetEdit binds a complete, immutable resource replacement to one document
// state. Other asset IDs retain their existing pointers, even when their old
// data aliases.
type Edit struct {
	binding edit.Stamp
	ref     Ref
	uses    []Use
	plan    space.Plan
	image   *image.RGBA
}

func (e *Edit) Ref() Ref { return e.ref }

func (e *Edit) Revision() uint64 { return e.binding.Revision() }

func (e *Edit) Uses() []Use { return slices.Clone(e.uses) }

func (e *Edit) Bounds() image.Rectangle { return e.image.Bounds() }

func (e *Edit) Image() *image.RGBA {
	out := image.NewRGBA(e.image.Bounds())
	copy(out.Pix, e.image.Pix)
	return out
}

type Replacement struct {
	Pointer int
	Data    []byte
}

func Append(
	size int,
	resources []Replacement,
) (space.Plan, error) {
	plan := space.Plan{Edit: rom.Edit{Size: size}}
	for _, resource := range resources {
		allocation, err := space.PlanAppend(plan.Edit.Size, resource.Data)
		if err != nil {
			return space.Plan{}, err
		}
		address := uint32(
			0x08000000 + allocation.Destination.Start,
		) // #nosec G115 -- PlanAppend enforces the ROM size limit.
		plan.Edit.Size = allocation.Edit.Size
		plan.Edit.Changes = append(
			plan.Edit.Changes,
			allocation.Edit.Changes...)
		plan.Edit.Changes = append(
			plan.Edit.Changes,
			rom.Change{
				Offset: resource.Pointer,
				Data:   binary.LittleEndian.AppendUint32(nil, address),
			},
		)
		if plan.Destination.Start == 0 {
			plan.Destination.Start = allocation.Destination.Start
		}
		plan.Destination.End = allocation.Destination.End
	}
	return plan, nil
}
