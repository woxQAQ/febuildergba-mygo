# 计划 001：图标替换只豁免自己的根指针声明

> 已完成并归档（2026-11-08）。正文保留当次实施计划与验证证据，不再作为待办或当前开发规则；其中 `plans/` 指归档前位置。当前维护入口为[架构](../../../docs/architecture.md)、[使用指南](../../../docs/user-guide.md)和[测试说明](../../../tests/README.md)。

> 执行者从本文件开始即可实施，不依赖审计会话或其他计划。按步骤验证；出现停止条件时报告实际证据，不扩大修改范围。完成后更新 `plans/README.md` 中的 001 状态及本文件的执行记录。
>
> 首先执行漂移检查：`git diff --stat 534dc8514e0799542e6f82f5345733a4c3e985ed..HEAD -- game/image_references.go game/graphics/editor_test.go`。同时检查 `git status --short`。有变动时逐段比对下方摘录；独立的新改动可以保留，相关逻辑不再匹配时停止并刷新计划。

## 状态

- 状态：DONE；2026-11-08 实施及所需验证通过，已归档。
- 优先级：P1。
- 工作量：S，包含合成 ROM 回归及常规检查。
- 修复风险：LOW；只收窄已有豁免，须证明正常图标替换仍可用。
- 类别：bug；审计发现 1，审计阶段源码置信度 HIGH；实际复现与验证见执行记录。
- 依赖：无。
- 编写基线：`534dc85`（完整 SHA 见上方），2026-11-08。
- 触发条件：维护道具图标替换时，修正同址其他资源声明被跳过的问题；本计划已由维护者选定。

## 问题与目标

替换一个道具图标会追加整份图标数据，并改写图标表的根指针。当前会话为了允许该写入，按四字节区间排除自身声明，却也排除了其他领域在同址声明的站点或载荷。异常或自定义 PLIST 布局因此可能让图标替换改变另一资源的入口。

目标是只排除图标根指针自己的站点声明；任何其他所有者的同址声明和部分重叠声明都继续保护。正常五版 PNG 替换、图标 0、未知字节保留、撤销重做与保存重开继续使用原有流程。

## 当前实现与术语

项目是 Go 1.27.3 + MyGo v0.8.3 的原生 ROM 编辑器，首阶段面向 macOS。领域包独立于 UI，工具由 Nix 提供；直接调用 Go 时使用 `GOTOOLCHAIN=local`，不安装新依赖。

- `game/image_references.go:10`：为 PNG 写入汇总保护区和已知用途。
- `game/claims.go:15`、`:139` 附近：会话汇总声明，图标根站点的 Owner 是 `space.Owner{Resource: "itemicon"}`，且 `Site: true`。
- `game/maps/claims.go:62`：地图 PLIST 的具体站点有独立 Owner，即使站点等于图标根指针也应保留。
- `game/graphics/replacement.go:79`：`validTargets` 检查这些保护区，冲突按已有 `edit.ErrLayout` 返回，规划失败不写文档。
- `internal/testutil/images.go`：五版独立图像样本，`ImageFixture`、`ImageLayouts`、`ImagePNG` 可直接复用。

当前错误条件，摘自 `game/image_references.go:17`：

```go
for _, claim := range s.claims(data) {
    if ref.Kind == graphics.ItemIcon &&
        claim.Range == (space.Range{Start: g.icons.Pointer, End: g.icons.Pointer + 4}) {
        continue
    }
    result.Ranges = append(result.Ranges, claim.Range)
}
```

`Owner` 是资源的比较身份，`Claim.Site` 表示指针站点，`Host` 表示容纳站点的记录。这里的 `"itemicon"` 与 `graphics.ItemIcon` 的字符串值 `"item_icon"` 不同；不要用后者替代前者比较 Owner。该差异是当前实现事实，本任务不重命名资源体系。

架构约束（`docs/architecture.md` 的“资源声明”）：

> “指针写入与声明重叠时，只允许写入者自己的站点声明或其宿主记录……其余均冲突。”

