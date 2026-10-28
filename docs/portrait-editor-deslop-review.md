# 头像页 kill-ai-slop 审查

状态：用户已批准 A、B、C 全部应用，五处精简已完成。界面与截图已更新。

## 范围与方法

检查头像表及人物／职业身份区的头像引用。扫描七个相关源码文件：`portraits/{page,preview,inspector}.go`、`recordform/{profile,preview}.go`、共享 `asseteditor.go` 与 `chrome.go`；同时复核 FE8U 宽／窄窗口、FE6 页面、人物身份区及键盘焦点。

内置扫描器不收录 `.go`，因此将源码逐字复制为临时 `.go.mjs` 文件进行文本扫描，原始行号保持不变。修改前内置规则为 0 命中；补充 MyGo 候选定位规则得到 11 处线索，人工确认 5 处可精简、6 处有意保留。补充规则只定位候选代码，最终结论来自源码和实际画面。

A／C 属于组件与文案的局部减法项，B 对应技能第 32 类“组内、组间使用同一间距”。真实游戏数据及已明确选择的视觉保持现有设计。

## 已应用的修改

| 组 | 修改前 | 当前行为与实现 |
| --- | --- | --- |
| A：减少重复动作控件 | `inspector.go:237`：每个部件同时显示可选中的地址和“查看”按钮，四个有效部件出现四个相同按钮。 | [resourceAddress](../internal/pages/portraits/inspector.go#L204) 将地址作为无边框的可聚焦控件；点击或 Enter 查看原始字节，Cmd/Ctrl+C 与右键菜单复制完整地址。保留悬停、焦点和查看提示，空／损坏引用不生成跳转。 |
| B：按关系收紧间距 | `preview.go:145`：眼睛和口型分别继承外层 Scroll 的 16／20 区块间距。 | [expressionControls](../internal/pages/portraits/preview.go#L192) 将两行表情控制放入同一组，组内间距 6；调色板、使用处保持区块间距，定位框仍贴着主图。 |
| C1：表情控件名称 | `preview.go:176`：眼睛帧选择器显示“眼”。 | 可见名称改为“眼睛”，坐标区仍显示“眼 X／Y”。 |
| C2：表情控件名称 | `preview.go:189`：口型选择器显示“嘴”。 | 可见名称改为“口型”，坐标区仍显示“嘴 X／Y”；保留原图、真实帧号及辅助功能标签。 |
| C3：使用处显示目标名称 | `preview.go:123`：按钮显示“人物 01 头像 ID”，缺少目标名称。 | [useLabel](../internal/pages/portraits/preview.go#L132) 从实际记录读取名称，显示“人物 01 Eirika”等；无名称时保留种类与 ID。字段名放入提示，跳转行为保持不变。 |

合计：3 组、5 处精简。A 是一处循环修改，影响多个有效部件行。

上表旧位置为修改前行号，对应源码快照位于 `/tmp/febuilder-portrait-deslop.j7Ja2U/source/`。

## 有意保留

| 候选 | 当前位置 | 保留理由 |
| --- | --- | --- |
| 等宽字体，四处 | [inspector.go:225](../internal/pages/portraits/inspector.go#L225)、[inspector.go:266](../internal/pages/portraits/inspector.go#L266)、[profile.go:306](../internal/components/recordform/profile.go#L306)、[profile.go:337](../internal/components/recordform/profile.go#L337) | 都用于真实 ROM 地址、原始字节或指针位置，服务扫描和编辑。 |
| 橙色定位框 | [preview.go:322](../internal/pages/portraits/preview.go#L322) | 是眼口实际叠加范围与拖动目标。 |
| 数字口型选项 | [preview.go:213](../internal/pages/portraits/preview.go#L213) | 对应实际帧索引：FE6 五帧，其余七帧。 |

现有强调色 `#C44B2C`、系统字体、真实像素图、16 色调色板、三种用途不同的图像预览、原始数值与 PNG 操作均保持现有设计。人物身份区的“小头像来源 0／沿用头像”保留原值及含义。

## 扫描结果

- 内置规则：0 → 0。
- MyGo 候选：11 → 7。
- 人工确认的待修项：5 → 0。
- 剩余七处中，六处是上述有意保留项；一处是只按 `effectView` 函数名定位的间距候选。该函数仍存在，扫描器无法理解新的子分组，已根据 `expressionControls` 的 6 像素组内间距及宽／窄窗口确认修复。

扫描、规则、分类及修改前图片备份在 `/tmp/febuilder-portrait-deslop.j7Ja2U/`。`core.json`、`scan.json` 是修改前结果；`after-core.json`、`after.json` 是修改后结果；`triage.json` 记录分类和应用状态。

## 当前截图与验证

- [FE8U 宽窗口](images/portrait-editor.png)
- [窄窗口](images/portrait-editor-narrow.png)
- [FE6](images/portrait-editor-fe6.png)
- [整张素材](images/portrait-editor-sheet.png)

临时 MyGo 探针复核五版 Tab 到达地址、Enter／点击导航、快捷键与右键复制、空／损坏引用、命名与无名称的使用处，以及宽窄窗口和中英文。继续复核表情帧、坐标及非法草稿、原子拖动、撤销重做、PNG 预览／应用和小头像来源，内存文档最终恢复原始字节。

项目检查 `just check` 与 `CGO_ENABLED=0 go test ./...` 已通过，包含 lint、Go/MyGo vet、710 条中英文消息检查、全部默认测试及两种构建。

本轮未改领域编码与 ROM 文件，不新增永久 UI 测试，不执行游戏内验证。
