package game_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
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
			for _, edit := range []struct {
				ids   []int
				value int64
			}{
				{[]int{1, 999}, 20}, {[]int{1, 2}, 128},
			} {
				if err := s.Records.Set(
					records.Units,
					edit.ids,
					"base.hp",
					edit.value,
				); err == nil {
					t.Fatal("invalid batch accepted")
				}
				if s.Document.State().Revision != stable.Revision ||
					!bytes.Equal(stable.Data, s.Document.Snapshot().Data) {
					t.Fatal("failed batch changed document")
				}
			}
			if err := s.Document.Apply(
				stable.Revision,
				"external edit",
				[]rom.Change{{Offset: 0x200, Data: []byte{1}}},
			); err != nil {
				t.Fatal(err)
			}
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
		})
	}
}
