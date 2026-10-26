# 功能覆盖与验收矩阵

基线：旧版子模块 `8e3557912ddab708d11bf85ad311b41bb6630fd1`。最终范围见 [重建计划](rebuild-plan.md)。本文件记录实际行为，不以窗口或源码文件数量估算功能完成率。

[来源索引](legacy-inventory.csv) 已枚举 `.csproj` 的独立编译项、`Program.cs` 中显式检查的 CLI 参数、配置目录的 TXT 文件，以及能静态提取到的窗口调用者。当前有 312 个表单、13 个用户控件、105 个其他源码项、12 个唯一 CLI 参数（含启动与 DEBUG 参数）、4,185 个按 `PATCH_` 文件名前缀归类的条目和 12,749 个其他 TXT 文件。`config` 只表示未按该前缀识别的 TXT，不代表文件内容都是配置；其中也有补丁定义、游戏文本、汇编符号和说明文档。

索引共 17,376 项，其中 TXT 共 16,934 个：`config/data` 下 422 个、`config/patch2` 下 16,509 个、`config/translate` 下 3 个。不同游戏、语言及补丁历史版本会分别占用条目；文件数量不能等同于不同功能数。所有 `assessment` 值均由生成器固定写成 `unassessed`，不是新版完成状态，也不应手工维护。正则提取的调用者不是完整调用图；动态跳转、菜单子操作、二进制素材、非 TXT 外部格式及独立工具仍需要人工对照。下面的首批行为矩阵不替代这些剩余盘点。

状态约定：**已有场景记录**或**通过**只说明列出的输入及操作已执行，并不表示保留自动测试。按用户要求，Go 项目的测试文件及专用辅助代码已移除；后续以真实 ROM 副本、实际操作、构建和静态分析验收。**已接入／待验收**表示缺少相应桌面或游戏结果，**部分实现**必须说明范围。详细证据见 [真实 ROM 记录](rom-acceptance.md)。

## 阶段 0 与首个闭环

