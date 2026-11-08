# MyGo 原生 UI 能力与项目适配评估

> 活动提案，核对日期：2026-11-07。项目固定基线：`2585e9f984bc31e34ade88b0b92580923b35e097`，依赖 `v0.7.4`。以 MyGo 原生 UI 的完整文档目录为能力范围，不按功能在哪次发布出现筛选。参考文档固定到上游 `5add3817c74bb7505d0b473bc5fb28abe2201b99`。全能力表描述该参考版的能力，不据此推断每个细节都已存在于 v0.7.4；具体试点单独核对当前 API，近期新增能力注明前提。此文是适配评估，不表示方案已实现或所有控件均已运行验证。

## 结论

**值得进一步利用 MyGo，优先解决控件选型和交互归属。** 当前基础生命周期、虚拟列表、窗口线程回传和领域计划已经形成合理边界；另一方面，正式 UI 仍用普通 TextInput 承担搜索、用 Tabs 表达画笔或缩放取值、手工计算单元格双击、用逐按钮 Tab 导航表示一组工具，并用两个普通文本框展示变更对照。这些都有现成控件或交互原语可供替代或试点。

`SearchField`、`Segmented`、`Toolbar`、`GridView`、`DiffView`、`DoubleClicked/RowDoubleClicked` 在当前 `v0.7.4` 就已存在。它们的采用与依赖升级是两条可以独立推进的工作；`LabelledBy`、`FocusTakesText` 等按目标版能力实施。[当前版 API][api-074]、[当前版 DiffView][diff-074]

优先顺序是：搜索与取值控件 → 图块选择的键盘交互 → 变更审核 → 表格编辑与工具组 → 其他机制的局部简化。不要为了减少应用代码而把事务、过期检查、领域验证和跨页草稿交给控件局部状态。

这里的“原生 UI”指 MyGo 提供的 Go 自绘 UI，包含键盘、IME、焦点、无障碍和主题机制；并非每个 ui 控件都是系统的 NSControl／GTK widget。`mygo.Dialog` 等系统对话框与 `ui.Modal` 也分别适用于文件选择和应用内选择任务。[原生 UI 概览][ui-index]

## 核对方式与范围

- 对照官方目录的全部能力族，给出当前使用、建议采用、按需求触发或暂不采用的理由。没有以“出现次数”估算完成率，也没有要求所有页面使用相同控件。
- 深入阅读了共享控件和正式页面中的记录、道具、人物、头像、文本文件审核、章节地图、部署路径、道路、脚本导航及菜单等代表流程；结合当前领域操作判断 UI 是否能直接承接。
- `cmd/ui-probe/main.go` 只算原型；不能因为那里导入了 ui/editor 就认为正式文本页已经获得代码编辑和 diff 能力。
- 调研期间工作区存在并行日志查看器开发。`internal/pages/log_viewer.go` 仅按当次读取状态旁观：已使用 Table、稳定 Sequence、FollowEnd、SplitVerticalPercent、Services 与 After，不把这些再列为“尚未采用”的固定缺口；本次不修改或验收该开发。
- 本轮只修改评估、计划与导航，不运行 UI、ROM 或模拟器，不重复上一轮的版本升级预检。以下界面改善与风险来自源码和框架契约，尚未取得交互或性能实测。

## 已经用好的机制及应该保留的应用职责

