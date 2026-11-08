# 计划 002：表搬迁预览绑定完整解释设置

> 已完成并归档（2026-11-08）。正文保留当次实施计划与验证证据，不再作为待办或当前开发规则；其中 `plans/` 指归档前位置。当前维护入口为[架构](../../../docs/architecture.md)、[使用指南](../../../docs/user-guide.md)和[测试说明](../../../tests/README.md)。

> 本文件是独立交接计划。逐步完成验证，出现停止条件时报告，不用重新捕获输入来延长旧计划寿命。完成后更新 `plans/README.md` 的 002 状态与本文件执行记录。
>
> 首先执行：`git diff --stat 534dc8514e0799542e6f82f5345733a4c3e985ed..HEAD -- game/records/relocation.go game/records/relocation_test.go`，以及 `git status --short`。相关文件有变化时比对当前实现摘录；不匹配则停止并刷新计划。

## 状态

- 状态：DONE；2026-11-08 实施及所需验证通过，已归档。
- 优先级：P1。
- 工作量：S。
- 修复风险：LOW；使用已有解释身份机制，并保留现有手动表操作限制。
- 类别：bug；审计发现 2，审计阶段源码置信度 HIGH；实际复现与验证见执行记录。
- 依赖：无。与其他领域写入计划没有硬依赖。
- 编写基线：`534dc85`，2026-11-08。
- 触发条件：修正表搬迁预览在其他表布局变化后仍可应用的问题；维护者已选定。

## 问题与目标

表搬迁规划使用当前手动表布局收集保护范围，因此它的安全性取决于解释设置。当前却使用忽略解释身份的 `CaptureBytes()`，应用时只检查被搬迁表自身是否进入手动模式。规划后将另一张表手动定位到原先允许复用的目标区间，旧预览仍可能覆盖新声明的表。

本计划使搬迁预览与文档身份、修订、解释设置一起绑定。改变任意表的有效解释设置后，旧计划应通过现有 `rom.ErrStale` 路径拒绝；恢复完全相同设置且文档修订没有变化时，沿用已有 Stamp 的身份语义。当前 UI 默认追加且审核弹层限制了触发机会，回归必须通过公开领域 API 覆盖该边界。

## 当前实现与约束

项目使用 Go 1.27.3、MyGo v0.8.3，领域包不依赖 UI。工具来自 Nix；直接 Go 命令设置 `GOTOOLCHAIN=local`，不增加依赖。

术语：`edit.Stamp` 绑定文档、修订与解释身份；`MovePlan` 是冻结写入与影响信息的搬迁预览；`FreeRange` 是有明确来源的可复用区间，空白字节不构成空间证据。

`game/records/relocation.go:46` 当前捕获：

```go
input := s.context.CaptureBytes()
if input.Stamp().Revision() != s.binding.Revision() {
    return nil, rom.ErrStale
}
```

同一函数又将手动表区间加入 `move.Reserved`，说明解释设置参与了写入安全性。返回计划使用该输入的 stamp：

```go
commit: edit.NewCommit(
    input.Stamp(),
    "move."+string(kind),
    plan.Edit,
),
```

`game/records/relocation.go:112` 的应用限制：

```go
if _, manual := s.layouts[move.kind]; manual {
    return edit.ErrLayout
}
if err := move.commit.Apply(s.context); err != nil {
    return err
}
```

`game/edit/context.go:67` 的 `CaptureBytes()` 显式移除解释身份；`Capture()` 保留它。`Stamp.Check` 已在解释不同时返回 `rom.ErrStale`，无需新的状态计数器或错误码。`records.syncInterpretation` 使用全部手动表设置构造身份；恢复相同设置会恢复相同解释身份，但撤销会增加文档修订，不能复活旧计划。

架构明确要求（`docs/architecture.md:111`）：

> “仅原始字节操作可用 `CaptureBytes()` 忽略布局；表、图像、部署、事件和地图仍须校验解释设置。”

