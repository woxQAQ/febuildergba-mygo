package shop

import (
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

// Source supplies confirmed references and prevents damaged owner layouts from
// overwriting unrelated live resources. Unresolved owners remain diagnostics.
type Source struct {
	References     func([]byte) ([]Reference, []error)
	ProtectPointer func(data []byte, owner space.Owner, site int) error
}

type Entry struct {
	Reference
	Address, Count           int
	Null, UsesDefault, Empty bool
	Err                      error
}

type Editor struct {
	context  *edit.Context
	format   Format
	source   Source
	image    rom.Snapshot
	binding  edit.Stamp
	entries  []Entry
	warnings []error
}

func New(context *edit.Context, format Format, source Source) *Editor {
	return &Editor{context: context, format: format, source: source}
}

func (s *Editor) Refresh() {
	if s.binding.Current(s.context) {
		return
	}
	s.image = s.context.Document().Snapshot()
	s.binding = s.context.Bind(s.image.Revision)
	s.entries, s.warnings = nil, nil
	refs := s.format.References()
	if s.source.References != nil {
		additional, warnings := s.source.References(s.image.Data)
		refs = append(refs, additional...)
		s.warnings = warnings
	}
	for _, ref := range refs {
		s.entries = append(s.entries, s.readEntry(ref))
	}
}

func (s *Editor) Entries() ([]Entry, []error) {
	s.Refresh()
	return slices.Clone(s.entries), slices.Clone(s.warnings)
}

func (s *Editor) readEntry(ref Reference) Entry {
	entry := Entry{Reference: ref}
	data := s.image.Data
	if ref.Pointer < 0xC0 || ref.Pointer%4 != 0 ||
		!gba.Within(data, ref.Pointer, 4) {
		entry.Err = ErrInventory
		return entry
	}
	entry.Null = gba.Uint32(data, ref.Pointer) == 0
	var inventory *Draft
	if entry.Null &&
		(ref.Key.Origin == Chapter || ref.Key.Origin == Preparation) {
		inventory, entry.Err = s.format.Default(data)
		entry.UsesDefault = true
	} else if address, valid := gba.Pointer(
		data,
		ref.Pointer,
	); valid {
		inventory, entry.Err = Read(data, address)
	} else {
		entry.Err = ErrInventory
	}
	if inventory != nil {
		entry.Address, entry.Count = inventory.Address(), inventory.Count()
		entry.Empty = inventory.WorldMapHidden()
	}
	return entry
}

// EntryDraft owns one selected preparation/default/world-map reference. Its
// inventory can be replaced with a fresh empty list without writing a null pointer.
type EntryDraft struct {
	binding   edit.Stamp
	entry     Entry
	inventory *Draft
}

func (d *EntryDraft) Entry() Entry      { return d.entry }
func (d *EntryDraft) Inventory() *Draft { return d.inventory }

func (d *EntryDraft) Current(
	s *Editor,
) bool {
	return d != nil && d.binding.Current(s.context)
}

func (d *EntryDraft) Modified() bool { return d.inventory != nil && d.inventory.Modified() }

// Clear also initializes a confirmed world-map slot whose pointer is null.
// It always uses a zero first halfword, including when the old terminator's
// second byte was nonzero.
func (d *EntryDraft) Clear() { d.inventory = NewDraft() }

func (s *Editor) NewDraft(key Key) (*EntryDraft, error) {
	s.Refresh()
	if err := s.context.Validate(s.image.Data); err != nil {
		return nil, err
	}
	for _, entry := range s.entries {
		if entry.Key != key {
			continue
		}
		if key.Origin == Chapter {
			return nil, edit.ErrField
		}
		if entry.Err != nil && (key.Origin != WorldMap || !entry.Null) {
			return nil, entry.Err
		}
		draft := &EntryDraft{binding: s.binding, entry: entry}
		if entry.Err == nil {
			inventory, err := Read(s.image.Data, entry.Address)
			if err != nil {
				return nil, err
			}
			draft.inventory = inventory
		}
		return draft, nil
	}
	return nil, edit.ErrField
}

// EntryPlan freezes both the appended inventory and its one selected pointer.
// Known uses describe sharing before the change, not a complete ROM reference index.
type EntryPlan struct {
	commit               *edit.Plan
	before, after, count int
	uses, affected       []Reference
}

func (p *EntryPlan) Before() int           { return p.before }
func (p *EntryPlan) After() int            { return p.after }
func (p *EntryPlan) Count() int            { return p.count }
func (p *EntryPlan) Size() int             { return p.commit.Size() }
func (p *EntryPlan) Uses() []Reference     { return slices.Clone(p.uses) }
func (p *EntryPlan) Affected() []Reference { return slices.Clone(p.affected) }

func (p *EntryPlan) Current(
	s *Editor,
) bool {
	return p != nil && p.commit.Current(s.context)
}

func (s *Editor) Plan(d *EntryDraft) (*EntryPlan, error) {
	if d == nil {
		return nil, rom.ErrStale
	}
	if err := d.binding.Check(s.context); err != nil {
		return nil, err
	}
	s.Refresh()
	if err := s.context.Validate(s.image.Data); err != nil {
		return nil, err
	}
	if !d.Modified() {
		return nil, edit.ErrValue
	}
	if s.source.ProtectPointer == nil {
		return nil, edit.ErrLayout
	}
	if err := s.source.ProtectPointer(
		s.image.Data,
		d.entry.Key.Owner(),
		d.entry.Pointer,
	); err != nil {
		return nil, err
	}
	appender := space.NewAppender(len(s.image.Data))
	after, err := appender.Add(d.inventory.Payload(), d.entry.Pointer)
	if err != nil {
		return nil, err
	}
	result := &EntryPlan{
		commit: edit.NewPlan(
			d.binding,
			"Edit shop inventory",
			appender.Plan().Edit,
		),
		before: d.entry.Address,
		after:  after,
		count:  d.inventory.Count(),
	}
	for _, entry := range s.entries {
		if d.entry.Address != 0 && entry.Address == d.entry.Address {
			result.uses = append(result.uses, entry.Reference)
		}
		if entry.Pointer == d.entry.Pointer ||
			(d.entry.Key.Origin == Default && entry.UsesDefault) {
			result.affected = append(result.affected, entry.Reference)
		}
	}
	return result, nil
}

func (s *Editor) Apply(plan *EntryPlan) error {
	if plan == nil {
		return rom.ErrStale
	}
	return plan.commit.Apply(s.context)
}
