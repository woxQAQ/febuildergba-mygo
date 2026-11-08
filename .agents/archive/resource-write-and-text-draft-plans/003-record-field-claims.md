# 计划 003：记录字段提交前检查资源归属并保持批量原子性

> 已完成并归档（2026-11-08）。正文保留当次实施计划与验证证据，不再作为待办或当前开发规则；其中 `plans/` 指归档前位置。当前维护入口为[架构](../../../docs/architecture.md)、[使用指南](../../../docs/user-guide.md)和[测试说明](../../../tests/README.md)。

> 本文件是独立实施交接。先完成漂移与基线检查，再按步骤建立回归、修复和验证。遇到停止条件时报告事实，不放宽保护以使测试通过。完成后更新索引状态和本文件执行记录。
>
> 漂移检查：`git diff --stat 534dc8514e0799542e6f82f5345733a4c3e985ed..HEAD -- game/records/editor.go game/records/claims_workflow_test.go`，再执行 `git status --short`。相关源码与下方摘录不同则先刷新计划。

## 状态

- 状态：DONE；2026-11-08 实施及所需验证通过，已归档。
- 优先级：P1。
- 工作量：M，约一天，含跨领域合成工作流。
- 修复风险：MED；必须只检查本次字段区间，使用与会话相同的记录 Owner，避免误拒合法记录编辑。
- 类别：bug；审计发现 3，审计阶段源码置信度 HIGH；实际复现与验证见执行记录。
- 依赖：无。建议在 001、002 之后顺序执行，以便归因；无代码或测试前置依赖。
- 编写基线：`534dc85`，2026-11-08。
- 触发条件：修正单项／批量字段写入绕过已知资源声明的问题；维护者已选定。

## 问题与目标

`Records.Set` 是人物、职业、道具、章节数值以及许多引用／位标志操作的提交入口。它已检查文档修订、字段位置和数值范围，却没有使用已经注入的 `space.Protection.Range`。因此重叠的有效表或其他领域载荷可能被普通字段编辑改写；批量操作也缺少对应检查。

目标是让每个实际发生变化的字段在同一输入快照上检查全部已知 Claims；所有校验成功后才向 `rom.Document` 提交一次。冲突返回稳定布局错误并保留 `space.ErrClaim` 原因和失败记录上下文。只读解释、空操作、手动定位、合法字段写入、未知字节与历史继续保持现有行为。

## 当前实现、术语与范例

项目使用 Go 1.27.3 + MyGo v0.8.3；本任务仅涉及领域核心，不导入 UI。Nix 提供工具，直接 Go 命令用 `GOTOOLCHAIN=local`，不添加依赖。

- `game/records/editor.go:16`：Editor 已有 `protection space.Protection`。
- `game/records/editor.go:137`：`Set` 的单项／批量入口。
- `game/claims.go:15`：记录行的 Owner 精确为 `space.Owner{Resource: string(kind), ID: id}`；表根指针用 ID `-1`，不能用于字段写入。
- `game/claims.go:58`：`protectRange` 校验边界并调用 `space.CheckRangeWrite`。
- `game/session.go`：向 `records.New` 注入 `Pointer: s.protectPointer, Range: s.protectRange`。
- `space/claims.go:77`：允许自己的区间和自己作为 Host 的指针站点，其余重叠均冲突。
- `game/support_sources.go`、`game/terrain_sources.go`：支援／地形指针的 Host 使用上述记录行身份。

当前 `Set` 的关键路径，`game/records/editor.go:145` 起：

```go
if err := s.binding.Check(s.context); err != nil {
    return err
}
var changes []rom.Change
for _, id := range ids {
    ctx.RecordID = new(id)
    _, row, field, err := s.Resolve(kind, id, fieldID)
    // 字段解析、数值范围检查、字节编码……
    changes = append(changes,
        rom.Change{Offset: row.Address + field.Offset, Data: data},
    )
}
ctx.RecordID = nil
if err := s.context.Document().Apply(
    s.image.Stamp().Revision(),
    string(kind)+"."+fieldID,
    changes,
); err != nil {
    return err
}
```

此摘录中间的说明注释是省略标记，核对时阅读实际函数，不把它作为待复制代码。

术语：Owner 表示资源身份；Host 是指针站点所处的记录；Range 为左闭右开的字节区间。必须检查 `[row.Address + field.Offset, offset + field.Width)`，不能把整行其他无关资源的重叠当成本次字段冲突。

架构约束（`docs/architecture.md`）：

> “原地载荷写入由 `space.CheckRangeWrite` 检查整段区间，只允许自己的声明及自己容纳的指针站点，其余均冲突。”

错误约定参照 `game/claims.go` 中已有转换：

```go
return fault.New(code, "game.protect_pointer", fault.Context{
    Resource: owner.Resource,
    Range: &fault.Range{Offset: site, Length: 4, Size: len(data)},
}, err)
```

