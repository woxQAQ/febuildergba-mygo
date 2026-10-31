package quote_test

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/quote"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

func TestFE8BattleConditionsWorkflow(t *testing.T) {
	for _, version := range []string{"FE8U", "FE8J"} {
		t.Run(version, func(t *testing.T) {
			s := testutil.Open(t, testutil.NewQuoteFixture(version))
			before := s.Document.Snapshot()
			draft, err := s.Quotes.NewDraft(quote.Battle)
			if err != nil {
				t.Fatal(err)
			}
			original := draft.Entries()
			samples := []struct {
				condition quote.BattleCondition
				raw       []byte
			}{
				{
					quote.BattleCondition{Participants: quote.Attacker},
					[]byte{1, 0, 0, 0, 255, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
				},
				{
					quote.BattleCondition{Participants: quote.Defender},
					[]byte{0, 0, 1, 0, 255, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
				},
				{
					quote.BattleCondition{
						Participants: quote.Pair,
						Triangle:     true,
					},
					[]byte{1, 0, 1, 0, 254, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
				},
				{
					quote.BattleCondition{
						Participants: quote.Attacker,
						Triangle:     true,
					},
					[]byte{1, 0, 0, 0, 254, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
				},
			}
			for _, sample := range samples {
				if err := draft.AddBattle(sample.condition); err != nil {
					t.Fatal(err)
				}
			}
			for i, sample := range samples {
				got, err := draft.BattleCondition(i + 2)
				if err != nil || got != sample.condition ||
					!bytes.Equal(draft.Entries()[i+2].Raw, sample.raw) {
					t.Fatal("wrong native condition or payload", i, got, err)
				}
			}
			if err := draft.AddBattle(
				quote.BattleCondition{Participants: quote.Neither},
			); !errors.Is(err, edit.ErrValue) ||
				len(draft.Entries()) != 6 {
				t.Fatal("invalid addition changed draft")
			}
			plan, err := s.Quotes.Plan(draft)
			if err != nil {
				t.Fatal(err)
			}
			draft.Set(2, quote.Unit, 0)
			inactive, err := draft.BattleCondition(2)
			if err != nil || inactive.Participants != quote.Neither {
				t.Fatal("two empty roles are not inactive")
			}
			if err := s.Quotes.Apply(plan); err != nil {
				t.Fatal(err)
			}
			result, err := s.Quotes.NewDraft(quote.Battle)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(result.Entries()[0].Raw, original[0].Raw) ||
				!bytes.Equal(result.Entries()[1].Raw, original[1].Raw) {
				t.Fatal("existing order or opaque bytes changed")
			}
			for i, sample := range samples {
				if !bytes.Equal(result.Entries()[i+2].Raw, sample.raw) {
					t.Fatal("plan was not frozen")
				}
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				!s.Document.Redo() {
				t.Fatal("mixed conditions not one history operation")
			}
			if err := s.Quotes.Apply(plan); !errors.Is(err, rom.ErrStale) {
				t.Fatal("stale plan accepted")
			}
			path := filepath.Join(t.TempDir(), "conditions.gba")
			if err := s.Document.SaveAs(path, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			saved, err := reopened.Quotes.NewDraft(quote.Battle)
			if err != nil {
				t.Fatal(err)
			}
			for i, sample := range samples {
				got, err := saved.BattleCondition(i + 2)
				if err != nil || got != sample.condition {
					t.Fatal("condition changed on reopen")
				}
			}
		})
	}
	s := testutil.Open(t, testutil.NewQuoteFixture("FE7U"))
	draft, err := s.Quotes.NewDraft(quote.General)
	if err != nil {
		t.Fatal(err)
	}
	if err := draft.AddBattle(
		quote.BattleCondition{},
	); !errors.Is(err, edit.ErrField) ||
		draft.Modified() {
		t.Fatal("FE8 modes leaked into FE7 table")
	}
}
