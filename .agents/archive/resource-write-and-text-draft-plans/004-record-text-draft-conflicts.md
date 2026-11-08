# 计划 004：记录页保留来源冲突的文本草稿

> 已完成并归档（2026-11-08）。正文保留当次实施计划与验证证据，不再作为待办或当前开发规则；其中 `plans/` 指归档前位置。当前维护入口为[架构](../../../docs/architecture.md)、[使用指南](../../../docs/user-guide.md)和[测试说明](../../../tests/README.md)。

> 本文件是独立交接计划。按步骤验证实际输入、跨入口修改、冲突与显式重读；不把私有辅助函数单测或“找到了控件”当作工作流证据。UI 复核只使用有界临时代码，结束后移除。完成后更新索引对应行及本文件执行记录。
>
> 首先执行：`git diff --stat 534dc8514e0799542e6f82f5345733a4c3e985ed..HEAD -- internal/pages/records/text.go internal/pages/records/item_text.go DESIGN.md docs/user-guide.md`，以及 `git status --short`。相关逻辑与当前状态摘录不匹配时停止并刷新计划。

## 状态

- 状态：DONE；2026-11-08 实施及所需验证通过，已归档。
- 优先级：P1。
- 工作量：M，包含有界临时 UI 复核。
- 修复风险：MED；须区分自动来源变化与用户已确认的引用更换，保留文本历史、焦点和控件身份。
- 类别：bug；审计发现 4，源码置信度 HIGH，尚无新增 UI 复现证据。
- 依赖：无。与其他三个计划可独立实施；不修改其领域保护逻辑。
- 编写基线：`534dc85`，2026-11-08。
- 触发条件：修复记录页名称／描述因其他入口改变原文而静默丢弃未应用输入的问题；维护者已选定。

## 问题与目标

人物、职业和道具的名称／描述先保留在页面草稿，预览后才写入 ROM。当前原文、文本 ID 或 `HistoryEpoch` 一变化，页面就把草稿重建成新原文。例如先修改道具描述但不应用，再从文本页修改同一文本，回到道具页会丢掉之前输入。

目标是：干净草稿可以自动跟随当前来源；脏草稿保留输入与原始来源，来源发生冲突时明确提示并禁止预览／应用，用户可复制后显式丢弃重读。用户在已有弃稿提示下确认更换道具文本引用，仍按原流程弃用该字段草稿；不能把这项显式操作误报为无法处理的外部冲突。

## 当前实现与约定

项目使用 Go 1.27.3、MyGo v0.8.3。页面状态仅在 UI 线程修改；不保留跨构建 `ui.Element`，控件保持稳定 Key／Handle。工具由 Nix 提供，直接 Go 命令用 `GOTOOLCHAIN=local`，不增加依赖。

术语：

- `fieldKey` 标识记录种类、记录 ID、字段 ID。
- 脏草稿表示 `draft.text != draft.original`，即有未应用输入；不能用“是否等于现在的 ROM 文本”替代这个判断。
- 来源在此指记录字段当前引用的文本 ID 及其可读原文；名称使用当前名称读取结果，描述使用当前 `text.Value.Content`。读取失败不是可用的新原文。
- `HistoryEpoch` 是窗口的历史刷新信号，撤销、重做和部分批量操作会递增；它不是“允许丢弃文本草稿”的授权。

`internal/pages/records/text.go:14` 和 `:32`：

```go
type textDraft struct {
    text, original string
    textID         uint16
    epoch          uint64
    preview        *text.Plan
    err            error
}

if draft == nil || draft.original != original || draft.textID != id ||
    draft.epoch != a.state.HistoryEpoch {
    draft = &textDraft{
        text: original, original: original, textID: id,
        epoch: a.state.HistoryEpoch,
    }
    a.texts[key] = draft
}
```

两个实际渲染入口是该文件中的 `descriptionEditor` 和 `nameEditor`。它们当前按 `draft.text != value.Content`／`draft.text != row.Name` 决定显示操作，不能可靠表达“脏草稿已与新来源相同，但原始基准已变”的冲突。`textPreview` 的应用按钮当前只检查 Busy。

可复用范例是 `internal/pages/texteditor/page.go:189`，它只在未修改草稿时自动替换原文：

```go
if d == nil || (d.text == d.original && d.original != value.Content) {
    d = &draft{text: value.Content, original: value.Content, revision: revision}
    p.drafts[id] = d
}
// 渲染时：
stale := d.original != value.Content
unavailable := p.state.Busy || !value.Editable || stale
```