| ID | 用户操作／能力 | 旧版依据或前置能力 | 当前实现与状态 | 验收证据／剩余条件 |
| --- | --- | --- | --- | --- |
| inventory.sources | 从固定基线追溯旧版入口 | 项目文件、Program、配置目录 | 来源索引已生成 | 可重复生成；行为叶项仍待盘点 |
| native.records | 浏览、搜索、多选密集记录表 | MyGo Table | 已有真实 ROM UI 场景记录 | 两份改版的记录与多选显示；实际滚动体验待桌面验收 |
| native.density | 能力比较与紧凑字段编辑 | 字段定义、MyGo Grid／TextInput | 已实现，原生截图和编辑操作通过 | 两份真实 ROM；中英文、明暗及 880×560；方向键、Tab、无效草稿、底部微调和保存；见 ROM 记录 |
| native.input | 中文组合输入与提交 | MyGo TextInput | 原生文本接口的组合／提交已验收 | 搜索中文、有效数值组合、非法中文数值草稿均通过；真实输入法候选仍待验收，见[桌面验收](desktop-acceptance.md) |
| native.pixels | 像素缩放与拾取 | 后续地图／图像编辑 | 原型已实现 | 已检查 1×、2×、4×、8× 坐标；尚未加载游戏素材 |
| native.script | 编辑大型脚本文本 | 后续事件编辑 | 原型已实现 | 10,000 行编辑与撤销已有记录；尚无事件语义 |
| native.files | 系统打开／保存对话框 | MyGo Dialog | 真实原生面板显示及取消通过 | `NSOpenPanel`／`NSSavePanel` 的取消、忙碌解除和父窗口关闭保护通过；选取、确认和替换仍待验收，见[桌面验收](desktop-acceptance.md) |
| native.close | 关闭时保存／放弃／取消 | 原生 NSAlert、文档保存 | FE8U 原版及 Vision Quest 副本通过 | 中英文按钮、保存后重开、放弃不写盘、取消保留、外部修改导致保存失败时保留窗口与历史 |
| native.history-focus | 菜单／键盘按焦点撤销与重做 | 原生菜单角色、文本草稿、文档历史 | 已修复并完成原生场景验收 | 搜索和十六进制草稿使用文本历史，数值字段使用文档历史；两个窗口切换隔离；批量／表定位接入同一规则，其逐项原生撤销待验收 |
| native.launch | 原生应用在 macOS 启动 | Go、SDK、MyGo | 已执行启动冒烟 | 本轮应用包已更新；启动记录不证明全部交互通过 |
| rom.open | 识别并打开 FE8U／FE8J／FE6 | ROM.LoadLow | 三份原版及两份 FE8U 改版通过 | FE8J／FE6 日文原版的文件及 UI 结果见[版本适配验收](version-adapters-acceptance.md)；FE7 待实现，更多改版布局另验 |
| rom.noop | 无修改保存保持字节一致 | ROM.Save | 三份原版及两份 FE8U 改版通过 | 完整文件一致；FE8J／FE6 结果见版本适配验收，FE8U 见 ROM 记录 |
| rom.history | 修改、撤销、重做及保存点 | Undo、InputFormRef | 已有文件与 UI 场景记录 | 单字段、批量、原始字节、跨保存点与分支历史 |
| rom.save | 保存、另存、备份、失败恢复 | MainFormUtil | 已有文件场景记录 | 精确差异、原件备份、失败保留原件；对话框另行验收 |
| rom.save-snapshot | 保存期间保持文档状态可读 | 后台保存边界 | 已实现并有并发场景记录 | I/O 不持有文档锁，保存点对应写出快照，新修改保持未保存 |
| rom.external | 检测外部修改并阻止覆盖 | ROMUpdateWatcher | 文件场景已有记录 | 保存及聚焦检查摘要；持续监视未实现 |
| rom.isolation | 不同 ROM 独立状态与历史 | 多文档重设计 | 已有工作区及两个原生窗口场景记录 | 原生窗口切换时区分文本／ROM 撤销；关闭另一文档不影响当前文档；通过文件面板打开相同路径的交互仍待验收 |
| fe8u.repoint | 按实际指针定位搬迁表 | ROMFE8U、U.FindROMPointer | 两份真实改版已有记录 | 包含 Dream of Five 位于 0x1B00150 的道具表；独立表失败不关闭其他表 |
| fe8u.units | 人物基础字段 | UnitForm／Designer | 部分实现，保存重开及指定章节效果通过 | 三份样本的个人 HP／力量生效；人物表等级不直接决定开场部署等级，见章节记录；其余字段与扩展行为未全部验收 |
| fe8u.classes | 职业基础字段 | ClassForm／Designer | 部分实现，保存重开及指定章节效果通过 | 三份样本的实际部署职业基础 HP 生效；上限、成长率、转职加成待对应场景，地形及动画资源未覆盖 |
| fe8u.items | 道具基础字段 | ItemForm／Designer | 部分实现，保存重开及指定章节效果通过 | 三份样本的实际装备武器威力影响预览与实际扣血；其他道具效果及补丁字段未覆盖 |
| fe8u.batch | 多选记录、预览、一次应用 | 多记录字段写入 | 已有真实 ROM UI 场景记录 | 三条人物记录的批量修改、单次撤销与保存；筛选清空多选 |
| fe8u.references | 人物／转职字段跳转职业 | 字段引用 | 已有 UI 场景记录 | 正向跳转已实现；全局反向引用待实现 |
| fe8u.names | 名称读取 | FETextDecode、TextForm.Direct、NarrowFont 安装定义 | 部分实现，两份真实语料通过 | 核对实际字形后启用窄字库映射；1,185 条记录无解码错误或未映射标记；其他编码与字库待实现 |
| fe8u.patches | 识别布局相关补丁 | PatchUtil、MagicSplitUtil | 部分实现 | 当前识别 NarrowFont、SkillSystem、魔力拆分及头像扩展；窄字库与头像扩展接入读取，其余扩展字段及补丁管理未实现 |
| fe8u.asset-preview | 人物头像、职业卡、道具图标预览 | ImagePortraitForm、ImageItemIconForm、ImageUtil | 已实现，真实素材读取通过 | 有界 LZ77／4bpp 解码，像素缩放，头像记录／图标／调色板原始数据跳转；已验证素材数量及当前原生窗口截图见 ROM 记录，缩放操作仍待验收 |
| fe8u.layout | 手动指定表位置与范围 | InputFormRef 高级操作 | 已有文件及真实 UI 场景记录 | 沿用已知字段，检查对齐及完整范围；仅本窗口解析设置，不改写指针 |
| fe8u.raw | 任意地址固定长度字节编辑 | 原始数据高级操作 | 已有真实 ROM 与 UI 场景记录 | 每次 1–4096 字节；文档归属与修订检查、全局撤销、结构化显示刷新 |
| ui.language | 中英文界面与诊断 | MyGo i18n、JSON catalogs | 已实现，目录重构验收通过 | 151 条消息；`App.SetLocale` 同步所有窗口与菜单，偏好由框架保存；原生切换与菜单重建在 race 检测下通过，见实现记录 |
| emulator.launch | 保存后打开模拟器 | 外部工具 | 已接入，真实副本已加载 | mGBA 0.10.5 读到修改字节；后续桌面程序成功载入并核对三份副本的章节状态 |
| emulator.chapter | 保存后的修改在章节内生效 | 人物／职业／道具写入与真实游戏运行 | FE8U 原版及两个改版的指定场景通过 | 新游戏进入首章；HP、力量、武器威力的独立／组合修改、实际扣血及耐久吻合；人物表等级不改变该场景部署等级，详见[章节验收](rom-chapter-acceptance.md) |
| rom.real-roundtrip | 真实 ROM 精确编辑后保存重开 | 本地语料 | 两份真实改版通过 | 人物、职业、道具及批量共五处字段字节；原件未改变 |

