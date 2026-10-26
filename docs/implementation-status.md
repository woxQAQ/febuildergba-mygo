# 实现与验证记录

设计基线已进入实施。当前交付包含阶段 0 的旧版来源索引、原生能力原型、阶段 1 的 FE8U 基础编辑器和高级编辑入口，以及阶段 2 的 FE8J／FE6 日文原版基础适配。新适配已完成文件与原生 UI 验收，章节内效果单列待验收；可变长写入与 FE7 仍未实现。此前 FE8U 原版与两个改版的开场章节已验证人物 HP／力量、职业 HP、武器威力及实际战斗结果，人物表等级未改变开场部署等级。完整语义盘点、其余游戏场景及部分桌面交互仍未完成。

范围见 [重建计划](rebuild-plan.md)，逐项状态见 [覆盖矩阵](coverage.md)，文件及 UI 结果见 [ROM 验收记录](rom-acceptance.md)，游戏运行结果见 [章节验收](rom-chapter-acceptance.md)，对话框及文本焦点结果见 [桌面验收](desktop-acceptance.md)，新版本的文件及 UI 结果见 [版本适配验收](version-adapters-acceptance.md)，使用方法见 [编辑器 README](../febuildergba-go/README.md)。

## 当前实现

| 边界 | 代码 | 实际职责 |
| --- | --- | --- |
| ROM 文档 | [document.go](../febuildergba-go/internal/rom/document.go) | 拥有字节、修订、撤销重做与保存点；原子批次、过期操作拒绝、备份和外部修改检测；通过快照保存，磁盘 I/O 不持有状态锁 |
| 游戏会话 | [session.go](../febuildergba-go/internal/game/session.go) | 游戏适配、表与字段定义、单记录及批量字段操作、只读字节副本 |
| 高级操作 | [advanced.go](../febuildergba-go/internal/game/advanced.go) | 当前窗口的显式表布局；原始字节区域绑定文档和读取修订，写入时检查归属、长度及修订 |
| 游戏规则 | [format.go](../febuildergba-go/internal/game/format.go)、[fe8u.go](../febuildergba-go/internal/game/fe8u.go)、[fe8j.go](../febuildergba-go/internal/game/fe8j.go)、[fe6.go](../febuildergba-go/internal/game/fe6.go) | 版本标头、表及文本位置、字段差异与素材布局；共用解析和编辑流程，FE8U 已知补丁特征保持 |
| 名称读取 | [text.go](../febuildergba-go/internal/game/text.go)、[narrow_font.go](../febuildergba-go/internal/game/narrow_font.go) | 有界 Huffman／anti-Huffman、日文 Shift-JIS 及紧凑职业名称；按实际字形验证 FE8U NarrowFont 映射，未识别字节保留标记 |
| 图像读取 | [graphics.go](../febuildergba-go/internal/gba/graphics.go)、[portrait.go](../febuildergba-go/internal/game/portrait.go)、[assets.go](../febuildergba-go/internal/game/assets.go) | 有界 LZ77、4bpp 图块及调色板；头像拼接、职业卡、道具图标和已知头像扩展 |
| 素材预览 | [preview.go](../febuildergba-go/internal/components/recordform/preview.go) | 根据字段的素材引用显示真实图像，按文档修订缓存位图，提供原始数据入口 |
| 页面与组件 | [workspace.go](../febuildergba-go/internal/pages/workspace.go)、[records](../febuildergba-go/internal/pages/records)、[recordform](../febuildergba-go/internal/components/recordform) | 标签导航、虚拟列表、搜索、多选、能力矩阵、同组等宽输入、字段草稿与底部微调、关联跳转 |
| 高级工作区 | [batch.go](../febuildergba-go/internal/pages/records/batch.go)、[table_layout.go](../febuildergba-go/internal/pages/records/table_layout.go)、[hexeditor](../febuildergba-go/internal/pages/hexeditor) | 批量预览与应用、手动表定位、任意地址的固定长度字节编辑 |
| 共享编辑状态 | [state.go](../febuildergba-go/internal/editor/state.go) | 每个窗口独立的游戏会话、忙碌及错误状态、文档撤销重做与草稿修订 |
| 系统边界 | [app](../febuildergba-go/internal/app) | 独立窗口、异步原生对话框、关闭确认、聚焦检测外部修改、菜单及模拟器启动 |
| 显示语言 | [locales](../febuildergba-go/internal/locales) | MyGo JSON catalogs 与生成的 Go API；系统语言匹配、英文回退、复数、全应用语言切换与偏好持久化 |
| 原生能力原型 | [ui-probe](../febuildergba-go/cmd/ui-probe/main.go) | 像素画布缩放与拾取、输入法输入框、10,000 行代码编辑器 |
| 旧版来源索引 | [legacy-inventory](../febuildergba-go/cmd/legacy-inventory/main.go) | 由旧版项目和配置生成 CSV，记录标题、来源及静态可发现的调用者 |

