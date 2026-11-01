# MyGo v0.7.3 → v0.7.4 评估

> 一次性评估，核对日期：2026-11-01。项目基线为 `32de81fa70b97d7941b67273efa67dbe7f77eef3`，包含调研开始时已有的斗技场及其他工作区修改；验证使用当时的隔离快照，不涵盖快照建立后的并行修改。本报告不维护后续升级状态；当前依赖以 `go.mod` 为准，架构与开发规则见[架构说明](../architecture.md)、[开发说明](../development.md)和 [AGENTS](../../AGENTS.md)。

## 结论

**建议近期按常规维护升级到 v0.7.4，优先级中等，无需阻断当前领域开发。** 最明确的收益是标准编辑菜单按焦点和可用操作启用，以及 macOS 原生窗口空闲时减少唤醒。新增 API 中，`MessageOptions.DestructiveButtons` 最符合当前的放弃修改、覆盖文件流程；其他能力暂时没有足以推动改造的需求。

隔离快照仅调整依赖后，`just check` 全部通过，v0.7.3 和 v0.7.4 的 `just build` 也都成功。本机虽然是 macOS 27.0.1，但没有复现发布说明中的旧版 DMG 打包失败，不能把它当作当前必须升级的理由。没有测量本项目的空闲功耗，也没有完成菜单及布局的原生交互复核。

## 版本与证据范围

| 对象          | 核对结果                                                                                 |
| ------------- | ---------------------------------------------------------------------------------------- |
| 当前依赖      | `github.com/egoist/mygo v0.7.3`，上游提交 `6f28d7bb07b31177cf218e51586f6a5e99b38016`     |
| 目标版本      | `v0.7.4`，上游提交 `9bd34e1eb9525d9a1fe698aadc964c49c8ff8e7e`，官方正式发布于 2026-11-01 |
| Go 与依赖要求 | 两个 tag 的 `go.mod`、`go.sum` 完全相同；没有提高工具链要求或引入新的 Go 依赖            |
| 本机环境      | `go1.27.3 darwin/arm64`，macOS 27.0.1；所有 Go 命令使用 `GOTOOLCHAIN=local`              |
| 来源          | [官方发布说明][release]、[完整差异][compare]、固定提交源码、项目调用路径及隔离构建结果   |

`ui/doc.go` 在两版间没有变化。Element 构建期生命周期、Handle、Services 和 `Window.Update` 等既有约定继续适用，本次没有需要重新设计领域与 UI 边界的证据。[模块要求][module]、[UI 契约][contract]

## 现有流程能得到什么

| 变更                                                               | 本项目落点                                                                                                | 判断                                                                                                                                                                                                    |
| ------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 标准编辑菜单按焦点启用，#971                                       | `internal/app/menus.go` 的 Undo／Redo、Cut／Copy／Paste／Select All；各页的 TextInput、TextArea、Combobox | **最直接的交互收益。** 无选区时 Copy／Cut 不再一直可用，文本没有历史时 Undo／Redo 正确禁用，只读输入的 Paste 菜单也会禁用。框架保留应用显式禁用的状态。[菜单规则][menus]、[命令判定][command]           |
| macOS 空闲原生窗口减少唤醒，#928                                   | `internal/app/window.go` 使用 `Content: ui.View(a.View)`，适用该原生 surface 路径                         | **无需业务改造即可获得的框架优化。** 上游将重复运行的 Core Animation observer 改为绘帧后注册一次；发布说明给出约 135 → 13 次唤醒／秒、CPU 0.8% → 0.3%。这些是上游测量，不能当成本项目实测。[实现][idle] |
| Grid 子项百分比按自身单元格计算，#909                              | 人物／职业能力与武器矩阵、通用字段、死亡对话等 Grid 布局                                                  | 修复布局计算，也意味着已有布局可能改变。当前许多字段使用显式宽度；本次未复现具体溢出，不宣称已经修复某个页面。升级后应有界复核宽窄窗口、缩放和局部横向滚动。[实现][grid]                                |
| 部分滚出视口的 PopoverMenu 定位，#896                              | `internal/pages/records/profile_controls.go` 的武器等级菜单；`internal/pages/zoom.go` 的缩放菜单          | 武器区存在滚动，是较相关的检查入口；固定标题栏缩放菜单相关性较低。本次未复现菜单偏移。[实现][popover]                                                                                                   |
| macOS 27 DMG 转换兼容，#925                                        | `just build` 调用 MyGo 打包                                                                               | 新版在支持时改用 `diskutil image create from`，旧系统保留 `hdiutil convert`。**本机两版打包均成功**，这次只能计为兼容性修复，不能计为已解决的本项目阻塞。[实现][dmg]                                    |
| Windows 原生窗口首帧、缩放时背景、菜单长标签修复，#913／#895／#923 | 同一原生窗口和菜单实现                                                                                    | 对未来 Windows 验证有价值；本轮没有 Windows 运行证据。[发布说明][release]                                                                                                                               |

