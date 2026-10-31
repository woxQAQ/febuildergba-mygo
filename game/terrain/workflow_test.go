package terrain_test

import (
	"bytes"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/terrain"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func TestClassTerrainWorkflow(t *testing.T) {
	for _, version := range []string{"FE6", "FE7U", "FE7J", "FE8U", "FE8J"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewTerrainFixture(version)
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			d := draft(t, s, 1)
			count, columns := 65, 6
			if version == "FE6" {
				count, columns = 51, 4
			}
			if len(d.Tables()) != columns ||
				len(table(t, d, terrain.Normal).Values) != count {
				t.Fatal("wrong native dimensions")
			}
			if !slices.Equal(
				table(t, d, terrain.Normal).Values[:5],
				[]int{-1, 1, 0, -128, -2},
			) {
				t.Fatal("signed or special values lost")
			}
			if table(t, d, terrain.Resistance).Err == nil {
				t.Fatal("invalid column accepted")
			}
			if err := d.Set(
				terrain.Resistance,
				0,
				1,
			); !errors.Is(
				err,
				terrain.ErrTable,
			) {
				t.Fatal("invalid column edit", err)
			}
			if version == "FE6" {
				if err := d.Set(
					terrain.Rain,
					0,
					1,
				); !errors.Is(
					err,
					edit.ErrField,
				) {
					t.Fatal("FE6 weather", err)
				}
			} else {
				if !table(t, d, terrain.Snow).Empty {
					t.Fatal("null pointer not distinguished")
				}
				if err := d.Set(
					terrain.Snow,
					0,
					1,
				); !errors.Is(
					err,
					terrain.ErrTable,
				) {
					t.Fatal("empty column edit", err)
				}
			}
			for _, edit := range []struct {
				kind      terrain.Kind
				id, value int
			}{{terrain.Normal, 12, 3}, {terrain.Avoid, 31, -3}} {
				if err := d.Set(edit.kind, edit.id, edit.value); err != nil {
					t.Fatal(err)
				}
			}
			plan, err := s.Terrain.Plan(d)
			if err != nil {
				t.Fatal(err)
			}
			moves := plan.Moves()
			if len(moves) != 2 ||
				moves[0].Pointer != f.GetPosition("terrain_normal_pointer") ||
				moves[1].Pointer != f.GetPosition("terrain_avoid_pointer") {
				t.Fatal("wrong entry selection", moves)
			}
			shared := moves[0].Shared
			if !slices.ContainsFunc(shared, func(ref terrain.Reference) bool {
				return ref.Class.ID == 0 &&
					ref.Address == f.GetPosition("terrain_movement")+1
			}) ||
				!slices.ContainsFunc(
					shared,
					func(ref terrain.Reference) bool { return ref.Class.ID == 2 && ref.Kind == terrain.Normal },
				) {
				t.Fatal("missing shared or overlapping entries", shared)
			}
			if version != "FE6" &&
				!slices.ContainsFunc(
					shared,
					func(ref terrain.Reference) bool { return ref.Class.ID == 1 && ref.Kind == terrain.Rain },
				) {
				t.Fatal("missing own weather alias")
			}
			unchanged(t, s, before)
			moves[0].Shared[0].Class.ID = 999
			moves[0].Changes[0].After = 100
			copyTables := d.Tables()
			copyTables[0].Values[12] = 101
			if err := d.Set(terrain.Normal, 12, 99); err != nil {
				t.Fatal(err)
			}
			if err := s.Terrain.Apply(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			prefix := bytes.Clone(before.Data)
			for _, move := range plan.Moves() {
				target, ok := gba.Pointer(after.Data, move.Pointer)
				if !ok || target != move.Destination ||
					target < len(before.Data) {
					t.Fatal("wrong destination")
				}
				copy(
					prefix[move.Pointer:move.Pointer+4],
					after.Data[move.Pointer:move.Pointer+4],
				)
				expected := bytes.Clone(
					before.Data[move.Address : move.Address+count],
				)
				if move.Kind == terrain.Normal {
					expected[12] = 3
				} else {
					expected[31] = 0xFD
				}
				if !bytes.Equal(expected, after.Data[target:target+count]) {
					t.Fatal("frozen payload or unselected values changed")
				}
			}
			if !bytes.Equal(prefix, after.Data[:len(prefix)]) {
				t.Fatal(
					"old table, tail, class bytes or independent pointer changed",
				)
			}
			if got := table(t, draft(t, s, 1), terrain.Normal).Values[12]; got != 3 {
				t.Fatal("plan changed with draft", got)
			}
			if err := s.Terrain.Apply(plan); !errors.Is(err, rom.ErrStale) {
				t.Fatal("plan reused", err)
			}
			if !s.Document.Undo() {
				t.Fatal("undo unavailable")
			}
			if !bytes.Equal(s.Document.Snapshot().Data, before.Data) {
				t.Fatal("undo did not restore pointers and size together")
			}
			if !s.Document.Redo() {
				t.Fatal("redo unavailable")
			}
			if !bytes.Equal(s.Document.Snapshot().Data, after.Data) {
				t.Fatal("redo differs")
			}
			path := filepath.Join(t.TempDir(), "terrain.gba")
			if err := s.Document.SaveAs(path, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			if table(t, draft(t, reopened, 1), terrain.Avoid).Values[31] != -3 ||
				table(t, draft(t, reopened, 2), terrain.Avoid).Values[31] != 30 {
				t.Fatal("reopen or shared owner changed")
			}
			next := draft(t, reopened, 1)
			if err := next.Set(terrain.Normal, 12, 4); err != nil {
				t.Fatal(err)
			}
			nextPlan, err := reopened.Terrain.Plan(next)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.Terrain.Apply(nextPlan); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSharedTerrainColumnsDiverge(t *testing.T) {
	s := testutil.Open(t, testutil.NewTerrainFixture("FE8U"))
	d := draft(t, s, 1)
	changes := []struct {
		kind  terrain.Kind
		value int
	}{{terrain.Normal, 3}, {terrain.Rain, 4}, {terrain.Avoid, -5}, {terrain.Defense, 6}}
	for _, change := range changes {
		if err := d.Set(change.kind, 12, change.value); err != nil {
			t.Fatal(err)
		}
	}
	plan, err := s.Terrain.Plan(d)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Terrain.Apply(plan); err != nil {
		t.Fatal(err)
	}
	saved := draft(t, s, 1)
	addresses := map[int]bool{}
	for _, change := range changes {
		actual := table(t, saved, change.kind)
		if actual.Values[12] != change.value || addresses[actual.Address] {
			t.Fatal("shared columns were merged or overwritten")
		}
		addresses[actual.Address] = true
	}
	independent := draft(t, s, 2)
	if table(t, independent, terrain.Normal).Values[12] != 2 ||
		table(t, independent, terrain.Avoid).Values[12] != 20 {
		t.Fatal("unselected class changed")
	}
}

func TestTerrainDraftConflictsAndLayouts(t *testing.T) {
	f := testutil.NewTerrainFixture("FE8U")
	s := testutil.Open(t, f)
	d := draft(t, s, 1)
	for _, input := range [][2]int{{-1, 0}, {65, 0}, {1, -129}, {1, 128}} {
		if err := d.Set(
			terrain.Normal,
			input[0],
			input[1],
		); !errors.Is(
			err,
			edit.ErrValue,
		) {
			t.Fatal("invalid value accepted", input, err)
		}
	}
	if d.Modified() {
		t.Fatal("invalid input changed draft")
	}
	if _, err := s.Terrain.Plan(d); !errors.Is(err, edit.ErrValue) {
		t.Fatal("empty plan", err)
	}
	if err := d.Set(terrain.Normal, 12, 4); err != nil {
		t.Fatal(err)
	}
	plan, err := s.Terrain.Plan(d)
	if err != nil {
		t.Fatal(err)
	}
	other := testutil.Open(t, f)
	before := other.Document.Snapshot()
	if err := other.Terrain.Apply(plan); !errors.Is(err, rom.ErrStale) {
		t.Fatal("cross-document plan", err)
	}
	if _, err := other.Terrain.Plan(d); !errors.Is(err, rom.ErrStale) {
		t.Fatal("cross-document draft", err)
	}
	unchanged(t, other, before)
	if err := s.Records.Set(records.Units, []int{1}, "level", 9); err != nil {
		t.Fatal(err)
	}
	before = s.Document.Snapshot()
	if _, err := s.Terrain.Plan(d); !errors.Is(err, rom.ErrStale) {
		t.Fatal("stale draft", err)
	}
	if err := s.Terrain.Apply(plan); !errors.Is(err, rom.ErrStale) {
		t.Fatal("stale plan", err)
	}
	unchanged(t, s, before)
	d = draft(t, s, 1)
	if err := d.Set(terrain.Normal, 12, 5); err != nil {
		t.Fatal(err)
	}
	plan, err = s.Terrain.Plan(d)
	if err != nil {
		t.Fatal(err)
	}
	layout := s.Records.Table(records.Classes).Layout
	if err := s.Records.UseLayout(records.Classes, layout); err != nil {
		t.Fatal(err)
	}
	if err := s.Terrain.Apply(plan); !errors.Is(err, rom.ErrStale) {
		t.Fatal("interpretation change accepted", err)
	}
	unchanged(t, s, before)

	// A manual class table at a different address owns only its own pointers.
	data := f.Bytes()
	manual := 0x340000
	f.WriteAt(
		manual,
		data[layout.Address:layout.Address+layout.Stride*layout.Count],
	)
	m := testutil.Open(t, f)
	layout.Address = manual
	if err := m.Records.UseLayout(records.Classes, layout); err != nil {
		t.Fatal(err)
	}
	d = draft(t, m, 1)
	if d.Class().Address != manual+84 {
		t.Fatal("manual record not used")
	}
	if err := d.Set(terrain.Normal, 12, 6); err != nil {
		t.Fatal(err)
	}
	plan, err = m.Terrain.Plan(d)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Terrain.Apply(plan); err != nil {
		t.Fatal(err)
	}
	after := m.Document.Snapshot()
	if gba.Uint32(
		after.Data,
		f.GetPosition("terrain_normal_pointer"),
	) != gba.Uint32(
		data,
		f.GetPosition("terrain_normal_pointer"),
	) {
		t.Fatal("automatic table pointer changed")
	}
	if table(t, draft(t, m, 1), terrain.Normal).Values[12] != 6 {
		t.Fatal("manual write missing")
	}
}

func TestTerrainConflictingResourcesAndCapacity(t *testing.T) {
	for _, mode := range []string{"own-payload", "independent-payload", "record-alias", "native-pointer-alias", "capacity"} {
		t.Run(mode, func(t *testing.T) {
			f := testutil.NewTerrainFixture("FE8U")
			site := f.GetPosition("terrain_normal_pointer")
			switch mode {
			case "own-payload":
				f.WritePointer(site, site-8)
			case "independent-payload":
				f.WritePointer(f.GetPosition("classes")+2*84+68, site-8)
			}
			s := testutil.Open(t, f)
			if mode == "record-alias" {
				if err := s.Records.UseLayout(
					records.Items,
					records.Layout{
						Address: f.GetPosition("terrain_class"),
						Stride:  84,
						Count:   1,
					},
				); err != nil {
					t.Fatal(err)
				}
			}
			id := 1
			if mode == "native-pointer-alias" {
				id = 0
				if err := s.Records.UseLayout(
					records.Classes,
					records.Layout{Address: 0x19458 - 56, Stride: 84, Count: 1},
				); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "capacity" {
				if err := s.Document.ApplyEdit(
					s.Document.State().Revision,
					"Fill capacity",
					rom.Edit{Size: rom.MaxSize},
				); err != nil {
					t.Fatal(err)
				}
			}
			d := draft(t, s, id)
			value := table(t, d, terrain.Normal).Values[12]
			if err := d.Set(terrain.Normal, 12, value^1); err != nil {
				t.Fatal(err)
			}
			before := s.Document.Snapshot()
			_, err := s.Terrain.Plan(d)
			wanted := error(terrain.ErrReferences)
			if mode == "record-alias" || mode == "native-pointer-alias" {
				wanted = edit.ErrLayout
			}
			if mode == "capacity" {
				wanted = space.ErrCapacity
			}
			if !errors.Is(err, wanted) {
				t.Fatalf("got %v, want %v", err, wanted)
			}
			unchanged(t, s, before)
		})
	}
}

func draft(t *testing.T, s *game.Session, id int) *terrain.Draft {
	t.Helper()
	d, err := s.Terrain.NewDraft(id)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func table(t *testing.T, d *terrain.Draft, kind terrain.Kind) terrain.Table {
	t.Helper()
	for _, table := range d.Tables() {
		if table.Kind == kind {
			return table
		}
	}
	t.Fatalf("missing table %s", kind)
	return terrain.Table{}
}

func unchanged(t *testing.T, s *game.Session, before rom.Snapshot) {
	t.Helper()
	after := s.Document.Snapshot()
	if before.Revision != after.Revision ||
		!bytes.Equal(before.Data, after.Data) {
		t.Fatal("failed operation or preview changed document")
	}
}