本任务用已有 `edit.ErrLayout` 作为主错误，保留原因链；外层 `game.set` 已提供 Table、RecordID、Field 和 Value。不要解析诊断字符串，不添加新的错误码和翻译。

测试参考 `game/records/items_workflow_test.go` 的 `TestItemPresetEditingWorkflow`：使用 `records_test`、五版样本、独立固定字节与完整副本比较；跨领域构造参考 `game/claims_test.go`。不导出生产私有配置，不逐函数写单测。

## 范围

允许修改：

- `game/records/editor.go`，只调整 `Set` 及紧密相关的私有辅助实现。
- `game/records/claims_workflow_test.go`，新建，包名 `records_test`。
- 本计划执行记录与索引对应行。

只读参考：`AGENTS.md`、`docs/architecture.md`、`docs/error-design.md`、`tests/README.md`、`game/session.go`、`game/claims.go`、`space/claims.go`、`game/records/schema.go`、`game/records/fields.go`、`game/records/layout_test.go`、`game/records/items_workflow_test.go`、`game/records/item_traits_workflow_test.go`、`game/support/bonuses.go`、`game/fe8u.go`、`internal/testutil/fixture.go`。

不修改通用 Claim 算法、ROM 提交／历史实现、保护来源、其他领域、版本地址、手动布局的读取规则、原始字节入口、UI、翻译、依赖或旧版子模块。不要为每个领域建立排除分支，也不要仅因为字段属于某类表就忽略其他 Owner。

## 命令

下列直接 Go 命令使用 Nushell 环境语法。

| 用途           | 命令                                                                                                                                   | 成功预期                 |
| -------------- | -------------------------------------------------------------------------------------------------------------------------------------- | ------------------------ |
| 现有记录流程   | `with-env { GOTOOLCHAIN: local } { go test ./game/records -count=1 }`                                                                  | 退出 0                   |
| 新归属工作流   | `with-env { GOTOOLCHAIN: local } { go test ./game/records -run '^TestRecordFieldClaimWorkflow$' -count=1 -v }`                         | 修复后全部场景 PASS      |
| 会话与下游流程 | `with-env { GOTOOLCHAIN: local } { go test ./game ./game/records ./game/support ./game/terrain ./game/arena ./game/chapter -count=1 }` | 退出 0                   |
| 常规检查       | `just check`                                                                                                                           | 所有 recipe 成功         |
| 修改范围       | `git diff --check`、`git diff --name-only`、`git ls-files --others --exclude-standard`                                                 | 无空白错误，只有允许文件 |

`just check` 会自动修复，应在隔离实现工作区中运行并复核所有 diff。本任务没有新增并发或改变 `rom.Document` 历史实现，不默认运行 race、UI、真实 ROM 集成或模拟器。

## 实施步骤

### 1. 确认 Owner 与当前基线

阅读以上参考，重点核对会话行 Owner 和 `Protection.Range` 的缺省行为。确认 Set 仍是先构造所有变化再一次 Apply，且不能依赖前一个写入改变后续解析。

验证：运行顶部 Git 命令和“现有记录流程”；后者必须退出 0。

### 2. 添加可读重叠资源的实际字段工作流

新增 `TestRecordFieldClaimWorkflow`，至少包含以下场景。每个场景单独打开样本，规划／操作前保存 `Document.Snapshot()` 和完整 `Document.State()`。失败后逐项比较两者；不要只检查 `Dirty`。

**跨领域冲突与批量失败原子性：**

- 使用 `testutil.NewMinimalFE8U()`；`itemBase := f.GetPosition("items")`，FE8U 道具 Stride 为 36。
- 将支援属性加成根站点 `0x28534` 写成指向 `itemBase + 36 + 20` 的指针。这是第二个道具的 `combat.uses` 位置，其原字节为 46，后面 8 字节处的首字节为 0，形成有一行及终止行的可读原生加成表。规则来源为 `game/support/bonuses.go` 的 8 字节记录，不需要修改生产配置。
- 先通过实际 `Supports.NewBonusDraft()` 确认加成表能读取；确认 Items 的两个记录也可读。这样测试使用的是另一领域明确拥有的载荷，不靠随机指针形状。
- 单项调用 `Records.Set(Items, []int{1}, "combat.might", 42)`，应得到主错误 `edit.ErrLayout`，且 `errors.Is(err, space.ErrClaim)` 为真。
- 独立场景调用 `Records.Set(Items, []int{0, 1}, "combat.might", 42)`。第 0 项安全、第 1 项冲突，整个批次不得提交第 0 项，字节、大小、修订、脏状态、历史目标全部不变。
- 检查 `fault.Frames(err)` 可找到失败 RecordID 1、字段 `combat.might` 和正确字节区间；不比较 `Error()` 文本。

**字段区间与正常历史：**

- 在相同重叠样本中，第二个道具的 `weapon_type` 字段位于 `+7`，与加成表的 `+20` 起始区间无交叠。将它写为 3 应成功，且只改一个字段；一次撤销／重做恢复完整字节和历史。
- 对冲突字段写入其当前值作为无修改操作，应不生成历史或修订；仍执行字段、ID、值范围等前置校验，不能跳过非法输入。

