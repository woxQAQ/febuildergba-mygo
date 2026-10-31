package quote_test

import (
	"bytes"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/quote"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/shop"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func TestQuoteTableWorkflow(t *testing.T) {
	for _, fixture := range testutil.Games() {
		t.Run(fixture.Name, func(t *testing.T) {
			kinds := []quote.Kind{
				quote.Battle,
				quote.General,
				quote.Triangle,
				quote.Death,
			}
			if fixture.Name == "FE7U" || fixture.Name == "FE7J" {
				kinds = append(kinds, quote.DeathLyn, quote.DeathOther)
			}
			if fixture.Name == "FE8U" || fixture.Name == "FE8J" {
				kinds = []quote.Kind{quote.Battle, quote.Death}
			}
			for _, kind := range kinds {
				t.Run(string(kind), func(t *testing.T) {
					f := testutil.NewQuoteFixture(fixture.Name)
					s := testutil.Open(t, f)
					if len(s.Quotes.Tables()) != len(kinds) {
						t.Fatal("wrong native table set")
					}
					before := s.Document.Snapshot()
					draft, err := s.Quotes.NewDraft(kind)
					if err != nil {
						t.Fatal(err)
					}
					original := draft.Entries()
					if len(original) != 2 || draft.Modified() {
						t.Fatal("native terminator or initial draft")
					}
					if err = draft.Add(); err != nil {
						t.Fatal(err)
					}
					if err = draft.Set(2, quote.Flag, 0x65); err != nil {
						t.Fatal(err)
					}
					if err = draft.Remove(2); err != nil {
						t.Fatal(err)
					}
					if err = draft.Duplicate(0); err != nil {
						t.Fatal(err)
					}
					if err = draft.Move(2, 0); err != nil {
						t.Fatal(err)
					}
					if err = draft.Set(1, quote.Unit, 3); err != nil {
						t.Fatal(err)
					}
					if err = draft.Remove(2); err != nil {
						t.Fatal(err)
					}
					expected := append(
						bytes.Clone(original[1].Raw),
						original[0].Raw...)
					expected[len(original[1].Raw)] = 3
					end := original[0].Address + 2*len(original[0].Raw)
					expected = append(
						expected,
						before.Data[end:end+len(original[0].Raw)]...)
					plan, err := s.Quotes.Plan(draft)
					if err != nil {
						t.Fatal(err)
					}
					sites := f.TablePointers(string(kind))
					if !slices.Equal(sites, plan.Pointers()) ||
						plan.Count() != 2 ||
						plan.After() < len(before.Data) {
						t.Fatal("incomplete consumer or allocation preview")
					}
					plan.Pointers()[0] = 0
					if !slices.Equal(sites, plan.Pointers()) {
						t.Fatal("mutable preview")
					}
					if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
						t.Fatal("preview changed bytes")
					}
					if err = draft.Set(0, quote.Unit, 9); err != nil {
						t.Fatal(err)
					}
					if err = s.Quotes.Apply(plan); err != nil {
						t.Fatal(err)
					}
					after := s.Document.Snapshot()
					if !bytes.Equal(
						expected,
						after.Data[plan.After():plan.After()+len(expected)],
					) {
						t.Fatal(
							"order, opaque bytes, terminator or frozen contents changed",
						)
					}
					prefix := bytes.Clone(before.Data)
					for _, site := range sites {
						at, valid := gba.Pointer(after.Data, site)
						if !valid || at != plan.After() {
							t.Fatal("native consumer was not moved", site)
						}
						copy(prefix[site:site+4], after.Data[site:site+4])
					}
					if !bytes.Equal(prefix, after.Data[:len(prefix)]) ||
						len(after.Data) != plan.Size() {
						t.Fatal(
							"changed old table, independent data or wrong size",
						)
					}
					if !s.Document.Undo() ||
						!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
						!s.Document.Redo() ||
						!bytes.Equal(after.Data, s.Document.Snapshot().Data) {
						t.Fatal(
							"table and consumers are not one history operation",
						)
					}
					path := filepath.Join(t.TempDir(), "quotes.gba")
					if err = s.Document.SaveAs(path, false); err != nil {
						t.Fatal(err)
					}
					saved, err := game.Open(path)
					if err != nil {
						t.Fatal(err)
					}
					reopened, err := saved.Quotes.NewDraft(kind)
					if err != nil {
						t.Fatal(err)
					}
					if rows := reopened.Entries(); len(rows) != 2 ||
						!bytes.Equal(
							rows[0].Raw,
							expected[:len(rows[0].Raw)],
						) ||
						!bytes.Equal(
							rows[1].Raw,
							expected[len(rows[0].Raw):2*len(rows[0].Raw)],
						) {
						t.Fatal("save/reopen changed records")
					}
				})
			}
		})
	}
}

