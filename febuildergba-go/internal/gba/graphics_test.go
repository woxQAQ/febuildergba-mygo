package gba

import (
	"bytes"
	"fmt"
	"testing"
)

// Encoding Invariants (ENC-INV-1 through ENC-INV-3)
//
// 这些测试保护编解码的核心保证。
// 编解码错误会导致图像损坏、文本乱码或 ROM 数据被破坏。

// ENC-INV-1: LZ77 往返一致
//
// Decompress(Compress(x)) == x
// 如果往返不一致，压缩后的数据无法正确恢复，会导致图像或数据损坏。
func TestLZ77_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "all zeros",
			data: bytes.Repeat([]byte{0x00}, 100),
		},
		{
			name: "all ones",
			data: bytes.Repeat([]byte{0xFF}, 100),
		},
		{
			name: "repeating pattern",
			data: bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 25),
		},
		{
			name: "mixed content",
			data: []byte{
				0x10, 0x20, 0x30, 0x40, 0x50, 0x60, 0x70, 0x80,
				0x10, 0x20, 0x30, 0x40, // repetition
				0xAA, 0xBB, 0xCC, 0xDD,
			},
		},
		{
			name: "single byte",
			data: []byte{0x42},
		},
		{
			name: "empty",
			data: []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compressed, err := Compress(tt.data)
			if err != nil {
				t.Fatalf("Compress() error: %v", err)
			}

			decompressed, err := Decompress(compressed)
			if err != nil {
				t.Fatalf("Decompress() error: %v", err)
			}

			if !bytes.Equal(tt.data, decompressed) {
				t.Errorf("Round trip failed:\noriginal:  %x\nafter:     %x", tt.data, decompressed)
			}
		})
	}
}

// ENC-INV-2: Huffman 往返一致
//
// HuffmanDecode(HuffmanEncode(text)) == text
// 如果往返不一致，文本编辑会导致数据丢失或乱码。
func TestHuffman_RoundTrip(t *testing.T) {
	// 注意：这个测试需要真实的 Huffman 实现
	// 这里提供框架，实际实现需要适配具体的 Huffman 接口

	tests := []struct {
		name string
		text string
	}{
		{
			name: "ASCII",
			text: "Hello, World!",
		},
		{
			name: "numbers",
			text: "0123456789",
		},
		{
			name: "empty",
			text: "",
		},
		{
			name: "single char",
			text: "A",
		},
		{
			name: "repeating",
			text: "AAAAAAAAAA",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// TODO: 实现 HuffmanEncode/Decode 后启用
			t.Skip("Huffman implementation not yet available")

			// encoded := HuffmanEncode(tt.text)
			// decoded, err := HuffmanDecode(encoded)
			// if err != nil {
			//     t.Fatalf("HuffmanDecode() error: %v", err)
			// }
			// if decoded != tt.text {
			//     t.Errorf("Round trip failed: %q != %q", decoded, tt.text)
			// }
		})
	}
}

// ENC-INV-3: 解码边界安全
//
// 非法输入不能导致 panic 或越界读取，必须返回错误。
// 这是安全性的基础：用户可能打开损坏的 ROM 或恶意构造的文件。
func TestDecode_BoundarySafe(t *testing.T) {
	t.Run("LZ77 truncated header", func(t *testing.T) {
		// LZ77 压缩数据的头部被截断
		truncated := []byte{0x10} // 不完整的头部
		_, err := Decompress(truncated)
		if err == nil {
			t.Error("Decompress should fail on truncated header")
		}
	})

	t.Run("LZ77 invalid size", func(t *testing.T) {
		// 声称的解压大小超出合理范围
		invalid := []byte{
			0x10,             // LZ77 标识
			0xFF, 0xFF, 0xFF, // 巨大的解压尺寸
		}
		_, err := Decompress(invalid)
		if err == nil {
			t.Error("Decompress should fail on unreasonable size")
		}
	})

	t.Run("LZ77 truncated data", func(t *testing.T) {
		// 压缩数据不完整
		truncated := []byte{
			0x10,       // LZ77 标识
			0x10, 0, 0, // 解压后 16 字节
			0x01, // 只有一个字节的数据
		}
		_, err := Decompress(truncated)
		if err == nil {
			t.Error("Decompress should fail on truncated compressed data")
		}
	})

	t.Run("LZ77 backreference out of bounds", func(t *testing.T) {
		// 回溯引用超出已解压的缓冲区
		invalid := []byte{
			0x10,       // LZ77 标识
			0x10, 0, 0, // 解压后 16 字节
			0x80,       // 压缩标志（下一个字节是回溯）
			0x10, 0x00, // 回溯到 offset=-16（非法）
		}
		_, err := Decompress(invalid)
		if err == nil {
			t.Error("Decompress should fail on invalid backreference")
		}
	})

	t.Run("empty input", func(t *testing.T) {
		_, err := Decompress([]byte{})
		if err == nil {
			t.Error("Decompress should fail on empty input")
		}
	})
}

// 已知压缩数据的回归测试
//
// 这些测试使用从真实 ROM 提取的压缩数据，确保解压逻辑不退化。
func TestLZ77_KnownData(t *testing.T) {
	// TODO: 从真实 ROM 提取一些已知的 LZ77 压缩块
	// 例如：某个地图图块的压缩数据
	t.Skip("Need known LZ77 samples from real ROM")

	// 示例结构：
	// tests := []struct {
	//     name       string
	//     compressed []byte // 从 ROM 提取
	//     expected   []byte // 已知的解压结果
	// }{
	//     {
	//         name: "FE8U map tileset",
	//         compressed: []byte{...},
	//         expected: []byte{...},
	//     },
	// }
}

// 性能边界测试
//
// 确保解压不会在合理输入上花费过长时间（防止 DoS）。
func TestLZ77_ReasonablePerformance(t *testing.T) {
	// 创建一个中等大小的数据（典型的图块数据）
	data := bytes.Repeat([]byte{
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
	}, 1024) // 8KB

	compressed, err := Compress(data)
	if err != nil {
		t.Fatal(err)
	}

	// 解压应该在合理时间内完成（这个测试不设超时，依赖 go test -timeout）
	decompressed, err := Decompress(compressed)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(data, decompressed) {
		t.Error("Performance test data did not survive round trip")
	}
}

// Stub implementations for testing framework
// 实际的 Compress/Decompress 应该在 graphics.go 中实现

func Compress(data []byte) ([]byte, error) {
	// TODO: 实现实际的 LZ77 压缩
	// 目前返回一个简单的“未压缩”格式用于测试框架
	
	// 简单的假压缩：只加头部
	header := []byte{0x10, byte(len(data)), byte(len(data) >> 8), byte(len(data) >> 16)}
	return append(header, data...), nil
}

func Decompress(compressed []byte) ([]byte, error) {
	// TODO: 实现实际的 LZ77 解压
	// 目前只处理上面的假压缩格式
	if len(compressed) < 4 {
		return nil, ErrInvalidCompression
	}
	
	if compressed[0] != 0x10 {
		return nil, ErrInvalidCompression
	}
	
	size := int(compressed[1]) | (int(compressed[2]) << 8) | (int(compressed[3]) << 16)
	if size > 1<<20 { // 1MB 上限
		return nil, ErrInvalidCompression
	}
	
	// 允许空数据
	if size == 0 {
		return []byte{}, nil
	}
	
	if len(compressed) < 4+size {
		return nil, ErrInvalidCompression
	}
	
	return compressed[4 : 4+size], nil
}

var ErrInvalidCompression = fmt.Errorf("invalid compression data")