沿用它的“保留／冲突／重读”行为，不复制其页面结构。复用已有 `locales.TextDraftStale()` 和 `locales.ReloadText()`，不新增翻译键；`TextDraftStale` 已说明草稿保留、可以复制并丢弃重读。

需要同时保留的显式契约在 `internal/pages/records/item_text.go`：选择不同文本时显示 `ItemTextDiscardDraft`，只有确认按钮成功才调用 `Records.Set`；取消或选回原 ID 不应丢弃草稿。当前 `commitItemTextChoice` 没有显式删除草稿，依赖下一次 `textDraftFor` 的自动重建，修复后必须把这一已确认行为放回正确提交位置。

项目规则（`AGENTS.md`）：

> “`ui.NewTester` 仅用于有界临时复现，完成后移除代码，不新增永久 UI 或模拟器测试。”

MyGo 维护实践见 `.agents/skills/mygo-maintenance/SKILL.md`；项目规则优先。领域已有测试覆盖文本计划的失效、原子写入和保存重开，本任务不另建 UI 草稿通用框架。

## 范围

允许最终保留的修改：

- `internal/pages/records/text.go`：草稿同步、冲突表达、名称／描述渲染及预览／应用守卫。
- `internal/pages/records/item_text.go`：只在成功确认不同引用后清理当前字段的草稿。
- `DESIGN.md`：补充现有名称／描述交互段落的冲突保留规则。
- `docs/user-guide.md`：在名称／描述流程补充冲突及显式重读操作，不另写实现总结。
- 本计划执行记录与 `plans/README.md` 的状态行。

允许临时创建并必须移除：`internal/pages/records/text_draft_review_test.go`，仅用于实际 UI 工作流，测试名 `TestRecordTextDraftReview`。

只读参考：`AGENTS.md`、`docs/development.md` 的原生界面复核段、`tests/README.md`、`internal/pages/records/form.go`、`internal/pages/records/page.go`、`internal/pages/texteditor/page.go`、`internal/editor/state.go`、`internal/components/textdraft.go`、`game/records/text.go`、`game/text/replacement.go`、`internal/testutil/text.go`、`internal/testutil/fixture.go`、`go.mod`。

不修改领域编解码、文本格式、记录字段保护、全局历史路由、Session 重置、`form.Reset/ClearDrafts` 的显式生命周期、共享组件、应用文件 I/O、翻译 JSON、生成 catalog、依赖或旧版子模块。新 UI 只复用既有样式／控件，不加新对话框、全局草稿注册器或自动合并机制。

## 命令

在仓库根目录，直接 Go 命令使用 Nushell 环境语法。

| 用途           | 命令                                                                                                                                                     | 成功预期                       |
| -------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------ |
| 当前领域基线   | `with-env { GOTOOLCHAIN: local } { go test ./game/records ./game/text -count=1 }`                                                                        | 退出 0                         |
| 定位框架源码   | `with-env { GOTOOLCHAIN: local } { go list -m -f '{{.Dir}}' github.com/egoist/mygo }`                                                                    | 返回锁定 v0.8.3 的模块目录     |
| 临时 UI 工作流 | `with-env { GOTOOLCHAIN: local, MYGO_ENV: production } { go test ./internal/pages/records -run '^TestRecordTextDraftReview$' -count=1 -v -timeout 60s }` | 修复后所有限定场景 PASS        |
| 生命周期检查   | `just vet`                                                                                                                                               | 退出 0，无存储构建值或线程诊断 |
| 常规检查       | `just check`                                                                                                                                             | 所有 recipe 成功               |
| 临时代码清理   | `git status --short`、`git ls-files --others --exclude-standard`                                                                                         | 没有临时 UI 文件或入口         |
| 文档与范围     | `git diff --check`、`git diff --name-only`                                                                                                               | 无空白错误，只有允许的最终文件 |

`just check` 包含自动修复，在隔离实现工作区中执行并检查完整 diff。本任务不引入后台回传或并发；领域 race 不证明 UI 行为，默认不运行模拟器、游戏或真实 ROM 集成。

## 实施步骤

### 1. 先通过实际界面复现数据丢失

完成漂移与当前领域基线检查，读取当前 MyGo 技能和锁定模块的 `ui/doc.go`、Tester／输入相关 API。临时 UI 文件可使用所属页面的同包测试，以构建真实 `Page.View` 或 `form.View` 并观察所属草稿；禁止直接调用 `textDraftFor` 写逐函数断言。

使用 `testutil.TextFixtureFor(testutil.TextVersions[0])` 的 FE8U 合成 ROM。为便于稳定选中一个有名称的道具，在打开前将道具 ID 1 的 `description_id`（`items + ItemStride + 2`）设为 4。该文本已有独立 Huffman 样本，`A`、`B`、`AB` 均可编码。通过 `testutil.Open` 打开临时文件。