### 撤销路由需要保留

当前菜单已有明确分工：文本获得焦点时使用 `RoleUndo/RoleRedo`，否则由 `editor.State.History()` 选择地图草稿或 ROM 文档历史。v0.7.4 的菜单校验补足前者的启用状态，并不会理解我们的领域草稿、修订冲突或 ROM 历史。

上游 macOS 菜单实现仍按应用给出的状态启用自定义条目；Windows／Linux 对标准编辑角色查询当前原生内容。因而源码层面没有要求删除或改写现有路由。需继续保留 `TextFocused`、`DraftHistory`、Busy 和过期检查；也不能仅依赖 macOS 的标准角色到快捷键回退来统一三平台行为。[macOS 菜单实现][native-menu]、[跨平台菜单说明][menus]

采用升级时，最有价值的临时复核是：文本输入与只读原文之间切换，输入和撤销后检查菜单状态，再切到有／无历史的地图草稿以及 ROM 字段，检查菜单和快捷键是否仍指向同一目标。遵守项目约定，不新增永久 UI 测试。

## 新能力的采用建议

| 能力                                                | 具体用途                                                                    | 建议                                                                                                                                                                                                                                                                                                 |
| --------------------------------------------------- | --------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `MessageOptions.DestructiveButtons`                 | 关闭未保存文档时的“放弃”，重新读取时丢弃修改，另存与 PNG 导出时覆盖已有文件 | **优先考虑的小改进。** 在 `internal/app/window.go` 中标记按钮索引 1；`dialogs.go` 的重读／覆盖和 `assets.go` 的覆盖可按语义标记索引 0。保留明确的取消按钮。macOS 11+ 和 Linux 提供区别显示，macOS 还可能阻止 Enter 激活破坏性默认按钮，因此需要核对键盘行为；Windows 没有同等显示承诺。[API][dialog] |
| `ui.Themed(c, theme)`                               | 将来需要独立浅色背景的素材／文档预览，或主题预览                            | 当前 `components.ApplyTheme` 的窗口主题足够；待确有局部主题需求再用。它是为子树提供颜色和尺寸参数的容器，不改变 ROM 图片的像素与调色板。[文档][theme]                                                                                                                                                |
| `Element.Scrollbars(false)`                         | 已有其他明确导航手段的特殊画布                                              | 当前地图已经使用 `ScrollBoth` 和 `TrackScroll`，无需重写。隐藏滚动条会减少可发现性，不建议默认应用到地图、表格或文本。[文档][scroll]                                                                                                                                                                 |
| `ui.Scrollbar`／`Painter.Scrollbar`                 | 自己管理滚动偏移的画布或代码编辑器，用统一的主题绘制滚动条                  | 当前 mapview 已交给框架滚动容器，没有必要再实现一套。新类型提供几何与绘制能力，不接管我们自己的滚动状态。[API][scrollbar]                                                                                                                                                                            |
| `PlaceholderColor`、`CaretWidth`、`DottedUnderline` | 有实际需求的输入提示、光标和链接反馈                                        | 小范围可用，尚没有当前必须改变的交互。错误仍需就近文字说明，不能仅靠提示色表达。[发布说明][release]                                                                                                                                                                                                  |
| `Element.IgnoreInput`                               | 保留辅助技术可操作性、忽略鼠标和键盘的特殊显示状态                          | 当前没有明确落点；它仍可被屏幕阅读器激活，不能替代 Busy／过期操作的 `Disabled`。[发布说明][release]                                                                                                                                                                                                  |
| `ui.DecimalInput`                                   | 以字符串保持精确十进制金额等值                                              | ROM 字段目前主要是整数、地址、位标志及 ID，已有十进制／十六进制解析与草稿语义。无需替换现有字段输入。[发布说明][release]                                                                                                                                                                             |

径向渐变、Calendar Browse、自定义动画的 `Context.Instant`、Markdown anchor、通知取消、updater 版本查询、Arch PKGBUILD 和 Windows 跳过安装器等目前没有直接产品需求；不能仅因 API 可用就扩大本次变更。

### 不应计入当前主流程的收益

- **macOS 非前台窗口首次点击修复 #950 属于 WebView。** 上游修改的是 `MyGoWebView.acceptsFirstMouse`，原生 `MyGoSurfaceView` 已有该行为。本项目正式窗口是原生 UI，不能用这条解释现有窗口点击体验。[实现][webview]
- **代码编辑器／diff／终端的滚动和选择修复并未自动升级正式文本编辑工作流。** 项目只有 `cmd/ui-probe/main.go` 导入 `ui/editor`；正式文本与十六进制页面使用 `ui.TextArea`，事件脚本使用类型化表格。发布说明中的侧向拖选、diff 横向滚动条和终端字符修复主要不落在当前主流程。[发布说明][release]
- **WebView `ScrollToContainer` 当前没有落点。** 项目没有把网页嵌入原生滚动区。
- **对话框 Escape 修复的直接收益有限。** 它修复取消按钮同时是默认按钮的情况；项目现有关键确认框已显式将取消设为另一个按钮。不能把常见的“放弃／取消”流程说成此前无法用 Escape 取消。[API][dialog]

