package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/internal/rom"
	"github.com/woxQAQ/febuildergba-mygo/tests/testutil"
)

// End-to-End Invariant Tests
//
// 这些测试验证跨模块的状态一致性。
// 使用合成 fixture，快速验证完整工作流的正确性。

// E2E-INV-1: 会话操作反映到文档
//
// 通过 Session 的字段修改必须产生 Document 的相应字节变化。
func TestSession_OperationsReflectInDocument(t *testing.T) {
	// 创建测试 fixture
	f := testutil.NewMinimalFE8U()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.gba")
	if err := f.Save(path); err != nil {
		t.Fatal(err)
	}

	// 打开文档
	_, err := rom.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	// 创建会话（需要实现 game.NewSession）
	// TODO: 适配实际的 Session 构造函数
	t.Skip("Session construction not yet implemented")

	// sess := game.NewSession(doc)
	// 
	// // 获取初始快照
	// before := doc.Snapshot()
	// 
	// // 通过 Session 修改一个字段（例如：人物 HP）
	// err = sess.EditField(game.Units, 0, "HP", 99)
	// if err != nil {
	//     t.Fatal(err)
	// }
	// 
	// // 获取修改后的快照
	// after := doc.Snapshot()
	// 
	// // 验证字节确实改变
	// if bytes.Equal(before.Data, after.Data) {
	//     t.Error("Session edit did not change document bytes")
	// }
	// 
	// // 验证 revision 增加
	// if after.Revision <= before.Revision {
	//     t.Error("Session edit did not advance document revision")
	// }
}

// E2E-INV-2: 批量操作产生单次撤销
//
// 批量修改多个字段后，一次 Undo 应该回滚所有修改。
func TestSession_BatchEditsFormSingleUndo(t *testing.T) {
	f := testutil.NewMinimalFE8U()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.gba")
	if err := f.Save(path); err != nil {
		t.Fatal(err)
	}

	_, err := rom.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	t.Skip("Session batch operations not yet implemented")

	// sess := game.NewSession(doc)
	// 
	// original := doc.Snapshot()
	// 
	// // 批量修改三个字段
	// edits := []game.FieldEdit{
	//     {Kind: game.Units, ID: 0, Field: "HP", Value: 50},
	//     {Kind: game.Units, ID: 1, Field: "HP", Value: 40},
	//     {Kind: game.Units, ID: 2, Field: "HP", Value: 30},
	// }
	// err = sess.ApplyBatch("bulk HP change", edits)
	// if err != nil {
	//     t.Fatal(err)
	// }
	// 
	// modified := doc.Snapshot()
	// 
	// // 验证修改生效
	// if bytes.Equal(original.Data, modified.Data) {
	//     t.Error("Batch edits did not change data")
	// }
	// 
	// // 一次 Undo 应该回滚所有修改
	// if !doc.Undo() {
	//     t.Fatal("Cannot undo batch operation")
	// }
	// 
	// restored := doc.Snapshot()
	// if !bytes.Equal(original.Data, restored.Data) {
	//     t.Error("Single undo did not restore all batch changes")
	// }
}

// E2E-INV-3: 保存后重新打开，修改持久化
//
// 修改 → 保存 → 关闭 → 重新打开，修改必须仍然存在。
func TestDocument_PersistAfterSaveAndReopen(t *testing.T) {
	f := testutil.NewMinimalFE8U()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.gba")
	if err := f.Save(path); err != nil {
		t.Fatal(err)
	}

	// 第一次打开：修改并保存
	doc, err := rom.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	rev := doc.State().Revision
	testChange := []rom.Change{{Offset: 0x100, Data: []byte{0xAA, 0xBB, 0xCC}}}
	if err := doc.Apply(rev, "test edit", testChange); err != nil {
		t.Fatal(err)
	}

	snap := doc.Snapshot()
	if err := doc.Save(); err != nil {
		t.Fatal(err)
	}

	// 验证修改在当前 session 中生效
	if snap.Data[0x100] != 0xAA || snap.Data[0x101] != 0xBB || snap.Data[0x102] != 0xCC {
		t.Error("Changes not applied in current session")
	}

	// 关闭并重新打开
	doc2, err := rom.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	snap2 := doc2.Snapshot()

	// 验证修改持久化
	if snap2.Data[0x100] != 0xAA || snap2.Data[0x101] != 0xBB || snap2.Data[0x102] != 0xCC {
		t.Error("Changes did not persist after save and reopen")
	}
}

// E2E-INV-4: 撤销后保存，撤销的修改不应持久化
//
// 修改 → 撤销 → 保存 → 重新打开，文件应该是初始状态。
func TestDocument_UndoBeforeSaveDoesNotPersist(t *testing.T) {
	f := testutil.NewMinimalFE8U()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.gba")
	if err := f.Save(path); err != nil {
		t.Fatal(err)
	}

	// 记录初始内容
	initialData, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// 打开并修改
	doc, err := rom.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	rev := doc.State().Revision
	if err := doc.Apply(rev, "temp", []rom.Change{{Offset: 0x100, Data: []byte{0xFF}}}); err != nil {
		t.Fatal(err)
	}

	// 撤销
	if !doc.Undo() {
		t.Fatal("Cannot undo")
	}

	// 保存
	snap := doc.Snapshot()
	if err := os.WriteFile(path, snap.Data, 0644); err != nil {
		t.Fatal(err)
	}

	// 重新打开
	doc2, err := rom.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	snap2 := doc2.Snapshot()

	// 验证文件内容与初始状态相同
	if snap2.Data[0x100] != initialData[0x100] {
		t.Errorf("Undone change persisted: got 0x%02X, want 0x%02X", snap2.Data[0x100], initialData[0x100])
	}
}

// E2E-INV-5: 版本识别在会话中保持一致
//
// 打开 FE8U ROM，会话必须识别为 FE8U，不能误识别为其他版本。
func TestSession_VersionConsistency(t *testing.T) {
	tests := []struct {
		name        string
		fixture     *testutil.FixtureBuilder
		expectedID  string
	}{
		{"FE8U", testutil.NewMinimalFE8U(), "FE8U"},
		{"FE6", testutil.NewMinimalFE6(), "FE6"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "test.gba")
			if err := tt.fixture.Save(path); err != nil {
				t.Fatal(err)
			}

			_, err := rom.Open(path)
			if err != nil {
				t.Fatal(err)
			}

			t.Skip("Session version detection not yet implemented")

			// sess := game.NewSession(doc)
			// if sess.Version() != tt.expectedID {
			//     t.Errorf("Version mismatch: got %s, want %s", sess.Version(), tt.expectedID)
			// }
		})
	}
}

// E2E-INV-6: 外部修改检测
//
// 文档打开后，如果磁盘文件被外部修改，Document.ExternalChanged() 必须返回 true。
func TestDocument_DetectsExternalChange(t *testing.T) {
	f := testutil.NewMinimalFE8U()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.gba")
	if err := f.Save(path); err != nil {
		t.Fatal(err)
	}

	doc, err := rom.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	// 初始状态：没有外部修改
	changed, err := doc.ExternalChanged()
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("Newly opened document should not report external change")
	}

	// 外部修改文件
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data[0x100] = 0x99 // 修改一个字节
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	// 检测外部修改
	changed, err = doc.ExternalChanged()
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("Should detect external modification")
	}
}
