package testutil

import (
	"bytes"
	"os"
	"testing"
)

// Tests for the fixture builder itself.
// 这些测试确保测试基础设施本身是正确的。

func TestFixtureBuilder_BasicConstruction(t *testing.T) {
	f := NewFixture("TEST_GAME\x00\x00\x00\x00", 0x1000)

	// 验证大小
	data := f.Bytes()
	if len(data) != 0x1000 {
		t.Errorf("Fixture size = %d, want 0x1000", len(data))
	}

	// 验证游戏 ID 写入（只检查前缀，因为 gameID 可能不足 12 字节）
	gameID := string(data[0xA0:0xAC])
	if !bytes.HasPrefix([]byte(gameID), []byte("TEST_GAME")) {
		t.Errorf("Game ID prefix mismatch, got %q", gameID)
	}

	// 验证 GBA 签名
	signature := data[0x104:0x108]
	expectedSig := []byte{0x24, 0xFF, 0xAE, 0x51}
	if !bytes.Equal(signature, expectedSig) {
		t.Errorf("GBA signature = %x, want %x", signature, expectedSig)
	}
}

func TestFixtureBuilder_WriteOperations(t *testing.T) {
	f := NewFixture("TEST", 0x100)

	// 写入各种类型
	f.WriteU8(0x10, 0x42)
	f.WriteU16(0x20, 0x1234)
	f.WriteU32(0x30, 0xDEADBEEF)
	f.WritePointer(0x40, 0x123456)

	data := f.Bytes()

	// 验证 U8
	if data[0x10] != 0x42 {
		t.Errorf("U8 at 0x10 = 0x%02X, want 0x42", data[0x10])
	}

	// 验证 U16 (little-endian)
	if data[0x20] != 0x34 || data[0x21] != 0x12 {
		t.Errorf("U16 at 0x20 = [0x%02X, 0x%02X], want [0x34, 0x12]", data[0x20], data[0x21])
	}

	// 验证 U32
	if data[0x30] != 0xEF || data[0x31] != 0xBE || data[0x32] != 0xAD || data[0x33] != 0xDE {
		t.Errorf("U32 at 0x30 = %x, want DEADBEEF", data[0x30:0x34])
	}

	// 验证指针（应该是 0x08123456）
	ptr := uint32(data[0x40]) | (uint32(data[0x41]) << 8) | (uint32(data[0x42]) << 16) | (uint32(data[0x43]) << 24)
	expected := uint32(0x08123456)
	if ptr != expected {
		t.Errorf("Pointer at 0x40 = 0x%08X, want 0x%08X", ptr, expected)
	}
}

func TestFixtureBuilder_PositionMarking(t *testing.T) {
	f := NewFixture("TEST", 0x100)

	// 标记几个位置
	f.MarkPosition("start", 0x10)
	f.MarkPosition("middle", 0x50)
	f.MarkPosition("end", 0x90)

	// 获取位置
	if f.GetPosition("start") != 0x10 {
		t.Errorf("start position = 0x%X, want 0x10", f.GetPosition("start"))
	}
	if f.GetPosition("middle") != 0x50 {
		t.Errorf("middle position = 0x%X, want 0x50", f.GetPosition("middle"))
	}
	if f.GetPosition("end") != 0x90 {
		t.Errorf("end position = 0x%X, want 0x90", f.GetPosition("end"))
	}
}

func TestMinimalFE8U_Structure(t *testing.T) {
	f := NewMinimalFE8U()
	data := f.Bytes()

	// 验证游戏 ID（FE8U 的完整 ID 是 15 字节）
	gameIDBytes := data[0xA0:0xAF]
	if !bytes.HasPrefix(gameIDBytes, []byte("FIREEMBLEM8")) {
		t.Errorf("Game ID prefix mismatch, got %q", string(gameIDBytes[:12]))
	}

	// 验证指针表
	const ptrUnits = 0x803D90
	unitsPtr := uint32(data[ptrUnits]) | (uint32(data[ptrUnits+1]) << 8) |
		(uint32(data[ptrUnits+2]) << 16) | (uint32(data[ptrUnits+3]) << 24)
	expectedUnitsPtr := uint32(0x08800000)
	if unitsPtr != expectedUnitsPtr {
		t.Errorf("Units pointer = 0x%08X, want 0x%08X", unitsPtr, expectedUnitsPtr)
	}

	// 验证人物数据存在
	const dataUnits = 0x800000
	nameID := uint16(data[dataUnits]) | (uint16(data[dataUnits+1]) << 8)
	if nameID != 0x0002 {
		t.Errorf("Eirika name ID = 0x%04X, want 0x0002", nameID)
	}

	// 验证 terminator
	allZero := true
	for i := 0; i < 0x88; i++ {
		if data[dataUnits+0x88+i] != 0 {
			allZero = false
			break
		}
	}
	if !allZero {
		t.Error("Unit terminator is not all zeros")
	}
}

func TestMinimalFE6_Structure(t *testing.T) {
	f := NewMinimalFE6()
	data := f.Bytes()

	// 验证游戏 ID
	gameID := string(data[0xA0:0xAC])
	if gameID != "FIREEMBLEM6\x00" {
		t.Errorf("Game ID = %q, want FE6", gameID)
	}

	// 验证指针位置不同于 FE8U
	const ptrUnits = 0x017FC8
	unitsPtr := uint32(data[ptrUnits]) | (uint32(data[ptrUnits+1]) << 8) |
		(uint32(data[ptrUnits+2]) << 16) | (uint32(data[ptrUnits+3]) << 24)
	expectedUnitsPtr := uint32(0x08800000)
	if unitsPtr != expectedUnitsPtr {
		t.Errorf("FE6 Units pointer = 0x%08X, want 0x%08X", unitsPtr, expectedUnitsPtr)
	}
}

func TestFixtureBuilder_MinimumSize(t *testing.T) {
	// 太小的 size 应该被自动扩展
	f := NewFixture("TEST", 100)
	if len(f.Bytes()) < 0x1000 {
		t.Errorf("Minimum size not enforced: got %d bytes", len(f.Bytes()))
	}
}

func TestFixtureBuilder_SaveAndLoad(t *testing.T) {
	f := NewMinimalFE8U()
	tmpDir := t.TempDir()
	path := tmpDir + "/test.gba"

	// 保存
	if err := f.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// 读取并验证
	// (实际使用时，这会被 rom.Open() 读取)
	// 这里只是验证文件确实被写入
	if _, err := os.Stat(path); err != nil {
		t.Errorf("Saved file does not exist: %v", err)
	}
}
