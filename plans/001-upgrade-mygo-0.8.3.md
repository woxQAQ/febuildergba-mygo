# 001：升级到 MyGo v0.8.3 并适配手势取消

> 提案，尚未执行。基线：`2585e9f984bc31e34ade88b0b92580923b35e097`，2026-11-07。P1，工作量 M，风险中，无前置任务。触发条件：开始本轮 MyGo 升级。目标上游提交：`5add3817c74bb7505d0b473bc5fb28abe2201b99`。
>
> 执行前阅读本文件及项目 `AGENTS.md`。先运行 `git diff --stat 2585e9f..HEAD -- go.mod go.sum internal/components/mapview internal/pages/portraits internal/pages/worldmaps`，再检查工作区 diff。相关实现发生变化时，重新核对下述假设；保留并行修改，不把其改动带入迁移。

## 目标与边界

将 `github.com/egoist/mygo` 从 `v0.7.4` 固定到 `v0.8.3`，保留现有布局、领域写入、历史归属和线程模型。地图画笔与头像部件拖动在系统取消时丢弃未完成预览；正常释放保留最后采样，一次手势仅提交一次。

不将本次升级扩展为焦点、路由、选择器、主题或领域包重构。`game`、`rom`、`space`、`gba` 继续不依赖 MyGo。不能修改只读旧版子模块，也不能批量迁移缓存中的旧 UI 探针。

## 当前实现与兼容事实

- `go.mod:8` 锁定 `github.com/egoist/mygo v0.7.4`，CLI 通过同一模块的 `tool` 指令锁定。上游两个 tag 的 Go 要求与模块依赖相同，均为 Go 1.27.3。
- `internal/components/mapview/drawing.go:30` 在没有按住时调用 `finishStroke`；后者没有根据 Clicked/DragEnded 的返回值判定是否真实释放：

  ```go
  if !pressed {
      v.finishStroke(canvas, drawing)
      return
  }
  // finishStroke:
  canvas.Clicked()
  canvas.DragEnded()
  points := slices.Clone(v.stroke)
  v.stroke = nil
  if drawing.Commit != nil {
      drawing.Commit(points)
  }
  ```

- `internal/pages/portraits/preview.go:305` 仅在 DragEnded 时清空 `page.drag`，并将定位作为一次 `State.Edit` 提交。
- `internal/components/mapview/input.go:25` 仅在 DragEnded 时清空 `v.panning`。
- `internal/pages/worldmaps/roads.go:304` 已检查真实释放：

  ```go
  clicked := canvas.Clicked()
  _, _, released := canvas.DragEnded()
  // 清理 stroke 后：
  if !clicked && !released {
      return true
  }
  ```

  道路路径是对照对象；不要把它改成“没有按住就提交”。

- 新版 `PressCanceled` 表示系统取得鼠标捕获或窗口失焦，Pressed/Dragged 变 false，Clicked 与 DragEnded 都不发生。框架提供 `Tester.CancelPress()` 与 `Tester.SetFocused(false)`。真实释放的最后位移会先以按住状态送达，再在下一 build 报告释放。
- 已对固定提交快照运行目标版 migrate-ui 预览、vet、默认测试、普通及无 CGO 构建、i18n check，均通过。没有任何正式页面交互或原生窗口验证；上述风险仍需复现。

错误和领域提交遵循已有作者模式，例如头像拖动的 `page.state.SetErr(page.state.Edit(func() error { ... }))`。不因升级绕开领域编辑器或直接写 ROM。

## 允许修改的范围

- `go.mod`、`go.sum`。
- `internal/components/mapview/drawing.go`、`input.go`；如取消重置确需共享状态，可修改同包 `view.go`。
- `internal/pages/portraits/preview.go`。
- `internal/pages/worldmaps/roads.go`：只允许必要的取消清理，不改变道路编码或提交规则。
- 如实际交互改变了使用说明，更新 `DESIGN.md` 或对应领域设计；没有改变用户契约则不另写实现总结。
- 临时复核代码可放在受影响包，复核后必须删除；不提交永久 UI／模拟器测试。
- 完成时更新 `plans/README.md` 中本任务状态。

领域生产代码、翻译、平台后端和其他机制改造不在范围内。若 `just check` 暴露与并行工作相关的问题，隔离验证并报告，不顺手修复。

## 执行步骤

### 1. 锁定版本并复核生产源码

在隔离的干净工作树或明确分离了并行修改的分支操作；不要回滚现有工作区。沿用历史中的动作式提交标题，例如 `upgrade mygo to 0.8.3 and handle canceled gestures`；不自动提交、推送或发布。

以下含环境变量的示例使用 Nushell：

```nu
with-env { GOTOOLCHAIN: local } {
    go get github.com/egoist/mygo@v0.8.3
    go mod tidy
    go tool mygo migrate-ui internal
    go tool mygo migrate-ui cmd
}
git diff -- go.mod go.sum
```

**预期**：模块版本为 v0.8.3；Go 指令和其他直接／间接依赖保持基线要求；生产迁移预览无输出。不要对仓库根目录运行 `migrate-ui -write .`。意外的源码改写或其他依赖变化需要先查明原因。

