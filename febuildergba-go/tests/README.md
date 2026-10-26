# 测试

测试按 Go 习惯放在对应包的 `*_test.go` 中，验证实际领域行为、算法和数据结构。修改后运行与本次变化相关的测试；通过相关测试和项目检查后结束验证，不要求打开 UI 或模拟器。

在 `febuildergba-go/` 下执行：

```sh
# 文档、会话、空间和图像核心
go test ./internal/rom ./internal/game ./internal/space ./internal/gba

# 所有默认测试
go test ./...

# 涉及并发或文档历史时
go test -race ./internal/rom ./internal/game ./internal/space

# 可选：使用本地真实 ROM，只修改临时副本
go test -tags=integration ./tests/integration
```

## 当前覆盖

| 位置                               | 验证内容                                                                                                                                |
| ---------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| `internal/rom/document_test.go`    | 实际文档的修订、保存点、撤销重做、越界／重叠／过期写入、并发快照                                                                        |
| `internal/rom/expansion_test.go`   | 扩容与写入的原子性、跨原文件末尾写入、FF 填充、撤销恢复原长度、重做与保存点、撤销后的历史分支                                           |
| `internal/game/game_test.go`       | 实际 FE8U／FE8J／FE7U／FE7J／FE6 会话的批量字段写入、失败不留修改、单次撤销、保存重开和手动布局检查                                     |
| `internal/game/relocation_test.go` | 五版本的十五种表搬迁；隐藏／结束记录、已知引用、源数据保留、过期／其他文档预览、声明区间和布局冲突                                      |
| `internal/game/fe7_test.go`        | 五版本实际会话的压缩／anti-Huffman 名称与恢复地址；FE7J 紧凑名字形、FE7 头像／职业卡／图标像素、253 条人物边界、字段布局及非法 ROM 拒绝 |
| `internal/game/name_edit_test.go` | 五版实际会话的固定 Huffman 样本、ASCII／Shift-JIS 字符边界、紧凑职业名字形、原生填充码、共享 ID／独立别名、历史与保存重开、非法／有损输入、树及容量边界 |
| `internal/space/write_test.go`     | 末尾分配、对齐、声明区间、共享及内部引用、区间冲突和 32 MiB 容量边界                                                                    |
| `internal/gba/graphics_test.go`    | 产品 `DecompressLZ77` 的固定编码样本、重叠回溯、输出上限和截断／损坏输入                                                                |
| `tests/testutil`                   | 合成 ROM 的标头、表指针和字段经实际适配器解析，说明见 [fixtures.md](../testdata/fixtures.md)                                            |
| `tests/e2e`                        | 保留已有的文件保存重开、撤销后保存和外部修改检查；移除了未调用实际会话的占位用例                                                        |
| `tests/integration`                | 可选五版原版 ROM 的人物／道具表搬迁、改名与历史、保存重开后继续改名；三个日文版本另验证紧凑职业名；缺失提示显示实际路径                                                                |

这里的保存／重开检查验证文件和领域数据，不执行 UI、系统对话框或游戏。未新增 UI 或模拟器测试。

## 测试边界

- LZ77 测试直接调用产品解码器；没有为了测试往返而编写假的压缩／解压实现。当前没有 LZ77 编码器，因此不声明压缩往返覆盖。
- Huffman 名称写入通过固定树及手工期望字节验证，同时经产品读取器往返；覆盖单／双字节完整分词、Shift-JIS 字符边界、紧凑字形原始码、0x1F 填充、七／八位结束码及读取余量，不使用测试专用编码器。原版字符集之外的字库、更多损坏树、头像状态和其他图像格式仍待具体功能补充。
- 并发快照测试不等于“保存不持锁”的确定性验证。保存实现仍使用锁内快照、锁外 I/O；当前测试不声称验证了所有保存时序。
- 合成 ROM 只证明已知布局的行为；不能据此确认未知补丁引用完整、ROM 能启动或所有游戏逻辑正确。
- 不以源码文件数量、测试数量或默认测试全绿推算完整功能覆盖率。实际能力见 [覆盖矩阵](../../docs/coverage.md)。

## 可选真实 ROM

文件放在 Go 项目目录 `febuildergba-go/.rom/`，不纳入版本管理。测试从 `tests/integration` 通过 `../../.rom` 读取：

- `Fire Emblem - The Sacred Stones (USA, Australia).gba`
- `Fire Emblem - Seima no Kouseki (Japan).gba`
- `Fire Emblem (USA, Australia).gba`
- `Fire Emblem - Rekka no Ken (Japan).gba`
- `Fire Emblem - Fuuin no Tsurugi (Japan).gba`

缺少文件时相应用例跳过；文件存在却无法解析或搬迁时测试失败，不将其标成通过。测试输出的通过与跳过需要分别报告。
