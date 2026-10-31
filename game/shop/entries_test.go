package shop_test

import (
	"bytes"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/shop"
	"github.com/woxQAQ/febuildergba-mygo/game/worldmap"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func TestNativeShopWorkflow(t *testing.T) {
	for _, fixture := range testutil.Games() {
		t.Run(fixture.Name, func(t *testing.T) {
			for _, origin := range []shop.Origin{shop.Default, shop.Preparation, shop.WorldMap} {
				if origin != shop.Default && fixture.Name != "FE8U" &&
					fixture.Name != "FE8J" {
					continue
				}
				t.Run(
					map[shop.Origin]string{shop.Default: "default", shop.Preparation: "preparation", shop.WorldMap: "world-map"}[origin],
					func(t *testing.T) {
						s := testutil.Open(
							t,
							testutil.NewNativeShopFixture(fixture.Name),
						)
						before := s.Document.Snapshot()
						entries, _ := s.Shops.Entries()
						worldCount := 0
						for _, entry := range entries {
							if entry.Key.Origin == shop.WorldMap {
								worldCount++
							}
						}
						wantWorld := 0
						if fixture.Name == "FE8U" || fixture.Name == "FE8J" {
							wantWorld = 87
						}
						if worldCount != wantWorld {
							t.Fatal("native world-map slot count", worldCount)
						}
						key := shop.Key{Origin: origin}
						draft, err := s.Shops.NewDraft(key)
						if err != nil {
							t.Fatal(err)
						}
						inventory := draft.Inventory()
						if !slices.Equal(
							inventory.Items(),
							[]shop.Item{
								{ID: 1, Extra: 0xA1},
								{ID: 2, Extra: 0xB2},
							},
						) ||
							draft.Modified() {
							t.Fatal("native inventory or initial draft")
						}
						if err := inventory.SetItem(0, 7); err != nil {
							t.Fatal(err)
						}
						if err := inventory.Move(0, 1); err != nil {
							t.Fatal(err)
						}
						if err := inventory.Add(9); err != nil {
							t.Fatal(err)
						}
						if err := inventory.SetExtra(2, 0x88); err != nil {
							t.Fatal(err)
						}
						want := []shop.Item{
							{ID: 2, Extra: 0xB2},
							{ID: 7, Extra: 0xA1},
							{ID: 9, Extra: 0x88},
						}
						plan, err := s.Shops.Plan(draft)
						if err != nil {
							t.Fatal(err)
						}
						if len(plan.Uses()) < 5 ||
							plan.Before() != inventory.Address() ||
							plan.Count() != 3 {
							t.Fatal("known sharing or preview")
						}
						affected := plan.Affected()
						wantAffected := 1
						if origin == shop.Default {
							wantAffected = 3
							if fixture.Name == "FE8U" ||
								fixture.Name == "FE8J" {
								wantAffected++
							}
						}
						if len(affected) != wantAffected {
							t.Fatalf("affected entries: %+v", affected)
						}
						plan.Affected()[0].Key.ID = 99
						if !slices.Equal(affected, plan.Affected()) {
							t.Fatal("mutable preview references")
						}
						if !bytes.Equal(
							before.Data,
							s.Document.Snapshot().Data,
						) {
							t.Fatal("preview wrote ROM")
						}
						if err := inventory.SetItem(0, 10); err != nil {
							t.Fatal(err)
						}
						if err := s.Shops.Apply(plan); err != nil {
							t.Fatal(err)
						}
						after := s.Document.Snapshot()
						prefix := bytes.Clone(before.Data)
						site := draft.Entry().Pointer
						copy(prefix[site:site+4], after.Data[site:site+4])
						if !bytes.Equal(prefix, after.Data[:len(prefix)]) {
							t.Fatal(
								"changed old inventory, another reference or unknown node bytes",
							)
						}
						if len(after.Data) != plan.Size() ||
							plan.After() < len(before.Data) ||
							after.Data[plan.After()+7] != 0xC3 {
							t.Fatal("allocation or terminator preservation")
						}
						if !s.Document.Undo() ||
							!bytes.Equal(
								before.Data,
								s.Document.Snapshot().Data,
							) ||
							!s.Document.Redo() ||
							!bytes.Equal(
								after.Data,
								s.Document.Snapshot().Data,
							) {
							t.Fatal(
								"inventory, pointer and size did not share one history operation",
							)
						}
						path := filepath.Join(t.TempDir(), "shops.gba")
						if err := s.Document.SaveAs(path, false); err != nil {
							t.Fatal(err)
						}
						reopened, err := game.Open(path)
						if err != nil {
							t.Fatal(err)
						}
						saved, err := reopened.Shops.NewDraft(key)
						if err != nil ||
							!slices.Equal(saved.Inventory().Items(), want) {
							t.Fatal("save/reopen lost the frozen plan", err)
						}
						fallback, err := reopened.Events.NewConditions(
							0,
							event.LocationEvents,
						)
						if err != nil {
							t.Fatal(err)
						}
						fallbackInventory, err := fallback.Shop(11)
						if err != nil {
							t.Fatal(err)
						}
						if origin == shop.Default &&
							!slices.Equal(fallbackInventory.Items(), want) {
							t.Fatal(
								"chapter fallback did not follow the engine default",
							)
						}
						if origin != shop.Default &&
							fallbackInventory.Items()[0].ID != 1 {
							t.Fatal(
								"independent inventory changed default consumers",
							)
						}
					},
				)
			}
		})
	}
}

func TestWorldMapShopInitializationAndClosing(t *testing.T) {
	for _, version := range []string{"FE8J", "FE8U"} {
		t.Run(version, func(t *testing.T) {
			s := testutil.Open(t, testutil.NewNativeShopFixture(version))
			key := shop.Key{Origin: shop.WorldMap, Slot: 2}
			draft, err := s.Shops.NewDraft(key)
			if err != nil || draft.Inventory() != nil || !draft.Entry().Null ||
				draft.Modified() {
				t.Fatal("null slot must require initialization", err)
			}
			if _, err := s.Shops.Plan(draft); !errors.Is(err, edit.ErrValue) {
				t.Fatal("uninitialized null slot was writable", err)
			}
			draft.Clear()
			if !draft.Inventory().WorldMapHidden() {
				t.Fatal("initialized empty inventory must hide the node entry")
			}
			if err := draft.Inventory().Add(5); err != nil {
				t.Fatal(err)
			}
			if draft.Inventory().WorldMapHidden() {
				t.Fatal("adding stock did not enable the node entry")
			}
			plan, err := s.Shops.Plan(draft)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Shops.Apply(plan); err != nil {
				t.Fatal(err)
			}
			entries, _ := s.Shops.Entries()
			for _, entry := range entries {
				if entry.Key == key &&
					(entry.Null || entry.Empty || entry.Count != 1 || entry.Err != nil) {
					t.Fatal("initialized world-map slot", entry)
				}
			}
			// A nonzero terminator high byte is empty to the loader but visible to
			// the menu. Closing it must replace the first halfword with zero.
			draft, err = s.Shops.NewDraft(
				shop.Key{Origin: shop.WorldMap, ID: 1, Slot: 2},
			)
			if err != nil {
				t.Fatal(err)
			}
			if draft.Entry().Empty || draft.Inventory().Count() != 0 ||
				draft.Inventory().WorldMapHidden() {
				t.Fatal(
					"halfword availability was confused with item-ID termination",
					err,
				)
			}
			if err := draft.Inventory().Add(5); err != nil {
				t.Fatal(err)
			}
			if err := draft.Inventory().Remove(0); err != nil {
				t.Fatal(err)
			}
			if draft.Inventory().WorldMapHidden() || draft.Modified() {
				t.Fatal(
					"removing temporary stock changed the original menu visibility",
				)
			}
			draft.Clear()
			if !draft.Inventory().WorldMapHidden() {
				t.Fatal("closing did not update draft menu visibility")
			}
			plan, err = s.Shops.Plan(draft)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Shops.Apply(plan); err != nil {
				t.Fatal(err)
			}
			saved, err := s.Shops.NewDraft(draft.Entry().Key)
			if err != nil || !saved.Entry().Empty || saved.Entry().Null ||
				saved.Inventory().Count() != 0 {
				t.Fatal("closing must use a valid zero inventory", err)
			}
		})
	}
}

func TestNativeShopConflictsAndOwnerProtection(t *testing.T) {
	t.Run("stale-and-document", func(t *testing.T) {
		s := testutil.Open(t, testutil.NewNativeShopFixture("FE8U"))
		draft, err := s.Shops.NewDraft(shop.Key{Origin: shop.WorldMap})
		if err != nil {
			t.Fatal(err)
		}
		if err := draft.Inventory().Add(3); err != nil {
			t.Fatal(err)
		}
		plan, err := s.Shops.Plan(draft)
		if err != nil {
			t.Fatal(err)
		}
		other := testutil.Open(t, testutil.NewNativeShopFixture("FE8U"))
		before := other.Document.Snapshot()
		if err := other.Shops.Apply(
			plan,
		); !errors.Is(err, rom.ErrStale) ||
			!bytes.Equal(before.Data, other.Document.Snapshot().Data) {
			t.Fatal("cross-document edit was not atomic", err)
		}
		if err := s.Records.Set(
			records.Units,
			[]int{1},
			"base.hp",
			9,
		); err != nil {
			t.Fatal(err)
		}
		before = s.Document.Snapshot()
		if err := s.Shops.Apply(plan); !errors.Is(err, rom.ErrStale) {
			t.Fatal(err)
		}
		if _, err := s.Shops.Plan(
			draft,
		); !errors.Is(err, rom.ErrStale) ||
			!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
			t.Fatal("stale edit wrote bytes", err)
		}
	})
	t.Run("layout-conflict", func(t *testing.T) {
		s := testutil.Open(t, testutil.NewNativeShopFixture("FE8U"))
		draft, err := s.Shops.NewDraft(shop.Key{Origin: shop.Preparation})
		if err != nil {
			t.Fatal(err)
		}
		if err := draft.Inventory().Add(3); err != nil {
			t.Fatal(err)
		}
		plan, err := s.Shops.Plan(draft)
		if err != nil {
			t.Fatal(err)
		}
		layout := s.Records.Table(records.Items).Layout
		layout.Count--
		before := s.Document.Snapshot()
		if err := s.Records.UseLayout(records.Items, layout); err != nil {
			t.Fatal(err)
		}
		if err := s.Shops.Apply(plan); !errors.Is(err, rom.ErrStale) {
			t.Fatal("layout change accepted old preview", err)
		}
		if _, err := s.Shops.Plan(
			draft,
		); !errors.Is(err, rom.ErrStale) ||
			!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
			t.Fatal("layout change accepted old draft", err)
		}
	})
	t.Run("records-overlap", func(t *testing.T) {
		f := testutil.NewNativeShopFixture("FE8U")
		s := testutil.Open(t, f)
		table := s.Records.Table(records.Items)
		layout := records.Layout{
			Address: f.GetPosition("worldmap_nodes"),
			Stride:  table.Stride,
			Count:   1,
		}
		if err := s.Records.UseLayout(records.Items, layout); err != nil {
			t.Fatal(err)
		}
		draft, err := s.Shops.NewDraft(shop.Key{Origin: shop.WorldMap})
		if err != nil {
			t.Fatal(err)
		}
		if err := draft.Inventory().Add(3); err != nil {
			t.Fatal(err)
		}
		before := s.Document.Snapshot()
		if _, err := s.Shops.Plan(
			draft,
		); !errors.Is(err, edit.ErrLayout) ||
			!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
			t.Fatal("node pointer overwrote a declared table", err)
		}
	})
	t.Run("damaged-world-map", func(t *testing.T) {
		f := testutil.NewNativeShopFixture("FE8U")
		f.WritePointer(
			f.GetPosition("worldmap_nodes_pointer"),
			len(f.Bytes())-worldmap.NodeSize,
		)
		s := testutil.Open(t, f)
		entries, warnings := s.Shops.Entries()
		if !slices.ContainsFunc(
			warnings,
			func(err error) bool { return errors.Is(err, worldmap.ErrNodes) },
		) ||
			slices.ContainsFunc(
				entries,
				func(entry shop.Entry) bool { return entry.Key.Origin == shop.WorldMap },
			) {
			t.Fatal("truncated nodes were accepted")
		}
		draft, err := s.Shops.NewDraft(shop.Key{Origin: shop.Preparation})
		if err != nil {
			t.Fatal(err)
		}
		if err := draft.Inventory().Add(3); err != nil {
			t.Fatal(err)
		}
		plan, err := s.Shops.Plan(draft)
		if err != nil {
			t.Fatal("unrelated preparation shop was blocked", err)
		}
		if err := s.Shops.Apply(plan); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("damaged-inventory", func(t *testing.T) {
		f := testutil.NewNativeShopFixture("FE8U")
		f.WriteU32(f.GetPosition("worldmap_nodes")+12, 0x02000000)
		s := testutil.Open(t, f)
		before := s.Document.Snapshot()
		if _, err := s.Shops.NewDraft(
			shop.Key{Origin: shop.WorldMap},
		); !errors.Is(
			err,
			shop.ErrInventory,
		) {
			t.Fatal("RAM pointer became an empty inventory", err)
		}
		draft, err := s.Shops.NewDraft(shop.Key{Origin: shop.WorldMap, Slot: 1})
		if err != nil {
			t.Fatal(err)
		}
		if err := draft.Inventory().Add(3); err != nil {
			t.Fatal(err)
		}
		plan, err := s.Shops.Plan(draft)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Shops.Apply(plan); err != nil {
			t.Fatal(err)
		}
		after := s.Document.Snapshot()
		site := f.GetPosition("worldmap_nodes") + 12
		if !bytes.Equal(before.Data[site:site+4], after.Data[site:site+4]) {
			t.Fatal("unselected invalid pointer changed")
		}
	})
	t.Run("chapter-pointer-overlap", func(t *testing.T) {
		f := testutil.NewNativeShopFixture("FE8U")
		nodes := f.GetPosition("worldmap_nodes")
		f.WritePointer(f.GetPosition("deploy_list")+4, nodes-8)
		f.WritePointer(nodes, f.GetPosition("location_table"))
		s := testutil.Open(t, f)
		draft, err := s.Events.NewConditions(0, event.LocationEvents)
		if err != nil {
			t.Fatal(err)
		}
		inventory, err := draft.Shop(6)
		if err != nil {
			t.Fatal(err)
		}
		if err := inventory.Add(3); err != nil {
			t.Fatal(err)
		}
		before := s.Document.Snapshot()
		if _, err := s.Events.PlanConditions(
			draft,
		); err == nil ||
			!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
			t.Fatal("chapter pointer overwrote a world-map node", err)
		}
	})
	t.Run("capacity", func(t *testing.T) {
		f := testutil.NewNativeShopFixture("FE8U")
		s := testutil.Open(t, f)
		image := s.Document.Snapshot()
		if err := s.Document.ApplyEdit(
			image.Revision,
			"Fill address space",
			rom.Edit{Size: rom.MaxSize},
		); err != nil {
			t.Fatal(err)
		}
		draft, err := s.Shops.NewDraft(shop.Key{Origin: shop.Preparation})
		if err != nil {
			t.Fatal(err)
		}
		if err := draft.Inventory().Add(3); err != nil {
			t.Fatal(err)
		}
		before := s.Document.Snapshot()
		if _, err := s.Shops.Plan(
			draft,
		); !errors.Is(err, space.ErrCapacity) ||
			!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
			t.Fatal("capacity failure wrote bytes", err)
		}
	})
}

