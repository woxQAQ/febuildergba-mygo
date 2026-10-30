// Package graphics plans atomic PNG replacement through native image backends.
package graphics

import (
	"bytes"
	"image/png"
	"maps"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
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
