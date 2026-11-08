package arena_test

import (
	"bytes"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/arena"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/romwrite"
)

func TestArenaNativeWorkflows(t *testing.T) {
	for _, version := range testutil.Games() {
		t.Run(version.Name, func(t *testing.T) {
			for _, group := range []arena.Kind{arena.Melee, arena.Ranged, arena.Magic, arena.BaseWeapons, arena.LinkUnits} {
				t.Run(string(rune('0'+group)), func(t *testing.T) {
					f := testutil.NewArenaFixture(version.Name)
					s := testutil.Open(t, f)
					if group == arena.LinkUnits &&
						!slices.Contains(s.Arena.Groups(), group) {
						if version.Name != "FE6" && version.Name != "FE7U" &&
							version.Name != "FE7J" {
							t.Fatal("missing link capability")
						}
						if _, err := s.Arena.NewDraft(
							group,
						); !errors.Is(
							err,
							edit.ErrValue,
						) {
							t.Fatal("unsupported link draft", err)
						}
						return
					}
					before := s.Document.Snapshot()
					d, err := s.Arena.NewDraft(group)
					if err != nil {
						t.Fatal(err)
					}
					expected := make(map[arena.Kind][]byte)
					switch {
					case group.ClassList():
						original := []byte{1, 2, 1, 0}
						if group == arena.Magic {
							original = []byte{2, 1, 0}
						}
						if !bytes.Equal(d.Raw(group), original) {
							t.Fatal(
								"weighted pool or byte alignment",
								d.Raw(group),
							)
						}
						keys := d.Rows(group)
						if err = d.Insert(group, 1, 3); err != nil {
							t.Fatal(err)
						}
						if err = d.Move(group, 1, 0); err != nil {
							t.Fatal(err)
						}
						if err = d.Delete(group, 1); err != nil {
							t.Fatal(err)
						}
						if group == arena.Magic {
							// Restore a promoted class after explicitly deleting it.
							if err = d.Insert(group, 2, 2); err != nil {
								t.Fatal(err)
							}
							expected[group] = []byte{3, 1, 2, 0}
						} else {
							expected[group] = []byte{3, 2, 1, 0}
						}
						if d.Rows(group)[1].Key != keys[1].Key {
							t.Fatal("row identity changed")
						}
					case group == arena.BaseWeapons:
						if len(d.Rows(arena.Upgrades)) != 26 {
							t.Fatal("upgrade buffer width")
						}
						if err = d.Set(arena.BaseWeapons, 0, 9); err != nil {
							t.Fatal(err)
						}
						if err = d.Set(arena.Upgrades, 0, 9); err != nil {
							t.Fatal(err)
						}
						expected[arena.BaseWeapons] = []byte{
							9,
							2,
							3,
							4,
							0,
							5,
							6,
							7,
						}
						expected[arena.Upgrades] = []byte{
							9,
							8,
							9,
							0,
							2,
							8,
							9,
							0,
							3,
							8,
							9,
							0,
							4,
							8,
							9,
							0,
							5,
							8,
							9,
							0,
							6,
							8,
							0,
							7,
							0,
							0xFF,
						}
					case group == arena.LinkUnits:
						rows := d.Rows(group)
						if len(rows) != 11 || rows[1].Value != 0 ||
							rows[2].Value != 2 ||
							rows[3].Value != 0x1234 {
							t.Fatal("fixed halfword slots", rows)
						}
						if err = d.Set(group, 10, 0); err != nil {
							t.Fatal(err)
						}
						expected[group] = []byte{
							1,
							0,
							0,
							0,
							2,
							0,
							0x34,
							0x12,
							3,
							0,
							4,
							0,
							5,
							0,
							6,
							0,
							7,
							0,
							8,
							0,
							0,
							0,
						}
					}
					plan, err := s.Arena.Plan(d)
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(s.Document.Snapshot().Data, before.Data) {
						t.Fatal("preview wrote ROM")
					}
					changes := plan.Changes()
					if group.ClassList() && len(changes[0].Shared) != 2 {
						t.Fatal(
							"shared and interior references missing",
							changes,
						)
					}
					// Mutation of drafts, previews and returned data cannot retarget a plan.
					raw := d.Raw(group)
					raw[0] = 0xCC
					changes[0].AfterBytes[0] = 0xCD
					changes[0].Pointers[0] = 0
					if group.ClassList() {
						if err = d.Set(group, 0, 1); err != nil {
							t.Fatal(err)
						}
					}
					if err = s.Arena.Apply(plan); err != nil {
						t.Fatal(err)
					}
					after := s.Document.Snapshot()
					prefix := bytes.Clone(before.Data)
					for _, change := range plan.Changes() {
						if !bytes.Equal(
							after.Data[change.Destination:change.Destination+len(change.AfterBytes)],
							expected[change.Kind],
						) {
							t.Fatal("native bytes", change.Kind)
						}
						for _, pointer := range change.Pointers {
							at, valid := gba.Pointer(after.Data, pointer)
							if !valid || at != change.Destination {
								t.Fatal("consumer not moved", pointer)
							}
							copy(
								prefix[pointer:pointer+4],
								after.Data[pointer:pointer+4],
							)
						}
					}
					if !bytes.Equal(prefix, after.Data[:len(prefix)]) {
						t.Fatal(
							"changed independent reference, source or unknown tail",
						)
					}
					if !s.Document.Undo() ||
						!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
						!s.Document.Redo() ||
						!bytes.Equal(after.Data, s.Document.Snapshot().Data) {
						t.Fatal("history not atomic")
					}
					path := filepath.Join(t.TempDir(), "arena.gba")
					if err = s.Document.SaveAs(path, false); err != nil {
						t.Fatal(err)
					}
					reopened, err := game.Open(path)
					if err != nil {
						t.Fatal(err)
					}
					saved, err := reopened.Arena.NewDraft(group)
					if err != nil {
						t.Fatal(err)
					}
					for kind, wanted := range expected {
						if !bytes.Equal(saved.Raw(kind), wanted) {
							t.Fatal("save/reopen", kind)
						}
					}
				})
			}
		})
	}
}

