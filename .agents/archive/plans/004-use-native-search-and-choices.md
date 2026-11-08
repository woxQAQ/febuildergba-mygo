# 004：在现有版本采用搜索与取值控件

> 提案，尚未执行。基线：`2585e9f984bc31e34ade88b0b92580923b35e097`，2026-11-07；当前 MyGo v0.7.4。P2，工作量 S–M，风险低至中。无升级前置依赖。触发条件：采用框架现成搜索和取值 UI。搜索与工具取值分为两个可独立审查的提交，不与焦点历史重构混合。
>
> 先检查 `git diff --stat 2585e9f..HEAD -- internal/pages/records/page.go internal/pages/texteditor/page.go internal/pages/portraits/page.go internal/components/mapview/view.go internal/pages/chapters/map_editor.go` 和工作区 diff。发现并行改动时核对当前代码再执行，不回滚他人的修改。

## 目标和现有事实

记录、文本、头像导航目前使用普通 TextInput 接收查询，例如：

```go
// internal/pages/records/page.go:201
if components.TextDraft(a.state,
    ui.TextInput(c.Key("search"), &pane.query).
        Placeholder(locales.Search()).Label(locales.Search())).Changed() {
    pane.selected = 0
    pane.chosen.Clear()
    pane.initialized = false
    pane.list = ui.ListState{}
}
```

`internal/pages/texteditor/page.go:146` 使用 OnChange(p.refresh)，头像页 `internal/pages/portraits/page.go:154` 同样绑定页面 query。SearchField 可保留这些绑定和过滤，同时提供清空按钮及有查询时的 Escape 清空。`v0.7.4/ui/combobox.go:420` 已有该实现，清空会发出 Changed 并把焦点留在内部输入。

另一方面，`internal/components/mapview/view.go:139` 用 Tabs 选择 50%／100%／200%，`internal/pages/chapters/map_editor.go:167` 用 Tabs 选择画笔／矩形／吸管／平移。它们属于取值，不对应独立页面；同版已有 Segmented 和 RadioGroup。

页面和控件边界遵守项目约定：页面绑定持久状态，Framework 接管键盘／焦点，领域继续负责写入。地图实际尺寸、缩放倍率、工具编号和历史不变。现有 `components.TextDraft` 参与历史路由，本任务保留。

## 范围

只修改：

- `internal/pages/records/page.go`、`internal/pages/texteditor/page.go`、`internal/pages/portraits/page.go` 的导航搜索。
- `internal/components/mapview/view.go` 的 zoom 取值控件。
- `internal/pages/chapters/map_editor.go` 的画笔取值控件。
- `DESIGN.md` 中对应的搜索与取值交互说明。
- 上述包的临时复核代码，完成后删除；`plans/README.md` 状态。

不修改领域包、查询匹配规则、选择器、顶层 Tabs、章节 TabsBase、其他页面标签、原生菜单或 TextDraft 的实现。不修改在建日志页面或翻译；SearchField 的框架清空文案使用已有内置翻译。

## 步骤

### 1. 替换三处查询输入

将三处构造器改为 SearchField，保留持久 query、现有 Key、Label、Placeholder、Busy 以及 Changed/OnChange 的过滤和选择重置逻辑。保持 TextDraft 包装；不要把 query 改成每帧派生的局部值，也不要在视图中启动后台搜索。

SearchField 是组合控件，内部输入拥有文本历史；若需要 Handle，请确认绑定能指向其可聚焦部分。不要额外添加一次清空回调，造成一次操作重复刷新或重置选择。

**验证**：在临时 `TestMyGoNativeControlsReview` 中使用实际记录／文本／头像页面和合成 Session：输入查询只过滤显示，点击清空或非空时 Escape 清空，选择和滚动按现有过滤规则重置；空查询 Escape 不触发文档关闭。非法或未提交正文仍保留；搜索撤销不修改 ROM；Busy 时不能输入。使用 `internal/testutil.Open` 等已有样本，按工作流检查结果，不给构造器逐个写单元测试。

```nu
with-env { GOTOOLCHAIN: local } {
    go test -count=1 -run TestMyGoNativeControlsReview ./internal/pages/records ./internal/pages/texteditor ./internal/pages/portraits
}
```

预期全部通过，ROM 修订不因搜索变化；一次清空触发一次有效过滤。保持当前版本，不为验证引入 v0.8 API。

### 2. 将地图取值改为 Segmented

两处保留原来的 `*int` 绑定和索引顺序，仅改为 Segmented。缩放前后的 ScrollState 比例调整保留，画笔／矩形／吸管／平移仍映射现有 mapview.Tool，不改变领域提交。保留 Label、Tooltip 和 Disabled。

保持每个取值组在 Tab 顺序中只有一个停靠，方向键选择后实际状态和绘制行为一致。当前宽度不足时，周围 Row 的换行仍可用；不要引入整个页面的横向溢出。真正的页面导航继续使用 Tabs/TabsBase。

**验证**：扩展临时流程，通过键盘和鼠标选中缩放及工具，检查缩放后的视口偏移、画笔执行的领域操作、禁用状态和多帧无重复提交。至少查看最小窗口及应用放大时的布局。

```nu
with-env { GOTOOLCHAIN: local } {
    go test -count=1 -run TestMyGoNativeControlsReview ./internal/components/mapview ./internal/pages/chapters
}
just vet
```

预期流程及 vet 退出 0。v0.7.4 没有 v0.8.3 的 Tester.AccessByRole，不能让验证 API 成为升级前提；角色选型可由控件契约及 Inspector 核对，若在已升级分支执行则可补充 Access 检查。

### 3. 清理并交付

更新 DESIGN 中搜索清空与工具取值的用户行为，删除所有 TestMyGoNativeControlsReview 临时代码，在隔离工作树执行：

```nu
just check
git diff --check
rg -n 'TestMyGoNativeControlsReview' internal
```

前两项退出 0，rg 无匹配；审查 `just check` 的自动修复，不能把并行源码或翻译带入提交。此任务不触及 ROM 历史／领域并发，不需要 race 或模拟器。

## 完成条件和偏离处理

- [ ] 三处搜索使用 SearchField，清空、Escape、过滤与文本历史经有界复核。
- [ ] 地图缩放和画笔用取值控件，索引、滚动、禁用和实际工具行为保持正确。
- [ ] 真正的页面 Tabs、领域写入、并行日志工作保持独立。
- [ ] `just check` 通过，临时 UI 代码移除，设计与计划状态更新。

若改变组合控件后原有 TextDraft 无法识别内部文本焦点，先保留最小复现并核对 FocusWithin／Handle 绑定，不删除历史保护、不顺手执行计划 003。若取值 UI 需要改动工具编号或领域算法才能成立，重新划定范围，而非扩大此计划。

## 固定来源

- [v0.7.4 SearchField 实现](https://github.com/egoist/mygo/blob/9bd34e1eb9525d9a1fe698aadc964c49c8ff8e7e/ui/combobox.go#L420)。
- [v0.7.4 原生控件 API](https://github.com/egoist/mygo/blob/9bd34e1eb9525d9a1fe698aadc964c49c8ff8e7e/ui/api_gen.go)。
- [SearchField 文档](https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/search-field.md)。
- [Segmented 与页面 Tabs 的区别](https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/segmented.md)。