没有增加第三方依赖；领域核心不导入 MyGo。当前写入只替换已有字节，不扩容或搬迁资源，未编辑字节保持原样。

## 编辑行为

- 职业能力并列展示基础、成长率、上限和转职加成；人物显示个人加值与成长率，不把个人加值解释成最终能力。列与单元格来自实际字段定义，缺失组合显示 `—`。宽窗口的武器熟练度置于矩阵旁，窄窗口换行。
- 其他属性采用同行标签和数值。数值右对齐并使用等宽数字；输入高度 26、能力行间距 2。加减按钮集中在底部当前字段栏，原始地址和字节编辑入口按需显示；Tab、上下键和文档撤销仍可用。
- 普通字段的有效数值立即进入内存 ROM；空值、非法值和输入法组合文字保留为草稿。
- 多选记录后显示批量字段与修改前后预览，显式应用形成一个撤销操作；改变搜索清空多选，避免隐藏记录进入批次。
- 手动表定位可指定地址、记录大小和记录数，沿用当前游戏的字段定义，检查对齐、字段覆盖范围及完整表范围。它只影响当前窗口，不修改指针或保存 ROM 元数据。
- 十六进制编辑每次读取 1–4096 字节，接受 ROM 偏移及 GBA 地址；显示读取时字节和待写入草稿。文档修订变化后拒绝旧草稿，保留内容供复制并重新读取。
- 字段、批量及原始字节修改共用历史；原始写入后重新解析表和名称。明确的游戏标头不匹配会停止结构化编辑，原始修改仍可撤销。
- 保存使用一致快照；保存期间的新修改不会被标记为已经保存。备份、临时替换和保存前外部内容检查保持有效。

系统字体及明暗主题用于密集工作区，原始字节采用等宽字体。常用操作保持在单个窗口内；地址与布局按需展开。数值输入显式接入文档快捷键，未应用的普通文本草稿保留文本自身的撤销。

## 验证方式与已执行结果

按用户要求，已删除 Go 项目的全部 9 个 `_test.go` 文件，以及 `internal/testrom`、`internal/romtest` 两个测试辅助包。没有另设测试替代入口。当前用真实 ROM 副本、实际操作、构建和静态分析推进验收；下文保留已执行的检查结果。

移除后已通过：

```sh
go build ./...
CGO_ENABLED=0 go build ./...
go vet ./...
go tool mygo vet .
go tool mygo build
```

此前构建曾更新 `febuildergba-go/build/darwin-arm64/FEBuilderGBA.app`（约 9.6 MB）和 `FEBuilderGBA 0.1.0 arm64.dmg`（约 3.1 MB），完成本地签名，尚未发行公证。当时根仓库无提交，构建日志无法取得 Git 版本元数据，但应用构建成功。当前 FE8J／FE6 适配后的新构建见下文版本适配记录。

### 目录与 i18n 重构验收

Go module 已调整为 `github.com/woxQAQ/febuildergba-mygo`。根目录只保留启动入口，窗口协调、页面、组件及共享编辑状态分别位于 `internal/app`、`internal/pages`、`internal/components` 和 `internal/editor`。原自定义双语字典已替换为 MyGo catalogs，151 条消息通过 `go tool mygo i18n check`。

- `golangci-lint run ./...` 为 0 issues；`go vet`、MyGo 生命周期检查、普通及无 CGO 构建均通过，未降低 lint 配置。
- ROM 保存现在汇报文件关闭和临时文件清理错误；整数编解码使用有界位掩码与显式符号扩展。旧版索引与头像读取按职责拆分，索引目录通过 `os.Root` 限制访问范围。用户显式选择的 ROM 路径与模拟器可执行文件保留带原因的局部 gosec 标注；模拟器参数仍通过独立 argv 传递。
- 旧版索引重构前后 17,376 条记录的 CSV 字节完全一致。
- 使用两份真实 ROM 的临时副本，通过完整页面与输入事件检查人物／职业／道具切换、负数输入、非法草稿、撤销重做、关联跳转、手动表定位、搜索后 Shift 多选、批量应用、十六进制写入与过期草稿保护。搜索改变时同时重置列表选择锚点，避免沿用筛选前的范围。
- 两份副本保存重开后，各只有首个人物的个人 HP 加值一个字节改变；备份与原件一致，原件 SHA-256 未改变。
- 原生窗口在 race 检测下通过中英文切换、菜单重建、按窗口定位的菜单撤销与截图检查；中文菜单为“文件／编辑／保存”，切换后为“File／Edit／Save”。未报告数据竞争。
- 临时验收驱动位于系统临时目录，项目中未新增测试入口或测试框架。