建立一个有界 `TestRecordTextDraftReview`：让实际页面通过输入控件产生草稿 `AB`，不要预览／应用；经同一会话 `Texts.PlanID(4, "B")` 和 `Texts.Apply` 模拟另一个合法文本入口的提交，重新构建原记录页。断言草稿仍为 `AB`，文档保持刚提交的 `B`，且显示冲突。跨入口提交可以直接调用领域公开 API，但页面输入必须真实经过控件。

验证：当前领域基线退出 0；临时 UI 命令在修复前因草稿被重置而失败。若尚未定位目标控件、输入没有写入对应草稿或没有到达跨入口提交，先修正临时复现，不能将它记为缺陷已复现。

### 2. 明确草稿同步与冲突状态

在 `text.go` 中保持字段级草稿归属，按下表调整。可使用紧邻流程的私有方法，避免建立独立通用状态机包。

| 状态                                                    | 来源变化后的行为                                                                             |
| ------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| 无草稿／干净草稿，来源读取成功                          | 可以初始化／跟随当前文本 ID 和原文                                                           |
| 脏草稿，同一 ID、同一原文，只有修订或 HistoryEpoch 变化 | 保留用户文本及原始基准，清旧预览；更新已观察的刷新信号，避免每帧重复失效；允许显式生成新预览 |
| 脏草稿，ID 或原文不同                                   | 保留用户文本、原 ID、原文；清旧预览；派生冲突状态，禁止预览和应用                            |
| 已有脏草稿，当前来源读取失败                            | 不用空字符串或 ID 0 覆盖旧草稿；显示读取错误，保留可复制内容，禁止写入                       |
| 用户显式丢弃并重读                                      | 只移除当前 fieldKey 的草稿，下一构建从当前有效来源初始化，不写 ROM                           |

冲突不能通过把 `original` 更新为当前值来消除。若原文后来因撤销恢复到原始来源，可以重新进行显式预览；旧领域 Plan 不能复活。不要把全局 HistoryEpoch 的递增解释为清空文本草稿。

仅替换 `textDraftFor` 的条件不够：描述读取失败时，调用方目前仍传入零值 `Value`。调整这个私有同步接口或调用次序，让读取失败与有效空文本可区分。

验证：运行临时 UI 命令。核心跨入口场景应保留输入；在临时流程补入“无修改草稿自动跟随”和“无关字段历史变化保留输入”断言，均 PASS；领域基线仍退出 0。

### 3. 将冲突连接到名称、描述及实际操作

同时覆盖 `descriptionEditor`、`nameEditor` 和 `textPreview`：

1. 是否显示操作由脏状态／冲突决定，不能只与新原文比较。即使用户草稿恰好等于现在的 ROM 内容，也不能静默改写原始基准。
2. 冲突时显示 `TextDraftStale()`，保留输入和选择复制能力；当前有效来源不可编辑时用只读表达，Busy 才阻止交互。不要因 NameID 变为 0 或描述早返回而藏起旧脏草稿。
3. 使用已有 `ReloadText()` 提供显式弃稿重读；普通取消也完整重置当前字段来源，不能只更新 `draft.text` 而留下旧 ID／原文。
4. 预览／应用的禁用条件包含冲突、读取失败、不可编辑和 Busy。回调执行时重新核对当前记录字段的来源，防止沿用上一构建的可用状态；仍通过既有 `Records.PlanTextEdit` 和 `Texts.Apply`，不直接写字节。
5. 保持 `components.TextDraft` 的文本历史声明以及原有输入身份。状态提示插入后额外构建数帧，确保没有替换目标输入或错误消费文档撤销。

在 `item_text.go` 的 `commitItemTextChoice` 中，只有 `selected != original` 且 `Records.Set` 成功后，才显式删除 `a.texts[choice.key]`；取消、选回原值和失败均保留。既有弃稿提示与确认按钮语义不变；不清其他字段、其他记录或整张草稿 map。

验证：运行临时 UI 命令和 `just vet`，全部成功。临时流程必须涵盖下方测试清单，特别是取消引用选择和确认不同引用的不同结果。

### 4. 完成限定复核、清理和文档

按下方清单完成有界 UI 复核，记录每个场景的实际断言结果。更新现有 DESIGN／使用指南的对应段落：自动来源冲突保留草稿，用户可复制并显式重读，确认更换引用仍按已有提示弃用当前字段草稿。不把所有文本草稿笼统写成“任何操作都不丢弃”。