## FE8J／FE6 基础版本适配

以下状态限于日文原版，详细布局、哈希与操作见[版本适配验收](version-adapters-acceptance.md)。复用现有工作区和文档历史；文件及 UI 检查不表示章节效果通过。

| ID | 用户操作／能力 | 旧版依据 | 当前实现与状态 | 剩余条件 |
| --- | --- | --- | --- | --- |
| fe8j.units | 浏览和修改人物基础字段 | `ROMFE8JP`、`UnitForm` | 255 条记录；数值、批量、撤销、保存重开与原生人物输入通过 | 章节效果及补丁扩展字段 |
| fe8j.classes | 浏览和修改职业基础字段 | `ROMFE8JP`、`ClassForm` | 128 条记录；字段写入和保存重开通过，原生矩阵已审阅 | 章节效果及其余职业资源 |
| fe8j.items | 浏览和修改道具基础字段 | `ROMFE8JP`、`ItemForm` | 206 条记录；威力写入与保存重开通过，原生页已审阅 | 章节效果、补丁组合 |
| fe8j.names | 读取名称并搜索 | `FETextDecode`、`FETextEncode` | 原版三表 589 条记录无解码错误或未映射标记，原生日文搜索通过 | 文本写入、自定义字库 |
| fe8j.assets | 查看人物／职业／道具素材 | `ImagePortraitForm`、`ImageItemIconForm` | 67／62／173 个非零引用可解码；原生页截图已审阅 | 导入、扩展格式、缩放实际操作 |
| fe6.units | 浏览和修改人物基础字段 | `ROMFE6JP`、`UnitFE6Form` | 226 条记录；正确跳过 ID 0，数值、批量、撤销、保存重开与原生人物输入通过 | 章节效果及改版扩展字段 |
| fe6.classes | 浏览和修改职业基础字段 | `ROMFE6JP`、`ClassFE6Form` | 76 条记录；72 字节布局、熟练度与特性位置接入；无转职加成字段，原生矩阵已审阅 | 章节效果及其余职业资源 |
| fe6.items | 浏览和修改道具基础字段 | `ROMFE6JP`、`ItemFE6Form` | 128 条记录；32 字节布局，无武器经验字段；威力写入、保存重开与原生页通过 | 章节效果及其他道具行为 |
| fe6.names | 读取名称并搜索 | `FETextDecode`、`FETextEncode` | 日文原版三表 430 条记录无解码错误或未映射标记，原生搜索通过 | 翻译版使用自定义字形，当前会误读；映射与文本写入待实现 |
| fe6.assets | 查看人物／职业／道具素材 | `ImagePortraitFE6Form`、`ImageItemIconForm` | 101／37／126 个非零引用可解码；16 字节头像布局及原生页已审阅 | 导入、动态图像、缩放实际操作 |
| adapters.shared-history | 多版本共用编辑与高级操作 | 文档、布局、原始区域 | 两版的批量单次撤销、非法批次整体拒绝、手动／自动布局、过期原始草稿拒绝通过 | 更多改版布局及补丁样本 |

