package worldmap_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/shop"
	"github.com/woxQAQ/febuildergba-mygo/game/worldmap"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/romwrite"
)

func TestNativeNodeEditingAndHistory(t *testing.T) {
	for _, version := range []string{"FE8U", "FE8J"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewWorldMapFixture(version)
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			d, err := s.WorldMap.NewDraft(0)
			if err != nil {
				t.Fatal(err)
			}
			node := d.Node()
			for field, want := range map[worldmap.Field]int{
				worldmap.X: -2, worldmap.Y: 0x1234, worldmap.BranchFlag: -1,
				worldmap.EirikaNext: 2, worldmap.EphraimNext: 3,
				worldmap.EirikaAlternate: 4, worldmap.EphraimAlternate: -1,
				worldmap.ClearedIcon: 5, worldmap.UnclearedIcon: 15,
				worldmap.ShipTravel: 0,
			} {
				if node.Value(field) != want {
					t.Fatalf(
						"field %d = %d, want %d",
						field,
						node.Value(field),
						want,
					)
				}
			}
			for field, value := range map[worldmap.Field]int{
				worldmap.Name: 0, worldmap.X: -32768, worldmap.Y: 32767,
				worldmap.EirikaChapter: 1, worldmap.EphraimChapter: 0,
				worldmap.BranchFlag: 0x89, worldmap.EirikaNext: -1,
				worldmap.EphraimNext: 28, worldmap.EirikaAlternate: 0,
				worldmap.EphraimAlternate: 1, worldmap.ClearedIcon: 16,
				worldmap.UnclearedIcon: 6, worldmap.Placement: 1,
				worldmap.Encounters: 3, worldmap.ShipTravel: 1,
			} {
				if err := d.Set(field, value); err != nil {
					t.Fatal(err)
				}
			}
			plan, err := s.WorldMap.Plan(d)
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.Changes()) != 15 ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("preview changed ROM or missed fields")
			}
			// Plans and returned preview data remain independent from subsequent input.
			plan.Changes()[0].After = 123
			if err := d.Set(worldmap.X, 12); err != nil {
				t.Fatal(err)
			}
			if err := s.WorldMap.Apply(plan); err != nil {
				t.Fatal(err)
			}
			want := bytes.Clone(before.Data)
			at := f.GetPosition("worldmap_nodes")
			copy(
				want[at:at+12],
				[]byte{1, 3, 16, 6, 1, 0, 0x89, 0, 0xFF, 28, 0, 1},
			)
			copy(
				want[at+24:at+32],
				[]byte{0, 0x80, 0xFF, 0x7F, 0, 0, 0xA5, 0xD7},
			)
			if !bytes.Equal(want, s.Document.Snapshot().Data) {
				t.Fatal(
					"wrong native bytes, inventory pointer, reserved bits or unrelated data changed",
				)
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				s.Document.State().CanUndo {
				t.Fatal("undo was not one complete operation")
			}
			if !s.Document.Redo() ||
				!bytes.Equal(want, s.Document.Snapshot().Data) {
				t.Fatal("redo")
			}
			if err := s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(s.Document.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(want, reopened.Document.Snapshot().Data) ||
				reopened.Document.State().Dirty {
				t.Fatal("save/reopen")
			}
			saved, err := reopened.WorldMap.NewDraft(0)
			if err != nil || saved.Node().Value(worldmap.X) != -32768 {
				t.Fatal("reopened node", err)
			}
			uses, _ := reopened.Texts.Uses(1)
			for _, use := range uses {
				if use.Kind == worldmap.Resource && use.ID == 0 {
					t.Fatal("stale text usage")
				}
			}
			found := false
			uses, _ = s.Texts.Uses(0)
			for _, use := range uses {
				found = found || use.Kind == worldmap.Resource && use.ID == 0
			}
			if !found {
				t.Fatal("node name missing from shared text usage")
			}
		})
	}
}