### 原生对话框与撤销焦点验收

修复了原生“编辑 → 撤销／重做”忽略文本焦点、始终调用 ROM 历史的问题。共享 `TextDraft` 组件标记尚未应用的文本输入；每帧采集焦点，菜单在窗口聚焦或编辑上下文变化时选择原生文本命令或文档历史。搜索、批量值、手动表定位及十六进制草稿接入同一规则，键盘快捷键也使用该判断。

- FE8U 原版与 Vision Quest 的两个真实原生窗口，在 `-race` 下完成搜索菜单／键盘撤销重做、数值文档撤销、跨窗口焦点切换与关闭隔离。十六进制地址和待写入草稿的撤销只改文字，ROM 修订不变；批量和手动表定位的单独原生撤销场景仍待验收。
- 通过 `NSTextInputClient` 检查搜索中的中文组合／提交、有效数值 `12` 的组合隔离与单次提交，以及非法中文数值草稿。真实输入法候选窗口尚未验收。
- `NSOpenPanel` 和 `NSSavePanel` 的显示、取消、忙碌解除及父窗口关闭保护通过。真实 `NSAlert` 的保存／不保存／取消、中英文按钮、保存失败后保留窗口与未保存历史通过。
- 关闭保存后的 FE8U 副本只有 `0x803D70` 的 HP 字节 `0 → 1`；重开读到保存值，备份与原件一致。Vision Quest 的放弃操作未写盘。外部替换导致关闭保存失败时，磁盘外部内容保持不变；两个 ROM 原件哈希不变。
- 已更新本地签名的应用包及 DMG，未新增依赖、测试文件或项目内验收入口。详细输入、临时证据、静态检查与限制见 [桌面验收](desktop-acceptance.md)。

### FE8J／FE6 基础适配

已将 FE8J 和 FE6 日文原版接入同一工作区、文档历史和固定长度写入。`gameFormat` 归集各游戏的表位置、文本和素材参数；FE6 的人物 ID 0 占位、48／72／32 字节表、职业熟练度／特性位置及不适用字段分别处理，没有新增通用元数据包。

- FE8J 三表 589 条、FE6 三表 430 条记录均无名称解码错误或未映射标记；日文搜索、原生人物输入、撤销重做、保存及字段差异在 `-race` 下通过。
- 有符号值、批量单次撤销、非法批次拒绝、手动定位、过期原始草稿及精确四字节保存重开通过，备份与原件一致；头像、职业素材和非零图标引用均能读取并已审阅实际图像。
- FE8U 原版及两个改版与 `b9b6e29` 的表、记录、字段、名称、补丁状态、图像摘要和相同编辑的完整保存结果一致，原件哈希保持不变。
- FE6 翻译样本重用日文字库槽位，当前会误读名称，尚未接入该自定义映射；FE7、变长写入、补丁组合和新版本章节效果保持待办。

详细输入、字段偏移、保存哈希及限制见 [版本适配验收](version-adapters-acceptance.md)。lint 为 0 issues，Go vet、MyGo 生命周期及 i18n 检查、普通和无 CGO 构建均通过；应用包及 DMG 已更新为约 9.8 MB／3.2 MB，并完成本地签名。本轮未新增依赖 module、单元测试或项目内验收入口；已有 `x/text` 改为直接依赖以读取 Shift-JIS。

### 真实 ROM

`febuildergba-go/.rom/` 包含《Vision Quest v3》和《Dream of Five》，分别为 28,947,180 和 30,932,456 字节，均为 FE8U 改版。两份输入的哈希、表地址、字段修改、备份、批量、手动布局、原始字节及 UI 结果详见 [ROM 验收记录](rom-acceptance.md)。原件保持不变。

两个修改后副本此前由 mGBA 0.10.5 加载，通过 GDB stub 确认修改后的等级和基础 HP 字节，并观察到启动活动。