## 后续功能组

下表保持完整路线的入口可追踪；每个功能组进入实现前，必须把索引中的相关表单、菜单动作、数据规则和格式拆成可验收叶项，不能直接将整组标记完成。

| ID | 功能组／代表性来源 | 依赖 | 状态与下一次拆分 |
| --- | --- | --- | --- |
| metadata.definitions | [游戏格式与规则定义库](metadata-design.md) | 具体版本、事件或补丁功能的需求 | 设计已确认／暂缓实施：当前不创建包或迁移定义，按实际需要接入 |
| adapters.games | `ROMFE8JP`、`ROMFE6JP`、`ROMFE7U`、`ROMFE7JP` | 文档、布局、编码 | 部分实现：FE8J／FE6 日文原版基础表、名称和预览已接入并完成文件／UI 验收；FE7J/U、更多字库和改版布局待实现，章节效果独立待验收 |
| storage.allocation | `InputFormRef.WriteBinaryData`、表扩展与指针更新 | 原子编辑、引用记录 | 待实现：可用空间来源、共享引用和扩容回滚 |
| text.edit | `TextForm`、`FETextEncode`、`SystemTextEncoder`、`ToolTranslateROMForm` | 编码、空间 | 待实现：控制码、字库、翻译及外部文本格式 |
| images.static | `ImagePortraitForm`、各图标／背景编辑器 | 调色板、压缩、空间 | 头像、职业卡及道具图标读取已实现；导入导出、调色板编辑器、其他图像类型和引用更新待实现 |
| chapters.maps | `MapEditorForm`、章节配置、事件单位 | 图像、章节布局、空间 | 待实现：地图绘制、地形、地图变化、部署和 AI |
| chapters.events | `EventScriptForm`、模板与世界地图事件 | 事件指令定义、引用 | 待实现：脚本编解码、交互编辑与运行验证 |
| gameplay.tables | 支援、商店、升级、掉落及其他规则表 | 表适配、引用 | 待盘点：按来源索引拆分各版本叶项 |
| patches.manage | `PatchForm`、`config/patch2/**/PATCH_*.txt` | 检测、空间、工具 | 待实现：条件、依赖、安装、配置、卸载和失败回滚 |
| tools.assembly | `EventAssemblerForm`、`ToolCustomBuildForm` | 原生外部工具、操作导入 | 待实现：EA、ARM、符号和版本记录 |
| assets.animation | `ImageBattleAnimeForm`、地图动画、魔法效果 | OAM、压缩、图像、空间 | 待实现：动画资源与时序、素材格式 |
| assets.audio | `SongTableForm`、`SongInstrumentDirectSoundForm`、歌曲交换 | 音频编解码、空间 | 待实现：播放、歌曲和音色导入导出 |
| diagnostic.lint | `ToolFELintForm`、各模块 `MakeCheckError` | 已知资源及引用 | 待实现：完整错误集合、定位与修复；当前仅基础解析诊断 |
| tools.rebuild | `ToolROMRebuildForm`／Make／Apply | 空间、引用、未知数据保留 | 待实现：重排、重建格式、完整运行对照 |
| tools.debug | `RAM`、`GDBSocket`、反汇编及内存编辑 | 平台与模拟器协议 | 待实现：macOS 等价行为和调试协议验证 |
| tools.distribution | `ToolUPSPatchSimpleForm`、问题报告、备份恢复 | 差异与文件格式 | 待实现：UPS、其他基线格式及附属信息 |
| cli.compatibility | `Program.ProcCommandLine` 及其他启动参数 | 对应领域操作 | 待实现：lint、rebuild、pointercalc、translate、makeups、songexchange、disasm；当前仅新程序的位置参数打开 |
| release.platforms | 打包、更新、配置迁移与平台行为 | 完整覆盖矩阵 | 待实现／盘点：先收口 macOS，其他平台逐项验收 |