func TestNodePositionEditingWithIndependentGraphics(t *testing.T) {
	for _, version := range []string{"FE8U", "FE8J"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewWorldMapBackgroundFixture(version)
			// The node map must not depend on a readable route table.
			f.WriteU32(f.GetPosition("worldmap_routes_pointer"), 0)
			s := testutil.Open(t, f)
			background, err := s.WorldMap.BackgroundImage(worldmap.Daytime)
			if err != nil || background == nil {
				t.Fatal("background with unavailable routes", err)
			}
			if cached, err := s.WorldMap.BackgroundImage(
				worldmap.Daytime,
			); err != nil ||
				cached != background {
				t.Fatal("background cache", err)
			}
			if dark, err := s.WorldMap.BackgroundImage(
				worldmap.Darkened,
			); err != nil ||
				dark == nil {
				t.Fatal("dark background", err)
			}
			before := s.Document.Snapshot()
			draft, err := s.WorldMap.NewDraft(0)
			if err != nil {
				t.Fatal(err)
			}
			if err := draft.Set(worldmap.X, 479); err != nil {
				t.Fatal(err)
			}
			if err := draft.Set(worldmap.Y, 319); err != nil {
				t.Fatal(err)
			}
			plan, err := s.WorldMap.Plan(draft)
			if err != nil || len(plan.Changes()) != 2 {
				t.Fatal("position preview", err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("preview changed the document")
			}
			if err := s.WorldMap.Apply(plan); err != nil {
				t.Fatal(err)
			}
			want := bytes.Clone(before.Data)
			at := f.GetPosition("worldmap_nodes") + 24
			copy(want[at:at+4], []byte{0xDF, 1, 0x3F, 1})
			if !bytes.Equal(want, s.Document.Snapshot().Data) {
				t.Fatal("position changed other node fields, shops or graphics")
			}
			if refreshed, err := s.WorldMap.BackgroundImage(
				worldmap.Daytime,
			); err != nil ||
				refreshed == background {
				t.Fatal("background outlived revision", err)
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				s.Document.State().CanUndo {
				t.Fatal("position undo was not one operation")
			}
			if !s.Document.Redo() ||
				!bytes.Equal(want, s.Document.Snapshot().Data) {
				t.Fatal("position redo")
			}
			if err := s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(s.Document.State().Path)
			if err != nil ||
				!bytes.Equal(want, reopened.Document.Snapshot().Data) {
				t.Fatal("position save/reopen", err)
			}
			// A broken background also leaves coordinate editing independent.
			f.WriteU8(f.GetPosition("worldmap_palette_map"), 0)
			s = testutil.Open(t, f)
			if _, err := s.WorldMap.BackgroundImage(
				worldmap.Daytime,
			); err == nil {
				t.Fatal("broken background accepted")
			}
			draft, err = s.WorldMap.NewDraft(0)
			if err != nil {
				t.Fatal(err)
			}
			if err := draft.Set(worldmap.X, -32768); err != nil {
				t.Fatal(err)
			}
			plan, err = s.WorldMap.Plan(draft)
			if err != nil {
				t.Fatal("graphics blocked coordinate editing", err)
			}
			if err := s.WorldMap.Apply(plan); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestNodeConflictAndReferenceFailuresAreAtomic(t *testing.T) {
	for _, scenario := range []string{"chapter", "text", "revision", "layout", "document", "record-overlap", "inventory-overlap", "root-overlap"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.NewWorldMapFixture("FE8U")
			at := f.GetPosition("worldmap_nodes")
			if scenario == "inventory-overlap" {
				f.WritePointer(at+12, at+28)
			}
			if scenario == "root-overlap" {
				f.WritePointer(
					f.GetPosition("worldmap_nodes_pointer"),
					f.TablePointers("units")[0]-24,
				)
			}
			s := testutil.Open(t, f)
			if scenario == "record-overlap" {
				if err := s.Records.UseLayout(
					records.Units,
					records.Layout{Address: at + 28, Stride: 52, Count: 1},
				); err != nil {
					t.Fatal(err)
				}
			}
			d, err := s.WorldMap.NewDraft(0)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Set(worldmap.X, 12); err != nil {
				t.Fatal(err)
			}
			var plan *worldmap.Plan
			want := error(romwrite.ErrClaim)
			switch scenario {
			case "chapter":
				want = edit.ErrValue
				if err := d.Set(worldmap.EirikaChapter, 255); err != nil {
					t.Fatal(err)
				}
			case "text":
				if err := d.Set(worldmap.Name, 65535); err != nil {
					t.Fatal(err)
				}
				want = nil
			case "revision", "layout", "document":
				plan, err = s.WorldMap.Plan(d)
				if err != nil {
					t.Fatal(err)
				}
				want = rom.ErrStale
			}
			switch scenario {
			case "revision":
				err = s.Records.Set(records.Units, []int{1}, "level", 9)
			case "layout":
				table := s.Records.Table(records.Chapters)
				err = s.Records.UseLayout(
					records.Chapters,
					records.Layout{
						Address: table.Address,
						Stride:  table.Stride,
						Count:   1,
					},
				)
			case "document":
				s = testutil.Open(t, f)
			}
			if err != nil {
				t.Fatal(err)
			}
			before, state := s.Document.Snapshot(), s.Document.State()
			if plan != nil {
				if _, err := s.WorldMap.Plan(d); !errors.Is(err, rom.ErrStale) {
					t.Fatal("stale draft accepted", err)
				}
				err = s.WorldMap.Apply(plan)
			} else {
				_, err = s.WorldMap.Plan(d)
			}
			if err == nil || want != nil && !errors.Is(err, want) {
				t.Fatalf("got %v, want %v", err, want)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				state != s.Document.State() {
				t.Fatal("failure mutated bytes/history")
			}
		})
	}
}

func TestNodeAndShopDraftIsolation(t *testing.T) {
	for _, shopFirst := range []bool{false, true} {
		f := testutil.NewWorldMapFixture("FE8J")
		s := testutil.Open(t, f)
		d, err := s.WorldMap.NewDraft(0)
		if err != nil {
			t.Fatal(err)
		}
		if err := d.Set(worldmap.X, 47); err != nil {
			t.Fatal(err)
		}
		nodePlan, err := s.WorldMap.Plan(d)
		if err != nil {
			t.Fatal(err)
		}
		stock, err := s.Shops.NewDraft(
			shop.Key{Origin: shop.WorldMap, ID: 0, Slot: 0},
		)
		if err != nil {
			t.Fatal(err)
		}
		stock.Clear()
		shopPlan, err := s.Shops.Plan(stock)
		if err != nil {
			t.Fatal(err)
		}
		if shopFirst {
			if err := s.Shops.Apply(shopPlan); err != nil {
				t.Fatal(err)
			}
			err = s.WorldMap.Apply(nodePlan)
		} else {
			if err := s.WorldMap.Apply(nodePlan); err != nil {
				t.Fatal(err)
			}
			err = s.Shops.Apply(shopPlan)
		}
		if !errors.Is(err, rom.ErrStale) {
			t.Fatal("cross-editor plan not invalidated", err)
		}
		fresh, err := s.WorldMap.NewDraft(0)
		if err != nil {
			t.Fatal(err)
		}
		if err := fresh.Set(worldmap.Y, 55); err != nil {
			t.Fatal(err)
		}
		plan, err := s.WorldMap.Plan(fresh)
		if err != nil {
			t.Fatal("hosted shop claims blocked node", err)
		}
		before := s.Document.Snapshot()
		if err := s.WorldMap.Apply(plan); err != nil {
			t.Fatal(err)
		}
		at := f.GetPosition("worldmap_nodes")
		if !bytes.Equal(
			before.Data[at+12:at+24],
			s.Document.Snapshot().Data[at+12:at+24],
		) {
			t.Fatal("overwrote stock pointers")
		}
	}
}

func TestNodeBoundsAndUnreadableInputs(t *testing.T) {
	for _, version := range testutil.Games() {
		t.Run(version.Name, func(t *testing.T) {
			f := testutil.NewWorldMapFixture(version.Name)
			s := testutil.Open(t, f)
			if version.Name != "FE8U" && version.Name != "FE8J" {
				if s.WorldMap.Available() {
					t.Fatal("unavailable format exposed")
				}
				if _, err := s.WorldMap.NewDraft(
					0,
				); !errors.Is(
					err,
					edit.ErrValue,
				) {
					t.Fatal(err)
				}
				return
			}
			d, err := s.WorldMap.NewDraft(0)
			if err != nil {
				t.Fatal(err)
			}
			for _, change := range []struct {
				field worldmap.Field
				value int
			}{
				{worldmap.X, -32769},
				{worldmap.Y, 32768},
				{worldmap.EirikaNext, -2},
				{worldmap.EphraimAlternate, 29},
				{worldmap.ClearedIcon, 17},
				{worldmap.Placement, 4},
				{worldmap.Encounters, 0},
				{worldmap.ShipTravel, 2},
				{worldmap.Field(255), 0},
			} {
				if err := d.Set(
					change.field,
					change.value,
				); !errors.Is(
					err,
					edit.ErrValue,
				) {
					t.Fatal("invalid input", change, err)
				}
			}
			if d.Modified() || s.Document.State().CanUndo {
				t.Fatal("invalid inputs changed state")
			}
			// Another node intentionally contains unknown icon and flag values.
			unknown, err := s.WorldMap.NewDraft(1)
			if err != nil {
				t.Fatal(err)
			}
			if err := unknown.Set(worldmap.X, 0); err != nil {
				t.Fatal(err)
			}
			plan, err := s.WorldMap.Plan(unknown)
			if err != nil {
				t.Fatal("unrelated unknown values blocked edit", err)
			}
			if err := s.WorldMap.Apply(plan); err != nil {
				t.Fatal(err)
			}
			for _, pointer := range []int{0, 0x80, len(f.Bytes()) - 4, f.GetPosition("worldmap_nodes") + 1} {
				f.WritePointer(f.GetPosition("worldmap_nodes_pointer"), pointer)
				broken := testutil.Open(t, f)
				if _, err := broken.WorldMap.NewDraft(
					0,
				); !errors.Is(
					err,
					worldmap.ErrNodes,
				) {
					t.Fatal("malformed root", pointer, err)
				}
				if err := broken.Records.Set(
					records.Units,
					[]int{1},
					"level",
					9,
				); err != nil {
					t.Fatal("node error blocked unrelated record", err)
				}
			}
		})
	}
}
