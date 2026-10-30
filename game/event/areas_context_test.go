package event_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func TestAreaChapterContext(t *testing.T) {
	for _, fixture := range testutil.Games() {
		t.Run(fixture.Name, func(t *testing.T) {
			f := testutil.NewAreaFixture(fixture.Name)
			const alias, manual = 0x244400, 0x244000
			header := f.GetPosition("deploy_header")
			raw := f.Bytes()
			f.WriteAt(alias, raw[header:header+80])
			f.WritePointer(f.GetPosition("deploy_list")+8, alias)
			f.WritePointer(f.GetPosition("deploy_list")+12, alias)
			s := testutil.Open(t, f)
			table := s.Records.Table(records.Chapters)
			chapter := table.Rows[0]
			f.WriteAt(manual, raw[chapter.Address:chapter.Address+table.Stride])
			offset := 116
			if fixture.Name == "FE7U" {
				offset = 120
			}
			if fixture.Name == "FE6" {
				offset = 58
			}
			f.WriteU8(manual+offset, 3)
			s = testutil.Open(t, f)
			d, err := s.Events.NewConditions(0, event.AlwaysEvents)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Set(1, event.AreaX1, 4); err != nil {
				t.Fatal(err)
			}
			plan, err := s.Events.PlanConditions(d)
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.Uses()) != 1 || plan.Uses()[0] != 0 {
				t.Fatal("independent header alias counted as shared site")
			}
			if err := s.Events.ApplyConditions(plan); err != nil {
				t.Fatal(err)
			}
			old, err := s.Events.Conditions(1, event.AlwaysEvents)
			if err != nil || old.Address != f.GetPosition("area_table") ||
				old.Events[1].X1 != 2 {
				t.Fatal("independent pointer changed")
			}
			relocation, err := s.Records.PlanTableMove(records.Chapters, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Records.ApplyTableMove(relocation); err != nil {
				t.Fatal(err)
			}
			moved, err := s.Events.Conditions(0, event.AlwaysEvents)
			if err != nil || moved.Events[1].X1 != 4 {
				t.Fatal("chapter relocation lost area context")
			}
			if err := s.Records.UseLayout(
				records.Chapters,
				records.Layout{Address: manual, Stride: table.Stride, Count: 1},
			); err != nil {
				t.Fatal(err)
			}
			actual, err := s.Events.Conditions(0, event.AlwaysEvents)
			if err != nil || actual.Pointer != alias+12 ||
				actual.Events[1].X1 != 2 {
				t.Fatal("manual layout ignored")
			}
			d, err = s.Events.NewConditions(0, event.AlwaysEvents)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Add(s.Events); err != nil {
				t.Fatal(err)
			}
			p, err := s.Events.PlanConditions(d)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Events.ApplyConditions(p); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAreaResourceRecovery(t *testing.T) {
	for _, name := range []string{"empty pointer", "malformed terminator", "unterminated", "unresolved script", "table ownership", "script ownership", "units ownership", "path ownership", "last word terminator"} {
		t.Run(name, func(t *testing.T) {
			f := testutil.NewAreaFixture("FE8U")
			site := f.GetPosition("deploy_header") + 12
			at := f.GetPosition("area_table")
			switch name {
			case "empty pointer":
				f.WriteU32(site, 0)
			case "malformed terminator":
				f.WriteU32(at+36, 0x12340000)
			case "unterminated":
				for i := range 257 {
					f.WriteAt(
						at+i*12,
						[]byte{11, 0, 0, 0, 0, 0, 0, 0, 0, 0, 3, 4},
					)
				}
			case "unresolved script":
				f.WriteU32(at+16, 0xDEADBEEF)
			case "last word terminator":
				f.WritePointer(site, len(f.Bytes())-4)
			}
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			d, err := s.Events.NewConditions(0, event.AlwaysEvents)
			if name == "malformed terminator" || name == "unterminated" {
				if !errors.Is(err, event.ErrEventConditions) ||
					!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
					t.Fatalf("malformed area table: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			owned := map[string]string{
				"table ownership":  "area_table",
				"script ownership": "area_script",
				"units ownership":  "area_units",
				"path ownership":   "area_path",
			}
			if key, ok := owned[name]; ok {
				start := f.GetPosition(key)
				_, err := s.Records.PlanTableMove(
					records.Items,
					[]space.FreeRange{
						{
							Start:  start,
							End:    start + 0x100,
							Source: "area fixture ownership",
						},
					},
				)
				if !errors.Is(err, space.ErrRange) ||
					!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
					t.Fatalf("live area resource reused: %v", err)
				}
				return
			}
			if name == "unresolved script" && len(d.Events()[1].Warnings) == 0 {
				t.Fatal("missing script warning")
			}
			if err := d.Add(s.Events); err != nil {
				t.Fatal(err)
			}
			plan, err := s.Events.PlanConditions(d)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Events.ApplyConditions(plan); err != nil {
				t.Fatal(err)
			}
			result, err := s.Events.Conditions(0, event.AlwaysEvents)
			if err != nil {
				t.Fatal(err)
			}
			if name == "empty pointer" || name == "last word terminator" {
				if len(result.Events) != 1 {
					t.Fatal("empty area table was not created")
				}
			} else if result.Events[1].Raw[4] != 0xEF {
				t.Fatal("unresolved script was rewritten")
			}
		})
	}
}