匹配现有测试约定：`game/records/relocation_test.go` 使用 `records_test` 外部包，通过 `testutil.Open` 装配会话，并验证 `PlanTableMove/ApplyTableMove`、字节、历史与保存重开。保留原有 `TestTableMoveRejectsStaleOrForeignPreview` 对被搬迁表进入手动模式时 `edit.ErrLayout` 的断言；这是一个单独的操作前提，不必改变其错误优先级。

## 范围

允许修改：

- `game/records/relocation.go`。
- `game/records/relocation_test.go`。
- 本计划执行记录与 `plans/README.md` 的状态行。

只读参考：`AGENTS.md`、`docs/architecture.md`、`docs/error-design.md`、`tests/README.md`、`game/edit/context.go`、`game/edit/commit_workflow_test.go`、`game/records/editor.go`、`game/records/layout_test.go`、`game/claims_test.go`、`internal/testutil/fixture.go`。

不修改 `edit.Stamp` 的通用语义、原始字节操作、空间分配器、ROM 历史实现、UI、翻译、版本规则、旧版子模块或其他表的手动定位行为。当前架构约束已准确，不将其改写为允许例外。

## 命令

命令在仓库根目录执行；以下环境语法为 Nushell。

| 用途               | 命令                                                                                                                   | 成功预期                 |
| ------------------ | ---------------------------------------------------------------------------------------------------------------------- | ------------------------ |
| 现有搬迁流程       | `with-env { GOTOOLCHAIN: local } { go test ./game/records -run '^TestTableMove' -count=1 }`                            | 退出 0                   |
| 新布局生命周期回归 | `with-env { GOTOOLCHAIN: local } { go test ./game/records -run '^TestTableMoveInterpretationLifecycle$' -count=1 -v }` | 修复后全部子场景 PASS    |
| 相关领域工作流     | `with-env { GOTOOLCHAIN: local } { go test ./game/records ./game/edit ./space ./game -count=1 }`                       | 退出 0                   |
| 常规检查           | `just check`                                                                                                           | 全部 recipe 成功         |
| 修改范围           | `git diff --check`、`git diff --name-only`、`git ls-files --others --exclude-standard`                                 | 无空白错误，只有允许文件 |

`just check` 包含自动修复，应在隔离实现工作区执行并检查产生的所有 diff。本计划不改变 `rom.Document` 历史或并发，不额外要求 race；若执行范围扩大到这些内容，先停止并重新评估范围。

## 步骤

### 1. 核对当前生命周期

完成漂移检查，确认 `UseLayout` 改变解释身份而不改变文档修订，`Capture()` 已提供所需绑定。运行现有搬迁流程。

验证：顶部两个 Git 命令显示可解释的基线；“现有搬迁流程”退出 0。

### 2. 增加其他表改变布局后的失败场景

在 `game/records/relocation_test.go` 新增 `TestTableMoveInterpretationLifecycle`，使用 `testutil.Games()` 的五版合成数据。每个子场景使用独立会话，不从生产私有配置反推预期。

至少覆盖：

| 场景                          | 构造与动作                                                                                                                                                          | 预期                                                           |
| ----------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------- |
| `append-other-layout`         | 为 Items 生成默认追加计划；把 Units 设置为合法手动布局，使用 ROM 内独立区间，Count 可设为 1                                                                         | 应用旧计划返回 `rom.ErrStale`，全部字节与完整 `rom.State` 不变 |
| `reuse-other-layout`          | 使用带非空 Source 的 `FreeRange [0x400000,0x401000)` 生成 Items 搬迁计划；随后将 Units 的合法手动布局设到 `move.Destination()`，Stride 使用该版本原有值、Count 为 1 | 旧计划被拒绝，新手动区域、文件长度、历史与修订均保留           |
| `restore-same-interpretation` | 为 Items 规划后将 Units 设为手动，再 `UseAutomaticLayout(Units)` 恢复原设置，期间不写 ROM                                                                           | 原计划恢复可用；一次应用、一次撤销和重做完整恢复字节与长度     |
| `undo-does-not-revive`        | 为 Items 规划；做一次真实字段修改后撤销并 Refresh，原字节恢复但修订增加                                                                                             | 原计划仍返回 `rom.ErrStale`                                    |

