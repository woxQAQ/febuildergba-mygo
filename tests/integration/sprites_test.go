//go:build integration

package integration

import (
 "bytes"
 "testing"

 "github.com/woxQAQ/febuildergba-mygo/game"
 "github.com/woxQAQ/febuildergba-mygo/game/records"
)

func readRecordSprites(t *testing.T,s *game.Session){
 table:=requireTable(t,s,records.Classes)
 before:=s.Document.Snapshot()
 standing,weapons:=0,0
 for _,row:=range table.Rows{
  id,err:=s.Records.Value(records.Classes,row.ID,"map_sprite")
  if err!=nil{continue}
  preview:=s.Sprites.Standing(int(id))
  if preview.Err!=nil{continue}
  if preview.Image==nil||preview.Image.Bounds().Empty(){t.Fatal("successful standing read has no pixels")}
  standing++
 }
 for id:=range 8{
  preview:=s.Sprites.Weapon(id)
  if preview.Err!=nil{continue}
  if preview.Image==nil||preview.Image.Bounds().Dx()!=16||preview.Image.Bounds().Dy()!=16{t.Fatal("successful weapon read has wrong dimensions")}
  weapons++
 }
 if !bytes.Equal(before.Data,s.Document.Snapshot().Data)||before.Revision!=s.Document.State().Revision{t.Fatal("reading sprites changed ROM")}
 if standing==0&&weapons==0{t.Skip("native standing and weapon resources unavailable")}
 t.Logf("adapter=%s, readable class sprites=%d/%d, weapon icons=%d/8",s.Version(),standing,len(table.Rows),weapons)
}