测试遵循 `game/graphics/editor_test.go` 中 `TestAssetImportRejections` 和 `TestAssetReplacementWorkflows`：通过 `game.Open` 装配真实配置，用独立样本调用 `Images.Plan/Apply`，不直接测试私有筛选函数。例如现有失败检查：

```go
if s.Document.State().Revision != before.Revision ||
    !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
    t.Fatal("rejected import changed the document")
}
```

本次还应比较完整 `rom.State`，以覆盖文件长度、脏状态及撤销／重做目标。

## 允许修改的范围

- `game/image_references.go`。
- `game/graphics/editor_test.go`；在现有外部包 `graphics_test` 中增加工作流。
- 本计划的执行记录及 `plans/README.md` 的状态行。

只读参考：`AGENTS.md`、`docs/architecture.md`、`docs/error-design.md`、`tests/README.md`、`game/claims.go`、`game/maps/claims.go`、`game/fe8u.go`、`internal/testutil/images.go`、`internal/testutil/fixture.go`、`game/chapter/format.go`。

不修改 `FEBuilderGBA/`、公共错误码、版本地址、PNG 编码规则、Claim 的通用算法、其他领域的所有者名称、UI、翻译和依赖。测试特殊输入在所属测试中构造，不修改共享样本来掩盖冲突。

## 命令

以下直接 Go 命令采用 Nushell 环境语法；`just` 已自行导出 `GOTOOLCHAIN=local`。

| 用途         | 命令                                                                                                                         | 成功预期                                    |
| ------------ | ---------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------- |
| 现有图像流程 | `with-env { GOTOOLCHAIN: local } { go test ./game/graphics -run '^TestAsset' -count=1 }`                                     | 退出 0                                      |
| 新回归       | `with-env { GOTOOLCHAIN: local } { go test ./game/graphics -run '^TestItemIconReplacementKeepsForeignClaims$' -count=1 -v }` | 修复前按预期断言失败，修复后所有子场景 PASS |
| 图像领域整体 | `with-env { GOTOOLCHAIN: local } { go test ./game/graphics ./game/itemicon ./game/portrait -count=1 }`                       | 退出 0                                      |
| 完整常规检查 | `just check`                                                                                                                 | lint、vet、翻译、测试和两种构建全部成功     |
| 修改范围     | `git diff --check`、`git diff --name-only`、`git ls-files --others --exclude-standard`                                       | 无空白错误，文件均在允许范围                |

`just check` 包含自动修复；执行者应在保留原修改的隔离实现工作区运行，检查额外 diff，不将无关修复混入本任务。此计划的编写者没有运行自动修复。

## 实施步骤

### 1. 核对基线与正常入口

完成顶部漂移检查，阅读只读参考中与本任务有关的段落。核对 `game/claims.go` 的图标根 Owner 和 `Site`；核对当前 `graphics.ItemIcon` 的值及 PNG 的错误转换。

验证：运行“现有图像流程”，必须退出 0。若当前已有相关失败，先报告基线问题。

### 2. 用实际 PNG 规划构造同址冲突

在 `game/graphics/editor_test.go` 增加 `TestItemIconReplacementKeepsForeignClaims`，至少包含 `foreign-plist-site` 和 `normal-root-only` 两个场景。

固定构造建议：

1. 使用 `testutil.ImageFixture(t, 0)`，这是 FE8U 合成样本。`testutil.ImageLayouts[0].IconPointer` 是图标根站点，当前固定为 `0x36B4`。
2. 使用 `f.GetPosition("chapters")` 找到首个章节；在 `+4` 写入图像资源 ID `1`。这是章节原生 `map_object_id` 字段，不从生产私有配置生成预期。
3. 让 FE8U 地图图像 PLIST 根站点 `0x19968` 指向 `IconPointer - 4`。这样 ID 1 的 PLIST 站点正好等于图标根站点，原来的图标指针和值仍完整可读。`maps.Format.Claims` 会为该四字节站点声明独立 Owner；不要求整个地图预览可解码。
4. 使用现有样例中的 16×16 PNG：透明背景，`(0,0)` 为不透明绿色，颜色属于现有共享调色板，且不同于原图。
5. 打开样本后、规划前，捕获完整字节和 `Document.State()`。调用 `Images.Plan(graphics.Ref{Kind: graphics.ItemIcon, ID: 0}, png)`，必须得到 `edit.ErrLayout` 且无计划；规划前后整个快照、长度、修订、脏状态和历史目标完全相同。
6. 正常场景使用未改 PLIST 的独立样本，同一 PNG 应可规划和应用；检查只更新允许根指针、旧图标数据保留，并能一次撤销／重做。五版完整成功覆盖由原有 `TestAssetReplacementWorkflows` 继续承担。

