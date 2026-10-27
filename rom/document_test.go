package rom

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// ROM Document Invariants (ROM-INV-1 through ROM-INV-7)
//
// These tests protect the core guarantees of the ROM document model.
// Any failure here indicates a fundamental violation of system correctness.

// ROM-INV-1: revision 单调递增
//
// 每次 Apply/Undo/Redo 都必须增加 revision。这确保 stale 检测正确工作。
// 如果 revision 可以回退或停滞，基于旧 revision 的操作可能会被错误接受。
func TestDocument_RevisionMonotonic(t *testing.T) {
	d := newTestDocument(t, 0x100)

	revisions := []uint64{d.State().Revision}

	// Apply 应该增加 revision
	_ = d.Apply(
		revisions[len(revisions)-1],
		"write1",
		[]Change{{Offset: 0x10, Data: []byte{0x42}}},
	)
	revisions = append(revisions, d.State().Revision)
	if revisions[1] <= revisions[0] {
		t.Errorf(
			"Apply did not increase revision: %d -> %d",
			revisions[0],
			revisions[1],
		)
	}

	// Undo 应该增加 revision（虽然回退状态，但 revision 仍前进）
	d.Undo()
	revisions = append(revisions, d.State().Revision)
	if revisions[2] <= revisions[1] {
		t.Errorf(
			"Undo did not increase revision: %d -> %d",
			revisions[1],
			revisions[2],
		)
	}

	// Redo 应该增加 revision
	d.Redo()
	revisions = append(revisions, d.State().Revision)
	if revisions[3] <= revisions[2] {
		t.Errorf(
			"Redo did not increase revision: %d -> %d",
			revisions[2],
			revisions[3],
		)
	}

	// 验证单调性：每个 revision 都严格大于前一个
	for i := 1; i < len(revisions); i++ {
		if revisions[i] <= revisions[i-1] {
			t.Errorf(
				"Revision not monotonic: [%d]=%d <= [%d]=%d",
				i,
				revisions[i],
				i-1,
				revisions[i-1],
			)
		}
	}
}

// ROM-INV-2: stateID 正确跳转
//
// Undo 必须回到 beforeID，Redo 必须前进到 afterID。
// 如果 stateID 跳转错误，Dirty 标记会不准确，用户可能丢失修改或看到错误的保存状态。
func TestDocument_StateTransition(t *testing.T) {
	d := newTestDocument(t, 0x100)

	initialState := d.stateID

	// 记录三次修改
	_ = d.Apply(
		d.State().Revision,
		"op1",
		[]Change{{Offset: 0x10, Data: []byte{0x01}}},
	)
	state1 := d.stateID

	_ = d.Apply(
		d.State().Revision,
		"op2",
		[]Change{{Offset: 0x20, Data: []byte{0x02}}},
	)
	state2 := d.stateID

	_ = d.Apply(
		d.State().Revision,
		"op3",
		[]Change{{Offset: 0x30, Data: []byte{0x03}}},
	)
	state3 := d.stateID

	// 验证状态递增
	if state1 == initialState || state2 == state1 || state3 == state2 {
		t.Errorf(
			"States not advancing: %d -> %d -> %d -> %d",
			initialState,
			state1,
			state2,
			state3,
		)
	}

	// Undo 应该回到 state2
	d.Undo()
	if d.stateID != state2 {
		t.Errorf("After undo, expected stateID=%d, got %d", state2, d.stateID)
	}

	// 再 Undo 应该回到 state1
	d.Undo()
	if d.stateID != state1 {
		t.Errorf(
			"After 2nd undo, expected stateID=%d, got %d",
			state1,
			d.stateID,
		)
	}

	// Redo 应该前进到 state2
	d.Redo()
	if d.stateID != state2 {
		t.Errorf("After redo, expected stateID=%d, got %d", state2, d.stateID)
	}

	// 再 Redo 应该前进到 state3
	d.Redo()
	if d.stateID != state3 {
		t.Errorf(
			"After 2nd redo, expected stateID=%d, got %d",
			state3,
			d.stateID,
		)
	}
}

