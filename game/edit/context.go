// Package edit binds domain operations to a document and its interpretation.
package edit

import (
	"bytes"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

// Context belongs to one editing session. Interpretation changes are published
// by the session on the same thread as domain reads and plan construction.
// Document persistence retains its own synchronization.
type Context struct {
	document       *rom.Document
	match          func([]byte) bool
	interpretation string
}

func New(document *rom.Document, match func([]byte) bool) *Context {
	return &Context{document: document, match: match}
}

func (c *Context) Validate(data []byte) error {
	if !c.match(data) {
		return ErrVersion
	}
	return nil
}

func (c *Context) Document() *rom.Document { return c.document }

// Interpret records the effective parsing settings, not UI selection or a ROM
// revision. Equal settings restore the same interpretation identity.
func (c *Context) Interpret(settings string) { c.interpretation = settings }

// Stamp identifies the exact input used to construct an operation. Its fields
// are private so a caller cannot retarget an existing plan to another document.
type Stamp struct {
	document       *rom.Document
	revision       uint64
	interpretation string
	interpreted    bool
}

// Bind captures the interpretation accompanying the supplied snapshot revision.
func (c *Context) Bind(revision uint64) Stamp {
	return Stamp{c.document, revision, c.interpretation, true}
}

// BindBytes is for operations whose meaning does not depend on game layouts.
func (c *Context) BindBytes(revision uint64) Stamp {
	return Stamp{document: c.document, revision: revision}
}

func (s Stamp) Revision() uint64 { return s.revision }

func (s Stamp) SameDocument(c *Context) bool {
	return c != nil && s.document != nil && s.document == c.document
}

func (s Stamp) Current(c *Context) bool {
	return s.SameDocument(c) &&
		s.revision == c.document.State().Revision &&
		(!s.interpreted || s.interpretation == c.interpretation)
}

// Check retains revision evidence when rejecting a stale operation.
func (s Stamp) Check(c *Context) error {
	if !s.SameDocument(c) ||
		(s.interpreted && s.interpretation != c.interpretation) {
		return rom.ErrStale
	}
	actual := c.document.State().Revision
	if actual != s.revision {
		return fault.Wrap(rom.ErrStale, "game.check_edit", fault.Context{
			Revision: &fault.Revision{Expected: s.revision, Actual: actual},
		})
	}
	return nil
}

// Apply keeps interpretation validation with the document's atomic revision
// check. All byte and size changes remain one history operation.
func (s Stamp) Apply(c *Context, label string, change rom.Edit) error {
	if err := s.Check(c); err != nil {
		return err
	}
	return c.document.ApplyEdit(s.revision, label, change)
}

// Plan freezes one complete edit. Domain previews retain their own typed
// diagnostics and resource information alongside this commit payload.
type Plan struct {
	stamp  Stamp
	label  string
	change rom.Edit
}

func NewPlan(stamp Stamp, label string, change rom.Edit) *Plan {
	frozen := rom.Edit{
		Size:    change.Size,
		Changes: make([]rom.Change, len(change.Changes)),
	}
	for i, part := range change.Changes {
		frozen.Changes[i] = rom.Change{
			Offset: part.Offset,
			Data:   bytes.Clone(part.Data),
		}
	}
	return &Plan{stamp: stamp, label: label, change: frozen}
}

func (p *Plan) Revision() uint64 { return p.stamp.Revision() }
func (p *Plan) Size() int        { return p.change.Size }

func (p *Plan) Current(
	c *Context,
) bool {
	return p != nil && p.stamp.Current(c)
}

func (p *Plan) Apply(c *Context) error {
	if p == nil {
		return rom.ErrStale
	}
	return p.stamp.Apply(c, p.label, p.change)
}
