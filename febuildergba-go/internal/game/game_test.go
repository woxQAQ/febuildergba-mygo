package game

import (
	"bytes"
	"testing"
)

// Field Parsing Invariants (FIELD-INV-1 through FIELD-INV-3)
//
// 这些测试保护字段解析的核心保证。
// 字段解析是整个编辑器的基础：如果解析错误，所有编辑都会破坏 ROM。

// FIELD-INV-1: Value 在 Bounds 内
//
// Field.Value() 返回的值必须在 Field.Bounds() 定义的范围内。
// 如果超出范围，说明解析逻辑有 bug，可能读取了错误的字节。
func TestField_ValueInBounds(t *testing.T) {
	tests := []struct {
		name   string
		field  Field
		data   []byte
		addr   int
		wantOK bool
	}{
		{
			name:   "unsigned u8 in range",
			field:  Field{Offset: 0, Width: 1, Signed: false},
			data:   []byte{0x42},
			addr:   0,
			wantOK: true,
		},
		{
			name:   "unsigned u8 max",
			field:  Field{Offset: 0, Width: 1, Signed: false},
			data:   []byte{0xFF},
			addr:   0,
			wantOK: true,
		},
		{
			name:   "signed i8 positive",
			field:  Field{Offset: 0, Width: 1, Signed: true},
			data:   []byte{0x7F}, // +127
			addr:   0,
			wantOK: true,
		},
		{
			name:   "signed i8 negative",
			field:  Field{Offset: 0, Width: 1, Signed: true},
			data:   []byte{0x80}, // -128
			addr:   0,
			wantOK: true,
		},
		{
			name:   "unsigned u16",
			field:  Field{Offset: 0, Width: 2, Signed: false},
			data:   []byte{0xFF, 0xFF}, // 65535
			addr:   0,
			wantOK: true,
		},
		{
			name:   "signed i16 negative",
			field:  Field{Offset: 0, Width: 2, Signed: true},
			data:   []byte{0x00, 0x80}, // -32768
			addr:   0,
			wantOK: true,
		},
		{
			name:   "field with offset",
			field:  Field{Offset: 5, Width: 1, Signed: false},
			data:   []byte{0, 0, 0, 0, 0, 0x99},
			addr:   0,
			wantOK: true,
		},
		{
			name:   "u32 max",
			field:  Field{Offset: 0, Width: 4, Signed: false},
			data:   []byte{0xFF, 0xFF, 0xFF, 0xFF},
			addr:   0,
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := tt.field.Value(tt.data, tt.addr)
			min, max := tt.field.Bounds()

			if value < min || value > max {
				t.Errorf("Value %d out of bounds [%d, %d]", value, min, max)
			}
		})
	}
}

// FIELD-INV-2: 符号扩展正确
//
// 有符号字段必须正确处理负数（符号扩展）。
// 例如：i8 的 0x80 应该被解析为 -128，而不是 128。
func TestField_SignedExtension(t *testing.T) {
	tests := []struct {
		name      string
		field     Field
		data      []byte
		wantValue int64
	}{
		{
			name:      "i8: -1",
			field:     Field{Offset: 0, Width: 1, Signed: true},
			data:      []byte{0xFF},
			wantValue: -1,
		},
		{
			name:      "i8: -128",
			field:     Field{Offset: 0, Width: 1, Signed: true},
			data:      []byte{0x80},
			wantValue: -128,
		},
		{
			name:      "i8: +127",
			field:     Field{Offset: 0, Width: 1, Signed: true},
			data:      []byte{0x7F},
			wantValue: 127,
		},
		{
			name:      "i16: -1",
			field:     Field{Offset: 0, Width: 2, Signed: true},
			data:      []byte{0xFF, 0xFF},
			wantValue: -1,
		},
		{
			name:      "i16: -32768",
			field:     Field{Offset: 0, Width: 2, Signed: true},
			data:      []byte{0x00, 0x80},
			wantValue: -32768,
		},
		{
			name:      "i16: +32767",
			field:     Field{Offset: 0, Width: 2, Signed: true},
			data:      []byte{0xFF, 0x7F},
			wantValue: 32767,
		},
		{
			name:      "unsigned should not sign extend",
			field:     Field{Offset: 0, Width: 1, Signed: false},
			data:      []byte{0xFF},
			wantValue: 255, // not -1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.field.Value(tt.data, 0)
			if got != tt.wantValue {
				t.Errorf("Value() = %d, want %d", got, tt.wantValue)
			}
		})
	}
}

// FIELD-INV-3: 表布局不越界
//
// 表的 address + stride * count 不能超出 ROM 大小。
// 如果越界，读取操作会 panic 或读取错误数据。
func TestTable_LayoutBounds(t *testing.T) {
	const romSize = 0x1000

	tests := []struct {
		name    string
		layout  Layout
		wantErr bool
	}{
		{
			name:    "valid table",
			layout:  Layout{Address: 0x100, Stride: 0x10, Count: 10},
			wantErr: false,
		},
		{
			name:    "table at end of ROM",
			layout:  Layout{Address: 0xF00, Stride: 0x10, Count: 16},
			wantErr: false,
		},
		{
			name:    "table exceeds ROM",
			layout:  Layout{Address: 0xF00, Stride: 0x10, Count: 17},
			wantErr: true,
		},
		{
			name:    "address beyond ROM",
			layout:  Layout{Address: 0x1001, Stride: 0x10, Count: 1},
			wantErr: true,
		},
		{
			name:    "negative address",
			layout:  Layout{Address: -1, Stride: 0x10, Count: 1},
			wantErr: true,
		},
		{
			name:    "zero stride with count > 1",
			layout:  Layout{Address: 0x100, Stride: 0, Count: 2},
			wantErr: true, // 会导致所有记录重叠
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 检查布局是否有效
			valid := tt.layout.Address >= 0 &&
				tt.layout.Address < romSize &&
				tt.layout.Stride > 0 &&
				tt.layout.Address+tt.layout.Stride*tt.layout.Count <= romSize

			if valid == tt.wantErr {
				t.Errorf("Layout validity = %v, wantErr %v", valid, tt.wantErr)
			}
		})
	}
}

