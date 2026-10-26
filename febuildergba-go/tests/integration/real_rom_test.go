//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/internal/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/rom"
)

// Integration tests using real ROMs.
// These tests require actual ROM files in .rom/ directory.
// Run with: go test -tags=integration ./...

const (
	romDir = "../../../.rom"
	fe8uROM = "Fire Emblem - The Sacred Stones (USA, Australia).gba"
	fe8jROM = "Fire Emblem - Seima no Kouseki (Japan).gba"
	fe6ROM  = "Fire Emblem - Fuuin no Tsurugi (Japan).gba"
)

func TestFE8U_RealROM_OpenAndInspect(t *testing.T) {
	path := filepath.Join(romDir, fe8uROM)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skipf("ROM not found: %s", path)
	}

	doc, err := rom.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	// 验证 ROM 大小合理
	state := doc.State()
	if state.Size < 16<<20 || state.Size > 32<<20 {
		t.Errorf("Unexpected ROM size: %d bytes", state.Size)
	}

	t.Skip("Session inspection not yet implemented")

	// sess := game.NewSession(doc)
	// 
	// // 验证识别为 FE8U
	// if sess.Version() != "FE8U" {
	//     t.Errorf("Version mismatch: got %s, want FE8U", sess.Version())
	// }
	// 
	// // 验证人物表可以解析
	// unitTable := sess.GetTable(game.Units)
	// if unitTable == nil {
	//     t.Fatal("Cannot get unit table")
	// }
	// if unitTable.Count < 50 { // FE8U 至少有 50+ 人物
	//     t.Errorf("Unit count too low: %d", unitTable.Count)
	// }
	// 
	// // 验证可以读取 Eirika 的名字
	// eirika := unitTable.Rows[0]
	// if eirika.Name == "" {
	//     t.Error("Eirika name is empty")
	// }
}

func TestFE8U_RealROM_EditAndSave(t *testing.T) {
	path := filepath.Join(romDir, fe8uROM)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skipf("ROM not found: %s", path)
	}

	// 创建临时副本
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, "test.gba")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	// 打开文档
	doc, err := rom.Open(tmpPath)
	if err != nil {
		t.Fatal(err)
	}

	t.Skip("Session edit operations not yet implemented")

	// sess := game.NewSession(doc)
	// 
	// // 修改 Eirika 的 HP
	// originalHP := sess.GetField(game.Units, 0, "HP")
	// newHP := int64(99)
	// 
	// rev := doc.State().Revision
	// err = sess.EditField(game.Units, 0, "HP", newHP)
	// if err != nil {
	//     t.Fatal(err)
	// }
	// 
	// // 验证修改生效
	// if sess.GetField(game.Units, 0, "HP") != newHP {
	//     t.Error("HP edit did not take effect")
	// }
	// 
	// // 保存
	// snap := doc.Snapshot()
	// if err := os.WriteFile(tmpPath, snap.Data, 0644); err != nil {
	//     t.Fatal(err)
	// }
	// doc.MarkSaved()
	// 
	// // 重新打开并验证
	// doc2, err := rom.Open(tmpPath)
	// if err != nil {
	//     t.Fatal(err)
	// }
	// sess2 := game.NewSession(doc2)
	// if sess2.GetField(game.Units, 0, "HP") != newHP {
	//     t.Error("HP change did not persist")
	// }
	// 
	// // 撤销并验证回到原值
	// doc.Undo()
	// if sess.GetField(game.Units, 0, "HP") != originalHP {
	//     t.Error("Undo did not restore original HP")
	// }
}

func TestFE6_RealROM_BasicParsing(t *testing.T) {
	path := filepath.Join(romDir, fe6ROM)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skipf("ROM not found: %s", path)
	}

	doc, err := rom.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	state := doc.State()
	if state.Size < 8<<20 {
		t.Errorf("FE6 ROM too small: %d bytes", state.Size)
	}

	t.Skip("FE6 session not yet implemented")

	// sess := game.NewSession(doc)
	// 
	// if sess.Version() != "FE6" {
	//     t.Errorf("Version mismatch: got %s, want FE6", sess.Version())
	// }
	// 
	// // FE6 的职业表 stride 应该是 0x48（无转职加成）
	// classTable := sess.GetTable(game.Classes)
	// if classTable.Stride != 0x48 {
	//     t.Errorf("FE6 class stride should be 0x48, got 0x%X", classTable.Stride)
	// }
}

func TestCompareWithLegacy(t *testing.T) {
	// 这个测试用于和旧版 C# 实现对比
	// 需要旧版 FEBuilderGBA.exe 可用
	t.Skip("Legacy comparison not implemented")

	// 步骤：
	// 1. 创建测试 ROM 副本
	// 2. 用新版修改某个字段
	// 3. 用旧版修改相同字段
	// 4. 对比两个输出的字节差异
	// 5. 如果有差异，记录在白名单或报告为 bug
}

// 批量测试：测试所有可用的真实 ROM
func TestAllAvailableROMs(t *testing.T) {
	roms := []struct {
		filename    string
		expectedVer string
	}{
		{fe8uROM, "FE8U"},
		{fe8jROM, "FE8J"},
		{fe6ROM, "FE6"},
	}

	for _, rom := range roms {
		t.Run(rom.expectedVer, func(t *testing.T) {
			path := filepath.Join(romDir, rom.filename)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Skipf("ROM not found: %s", path)
			}

			doc, err := rom.Open(path)
			if err != nil {
				t.Fatalf("Cannot open %s: %v", rom.filename, err)
			}

			// 基本健全性检查
			state := doc.State()
			if state.Size == 0 {
				t.Error("ROM size is zero")
			}
			if state.Revision == 0 {
				t.Error("Initial revision should not be zero")
			}
			if state.Dirty {
				t.Error("Newly opened ROM should not be dirty")
			}

			t.Logf("%s: %d bytes, revision %d", rom.expectedVer, state.Size, state.Revision)
		})
	}
}

// 压力测试：大量编辑和撤销
func TestStress_ManyEditsAndUndos(t *testing.T) {
	path := filepath.Join(romDir, fe8uROM)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skipf("ROM not found: %s", path)
	}

	// 创建临时副本
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, "stress.gba")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	doc, err := rom.Open(tmpPath)
	if err != nil {
		t.Fatal(err)
	}

	// 执行 1000 次小修改
	const iterations = 1000
	for i := 0; i < iterations; i++ {
		rev := doc.State().Revision
		offset := 0x800000 + (i % 0x1000)
		err := doc.Apply(rev, "stress", []rom.Change{{Offset: offset, Data: []byte{byte(i)}}})
		if err != nil {
			t.Fatalf("Edit %d failed: %v", i, err)
		}
	}

	// 验证可以撤销所有修改
	for i := 0; i < iterations; i++ {
		if !doc.Undo() {
			t.Fatalf("Undo %d failed", i)
		}
	}

	// 验证不能再撤销
	if doc.Undo() {
		t.Error("Should not be able to undo beyond initial state")
	}

	t.Logf("Successfully completed %d edits and undos", iterations)
}