func TestQuoteReferencesAndFields(t *testing.T) {
	t.Run("independent-shared-tables", func(t *testing.T) {
		f := testutil.NewQuoteFixture("FE7U")
		for _, site := range f.TablePointers("quote_triangle") {
			f.WritePointer(site, f.GetPosition("quote_general"))
		}
		s := testutil.Open(t, f)
		before := s.Document.Snapshot()
		draft, err := s.Quotes.NewDraft(quote.General)
		if err != nil {
			t.Fatal(err)
		}
		if err = draft.Set(0, quote.Unit, 3); err != nil {
			t.Fatal(err)
		}
		plan, err := s.Quotes.Plan(draft)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(plan.Shared(), []quote.Kind{quote.Triangle}) {
			t.Fatal("missing independent sharing")
		}
		if err = s.Quotes.Apply(plan); err != nil {
			t.Fatal(err)
		}
		other, err := s.Quotes.NewDraft(quote.Triangle)
		if err != nil {
			t.Fatal(err)
		}
		if value, _ := other.Value(0, quote.Unit); value != 1 {
			t.Fatal("independent shared table changed")
		}
		after := s.Document.Snapshot()
		for _, site := range f.TablePointers("quote_triangle") {
			if !bytes.Equal(before.Data[site:site+4], after.Data[site:site+4]) {
				t.Fatal("unselected consumer changed")
			}
		}
	})
	for _, version := range []string{"FE6", "FE7J", "FE7U", "FE8J", "FE8U"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewQuoteFixture(version)
			s := testutil.Open(t, f)
			draft, err := s.Quotes.NewDraft(quote.Death)
			if err != nil {
				t.Fatal(err)
			}
			if err = draft.Set(0, quote.Text, 2); err != nil {
				t.Fatal(err)
			}
			if err = draft.Set(0, quote.Flag, 0x65); err != nil {
				t.Fatal(err)
			}
			if err = draft.Set(0, quote.Chapter, 0); err != nil {
				t.Fatal(err)
			}
			if version == "FE8U" || version == "FE8J" {
				if err = draft.Set(0, quote.Route, 2); err != nil {
					t.Fatal(err)
				}
			}
			if version != "FE6" {
				if err = draft.Set(
					0,
					quote.Script,
					0x08000000+uint32(f.GetPosition("location_script")),
				); err != nil {
					t.Fatal(err)
				}
			} // #nosec G115 -- Fixture positions fit in a GBA address.
			plan, err := s.Quotes.Plan(draft)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.Quotes.Apply(plan); err != nil {
				t.Fatal(err)
			}
			data := s.Document.Snapshot().Data
			at := plan.After()
			switch version {
			case "FE6":
				if !bytes.Equal(
					data[at:at+16],
					[]byte{
						1,
						0,
						0xB2,
						0xC3,
						2,
						0,
						0,
						0,
						0x65,
						0,
						0,
						0,
						2,
						0,
						0,
						0,
					},
				) {
					t.Fatal("FE6 defeat and final text layout")
				}
			case "FE7U", "FE7J":
				if !bytes.Equal(
					data[at:at+8],
					[]byte{1, 0, 0xB2, 0xC3, 2, 0, 0, 0},
				) ||
					gba.Uint32(data, at+12) != 0x65 ||
					gba.Uint32(data, at+8) != 0x08251000 {
					t.Fatal("FE7 defeat layout")
				}
			default:
				if !bytes.Equal(
					data[at:at+8],
					[]byte{1, 0, 2, 0, 0x65, 0, 2, 0},
				) ||
					gba.Uint32(data, at+8) != 0x08251000 {
					t.Fatal("FE8 defeat layout")
				}
			}
			uses := s.Texts.Uses(2)
			if !slices.Contains(
				uses,
				text.Use{Kind: string(quote.Death), ID: 0, Field: quote.Text},
			) {
				t.Fatal("missing dialog text reference")
			}
			if version == "FE6" &&
				!slices.Contains(
					uses,
					text.Use{
						Kind:  string(quote.Death),
						ID:    0,
						Field: quote.FinalText,
					},
				) {
				t.Fatal("missing final chapter reference")
			}
			if version == "FE8U" || version == "FE8J" {
				draft, err = s.Quotes.NewDraft(quote.Death)
				if err != nil {
					t.Fatal(err)
				}
				if err = draft.Set(0, quote.Text, 0); err != nil {
					t.Fatal(err)
				}
				if err = draft.Set(0, quote.Script, 0); err != nil {
					t.Fatal(err)
				}
				plan, err = s.Quotes.Plan(draft)
				if err != nil {
					t.Fatal(err)
				}
				if err = s.Quotes.Apply(plan); err != nil {
					t.Fatal(err)
				}
				silent, err := s.Quotes.NewDraft(quote.Death)
				if err != nil {
					t.Fatal(err)
				}
				if len(silent.Entries()) != 2 {
					t.Fatal("silent flag-only record became a terminator")
				}
			}
		})
	}
}