### 2. 先复现取消，再修正应用手势状态

用临时 `ui.NewTester` 驱动实际组件或页面，构建以下流程，不给私有函数逐个写单元测试：

| 操作           | 取消前提                                  | 预期                                                             |
| -------------- | ----------------------------------------- | ---------------------------------------------------------------- |
| 地图画笔／矩形 | Press、Move 后 CancelPress；另测窗口失焦  | 不调用 Drawing.Commit，草稿不变；下一次合法笔画只包含本次采样。  |
| 地图平移       | 开始平移后取消                            | 清空手势标记；下一次单击可以选择格子。已经完成的视口位移保留。   |
| 头像眼／口定位 | 部件内按下并移动后取消                    | 预览回到原位置，文档修订和历史不变；随后正常拖动只生成一次历史。 |
| 世界地图道路   | 开始笔画后取消                            | 保留现有不提交行为；新笔画正常提交，Escape 取消仍有效。          |
| 所有修改手势   | 右键或中键按住并移动                      | 不执行主键绘制或定位；右键菜单按现有语义工作。                   |
| 正常释放       | 最后一次 Move 与 Release 在连续输入中到达 | 包含最后坐标，一次提交；增加空帧不会重复提交。                   |

在消费“没有按住”的分支之前处理 PressCanceled：只清理当前未完成预览并返回。头像清理 `page.drag`，地图清理 stroke／panning；不要将取消解释为释放。正常笔画完成也应以真实 Clicked/DragEnded 为依据，同时保留主键未移动的单格操作。

地图的 Drawing.Commit 仅调用草稿操作；头像沿用原有领域事务；不得将预览临时值提前写入 ROM。审查被禁用、离开页面或来源切换时是否也清理未完成手势。

**验证**：临时流程使用统一前缀 `TestMyGo083Review`，执行：

```nu
with-env { GOTOOLCHAIN: local } {
    go test -count=1 -run TestMyGo083Review ./internal/components/mapview ./internal/pages/portraits ./internal/pages/worldmaps
}
```

预期所有场景通过，文档／草稿修订与提交次数符合表格。合成 ROM 使用 `internal/testutil.Open`、`ImageFixture`、`NewWorldMapRoadFixture` 等既有构造，不读取固定真实 ROM；参考 `tests/README.md` 的工作流测试约定。先确认未修复代码的取消场景确实失败，再确认修复通过。

### 3. 完成有界原生交互复核

在当前 macOS 环境启动开发构建，仅复核地图、头像、道路、Split、文本／Combobox 和原生编辑菜单。核对真实失焦、中文 IME、弹层 Escape、宽窄窗口及应用缩放。对照失活窗口中的内置 Tabs 与自绘章节 Tabs；只有观察到错误且在范围内时处理，否则记录限制并独立计划，不扩大主题修改。

普通文本撤销仍归文本，数值直写仍按既定策略选择 ROM 历史，地图草稿仍归自身，空／过期草稿不落回 ROM。原生菜单与快捷键应一致。Tester 不能证明平台菜单、IME 或鼠标捕获行为，不把无头复核写成原生验证。

**验证记录**：注明当次平台、输入、操作和结果；未有 Windows／Linux 环境时明确未执行，不构造“全平台通过”。不需要启动模拟器或游戏。

### 4. 删除临时复核并运行交付检查

```nu
just check
git diff --check
rg -n 'TestMyGo083Review|ui.NewTester' internal/components/mapview internal/pages/portraits internal/pages/worldmaps
```

预期 `just check` 和 diff 检查退出 0，最后的 rg 无新增临时代码匹配。`just check` 包含自动修复，因此先确认工作树隔离，审查全部 diff。没有修改 ROM 历史或领域并发则无需 test-race；如果超出此范围，先重新界定任务。

## 完成条件

- [ ] go.mod 固定 v0.8.3，CLI 同步，Go 与其他依赖无无关变化。
- [ ] 临时流程已证明取消不提交、状态不残留、真实释放恰好提交一次；记录范围后删除复核代码。
- [ ] 原生复核与未验证平台分别记录。
- [ ] `just check`、`git diff --check` 退出 0。
- [ ] 没有改动只读子模块、领域写入规则、真实 ROM 或并行开发内容。
- [ ] 更新计划状态，并按 AGENTS 处理已完成方案的归档与索引。

## 偏离时的处理

若手势链路已变化、生产 codemod 非空、目标要求新的工具链、或者必须修改领域事务才能通过，先重新核对本计划并记录差异，不按旧片段套补丁。若同一验证两次合理修复后仍失败，记录最小复现和实际输出，保留可审查的改动，不声称升级完成。

本次预检已经证明源码可编译，不代表交互边界可跳过。后续 MyGo 升级也应重新阅读输入语义，而不是重复使用本次通过记录。

## 固定来源

- [v0.8.0 发布说明](https://github.com/egoist/mygo/releases/tag/v0.8.0)。
- [v0.8.3 输入实现](https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/ui/input.go#L1719)。
- [迁移指南](https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/migration.md)。
- [Tester 能力与限制](https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/testing.md)。