移除 `text_draft_review_test.go` 及所有临时入口，再运行当前领域基线与 `just check`。不提交永久 UI 测试；不为了验证私有辅助函数添加单测。

验证：临时 UI 命令在删除前已退出 0；删除后 Git 状态中无临时文件；常规检查退出 0；文档无新增失效链接且 `git diff --check` 通过。执行记录区分合成领域与临时 UI 证据，未进行原生应用／游戏验证则明确标注。

## 临时工作流清单

`TestRecordTextDraftReview` 使用真实控件、实际会话操作及重建，不依赖真实 ROM、网络、计时等待或页面内部函数单测。至少断言：

- 道具描述未应用输入，经另一文本入口改变原文后，内容可复制、显示冲突、旧预览清除，预览／应用不能修改文档。
- 名称执行同类跨入口流程；名称读取错误和空引用不会藏起已有脏输入。
- 干净草稿能跟随当前来源；脏草稿遇到无关文档修改或 HistoryEpoch 递增仍保留。
- 两条共享文本的记录各有草稿，成功应用其中一个后，另一条保留并进入冲突；其他未涉及字段不清空。
- 显式重读只清当前字段，读取新内容且不改变 ROM 修订和历史。
- 道具文本选择器：取消、选回原 ID、提交失败保留草稿；成功确认不同 ID 在已有提示下只丢弃该字段草稿。
- 在目标输入中执行文本撤销／重做，文档不新增写入；离开输入后的文档撤销仍由已有路由处理。不能仅凭 `TextInputActive()` 判定焦点正确，要检查目标草稿内容和文档修订。
- 冲突前后额外构建数帧；中文和英文各检查一次现有提示，窄窗口或放大界面时操作可达。

使用固定 ASCII 输入以适配样本字库；此任务不验证日文编码、全部版本或游戏画面。临时程序 60 秒预算若不足，先缩小复核到上述流程，不添加无界重试或睡眠。

## 完成标准

- [x] 脏名称／描述在其他入口改变来源、读取失败或历史刷新后保留，冲突明确且不能写入。
- [x] 干净草稿自动刷新；重读只影响当前字段、不修改 ROM；原有显式引用更换契约保留。
- [x] 临时实际 UI 流程全部通过，焦点／文本历史和额外构建已复核，实际证据已记录。
- [x] 临时 UI 文件和入口已移除；没有新增永久 UI 或逐函数单测。
- [x] 领域基线、`just vet`、`just check` 和 `git diff --check` 均通过。
- [x] 最终文件均在允许范围；文档与实际行为一致；索引已更新。

## 停止条件

- 当前记录页已有与摘录不同的草稿身份／冲突模型，或临时流程在基线已保留草稿。
- 必须修改领域文本模型、全局历史路由、翻译或 form 的显式重置契约才能解决；报告范围差异，不自行扩展。
- 无法用有效合成文本到达操作，或 UI 复核环境不可用；标记 BLOCKED 并记录已完成项，不能把静态检查当作交互通过。
- 解决数据丢失需要新增永久 UI 测试、自动合并草稿或额外弹层。
- 同一验证在合理修正后仍连续失败。

## 维护与评审

评审重点是三个独立动作：源内容更新、用户编辑草稿、用户明确弃稿。前两者不能自动授权第三者。后续批量文本导入、名称别名显示或引用选择器变化都应复核这三个边界；本任务不改变窗口关闭时对未应用草稿的既有处理。

若需隔离分支，可用 `advisor/004-record-text-draft-conflicts`。不自动提交或推送；被要求提交时沿用仓库短动词英文风格，例如 `preserve record text drafts on source conflicts`。

## 执行记录

2026-11-08：已实现并完成源码复核；全部实现结束后统一运行的 `just check` 退出 0，lint、MyGo vet、翻译检查、默认测试及默认／无 CGO 构建全部通过。`git diff --check` 通过，最终修改范围已复核。使用 FE8U 合成样本，通过真实 Page.View、输入控件和领域操作复现旧描述草稿被替换，修复后临时 `TestRecordTextDraftReview` 的 12 个限定场景通过。覆盖名称／描述冲突、来源恰好变成草稿、空引用／损坏读取、共享记录、干净刷新、无关历史、复制、文本撤销重做、引用取消／同值／过期／实际写入失败／成功弃稿，并检查额外构建。中英文提示及 880×700、150% 缩放下的实际重读点击通过。临时文件已移除，`go tool mygo vet .` 已通过，未新增永久 UI 或逐函数测试。未运行原生应用、真实 ROM、race 或游戏验证。
