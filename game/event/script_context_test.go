package event_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func TestScriptEntryContexts(t *testing.T) {
	for _, fixture := range testutil.Games() {
		t.Run(fixture.Name, func(t *testing.T) {
			f := testutil.NewScriptFixture(fixture.Name)
			const header, conditions, manual = 0x24F400, 0x24F800, 0x24F000
			originalHeader := f.GetPosition("deploy_header")
			originalConditions := f.GetPosition("objective_table")
			raw := f.Bytes()
			f.WriteAt(header, raw[originalHeader:originalHeader+80])
			f.WriteAt(conditions, raw[originalConditions:originalConditions+88])
			f.WritePointer(header+12, conditions)
			f.WritePointer(f.GetPosition("deploy_list")+8, header)
			s := testutil.Open(t, f)
			table := s.Records.Table(records.Chapters)
			f.WriteAt(
				manual,
				raw[table.Rows[1].Address:table.Rows[1].Address+table.Stride],
			)
			s = testutil.Open(t, f)
			initial, err := s.Events.ScriptEntries(0)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Records.UseLayout(
				records.Chapters,
				records.Layout{Address: manual, Stride: table.Stride, Count: 1},
			); err != nil {
				t.Fatal(err)
			}
			manualEntries, err := s.Events.ScriptEntries(0)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, entry := range manualEntries {
				if entry.Pointer == conditions+4 {
					found = true
				}
			}
			if !found || len(initial) == 0 {
				t.Fatal("script entry cache ignored chapter layout")
			}
			d, err := s.Events.NewScriptDraft(0, conditions+4)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Insert(0, event.ScriptWait); err != nil {
				t.Fatal(err)
			}
			plan, err := s.Events.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.Uses()) != 1 {
				t.Fatal("manual script sharing ignored")
			}
			if err := s.Events.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			s.Records.UseAutomaticLayout(records.Chapters)
			untouched, err := s.Events.Script(0, f.GetPosition("script_site"))
			if err != nil ||
				untouched.Entry.Address != f.GetPosition("script_root") {
				t.Fatal("independent condition table changed")
			}
			move, err := s.Records.PlanTableMove(records.Chapters, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Records.ApplyTableMove(move); err != nil {
				t.Fatal(err)
			}
			moved, err := s.Events.Script(1, conditions+4)
			if err != nil || moved.Instructions[0].Op != event.ScriptWait {
				t.Fatal("relocated chapter table lost script entry")
			}
		})
	}
}

func TestScriptResourceAndDynamicGuards(t *testing.T) {
	for _, scenario := range []string{"foreign preview", "stale preview", "layout preview", "capacity"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.NewScriptFixture("FE8U")
			s := testutil.Open(t, f)
			if scenario == "capacity" {
				snapshot := s.Document.Snapshot()
				if err := s.Document.ApplyEdit(
					snapshot.Revision,
					"capacity fixture",
					rom.Edit{Size: rom.MaxSize},
				); err != nil {
					t.Fatal(err)
				}
			}
			d, err := s.Events.NewScriptDraft(0, f.GetPosition("script_site"))
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Insert(0, event.ScriptWait); err != nil {
				t.Fatal(err)
			}
			before := s.Document.Snapshot()
			plan, err := s.Events.PlanScriptEdit(d)
			if scenario == "capacity" {
				if !errors.Is(err, space.ErrCapacity) ||
					!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
					t.Fatal("script capacity failure", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "foreign preview":
				s = testutil.Open(t, f)
			case "stale preview":
				if err := s.Records.Set(
					records.Chapters,
					[]int{0},
					"weather",
					2,
				); err != nil {
					t.Fatal(err)
				}
			case "layout preview":
				table := s.Records.Table(records.Chapters)
				if err := s.Records.UseLayout(
					records.Chapters,
					records.Layout{
						Address: table.Address,
						Stride:  table.Stride,
						Count:   table.Count,
					},
				); err != nil {
					t.Fatal(err)
				}
			}
			before = s.Document.Snapshot()
			if err := s.Events.ApplyScriptEdit(
				plan,
			); !errors.Is(err, rom.ErrStale) ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("stale script preview applied", err)
			}
		})
	}
	t.Run("deployment alias", func(t *testing.T) {
		f := testutil.NewScriptFixture("FE8U")
		units := f.GetPosition("objective_units")
		f.WriteAt(units, []byte{0x20, 0x0E, 5, 0, 0x20, 1, 0, 0})
		f.WritePointer(f.GetPosition("script_site"), units)
		s := testutil.Open(t, f)
		d, err := s.Events.NewScriptDraft(0, f.GetPosition("script_site"))
		if err != nil {
			t.Fatal(err)
		}
		if !d.Complete() {
			t.Fatal("fixed code-shaped data did not reproduce overlap")
		}
		if err := d.Set(0, event.ScriptFrames, 6); err != nil {
			t.Fatal(err)
		}
		before := s.Document.Snapshot()
		if _, err := s.Events.PlanScriptEdit(
			d,
		); !errors.Is(
			err,
			edit.ErrLayout,
		) {
			t.Fatal("code-shaped deployment data overwritten", err)
		}
		if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
			t.Fatal("overlap failure mutated document")
		}
	})
	t.Run("dynamic call", func(t *testing.T) {
		f := testutil.NewScriptFixture("FE8U")
		f.WriteU32(f.GetPosition("script_call")+4, 0xFFFFFFFF)
		s := testutil.Open(t, f)
		d, err := s.Events.NewScriptDraft(0, f.GetPosition("script_site"))
		if err != nil {
			t.Fatal(err)
		}
		if err := d.Insert(0, event.ScriptWait); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Events.PlanScriptEdit(
			d,
		); !errors.Is(
			err,
			event.ErrScriptStructure,
		) {
			t.Fatal("dynamic script relocated", err)
		}
	})
	t.Run("empty ending", func(t *testing.T) {
		f := testutil.NewScriptFixture("FE8J")
		site := f.GetPosition("objective_ending_site")
		f.WriteU32(site, 0)
		s := testutil.Open(t, f)
		d, err := s.Events.NewScriptDraft(0, site)
		if err != nil {
			t.Fatal(err)
		}
		if d.Modified() || len(d.Instructions()) != 0 {
			t.Fatal("browsing empty entry created data")
		}
		if err := d.Insert(0, event.ScriptEnd); err != nil {
			t.Fatal(err)
		}
		if err := d.Insert(0, event.ScriptWait); err != nil {
			t.Fatal(err)
		}
		if err := d.Set(0, event.ScriptFrames, 3); err != nil {
			t.Fatal(err)
		}
		plan, err := s.Events.PlanScriptEdit(d)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Events.ApplyScriptEdit(plan); err != nil {
			t.Fatal(err)
		}
		result, err := s.Events.Script(0, site)
		if err != nil || !result.Complete || len(result.Instructions) != 2 {
			t.Fatal("explicit empty script creation")
		}
	})
}