**记录之间的重叠与手动布局：**

- 用五版 `testutil.Games()` 构造样本，将 Items 手动布局设置到 Units 的现有区域，Stride 和 Count 使用合法值（Count 可为 1）。对道具 `weapon_type` 的真实改变应因另一记录 Owner 拒绝，整个状态不变。
- 在独立样本打开前，把两条道具的原始字节复制到 `0x400000`；打开后将 Items 手动定位到该独立区域，再做相同字段编辑。应成功、只改变目标字段，并有完整原子历史。复制输入在测试中通过 Fixture 完成，不在生产 Set 内重定位。
- 五版既有普通字段、道具特性、引用与保存重开由原记录流程继续覆盖。

验证：运行“新归属工作流”。修复前跨领域和记录重叠场景应因意外写入而失败；安全字段及正常手动布局场景应可执行。前置加成表读取失败不能充当成功拒绝的证据。

### 3. 在实际变化加入集合前检查区间

保持现有解析、值范围校验和小端编码。对比快照中的目标字段与编码结果；等值写入只跳过归属回调，仍保留现有变化汇总和 Document.Apply 的范围／重复区间校验，不因优化空写入改变重复 ID 的结果。对实际变化：

1. 构造与会话一致的行 Owner：`space.Owner{Resource: string(kind), ID: row.ID}`，Slot 保持零；不要使用表根的 `-1`。
2. 在 `s.image.Data` 上调用已注入的 `s.protection.Range`，传入完整字段区间。缺少 Range 回调时对实际写入拒绝，不能默认允许。
3. 失败时用 `fault.New(edit.ErrLayout, ...)` 保留底层原因与实际区间。保留外层当前 RecordID，避免提前清成 nil。
4. 全部字段校验成功后才一次调用原有 Document.Apply；不在循环内 Apply，不先修改原始数据再检查下一项，不重捕获 stamp。

这个入口已有原子提交和同一输入快照，直接补上注入检查即可；无需重写为另一套事务框架。保持空选择、等值、重复 ID 和越界错误的既有行为，不借本任务增加批量去重策略。

验证：运行“新归属工作流”和“现有记录流程”，均退出 0。必须同时看到冲突被拒绝和非重叠字段正常写入。

### 4. 完成下游与仓库验证

运行“会话与下游流程”和 `just check`。若旧样本因真实重叠触发新保护，先定位具体 Owner 和范围；不能直接关闭保护或修改无关共享样本。

验证：所有检查成功，修改范围检查合规。更新索引及实际执行证据。

## 完成标准

- [x] 每个实际变化字段都经过注入的 Range 检查，使用正确行 Owner 和完整字段区间。
- [x] 单项、批量、记录重叠三类失败均保留全部文档状态；安全前项不发生部分提交。
- [x] 主错误、原因链和失败记录上下文符合上述要求。
- [x] 正常字段、未重叠字段、等值操作和五版独立手动布局工作流通过。
- [x] 下游领域测试及 `just check` 退出 0；diff 范围合规；索引已更新。

## 停止条件

- 会话 Owner 结构已变更，或 Range 回调不能表达本次记录字段写入。
- `0x28534` 样本未形成可读支援加成表；修正前置构造，不能用其错误作为冲突依据。
- 正常记录被拒绝，且原因指向未理解的真实共享／宿主关系；报告 Owner、Host、区间和调用链，不能增加白名单绕过。
- 修复需要修改保护来源、共享样本、公开错误码或其他允许范围外文件。
- 同一验证在合理修正后仍连续失败。

## 维护与评审

新领域只需提供 Claims，不应再修改 Set。评审尤其检查记录 Owner 与支援／地形 Host 的身份一致性、字段宽度覆盖、失败记录上下文和整批提交时机。以后若统一 Owner 的构造位置，应作为独立设计调整，本任务不新增公共命名框架。

隔离分支可用 `advisor/003-record-field-claims`。不自动提交或推送；被要求提交时可沿用 `check resource claims before record field writes` 这类仓库现有短动词英文风格。

## 执行记录

2026-11-08：已实现并完成源码复核；全部实现结束后统一运行的 `just check` 退出 0，lint、MyGo vet、翻译检查、默认测试及默认／无 CGO 构建全部通过。`git diff --check` 通过，最终修改范围已复核。新增 `TestRecordFieldClaimWorkflow`；修改前单项、批量后项和五版记录重叠均意外写入，修改后 `./game/records ./game ./game/support ./game/terrain ./game/arena ./game/chapter` 的 `-count=1` 测试通过。断言包含主错误及原因链、失败记录与区间、完整文档状态、非重叠字段、等值／重复区间，以及五版独立手动布局和原子历史。未改通用 Claims、历史或并发，未运行真实 ROM、UI、race 或游戏验证。
