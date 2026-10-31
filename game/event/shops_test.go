package event_test

import (
	"bytes"
	"errors"
	"image"
	"path/filepath"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/shop"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func TestChapterShopWorkflow(t *testing.T) {
	for _, fixture := range testutil.Games() {
		t.Run(fixture.Name, func(t *testing.T) {
			f := testutil.NewShopFixture(fixture.Name)
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			d, err := s.Events.NewConditions(0, event.LocationEvents)
			if err != nil {
				t.Fatal(err)
			}
			original := d.Events()
			inventory, err := d.Shop(6)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(
				inventory.Items(),
				[]shop.Item{{ID: 1, Extra: 0xA1}, {ID: 2, Extra: 0xB2}},
			) ||
				d.Modified() {
				t.Fatal("native inventory interpretation")
			}
			if err := d.SetShopKind(6, event.SecretShop); err != nil {
				t.Fatal(err)
			}
			if err := d.SetLocationPosition(6, image.Pt(3, 4)); err != nil {
				t.Fatal(err)
			}
			if err := inventory.SetItem(0, 3); err != nil {
				t.Fatal(err)
			}
			if err := inventory.Move(0, 1); err != nil {
				t.Fatal(err)
			}
			if err := inventory.Add(4); err != nil {
				t.Fatal(err)
			}
			if err := inventory.SetExtra(2, 0x99); err != nil {
				t.Fatal(err)
			}
			if err := inventory.Remove(1); err != nil {
				t.Fatal(err)
			}
			if err := d.CopyLocation(6); err != nil {
				t.Fatal(err)
			}
			copied, err := d.Shop(12)
			if err != nil {
				t.Fatal(err)
			}
			if err := copied.SetItem(0, 5); err != nil {
				t.Fatal(err)
			}
			if err := d.AddLocation(s.Events, event.Shop); err != nil {
				t.Fatal(err)
			}
			if err := d.SetShopKind(13, event.Vendor); err != nil {
				t.Fatal(err)
			}
			added, err := d.Shop(13)
			if err != nil {
				t.Fatal(err)
			}
			if err := added.Add(7); err != nil {
				t.Fatal(err)
			}
			plan, err := s.Events.PlanConditions(d)
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.Inventories()) != 3 ||
				!slices.Equal(plan.Uses(), []int{0}) {
				t.Fatal("inventory preview or independent header scope")
			}
			wantUses := []shop.Key{
				{Origin: shop.Chapter, ID: 0, Slot: 6},
				{Origin: shop.Chapter, ID: 0, Slot: 11},
				{Origin: shop.Chapter, ID: 1, Slot: 6},
				{Origin: shop.Chapter, ID: 1, Slot: 11},
			}
			sameUse := func(ref shop.Reference, key shop.Key) bool { return ref.Key == key }
			if !slices.EqualFunc(
				plan.Inventories()[0].Uses,
				wantUses,
				sameUse,
			) {
				t.Fatalf(
					"known inventory references: %+v",
					plan.Inventories()[0].Uses,
				)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("preview wrote ROM")
			}
			// Both the draft and returned preview details remain detached from the plan.
			if err := inventory.SetItem(0, 8); err != nil {
				t.Fatal(err)
			}
			plan.Inventories()[0].Uses[0].Key.ID = 99
			if !slices.EqualFunc(
				plan.Inventories()[0].Uses,
				wantUses,
				sameUse,
			) {
				t.Fatal("returned preview details expose the immutable plan")
			}
			if err := s.Events.ApplyConditions(plan); err != nil {
				t.Fatal(err)
			}
			result, err := s.Events.Conditions(0, event.LocationEvents)
			if err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			for index, want := range map[int][]byte{
				6:  {2, 0xB2, 4, 0x99, 0, 0xC3},
				12: {5, 0xB2, 4, 0x99, 0, 0xC3},
				13: {7, 0, 0, 0},
			} {
				at, ok := gba.Pointer(
					after.Data,
					result.Events[index].Address+4,
				)
				if !ok || at < len(before.Data) ||
					!gba.Within(after.Data, at, 42) ||
					!bytes.Equal(after.Data[at:at+len(want)], want) ||
					!bytes.Equal(
						after.Data[at+len(want):at+42],
						make([]byte, 42-len(want)),
					) {
					t.Fatalf(
						"inventory %d payload or native read extent",
						index,
					)
				}
			}
			wantLocation := bytes.Clone(original[6].Raw)
			copy(wantLocation[4:8], result.Events[6].Raw[4:8])
			wantLocation[8], wantLocation[9], wantLocation[10] = 3, 4, 0x15
			if fixture.Name == "FE8U" || fixture.Name == "FE8J" {
				wantLocation[10] = 0x18
			}
			if !bytes.Equal(result.Events[6].Raw, wantLocation) ||
				result.Events[13].Location.ShopKind != event.Vendor {
				t.Fatal(
					"native category, coordinates or ignored high byte changed",
				)
			}
			for i, condition := range original {
				if i != 6 && !bytes.Equal(condition.Raw, result.Events[i].Raw) {
					t.Fatalf("unselected location %d changed", i)
				}
			}
			alias, err := s.Events.Conditions(1, event.LocationEvents)
			if err != nil || alias.Address != f.GetPosition("location_table") {
				t.Fatal("independent header changed", err)
			}
			prefix := bytes.Clone(after.Data[:len(before.Data)])
			copy(
				prefix[f.GetPosition("deploy_header")+8:],
				before.Data[f.GetPosition("deploy_header")+8:f.GetPosition("deploy_header")+12],
			)
			if !bytes.Equal(prefix, before.Data) {
				t.Fatal("old inventory, tail or unrelated ROM bytes changed")
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				!s.Document.Redo() ||
				!bytes.Equal(after.Data, s.Document.Snapshot().Data) {
				t.Fatal("location and inventory history was not atomic")
			}
			path := filepath.Join(t.TempDir(), "shops.gba")
			if err := s.Document.SaveAs(path, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			again, err := reopened.Events.NewConditions(0, event.LocationEvents)
			if err != nil {
				t.Fatal(err)
			}
			saved, err := again.Shop(6)
			if err != nil ||
				!slices.Equal(
					saved.Items(),
					[]shop.Item{{ID: 2, Extra: 0xB2}, {ID: 4, Extra: 0x99}},
				) {
				t.Fatal("saved inventory", err)
			}
			if err := saved.Remove(0); err != nil {
				t.Fatal(err)
			}
			next, err := reopened.Events.PlanConditions(again)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.Events.ApplyConditions(next); err != nil {
				t.Fatal(err)
			}
			entries, err := reopened.Events.ScriptEntries(0)
			if err != nil {
				t.Fatal(err)
			}
			latest, err := reopened.Events.Conditions(0, event.LocationEvents)
			if err != nil {
				t.Fatal(err)
			}
			for _, script := range entries {
				for _, condition := range latest.Events {
					if condition.OperandKind == event.ShopTarget &&
						script.Pointer == condition.Address+4 {
						t.Fatal("shop entered script navigation")
					}
				}
			}
		})
	}
}

func TestShopCapacityAndDraftIsolation(t *testing.T) {
	s := testutil.Open(t, testutil.NewLocationFixture("FE8U"))
	d, err := s.Events.NewConditions(0, event.LocationEvents)
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := d.Shop(6)
	if err != nil {
		t.Fatal(err)
	}
	items := inventory.Items()
	items[0].ID = 99
	if inventory.Modified() {
		t.Fatal("returned items expose draft storage")
	}
	for _, operation := range []func() error{
		func() error { return inventory.Add(0) },
		func() error { return inventory.Add(256) },
		func() error { return inventory.SetItem(0, 0) },
		func() error { return inventory.SetExtra(0, 256) },
		func() error { return inventory.SetExtra(0, -1) },
		func() error { return inventory.Move(0, 2) },
		func() error { return inventory.Remove(-1) },
		func() error { return d.SetShopKind(6, 3) },
	} {
		if operation() == nil || d.Modified() {
			t.Fatal("invalid input changed draft")
		}
	}
	for inventory.Count() < 20 {
		if err := inventory.Add(0xFF); err != nil {
			t.Fatal(err)
		}
	}
	if err := inventory.Add(
		1,
	); !errors.Is(err, edit.ErrValue) ||
		inventory.Count() != 20 {
		t.Fatal("native inventory capacity", err)
	}
	plan, err := s.Events.PlanConditions(d)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(plan.Uses(), []int{0, 1}) {
		t.Fatal("shared header scope")
	}
	if err := s.Events.ApplyConditions(plan); err != nil {
		t.Fatal(err)
	}
	shared, err := s.Events.NewConditions(1, event.LocationEvents)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := shared.Shop(6)
	if err != nil || saved.Count() != 20 {
		t.Fatal("20-item shop did not survive shared header update", err)
	}
	payload := s.Document.Snapshot().Data[saved.Address() : saved.Address()+42]
	if payload[40] != 0 || payload[41] != 0xC3 {
		t.Fatal("maximum inventory lacks native terminator")
	}
	for saved.Count() > 0 {
		if err := saved.Remove(0); err != nil {
			t.Fatal(err)
		}
	}
	empty, err := s.Events.PlanConditions(shared)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Events.ApplyConditions(empty); err != nil {
		t.Fatal(err)
	}
	fresh, err := s.Events.NewConditions(0, event.LocationEvents)
	if err != nil {
		t.Fatal(err)
	}
	emptyInventory, err := fresh.Shop(6)
	if err != nil || emptyInventory.Count() != 0 {
		t.Fatal("empty shop lost its terminator", err)
	}
}

func TestShopInvalidResourcesRemainIsolated(t *testing.T) {
	for _, scenario := range []string{"null", "ram", "unaligned", "short", "unterminated"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.NewShopFixture("FE8U")
			site := f.GetPosition("location_table") + 6*12 + 4
			switch scenario {
			case "null":
				f.WriteU32(site, 0)
			case "ram":
				f.WriteU32(site, 0x02000000)
			case "unaligned":
				f.WritePointer(site, f.GetPosition("location_inventory")+1)
			case "short":
				f.WritePointer(site, len(f.Bytes())-2)
			case "unterminated":
				f.WriteAt(
					f.GetPosition("location_inventory"),
					bytes.Repeat([]byte{1, 0xA5}, 21),
				)
			}
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			d, err := s.Events.NewConditions(0, event.LocationEvents)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := d.Shop(
				6,
			); !errors.Is(err, shop.ErrInventory) ||
				d.Modified() {
				t.Fatal("malformed inventory accepted", err)
			}
			if err := d.SetLocationPosition(0, image.Pt(2, 3)); err != nil {
				t.Fatal(err)
			}
			plan, err := s.Events.PlanConditions(d)
			if err != nil {
				t.Fatal("unrelated location blocked by inventory", err)
			}
			if err := s.Events.ApplyConditions(plan); err != nil {
				t.Fatal(err)
			}
			result, err := s.Events.Conditions(0, event.LocationEvents)
			if err != nil ||
				!bytes.Equal(result.Events[6].Raw, before.Data[site-4:site+8]) {
				t.Fatal("malformed resource was rewritten", err)
			}
		})
	}
}

