# MyGo v0.7.4 → v0.8.3 升级评估

> 活动提案，核对日期：2026-11-07。项目基线：`2585e9f984bc31e34ade88b0b92580923b35e097`，MyGo 当前锁定 `v0.7.4`。此文只评估版本更新；全部原生 UI 能力的项目适配见 [UI 能力评估](mygo-ui-assessment.md)，任务导航见[计划索引](README.md)。升级尚未实施，当前版本以 `go.mod` 为准。

## 建议

近期升级到 `v0.8.3`，将手势取消适配与版本更新一起审查。当前实际使用的拖动、文本、滚动、窗口和 GPU 资源管理可获得上游修复；工具链要求不变，固定快照预检通过。交互风险仍需计划 001 的临时及原生复核。

采用 SearchField、Segmented、GridView、DiffView 等既有控件不依赖本次升级；不能用这份发布差异评估代替原生 UI 能力评估。

## 版本与来源

| 项目             | 核对结果                                                                                      |
| ---------------- | --------------------------------------------------------------------------------------------- |
| 最新正式发布     | GitHub `releases/latest` 在核对当日指向 `v0.8.3`，不是仅根据 tag 排序推断。                   |
| 当前上游提交     | `v0.7.4`：`9bd34e1eb9525d9a1fe698aadc964c49c8ff8e7e`。                                        |
| 目标上游提交     | `v0.8.3`：`5add3817c74bb7505d0b473bc5fb28abe2201b99`，2026-11-07 发布。                       |
| 工具链           | 两个 tag 的 `go.mod`、`go.sum` 没有差异；均要求 Go `1.27.3`。本机为 `go1.27.3 darwin/arm64`。 |
| 实际项目依赖变化 | 临时模块配置仅将 MyGo 从 `v0.7.4` 改为 `v0.8.3`，其余 require 条目未变。主工作区依赖未修改。  |
| UI 契约          | 两个 tag 的 `ui/doc.go` 没有差异。Element、Handle、Services、窗口 UI 线程等既有边界继续成立。 |

核对了用户给出的[在线 UI 入口][ui-live]，并以目标 tag 的[文档索引][ui-index]、迁移指南和实际实现确认版本范围。[完整差异][compare]用于核对模块要求与 UI 契约。发布说明按 [0.8.0][release-080]、[0.8.1][release-081]、[0.8.3][release-083] 连续阅读。`0.8.3` 的源码还撤回了 `0.8.2` 引入的 Swift native modules 变更，计划直接锁定最终补丁 tag，不以中间版本作为交付目标。[目标提交][target]

## 本项目能获得的收益与行为变化

| 变更                                                                                       | 项目落点与判断                                                                                                                                                                                            |
| ------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 拖动的最后一次移动不再丢失；Pressed/Dragged 只跟随主键；系统取消不再产生 Clicked/DragEnded | 地图画笔、地图平移、头像部件定位、世界地图道路和 Split 都直接相关。最后采样修复有收益，但取消语义必须人工适配，见下节。[输入实现][input-source]                                                           |
| macOS 文本布局、Windows GPU 和跨平台字形图集优化                                           | 人物／职业表单、文本浏览和大量字段有直接使用路径。发布说明给出的约减半 CPU/GPU 时间是上游场景，不是本项目实测，不作为验收数字。[0.8.0 发布说明][release-080]                                              |
| 窗口关闭释放内容、macOS 动画时回收闲置纹理、裁剪 mask 复用和预算修复                       | 本项目使用多窗口、地图及头像 Bitmap，可获得框架资源管理修复。尚未在本项目复现对应泄漏。[0.8.0][release-080]、[0.8.1][release-081]、[0.8.3][release-083]                                                   |
| 滚轮反向、Tooltip 穿透滚动、首次聚焦及 Base 控件禁用项导航修复                             | 对多滚动区、Combobox、章节 `TabsBase` 有直接路径。升级后应复核弹层、键盘导航及中英文输入。[0.8.0][release-080]                                                                                            |
| 失活窗口的内置控件弱化 accent                                                              | `components.ApplyTheme` 的主题继续可用；`internal/pages/chapters/navigation.go:59` 的手绘选中边框直接取 Accent，需与内置控件对照检查。地图选区或错误色具有自身语义，不能全局机械变灰。[窗口文档][windows] |
| HandleInput/ObserveInput 收到修饰键、锁定键及 KeyUnknown                                   | 固定基线正式 UI 没有调用这两个低层 handler，因此本次没有已知批量改写点。将来处理所有按键时需显式放行未使用事件，不能吞掉 Alt。[输入文档][input]                                                           |
| Markdown mentions/references、TypeScript 生成名变化、WebView/音频能力                      | 正式原生工作流没有相应消费路径，不计入本次升级必要性。ROM 文档仍由 `rom.Document` 管理，上游自带 Document 的保存修复不能当作我们的 ROM 修复。[0.8.3][release-083]                                         |

