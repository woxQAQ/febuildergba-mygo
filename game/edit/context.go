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

// Input pairs a document snapshot with the stamp captured alongside it.
// Data belongs to this input and must remain read-only while it is used for
// parsing or planning. Drafts retain Stamp rather than the full ROM snapshot.
type Input struct {
	Data  []byte
	stamp Stamp
}

func (i Input) Stamp() Stamp { return i.stamp }

// Capture reads bytes and binds their revision to the current interpretation.
// Capturing and changing interpretation follow the editing session's thread.
func (c *Context) Capture() Input {
	image := c.document.Snapshot()
	return Input{Data: image.Data, stamp: Stamp{
		document: c.document, revision: image.Revision,
		interpretation: c.interpretation, interpreted: true,
	}}
}

// CaptureBytes reads an input whose meaning does not depend on game layouts.
func (c *Context) CaptureBytes() Input {
	input := c.Capture()
	input.stamp.interpretation = ""
	input.stamp.interpreted = false
	return input
}

func (s Stamp) Revision() uint64 { return s.revision }

func (s Stamp) sameDocument(c *Context) bool {
	return c != nil && s.document != nil && s.document == c.document
}

func (s Stamp) Current(c *Context) bool {
	return s.sameDocument(c) &&
		s.revision == c.document.State().Revision &&
		(!s.interpreted || s.interpretation == c.interpretation)
}

// Check retains revision evidence when rejecting a stale operation.
func (s Stamp) Check(c *Context) error {
	if !s.sameDocument(c) ||
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

// apply keeps interpretation validation with the document's atomic revision
// check. All byte and size changes remain one history operation.
func (s Stamp) apply(c *Context, label string, change rom.Edit) error {
	if err := s.Check(c); err != nil {
		return err
	}
	return c.document.ApplyEdit(s.revision, label, change)
}

// Commit is an immutable pending edit bound to its original document, revision
// and interpretation. Construction does not apply the edit; Apply validates the
// binding and submits all bytes and expansion as one document history operation.
// Domain previews retain their typed diagnostics alongside this commit.
type Commit struct {
	stamp  Stamp
	label  string
	change rom.Edit
}

// NewCommit copies change so later mutations cannot alter the pending edit.
// stamp must identify the input used to construct change.
func NewCommit(stamp Stamp, label string, change rom.Edit) *Commit {
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
	return &Commit{stamp: stamp, label: label, change: frozen}
}

func (commit *Commit) Revision() uint64 { return commit.stamp.Revision() }
func (commit *Commit) Size() int        { return commit.change.Size }

func (commit *Commit) Current(
	c *Context,
) bool {
	return commit != nil && commit.stamp.Current(c)
}

func (commit *Commit) Apply(c *Context) error {
	if commit == nil {
		return rom.ErrStale
	}
	return commit.stamp.apply(c, commit.label, commit.change)
}
