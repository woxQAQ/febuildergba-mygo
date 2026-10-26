# 测试架构

本文档定义 FEBuilderGBA-MYGO 的测试策略、组织结构和长期约定。

## 设计原则

1. **测试核心行为，而非实现细节**：测试对外契约和安全保证，不测内部算法
2. **测试是软件，需要设计**：测试有清晰的层次和职责划分
3. **不允许无意义的单元测试**：不测纯函数的琐碎逻辑，测试必须保护关键行为
4. **Agent 可验证**：测试提供快速反馈，让 Agent 知道是否破坏了核心逻辑

## 测试层次

```
┌─────────────────────────────────────────┐
│  集成测试 (integration tests)            │  真实 ROM，完整工作流
│  tests/integration/*_test.go            │  go test -tags=integration
├─────────────────────────────────────────┤
│  端到端行为 (e2e invariants)           │  跨模块的状态一致性
│  tests/e2e/*_test.go                    │  合成 fixture
├─────────────────────────────────────────┤
│  核心行为 (core invariants)            │  单模块的关键保证
│  internal/*/相应的测试文件           │  快速，自包含
├─────────────────────────────────────────┤
│  测试基础设施 (test infrastructure)      │  Fixture 生成、helpers
│  tests/testutil/                        │  被所有测试使用
└─────────────────────────────────────────┘
```

### 核心行为测试

位置：`internal/<package>/相应的测试文件`

测试单个模块必须维护的行为，这些是系统正确性的基础保证。测试失败意味着核心逻辑被破坏。

**特点**：
- 快速（< 100ms per package）
- 自包含（不依赖外部文件）
- 使用合成 fixture（`testdata/`）
- 每次 `go test ./...` 自动运行

**覆盖的行为**：见下文"行为清单"。

### 端到端行为测试

位置：`tests/e2e/*_test.go`

测试跨模块的状态一致性，例如：
- 游戏会话的操作必须反映到 ROM 文档
- 批量编辑必须产生单次撤销操作
- 保存后重新打开，修改必须持久化

**特点**：
- 中等速度（< 1s per test）
- 使用合成 fixture
- 每次 `go test ./...` 自动运行

### 集成测试

位置：`tests/integration/*_test.go`

使用真实 ROM 验证完整工作流，例如：
- 打开 FE8U → 修改人物 HP → 保存 → 用旧版验证字节
- 打开 FE6 → 批量修改职业 → 撤销 → 验证回滚

**特点**：
- 较慢（可能需要几秒）
- 依赖 `.rom/` 目录的真实 ROM
- 使用 `//go:build integration` 标记
- 仅在 `go test -tags=integration ./...` 时运行

## 行为清单

这些行为是系统的长期约定，任何破坏这些行为的修改都是 bug。

### ROM 文档行为 (`internal/rom/相应的测试文件`)

| ID | 行为 | 测试函数 | 为什么重要 |
|---|---|---|---|
| ROM-INV-1 | `revision` 单调递增 | `TestDocument_RevisionMonotonic` | 确保 stale 检测正确工作 |
| ROM-INV-2 | `stateID` 正确跳转 | `TestDocument_StateTransition` | Undo/redo 必须回到正确的状态 |
| ROM-INV-3 | `savedID` 对应未修改状态 | `TestDocument_SavedState` | 用户必须能准确看到是否有未保存修改 |
| ROM-INV-4 | Undo 完全恢复字节 | `TestDocument_UndoRestoresBytes` | 撤销不能丢失数据 |
| ROM-INV-5 | 重叠写入被拒绝 | `TestDocument_OverlappingWrites` | 防止模糊的写入语义 |
| ROM-INV-6 | Stale revision 被拒绝 | `TestDocument_StaleRevision` | 防止基于过期状态的修改 |
| ROM-INV-7 | 保存不持有状态锁 | `TestDocument_SaveConcurrency` | 保存期间不阻塞读取和新修改 |

### 字段解析行为 (`internal/game/相应的测试文件`)

| ID | 行为 | 测试函数 | 为什么重要 |
|---|---|---|---|
| FIELD-INV-1 | Value 在 Bounds 内 | `TestField_ValueInBounds` | 防止越界读取 |
| FIELD-INV-2 | 符号扩展正确 | `TestField_SignedExtension` | 确保有符号值正确解析 |
| FIELD-INV-3 | 表布局不越界 | `TestTable_LayoutBounds` | 防止读取 ROM 外的内存 |

### 编码行为 (`internal/gba/相应的测试文件`, `internal/game/相应的测试文件`)

| ID | 行为 | 测试函数 | 为什么重要 |
|---|---|---|---|
| ENC-INV-1 | LZ77 往返一致 | `TestLZ77_RoundTrip` | 解压缩必须可逆 |
| ENC-INV-2 | Huffman 往返一致 | `TestHuffman_RoundTrip` | 文本解码必须可逆 |
| ENC-INV-3 | 解码边界安全 | `TestDecode_BoundarySafe` | 非法输入不能导致 panic 或越界 |

### 会话操作行为 (`internal/game/相应的测试文件`)

| ID | 行为 | 测试函数 | 为什么重要 |
|---|---|---|---|
| SES-INV-1 | 批量操作原子性 | `TestSession_BatchAtomic` | 批量修改必须全成功或全失败 |
| SES-INV-2 | 字段修改反映到文档 | `TestSession_FieldWritesPersist` | 修改必须实际写入 ROM |