### 必须先验证的手势边界

这三类操作有不同后果，不能用一个“拖动有风险”代替分析：

- `internal/components/mapview/drawing.go:30` 在 `Dragged()` 返回 false 时调用 `finishStroke`；后者消费 Clicked/DragEnded，却不检查其结果就执行 `Drawing.Commit`。目标版在系统取消时也会使 Dragged 变 false，因此这条路径会把尚未正常松开的笔画交给领域草稿。该风险由源码组合确认，尚未通过本项目临时 UI 复现。
- `internal/pages/portraits/preview.go:272` 只在 DragEnded 时清空 `page.drag`。取消将没有 DragEnded，可能留下部件定位预览和累计位移；不应把它描述成取消当下已写入 ROM。
- `internal/components/mapview/input.go:9` 只在 DragEnded 时清空 panning，取消后可能残留平移状态。`internal/pages/worldmaps/roads.go:204` 已要求 Clicked 或 DragEnded 才提交笔画，当前代码会丢弃无释放通知的道路笔画；它需要作为正确行为的对照及复核对象，不是同一种误提交缺陷。

目标版提供 `Element.PressCanceled()`，对应 `Tester.CancelPress()` 和窗口失焦。应用应丢弃未完成预览，真实释放仍须保留最后采样，并且一次完整操作只提交一次。取消不会撤回已经完成的普通视口平移。[实现][input-source]、[上游行为样例][pointer-tests]

## 已执行验证与证据界限

最终预检使用固定项目提交的源码快照、独立模块配置和目标版 CLI，不改写主工作区依赖、产品源码、翻译或缓存中的历史探针。工作区并行修改不属于这个快照。

| 检查                                         | 结果                                                                                                                       |
| -------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------- |
| `migrate-ui internal`、`migrate-ui cmd` 预览 | 无输出，无生产文件需要 codemod。对完整原工作区执行 `migrate-ui .` 会列出被忽略缓存中的旧探针，不能据此批量改写 `.cache/`。 |
| `go tool mygo vet .`                         | 通过，包含普通 Go vet 和 MyGo 生命周期检查。                                                                               |
| `go test ./...`                              | 通过，固定快照运行退出码 0；部分结果复用 Go 有效缓存。                                                                     |
| `go build ./...`                             | 通过，darwin/arm64。                                                                                                       |
| `CGO_ENABLED=0 go build ./...`               | 通过，darwin/arm64。                                                                                                       |
| `go tool mygo i18n check`                    | 通过，2077 条消息、en 与 zh-Hans；没有重新生成 catalog。                                                                   |

所有 Go 命令使用 `GOTOOLCHAIN=local`。没有运行完整 `just check`，没有执行自动修复 lint、安装包构建、原生 UI／IME 临时复核、性能测量、Windows／Linux 实机、真实 ROM 集成、race 或游戏内验证。默认测试不包含这些页面的交互，因此预检通过不能消除手势及撤销路由风险。本次没有修改领域行为，无需为评估运行 ROM 或游戏。

调研聚焦 MyGo 依赖迁移和正式 UI 的代表流程，不是全项目安全审计，也没有逐一审查所有新控件或并行开发内容。

## 实施入口

[计划 001](001-upgrade-mygo-0.8.3.md)定义升级、手势取消与验证边界；标签关联和焦点试点分别在计划索引中独立安排。本报告不会维护后续产品功能状态，完成后按 AGENTS 处理归档。

## 一手来源

以下源码及文档链接固定到本次目标提交；在线文档可能继续演进。

[ui-live]: https://mygo.egoist.dev/docs/ui
[release-080]: https://github.com/egoist/mygo/releases/tag/v0.8.0
[release-081]: https://github.com/egoist/mygo/releases/tag/v0.8.1
[release-083]: https://github.com/egoist/mygo/releases/tag/v0.8.3
[target]: https://github.com/egoist/mygo/commit/5add3817c74bb7505d0b473bc5fb28abe2201b99
[compare]: https://github.com/egoist/mygo/compare/9bd34e1eb9525d9a1fe698aadc964c49c8ff8e7e...5add3817c74bb7505d0b473bc5fb28abe2201b99
[ui-index]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/README.md
[input]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/input.md
[input-source]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/ui/input.go#L1719
[pointer-tests]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/ui/pointer_cancel_test.go
[focus-source]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/ui/focus_events.go
[views]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/views.md
[form]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/form.md
[select]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/select.md
[custom]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/custom-widgets.md
[list]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/list.md
[grid]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/grid.md
[split]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/split.md
[number]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/number-input.md
[navigation]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/navigation.md
[inspector]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/inspector.md
[windows]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/windows.md