func TestChapterShopDefaultOverride(t *testing.T) {
	s := testutil.Open(t, testutil.NewNativeShopFixture("FE8J"))
	before := s.Document.Snapshot()
	draft, err := s.Events.NewConditions(0, event.LocationEvents)
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := draft.Shop(11)
	if err != nil {
		t.Fatal(err)
	}
	if err := inventory.SetItem(0, 8); err != nil {
		t.Fatal(err)
	}
	plan, err := s.Events.PlanConditions(draft)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Events.ApplyConditions(plan); err != nil {
		t.Fatal(err)
	}
	changed, err := s.Events.NewConditions(0, event.LocationEvents)
	if err != nil {
		t.Fatal(err)
	}
	stock, err := changed.Shop(11)
	if err != nil || stock.Items()[0].ID != 8 {
		t.Fatal("default override was not committed", err)
	}
	other, err := s.Events.NewConditions(1, event.LocationEvents)
	if err != nil {
		t.Fatal(err)
	}
	stock, err = other.Shop(11)
	if err != nil || stock.Items()[0].ID != 1 ||
		gba.Uint32(other.Events()[11].Raw, 4) != 0 {
		t.Fatal("independent null reference changed", err)
	}
	if !s.Document.Undo() ||
		!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
		t.Fatal("default override was not one undo")
	}
}