## 测试数据组织

```
febuildergba-go/
  testdata/                      # 合成 fixture（提交到 git）
    minimal_fe8u.bin             # 最小可测的 FE8U ROM（~1KB）
    minimal_fe6.bin              # 最小可测的 FE6 ROM
    fixtures.md                  # 说明每个 fixture 的结构
  tests/
    testutil/                    # 测试工具和 helpers
      fixture.go                 # Fixture 生成器
      assertions.go              # 自定义断言
      rom.go                     # ROM 测试辅助函数
    e2e/                         # 端到端行为测试
      edit_workflow_test.go
    integration/                 # 真实 ROM 集成测试
      fe8u_test.go               # 使用 .rom/ 中的真实 ROM
      fe6_test.go
      compare_legacy_test.go     # 和旧版对比（可选）
    README.md                    # 本文档
  .rom/                          # 真实 ROM（gitignore）
    Fire Emblem - The Sacred Stones (USA, Australia).gba
    Fire Emblem - Fuuin no Tsurugi (Japan).gba
```

## Fixture 设计

合成 fixture 必须：
1. **最小化**：仅包含测试所需的必要字节
2. **自解释**：在 `testdata/fixtures.md` 中文档化结构
3. **可维护**：通过 `tests/testutil/fixture.go` 的生成器创建，不手工编辑二进制

示例：`minimal_fe8u.bin` 包含：
- 0x00-0xA0: GBA ROM header（FE8U 标识）
- 0xA0-0xAC: 游戏标头（版本标识）
- 0x800000: 一个人物记录（0x88 字节）
- 0x803D90: 指向人物表的指针

生成方式：
```go
f := testutil.NewFixture()
f.WriteHeader("FE8U")
f.WritePointer(0x803D90, 0x800000)
f.WriteUnitRecord(0x800000, testutil.UnitData{HP: 20, Class: 1})
f.Save("testdata/minimal_fe8u.bin")
```

## 运行测试

### 日常开发（快速反馈）

```bash
# 运行所有核心行为测试（< 1s）
go test ./internal/...

# 运行所有自包含测试（包括 e2e）
go test ./...
```

### 完整验证

```bash
# 运行所有测试（包括真实 ROM 集成测试）
go test -tags=integration ./...

# 带覆盖率
go test -tags=integration -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### CI 配置

```yaml
# 建议的 CI 配置
- name: Unit tests
  run: go test ./...
  
- name: Integration tests (if ROM available)
  run: |
    if [ -f .rom/fe8u.gba ]; then
      go test -tags=integration ./...
    fi
```

## Agent workflow

当 Agent 修改代码时，应该：

1. **修改后立即验证**：
   ```bash
   go test ./internal/rom ./internal/game ./internal/gba
   ```

2. **如果测试失败**：
   - 查看失败的测试函数名（例如 `TestDocument_UndoRestoresBytes`）
   - 参考本文档的"行为清单"，理解破坏了哪个约定
   - 修复代码或更新测试（如果行为本身需要演化）

3. **如果是新功能**：
   - 添加相应的行为测试
   - 更新本文档的"行为清单"

4. **定期完整验证**：
   ```bash
   go test -tags=integration ./...
   ```

## 测试演化规则

### 何时添加新的行为测试

当满足以下任一条件时：
1. 发现一个 bug，且该 bug 违反了某个隐含的行为
2. 添加新的核心模块（如空间分配、引用管理）
3. 重构改变了内部实现，但对外契约不变

### 何时修改现有行为测试

仅当以下情况：
1. 行为本身需要演化（例如，决定允许某些重叠写入）
2. 测试有 bug（测试本身写错了）

**禁止**因为"测试难通过"而修改行为测试。如果行为测试失败，说明代码违反了约定。

### 何时删除行为测试

极少情况。仅当：
1. 相关模块被完全移除
2. 行为被更强的行为取代

## 不测试的内容

以下内容**不需要**测试：

1. **纯 UI 布局**：通过手动验证，不写自动化测试
2. **琐碎的纯函数**：例如简单的格式化、类型转换
3. **第三方库的行为**：假定 MyGo、Go 标准库是正确的
4. **性能**：性能测试是独立的 benchmark，不在单元测试中

## 常见问题

### Q: 为什么不用 table-driven tests 测所有边界情况？

A: 我们使用 table-driven tests，但聚焦在**行为**上。不是为了覆盖率而测，而是为了保护关键约定。

### Q: 为什么不用 mock？

A: 核心模块之间的依赖是真实的领域关系，用 mock 会让测试脱离实际。我们用真实依赖 + 合成 fixture。

### Q: 集成测试依赖真实 ROM，CI 环境没有怎么办？

A: 集成测试用 build tag 标记，CI 可以跳过。核心正确性由行为测试保证，集成测试是额外的信心。

### Q: 测试覆盖率目标是多少？

A: 没有固定目标。我们测**关键行为**，不追求行数覆盖率。如果所有行为都有测试，覆盖率自然会合理。

## 参考

- [工程原则](../README.md#工程原则)：测试也是软件，需要设计
- [重建计划](../../docs/rebuild-plan.md)：功能范围和依赖顺序
- [覆盖矩阵](../../docs/coverage.md)：实现状态