| 现有机制                                  | 代表证据                                                                                                     | 判断                                                                               |
| ----------------------------------------- | ------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------- |
| Element 只在构建期使用，持久身份用 Handle | `internal/pages/records/fields.go:31`、头像与文件审核的 Handle；上一轮目标版 vet 通过。                      | 保留。不能把 Element 或 Base parts 存到页面，也不能用 Valid 守卫代替所有权设计。   |
| 事件回调与同步 polling 混合               | `internal/pages/records/page.go:201` 的 Changed；领域写入与集合增删采用 OnChange/OnClick。                   | 两者都受支持。会改变正在构建的集合时用回调；局部 return/break 等流程保留 polling。 |
| ListState.Key、Selection 与虚拟 Table     | `internal/pages/records/page.go:214`、`internal/pages/eventeditor/table.go:21`、头像列表。                   | 基础列表已用好。继续按领域 ID 标识行；排序、过滤、行编辑、共享输入另行分析。       |
| 基于 Base 定制外观                        | `internal/pages/chapters/navigation.go:59` 使用 TabsBase；头像入口、武器等级和道具图标使用 ButtonBase。      | 这是复用框架机制的正确方式，不能仅因“自绘”就要求换成标准外观。                     |
| Window.Update 与不可变后台输入            | `internal/app/resource_png.go:23` 读取和解码，UI 回传后页面检查来源。                                        | 保留。MyGo 只解决线程发布，不理解 Document、Revision、布局或导航代次。             |
| 跨页面草稿、领域历史和不可变计划          | `internal/editor/state.go:88`；地图、文本、素材的 Draft/Plan。                                               | 保留应用所有权；ui.Local、editor.Buffer 的 Undo 均不能取代 ROM 或领域草稿历史。    |
| Bitmap 与像素画预览缓存                   | `internal/components/mapview/view.go:78`；`internal/components/thumbnails.go:45`。                           | 已避免无条件逐帧重建。测量前不要求全部改成 Canvas、Cache 或 shader。               |
| Modal、系统对话框和 scoped shortcut       | `internal/pages/records/item_text.go:134`、`internal/components/reference.go:7`、`internal/app/dialogs.go`。 | 框架接管模态键盘与平台文件流程；确认、过期拒绝和原始焦点已不存在时的回退仍归页面。 |

```mermaid
flowchart LR
    Domain[领域结果、草稿与不可变计划] --> Page[页面保存选择、未完成输入和来源]
    Page --> Widget[MyGo 控件或 Base 控件]
    Widget --> Interaction[焦点、键盘、IME、无障碍、滚动]
    Interaction --> Action[按领域 ID 发起操作]
    Action --> Domain
```

## 具体机会与优先级

P2 表示值得近期实施，P3 表示明确需求触发；S 为数小时量级，M 为约一天量级的粗估，包含有界复核。高置信度表示源码事实已核实，不表示界面改善已实测；有交互差异的替换仍需试点。

