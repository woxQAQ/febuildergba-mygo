# Test Fixtures

本文档说明 `testdata/` 中合成 fixture 的结构。这些 fixture 用于核心不变量测试和端到端测试。

## 设计原则

1. **最小化**：仅包含测试所需的必要字节，不是完整 ROM
2. **自解释**：结构在此文档中明确说明
3. **可维护**：通过 `tests/testutil/fixture.go` 生成，不手工编辑

## `minimal_fe8u.bin`

最小的可测试 FE8U ROM，用于测试基础解析和编辑。

### 结构

| 偏移 | 长度 | 内容 | 说明 |
|-----|------|------|------|
| 0x0000 | 0xA0 | GBA header | 标准 GBA ROM 头部 |
| 0x00A0 | 0x0C | Game ID | `FIREEMBLEM8\x0A\x45\x00\x00` (FE8U) |
| 0x00AC | ... | Padding | 填充到指针表位置 |
| 0x803D90 | 4 | Unit table pointer | `00 00 80 08` (→ 0x800000) |
| 0x803EB8 | 4 | Class table pointer | `88 01 80 08` (→ 0x800188) |
| 0x809B10 | 4 | Item table pointer | `10 02 80 08` (→ 0x800210) |
| 0x800000 | 0x88 | Unit #1 | Eirika 的数据 |
| 0x800088 | 0x88 | Unit #2 | Seth 的数据 |
| 0x800110 | 0x88 | Unit #3 | Terminator (0x00...) |
| 0x800188 | 0x54 | Class #1 | Ephraim Lord 的数据 |
| 0x8001DC | 0x54 | Class #2 | Eirika Lord 的数据 |
| 0x800230 | 0x54 | Class #3 | Terminator |
| 0x800210 | 0x24 | Item #1 | Iron Sword 的数据 |
| 0x800234 | 0x24 | Item #2 | Iron Lance 的数据 |
| 0x800258 | 0x24 | Item #3 | Terminator |

### Unit #1 (Eirika) 详细结构

| 偏移 | 字节 | 字段 | 值 | 说明 |
|-----|------|------|-----|------|
| 0x00 | 02 00 | Name ID | 0x0002 | 指向文本表的 ID |
| 0x02 | 07 00 | Description ID | 0x0007 | 描述文本 ID |
| 0x04 | 03 | Character ID | 0x03 | Eirika |
| 0x05 | 07 | Class | 0x07 | Eirika Lord |
| 0x06 | 01 | Portrait | 0x01 | 头像 ID |
| 0x07 | 00 | ??? | 0x00 | 保留字节 |
| 0x08 | 01 | Level | 1 | 等级 |
| 0x09-0x0F | ... | Base stats | HP=16, Pow=4, Skl=9, Spd=12, Def=3, Res=2, Lck=8 | 基础能力 |
| 0x10 | 0B | Sword | 11 | 剑熟练度 |
| 0x11-0x17 | 00 | Other weapons | 0 | 其他武器熟练度 |
| 0x18-0x1F | ... | Growth rates | 成长率 |
| 0x20-0x87 | ... | Other fields | 其他字段（道具、技能等） |

### 生成代码

通过 `tests/testutil/fixture.go` 生成：

```go
f := NewFE8UFixture()
f.AddUnit(Unit{
    NameID: 0x0002,
    CharID: 0x03,
    Class:  0x07,
    Level:  1,
    HP:     16,
    // ...
})
f.AddClass(Class{ID: 0x07, Name: "Eirika Lord"})
f.AddItem(Item{ID: 0x01, Name: "Iron Sword"})
f.Save("testdata/minimal_fe8u.bin")
```

## `minimal_fe6.bin`

最小的可测试 FE6 ROM。

### 与 FE8U 的差异

| 项目 | FE8U | FE6 | 说明 |
|-----|------|-----|------|
| Unit stride | 0x88 | 0x88 | 相同 |
| Class stride | 0x54 | 0x48 | FE6 更短（无转职加成） |
| Item stride | 0x24 | 0x20 | FE6 更短（无武器经验） |
| Text encoding | Huffman | Huffman | 相同（但文本表位置不同） |
| Pointer locations | 见 FE8U 表 | 不同 | FE6 的指针表在不同位置 |

### 结构

| 偏移 | 长度 | 内容 | 说明 |
|-----|------|------|------|
| 0x0000 | 0xA0 | GBA header | 标准头部 |
| 0x00A0 | 0x0C | Game ID | `FIREEMBLEM6` (FE6) |
| 0x017FC8 | 4 | Unit table pointer | → 0x800000 |
| 0x017FD0 | 4 | Class table pointer | → 0x800200 |
| 0x017FE8 | 4 | Item table pointer | → 0x800400 |
| ... | ... | 数据表 | 类似 FE8U，但字段布局不同 |

## `corrupt_header.bin`

用于测试错误处理，包含非法标头。

```
0x00A0: "NOTAFIREGAME" (非法游戏 ID)
```

期望：`Open()` 返回 `ErrVersion`

## `overlapping_tables.bin`

用于测试表重叠检测。

```
Unit table: 0x800000-0x800200
Class table: 0x800100-0x800300  (与 Unit 重叠)
```

期望：`Inspect()` 返回错误或警告

## 更新 Fixture 的流程

1. 修改 `tests/testutil/fixture.go` 的生成器
2. 运行 `go test ./tests/testutil -update` 重新生成
3. 更新本文档的结构说明
4. 提交生成器代码和生成的 `.bin` 文件

## 已知限制

这些 fixture 不包含：
- 完整的文本表（仅 stub）
- 图像数据（头像、图标等）
- 事件脚本
- 音频数据

如果需要测试这些功能，应该：
- 扩展 fixture 生成器
- 或者使用真实 ROM 的集成测试
