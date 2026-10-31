package game_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game/deployment"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

// A damaged event graph cannot authorize a deployment group pointer to
// overwrite the support talk table: the cross-domain claim conflicts.
func TestDeploymentPointerConflictsWithSupportClaims(t *testing.T) {
	f := testutil.NewDeploymentFixture("FE8U")
	site := f.GetPosition("deploy_first")
	units := f.GetPosition("deploy_units")
	// A valid talk table whose payload covers the group pointer site. Its
	// bytes at the site keep the group pointer itself readable.
	table := site - 8
	row := []byte{1, 0, 2, 0, 1, 0, 1, 0, 0, 0, 0, 0, 1, 0, 0, 0}
	f.WritePointer(0x84784, table)
	f.WritePointer(0x847FC, table)
	f.WriteAt(table, row)
	f.WritePointer(site, units)
	end := bytes.Repeat([]byte{0xC8}, 16)
	end[0], end[1] = 0xFF, 0xFF
	f.WriteAt(table+16, end)
	s := testutil.Open(t, f)
	before := s.Document.Snapshot()
	draft, err := s.Deployments.NewDraft(0, site)
	if err != nil {
		t.Fatal(err)
	}
	if err := draft.Set(0, "deploy_level", 7); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Deployments.Plan(
		draft,
	); !errors.Is(err, deployment.ErrDeployment) ||
		!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
		t.Fatalf(
			"a deployment group pointer overwrote the support talk table: %v",
			err,
		)
	}
}

// A manually laid-out character table owns its support payloads, so a table
// move cannot reuse them: claims follow the effective layout, not the
// automatically discovered table.
func TestTableMoveRespectsManualLayoutClaims(t *testing.T) {
	f := testutil.NewDeploymentFixture("FE8U")
	s := testutil.Open(t, f)
	table := s.Records.Table(records.Units)
	if table == nil || table.Err != nil {
		t.Fatal("character table unreadable")
	}
	manual, payload := 0x340000, 0x350000
	data := s.Document.Snapshot().Data
	f.WriteAt(
		manual,
		data[table.Address:table.Address+table.Stride*table.Count],
	)
	// The manual table's first row owns a support record that the automatic
	// table lacks.
	f.WritePointer(manual+44, payload)
	f.WriteAt(payload, make([]byte, 24))
	m := testutil.Open(t, f)
	layout := table.Layout
	layout.Address = manual
	if err := m.Records.UseLayout(records.Units, layout); err != nil {
		t.Fatal(err)
	}
	if got := m.Records.Table(records.Units).Address; got != manual {
		t.Fatalf("manual layout not in effect: %X", got)
	}
	free := []space.FreeRange{
		{
			Range:  space.Range{Start: payload, End: payload + 0x100},
			Source: "declared interval overlapping a manual support payload",
		},
	}
	before := m.Document.Snapshot()
	if _, err := m.Records.PlanTableMove(
		records.Items,
		free,
	); !errors.Is(err, space.ErrRange) ||
		before.Revision != m.Document.State().Revision {
		t.Fatalf(
			"a table move could overwrite a manual support payload: %v",
			err,
		)
	}
}
