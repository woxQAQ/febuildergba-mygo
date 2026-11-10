package event_test

import (
	"bytes"
	"cmp"
	"errors"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/quote"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

func quoteScriptUses(uses []text.Use) []text.Use {
	uses = slices.DeleteFunc(uses, func(use text.Use) bool {
		return use.Kind != event.TextUse || use.ID != event.QuoteScripts
	})
	slices.SortFunc(uses, func(a, b text.Use) int {
		return cmp.Or(
			cmp.Compare(a.Source, b.Source),
			cmp.Compare(a.Pointer, b.Pointer),
			cmp.Compare(a.Address, b.Address),
			cmp.Compare(a.Field, b.Field),
		)
	})
	return uses
}

func TestQuoteScriptBrowsingWorkflow(t *testing.T) {
	for _, version := range []string{"FE7U", "FE7J", "FE8U", "FE8J"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewQuoteScriptFixture(version)
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			if !s.Events.QuoteScriptsAvailable() {
				t.Fatal("native script roots unavailable")
			}
			root, child := f.GetPosition(
				"quote_script_root",
			), f.GetPosition(
				"quote_script_child",
			)
			type role struct {
				kind           quote.Kind
				position       string
				offset, stride int
			}
			roles := []role{
				{quote.Battle, "quote_battle", 12, 16},
				{quote.Death, "quote_death", 8, 12},
			}
			fe7 := version == "FE7U" || version == "FE7J"
			if fe7 {
				roles[0].offset, roles[1].stride = 8, 16
				roles = append(roles,
					role{quote.DeathLyn, "quote_death_lyn", 4, 12},
					role{quote.DeathOther, "quote_death_other", 4, 12})
			}
			entries, err := s.Events.ScriptEntries(event.QuoteScripts)
			if err != nil || len(entries) != len(roles)*2+1 {
				t.Fatal(
					"native roots or shared call missing",
					len(entries),
					err,
				)
			}
			var want []text.Use
			for _, role := range roles {
				for record := range 2 {
					site := f.GetPosition(
						role.position,
					) + record*role.stride + role.offset
					if !slices.ContainsFunc(
						entries,
						func(entry event.ScriptEntry) bool {
							return entry.Chapter == event.QuoteScripts &&
								entry.Source == string(role.kind) &&
								entry.Record == record &&
								entry.Pointer == site &&
								entry.Address == root &&
								entry.Err == nil
						},
					) {
						t.Fatal(
							"record identity lost",
							role.kind,
							record,
							entries,
						)
					}
					use := text.Use{
						Kind:    event.TextUse,
						ID:      event.QuoteScripts,
						Source:  string(role.kind),
						Record:  record,
						Pointer: site,
						Address: root,
						Field:   event.ScriptTextID,
					}
					want = append(want, use)
					if fe7 {
						use.Address, use.Field = root+8, event.ScriptTextPrimary
						want = append(want, use)
					}
				}
			}
			want = quoteScriptUses(want)
			uses, complete := s.Texts.Uses(1)
			if !complete || !slices.Equal(quoteScriptUses(uses), want) {
				t.Fatal(
					"quote text scope or aliases",
					complete,
					quoteScriptUses(uses),
					want,
				)
			}
			for _, use := range want {
				program, err := s.Events.Script(use.ID, use.Pointer)
				if err != nil || !program.ReadOnly || !program.Complete ||
					program.Entry.Record != use.Record ||
					program.Entry.Source != use.Source ||
					!slices.ContainsFunc(
						program.Instructions,
						func(row event.ScriptInstruction) bool { return row.Address == use.Address },
					) {
					t.Fatal(
						"usage does not navigate to its global instruction",
						use,
						err,
					)
				}
				for _, instruction := range program.Instructions {
					if instruction.Editable {
						t.Fatal("browsing granted instruction writes")
					}
				}
			}
			childUses, childComplete := s.Events.TextUses(2)
			childUse := text.Use{
				Kind:    event.TextUse,
				ID:      event.QuoteScripts,
				Record:  -1,
				Source:  "script_call_entry",
				Pointer: f.GetPosition("quote_script_call") + 4,
				Address: child,
				Field:   event.ScriptTextID,
			}
			if !childComplete || !slices.Contains(childUses, childUse) {
				t.Fatal(
					"static child lost independent scope",
					childUses,
					childComplete,
				)
			}
			draft, err := s.Events.NewScriptDraft(
				event.QuoteScripts,
				f.GetPosition("quote_script_battle_site"),
			)
			if err != nil || !draft.ReadOnly() || draft.Relocatable() {
				t.Fatal("global draft permissions", err)
			}
			if operands := draft.TextOperands(
				0,
			); len(operands) != 1 ||
				operands[0].ID != 1 {
				t.Fatal("readable text lost", operands)
			}
			for _, change := range []func() error{
				func() error { return draft.Set(0, event.ScriptTextID, 2) },
				func() error { return draft.Insert(0, event.ScriptEnd) },
				func() error { return draft.Remove(0) },
				func() error { return draft.Duplicate(0) },
				func() error { return draft.Move(0, 1) },
				func() error { return draft.SetMovement(0, event.ScriptMovementPointer, nil) },
			} {
				if err := change(); !errors.Is(err, event.ErrScriptStructure) {
					t.Fatal("global script accepted a write", err)
				}
			}
			if _, err := s.Events.PlanScriptEdit(
				draft,
			); !errors.Is(err, event.ErrScriptStructure) ||
				draft.Modified() {
				t.Fatal("read-only plan or draft mutated", err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				before.Revision != s.Document.Snapshot().Revision {
				t.Fatal("browsing or rejected writes changed the document")
			}

			file, err := s.Texts.File()
			if err != nil {
				t.Fatal(err)
			}
			imported, err := file.Import([]byte("[0001]\r\nAA\r\n"))
			if err != nil {
				t.Fatal(err)
			}
			filePlan, err := s.Texts.PlanImport(imported)
			if err != nil || filePlan.Entries()[0].UsesIncomplete ||
				!slices.Equal(
					quoteScriptUses(filePlan.Entries()[0].Uses),
					want,
				) {
				t.Fatal("file preview lost quote script scope", err)
			}
			plan, err := s.Texts.PlanID(1, "AA")
			if err != nil || !plan.UsesComplete() ||
				!slices.Equal(quoteScriptUses(plan.Uses()), want) {
				t.Fatal("single text preview lost quote script scope", err)
			}
			if err := s.Texts.Apply(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			if !bytes.Equal(
				before.Data[root:child+32],
				after.Data[root:child+32],
			) ||
				draft.Current(s.Events) {
				t.Fatal(
					"text edit changed scripts or retained an old input stamp",
				)
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				!s.Document.Redo() {
				t.Fatal("text replacement history")
			}
			if err := s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(s.Document.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			saved, complete := reopened.Texts.Uses(1)
			if !complete || !slices.Equal(quoteScriptUses(saved), want) ||
				!bytes.Equal(after.Data, reopened.Document.Snapshot().Data) {
				t.Fatal(
					"text replacement lost quote script navigation on reopen",
				)
			}
		})
	}
}

func TestQuoteScriptReferenceRefresh(t *testing.T) {
	f := testutil.NewQuoteScriptFixture("FE8U")
	s := testutil.Open(t, f)
	before := s.Document.Snapshot()
	uses, _ := s.Texts.Uses(1)
	original := quoteScriptUses(uses)
	reader, err := s.Events.NewScriptDraft(
		event.QuoteScripts,
		f.GetPosition("quote_script_battle_site"),
	)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := s.Texts.PlanID(1, "AA")
	if err != nil {
		t.Fatal(err)
	}
	other := testutil.Open(t, f)
	if err := other.Texts.Apply(preview); !errors.Is(err, rom.ErrStale) {
		t.Fatal("quote text preview crossed documents", err)
	}
	draft, err := s.Quotes.NewDraft(quote.Battle)
	if err != nil {
		t.Fatal(err)
	}
	if err := draft.Set(
		0,
		quote.Script,
		testutil.PointerWord(f.GetPosition("quote_script_child")),
	); err != nil {
		t.Fatal(err)
	}
	plan, err := s.Quotes.Plan(draft)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Quotes.Apply(plan); err != nil {
		t.Fatal(err)
	}
	after := s.Document.Snapshot()
	updated, complete := s.Events.TextUses(1)
	updated = quoteScriptUses(updated)
	if !complete || len(updated) != 3 ||
		slices.ContainsFunc(updated, func(use text.Use) bool {
			return use.Pointer == f.GetPosition("quote_script_battle_site") ||
				use.Source == string(quote.Battle) && use.Record == 0
		}) ||
		reader.Current(s.Events) ||
		preview.Current(s.Texts) {
		t.Fatal(
			"relocated quote table retained stale entries",
			updated,
			complete,
		)
	}
	if err := s.Texts.Apply(
		preview,
	); !errors.Is(err, rom.ErrStale) ||
		!bytes.Equal(after.Data, s.Document.Snapshot().Data) {
		t.Fatal("stale text preview changed ROM", err)
	}
	if !s.Document.Undo() {
		t.Fatal("quote reference undo")
	}
	restored, _ := s.Events.TextUses(1)
	if !slices.Equal(quoteScriptUses(restored), original) ||
		!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
		!s.Document.Redo() {
		t.Fatal("reference index did not follow undo/redo")
	}
	layoutReader, err := s.Events.NewScriptDraft(
		event.QuoteScripts,
		updated[0].Pointer,
	)
	if err != nil {
		t.Fatal(err)
	}
	chapters := s.Records.Table(records.Chapters)
	if err := s.Records.UseLayout(records.Chapters, records.Layout{
		Address: chapters.Address, Stride: chapters.Stride, Count: 1,
	}); err != nil {
		t.Fatal(err)
	}
	manual, _ := s.Events.TextUses(1)
	if !slices.Equal(quoteScriptUses(manual), updated) ||
		layoutReader.Current(s.Events) {
		t.Fatal("manual chapter layout changed independent quote roots")
	}
	if err := s.Document.Save(); err != nil {
		t.Fatal(err)
	}
	reopened, err := game.Open(s.Document.State().Path)
	if err != nil {
		t.Fatal(err)
	}
	saved, _ := reopened.Events.TextUses(1)
	if !slices.Equal(quoteScriptUses(saved), updated) {
		t.Fatal("relocated table navigation changed after reopening")
	}
}

func TestQuoteScriptPartialDiscovery(t *testing.T) {
	for _, scenario := range []string{"invalid script", "invalid table", "unknown boundary", "opaque ASM", "dynamic call", "cycle", "instruction limit", "entry limit", "damaged chapters"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.NewQuoteScriptFixture("FE8U")
			root, call := f.GetPosition(
				"quote_script_root",
			), f.GetPosition(
				"quote_script_call",
			)
			switch scenario {
			case "invalid script":
				f.WriteU32(
					f.GetPosition("quote_script_battle_site"),
					0x02000000,
				)
			case "invalid table":
				f.WriteU32(f.TablePointers("quote_battle")[0], 0)
			case "unknown boundary":
				f.WriteAt(call, []byte{0xFF, 0xFF, 0xFF, 0xFF})
			case "opaque ASM":
				f.WriteAt(call, []byte{0x40, 0x0D, 0, 0, 0, 0, 0, 2})
			case "dynamic call":
				f.WriteU32(call+4, 0xFFFFFFFF)
			case "cycle":
				f.WritePointer(call+4, root)
			case "instruction limit":
				f.WriteAt(root, bytes.Repeat([]byte{0x20, 0x1B, 1, 0}, 8192))
			case "entry limit":
				at := f.GetPosition("quote_battle")
				raw := f.Bytes()[at : at+16]
				f.WriteAt(at, bytes.Repeat(raw, 513))
				f.WriteAt(
					at+513*16,
					[]byte{
						0xFF,
						0xFF,
						0,
						0,
						0,
						0,
						0,
						0,
						0,
						0,
						0,
						0,
						0,
						0,
						0,
						0,
					},
				)
			case "damaged chapters":
				for _, site := range f.TablePointers("chapters") {
					f.WriteU32(site, 0)
				}
			}
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			uses, complete := s.Events.TextUses(1)
			if len(quoteScriptUses(uses)) == 0 ||
				complete != (scenario == "cycle") {
				t.Fatal(
					"surviving roots or scope completeness",
					complete,
					quoteScriptUses(uses),
				)
			}
			entries, err := s.Events.ScriptEntries(event.QuoteScripts)
			if err != nil || len(entries) == 0 || len(entries) > 512 {
				t.Fatal("independent bounded directory", len(entries), err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("partial discovery wrote ROM")
			}
		})
	}
}

func TestQuoteScriptSplitConsumersAndRoles(t *testing.T) {
	for _, scenario := range []string{"split consumer", "shared role"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.NewQuoteScriptFixture("FE7U")
			if scenario == "split consumer" {
				const separate = 0x297000
				at := f.GetPosition("quote_death")
				f.WriteAt(separate, f.Bytes()[at:at+48])
				f.WritePointer(f.TablePointers("quote_death")[1], separate)
				f.WritePointer(separate+8, f.GetPosition("quote_script_child"))
			} else {
				for _, site := range f.TablePointers("quote_death_other") {
					f.WritePointer(site, f.GetPosition("quote_death_lyn"))
				}
			}
			s := testutil.Open(t, f)
			uses, complete := s.Events.TextUses(1)
			uses = quoteScriptUses(uses)
			if complete != (scenario == "shared role") {
				t.Fatal("split consumer completeness", complete)
			}
			if scenario == "split consumer" {
				if !slices.ContainsFunc(uses, func(use text.Use) bool {
					return use.Source == string(quote.Death) &&
						use.Pointer == 0x297018 &&
						use.Record == 1
				}) {
					t.Fatal("independent split table records omitted", uses)
				}
			} else {
				for _, kind := range []quote.Kind{quote.DeathLyn, quote.DeathOther} {
					if !slices.ContainsFunc(uses, func(use text.Use) bool {
						return use.Source == string(kind) &&
							use.Pointer == f.GetPosition("quote_death_lyn")+4 &&
							use.Record == 0
					}) {
						t.Fatal("shared table role collapsed", kind, uses)
					}
				}
			}
		})
	}
	f := testutil.NewQuoteScriptFixture("FE6")
	s := testutil.Open(t, f)
	if s.Events.QuoteScriptsAvailable() {
		t.Fatal("FE6 text-only quotes acquired script roots")
	}
	if _, err := s.Events.ScriptEntries(
		event.QuoteScripts,
	); !errors.Is(
		err,
		edit.ErrField,
	) {
		t.Fatal("unsupported directory error", err)
	}
}