// ROM-INV-3: savedID 对应未修改状态
//
// Dirty 标记必须准确反映文档是否有未保存的修改。
// 如果 savedID 不正确，用户可能丢失修改或看到错误的保存提示。
func TestDocument_SavedState(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.gba")
	if err := os.WriteFile(
		path,
		bytes.Repeat([]byte{0x00}, 0x100),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	// 初始状态：未修改
	if d.State().Dirty {
		t.Error("New document should not be dirty")
	}

	// Apply 后：有修改
	rev := d.State().Revision
	_ = d.Apply(rev, "change", []Change{{Offset: 0x10, Data: []byte{0x42}}})
	if !d.State().Dirty {
		t.Error("Document should be dirty after Apply")
	}

	// Undo 后：回到初始状态，应该不 dirty
	d.Undo()
	if d.State().Dirty {
		t.Error("Document should not be dirty after undoing to initial state")
	}

	// Redo 后：又有修改
	d.Redo()
	if !d.State().Dirty {
		t.Error("Document should be dirty after redoing change")
	}

	// 保存后：不 dirty
	if err := d.Save(); err != nil {
		t.Fatal(err)
	}
	if d.State().Dirty {
		t.Error("Document should not be dirty after save")
	}

	// 新的修改：又 dirty
	rev = d.State().Revision
	_ = d.Apply(rev, "change2", []Change{{Offset: 0x20, Data: []byte{0x99}}})
	if !d.State().Dirty {
		t.Error("Document should be dirty after new change")
	}
}

// ROM-INV-4: Undo 完全恢复字节
//
// Undo 必须精确恢复修改前的字节，不能有任何偏差。
// 这是撤销功能的核心保证：用户必须能无损回滚操作。
func TestDocument_UndoRestoresBytes(t *testing.T) {
	d := newTestDocument(t, 0x100)

	// 记录初始快照
	original := d.Snapshot().Data

	// 修改多个区域
	rev := d.State().Revision
	changes := []Change{
		{Offset: 0x10, Data: []byte{0xAA, 0xBB, 0xCC}},
		{Offset: 0x50, Data: []byte{0x11, 0x22}},
		{Offset: 0x90, Data: []byte{0xFF}},
	}
	if err := d.Apply(rev, "multi", changes); err != nil {
		t.Fatal(err)
	}

	// 验证修改生效
	modified := d.Snapshot().Data
	if bytes.Equal(original, modified) {
		t.Fatal("Apply did not change data")
	}

	// Undo
	if !d.Undo() {
		t.Fatal("Undo returned false")
	}

	// 验证完全恢复
	restored := d.Snapshot().Data
	if !bytes.Equal(original, restored) {
		t.Errorf(
			"Undo did not restore bytes:\noriginal: %x\nrestored: %x",
			original[:0x100],
			restored[:0x100],
		)
	}
}

// ROM-INV-5: 重叠写入被拒绝
//
// 同一次 Apply 中的多个 Change 不能重叠，这会导致模糊的语义。
// 例如：Change{0x10, [0xAA]} 和 Change{0x10, [0xBB]} 重叠，应该被拒绝。
func TestDocument_OverlappingWrites(t *testing.T) {
	d := newTestDocument(t, 0x100)

	tests := []struct {
		name    string
		changes []Change
		wantErr bool
	}{
		{
			name: "non-overlapping",
			changes: []Change{
				{Offset: 0x10, Data: []byte{0xAA}},
				{Offset: 0x20, Data: []byte{0xBB}},
			},
			wantErr: false,
		},
		{
			name: "adjacent (should succeed)",
			changes: []Change{
				{Offset: 0x10, Data: []byte{0xAA, 0xBB}},
				{Offset: 0x12, Data: []byte{0xCC}},
			},
			wantErr: false,
		},
		{
			name: "exact overlap",
			changes: []Change{
				{Offset: 0x10, Data: []byte{0xAA}},
				{Offset: 0x10, Data: []byte{0xBB}},
			},
			wantErr: true,
		},
		{
			name: "partial overlap",
			changes: []Change{
				{Offset: 0x10, Data: []byte{0xAA, 0xBB, 0xCC}},
				{Offset: 0x12, Data: []byte{0xDD, 0xEE}},
			},
			wantErr: true,
		},
		{
			name: "contained",
			changes: []Change{
				{Offset: 0x10, Data: []byte{0xAA, 0xBB, 0xCC, 0xDD}},
				{Offset: 0x11, Data: []byte{0xFF}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rev := d.State().Revision
			err := d.Apply(rev, tt.name, tt.changes)
			if (err != nil) != tt.wantErr {
				t.Errorf("Apply() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ROM-INV-6: Stale revision 被拒绝
//
// 基于过期 revision 的修改必须被拒绝，防止竞态和数据不一致。
// 这是并发安全和长任务保护的基础。
func TestDocument_StaleRevision(t *testing.T) {
	d := newTestDocument(t, 0x100)

	// 记录初始 revision
	rev1 := d.State().Revision

	// 第一次修改
	if err := d.Apply(
		rev1,
		"op1",
		[]Change{{Offset: 0x10, Data: []byte{0x01}}},
	); err != nil {
		t.Fatal(err)
	}

	// 尝试基于过期的 rev1 再次修改：应该被拒绝
	err := d.Apply(rev1, "op2", []Change{{Offset: 0x20, Data: []byte{0x02}}})
	if !errors.Is(err, ErrStale) {
		t.Errorf("Expected ErrStale when using old revision, got: %v", err)
	}

	// 使用当前 revision 应该成功
	rev2 := d.State().Revision
	if err := d.Apply(
		rev2,
		"op3",
		[]Change{{Offset: 0x30, Data: []byte{0x03}}},
	); err != nil {
		t.Errorf("Apply with current revision should succeed: %v", err)
	}
}

// Concurrent snapshots must remain consistent while edits are applied.
func TestDocument_ConcurrentSnapshots(t *testing.T) {
	d := newTestDocument(t, 0x10000)

	// 启动多个 goroutine 并发读取快照
	var wg sync.WaitGroup
	const readers = 10
	errors := make(chan error, readers)

	for range readers {
		wg.Go(func() {
			for range 100 {
				snap := d.Snapshot()
				if len(snap.Data) != 0x10000 {
					errors <- ErrSize
					return
				}
			}
		})
	}

	// 同时执行一些写操作
	for i := range 50 {
		rev := d.State().Revision
		offset := i * 0x100
		_ = d.Apply(
			rev,
			"write",
			[]Change{{Offset: offset, Data: []byte{byte(i)}}},
		)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent read failed: %v", err)
	}
}

// 边界条件测试

func TestDocument_EmptyChanges(t *testing.T) {
	d := newTestDocument(t, 0x100)
	rev := d.State().Revision

	// 空 changes 应该成功但不记录操作
	err := d.Apply(rev, "noop", []Change{})
	if err != nil {
		t.Errorf("Empty changes should succeed: %v", err)
	}

	// 不应该产生可撤销的操作
	if d.State().CanUndo {
		t.Error("Empty changes should not create undo point")
	}
}

func TestDocument_IdenticalWrite(t *testing.T) {
	d := newTestDocument(t, 0x100)

	// 写入相同的值（no-op）
	rev := d.State().Revision
	original := d.Snapshot().Data[0x10]
	err := d.Apply(
		rev,
		"same",
		[]Change{{Offset: 0x10, Data: []byte{original}}},
	)
	if err != nil {
		t.Errorf("Identical write should succeed: %v", err)
	}

	// 不应该产生可撤销的操作
	if d.State().CanUndo {
		t.Error("Identical write should not create undo point")
	}
}

func TestDocument_OutOfBounds(t *testing.T) {
	d := newTestDocument(t, 0x100)
	rev := d.State().Revision

	tests := []struct {
		name   string
		change Change
	}{
		{"negative offset", Change{Offset: -1, Data: []byte{0x42}}},
		{"offset beyond size", Change{Offset: 0x101, Data: []byte{0x42}}},
		{
			"length overflow",
			Change{Offset: 0x90, Data: bytes.Repeat([]byte{0x42}, 0x100)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := d.Apply(rev, tt.name, []Change{tt.change})
			if err == nil || !errors.Is(err, ErrRange) {
				t.Errorf("Expected ErrRange, got: %v", err)
			}
		})
	}
}

func TestDocument_UndoRedoEmpty(t *testing.T) {
	d := newTestDocument(t, 0x100)

	// 初始状态不能 undo
	if d.Undo() {
		t.Error("Undo on empty history should return false")
	}

	// 初始状态不能 redo
	if d.Redo() {
		t.Error("Redo on empty history should return false")
	}

	// Apply 一次修改
	rev := d.State().Revision
	_ = d.Apply(rev, "op", []Change{{Offset: 0x10, Data: []byte{0x42}}})

	// 现在可以 undo
	if !d.Undo() {
		t.Error("Should be able to undo after apply")
	}

	// Undo 后不能再 undo
	if d.Undo() {
		t.Error("Should not be able to undo twice")
	}

	// 但可以 redo
	if !d.Redo() {
		t.Error("Should be able to redo after undo")
	}

	// Redo 后不能再 redo
	if d.Redo() {
		t.Error("Should not be able to redo twice")
	}
}

// Helper: 创建测试文档
func newTestDocument(t *testing.T, size int) *Document {
	t.Helper()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.gba")
	if err := os.WriteFile(
		path,
		bytes.Repeat([]byte{0x00}, size),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return d
}