## 更新规则

新增能力必须记录对应源条目、游戏版本、补丁前提、外部格式及操作级验收。自动验证通过不能消除真实 ROM、社区资源或平台验证的待办。行为状态只在本文件及关联验证记录中维护。

来源索引用于定位旧实现、核对遗漏和查找关联资源。当前 Go 应用运行时不读取该 CSV，也不依赖旧版 TXT 目录。只有旧版基线或扫描规则改变时才需要重新生成索引；新版增加功能、调整目录或迁移 i18n 时，更新行为矩阵即可。

核对当前子模块提交 `8e3557912ddab708d11bf85ad311b41bb6630fd1` 后，重新生成的 17,376 条记录与仓库 CSV 字节一致，索引无需更新。可从 Go 模块目录生成到临时文件再比较：

```sh
go run ./cmd/legacy-inventory -out /tmp/febuilder-legacy-inventory.csv
cmp ../docs/legacy-inventory.csv /tmp/febuilder-legacy-inventory.csv
```

## 旧版 TXT 与新版兼容边界

TXT 是文件后缀，具体迁移方式取决于内容与对外契约。新版内部无需复刻 `bin/Debug/config` 的目录层次，也不要求每个旧文件对应一个新文件；需要保留规则含义，以及已承诺的社区资源读写能力。旧编辑器的 UI 文案不作为迁移任务：新版按自己的交互设计编写文案，原文案无需逐条对应或转换。游戏规则说明、枚举含义和社区补丁元信息仍按领域内容评估。

| 内容 | 已检查的旧版实例 | 新版处理原则 |
| --- | --- | --- |
| 旧编辑器自身的界面文案 | `config/translate` 等界面翻译资源 | 不迁移、不计入功能覆盖任务；新版 UI 独立编写文案，由 MyGo catalogs 管理 |
| 枚举及版本相关说明 | `data/ai1_FE8.en.txt` 包含 AI 值、说明和 J/U 条件 | 值与适用条件进入领域数据，显示文案与规则分离 |
| 指令编码与参数定义 | `data/event_FE8.en.txt` 包含字节模板、参数类型、指针语义和说明 | 用结构化领域定义表达，按数据规模选用 Go 表或可校验的数据文件；不能只提取翻译文本 |
| 社区补丁定义 | `PATCH_*.txt` 等文件中的 `TYPE`、`PATCHED_IF`、`EA`、`EDIT_PATCH` | 实现旧格式兼容读取，转换为内部补丁定义，分别处理检测、配置和安装；无需让核心模型沿用旧键值文本 |
| 写入 ROM 的游戏文本 | `patch2/FE8U/skill20220703/0xe00.txt` 等补丁附带文本 | 按游戏资源及 ROM 编码处理，与编辑器的 MyGo i18n 分开 |
| 汇编符号、说明与历史副本 | `*.sym.txt`、README、多个日期版本的 SkillSystem 目录 | 按对应工具工作流使用，保留所需来源及版本信息，不为每个文件建立独立迁移任务 |

补丁引用关系属于格式语义。例如 `FE6/AllInstrumentEA/Patch_AllInstrument.txt` 通过相对路径引用 `FE7U/AllInstrumentEA` 的 `.event` 和 `.bin`。兼容导入必须保留或完整解析这些关系；不能只收集 TXT，也不能随意扁平化补丁包。该文件还说明仅按大小写敏感的 `PATCH_` 前缀分类会漏掉实际补丁，索引分类仍需结合读取器和内容核实。

当前已有 FE8U 表布局和字段定义直接使用 Go 数据结构；完整事件定义加载和社区补丁安装仍未实现。后续实现这些能力时，再按对应工作流引入格式适配，不因来源索引列出了文件就提前复制全部旧资源。
