package testutil_test

import (
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/internal/game"
	"github.com/woxQAQ/febuildergba-mygo/tests/testutil"
)

// Fixtures must exercise the real version, layout and field readers.
func TestGameFixtures(t *testing.T) {
	for _, tt := range []struct {
		id    string
		build func() *testutil.FixtureBuilder
	}{
		{"FE8U", testutil.NewMinimalFE8U},
		{"FE8J", testutil.NewMinimalFE8J},
		{"FE7U", testutil.NewMinimalFE7U},
		{"FE7J", testutil.NewMinimalFE7J},
		{"FE6", testutil.NewMinimalFE6},
	} {
		t.Run(tt.id, func(t *testing.T) {
			f := tt.build()
			path := filepath.Join(t.TempDir(), "fixture.gba")
			if err := f.Save(path); err != nil {
				t.Fatal(err)
			}
			s, err := game.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			if s.Adapter.ID() != tt.id {
				t.Fatalf("identified as %s", s.Adapter.ID())
			}
			for _, kind := range game.Kinds {
				table := s.Table(kind)
				if table.Err != nil || table.Address != f.GetPosition(string(kind)) {
					t.Fatalf("%s: address %X, error %v", kind, table.Address, table.Err)
				}
			}
			if value, err := s.Value(game.Units, 1, "base.hp"); err != nil || value != 16 {
				t.Fatalf("HP %d, error %v", value, err)
			}
			if s.Table(game.Classes).Count != 2 || s.Table(game.Items).Count != 2 {
				t.Fatal("class or item terminator was not recognized")
			}
		})
	}
}
