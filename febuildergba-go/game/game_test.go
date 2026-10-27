package game

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

func gameFixtures() []struct {
	name  string
	build func() *testutil.FixtureBuilder
} {
	return []struct {
		name  string
		build func() *testutil.FixtureBuilder
	}{
		{"FE8U", testutil.NewMinimalFE8U},
		{"FE8J", testutil.NewMinimalFE8J},
		{"FE7U", testutil.NewMinimalFE7U},
		{"FE7J", testutil.NewMinimalFE7J},
		{"FE6", testutil.NewMinimalFE6},
	}
}

func openFixture(t *testing.T, fixture *testutil.FixtureBuilder) *Session {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.gba")
	if err := fixture.Save(path); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSessionEdits(t *testing.T) {
	for _, fixture := range gameFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			s := openFixture(t, fixture.build())
			if s.Adapter.ID() != fixture.name {
				t.Fatalf("version %q", s.Adapter.ID())
			}
			for _, table := range s.Tables {
				if table.Err != nil || table.Count < 2 {
					t.Fatalf("%s: count %d, error %v", table.Kind, table.Count, table.Err)
				}
			}
			before := s.Document.Snapshot()
			if err := s.Set(Units, []int{1, 2}, "base.hp", -12); err != nil {
				t.Fatal(err)
			}
			for _, id := range []int{1, 2} {
				value, err := s.Value(Units, id, "base.hp")
				if err != nil || value != -12 {
					t.Fatalf("HP for %d: %d, %v", id, value, err)
				}
				row := s.Table(Units).Rows[id-1]
				if s.Document.Snapshot().Data[row.Address+12] != 0xF4 {
					t.Fatal("field value did not reach document bytes")
				}
			}
			if !s.Document.Undo() || s.Document.State().CanUndo || !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
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
			reopened, err := Open(s.Document.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			if value, err := reopened.Value(Units, 2, "base.hp"); err != nil || value != -12 {
				t.Fatalf("saved HP: %d, %v", value, err)
			}
			stable := s.Document.Snapshot()
			for _, edit := range []struct {
				ids   []int
				value int64
			}{
				{[]int{1, 999}, 20}, {[]int{1, 2}, 128},
			} {
				if err := s.Set(Units, edit.ids, "base.hp", edit.value); err == nil {
					t.Fatal("invalid batch accepted")
				}
				if s.Document.State().Revision != stable.Revision || !bytes.Equal(stable.Data, s.Document.Snapshot().Data) {
					t.Fatal("failed batch changed document")
				}
			}
			if err := s.Document.Apply(stable.Revision, "external edit", []rom.Change{{Offset: 0x200, Data: []byte{1}}}); err != nil {
				t.Fatal(err)
			}
			if err := s.Set(Units, []int{1}, "base.hp", 20); !errors.Is(err, rom.ErrStale) {
				t.Fatalf("stale session: %v", err)
			}
		})
	}
}

func TestSessionLayoutValidation(t *testing.T) {
	s := openFixture(t, testutil.NewMinimalFE8U())
	original := s.Document.Snapshot()
	for _, layout := range []Layout{
		{Address: -4, Stride: 52, Count: 2},
		{Address: 0x200001, Stride: 52, Count: 2},
		{Address: 0x200000, Stride: 4, Count: 2},
		{Address: 0x200000, Stride: 53, Count: 2},
		{Address: 0x200000, Stride: 52, Count: 0},
		{Address: 0x200000, Stride: 52, Count: 256},
		{Address: len(original.Data) - 4, Stride: 52, Count: 2},
		{Address: 0x200000, Stride: int(^uint(0) >> 1), Count: 2},
	} {
		if err := s.UseLayout(Units, layout); !errors.Is(err, ErrLayout) {
			t.Errorf("layout %+v: %v", layout, err)
		}
	}
	if err := s.UseLayout(Units, Layout{Address: 0x200000, Stride: 52, Count: 2}); err != nil {
		t.Fatal(err)
	}
	if !s.Table(Units).Manual || s.Table(Units).Count != 2 || !bytes.Equal(original.Data, s.Document.Snapshot().Data) {
		t.Fatal("manual interpretation changed ROM bytes or ignored layout")
	}
	s.UseAutomaticLayout(Units)
	if s.Table(Units).Manual || s.Table(Units).Count != 255 {
		t.Fatal("automatic interpretation not restored")
	}
}
