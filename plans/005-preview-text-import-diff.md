# 005：试用原生只读文本导入差异审核

> 提案，尚未执行。基线：`2585e9f984bc31e34ade88b0b92580923b35e097`，2026-11-07；MyGo v0.7.4。P2，工作量 M，风险中。无升级前置依赖。触发条件：改善文本文件导入的 Before/After 阅读与比较，不改变导入能力。
>
> 先核对 `git diff --stat 2585e9f..HEAD -- internal/pages/texteditor/page.go internal/pages/texteditor/files.go DESIGN.md` 和工作区 diff；代码发生变化则重新确认当前数据所有权。保留并行修改，不自动提交或推送。

## 目标与当前状态

`internal/pages/texteditor/files.go:127` 的 fileReview 已有不可变 Import/ImportPlan、条目列表、来源过期检查、共享用途和统一应用。所选条目的比较从 `files.go:247` 起构建两列：

```go
before := row.Before
ui.TextArea(c.Key("before"), &before).
    ReadOnly(true).Height(140).Label(locales.Before())
// 另一列：
after := row.After
ui.TextArea(c.Key("after"), &after).
    ReadOnly(true).Height(140).Label(locales.After())
```

这能阅读全文，但未显示实际文字变化，也没有同步滚动。MyGo 当前版本已有 `github.com/egoist/mygo/ui/editor.DiffView`，不需要更新依赖。目标是仅替换比较显示，继续从领域 FileEntry 的 Before/After 取值。

不可引入可编辑新文、逐 hunk Keep/Revert、部分应用或从 Buffer 重新规划导入。应用和取消仍由现有 filePlan/closeFileReview 控制；缓冲区不是 ROM、草稿或计划的新事实来源。

## 范围及代码组织

只修改：

- `internal/pages/texteditor/files.go`：比较状态及其紧密相关私有构造／渲染辅助函数放在同一流程文件。
- `internal/pages/texteditor/page.go`：给 Page 增加该私有状态的指针；Reset 沿用整体重置。
- `DESIGN.md`：记录只读比较、宽屏 split／窄屏 unified 和完整内容可读。
- 临时 `internal/pages/texteditor/mygo_diff_review_test.go`，复核后删除。
- `plans/README.md` 的任务状态。

不修改 `game/text`、ImportPlan、文件格式、文本编码、正文编辑器、其他审核页或全局历史路由。不增加依赖，也不为单一用途创建通用 diff 框架。Go 导入默认按包名：files.go 可直接导入 ui/editor；page.go 现有 internal/editor 保持原名，通过同包私有 comparison 类型引用即可避免同文件同名冲突。

## 步骤

### 1. 建立独立且持久的显示状态

在 files.go 定义私有比较状态，持有当前 Import 身份、选中条目 ID、Before/After 原始字符串和 `*editor.DiffState`。Page 仅持有此类型指针。

仅在 Import、ID 或实际字符串变化时创建两个 `editor.NewBuffer` 和 DiffState。状态必须跨 build 保留，以保留滚动和文本选择；不能每帧 NewBuffer，也不能让过期 UI 缓冲替代领域原值。关闭审核、替换文件或重置 Session 时丢弃比较状态；错误和过期提示仍按原路径显示。

初始约束：Editable=false、HunkActions=false、Wrap=true、ExpandUnchanged=true。完整原文和新文不得默认折叠消失。保留前后侧的可理解名称和 UI 文字；不通过 Buffer.WriteTo 导出或写 ROM。

**验证**：`just vet` 退出 0；同一条目增加空帧不会重置选择／滚动，换文件或换 ID 后显示对应内容。检查没有存储 Element/Context，没有新增后台修改 UI 状态。

### 2. 替换比较显示并处理窄窗口

将原两个 TextArea 比较区替换为 DiffView；保留整个 fileReview 的条目表、摘要、错误、共享用途及应用／取消操作。可用逻辑宽度至少 760 DIP 时使用 DiffSplit，否则 DiffUnified。比较区域保持有界高度和内部滚动，全文始终可到达，不让新增内容把操作挤出窗口。

配置采用当前依赖可用 API，遵循项目字号及主题，不硬编码一套浅色 diff 色。检查控制码、中文和多行的阅读效果；如果标准词级高亮对游戏控制码产生误导，再评估字符级选项，不把控制码解释成 Markdown 或另一门语言。

**验证**：临时 `TestMyGoImportDiffReview` 工作流使用真实 Page 和 `internal/testutil.Open` 的合成会话，构造导入，选择至少两条有不同变化的条目。检查插入、删除、相同内容、空文、中文、控制码和多行，缩放及宽窄布局下能阅读完整两侧。

```nu
with-env { GOTOOLCHAIN: local } {
    go test -count=1 -run TestMyGoImportDiffReview ./internal/pages/texteditor
}
```

预期输入和粘贴不会改变任一显示 Buffer 或 Import；浏览、复制、切换布局不改变文档修订。v0.7.4 的 Tester 功能按该版本使用，不将目标版新测试 API 变为升级前提。

### 3. 验证审核和来源边界

继续使用同一工作流：取消比较后保留原单条草稿；成功应用原有 ImportPlan 只清理受影响草稿；来源修订变化后比较可以继续只读展示，但不能应用旧计划；切换文件后迟到回调不能恢复旧比较。

对照原两个只读 TextArea 的菜单行为，验证 Copy、Select All、焦点离开和现有历史目标；不在本任务重定义窗口 Undo。对没有匹配项、无效条目和失败计划仍保留错误文字，不能显示一份缓冲区就当作审核成功。

**验证**：重新执行同一临时工作流，所有行为通过；领域写入仍只经现有 ApplyImport。检查原有 `game/text` 的导入流程测试无需更改预期。

### 4. 清理与交付

若短文本及控制码比较确有阅读收益并满足上述约束，保留候选 diff，更新 DESIGN；若标准控件无法满足完整只读对照，记录具体限制并恢复显示改动，不扩展成新编辑器。

删除临时复核后在隔离工作树运行：

```nu
just check
git diff --check
rg -n 'TestMyGoImportDiffReview' internal
```

前两项退出 0，rg 无匹配。只变更 UI 显示时不运行 ROM 集成、模拟器或 race；不可把原生菜单未复核写成通过。

## 完成条件与偏离处理

- [ ] DiffView 只读，完整比较可阅读，状态按来源及条目保持身份。
- [ ] 宽窄窗口、中文、控制码、空文和切换条目经有界复核。
- [ ] 浏览不写入，取消／应用／过期／迟到回调沿用原领域契约。
- [ ] 有可审查候选或具体的否定结论；没有留下永久 UI 测试。
- [ ] 保留候选时 `just check` 通过，更新必要设计说明和任务状态。

如果必须启用 Editable 或从 Buffer 重建 ImportPlan 才能使用 DiffView，停止该方向并记录限制；这是改变产品编辑契约，超出本计划。若缓存需要 Document/布局身份以外的来源，先核对现有 Import 的身份和失效语义，不自行延长旧计划有效期。

## 固定来源

- [v0.7.4 DiffState 与 DiffView](https://github.com/egoist/mygo/blob/9bd34e1eb9525d9a1fe698aadc964c49c8ff8e7e/ui/editor/diffview.go)。
- [DiffView 的显示、编辑与审核契约](https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/diff-viewer.md)。
- [Buffer 的历史和文本存储](https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/code-editor.md#buffers)。
