package components

import "github.com/woxQAQ/febuildergba-mygo/game"

// State is shared by a window's pages and components and mutated on the UI thread.
type State struct {
	Session        *game.Session
	Busy, External bool
	Err            error
	HistoryEpoch   uint64
	// TextFocused is recomputed each frame for unapplied text fields.
	TextFocused bool
}

func (s *State) SetSession(session *game.Session) {
	s.Session = session
	s.Err = nil
	s.External = false
	s.HistoryEpoch++
}

func (s *State) Undo() {
	if s.Session == nil || s.Busy {
		return
	}
	if s.Session.Document.Undo() {
		s.HistoryEpoch++
	}
	s.Session.Refresh()
	s.Err = nil
}

func (s *State) Redo() {
	if s.Session == nil || s.Busy {
		return
	}
	if s.Session.Document.Redo() {
		s.HistoryEpoch++
	}
	s.Session.Refresh()
	s.Err = nil
}
