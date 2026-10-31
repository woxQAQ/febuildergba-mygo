package game_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

func TestSessionEdits(t *testing.T) {
	for _, fixture := range testutil.Games() {
		t.Run(fixture.Name, func(t *testing.T) {
			s := testutil.Open(t, fixture.Build())
			if s.Version() != fixture.Name {
				t.Fatalf("version %q", s.Version())
			}
			for _, table := range s.Records.Tables() {
				if table.Err != nil || table.Count < 2 {
					t.Fatalf(
						"%s: count %d, error %v",
						table.Kind,
						table.Count,
						table.Err,
					)
				}
			}
			before := s.Document.Snapshot()
			if err := s.Records.Set(
				records.Units,
				[]int{1, 2},
				"base.hp",
				-12,
			); err != nil {
				t.Fatal(err)
			}
			for _, id := range []int{1, 2} {
				value, err := s.Records.Value(records.Units, id, "base.hp")
				if err != nil || value != -12 {
					t.Fatalf("HP for %d: %d, %v", id, value, err)
				}
				row := s.Records.Table(records.Units).Rows[id-1]
				if s.Document.Snapshot().Data[row.Address+12] != 0xF4 {
					t.Fatal("field value did not reach document bytes")
				}
			}
			if !s.Document.Undo() || s.Document.State().CanUndo ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("batch did not undo as one complete operation")
			}
			s.Refresh()
			if !s.Document.Redo() {
				t.Fatal("cannot redo batch")
			}
			s.Refresh()
			if err := s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(s.Document.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			if value, err := reopened.Records.Value(
				records.Units,
				2,
				"base.hp",
			); err != nil ||
				value != -12 {
				t.Fatalf("saved HP: %d, %v", value, err)
			}
			stable := s.Document.Snapshot()
			state := s.Document.State()
			for _, sample := range []struct {
				ids    []int
				value  int64
				code   fault.Code
				record int
			}{
				{[]int{1, 999}, 20, edit.ErrField, 999},
				{[]int{1, 2}, 128, edit.ErrValue, 1},
			} {
				err := s.Records.Set(
					records.Units,
					sample.ids,
					"base.hp",
					sample.value,
				)
				frame, ok := errors.AsType[*fault.Error](err)
				if !ok || fault.CodeOf(err) != sample.code ||
					!errors.Is(
						err,
						sample.code,
					) || frame.Code.Kind != fault.InvalidInput ||
					frame.Op != "game.set" || frame.Context.Table != string(records.Units) ||
					frame.Context.Field != "base.hp" || frame.Context.RecordID == nil ||
					*frame.Context.RecordID != sample.record || frame.Context.Value == nil ||
					*frame.Context.Value != sample.value {
					t.Fatalf("missing failed batch context: %v", err)
				}
				if sample.code == edit.ErrValue &&
					(frame.Context.Bounds == nil ||
						*frame.Context.Bounds != (fault.Bounds{Min: -128, Max: 127})) {
					t.Fatalf("missing field bounds: %v", err)
				}
				if s.Document.State() != state ||
					!bytes.Equal(stable.Data, s.Document.Snapshot().Data) {
					t.Fatal("failed batch changed bytes, history or save state")
				}
			}
			region, err := s.Bytes.Read(0x200, 1)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Document.Apply(
				stable.Revision,
				"external edit",
				[]rom.Change{{Offset: 0x200, Data: []byte{1}}},
			); err != nil {
				t.Fatal(err)
			}
			stable, state = s.Document.Snapshot(), s.Document.State()
			if err := s.Records.Set(
				records.Units,
				[]int{1},
				"base.hp",
				20,
			); !errors.Is(
				err,
				rom.ErrStale,
			) {
				t.Fatalf("stale session: %v", err)
			}
			err = s.Bytes.Apply(region, []byte{0xAA})
			if fault.CodeOf(err) != rom.ErrStale ||
				!errors.Is(err, rom.ErrStale) {
				t.Fatalf("stale draft is not a conflict: %v", err)
			}
			var revisions *fault.Revision
			for frame := range fault.Frames(err) {
				if frame.Context.Revision != nil {
					revisions = frame.Context.Revision
				}
			}
			if revisions == nil || revisions.Expected != region.Revision() ||
				revisions.Actual != stable.Revision {
				t.Fatalf("missing revision evidence: %v", err)
			}
			if s.Document.State() != state ||
				!bytes.Equal(stable.Data, s.Document.Snapshot().Data) {
				t.Fatal("stale draft changed bytes, history or save state")
			}
			region, err = s.Bytes.Read(0x200, 1)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Bytes.Apply(region, []byte{0xAA}); err != nil {
				t.Fatalf("fresh draft could not recover: %v", err)
			}
			expected := bytes.Clone(stable.Data)
			expected[0x200] = 0xAA
			if !bytes.Equal(expected, s.Document.Snapshot().Data) {
				t.Fatal("recovered edit lost bytes outside the draft")
			}
			if !s.Document.Undo() ||
				!bytes.Equal(stable.Data, s.Document.Snapshot().Data) {
				t.Fatal("recovered edit did not undo as one operation")
			}
			s.Refresh()
			if err := s.Records.Set(
				records.Units,
				[]int{1},
				"base.hp",
				20,
			); err != nil {
				t.Fatalf("refreshed session could not recover: %v", err)
			}
			if value, err := s.Records.Value(
				records.Units,
				1,
				"base.hp",
			); err != nil ||
				value != 20 {
				t.Fatalf("recovered HP: %d, %v", value, err)
			}
		})
	}
}
