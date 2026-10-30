package graphics

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/draw"
	"image/png"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func (s *Editor) Plan(ref Ref, content []byte) (result *Edit, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.plan_asset",
			fault.Context{Resource: string(ref.Kind), ResourceID: new(ref.ID)},
		)
	}()
	backend, err := s.backend(ref)
	if err != nil {
		return nil, err
	}
	if len(content) == 0 || len(content) > MaxPNG {
		return nil, ErrSize
	}
	config, err := png.DecodeConfig(bytes.NewReader(content))
	if err != nil {
		return nil, fault.New(
			ErrImage,
			"game.decode_png_header",
			fault.Context{},
			err,
		)
	}
	if config.Width < 8 || config.Height < 8 || config.Width > 160 ||
		config.Height > 160 {
		return nil, ErrSize
	}
	bitmap, err := png.Decode(bytes.NewReader(content))
	if err != nil {
		return nil, fault.New(ErrImage, "game.decode_png", fault.Context{}, err)
	}
	original, err := backend.Image(s.image.Data, ref.ID)
	if err != nil {
		return nil, err
	}
	if bitmap.Bounds() != original.Bounds() {
		return nil, ErrSize
	}
	normalized := image.NewRGBA(bitmap.Bounds())
	draw.Draw(
		normalized,
		normalized.Bounds(),
		bitmap,
		bitmap.Bounds().Min,
		draw.Src,
	)
	if bytes.Equal(normalized.Pix, original.Pix) {
		return nil, edit.ErrValue
	}
	encoding, err := backend.Encode(s.image.Data, ref.ID, bitmap)
	if err != nil {
		return nil, err
	}
	references := s.references(s.image.Data, ref)
	if !validTargets(s.image.Data, encoding.Parts, references) {
		return nil, edit.ErrLayout
	}
	plan, err := Append(len(s.image.Data), encoding.Parts)
	if err != nil {
		return nil, err
	}
	for _, region := range references.Ranges {
		for _, change := range plan.Edit.Changes {
			if change.Offset < region.End &&
				region.Start < change.Offset+len(change.Data) {
				return nil, edit.ErrLayout
			}
		}
	}
	prospective := bytes.Repeat([]byte{0xFF}, plan.Edit.Size)
	copy(prospective, s.image.Data)
	for _, change := range plan.Edit.Changes {
		copy(prospective[change.Offset:], change.Data)
	}
	decoded, err := backend.Image(prospective, ref.ID)
	if err != nil {
		return nil, err
	}
	expected := image.NewRGBA(bitmap.Bounds())
	draw.Draw(
		expected,
		expected.Bounds(),
		encoding.Expected,
		image.Point{},
		draw.Src,
	)
	if !bytes.Equal(expected.Pix, decoded.Pix) {
		return nil, ErrImage
	}
	return &Edit{
		binding: s.binding,
		ref:     ref,
		plan:    plan,
		uses:    slices.Clone(references.Uses),
		image:   decoded,
	}, nil
}

func (s *Editor) Apply(plan *Edit) (err error) {
	ctx := fault.Context{}
	if plan != nil {
		ctx.Resource = string(plan.ref.Kind)
		ctx.ResourceID = new(plan.ref.ID)
	}
	defer func() { err = fault.Wrap(err, "game.apply_asset", ctx) }()
	if plan == nil {
		return rom.ErrStale
	}
	return plan.binding.Apply(
		s.context,
		"asset."+string(plan.ref.Kind),
		plan.plan.Edit,
	)
}

func validTargets(
	data []byte,
	parts []Replacement,
	references References,
) bool {
	seen := make(map[int]bool)
	for _, part := range parts {
		at := part.Pointer
		if at < 0xC0 || at%4 != 0 || !gba.Within(data, at, 4) || seen[at] {
			return false
		}
		seen[at] = true
		for _, region := range references.Ranges {
			if at < region.End && region.Start < at+4 {
				return false
			}
		}
		for _, site := range references.Sites {
			if at < site+4 && site < at+4 {
				return false
			}
		}
	}
	return true
}

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