## 已完成的验证及限制

验证时把调研开始时的项目文件复制到隔离目录，排除旧版子模块、真实 ROM、缓存和构建产物，保留当时已有的业务修改。主工作区的 `go.mod`、`go.sum` 和产品代码未由本次评估修改。

| 检查                | 结果与范围                                                                                                                              |
| ------------------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| 固定 tag 与模块内容 | v0.7.3／v0.7.4 的工具链及 Go 依赖要求相同，模块来源与 Git tag 提交一致                                                                  |
| 隔离快照升级        | `go get github.com/egoist/mygo@v0.7.4` 仅改变项目 MyGo 版本与校验和                                                                     |
| `just check`        | 通过：golangci-lint 0 issues、MyGo vet、1443 条双语翻译检查、默认测试、默认构建及 `CGO_ENABLED=0` 构建                                  |
| `migrate-ui` 预览   | 斗技场在建代码及已有临时复核代码出现建议；v0.7.3 也给出这些建议，两 tag 的迁移器实现未变。它们不是本次新增 API 迁移要求；未应用自动迁移 |
| v0.7.3 `just build` | 通过：darwin/arm64 应用、本地签名及 DMG 均生成                                                                                          |
| v0.7.4 `just build` | 通过：darwin/arm64 应用、本地签名及 DMG 均生成；CLI 显示应用约 12.7 MB、DMG 约 4.0 MB，与旧版相同的显示精度                             |

快照包含工作区已有的临时斗技场 `ui.NewTester` 复核，随默认测试通过；它只说明当次斗技场草稿、组切换、预览与焦点等流程，不能证明标准编辑菜单、Grid 或平台窗口行为。本次没有新增 UI 测试，也没有启动原生应用、进行功耗基准、真实 ROM 集成、游戏内验证或 Windows／Linux 运行验证。没有修改文档历史或领域并发实现，未运行 `test-race`。

建议后续升级保持范围清晰：先更新版本与校验和，按项目约定完成检查及上述菜单／布局临时复核；`DestructiveButtons` 如采用，作为独立交互修改审查。现有领域包、历史系统和文本草稿模型无需为这次补丁版本调整。

## 来源

[release]: https://github.com/egoist/mygo/releases/tag/v0.7.4
[compare]: https://github.com/egoist/mygo/compare/6f28d7bb07b31177cf218e51586f6a5e99b38016...9bd34e1eb9525d9a1fe698aadc964c49c8ff8e7e
[module]: https://github.com/egoist/mygo/blob/9bd34e1eb9525d9a1fe698aadc964c49c8ff8e7e/go.mod
[contract]: https://github.com/egoist/mygo/blob/9bd34e1eb9525d9a1fe698aadc964c49c8ff8e7e/ui/doc.go
[menus]: https://github.com/egoist/mygo/blob/9bd34e1eb9525d9a1fe698aadc964c49c8ff8e7e/docs/menus.md
[command]: https://github.com/egoist/mygo/blob/9bd34e1eb9525d9a1fe698aadc964c49c8ff8e7e/ui/input.go#L1060
[native-menu]: https://github.com/egoist/mygo/blob/9bd34e1eb9525d9a1fe698aadc964c49c8ff8e7e/internal/darwin/menu.go
[idle]: https://github.com/egoist/mygo/commit/2b66322aa3355057c44317a1c510bb24946b8fa5
[grid]: https://github.com/egoist/mygo/blob/9bd34e1eb9525d9a1fe698aadc964c49c8ff8e7e/ui/grid.go
[popover]: https://github.com/egoist/mygo/blob/9bd34e1eb9525d9a1fe698aadc964c49c8ff8e7e/ui/popovermenu.go#L119
[dmg]: https://github.com/egoist/mygo/blob/9bd34e1eb9525d9a1fe698aadc964c49c8ff8e7e/cmd/mygo/dmg.go#L133
[dialog]: https://github.com/egoist/mygo/blob/9bd34e1eb9525d9a1fe698aadc964c49c8ff8e7e/dialog.go#L75
[theme]: https://github.com/egoist/mygo/blob/9bd34e1eb9525d9a1fe698aadc964c49c8ff8e7e/docs/ui/styling.md
[scroll]: https://github.com/egoist/mygo/blob/9bd34e1eb9525d9a1fe698aadc964c49c8ff8e7e/docs/ui/scroll.md
[scrollbar]: https://github.com/egoist/mygo/blob/9bd34e1eb9525d9a1fe698aadc964c49c8ff8e7e/ui/scrollbar.go
[webview]: https://github.com/egoist/mygo/blob/9bd34e1eb9525d9a1fe698aadc964c49c8ff8e7e/internal/darwin/webview.go#L324
