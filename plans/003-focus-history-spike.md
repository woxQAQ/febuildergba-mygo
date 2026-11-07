# 003：验证框架焦点查询与领域历史归属的组合

> 设计试点，尚未执行。基线：`2585e9f984bc31e34ade88b0b92580923b35e097`，2026-11-07。P2，工作量 M，风险高。依赖：计划 001。触发条件：决定减少逐控件的 TextDraft 登记。此计划先产出有运行证据的有限候选，不授权直接全项目改写历史路由。
>
> 先运行 `git diff --stat 2585e9f..HEAD -- internal/editor/state.go internal/components/textdraft.go internal/pages/workspace.go internal/pages/records internal/pages/portraits/inspector.go internal/pages/texteditor/page.go internal/app/menus.go`，再检查工作区 diff。基线或历史策略已变化时重新核对矩阵，保留并行修改。

## 问题与不变量

MyGo v0.8.3 提供 `c.FocusTakesText()`，能回答当前焦点是否接收文字，包括 TextInput、TextArea、Combobox 和自定义文本客户端。现有 `components.TextDraft` 则依赖调用方逐个登记：

```go
// internal/components/textdraft.go:10
state.TextFocused = state.TextFocused || input.FocusWithin()
```

`internal/pages/workspace.go:337` 在每个 build 开头重置 TextFocused 和 DraftHistory，末尾据此设置菜单和快捷键。`internal/editor/state.go:88` 的 History 在 TextFocused 时返回空命令；否则优先 DraftHistory，再取 ROM 文档历史。`internal/app/menus.go:23` 根据 Workspace.UsesTextHistory 选择原生 Undo/Redo role 或领域命令。

框架的“接收文本”和应用的“撤销归属”必须分开：

| 焦点／场景                                    | 目标历史                                                    |
| --------------------------------------------- | ----------------------------------------------------------- |
| 搜索、未提交正文、Combobox 查询、表格临时文字 | 原生文本历史，不能撤销 ROM。                                |
| 已约定为立即写入的普通记录数值、头像定位数值  | 保持既有领域策略；当前 OnTextKey 显式调用 State.Undo/Redo。 |
| 地图或道路草稿区域                            | 自身草稿历史。空或过期草稿也占有历史目标，不落回 ROM。      |
| 普通页面，无文本／草稿历史占用                | ROM 文档历史。                                              |
| 原生系统对话框或 Busy                         | 不向领域派发新的历史命令。                                  |

特别注意 `internal/pages/records/item_fields.go`：部分 numberField 外层又包了 TextDraft，而 numberField 的 OnTextKey 仍调用 State.Undo。不能单凭“都是数字框”假定这些组合已一致；先复现，明确现有代码和设计描述是否冲突。

本任务不能把 editor.State 或领域数据搬入 ui.Local，不能删掉 stamp、HistoryEpoch、Busy 或过期保护。UI 状态继续只在 UI 线程修改，后台通过 Window.Update 发布。

## 允许范围

在隔离工作树或可丢弃的试验分支，只允许修改：

- `internal/editor/state.go`：有限的历史焦点声明数据，不引入 MyGo 依赖。
- `internal/components/textdraft.go`：显式文本历史声明；必要时在同一文件提供数值输入的文档历史声明。
- `internal/pages/workspace.go`：集中解析默认焦点与显式声明，菜单和快捷键消费同一个结果。
- `internal/pages/records/fields.go`、`item_fields.go` 与 `internal/pages/portraits/inspector.go`：标记已有的历史策略，不改领域操作。
- `internal/pages/records/page.go`、`internal/pages/texteditor/page.go`：只在这两个普通输入试点移除自动登记。
- `internal/app/menus.go`：仅在组合接口必须调整时修改；不改原生平台菜单机制。
- 有界临时 `*_test.go`，完成后移除。
- 本计划中的试点结论、`plans/README.md` 状态；如最终采纳候选，再同步 `docs/architecture.md` 和 `DESIGN.md` 的当前语义。

其他页面的 TextDraft 保留。地图和道路的历史选择不重构；测试调用现有工作流。不建立通用焦点管理器、命令总线或异步任务框架。

## 步骤与验证门槛

### 1. 建立三种历史的运行基线

用实际 Workspace 和合成 Session，在临时 `TestMyGoHistoryReview` 流程中构造至少一次 ROM 编辑、一次独立地图／道路草稿修改和一次未提交文字编辑。使用现有 `internal/testutil.Open`、`NewMapFixture`、`NewWorldMapRoadFixture` 等样本；不依赖固定真实 ROM。