func TestQuoteConflictAndFailureAtomicity(t *testing.T) {
	t.Run("document-revision-layout", func(t *testing.T) {
		s := testutil.Open(t, testutil.NewQuoteFixture("FE8U"))
		other := testutil.Open(t, testutil.NewQuoteFixture("FE8U"))
		draft, err := s.Quotes.NewDraft(quote.Battle)
		if err != nil {
			t.Fatal(err)
		}
		if err = draft.Set(0, quote.Unit, 3); err != nil {
			t.Fatal(err)
		}
		plan, err := s.Quotes.Plan(draft)
		if err != nil {
			t.Fatal(err)
		}
		before := other.Document.Snapshot()
		if err = other.Quotes.Apply(
			plan,
		); !errors.Is(err, rom.ErrStale) ||
			!bytes.Equal(before.Data, other.Document.Snapshot().Data) {
			t.Fatal("cross-document write", err)
		}
		layout := s.Records.Table(records.Items).Layout
		layout.Count--
		before = s.Document.Snapshot()
		if err = s.Records.UseLayout(records.Items, layout); err != nil {
			t.Fatal(err)
		}
		if err = s.Quotes.Apply(plan); !errors.Is(err, rom.ErrStale) {
			t.Fatal("layout accepted old plan", err)
		}
		if _, err = s.Quotes.Plan(
			draft,
		); !errors.Is(err, rom.ErrStale) ||
			!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
			t.Fatal("layout accepted old draft", err)
		}
		draft, err = s.Quotes.NewDraft(quote.Battle)
		if err != nil {
			t.Fatal(err)
		}
		if err = draft.Set(0, quote.Unit, 3); err != nil {
			t.Fatal(err)
		}
		plan, err = s.Quotes.Plan(draft)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.Records.Set(
			records.Units,
			[]int{1},
			"base.hp",
			9,
		); err != nil {
			t.Fatal(err)
		}
		before = s.Document.Snapshot()
		if err = s.Quotes.Apply(
			plan,
		); !errors.Is(err, rom.ErrStale) ||
			!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
			t.Fatal("revision accepted old plan", err)
		}
	})
	t.Run("divergent-consumers", func(t *testing.T) {
		f := testutil.NewQuoteFixture("FE6")
		f.WritePointer(0x6D2BC, f.GetPosition("quote_general"))
		s := testutil.Open(t, f)
		before := s.Document.Snapshot()
		if _, err := s.Quotes.NewDraft(
			quote.Death,
		); !errors.Is(err, quote.ErrReferences) ||
			!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
			t.Fatal("divergent native consumers merged", err)
		}
		draft, err := s.Quotes.NewDraft(quote.Battle)
		if err != nil {
			t.Fatal(err)
		}
		if err = draft.Set(0, quote.Unit, 3); err != nil {
			t.Fatal(err)
		}
		plan, err := s.Quotes.Plan(draft)
		if err != nil {
			t.Fatal("unrelated table blocked", err)
		}
		if err = s.Quotes.Apply(plan); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("pointer-and-terminator", func(t *testing.T) {
		for _, value := range []uint32{0, 0x02000000, 0x08260001, 0x09000000, 0x08FFFFFC} {
			f := testutil.NewQuoteFixture("FE8U")
			f.WriteU32(0x846AC, value)
			s := testutil.Open(t, f)
			if _, err := s.Quotes.NewDraft(
				quote.Battle,
			); !errors.Is(
				err,
				quote.ErrTable,
			) {
				t.Fatal("invalid pointer or truncated record", value, err)
			}
		}
		f := testutil.NewQuoteFixture("FE8U")
		at := f.GetPosition("quote_battle")
		f.WriteAt(at+32, make([]byte, 16))
		s := testutil.Open(t, f)
		if _, err := s.Quotes.NewDraft(
			quote.Battle,
		); !errors.Is(
			err,
			quote.ErrTable,
		) {
			t.Fatal("accepted zero-run as FE8 terminator", err)
		}
	})
	t.Run("invalid-text-and-script", func(t *testing.T) {
		s := testutil.Open(t, testutil.NewQuoteFixture("FE8U"))
		draft, err := s.Quotes.NewDraft(quote.Battle)
		if err != nil {
			t.Fatal(err)
		}
		if err = draft.Set(0, quote.Text, 0x7FFF); err != nil {
			t.Fatal(err)
		}
		before := s.Document.Snapshot()
		if _, err = s.Quotes.Plan(
			draft,
		); err == nil ||
			!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
			t.Fatal("invalid text was accepted", err)
		}
		if err = draft.Set(0, quote.Text, 1); err != nil {
			t.Fatal(err)
		}
		for _, value := range []uint32{0x02000000, 0x08000000, 0x08251001, 0x09FFFFFC} {
			if err = draft.Set(0, quote.Script, value); err != nil {
				t.Fatal(err)
			}
			if _, err = s.Quotes.Plan(
				draft,
			); !errors.Is(err, edit.ErrValue) ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("invalid script was accepted", value, err)
			}
		}
	})
	t.Run("extended-character-word", func(t *testing.T) {
		f := testutil.NewQuoteFixture("FE8U")
		f.WriteU8(f.GetPosition("quote_death")+1, 0x34)
		s := testutil.Open(t, f)
		draft, err := s.Quotes.NewDraft(quote.Death)
		if err != nil {
			t.Fatal(err)
		}
		if err = draft.Set(0, quote.Unit, 2); !errors.Is(err, edit.ErrLayout) {
			t.Fatal("extended character word was overwritten", err)
		}
		if err = draft.Set(0, quote.Flag, 0x66); err != nil {
			t.Fatal(err)
		}
		plan, err := s.Quotes.Plan(draft)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.Quotes.Apply(plan); err != nil {
			t.Fatal(err)
		}
		if gba.Uint16(s.Document.Snapshot().Data, plan.After()) != 0x3401 {
			t.Fatal("extended character word changed during unrelated edit")
		}
	})
	t.Run("script-consumer-overlap", func(t *testing.T) {
		f := testutil.NewQuoteFixture("FE8U")
		f.WritePointer(f.GetPosition("quote_battle")+12, 0x846AC)
		s := testutil.Open(t, f)
		draft, err := s.Quotes.NewDraft(quote.Battle)
		if err != nil {
			t.Fatal(err)
		}
		if err = draft.Set(0, quote.Unit, 3); err != nil {
			t.Fatal(err)
		}
		before := s.Document.Snapshot()
		if _, err = s.Quotes.Plan(
			draft,
		); !errors.Is(err, edit.ErrLayout) ||
			!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
			t.Fatal("consumer overwrote a declared script prefix", err)
		}
	})
	t.Run("owner-protection", func(t *testing.T) {
		f := testutil.NewQuoteFixture("FE8U")
		s := testutil.Open(t, f)
		layout := s.Records.Table(records.Items).Layout
		layout.Address = 0x846AC
		layout.Count = 1
		if err := s.Records.UseLayout(records.Items, layout); err != nil {
			t.Fatal(err)
		}
		draft, err := s.Quotes.NewDraft(quote.Battle)
		if err != nil {
			t.Fatal(err)
		}
		if err = draft.Set(0, quote.Unit, 3); err != nil {
			t.Fatal(err)
		}
		before := s.Document.Snapshot()
		if _, err = s.Quotes.Plan(
			draft,
		); !errors.Is(err, edit.ErrLayout) ||
			!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
			t.Fatal("quote consumer overwrote records", err)
		}
		f = testutil.NewQuoteFixture("FE8U")
		f.WritePointer(0x846AC, f.GetPosition("shop_preparation_pointer"))
		f.WriteU16(f.GetPosition("shop_preparation_pointer")+16, 0xFFFF)
		s = testutil.Open(t, f)
		stock, err := s.Shops.NewDraft(shop.Key{Origin: shop.Preparation})
		if err != nil {
			t.Fatal(err)
		}
		if err = stock.Inventory().Add(3); err != nil {
			t.Fatal(err)
		}
		before = s.Document.Snapshot()
		if _, err = s.Shops.Plan(
			stock,
		); !errors.Is(err, edit.ErrLayout) ||
			!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
			t.Fatal("shop overwrote quote records", err)
		}
	})
	t.Run("capacity-and-empty-table", func(t *testing.T) {
		s := testutil.Open(t, testutil.NewQuoteFixture("FE8U"))
		draft, err := s.Quotes.NewDraft(quote.Battle)
		if err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if err = draft.Remove(0); err != nil {
				t.Fatal(err)
			}
		}
		plan, err := s.Quotes.Plan(draft)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.Quotes.Apply(plan); err != nil {
			t.Fatal(err)
		}
		empty, err := s.Quotes.NewDraft(quote.Battle)
		if err != nil || len(empty.Entries()) != 0 {
			t.Fatal("empty table", err)
		}
		if err = empty.Add(); err != nil {
			t.Fatal(err)
		}
		plan, err = s.Quotes.Plan(empty)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.Quotes.Apply(plan); err != nil {
			t.Fatal(err)
		}
		if err = s.Document.ApplyEdit(
			s.Document.State().Revision,
			"Fill address space",
			rom.Edit{Size: rom.MaxSize},
		); err != nil {
			t.Fatal(err)
		}
		draft, err = s.Quotes.NewDraft(quote.Battle)
		if err != nil {
			t.Fatal(err)
		}
		if err = draft.Set(0, quote.Unit, 3); err != nil {
			t.Fatal(err)
		}
		before := s.Document.Snapshot()
		if _, err = s.Quotes.Plan(
			draft,
		); !errors.Is(err, space.ErrCapacity) ||
			!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
			t.Fatal("capacity failure wrote bytes", err)
		}
	})
}
