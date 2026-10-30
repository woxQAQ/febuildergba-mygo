// Package graphics plans atomic PNG replacement through native image backends.
package graphics

import (
	"bytes"
	"image"
	"image/draw"
	"image/png"
	"maps"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

// References supplies confirmed cross-resource protection and record uses for
// the exact snapshot being planned. Native image encoders own their layouts.
type References struct {
	Uses   []Use
	Ranges []space.Range
	Sites  []int
}

type Editor struct {
	context    *edit.Context
	backends   map[Kind]Backend
	references func([]byte, Ref) References
	image      rom.Snapshot
	binding    edit.Stamp
}

func New(
	context *edit.Context,
	backends map[Kind]Backend,
	references func([]byte, Ref) References,
) *Editor {
	return &Editor{
		context:    context,
		backends:   maps.Clone(backends),
		references: references,
	}
}

func (s *Editor) refresh() error {
	if !s.binding.Current(s.context) {
		s.image = s.context.Document().Snapshot()
		s.binding = s.context.Bind(s.image.Revision)
	}
	return s.context.Validate(s.image.Data)
}

func (s *Editor) backend(ref Ref) (Backend, error) {
	if err := s.refresh(); err != nil {
		return nil, err
	}
	backend, ok := s.backends[ref.Kind]
	if !ok {
		return nil, ErrImage
	}
	return backend, nil
}

func (s *Editor) Preview(ref Ref) (result Preview, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.preview_asset",
			fault.Context{Resource: string(ref.Kind), ResourceID: new(ref.ID)},
		)
	}()
	backend, err := s.backend(ref)
	if err != nil {
		return Preview{}, err
	}
	return backend.Preview(s.image.Data, ref.ID)
}

func (s *Editor) PNG(ref Ref) (content []byte, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.export_asset",
			fault.Context{Resource: string(ref.Kind), ResourceID: new(ref.ID)},
		)
	}()
	backend, err := s.backend(ref)
	if err != nil {
		return nil, err
	}
	bitmap, err := backend.Image(s.image.Data, ref.ID)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := png.Encode(&out, bitmap); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

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