手动布局通过公开 API 设置；不要直接写 `s.layouts` 或 `Context.Interpret`。复用场景在改变布局后重新捕获作为失败比较基准，确保测试区分“布局操作本来就不改字节”和“旧计划不得提交”。可再检查新规划会因当前占用区间拒绝复用，验证不是关闭全部搬迁。

测试模式参考同文件 `TestTableMoveUsesDeclaredSpace` 及 `TestTableMoveRejectsStaleOrForeignPreview`。断言使用 `errors.Is`；不解析 `Error()`。

验证：运行新回归，基线至少两个“other-layout”场景必须因旧计划意外被接受而失败。若失败源于布局无效或前置样本错误，先修正构造。正常恢复场景必须可达。

### 3. 将搬迁改为解释绑定

把 `PlanTableMove` 的输入改为 `s.context.Capture()`，保持同次捕获的字节和 stamp 一起传入冻结提交。保留修订检查、预演读回、源区保护、现有应用时的手动表前提和历史标签。

更新“Byte-bound moves”之类不再准确的局部注释，使其解释实际操作前提。禁止在 `ApplyTableMove` 重新构建计划、重新获取 stamp，或只维护单张表的额外 generation 来绕开完整解释身份。

验证：运行新回归和现有搬迁流程，全部退出 0；被搬迁表进入手动模式的原有错误测试仍通过。

### 4. 验证并交接

运行相关领域工作流与 `just check`，检查 diff 范围。领域改动不默认启动 UI、真实 ROM 或游戏；现有保存重开验证属于合成文件流程，不代表游戏内验证。

验证：所有命令退出 0，范围检查只出现允许文件。更新索引状态及实际执行证据。

## 完成标准

- [x] `PlanTableMove` 不再调用 `CaptureBytes()`，仍使用原输入 stamp 冻结提交。
- [x] 五版中，其他表布局改变使追加和显式复用两类旧计划失效，失败保持完整文档状态。
- [x] 相同解释恢复的语义保持；撤销不复活旧修订计划。
- [x] 原有五版搬迁、独立引用、未知字节、撤销重做及保存重开流程通过。
- [x] 相关领域测试与 `just check` 退出 0；`git diff --check` 通过；索引已更新。

## 停止条件

- 当前实现已经解释绑定，或 `UseLayout` 的修订／身份语义与摘录不同。
- 合法的独立手动布局不能构造，或规划在布局变化前就失败，无法证明生命周期缺口。
- 需要改变通用 Stamp 语义或放宽已确认的冲突检查。
- 其他计划修改了目标代码且存在不能明确解释的交叉影响；先核对新基线。
- 合理修正后同一验证连续失败，或出现无法解释的正常搬迁回归。

## 维护与评审

审查输入绑定是否覆盖“计划依赖过的全部解释设置”。不要把当前 UI 的弹层或默认追加当作领域 API 的安全边界。未来新增可复用空间来源时，应继续在规划时检查 Claims，并让冻结计划在解释变化后失效。

若需隔离分支，使用 `advisor/002-table-move-interpretation`。不自动提交或推送；被要求提交时沿用仓库短动词英文风格，例如 `bind table moves to their interpretation`。

## 执行记录

2026-11-08：已实现并完成源码复核；全部实现结束后统一运行的 `just check` 退出 0，lint、MyGo vet、翻译检查、默认测试及默认／无 CGO 构建全部通过。`git diff --check` 通过，最终修改范围已复核。新增五版共 20 个解释生命周期子场景；修改前追加／复用两类旧计划在五版均意外成功，修改后 `./game/records ./game/edit ./space ./game` 的 `-count=1` 测试通过。测试同时验证恢复相同解释、撤销不复活旧修订、失败完整状态及原子历史。未修改通用 Stamp 或历史实现，未运行真实 ROM、UI、race 或游戏验证。