依次聚焦记录搜索、正文、普通数值、头像数值、道具自定义数值、草稿区域，测试快捷键和原生菜单角色路径。通过文档 Revision／值、草稿历史和输入文字分别判断哪个历史发生变化，不能只检查“Undo 按钮可用”。

临时流程命名使用 `TestMyGoHistoryReview` 前缀，验证命令：

```nu
with-env { GOTOOLCHAIN: local } {
    go test -count=1 -run TestMyGoHistoryReview ./internal/pages ./internal/pages/records ./internal/pages/portraits
}
```

**预期**：得到上述矩阵的实际结果与冲突清单。若道具数值出现已存在的无动作或错误历史，记录为明确基线差异，先界定目标语义；不要通过复制这一错误使“兼容”检查变绿，也不要把它混进升级任务。

### 2. 试验默认查询与显式声明的数据结构

建议候选是一个仅覆盖当前 build 的有限枚举，含义为“未声明、原生文本历史、文档历史”。它描述应用历史归属，不冒充实际焦点，也不拥有命令回调；新名字需在代码注释说明。DraftHistory 继续保存独立草稿命令及空历史占用语义。

- 在每个 build 开头清空声明。
- 普通文本不必声明，默认由 FocusTakesText 判断。
- 具有特殊 OnTextKey 策略的数值控件显式声明文档历史；只在该控件实际持有焦点时生效。
- TextDraft 保留为调用方明确要求原生文本历史的包装；外层明确策略可以覆盖内部默认策略，例如道具现有组合。不得按标题、字段 ID 名称或显示字符串猜测历史归属。
- 在页面及弹层构建后集中求值，随后让 State.History、Workspace.UsesTextHistory 和菜单更新读取同一个结果。不要在视图前半段重新取一次过期快照。
- 只对记录搜索和文本正文取消重复登记；其他页面保留，逐步迁移。

若枚举仍需不断添加例外，放弃该候选并记录原因，优先重新梳理控件的历史归属，而非继续增加布尔值。不得将 Context/Element 放入 editor.State。

**验证**：`just vet` 退出 0，重新运行步骤 1 的实际流程；每个场景选择与目标矩阵一致的历史，额外空帧不触发撤销。

### 3. 检查焦点变化、原生菜单和 IME

临时 Tester 流程覆盖：文本框消失并返回、Modal 打开与关闭、隐藏目标的待执行焦点、禁用／只读控件、列表滚动、空／过期草稿、窗口失焦再返回。焦点请求仍使用已有 Handle；本任务不要求顺便改成 FocusBind。

在 macOS 原生窗口对普通文本、直写数值和草稿分别操作菜单及快捷键，输入中文并使用组合输入中的 Enter/Escape。无头 Tester 的 `Command` 与真实菜单验证分别记录，不能互相替代。其他平台未执行则明确保留限制。

**预期**：菜单和快捷键指向相同历史；一次输入只调用一次；没有原生文本撤销穿透到 ROM，也没有空草稿落回 ROM。`ui.NewTester` 的正常返回不能单独证明原生菜单行为。

### 4. 形成可审阅结论并清理试验

若全部语义成立，保留有限候选 diff，说明移除了哪两个登记点、还保留哪些显式覆盖，以及后续逐页迁移的触发条件；同步当前架构和历史约定。若不能满足矩阵，删除候选产品改动，仅在本计划记录实际失败与下一步设计问题，不默认启动全面重构。

删除临时 UI 代码后，在隔离工作树执行：

```nu
just check
git diff --check
rg -n 'TestMyGoHistoryReview' internal
```

前两项应退出 0，最后的 rg 无匹配。只有 UI 和菜单路由变化时，`just test-race` 不覆盖这些包，不把 race 写成安全证明。若没有改变 ROM 历史实现，不扩展到该 recipe。翻译未变化时不重新生成 catalog。

## 试点完成条件

- [ ] 历史矩阵有实际操作证据；当前实现与设计冲突单独列出。
- [ ] 有通过矩阵的有限候选，或有可复现的否定结论；不能只有“应该可行”。
- [ ] 菜单、快捷键、普通文本、数值、独立草稿、空／过期状态均有明确归属。
- [ ] 临时 UI 代码已移除；保留候选时 `just check` 通过。
- [ ] 未开始其他页面的批量改造，更新计划状态及结论。

## 固定来源

- [FocusTakesText 的精确定义](https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/ui/focus_events.go#L73)。
- [焦点、Handle、FocusBind、快捷键](https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/input.md)。
- [OnTextKey 与文本历史](https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/text-input.md)。