| 优先级 | 建议                                                   | 当前证据与影响                                                                                                                                                                                             | 工作量／改造风险                                                    | 置信度                           |
| ------ | ------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------- | -------------------------------- |
| P2     | 搜索使用 SearchField                                   | `internal/pages/records/page.go:201`、`internal/pages/texteditor/page.go:146`、`internal/pages/portraits/page.go:154` 使用 TextInput。SearchField 提供清空、Escape、搜索语义和默认翻译，过滤逻辑仍归页面。 | S／低；需复核清空后的选择和文本撤销。                               | 高。                             |
| P2     | 将画笔、缩放等取值从 Tabs 改为 Segmented 或 RadioGroup | `internal/pages/chapters/map_editor.go:167`、`internal/components/mapview/view.go:139`、`internal/pages/worldmaps/roads.go:136`。这些改变工具或数值，没有对应独立页面。                                    | S／低至中；选项宽度、焦点身份与键盘通知可能变化。                   | 高。                             |
| P2     | 为图块选择提供框架集合导航                             | `internal/pages/worldmaps/roads.go:175` 把每个图块做成独立 ButtonBase；`internal/pages/chapters/map_editor.go:260` 的图集 canvas 只处理 Clicked。前者可试 GridView，后者先补固定图集的键盘选择。           | M／中；编号、Flip、草稿历史和视口必须保留。                         | 现状高，替换收益中。             |
| P2     | 文本导入审核试用只读 DiffView                          | `internal/pages/texteditor/files.go:247` 用两个只读 TextArea 展示 Before/After，没有差异高亮或同步滚动。                                                                                                   | M／中；保持领域审核来源、完整内容与只读。                           | 现状高，复杂短文显示收益中。     |
| P2     | 工具组使用 Toolbar 或 FocusGroup                       | `internal/pages/chapters/deployment_paths.go:228` 的增删重排是 Row.Wrap 中多个独立按钮；脚本操作栏类似。可减少 Tab 停靠并支持方向键。                                                                      | S–M／中；不能把输入框或重要确认操作无条件藏进 overflow。            | 高。                             |
| P2     | 评估原生双击通知，保留 NumberCells 的编辑事务          | `internal/components/number_cells.go:278` 自行比较 key 和 500ms；框架已有 DoubleClicked 与 RowDoubleClicked。                                                                                              | M／中至高；跨单元格两次点击、完整点击区域和第二次释放时机必须一致。 | 重复事实高，可直接替换的判断中。 |
| P2     | 关联表单 caption、提示、错误                           | `internal/pages/records/fields.go:294`、`internal/pages/records/item_fields.go:140`。优先使用 LabelledBy；普通 Form/Field 可用于简单纵向表单。                                                             | S／低；完整名称不可被短标签覆盖。                                   | 高；LabelledBy 依赖目标版。      |
| P2     | 集中默认文本焦点，保留显式历史归属                     | `internal/components/textdraft.go:10`、`internal/pages/workspace.go:333`。FocusTakesText 可以减少逐文本框登记，但它不决定领域历史。                                                                        | M，先 spike／高。                                                   | 现状高，最佳结构中；依赖目标版。 |
| P3     | 选择器按稳定领域值选择                                 | `internal/pages/records/profile_controls.go:122`、`internal/pages/eventeditor/details.go:16` 维护标题与值的对应关系。短选项试 SelectBase[T] 或 Radio[T]，长选项继续 Combobox。                             | S–M／中；未知值、自定义输入、翻译变化都要保留。                     | 中；现有映射不等于已经错误。     |
| P3     | 按任务增加查找、高亮或诊断                             | `internal/pages/texteditor/page.go:246` 已有 TextArea 和控制指令参考。FindBar/TextRanges 可提供局部查找和标记；CodeEditor 需要明确文本语言与更多编辑需求。                                                 | M／中至高。                                                         | 中。                             |

### 搜索和选择：优先选对控件

SearchField 与 FindBar 解决不同任务。前者过滤记录集合，适合记录、头像、道具图标、文本及日志列表；后者在一份文本中导航匹配结果，应用仍负责计数和定位。不能把所有搜索统一成 FindBar，也不能假定换成 SearchField 就自动提供索引或后台搜索。

Tabs 保留给实际页面，如人物“基础／支援／原始数据”、章节“配置／地图／部署”、世界地图“据点／路线”。画笔、缩放、日夜显示或起终点选择属于状态取值，可用 Segmented、RadioGroup 或 Toggle；其中按钮被选中不应被辅助技术报告成进入另一个页面。[Tabs][tabs]、[Segmented][segmented]、[Radio][radio]

Checkbox 适合独立布尔字段；Switch 更偏向立即生效的设置；Toggle 更适合工具开关。并不因存在这些外观就全部替换 Checkbox。CheckboxState/CheckboxGroup 只在明确的全选／混合状态任务中使用，不能把有领域限制的一组 flags 自动变成“全部打开”。

### 集合和表格：复用导航，不删除领域编辑状态

GridView 适合按可用宽度换列的离散图块集合：有稳定 Key、Selected、Label、二维方向键和自动滚入视野；道路画笔是实际落点。地图图集当前以图片坐标反查格子，固定列数是资源展示含义的一部分；自动换列的 GridView 会改变其空间排列。可保留 atlas Bitmap，用框架 scoped shortcut、焦点及无障碍关系补齐键盘路径，不必把每个像素变成控件。[GridView][grid-view]、[List][list]、[Drawing][drawing]

Table 已有列宽与顺序、排序声明、HeaderMenu、CellMenu、选择、多选、重排和 cell selection。当前记录列表已使用稳定 ID，不需要重建一套表格。具体注意：

