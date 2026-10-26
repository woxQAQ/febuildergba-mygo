# 功能覆盖矩阵

基线：旧版子模块 `8e3557912ddab708d11bf85ad311b41bb6630fd1`。最终范围见 [重建计划](rebuild-plan.md)。本文件记录实际行为，不以窗口或源码文件数量估算功能完成率。

[来源索引](../febuildergba-go/README.md#覆盖索引) 已枚举 `.csproj` 的独立编译项、`Program.cs` 中显式检查的 CLI 参数、配置目录的 TXT 文件，以及能静态提取到的窗口调用者。当前有 312 个表单、13 个用户控件、105 个其他源码项、12 个唯一 CLI 参数（含启动与 DEBUG 参数）、4,185 个按 `PATCH_` 文件名前缀归类的条目和 12,749 个其他 TXT 文件。`config` 只表示未按该前缀识别的 TXT，不代表文件内容都是配置；其中也有补丁定义、游戏文本、汇编符号和说明文档。

索引共 17,376 项，其中 TXT 共 16,934 个：`config/data` 下 422 个、`config/patch2` 下 16,509 个、`config/translate` 下 3 个。不同游戏、语言及补丁历史版本会分别占用条目；文件数量不能等同于不同功能数。所有 `assessment` 值均由生成器固定写成 `unassessed`，不是新版完成状态，也不应手工维护。正则提取的调用者不是完整调用图；动态跳转、菜单子操作、二进制素材、非 TXT 外部格式及独立工具仍需要人工对照。下面的首批行为矩阵不替代这些剩余盘点。

状态使用待盘点、待实现、实现中、部分实现和已实现；部分实现需注明范围。

## 基础编辑与原生界面

| ID | 能力 | 旧版依据或前置能力 | 实现范围 |
| --- | --- | --- | --- |
| inventory.sources | 旧版来源索引 | 项目文件、Program、配置目录 | 已实现生成器；行为叶项仍待盘点 |
| native.records | 浏览、搜索、多选记录 | MyGo Table | 已实现 |
| native.density | 能力矩阵与紧凑字段编辑 | 字段定义、MyGo Grid／TextInput | 已实现；窄窗口换行，支持键盘输入和底部微调 |
| native.input | 组合输入与提交 | MyGo TextInput | 已接入；组合文字保留为草稿 |
| native.pixels | 像素缩放与拾取 | 地图／图像编辑 | 原型已实现；地图编辑尚未接入 |
| native.script | 大型脚本文本编辑 | 事件编辑 | 原型已实现；尚无事件语义 |
| native.files | 系统打开／保存对话框 | MyGo Dialog | 已接入异步原生面板、忙碌状态和取消处理 |
| native.close | 关闭时保存／放弃／取消 | NSAlert、文档保存 | 已接入；保存失败保留窗口与修改 |
| native.history-focus | 按焦点撤销与重做 | 原生菜单、文本草稿、文档历史 | 已实现；文本草稿与 ROM 历史分别处理 |
| native.launch | macOS 原生应用 | Go、SDK、MyGo | 已接入应用入口及打包配置 |
| rom.open | 识别并打开 ROM | ROM.LoadLow | 已接入 FE8U／FE8J／FE6；FE7 待实现 |
| rom.noop | 无修改保存 | ROM.Save | 已实现；保留原有字节 |
| rom.history | 修改、撤销、重做及保存点 | Undo、InputFormRef | 已实现；单字段、批量和原始字节共用历史 |
| rom.save | 保存、另存、备份及失败处理 | MainFormUtil | 已实现快照写入、备份和替换 |
| rom.save-snapshot | 保存期间保持文档可读 | 后台保存边界 | 已实现；I/O 不持有状态锁，新修改保持未保存 |
| rom.external | 检测外部修改 | ROMUpdateWatcher | 部分实现；保存与聚焦时检查摘要，持续监视未实现 |
| rom.isolation | 独立文档与历史 | 多文档模型 | 已实现；每窗口独立会话 |
| fe8u.repoint | 按实际指针定位搬迁表 | ROMFE8U、U.FindROMPointer | 已实现；单表解析失败不关闭其他表 |
| fe8u.units | 人物基础字段 | UnitForm／Designer | 部分实现；基础字段已接入，补丁扩展字段未完整覆盖 |
| fe8u.classes | 职业基础字段 | ClassForm／Designer | 部分实现；基础能力等字段已接入，地形与动画资源未覆盖 |
| fe8u.items | 道具基础字段 | ItemForm／Designer | 部分实现；基础字段已接入，补丁扩展字段未完整覆盖 |
| fe8u.batch | 多选预览与批量应用 | 多记录字段写入 | 已实现；一次应用形成单次撤销，搜索改变清空多选 |
| fe8u.references | 关联职业跳转 | 字段引用 | 部分实现；正向跳转已接入，全局反向引用待实现 |
| fe8u.names | 名称读取 | FETextDecode、TextForm.Direct、NarrowFont | 部分实现；已有窄字库映射，其他编码与字库待接入 |
| fe8u.patches | 布局相关补丁识别 | PatchUtil、MagicSplitUtil | 部分实现；NarrowFont、SkillSystem、魔力拆分及头像扩展特征识别，补丁管理未实现 |
| fe8u.asset-preview | 头像、职业卡及图标预览 | ImagePortraitForm、ImageItemIconForm、ImageUtil | 已实现读取、像素缩放及原始数据跳转；导入待实现 |
| fe8u.layout | 手动表定位 | InputFormRef 高级操作 | 已实现；沿用已知字段，只影响当前窗口，不改写指针 |
| fe8u.raw | 固定长度字节编辑 | 原始数据操作 | 已实现；每次 1–4096 字节，写入检查文档归属及修订 |
| ui.language | 中英文界面与诊断 | MyGo i18n | 已实现；切换同步所有窗口与菜单，保存语言偏好 |
| emulator.launch | 保存后打开模拟器 | 外部工具 | 已实现；支持配置可执行文件、PATH 和 macOS bundle |
| emulator.warp | 跳转指定章节 | EmulatorMemoryUtil.CHEAT_WARP_FE6/FE7/FE8 | 待实现；涉及 RAM 事件注入、版本差异及章节状态初始化 |

## FE8J／FE6 基础版本适配

以下适配面向日文原版，共用现有工作区与文档历史。

| ID | 能力 | 旧版依据或前置能力 | 实现范围 |
| --- | --- | --- | --- |
| fe8j.units | 人物基础字段 | ROMFE8JP、UnitForm | 已接入；补丁扩展字段待实现 |
| fe8j.classes | 职业基础字段 | ROMFE8JP、ClassForm | 已接入；其他职业资源待实现 |
| fe8j.items | 道具基础字段 | ROMFE8JP、ItemForm | 已接入；补丁扩展待实现 |
| fe8j.names | 名称读取与搜索 | FETextDecode、FETextEncode | 已接入；文本写入与自定义字库待实现 |
| fe8j.assets | 头像、职业卡及图标 | ImagePortraitForm、ImageItemIconForm | 已接入读取；导入和扩展格式待实现 |
| fe6.units | 人物基础字段 | ROMFE6JP、UnitFE6Form | 已接入；跳过 ID 0 占位，扩展字段待实现 |
| fe6.classes | 职业基础字段 | ROMFE6JP、ClassFE6Form | 已接入 72 字节布局；无转职加成字段，其他职业资源待实现 |
| fe6.items | 道具基础字段 | ROMFE6JP、ItemFE6Form | 已接入 32 字节布局；无武器经验字段，扩展行为待实现 |
| fe6.names | 名称读取与搜索 | FETextDecode、FETextEncode | 已接入日文原版；翻译版自定义字库会误读，映射与写入待实现 |
| fe6.assets | 头像、职业卡及图标 | ImagePortraitFE6Form、ImageItemIconForm | 已接入读取及 16 字节头像布局；导入与动态图像待实现 |
| adapters.shared-history | 多版本共用编辑与历史 | 文档、布局、原始区域 | 已接入批量、撤销、手动定位及原始字节操作 |

## 后续功能组

下表保持完整路线的入口可追踪；每个功能组进入实现前，必须把索引中的相关表单、菜单动作、数据规则和格式拆成具体功能叶项，不能直接将整组标记完成。

| ID | 功能组／代表性来源 | 依赖 | 状态与下一次拆分 |
| --- | --- | --- | --- |
| metadata.definitions | [游戏格式与规则定义库](metadata-design.md) | 具体版本、事件或补丁功能的需求 | 设计已确认／暂缓实施：当前不创建包或迁移定义，按实际需要接入 |
| adapters.games | `ROMFE8JP`、`ROMFE6JP`、`ROMFE7U`、`ROMFE7JP` | 文档、布局、编码 | 部分实现：FE8J／FE6 日文原版基础表、名称和预览已接入；FE7J/U、更多字库和改版布局待实现 |
| storage.allocation | `InputFormRef.WriteBinaryData`、表扩展与指针更新 | 原子编辑、引用记录 | 待实现：可用空间来源、共享引用和扩容回滚 |
| text.edit | `TextForm`、`FETextEncode`、`SystemTextEncoder`、`ToolTranslateROMForm` | 编码、空间 | 待实现：控制码、字库、翻译及外部文本格式 |
| images.static | `ImagePortraitForm`、各图标／背景编辑器 | 调色板、压缩、空间 | 头像、职业卡及道具图标读取已实现；导入导出、调色板编辑器、其他图像类型和引用更新待实现 |
| chapters.maps | `MapEditorForm`、章节配置、事件单位 | 图像、章节布局、空间 | 待实现：地图绘制、地形、地图变化、部署和 AI |
| chapters.events | `EventScriptForm`、模板与世界地图事件 | 事件指令定义、引用 | 待实现：脚本编解码与交互编辑 |
| gameplay.tables | 支援、商店、升级、掉落及其他规则表 | 表适配、引用 | 待盘点：按来源索引拆分各版本叶项 |
| patches.manage | `PatchForm`、`config/patch2/**/PATCH_*.txt` | 检测、空间、工具 | 待实现：条件、依赖、安装、配置、卸载和失败回滚 |
| tools.assembly | `EventAssemblerForm`、`ToolCustomBuildForm` | 原生外部工具、操作导入 | 待实现：EA、ARM、符号和版本记录 |
| assets.animation | `ImageBattleAnimeForm`、地图动画、魔法效果 | OAM、压缩、图像、空间 | 待实现：动画资源与时序、素材格式 |
| assets.audio | `SongTableForm`、`SongInstrumentDirectSoundForm`、歌曲交换 | 音频编解码、空间 | 待实现：播放、歌曲和音色导入导出 |
| diagnostic.lint | `ToolFELintForm`、各模块 `MakeCheckError` | 已知资源及引用 | 待实现：完整错误集合、定位与修复；当前仅基础解析诊断 |
| tools.rebuild | `ToolROMRebuildForm`／Make／Apply | 空间、引用、未知数据保留 | 待实现：重排与重建格式 |
| tools.debug | `RAM`、`GDBSocket`、反汇编及内存编辑 | 平台与模拟器协议 | 待实现：macOS 平台接口和调试协议 |
| tools.distribution | `ToolUPSPatchSimpleForm`、问题报告、备份恢复 | 差异与文件格式 | 待实现：UPS、其他基线格式及附属信息 |
| cli.compatibility | `Program.ProcCommandLine` 及其他启动参数 | 对应领域操作 | 待实现：lint、rebuild、pointercalc、translate、makeups、songexchange、disasm；当前仅新程序的位置参数打开 |
| release.platforms | 打包、更新、配置迁移与平台行为 | 完整覆盖矩阵 | 待实现／盘点：macOS 分发与更新，以及 Windows/Linux 平台适配 |

## 更新规则

新增能力记录来源条目、版本、补丁前提、外部格式及实现状态，当前摘要见[实现记录](implementation-status.md)。

来源索引用于定位旧实现、核对遗漏和查找关联资源。当前 Go 应用运行时不读取该 CSV，也不依赖旧版 TXT 目录。索引按需在本地生成，不纳入版本管理；旧版基线或扫描规则改变后应重新生成。生成命令见[编辑器 README](../febuildergba-go/README.md#覆盖索引)。

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