这个场景验证同一物理站点上的独立已知消费者，不靠任意“形似指针”的数据证明资源归属。

验证：运行“新回归”。修复前 `foreign-plist-site` 应因规划意外成功而失败；`normal-root-only` 应通过。若因 PNG 颜色、表读取或版本校验先失败，修正样本，不能把前置错误当作冲突证据。

### 3. 收窄排除条件

在原条件中同时要求：目标是道具图标、区间精确等于图标根站点、`claim.Site` 为真、`claim.Owner` 精确等于会话声明的图标根 Owner。其他所有声明均追加到保护区。

保持原有保护区、站点集合和 PNG 领域错误处理；不增加“地图除外”等领域成对分支，也不直接修改 `validTargets` 来绕开冲突。

验证：运行“新回归”和“现有图像流程”，均退出 0。

### 4. 完成领域与仓库验证

运行“图像领域整体”和 `just check`，再检查修改范围。此变更没有 UI 或并发改动，不默认启动原生界面、模拟器或真实 ROM 集成；不需要将领域结果称为 UI 或游戏内证据。

验证：以上命令全部成功，`git diff --check` 无输出，范围检查仅出现允许文件。更新索引状态并记录实际命令结果；有跳过项时保留 TODO／BLOCKED，不写成 DONE。

## 完成标准

- [x] 新增实际 PNG 规划回归，修复前能证明错误豁免，修复后拒绝同址其他 Owner。
- [x] 正常自身根站点可以替换；五版原有图像工作流全部通过。
- [x] 冲突失败不改变字节、大小、修订、脏状态或撤销／重做目标。
- [x] 三组领域测试及 `just check` 退出 0。
- [x] `git diff --check` 通过，修改范围合规，索引已更新。

## 停止条件

- 图标根声明的 Owner 或 `Site` 与本文件事实不符。
- 同址 PLIST 场景没有走到 PNG 规划的归属检查，或在基线上已经被拒绝；先报告具体原因并刷新计划。
- 需要重命名资源、修改通用 Claim 规则、版本地址或 PNG 编码器才能修复。
- 合理修正后同一验证仍连续失败，或出现无法解释的正常图标拒绝。

## 维护与评审

审查重点是“豁免声明身份”而非“豁免地址”。今后新增消费者，只要提供 Claims，就应自动得到保护。`graphics.ItemIcon` 和根声明 Resource 当前采用不同词汇，任何统一命名应单独评估，不与此安全边界修复混合。

若需要隔离分支，可使用 `advisor/001-itemicon-claim-owner`。本任务不自动提交或推送；被明确要求提交时沿用仓库简短动词英文风格，例如 `preserve foreign claims at the item icon root`。

## 执行记录

2026-11-08：已实现并完成源码复核；全部实现结束后统一运行的 `just check` 退出 0，lint、MyGo vet、翻译检查、默认测试及默认／无 CGO 构建全部通过。`git diff --check` 通过，最终修改范围已复核。新增 `TestItemIconReplacementKeepsForeignClaims` 的正常根站点与外部 PLIST 同址两场景；修改前外部场景因规划意外成功失败，修改后图像、道具图标、头像三个领域包的 `-count=1` 测试通过。失败原子性、旧载荷保留与一次撤销／重做均由实际 PNG 工作流断言。未运行真实 ROM、原生 UI、race 或游戏验证。