- 排序由应用实现，表格仅报告 Sort；必须同时维持领域 ID 和选择，不能依显示行号写 ROM。
- Reorder 只用于领域确实允许改变顺序的集合，回调调用既有草稿操作，不能改变表中必须保留的原生顺序，也不能把视图排序写回 ROM。
- 部署路径目前明确只编辑一个单元格。CellSelect 是矩形多选能力，不等价于当前交互；不能为使用现成 API 引入批量写入。
- DoubleClicked 会取得点击；RowDoubleClicked 保留行的选择行为，但只说明当前指针在这一单元格，未必满足“两次点击同一格”的约束。优先做临时复现，再决定是否删除 NumberCells 的计时状态。
- NumberCells 仍需持有非法文字、未确认编辑、复制／重排身份、来源失效和确认原子性。EditableText 以名字重命名为模型，失焦确认和选中扩展名等行为也不同，不能替代整个 NumberCells。[Table][table]、[EditableText][editable-text]、[当前版行双击][row-click-074]

Tree/Outline 适合真正的层次结构。脚本引用图可能包含环和共享入口，尚不能仅靠缩进假装成树；日志属性树如有明确浏览需求可以试点。长层次集合使用 Outline/OutlineTable 的虚拟化，小型自定义层次可用 TreeBase。

### 文本、代码和审核：根据编辑契约决定

**只读 DiffView 是值得试验的独立改进。** 将现有导入审核中选定条目的 Before/After 作为不可变显示输入；按来源、条目及内容变化建立 Buffer，保留 DiffState，设置 Editable=false，不启用 Keep/Revert 等改变缓冲区的动作。浏览、复制和切换比较都不改变 Import/Plan；应用仍使用原来的领域计划。短句中控制码、中文和换行的高亮效果需要实际评估。[DiffView][diff-viewer]

TextArea 已提供 IME、文本选择、剪贴板、Undo、可见段落布局、TextRanges 和 TrackLines。不要因为大文档 API 存在就认定当前几十行文本必须换成 TextBuffer 或代码编辑器。TextBuffer 适合确认存在全字符串复制成本的长文本；Buffer/Snapshot 的字节、rune、UTF-16 偏移各有契约，不能将 UI 偏移直接当成 ROM 字节偏移。[Text input][text-input]、[Text primitives][text-input-client]

CodeEditor 可为将来明确定义的文本控制语言提供 tokenizer、补全、诊断和查找。当前事件脚本已经是结构化表格，不应替换成需要重新输入源码的入口。RichText 编辑的是字体、段落和附件，Markdown 渲染的是 Markdown；二者不能解释 FE 文本控制指令，也不能作为无损 ROM 编辑器。Markdown 可在将来内置帮助时使用。[Code editor][code-editor]、[Rich text][rich-text-editor]、[Markdown][markdown]

### 动作、反馈和弹层：用框架接管常规交互

纯动作栏可用 Toolbar 接管方向键、焦点组和窄窗口 overflow；需一直可见的应用／取消、来源冲突和输入表单保留在任务附近。若现有换行策略必须保留，先加 FocusGroup 和语义角色，比强制 Toolbar overflow 更合适。

图标按钮已经有 Label/Tooltip，是正确基础；后续统一图标时可用嵌入的 SVG 与 ui.Icon，避免字体缺字或图形不一致。项目当前没有已选定的 SVG 资源，且 DESIGN 明确允许紧凑符号，所以这属于设计改善选项，不是当前 bug，也不应为此新增图标框架依赖。

StatusBar 当前在 Busy 时显示 Working，能说明正在处理；长耗时任务可附加 Spinner 或不确定 Progress，真实有进度时才显示百分比。Meter 可表达已确认容量等有意义的范围，不能把“看起来空白”视为可用空间后绘制容量条。Toast 适合复制／导出结果等短反馈，错误、冲突和必须完成的审核继续常驻；ToastAction 的回调也必须重新检查领域来源，不能绕过统一历史。