func TestArenaRejectsUnsafeChangesWithoutWriting(t *testing.T) {
	for _, scenario := range []string{"tier", "empty", "missing-class", "weapon-lookup", "weapon-marker", "invalid-item", "revision", "layout", "document", "overlap", "capacity"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.NewArenaFixture("FE8U")
			s := testutil.Open(t, f)
			group := arena.Melee
			if scenario == "weapon-lookup" || scenario == "weapon-marker" ||
				scenario == "invalid-item" {
				group = arena.BaseWeapons
			}
			d, err := s.Arena.NewDraft(group)
			if err != nil {
				t.Fatal(err)
			}
			want := error(arena.ErrClasses)
			switch scenario {
			case "tier":
				err = d.Set(group, 1, 1)
			case "empty":
				for len(d.Rows(group)) > 0 {
					if err = d.Delete(group, 0); err != nil {
						t.Fatal(err)
					}
				}
			case "missing-class":
				err = d.Set(group, 0, 250)
			case "weapon-lookup":
				err = d.Set(arena.Upgrades, 0, 9)
				want = arena.ErrWeapons
			case "invalid-item":
				err = d.Set(arena.Upgrades, 1, 200)
				want = edit.ErrValue
			case "weapon-marker":
				original := d.Raw(arena.Upgrades)
				for _, pair := range [][2]int{{3, 1}, {25, 1}, {1, 255}, {1, 0}} {
					if err = d.Set(
						arena.Upgrades,
						pair[0],
						pair[1],
					); !errors.Is(
						err,
						edit.ErrValue,
					) {
						t.Fatal("marker allowed", err)
					}
				}
				if !bytes.Equal(original, d.Raw(arena.Upgrades)) ||
					d.Modified() {
					t.Fatal("invalid operation changed draft")
				}
				return
			default:
				err = d.Set(group, 0, 3)
			}
			if err != nil {
				t.Fatal(err)
			}
			var plan *arena.Plan
			if scenario == "revision" || scenario == "layout" ||
				scenario == "document" {
				plan, err = s.Arena.Plan(d)
				if err != nil {
					t.Fatal(err)
				}
				want = rom.ErrStale
			}
			switch scenario {
			case "revision":
				err = s.Records.Set(records.Units, []int{1}, "level", 9)
			case "layout":
				table := s.Records.Table(records.Classes)
				err = s.Records.UseLayout(
					records.Classes,
					records.Layout{
						Address: table.Address,
						Stride:  table.Stride,
						Count:   4,
					},
				)
			case "document":
				s = testutil.Open(t, f)
			case "overlap":
				err = s.Records.UseLayout(
					records.Units,
					records.Layout{
						Address: f.TablePointers("arena_melee")[0],
						Stride:  52,
						Count:   1,
					},
				)
				want = arena.ErrReference
				if err == nil {
					d, err = s.Arena.NewDraft(group)
					if err == nil {
						err = d.Set(group, 0, 3)
					}
				}
			case "capacity":
				err = s.Document.ApplyEdit(
					s.Document.State().Revision,
					"fill",
					rom.Edit{Size: rom.MaxSize},
				)
				want = romwrite.ErrCapacity
				if err == nil {
					d, err = s.Arena.NewDraft(group)
					if err == nil {
						err = d.Set(group, 0, 3)
					}
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			before, state := s.Document.Snapshot(), s.Document.State()
			if plan != nil {
				err = s.Arena.Apply(plan)
			} else {
				_, err = s.Arena.Plan(d)
			}
			if !errors.Is(err, want) {
				t.Fatalf("got %v, want %v", err, want)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				s.Document.State() != state {
				t.Fatal("failure changed bytes or history")
			}
		})
	}
}

func TestArenaReadIsolationAndResourceClaims(t *testing.T) {
	for _, scenario := range []string{"split", "unterminated", "truncated", "upgrade-end", "claims"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.NewArenaFixture("FE8U")
			switch scenario {
			case "split":
				f.WritePointer(f.TablePointers("arena_base")[1], 0x290300)
			case "unterminated":
				f.WriteAt(
					f.GetPosition("arena_melee"),
					bytes.Repeat([]byte{1}, 257),
				)
			case "truncated":
				f.WritePointer(
					f.TablePointers("arena_link")[0],
					len(f.Bytes())-2,
				)
			case "upgrade-end":
				f.WriteU8(f.GetPosition("arena_upgrades")+25, 0)
			case "claims":
				// The arena class list covers a support pointer site. A support edit must
				// reject the overlap through the shared claim registry.
				f.WritePointer(0x28534, 0x290400)
				f.WriteAt(
					0x290400,
					[]byte{1, 1, 2, 3, 4, 5, 6, 0xA9, 0, 1, 2, 3, 4, 5, 6, 7},
				)
				f.WritePointer(f.TablePointers("arena_melee")[0], 0x28534)
			}
			s := testutil.Open(t, f)
			if scenario == "claims" {
				d, err := s.Supports.NewBonusDraft()
				if err != nil {
					t.Fatal(err)
				}
				if err = d.Set(0, 0, 9); err != nil {
					t.Fatal(err)
				}
				before := s.Document.State()
				if _, err = s.Supports.PlanBonuses(
					d,
				); !errors.Is(
					err,
					romwrite.ErrClaim,
				) {
					t.Fatal("arena resource not protected", err)
				}
				if s.Document.State() != before {
					t.Fatal("failed claim changed history")
				}
				return
			}
			group, want := arena.BaseWeapons, error(arena.ErrData)
			switch scenario {
			case "split":
				want = arena.ErrReference
			case "unterminated":
				group = arena.Melee
			case "truncated":
				group = arena.LinkUnits
			}
			before := s.Document.State()
			if _, err := s.Arena.NewDraft(group); !errors.Is(err, want) {
				t.Fatal("bad resource accepted", err)
			}
			other := arena.LinkUnits
			if group == arena.LinkUnits {
				other = arena.Melee
			}
			d, err := s.Arena.NewDraft(other)
			if err != nil {
				t.Fatal("unrelated group blocked", err)
			}
			if other == arena.LinkUnits {
				err = d.Set(other, 0, 0)
			} else {
				err = d.Set(other, 0, 3)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.Arena.Plan(d); err != nil {
				t.Fatal("unrelated edit blocked", err)
			}
			if s.Document.State() != before {
				t.Fatal("read/preview changed history")
			}
		})
	}
}