func TestShopConflictAndStorageProtection(t *testing.T) {
	for _, scenario := range []string{"revision", "layout", "document", "capacity", "read buffer", "pointer conflict"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.NewShopFixture("FE8U")
			s := testutil.Open(t, f)
			d, err := s.Events.NewConditions(0, event.LocationEvents)
			if err != nil {
				t.Fatal(err)
			}
			inventory, err := d.Shop(6)
			if err != nil {
				t.Fatal(err)
			}
			if err := inventory.Add(3); err != nil {
				t.Fatal(err)
			}
			plan, err := s.Events.PlanConditions(d)
			if err != nil {
				t.Fatal(err)
			}
			want := error(rom.ErrStale)
			switch scenario {
			case "revision":
				if err := s.Records.Set(
					records.Units,
					[]int{1},
					"base.hp",
					9,
				); err != nil {
					t.Fatal(err)
				}
			case "layout":
				if err := s.Records.UseLayout(
					records.Chapters,
					s.Records.Table(records.Chapters).Layout,
				); err != nil {
					t.Fatal(err)
				}
			case "document":
				s = testutil.Open(t, f)
			case "capacity":
				if err := s.Document.ApplyEdit(
					s.Document.State().Revision,
					"capacity",
					rom.Edit{Size: rom.MaxSize},
				); err != nil {
					t.Fatal(err)
				}
				d, err = s.Events.NewConditions(0, event.LocationEvents)
				if err != nil {
					t.Fatal(err)
				}
				inventory, err = d.Shop(6)
				if err != nil {
					t.Fatal(err)
				}
				if err := inventory.Add(3); err != nil {
					t.Fatal(err)
				}
				want = space.ErrCapacity
			case "read buffer":
				want = space.ErrRange
			case "pointer conflict":
				f.WritePointer(
					f.GetPosition("deploy_list")+4,
					f.GetPosition("items"),
				)
				f.WritePointer(
					f.GetPosition("items")+8,
					f.GetPosition("location_table"),
				)
				s = testutil.Open(t, f)
				d, err = s.Events.NewConditions(0, event.LocationEvents)
				if err != nil {
					t.Fatal(err)
				}
				inventory, err = d.Shop(6)
				if err != nil {
					t.Fatal(err)
				}
				if err := inventory.Add(3); err != nil {
					t.Fatal(err)
				}
				want = event.ErrEventConditions
			}
			before := s.Document.Snapshot()
			switch scenario {
			case "capacity", "pointer conflict":
				_, err = s.Events.PlanConditions(d)
			case "read buffer":
				at := f.GetPosition("location_inventory") + 20
				_, err = s.Records.PlanTableMove(
					records.Items,
					[]space.FreeRange{
						{
							Start:  at,
							End:    at + 0x100,
							Source: "shop loader read extent",
						},
					},
				)
			default:
				err = s.Events.ApplyConditions(plan)
			}
			if !errors.Is(err, want) ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				before.Revision != s.Document.State().Revision {
				t.Fatalf(
					"failed shop operation changed document: %v; want %v",
					err,
					want,
				)
			}
		})
	}
}