现有 Modal 已管理焦点隔离、Escape 和返回焦点；Popover 适合锚定的短任务，Menu/ContextMenu 适合命令，Tooltip 适合短帮助。系统文件对话框继续交给 mygo.Dialog；不存在用 ui.AlertDialog 全量替代系统确认框的需要。[Overlays][overlays]、[Toolbar][toolbar]、[Toast][toast]

### 主题、布局与可访问性：使用系统信息，但保留已决定的设计

当前 ApplyTheme 基于框架主题设置品牌色、正文 13 和密度；这些在 DESIGN 中明确，不能把覆盖系统 Accent 或固定正文直接认定为未用框架的缺陷。若采纳系统 TextScale、高对比或局部 Themed，需要同时明确字号、颜色和密集矩阵的产品策略，避免字体缩放与应用 Zoom 重复计算。

Grid 的 Fixed/Fr/FitContent、局部 Scroll、Wrap 和 Split 已在使用。Track.Min/MinContent 可减少某些手算列宽，但 Sidebar 的首选宽度恢复、地图保持并排和固定 2×2 组成等是具体约束，框架不能自动推导。Sticky 可以用于长审核中的操作区，ListState.Header 用于虚拟列表分段；ScrollSnap 不适合精确字段浏览。Hidden 适合保留可见性切换下的内部滚动状态，不能据此始终构建全部重页面。

可访问性也不止 Label：Toolbar/RadioGroup/GridView 的原生语义、Heading、选中状态、Busy、描述和错误关联、KeyShortcuts 都有作用。绘制 canvas 标一个 Label 并不能让其格子自动可操作；应先确保等价的键盘选择和状态说明。辅助技术需求不等于让所有未知资源变成可写字段。[Accessibility][accessibility]、[Styling][styling]、[Layout][layout]

## 全部能力族的适用判断

本表覆盖官方 UI 目录。每项都有判断；“按需”不表示需要现在增加产品功能。“保留”表示当前策略合理或已有相应机制；“采用／试点”对应上面的实际落点。

