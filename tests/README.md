# 测试

默认测试放在对应包的 `*_test.go` 中，用例及其验证范围以测试代码为准。合成 ROM 由 [internal/testutil](../internal/testutil) 构造，规则见 [fixtures.md](../testdata/fixtures.md)；需要真实 ROM 的可选检查放在 [integration](integration)，用 build tag 隔离。本文只维护运行方式、测试约束和本地数据准备；新增用例不需要同步登记。

## 运行

在仓库根目录执行：

```sh
# 文档、会话、空间和图像核心
go test ./rom ./game ./space ./gba

# 所有默认测试
go test ./...

# 涉及并发或文档历史时
go test -race ./rom ./game ./space

# 可选：使用本地真实 ROM，只修改临时副本
go test -tags=integration -v ./tests/integration
```

## 约束

- 验证实际领域行为、算法和数据结构，不按函数逐一编写单元测试，不复制一套实现来验证产品实现。
- 编解码使用独立构造的固定数据和期望结果，避免仅靠往返测试证明正确。
- 不新增 UI 或模拟器测试。文件保存重开与合成 ROM 检查不代表游戏运行正确。
- 修改后运行相关测试及项目检查，通过后结束验证，不无限扩大测试范围。

产品功能的实现状态统一记录在[覆盖矩阵](../../docs/coverage.md)，不根据测试数量推算功能覆盖率。
