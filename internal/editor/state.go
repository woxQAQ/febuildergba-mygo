// Package editor owns window editing state shared by pages and controls.
package editor

import (
	"log/slog"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/applog"
)

// History contains the commands available for one editing target. A nil command
// is unavailable; an empty draft history never falls through to ROM history.
type History struct {
	Undo, Redo func()
}

// HistoryClaim declares which history owns undo and redo while the declaring
// control holds the focus. It is a routing declaration for the current build,
// not a focus query and not a command source: controls claim a target only
// when focused, and the workspace resolves the effective TextFocused once per
// build from the claim and the framework's text-focus report.
type HistoryClaim int

const (
	// HistoryClaimNone leaves routing to the framework's text-focus query.
	HistoryClaimNone HistoryClaim = iota
	// HistoryClaimText keeps undo and redo with the focused control's native
	// text history.
	HistoryClaimText
	// HistoryClaimDocument routes undo and redo to the ROM document history
	// even while a text-taking control holds the focus.
	HistoryClaimDocument
)

// documentState is shared by the workspace and its tool windows. All writes
// remain on the UI thread; each State keeps its own focus and draft routing.
type documentState struct {
	Session        *game.Session
	Busy, External bool
	Err            error
	HistoryEpoch   uint64
	// Changed invalidates windows displaying this document on the UI thread.
	Changed    func()
	loggedCode fault.Code
	loggedText string
}

// State owns one window's history routing over a shared document state.
type State struct {
	*documentState

	// Focus and draft commands are recomputed by the visible page each build.
	TextFocused  bool
	HistoryClaim HistoryClaim
	DraftHistory *History
}

func New() *State {
	return &State{documentState: &documentState{}}
}

// NewView shares the document and operation state with a separate window.
func (s *State) NewView() *State {
	return &State{documentState: s.documentState}
}

func (s *State) notify() {
	if s.Changed != nil {
		s.Changed()
	}
}

func (s *State) SetBusy(busy bool) {
	if s.Busy != busy {
		s.Busy = busy
		s.notify()
	}
}

func (s *State) SetExternal(external bool) {
	if s.External != external {
		s.External = external
		s.notify()
	}
}

func (s *State) SetSession(session *game.Session) {
	s.Session = session
	s.SetErr(nil)
	s.External = false
	s.TextFocused = false
	s.HistoryClaim = HistoryClaimNone
	s.DraftHistory = nil
	s.HistoryEpoch++
	s.notify()
}

// Report records a changed diagnostic without changing the page's presentation.
// Callers retain local draft errors; SetErr also updates the window banner.
func (s *State) Report(operation string, err error) {
	if err == nil {
		return
	}
	code, diagnostic := fault.CodeOf(err), err.Error()
	if code != s.loggedCode || diagnostic != s.loggedText {
		applog.RecordError(operation, err)
		s.loggedCode, s.loggedText = code, diagnostic
	}
}

func (s *State) SetErr(err error) {
	s.Err = err
	if err == nil {
		s.loggedCode, s.loggedText = fault.Code{}, ""
	}
	s.Report("editor.operation", err)
	s.notify()
}

// Edit observes a synchronous domain commit without moving logging into the
// domain. No-op writes and failed plans produce no history event.
func (s *State) Edit(work func() error) error {
	before := s.Session.Document.State()
	err := work()
	after := s.Session.Document.State()
	if err == nil && after.Revision != before.Revision {
		slog.Debug(
			"document history",
			"action",
			"apply",
			"label",
			after.UndoLabel,
			"revision",
			after.Revision,
			"size",
			after.Size,
		)
	}
	s.Report("editor.operation", err)
	s.notify()
	return err
}

// ResolveHistoryFocus settles the build's history routing from the declared
// claim and the framework's text-focus report. Claims outrank the query, so an
// immediate-commit field keeps document history and a declared text draft
// keeps native text history. History, UsesTextHistory and the menu snapshot
// all read the resolved TextFocused.
func (s *State) ResolveHistoryFocus(takesText bool) {
	s.TextFocused = takesText
	switch s.HistoryClaim {
	case HistoryClaimText:
		s.TextFocused = true
	case HistoryClaimDocument:
		s.TextFocused = false
	}
}

// History selects the same target for menus and keyboard commands. Native text
// history is handled by MyGo; pages claim draft history while their editor has
// focus, including when the draft is stale or its history is empty.
func (s *State) History() History {
	if s.Busy || s.TextFocused || s.Session == nil {
		return History{}
	}
	if s.DraftHistory != nil {
		return *s.DraftHistory
	}
	document := s.Session.Document.State()
	var history History
	if document.CanUndo {
		history.Undo = func() {
			label := s.Session.Document.State().UndoLabel
			if s.Session.Document.Undo() {
				s.HistoryEpoch++
				s.historyEvent("undo", label)
			}
			s.Session.Refresh()
			s.SetErr(nil)
		}
	}
	if document.CanRedo {
		history.Redo = func() {
			label := s.Session.Document.State().RedoLabel
			if s.Session.Document.Redo() {
				s.HistoryEpoch++
				s.historyEvent("redo", label)
			}
			s.Session.Refresh()
			s.SetErr(nil)
		}
	}
	return history
}

func (s *State) historyEvent(action, label string) {
	document := s.Session.Document.State()
	slog.Debug("document history", "action", action, "label", label,
		"revision", document.Revision, "size", document.Size)
}

func (s *State) Undo() {
	if undo := s.History().Undo; undo != nil {
		undo()
	}
}

func (s *State) Redo() {
	if redo := s.History().Redo; redo != nil {
		redo()
	}
}