| 能力族                                                                    | 当前项目的采用判断                                                                                                                                            |
| ------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Views、Keys、Local、Handle、Services、FocusBind、callbacks、Window.Update | 保留生命周期与线程边界；Local 仅存控件内部状态，v0.8 的 Closer 清理可取消随控件结束的任务；FocusBind 只在有限焦点流程减少重复时采用。                         |
| Input、focus groups、shortcuts、text-input client、IME、clipboard         | 保留文本控件；优先复用分组焦点和 scoped command；不自行实现文本编辑或输入法。                                                                                 |
| Layout、Grid、Split、Scroll、Hidden、Sticky、ScrollIntoView               | 已在使用核心能力；按实际约束减少手算，不移除必要局部滚动；Sticky/Min/Reset 属于局部改善。                                                                     |
| Styling、Theme、Themed、Preferences、RTL                                  | 保留品牌与正文设计；系统偏好策略需独立明确。现有中英文不要求 RTL 界面，但不破坏混合文字和逻辑边界。                                                           |
| Text、RichText、fonts、Selectable、TextRanges                             | 保留可选择的只读内容；按需要增加高亮与跨段选择，未要求富文本排版的字段不引入格式。                                                                            |
| Button、PrimaryButton、ButtonBase、Link、Icon                             | 基本已采用；URL 用 Link，应用回调用按钮或无 URL Link；图标由父按钮命名，后续 SVG 选型单独审查。                                                               |
| Toggle、ToggleGroup、Segmented、Radio/RadioGroup                          | 取值与工具状态的优先选择；与页面 Tabs、独立布尔字段分开使用。                                                                                                 |
| Checkbox、CheckboxState/Group、Switch                                     | flags 保留 Checkbox；有真实全选／混合状态或立即生效设置时采用相应控件，不增设“全开未知位”。                                                                   |
| Select、SelectBase、Combobox、ComboboxBase、Autocomplete                  | 已用字符串选择器及一个 Base 自定义选择器；短项可按领域值选择，长项保留搜索；允许自定义值时选择 Autocomplete，不把查询文字当已选 ID。                          |
| Slider、RangeSlider、Stepper、NumberInput、DecimalInput、Rating           | 当前精确整数、地址和非法输入契约优先。少量纯 UI 数值可用 NumberInput/Stepper；没有金额、评分或连续范围任务，不引入 Decimal/Rating/RangeSlider。               |
| TextInput、TextArea、TextBuffer、TextInputClient                          | 普通输入沿用标准控件；长文本成本确认后再选 Buffer；不为现有数值框写文本客户端。                                                                               |
| SearchField、FindBar                                                      | SearchField 可直接试点集合过滤；FindBar 由正文内查找任务触发，应用提供匹配和定位。                                                                            |
| TokenField、EditableText                                                  | 当前资源引用是单一 ID，不是标签集合；人物名称是带预览／应用的领域文本，不按失焦直接重命名。暂不作为替代入口。                                                 |
| CodeEditor、DiffView                                                      | 优先试只读导入 diff；CodeEditor 按控制语言需求评估，不替代结构化脚本。                                                                                        |
| RichText editor、Markdown                                                 | 前者与 ROM 文本格式不匹配；后者将来可用于内置帮助，不用于解释游戏控制码。                                                                                     |
| Calendar、DateInput、TimeInput                                            | 游戏回合、帧数和延迟不是日历或时钟；当前无适用领域字段。                                                                                                      |
| ColorWell、ColorPicker                                                    | 有潜在配色编辑任务，但当前 PaletteDraft 入口是 PNG 导入，没有逐色 Set API；必须先补领域草稿、BGR555 量化和保留位规则，不能只把色格变为 ColorWell 就宣称可写。 |
| List、Table、Selection、sort、reorder、cell menu                          | 基础已使用；按稳定 ID 增补具体交互；应用负责领域写入和排序含义。                                                                                              |
| GridView                                                                  | 道路图块和可随宽度换列的素材集合值得试点；保持物理图集含义的 canvas 不直接替换。                                                                              |
| Tree、TreeBase、Outline、OutlineTable                                     | 真正层次浏览时采用；共享和循环引用先定义展示模型，不能默认当树。                                                                                              |
| Tabs、TabsBase、Sidebar                                                   | 真正页面保留 Tabs；ui.Sidebar 适合分类入口，不能替代需要 ID 列、多选和虚拟化的记录 Table。                                                                    |
| Router、Back/Forward、Breadcrumbs                                         | 跨引用后返回原处有潜在价值；先定义导航位置、草稿和历史保留策略。顶层标签不因 API 存在而改成路径路由。                                                         |
| Menu、ContextMenu、PopoverMenu、MenuBase                                  | 已采用系统及窗口内菜单；统一外观不是必须目标；自定义菜单需保留原生编辑项、键盘和可访问性。                                                                    |
| Modal、AlertDialog、DialogBase、Popover、Overlay                          | 当前选择任务正确使用 Modal；更复杂内容先用 Base；不自行重写焦点隔离。                                                                                         |
| Tooltip、TooltipBase、Toast、ToastAction                                  | Tooltip 已普遍使用；短反馈按需 Toast，持续错误仍常驻；自定义 TooltipBase 要单独给触发控件 Description。                                                       |
| Progress、Spinner、Meter、Badge                                           | 可为长任务或有实际含义的容量补充，不虚构进度，不把普通正文改成 badge。                                                                                        |
| Image、Bitmap、SVG、Icon、Avatar                                          | 保留 Pixelated、实际图片比例及缓存；头像必须保留完整像素图，不使用圆形 Avatar 裁切或字母占位代替资源。                                                        |
| Video、VideoPlayer、GIF                                                   | 当前头像帧切换不是 GIF 播放。将来真正动画预览可接领域帧解码器；不以播放器表示已实现原生动画领域。                                                             |
| Painter、Path、ImageRegion、Canvas、Cache、HitTest                        | 地图绘制已用 Painter；共享 atlas 可试 ImageRegion；Cache/Canvas 在测量后采用，不能替代来源缓存与失效。                                                        |
| Transforms、Transitions、Motion、AnimationFrame/After                     | 窗口缩放与地图取值保留现有策略；动画仅用于有意义反馈，尊重 ReduceMotion；静态画面不加无条件帧循环。                                                           |
| Shaders、blur、mask、3D、glass 类视觉效果                                 | 当前数据编辑无明确任务，不为了表现力引入；不会自动提高 ROM 图像正确性。                                                                                       |
| ui/chart                                                                  | 当前能力矩阵是精确字段比较，不默认改成图表；未来分析任务有实际数据模型时再使用。                                                                              |
| Native UI windows、title bar、zoom、active/hidden state                   | 已使用 TitleBar、窗口尺寸和 SetZoom；按平台复核活动态和生命周期，不假设框架替代应用多文档管理。                                                               |
| Inspector、Tester、Rendering、Performance、Migration                      | 使用实际控件检查身份、样式和语义；按项目规则只做有界临时 UI 复核；测量后优化，不留下永久 UI 测试。                                                            |
| Browser/WASM native UI                                                    | 是部署方向，不是低成本换后端；现有文件、模拟器、系统菜单等需独立适配，不作为此次 UI 采用事项。                                                                |