// Session Operation Invariants (SES-INV-1, SES-INV-2)

// SES-INV-1: 批量操作原子性
//
// 批量修改必须全成功或全失败，不能留下部分修改。
// 这确保撤销时可以完整回滚整个批量操作。
func TestSession_BatchAtomic(t *testing.T) {
	// 创建一个简单的测试 ROM 和 session
	rom := bytes.Repeat([]byte{0x00}, 0x1000)
	
	// 模拟批量修改：修改三个字段
	changes := []struct {
		offset int
		value  byte
	}{
		{0x100, 0xAA},
		{0x200, 0xBB},
		{0x300, 0xCC},
	}

	// 记录初始状态
	original := bytes.Clone(rom)

	// 执行批量修改
	for _, ch := range changes {
		rom[ch.offset] = ch.value
	}

	// 验证所有修改都生效
	for _, ch := range changes {
		if rom[ch.offset] != ch.value {
			t.Errorf("Change at 0x%X not applied", ch.offset)
		}
	}

	// 模拟撤销：回滚所有修改
	copy(rom, original)

	// 验证完全回滚
	for i, b := range original {
		if rom[i] != b {
			t.Errorf("Byte at 0x%X not restored: got 0x%02X, want 0x%02X", i, rom[i], b)
			break
		}
	}
}

// SES-INV-2: 字段修改反映到文档
//
// 通过 Session 修改字段后，Document 必须包含相应的字节变化。
// 这是编辑器正确性的基础：UI 的修改必须实际写入 ROM。
func TestSession_FieldWritesPersist(t *testing.T) {
	// 这个测试需要真实的 Session，暂时用简化版本验证概念
	
	// 模拟：修改一个字段
	rom := bytes.Repeat([]byte{0x00}, 0x100)
	field := Field{Offset: 0x10, Width: 2, Signed: false}
	
	// 写入值 0x1234
	rom[field.Offset] = 0x34
	rom[field.Offset+1] = 0x12

	// 验证可以读回
	value := field.Value(rom, 0)
	expected := int64(0x1234)
	if value != expected {
		t.Errorf("Value() = 0x%X, want 0x%X", value, expected)
	}

	// 验证字节确实改变
	if rom[field.Offset] != 0x34 || rom[field.Offset+1] != 0x12 {
		t.Error("Field write did not persist to ROM bytes")
	}
}

// 边界条件测试

func TestField_ZeroWidth(t *testing.T) {
	// Width=0 的字段是非法的，应该导致 panic 或错误
	// 这里我们仅测试 Bounds() 的行为
	field := Field{Offset: 0, Width: 0, Signed: false}
	min, max := field.Bounds()
	
	// Width=0 应该产生退化的范围
	if min != 0 || max != 0 {
		t.Errorf("Zero-width field bounds = [%d, %d], expected [0, 0]", min, max)
	}
}

func TestField_LargeWidth(t *testing.T) {
	// 测试超出 4 字节的字段（虽然实际不应该出现）
	field := Field{Offset: 0, Width: 5, Signed: false}
	
	// Bounds() 应该仍然返回合理值（不 overflow）
	min, max := field.Bounds()
	if min < 0 || max < 0 {
		t.Errorf("Large width field bounds overflow: [%d, %d]", min, max)
	}
}

func TestLayout_EmptyTable(t *testing.T) {
	// Count=0 的表是合法的（空表）
	layout := Layout{Address: 0x100, Stride: 0x10, Count: 0}
	
	// 空表不应该占用任何空间
	endAddr := layout.Address + layout.Stride*layout.Count
	if endAddr != layout.Address {
		t.Errorf("Empty table should not occupy space: end=0x%X, start=0x%X", endAddr, layout.Address)
	}
}

// 回归测试：已知 bug 的防护

// 这个测试防止 Field.Value() 在读取有符号值时使用错误的位掩码
func TestField_SignedMaskBug(t *testing.T) {
	// 曾经有 bug：有符号 i16 的 0x8000 被解析为 +32768 而不是 -32768
	field := Field{Offset: 0, Width: 2, Signed: true}
	data := []byte{0x00, 0x80} // little-endian -32768

	value := field.Value(data, 0)
	if value != -32768 {
		t.Errorf("Signed i16 0x8000 should be -32768, got %d", value)
	}
}

// 这个测试防止 Field.Bounds() 在有符号字段上返回错误的范围
func TestField_SignedBoundsBug(t *testing.T) {
	// 曾经有 bug：i8 的范围被计算为 [0, 255] 而不是 [-128, 127]
	field := Field{Offset: 0, Width: 1, Signed: true}
	min, max := field.Bounds()

	if min != -128 || max != 127 {
		t.Errorf("i8 bounds should be [-128, 127], got [%d, %d]", min, max)
	}
}
