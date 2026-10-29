# 功能覆盖矩阵

基线为旧版子模块 `8e3557912ddab708d11bf85ad311b41bb6630fd1`，最终范围见[重建计划](rebuild-plan.md)。本表维护功能 ID、实现状态、范围和证据入口；操作步骤见[使用指南](user-guide.md)，详细语义见[格式参考](README.md#格式与行为参考)。

**旧版行为盘点尚未完成。** 已有来源索引和已实现领域的细项，后续功能组、动态入口及外部格式仍需展开。不能用条目数、窗口数或测试数计算整体完成率。生成器、来源分类和排除理由见[旧版来源与兼容边界](legacy-inventory.md)。

## 如何阅读

- 盘点范围：细项记录已确认的操作；“待展开的旧版功能组”保留未穷尽的范围；“补充盘点入口”列出已定位但仍需细化的来源。一个来源可对应多个功能，来源存在不代表行为已盘点完。
- 实现状态：待盘点、待实现、部分实现、已接入、已实现。“已接入”表示当前流程可用；“已实现”仅针对行内声明的能力和前提，不包含尚未列出的旧版能力。
- 验证证据单独列出：合成样本、真实 ROM 文件流程、临时 UI 检查和游戏内运行不能互相替代。下表整理既有实现记录，本轮文档整理不构成重新执行全部回归。
- 新版工程事项与验证条目单独组织，不作为旧版功能完成率的分母或分子。功能组汇总与细项也不能重复计数。

## ROM 与 FE8U 基础编辑

版本前提、旧版依据与详细限制：[ROM 与记录参考](reference/rom-editing.md#基础编辑与原生界面)。

| ID                 | 能力                       | 实现状态 | 范围与缺口                           |
| ------------------ | -------------------------- | -------- | ------------------------------------ |
| rom.open           | 识别并打开 ROM             | 已接入   | 五版标头及最小长度；不认证自定义布局 |
| rom.noop           | 无修改保存                 | 已实现   | 原字节保留                           |
| rom.history        | 修改、撤销、重做及保存点   | 已实现   | 所有已接入编辑共用文档历史           |
| rom.save           | 保存、另存、备份及失败处理 | 已实现   | 快照、备份、替换；备份不自动清理     |
| rom.save-snapshot  | 保存期间保持文档可读       | 已实现   | I/O 不持有状态锁，新修改保持未保存   |
| rom.external       | 检测外部修改               | 部分实现 | 保存与聚焦时检查；持续监视未实现     |
| rom.isolation      | 独立文档与历史             | 已实现   | 每窗口独立会话                       |
| fe8u.repoint       | 按实际指针定位搬迁表       | 已实现   | 实际表指针；单表失败隔离             |
| fe8u.units         | 人物基础字段               | 部分实现 | 基础字段；补丁扩展未完整覆盖         |
| fe8u.classes       | 职业基础字段               | 部分实现 | 基础字段；专用地形、动画编辑待接入   |
| fe8u.items         | 道具基础字段               | 部分实现 | 基础字段；补丁扩展未完整覆盖         |
| fe8u.batch         | 多选预览与批量应用         | 已实现   | 一次历史；搜索改变清空多选           |
| fe8u.references    | 关联职业跳转               | 部分实现 | 正向跳转；全局反向引用待实现         |
| fe8u.names         | 名称读取                   | 部分实现 | 已有窄字库映射；其他字库待接入       |
| fe8u.patches       | 布局相关补丁识别           | 部分实现 | 已知布局特征；补丁管理未实现         |
| fe8u.asset-preview | 头像、职业卡及图标预览     | 已实现   | 标准素材读写；扩展头像只读           |
| fe8u.layout        | 手动表定位                 | 已实现   | 只影响当前窗口解析                   |
| fe8u.raw           | 固定长度字节编辑           | 已实现   | 固定长度、文档身份与修订检查         |
| emulator.launch    | 保存后打开模拟器           | 已实现   | 可执行文件、PATH、macOS bundle       |
| emulator.warp      | 跳转指定章节               | 待实现   | RAM 注入、版本及章节状态待适配       |

## FE8J／FE6 基础版本适配

版本前提、旧版依据与详细限制：[版本参考](reference/rom-editing.md#fe8jfe6-基础版本适配)。

| ID                      | 能力                 | 实现状态 | 范围与缺口                              |
| ----------------------- | -------------------- | -------- | --------------------------------------- |
| fe8j.units              | 人物基础字段         | 已接入   | ROMFE8JP、UnitForm                      |
| fe8j.classes            | 职业基础字段         | 已接入   | ROMFE8JP、ClassForm                     |
| fe8j.items              | 道具基础字段         | 已接入   | ROMFE8JP、ItemForm                      |
| fe8j.names              | 名称读取与搜索       | 已接入   | FETextDecode、FETextEncode              |
| fe8j.assets             | 头像、职业卡及图标   | 已接入   | ImagePortraitForm、ImageItemIconForm    |
| fe6.units               | 人物基础字段         | 已接入   | ROMFE6JP、UnitFE6Form                   |
| fe6.classes             | 职业基础字段         | 已接入   | ROMFE6JP、ClassFE6Form                  |
| fe6.items               | 道具基础字段         | 已接入   | ROMFE6JP、ItemFE6Form                   |
| fe6.names               | 名称读取与搜索       | 已接入   | FETextDecode、FETextEncode              |
| fe6.assets              | 头像、职业卡及图标   | 已接入   | ImagePortraitFE6Form、ImageItemIconForm |
| adapters.shared-history | 多版本共用编辑与历史 | 已接入   | 文档、布局、原始区域                    |

## FE7U／FE7J 基础版本适配

版本前提、旧版依据与详细限制：[版本参考](reference/rom-editing.md#fe7ufe7j-基础版本适配)。

| ID               | 能力                             | 实现状态 | 范围与缺口                              |
| ---------------- | -------------------------------- | -------- | --------------------------------------- |
| fe7.units        | 人物基础字段                     | 已接入   | FE7U／FE7J；52 字节                     |
| fe7.classes      | 职业基础字段                     | 已接入   | FE7U／FE7J；84 字节                     |
| fe7.items        | 道具基础字段                     | 已接入   | FE7U／FE7J；36 字节                     |
| fe7u.names       | 名称读取与搜索                   | 已接入   | FE7U；Huffman 根／树指针在 0x6B8／0x6BC |
| fe7j.names       | 名称读取与搜索                   | 已接入   | FE7J；Shift-JIS 与 FE7 专属名字形       |
| fe7u.assets      | 头像、职业卡及图标               | 已接入   | FE7U；28 字节头像记录                   |
| fe7j.assets      | 头像、职业卡及图标               | 已接入   | FE7J；28 字节头像记录                   |
| fe7.shared-edits | 批量、手动布局、原始数据及表搬迁 | 已接入   | 共用会话与文档操作                      |

## 空间分配与表搬迁

版本前提、旧版依据与详细限制：[空间与引用](reference/rom-editing.md#空间分配与表搬迁)。

| ID                        | 能力                             | 实现状态 | 范围与缺口                               |
| ------------------------- | -------------------------------- | -------- | ---------------------------------------- |
| storage.append            | 末尾分配、对齐和容量检查         | 已实现   | FE8U／FE8J／FE7U／FE7J／FE6；最大 32 MiB |
| storage.free-ranges       | 有来源的可用区间                 | 部分实现 | 核心 API 已有；UI 声明及持久化待实现     |
| storage.references        | 搬迁时更新已确认引用             | 部分实现 | 已确认引用；全局查询待实现               |
| storage.table-move        | 已有人物／职业／道具／章节表搬迁 | 已实现   | 五版四类已有表；记录数不增加             |
| storage.expansion-history | 扩容与字节／指针共用历史         | 已实现   | ROM 文档                                 |
| storage.table-grow        | 增加表记录、修改容量规则         | 待实现   | 表布局与游戏寻址规则                     |
| storage.reclaim           | 旧区回收及完整共享引用分析       | 待实现   | 已确认全部使用方                         |

## 五版名称写入

版本前提、旧版依据与详细限制：[名称编码](reference/text-and-images.md#五版名称写入)。

| ID                 | 能力                   | 实现状态 | 范围与缺口                                       |
| ------------------ | ---------------------- | -------- | ------------------------------------------------ |
| text.name-input    | 名称草稿与共享用途预览 | 已接入   | FE8U／FE8J／FE7U／FE7J／FE6；已有非零文本 ID     |
| text.name-encode   | 按当前树压缩名称       | 已实现   | 英文 ASCII／日文 Shift-JIS；有效 Huffman 树      |
| text.name-symbols  | 紧凑职业名双向映射     | 已实现   | FE8J／FE7J／FE6 原版字形                         |
| text.name-write    | 追加文本并原子更新入口 | 已实现   | 有效主文本表指针；最多 32 MiB                    |
| text.name-versions | 版本与字库边界         | 已接入   | 原版字符集；字库、扩表及 anti-Huffman 写入待实现 |

## 五版描述正文编辑

版本前提、旧版依据与详细限制：[描述编码](reference/text-and-images.md#五版描述正文编辑)。

| ID                        | 能力                           | 实现状态 | 范围与缺口                                        |
| ------------------------- | ------------------------------ | -------- | ------------------------------------------------- |
| text.description-input    | 人物／职业／道具描述及使用描述 | 已接入   | FE8U／FE7U／FE8J／FE7J／FE6 原版；已有非零文本 ID |
| text.description-controls | 正文与控制单元无损往返         | 已接入   | 英文 ASCII／日文 Shift-JIS、换行及原有控制码      |
| text.description-write    | 追加描述并原子更新入口         | 已接入   | 有效主文本表指针与 Huffman 树                     |
| text.description-versions | 版本与字库边界                 | 已接入   | 原版字符集；自定义字库及新 ID 待实现              |

## 五版通用文本与控制指令

版本前提、旧版依据与详细限制：[文本与指令](reference/text-and-images.md#五版通用文本与控制指令)。

| ID                       | 能力                         | 实现状态 | 范围与缺口                                     |
| ------------------------ | ---------------------------- | -------- | ---------------------------------------------- |
| text.browse              | 浏览、搜索与 ID 定位         | 已接入   | 五版；连续有效的现有文本表                     |
| text.commands            | 可读指令及原始表示           | 已接入   | 原版共有指令及英文引号；Shift-JIS 字符优先识别 |
| text.command-edit        | 指令增删与参数修改           | 已接入   | 已确认指令及当前 Huffman 树                    |
| text.shared-preview      | 草稿与共享影响预览           | 已接入   | 已知记录字段；事件使用方未完整索引             |
| text.dialogue-write      | 原子写入及历史               | 已接入   | 有效主入口；最多 32 MiB                        |
| text.dialogue-extensions | 表容量、自定义引擎与外部格式 | 待实现   | 新 ID、自定义引擎、外部格式、游戏预览          |

## 五版人物／职业详情与头像表

版本前提、旧版依据与详细限制：[人物与职业设计](record-editor-design.md)、[头像设计](portrait-editor-design.md)。

| ID                   | 能力                 | 实现状态 | 范围与缺口                                 |
| -------------------- | -------------------- | -------- | ------------------------------------------ |
| records.abilities    | 横向能力矩阵与范围   | 已接入   | 五版已有记录                               |
| records.reference    | 初始参照及线性估算   | 已接入   | 当前默认职业与字段                         |
| records.weapons      | 熟练度、等级与继承   | 已接入   | 版本门槛、默认职业                         |
| records.traits       | 四字节特性           | 已接入   | 版本已确认位                               |
| records.links        | 身份与关联资源       | 已接入   | 已确认字段与字节入口；专用编辑器待接入     |
| portraits.table      | 独立资源表与使用处   | 已接入   | 当前头像表指针                             |
| portraits.parts      | 部件与表情检查       | 已接入   | 五版原生素材                               |
| portraits.position   | 眼口定位与默认眨眼   | 已接入   | FE6 16 字节／FE7、FE8 28 字节记录          |
| portraits.references | 人物引用与小头像来源 | 已接入   | 原生引用与通用小头像；通用 PNG、扩展待接入 |
| portraits.edit       | PNG 与原始资源       | 已接入   | 已有原生 ID；新增、扩表、自定义引擎待接入  |

## 五版静态素材 PNG 读写

版本前提、旧版依据与详细限制：[PNG 格式与写入](reference/text-and-images.md#五版静态素材-png-读写)。

| ID                   | 能力                             | 实现状态 | 范围与缺口                          |
| -------------------- | -------------------------------- | -------- | ----------------------------------- |
| image.png            | PNG 导出与替换预览               | 已接入   | 五版；已有图标、普通头像及职业卡    |
| image.palette        | 调色板与透明色                   | 已接入   | GBA 4bpp、BGR555                    |
| image.portrait-sheet | 静态头像及眼睛、嘴部、小头像图块 | 已接入   | FE7／FE8；FE6 独立布局              |
| image.write          | 原子追加与资源入口更新           | 已接入   | 默认末尾追加；32 MiB 上限           |
| image.bounds         | 写入及验证边界                   | 已接入   | 原生写入；MUG_EXCEED／HALFBODY 只读 |

## 五版章节配置

版本前提、旧版依据与详细限制：[格式与来源](reference/chapters-and-events.md#五版章节配置)。

| ID                 | 能力                       | 实现状态 | 范围与缺口                                                    |
| ------------------ | -------------------------- | -------- | ------------------------------------------------------------- |
| chapters.browse    | 列表、名称、搜索与多选     | 已接入   | 五版原版及可定位的已有表                                      |
| chapters.layout    | 版本布局与表结束           | 已接入   | 已知指针、完整记录；最多 256 条                               |
| chapters.config    | 天气、迷雾与基本配置       | 已接入   | 已确认字段                                                    |
| chapters.resources | 地图与事件资源 ID          | 已接入   | 资源 ID、静态预览、地图绘制与尺寸写入；其他资源内容另行接入 |
| chapters.music     | 音乐 ID 与线路差异         | 已接入   | 音乐 ID；不含音频播放、导入                                   |
| chapters.text      | 关联文本 ID 与正文跳转     | 已接入   | 文案与实际胜利判定分别编辑                                    |
| chapters.advanced  | 手动布局、原始字节与表搬迁 | 已接入   | 已知字段、完整范围和确认引用                                  |
| chapters.history   | 批量、撤销重做与保存重开   | 已接入   | 共用 ROM 文档                                                 |

## 五版章节地图

版本前提、旧版依据与详细限制：[格式与来源](reference/chapters-and-events.md#五版章节地图)。

| ID             | 能力                    | 实现状态 | 范围与缺口                                                                                           |
| -------------- | ----------------------- | -------- | ---------------------------------------------------------------------------------------------------- |
| maps.resources | 按章节组合资源          | 已接入   | FE8U/J、FE7U/J、FE6；有效当前章节布局                                                                |
| maps.render    | 4bpp 背景图块与地图合成 | 已接入   | 有界 LZ77、512 字节 BGR555 调色板                                                                    |
| maps.inspect   | 格子坐标与地形          | 已接入   | 图块值及地形 ID；属性编辑待接入                                                                      |
| maps.view      | 缩放、平移、网格与选择  | 已接入   | 章节详情中的静态基础地图                                                                             |
| maps.refresh   | 文档修订与章节上下文    | 已接入   | 当前会话、章节 ID、实际记录地址及适用布局；保留失效的未应用草稿                                      |
| maps.errors    | 资源诊断与范围检查      | 已接入   | 原生上限宽 60、高 63、总计 1536 格                                                                   |
| maps.paint     | 地图绘制        | 已接入   | 当前图块集选择、画笔、吸管、矩形填充；地形跟随图块，一次手势一步草稿历史                             |
| maps.write     | 共享预览与原子写入      | 已接入   | 验证共享章节图块集；追加压缩数据并更新选定入口，保留独立入口与未知尾部；过期拒绝、撤销重做及保存重开 |
| maps.resize    | 尺寸调整与越界影响      | 已接入   | 九点锚定扩展／裁剪；部署、路径、区域、地图变化矩形及可确认脚本坐标的越界变化；不平移关联坐标，未知上下文提示                                                       |
| maps.changes   | 地图变化与事件联动      | 待实现   | 门、村庄、桥梁等变化资源编辑及脚本引用；矩形边界已用于尺寸预览                                                                   |

## 五版章节初始部署

版本前提、旧版依据与详细限制：[格式与来源](reference/chapters-and-events.md#五版章节初始部署)。

| ID                    | 能力                     | 实现状态 | 范围与缺口                           |
| --------------------- | ------------------------ | -------- | ------------------------------------ |
| deployments.entries   | 部署入口与版本差异       | 已接入   | 五版有效章节及事件资源表             |
| deployments.discovery | 开场及条件事件引用       | 已接入   | 已确认静态引用；不模拟动态调用或条件 |
| deployments.fields    | 单位属性及位置           | 已接入   | 原版记录字段                         |
| deployments.structure | 新增、复制、删除         | 已接入   | 可读取的选定部署入口                 |
| deployments.movement  | FE8 完整移动路径         | 已接入   | FE8 编码路径；不模拟寻路及动画       |
| deployments.map       | 地图与列表联动           | 已接入   | 可读取的基础地图                     |
| deployments.write     | 预览、共享影响和原子写入 | 已接入   | 选定入口追加；全局使用处未完整索引   |
| deployments.history   | 历史、保存重开及失败隔离 | 已接入   | 共用 ROM 文档和 32 MiB 上限          |

## FE8 完整部署移动路径

版本前提、旧版依据与详细限制：[格式与来源](reference/chapters-and-events.md#fe8-完整部署移动路径)。

| ID              | 能力                   | 实现状态 | 范围与缺口                    |
| --------------- | ---------------------- | -------- | ----------------------------- |
| paths.fields    | 原生段字段             | 已接入   | FE8U／FE8J 的 20 字节部署记录 |
| paths.structure | 段增删与排序           | 已接入   | 原生缓冲区最多 16 段          |
| paths.map       | 编码目标与节点选择     | 已接入   | 可读取的章节基础地图          |
| paths.write     | 部署和增援共用原子写入 | 已接入   | 当前文档、修订和布局          |

## 五版回合增援

版本前提、旧版依据与详细限制：[格式与来源](reference/chapters-and-events.md#五版回合增援)。

| ID               | 能力                         | 实现状态 | 范围与缺口                       |
| ---------------- | ---------------------------- | -------- | -------------------------------- |
| turns.read       | 原生回合条件表               | 已接入   | 五版有效章节事件头               |
| turns.conditions | 起止回合与触发条件           | 已接入   | 类型 2；其他类型保留、查看或删除 |
| turns.create     | 新增与删除回合增援           | 已接入   | 现有文档及章节布局               |
| turns.discovery  | 回合脚本关联部署             | 已接入   | 静态引用；其他条件和指令待接入   |
| turns.write      | 原子追加与共享影响           | 已接入   | 同一文档、修订和手动布局         |
| turns.history    | 历史、重开、上下文与失败隔离 | 已接入   | 五版合成样本与文档历史           |

## 五版区域触发事件

版本前提、旧版依据与详细限制：[格式与来源](reference/chapters-and-events.md#五版区域触发事件)。

| ID               | 能力               | 实现状态 | 范围与缺口                |
| ---------------- | ------------------ | -------- | ------------------------- |
| areas.read       | 原生常驻条件表     | 已接入   | 五版有效事件头            |
| areas.conditions | 区域与完成标志     | 已接入   | 类型 0B；版本端点语义不同 |
| areas.map        | 范围叠加与选择     | 已接入   | 静态基础地图              |
| areas.create     | 区域增援与部署关联 | 已接入   | 现有章节及布局            |
| areas.write      | 共享预览与原子历史 | 已接入   | 同一文档、修订和布局      |

## 五版人物对话事件

版本前提、旧版依据与详细限制：[格式与来源](reference/chapters-and-events.md#五版人物对话事件)。

| ID               | 能力                 | 实现状态 | 范围与缺口                     |
| ---------------- | -------------------- | -------- | ------------------------------ |
| talks.read       | 对话表与人物关系     | 已接入   | 事件头 +4                      |
| talks.conditions | 双方人物和标志       | 已接入   | 章节普通对话；不包含战斗对话表 |
| talks.create     | 普通对话脚本         | 已接入   | 已有可读取的主表 ROM 文本 ID   |
| talks.resources  | 已有事件资源跳转     | 已接入   | 有界静态发现                   |
| talks.write      | 草稿、共享与资源保护 | 已接入   | 同一文档、修订和布局           |

## 五版章节胜败条件

版本前提、旧版依据与详细限制：[格式与来源](reference/chapters-and-events.md#五版章节胜败条件)。

| ID                | 能力                 | 实现状态 | 范围与缺口                       |
| ----------------- | -------------------- | -------- | -------------------------------- |
| objectives.flags  | 标志判定和完成标志   | 已接入   | 常驻表类型 01                    |
| objectives.rout   | 已确认全灭回调       | 已接入   | FE6 类型 0D，FE7 类型 0E         |
| objectives.create | 首领／全灭／失败模板 | 已接入   | 原生模板；自定义和更多模板待实现 |
| objectives.links  | 文案和实际规则       | 已接入   | 当前章节及已识别脚本             |
| objectives.write  | 两页草稿和原子历史   | 已接入   | 与区域页共用常驻表指针           |

## 通用事件脚本

版本前提、旧版依据与详细限制：[格式与来源](reference/chapters-and-events.md#通用事件脚本)。

| ID                      | 能力                   | 实现状态 | 范围与缺口                                                                                                                                      |
| ----------------------- | ---------------------- | -------- | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| scripts.entries         | 章节入口与调用导航     | 已接入   | 五版当前章节布局                                                                                                                                |
| scripts.decode          | 有界指令列表           | 已接入   | 常用指令；未知或不完整段只读                                                                                                                    |
| scripts.operands        | 原生参数编辑           | 已接入   | 已确认参数、共享位段、带符号值及人物／道具选择；未解释编码保持只读                                                                              |
| scripts.units           | 剧情移动与阵营变更     | 已接入   | 五版按人物移动及三阵营；FE7／FE8 速度；FE6／FE7 人物、坐标及 FE7 阵营分流的手动路径编辑，64 字节边界、独立追加及原子历史                        |
| scripts.rewards         | 剧情道具与金币奖励     | 已接入   | FE6／FE7 指定人物或行动单位道具；FE8 从槽 3 给予道具／金币                                                                                      |
| scripts.presentation    | 背景、音乐及画面过渡   | 已接入   | 五版基础演出；FE8 音乐渐变、音量、CG／教程／文本框模式、继续对话及地图恢复；世界地图和战斗演出另行适配                                          |
| scripts.focus           | 镜头定位与光标高亮     | 已接入   | 五版按人物／坐标定位；FE7／FE8 居中；FE6／FE7 60 帧闪烁及持续坐标光标；FE8 普通／发光高亮与清除                                                 |
| scripts.unit-conditions | 单位状态条件           | 已接入   | FE8 存在、存活、出场、阵营、行动单位、坐标、职业及幸运查询；FE6／FE7 我方队伍及行动单位分支；FE7 出场及死亡标志一次性判定                       |
| scripts.loads           | 单位加载与部署导航     | 已接入   | FE8 LOAD1／2／3、可证明槽来源；FE6 当前队伍模板；FE7 存活、当前队伍、路线／难度及四路部署；独立文本／部署资源契约；RAM 和未知动态来源不自动搬迁 |
| scripts.queue           | 槽赋值、运算与队列     | 已接入   | FE8 十种槽运算及 FIFO；仅恒等复制保留字面量所有权；显式清空后追踪队列；不模拟运行状态或合并任意控制流                                           |
| scripts.control         | 执行控制与条件分支     | 已接入   | FE8 等待变体、计数器、执行模式、按键屏蔽及状态查询；FE6／FE7 跳过状态和结束边界；FE7 路线、难度、选择、教程及回合分支                           |
| scripts.unit-state      | 单位状态修改           | 已接入   | FE8 显隐、HP、行动、出场及移除；操作的槽依赖在指令帮助中说明                                                                                    |
| scripts.dialogue        | 对话模式与路线文本     | 已接入   | FE8 对话模式及续接；FE7 不透明对话和两路文本的独立选择、校验及导航                                                                              |
| scripts.structure       | 插入、复制、删除和排序 | 已接入   | 完整且可静态重定位的脚本                                                                                                                        |
| scripts.write           | 共享预览与原子历史     | 已接入   | 当前文档、修订和布局                                                                                                                            |

## 待展开的旧版功能组

这些组仍需核对菜单子操作、版本差异、格式和附属文件；即使已有部分能力，也不能整组标为完成。代表性来源及依赖由具体功能接入时细化。

| ID                 | 功能组                       | 代表性来源                                                                          | 前置能力                 | 实现状态与缺口                                                                 |
| ------------------ | ---------------------------- | ----------------------------------------------------------------------------------- | ------------------------ | ------------------------------------------------------------------------------ |
| adapters.games     | 游戏版本与改版布局           | [ROMFE8JP.cs](../FEBuilderGBA/FEBuilderGBA/ROMFE8JP.cs)                             | 文档、布局、编码         | 部分实现；原版基础表已接入；字库、补丁和改版规则继续盘点                       |
| storage.allocation | 分配、表扩展与引用           | [InputFormRef.cs](../FEBuilderGBA/FEBuilderGBA/InputFormRef.cs)                     | 原子编辑、引用记录       | 部分实现；已有搬迁；表容量、回收及完整引用管理待实现                           |
| text.edit          | 文本、字库、翻译及外部格式   | [TextForm.cs](../FEBuilderGBA/FEBuilderGBA/TextForm.cs)                             | 编码、空间               | 部分实现；已有 ID 编辑；翻译和外部格式待实现                                   |
| images.static      | 静态图像与调色板             | [ImagePortraitForm.cs](../FEBuilderGBA/FEBuilderGBA/ImagePortraitForm.cs)           | 调色板、压缩、空间       | 部分实现；头像、职业卡、图标；其他图像类型待实现                               |
| chapters.maps      | 章节、地图与部署             | [MapEditorForm.cs](../FEBuilderGBA/FEBuilderGBA/MapEditorForm.cs)                   | 图像、章节布局、空间     | 部分实现；已有绘制、尺寸与越界预览、共享检查及原子保存；地图变化编辑、扩章待实现 |
| chapters.events    | 事件指令、模板和世界地图     | [EventScriptForm.cs](../FEBuilderGBA/FEBuilderGBA/EventScriptForm.cs)               | 事件指令定义、引用       | 部分实现；已确认条件及常用脚本；其余工作流待盘点                               |
| gameplay.tables    | 支援、商店、成长及规则表     | [SupportUnitForm.cs](../FEBuilderGBA/FEBuilderGBA/SupportUnitForm.cs)               | 表适配、引用             | 待盘点；已列部分独立入口；各版本行为与格式未列全                               |
| patches.manage     | 社区补丁管理                 | [PatchForm.cs](../FEBuilderGBA/FEBuilderGBA/PatchForm.cs)                           | 检测、空间、工具         | 待实现；条件、依赖、配置、安装、卸载和回滚                                     |
| tools.assembly     | EA、ARM 与构建工具链         | [EventAssemblerForm.cs](../FEBuilderGBA/FEBuilderGBA/EventAssemblerForm.cs)         | 原生外部工具、操作导入   | 待实现；脚本、符号、工具协议和版本记录                                         |
| assets.animation   | 战斗、地图、魔法动画与 OAM   | [ImageBattleAnimeForm.cs](../FEBuilderGBA/FEBuilderGBA/ImageBattleAnimeForm.cs)     | OAM、压缩、图像、空间    | 待实现；资源、时序及外部格式仍需逐项盘点                                       |
| assets.audio       | 歌曲、音色与音频资源         | [SongTableForm.cs](../FEBuilderGBA/FEBuilderGBA/SongTableForm.cs)                   | 音频编解码、空间         | 待实现；播放、MIDI／WAV、歌曲交换及格式需展开                                  |
| diagnostic.lint    | FELint、定位与修复           | [ToolFELintForm.cs](../FEBuilderGBA/FEBuilderGBA/ToolFELintForm.cs)                 | 已知资源及引用           | 待实现；完整错误集合；当前只有编辑流程解析诊断                                 |
| tools.rebuild      | ROM 重排及重建格式           | [ToolROMRebuildForm.cs](../FEBuilderGBA/FEBuilderGBA/ToolROMRebuildForm.cs)         | 空间、引用、未知数据保留 | 待实现；未知数据保留、引用及资源导入导出                                       |
| tools.debug        | RAM、GDB、反汇编与内存编辑   | [RAM.cs](../FEBuilderGBA/FEBuilderGBA/RAM.cs)                                       | 平台与模拟器协议         | 待实现；平台接口、协议及完整调试工作流                                         |
| tools.distribution | UPS、问题报告与备份恢复      | [ToolUPSPatchSimpleForm.cs](../FEBuilderGBA/FEBuilderGBA/ToolUPSPatchSimpleForm.cs) | 差异与文件格式           | 待实现；格式及非 ROM 附属信息需展开                                            |
| cli.compatibility  | 旧版 CLI 契约                | [Program.cs](../FEBuilderGBA/FEBuilderGBA/Program.cs)                               | 对应领域操作             | 待实现；12 个参数需分别判断；当前仅位置参数打开                                |
| release.platforms  | 发布、更新、配置迁移和跨平台 | [OptionForm.cs](../FEBuilderGBA/FEBuilderGBA/OptionForm.cs)                         | 完整覆盖矩阵             | 待盘点；已有本地 macOS 构建；发行及其他平台待实现                              |

## 补充盘点入口

以下能力从旧版独立入口补入。状态“待盘点”表示已找到来源，但版本、完整操作或对外格式尚未核完；“待实现”也只确认了所述操作，不表示整个来源文件已关闭盘点。全部入口均属于固定旧版基线，暂无对应完整新版工作流。

| ID                 | 能力                   | 旧版来源                                                                                    | 实现状态 | 仍需核对                                        |
| ------------------ | ---------------------- | ------------------------------------------------------------------------------------------- | -------- | ----------------------------------------------- |
| events.battle-talk | 战斗对话表             | [EventBattleTalkForm.cs](../FEBuilderGBA/FEBuilderGBA/EventBattleTalkForm.cs)               | 待实现   | 已定位独立表；与章节普通对话分开核对版本        |
| events.death-talk  | 死亡对话及死亡标志     | [EventHaikuForm.cs](../FEBuilderGBA/FEBuilderGBA/EventHaikuForm.cs)                         | 待盘点   | 人物、文本、标志与各版本结束规则                |
| events.locations   | 地点、访问、宝箱等触发 | [EventCondForm.cs](../FEBuilderGBA/FEBuilderGBA/EventCondForm.cs)                           | 待盘点   | 按条件类型核对，不能由区域事件代替              |
| worldmap.points    | 世界地图节点           | [WorldMapPointForm.cs](../FEBuilderGBA/FEBuilderGBA/WorldMapPointForm.cs)                   | 待盘点   | 节点字段、版本和引用                            |
| worldmap.paths     | 世界地图路线连接       | [WorldMapPathForm.cs](../FEBuilderGBA/FEBuilderGBA/WorldMapPathForm.cs)                     | 待实现   | 已定位路线表；路线资源和扩展需继续核对          |
| worldmap.movement  | 世界地图路径及移动编辑 | [WorldMapPathMoveEditorForm.cs](../FEBuilderGBA/FEBuilderGBA/WorldMapPathMoveEditorForm.cs) | 待盘点   | 图形路径、移动数据及写入流程                    |
| worldmap.events    | 世界地图事件入口       | [WorldMapEventPointerForm.cs](../FEBuilderGBA/FEBuilderGBA/WorldMapEventPointerForm.cs)     | 待盘点   | 版本入口、指令及脚本导入导出                    |
| gameplay.support   | 支援关系、对话与属性   | [SupportUnitForm.cs](../FEBuilderGBA/FEBuilderGBA/SupportUnitForm.cs)                       | 待盘点   | 联合核对 SupportTalk／SupportAttribute 及各版本 |
| gameplay.shops     | 商店货物编辑           | [ItemShopForm.cs](../FEBuilderGBA/FEBuilderGBA/ItemShopForm.cs)                             | 待盘点   | 商店发现、货物列表、扩展及章节引用              |
| gameplay.arena     | 斗技场规则             | [ArenaClassForm.cs](../FEBuilderGBA/FEBuilderGBA/ArenaClassForm.cs)                         | 待盘点   | 联合核对敌方武器与联机参战限制                  |
| tools.flag-names   | 标志命名与附属信息     | [ToolFlagNameForm.cs](../FEBuilderGBA/FEBuilderGBA/ToolFlagNameForm.cs)                     | 待实现   | 已定位名称修改；保存、载入与兼容契约待细化      |
| tools.flag-uses    | 标志使用处查询         | [ToolUseFlagForm.cs](../FEBuilderGBA/FEBuilderGBA/ToolUseFlagForm.cs)                       | 待盘点   | 扫描范围、定位和未识别引用                      |
| tools.diff-patch   | ROM 差异生成 BIN 补丁  | [ToolDiffForm.cs](../FEBuilderGBA/FEBuilderGBA/ToolDiffForm.cs)                             | 待实现   | 已定位 PATCH 文本及二进制输出；多文件流程需细化 |
| text.translation   | ROM 翻译与字库处理     | [ToolTranslateROMForm.cs](../FEBuilderGBA/FEBuilderGBA/ToolTranslateROMForm.cs)             | 待盘点   | 与已有文本 ID 编辑分开核对工作流                |
| tools.event-export | EA 事件导出            | [ToolExportEAEventForm.cs](../FEBuilderGBA/FEBuilderGBA/ToolExportEAEventForm.cs)           | 待盘点   | 导出范围、资源引用与格式兼容                    |

## 新版工程事项

这里维护原生 UI、开发工具及暂缓设计，不参与旧版能力覆盖统计。约束见 [AGENTS](../AGENTS.md)，实现组织见[架构](architecture.md)，开发启动与翻译见[开发说明](development.md)。原型条目仅描述控件试验，产品语义由对应功能行说明。

| ID                   | 能力                                       | 实现状态 | 范围与缺口                             |
| -------------------- | ------------------------------------------ | -------- | -------------------------------------- |
| inventory.sources    | 旧版来源索引                               | 已实现   | 生成器已实现；行为盘点未完成           |
| native.records       | 浏览、搜索、多选记录                       | 已实现   | 当前记录列表                           |
| native.density       | 能力矩阵与紧凑字段编辑                     | 已实现   | 能力矩阵、窄窗口及当前字段检查器       |
| native.input         | 组合输入与提交                             | 已接入   | 组合文字保留草稿                       |
| native.pixels        | 像素缩放与拾取                             | 原型     | 控件原型；产品地图能力见 maps.*        |
| native.script        | 大型脚本文本编辑                           | 原型     | 大文本控件原型；产品语义见 scripts.*   |
| native.files         | 系统打开／保存对话框                       | 已接入   | 异步原生面板、忙碌状态、取消           |
| native.close         | 关闭时保存／放弃／取消                     | 已接入   | 保存失败保留窗口与修改                 |
| native.history-focus | 按焦点撤销与重做                           | 已实现   | 输入草稿与 ROM 历史分别处理            |
| native.launch        | macOS 原生应用                             | 已接入   | 应用入口与本地打包                     |
| diagnostics.errors   | 结构化错误与原因保留                       | 已接入   | 结构化错误、翻译和日志；独立面板待实现 |
| ui.language          | 中英文界面与诊断                           | 已实现   | 中英文、原生菜单与偏好                 |
| developer.restore    | 开发 ROM 路径恢复                          | 已接入   | 开发路径恢复，不保存内存修改           |
| metadata.definitions | [游戏格式与规则定义库](metadata-design.md) | 暂缓     | 按具体版本、事件或补丁需求接入         |

## 验证证据

默认测试使用独立构造的合成数据，调用实际文档和领域操作。可选[集成测试](../tests/integration/real_rom_test.go)动态发现 `.rom/` 中的输入，按可编辑入口验证通用流程，不绑定固定文件或内容；跳过项不计为通过。保存、重开等流程只操作临时副本。这些证据不包含游戏内执行验证。临时 UI 观察不作为永久回归，纯布局调整无需专门测试；规则见[测试说明](../tests/README.md)。

| ID                    | 验证范围                               | 证据入口                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| --------------------- | -------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| maps.validation       | 五版地图绘制、尺寸、影响与文件流程 | [地图读取](../game/maps_test.go)、[绘制与原子写入](../game/map_edit_test.go)、[共享图块集与布局](../game/map_context_test.go)、[尺寸工作流](../game/map_resize_test.go)、[引用与原生边界](../game/map_impact_test.go)、[合成样本](../internal/testutil/maps.go)、[尺寸样本](../internal/testutil/map_resize.go)、[绘制文件流程](../tests/integration/maps_test.go)、[尺寸文件流程](../tests/integration/map_resize_test.go)                                                                                                                                                                                                                                                             |
| paths.validation      | FE8 两版路径字节、文档历史与保存重开   | [路径流程](../game/deployment_paths_test.go)、[合成样本](../internal/testutil/paths.go)                                                                                                                                                                                                                                                                                                                                                                                                     |
| areas.validation      | 五版区域条件、事件写入与文件流程       | [区域流程](../game/area_events_test.go)、[合成样本](../internal/testutil/areas.go)                                                                                                                                                                                                                                                                                                                                                                                                          |
| talks.validation      | 五版人物对话、独立引用与文件流程       | [对话流程](../game/talk_events_test.go)、[合成样本](../internal/testutil/talks.go)                                                                                                                                                                                                                                                                                                                                                                                                          |
| objectives.validation | 五版胜败条件、脚本模板与文件流程       | [胜败流程](../game/objectives_test.go)、[合成样本](../internal/testutil/objectives.go)                                                                                                                                                                                                                                                                                                                                                                                                      |
| scripts.validation    | 五版指令字节、资源编辑与文件流程       | [基本脚本](../game/scripts_test.go)、[剧情](../game/script_scene_test.go)、[镜头](../game/script_camera_test.go)、[加载／队列](../game/script_load_test.go)、[位段／控制](../game/script_control_test.go)、[路线资源](../game/script_word_variants_test.go)、[条件分支](../game/script_word_conditions_test.go)、[手动移动及结束边界](../game/script_movement_test.go)、[动态文件流程](../tests/integration/scripts_test.go)、[路径文件流程](../tests/integration/scripts_movement_test.go) |

其他领域测试位于对应包；样本构造规则见 [fixture 约定](../testdata/fixtures.md)，新增用例无需同步登记数量。

## 更新规则

修改能力时更新对应行，保留稳定 ID，注明版本、补丁前提、旧版入口、外部格式及未支持范围。详细格式与算法只在专题参考维护，测试证据以代码为准；不另建重复的实现状态文档。完整来源尚未归类前，保留“盘点未完成”的说明。
