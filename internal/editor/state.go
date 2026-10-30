// Package editor owns window editing state shared by pages and controls.
package editor

import "github.com/woxQAQ/febuildergba-mygo/game"

// History contains the commands available for one editing target. A nil command
// is unavailable; an empty draft history never falls through to ROM history.
type History struct {
	Undo, Redo func()
}

// State is shared by a window's pages and components and mutated on the UI
// thread.
type State struct {
	Session        *game.Session
	Busy, External bool
	Err            error
	HistoryEpoch   uint64
	// Focus and draft commands are recomputed by the visible page each build.
	TextFocused  bool
	DraftHistory *History
}

func (s *State) SetSession(session *game.Session) {
	s.Session = session
	s.Err = nil
	s.External = false
	s.TextFocused = false
	s.DraftHistory = nil
	s.HistoryEpoch++
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
			if s.Session.Document.Undo() {
				s.HistoryEpoch++
			}
			s.Session.Refresh()
			s.Err = nil
		}
	}
	if document.CanRedo {
		history.Redo = func() {
			if s.Session.Document.Redo() {
				s.HistoryEpoch++
			}
			s.Session.Refresh()
			s.Err = nil
		}
	}
	return history
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
