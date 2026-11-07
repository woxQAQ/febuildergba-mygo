# 002：将记录字段的可见标签关联到实际输入

> 提案，尚未执行。基线：`2585e9f984bc31e34ade88b0b92580923b35e097`，2026-11-07。P2，工作量 S，风险低。先完成计划 001 的版本升级；本任务独立审查。触发条件：开始采用 MyGo 的字段关联机制。
>
> 先运行 `git diff --stat 2585e9f..HEAD -- internal/pages/records/fields.go internal/pages/records/item_fields.go` 并检查工作区 diff。以下片段不再匹配时重新核对设计，保留并行修改。

## 为什么做

通用记录字段和道具数值网格已经有可见标题与控件 Label，但通过标题上的 OnClick 手动请求 Handle 聚焦。MyGo v0.8.3 的 `LabelledBy` 同时表达命名与点击关系，可以在保持紧凑布局的同时删除这部分重复机制。

这不是没有无障碍名称的问题，也不是要求把密集字段全部换成 `ui.Form`。目标是建立真实标签关系，不改变数值解析、草稿、领域写入或撤销策略。

## 当前状态

`internal/pages/records/fields.go:294` 的 inlineField：

```go
caption := ui.Text(c, label).
    // 布局与帮助配置省略
    OnClick(func() { a.focusField(key) })
// 读取字段 value 后：
a.numberField(c, kind, row, f, value).Width(inputWidth).Shrink(0)
```

`internal/pages/records/item_fields.go:140` 的 itemNumberGrid 采用相同方式，控件外层还保留 `components.TextDraft`。该包装参与历史路由，本任务不删除。

`numberField` 返回当前 build 的 `ui.Element`，已经绑定 fieldDraft.input，用于菜单、导航等其他请求。删除标题回调并不意味着删除整个 Handle 或 a.focusField：同包其他位置仍调用它们。

目标版 LabelledBy 会用标签的可访问名称覆盖控件原有 Label；标签的 `.Label(...)` 优先于其可见文字。因此应给短 caption 设置完整的 `a.fieldLabel(kind, field)`，避免把“战斗属性／力量”缩成无法区分上下文的“力量”。不要仅保留控件的旧 Label 并假设它优先。

## 范围和约定

只修改：

- `internal/pages/records/fields.go` 的 inlineField 标签关联。
- `internal/pages/records/item_fields.go` 的 itemNumberGrid 标签关联。
- 对应 `DESIGN.md` 的通用标签交互说明，如现有说明不足。
- 临时 `internal/pages/records/mygo_label_review_test.go`，完成后删除。
- `plans/README.md` 中本任务状态。

不修改 numberField 的参数、解析、OnTextKey、提交或 TextDraft；不改变引用跳转按钮和 Tooltip；不全量替换表单、选择器或其他页面。作者风格沿用当前包，默认按包名导入，不为避免局部名称冲突增加别名。

## 步骤

### 1. 在当前 build 内建立关联

保留 caption 的布局、文字、Tooltip，设置它的完整上下文 Label；删除仅为聚焦输入而存在的 OnClick。构建数值框后在返回的 Element 上调用 `.LabelledBy(caption)`。

```go
caption := ui.Text(c, label).
    Label(a.fieldLabel(kind, f))
// 保留原尺寸、帮助等配置。
a.numberField(c, kind, row, f, value).
    LabelledBy(caption).
    Width(inputWidth).
    Shrink(0)
```

道具网格同样关联，保留外层 TextDraft。Element 只在本次 build 使用；不要把 caption 存入 fieldDraft、form 或回调。

若读取字段失败，只显示已有错误，不能把标签关联到另一个字段或引用按钮。

**验证**：`just vet` 退出 0；diff 中没有新的跨帧 Element，也没有历史或领域行为改动。

### 2. 用实际编辑流程做临时复核

临时代码在 records 包创建实际 Page／form 和合成 Session，沿用 `internal/testutil.Open`、`tests/README.md` 的实际领域操作约定，测试名称使用 `TestMyGoLabelReview` 前缀。

检查通用字段与道具字段：点击标题后在正确控件输入；上下文名称可通过 `Tester.AccessByRole(ui.RoleTextField, fullLabel)` 读取；Busy 时标题不聚焦可写控件。输入非法整数时保留文字与错误，切换记录后点击标题不能把输入送到旧记录；普通聚焦、Tab、数值微调和既有撤销行为保持原样。

```nu
with-env { GOTOOLCHAIN: local } {
    go test -count=1 -run TestMyGoLabelReview ./internal/pages/records
}
```

**预期**：流程通过，输入只改变选中的记录；可访问名称包含原有完整上下文。临时测试不以截图像素完全一致或 LabelledBy 的实现细节作为预期。

### 3. 删除临时复核并检查交付

```nu
just check
git diff --check
rg -n 'TestMyGoLabelReview' internal/pages/records
```

预期前两项退出 0，rg 无匹配。`just check` 会自动修复，须在隔离工作树执行并审查 diff。本任务不需要 ROM 集成、模拟器或 race；纯布局和标签关系不新增永久测试。

## 完成条件

- [ ] 两处字段流程通过 LabelledBy 关联可见 caption 和数值控件。
- [ ] 完整可访问名称、Busy、非法输入、记录切换和现有历史策略经有界复核保留。
- [ ] 没有删除仍用于其他入口的 Handle 或 focusField。
- [ ] `just check` 与 diff 检查通过，临时 UI 代码移除。
- [ ] 更新计划状态与必要设计说明，不追加一次性实现总结。

若 LabelledBy 在目标控件上转交到了错误的内部子控件，或需要改变数值历史才能通过，记录最小复现并重新评估，不能通过恢复手写回调掩盖问题，也不混入计划 003 的历史改造。

## 固定来源

- [Form 与 LabelledBy](https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/form.md)。
- [标签实现与名称优先级](https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/ui/form.go#L131)。
- [Element 生命周期与键](https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/views.md)。