后续加入 FE8U 原版，三份样本均从新游戏进入内部章节 `0x00`、第 1 回合。分别只改人物、只改职业、只改武器，以及组合修改，对照属性页、装备页、攻击预览和实际战斗。人物个人 HP `+3`、力量 `+2`，职业基础 HP `+4`、武器威力 `+3` 均产生预期效果；组合副本 HP `+7`、每次攻击伤害 `+5`，敌人实际 HP 和耐久消耗吻合。无修改重放与基线一致，现有 mGBA 桌面程序也载入并核对了相同章节状态。原件哈希保持不变，完整数值及证据见 [章节验收](rom-chapter-acceptance.md)。

另建只改人物表等级的副本后，三个开场角色的等级均不变。FE8U 使用章节部署定义的等级；人物表等级写入成功不代表部署等级改变。章节单位编辑尚未实现，成长率／升级、上限、转职及其他道具效果仍待对应游戏场景验收。

Dream of Five 的窄字库名称已经修复。两份 ROM 的人物、职业和道具合计 1,185 条记录均无未映射名称标记；这是当前语料的显示结果，不代表任意自定义字库都已支持。字体别名仅影响显示与搜索，原始编码保持不变。

人物与职业编辑页已显示真实头像或职业卡，道具页显示图标，支持像素缩放和素材原始数据跳转。按各表资源 ID 去重后，Vision Quest 的 131 个人物头像、73 个职业头像／卡片、186 个图标，以及 Dream of Five 的 134、51、194 个对应素材均能解码。已检查实际图像输出。图像导入、动画、字库编辑及 HALFBODY 格式的真实样本验收仍待完成。

### 已有文件与交互记录

本轮及此前已检查过：整体撤销与跨保存点重做、分支历史、无效／越界／过期批次的完整拒绝、外部修改阻止覆盖、另存与备份失败保留原件、保存期间继续编辑时的快照一致性、损坏指针后的手动定位恢复，以及相互独立的工作区。

已用真实 ROM 副本验证记录显示、多选批量预览、保存重开、手动布局、原始字节编辑和 880×560 最小窗口。本轮紧凑布局另外通过应用自身的 CapturePage 获取原生窗口图像，审阅人物、职业、道具、原始数据、中英文、明暗主题及最小尺寸。

向原生应用自身发送 AppKit 键鼠事件，确认了方向键、Tab、不完整数值草稿、底部加号、Cmd+Z／Cmd+Shift+Z 和 Cmd+S。两份副本最终各只有指定职业基础 HP 的一个字节改变，备份与原件一致。临时采集及操作代码已移除，结果详见 ROM 记录。此结果尚未覆盖输入法候选、系统菜单逐项点击、对话框、触控板和完整 GPU 交互体验。后续对话框及编辑菜单检查见 [桌面验收](desktop-acceptance.md)，其中文件选取／确认与替换、真实候选、触控板和完整 GPU 体验仍待完成。

### 当前环境

- Go `go1.27.1 darwin/arm64`，macOS Command Line Tools SDK 可用。
- 原生应用已携两份真实 ROM 的临时副本完成截图及键鼠事件检查；应用包和 DMG 已按当前布局更新。
- `/Applications/mGBA.app` 存在并已用于上述真实 ROM 检查，版本 `0.10.5`。PATH 没有 `mgba`，编辑器支持 bundle 路径。
- 原生程序出现过 `IMKCFRunLoopWakeUpReliable` 日志，实际输入法影响尚未确认。
- PATH 中没有 `mono`，旧 C# 程序的对照执行环境仍未建立。

## 尚未收口的事项

1. 来源索引已经建立，完整行为叶项、动态入口及外部格式仍需盘点。
2. NarrowFont 之外的字符映射、更多原版／改版样本、补丁组合和额外字段仍需实现及验收。
3. SkillSystem 与魔力拆分目前只有有限特征识别；安装、配置和扩展表编辑未实现。
4. 打开／另存面板的实际文件选取、确认及替换，真实中文输入法候选、系统菜单其余操作、触控板与完整 GPU 交互仍需桌面验收；已通过的取消、关闭确认和原生文本接口结果见桌面记录。
5. 手动布局目前沿用已有字段定义；全局引用查询、可变长写入和资源搬迁尚未实现。
6. 外部文件只在保存及聚焦时检查，尚无持续监视；备份和历史不自动清理；语言偏好由 MyGo 持久化，其他编辑偏好未持久化。
7. FE8U 原版及两个改版的开场章节中，人物／职业 HP、力量、武器威力和实际战斗已完成确认；等级字段边界已记录。阶段 1 的其余场景按覆盖矩阵继续收口；FE8J／FE6 已接入基础适配并完成文件与 UI 验收，新版本章节效果独立待验收。

当前不能宣称阶段 0 全部完成、阶段 1 已通过完整游戏验收，或新版与旧版功能等价。
