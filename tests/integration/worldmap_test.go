//go:build integration

package integration

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/worldmap"
)

func editWorldMapNode(t *testing.T, s *game.Session) {
	if !s.WorldMap.Available() {
		t.Skip("no native world-map node editor for this input")
	}
	nodes, err := s.WorldMap.Nodes()
	if err != nil || len(nodes) == 0 {
		t.Skipf("world-map nodes unavailable: %v", err)
	}
	node := nodes[0]
	draft, err := s.WorldMap.NewDraft(node.ID)
	if err != nil {
		t.Fatal(err)
	}
	before := s.Document.Snapshot()
	want := bytes.Clone(before.Data)
	// Coordinates, references and source bytes come from this input, not a
	// particular release. Only the native field offsets are fixed expectations.
	x := node.Value(worldmap.X) ^ 1
	if err := draft.Set(worldmap.X, x); err != nil {
		t.Fatal(err)
	}
	binary.LittleEndian.PutUint16(want[node.Address+24:], uint16(x))
	texts, _ := s.Texts.Entries()
	for _, entry := range texts {
		if entry.Err == nil && entry.ID != node.Name {
			if err := draft.Set(worldmap.Name, int(entry.ID)); err != nil {
				t.Fatal(err)
			}
			binary.LittleEndian.PutUint16(want[node.Address+28:], entry.ID)
			break
		}
	}
	if chapters := s.Records.Table(
		records.Chapters,
	); chapters != nil &&
		chapters.Err == nil {
		for _, row := range chapters.Rows {
			if row.ID >= 0 && row.ID < 256 &&
				row.ID != node.Value(worldmap.EirikaChapter) {
				if err := draft.Set(
					worldmap.EirikaChapter,
					row.ID,
				); err != nil {
					t.Fatal(err)
				}
				want[node.Address+4] = byte(row.ID)
				break
			}
		}
	}
	plan, err := s.WorldMap.Plan(draft)
	checkPlan(t, s, before, err)
	t.Logf(
		"node=%d address=0x%08X fields=%d",
		node.ID,
		node.Address,
		len(plan.Changes()),
	)
	if err := s.WorldMap.Apply(plan); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, s.Document.Snapshot().Data) {
		t.Fatal(
			"node edit changed unrelated bytes, inventory pointers or file length",
		)
	}
	reopened := historyAndReopen(t, s, before)
	saved, err := reopened.WorldMap.NewDraft(node.ID)
	if err != nil || saved.Node().Value(worldmap.X) != x ||
		saved.Node().Name != draft.Node().Name ||
		saved.Node().Value(worldmap.EirikaChapter) != draft.Node().Value(worldmap.EirikaChapter) {
		t.Fatal("node fields differ after reopening", err)
	}
}