## 采用路径与验收原则

已有计划 002、003 继续分别处理标签关联与焦点历史。新增两个不依赖升级的试点：

1. [004：搜索与取值控件](004-use-native-search-and-choices.md)。先记录、文本、头像搜索，再替换明确属于画笔／缩放的 Tabs；不改真正页面导航。
2. [005：只读文本导入 diff](005-preview-text-import-diff.md)。独立显示 Before/After 差异，不增加可编辑缓冲、部分应用或新导入入口。

道路 GridView、NumberCells 双击、工具组、按值选择器是后续有具体边界的候选，不在这轮评估中实施。调色板逐色编辑、导航历史、控制语言编辑器属于产品方向，需要领域模型或交互定义，不伪装成单纯换控件。

后续每个试点检查：键盘／鼠标、隐藏与返回、禁用／只读、中文 IME、来源失效、选择与滚动、实际历史目标。使用真实页面或领域操作的有界临时复核；完成后移除代码，执行项目要求的 `just check`。纯文档评估只检查文档格式、链接和 diff，不把源码审查写成运行通过。

## 固定来源与目录范围

文档默认固定到 `v0.8.3`，当前版 API 可用性由 `v0.7.4` 源码单独核对。以下索引中的其余章节也按上表所属能力族考虑；未深入逐项运行平台特性或测量性能。

[ui-index]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/README.md
[api-074]: https://github.com/egoist/mygo/blob/9bd34e1eb9525d9a1fe698aadc964c49c8ff8e7e/ui/api_gen.go
[diff-074]: https://github.com/egoist/mygo/blob/9bd34e1eb9525d9a1fe698aadc964c49c8ff8e7e/ui/editor/diffview.go
[row-click-074]: https://github.com/egoist/mygo/blob/9bd34e1eb9525d9a1fe698aadc964c49c8ff8e7e/ui/rowclick.go
[tabs]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/tabs.md
[segmented]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/segmented.md
[radio]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/radio.md
[grid-view]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/grid-view.md
[list]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/list.md
[table]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/table.md
[editable-text]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/editable-text.md
[drawing]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/drawing.md
[diff-viewer]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/diff-viewer.md
[text-input]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/text-input.md
[text-input-client]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/text-input-client.md
[code-editor]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/code-editor.md
[rich-text-editor]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/rich-text-editor.md
[markdown]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/markdown.md
[overlays]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/overlays.md
[toolbar]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/toolbar.md
[toast]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/toast.md
[accessibility]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/accessibility.md
[styling]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/styling.md
[layout]: https://github.com/egoist/mygo/blob/5add3817c74bb7505d0b473bc5fb28abe2201b99/docs/ui/layout.md
