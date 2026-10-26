# 实现记录

当前支持 FE8U、FE8J 和 FE6 日文原版的人物、职业、道具基础编辑，以及批量修改、手动表定位、固定长度十六进制编辑和已有表搬迁。表搬迁可原子更新已确认引用并扩容；表记录数扩展、文本／图像写入、FE7、章节编辑和跳章尚未实现，完整功能状态见[覆盖矩阵](coverage.md)。

范围与开发规则见[重建计划](rebuild-plan.md)，运行、操作和构建命令见[编辑器 README](../febuildergba-go/README.md)。

## 当前实现

| 边界 | 代码 | 实际职责 |
| --- | --- | --- |
| ROM 文档 | [document.go](../febuildergba-go/internal/rom/document.go) | 字节、修订、撤销重做与保存点；原子批次、过期操作拒绝、备份和外部修改检测；快照保存期间不持有状态锁 |
| 空间与引用 | [write.go](../febuildergba-go/internal/space/write.go)、[relocation.go](../febuildergba-go/internal/game/relocation.go) | 末尾追加、有来源的可用区间、4 字节对齐、已确认的外部／内部引用与表搬迁预览；保留源数据 |
| 游戏会话 | [session.go](../febuildergba-go/internal/game/session.go) | 游戏适配、表与字段定义、单记录及批量字段操作 |
| 高级操作 | [advanced.go](../febuildergba-go/internal/game/advanced.go) | 窗口内显式表布局；原始字节写入检查文档归属、长度及修订 |
| 游戏规则 | [format.go](../febuildergba-go/internal/game/format.go)、[fe8u.go](../febuildergba-go/internal/game/fe8u.go)、[fe8j.go](../febuildergba-go/internal/game/fe8j.go)、[fe6.go](../febuildergba-go/internal/game/fe6.go) | 版本标头、表及文本位置、字段差异和素材布局；共用解析与编辑流程 |
| 名称读取 | [text.go](../febuildergba-go/internal/game/text.go)、[narrow_font.go](../febuildergba-go/internal/game/narrow_font.go) | 有界 Huffman／anti-Huffman、Shift-JIS 和紧凑职业名称；验证 NarrowFont 字形后映射，未知编码保留标记 |
| 图像读取 | [graphics.go](../febuildergba-go/internal/gba/graphics.go)、[portrait.go](../febuildergba-go/internal/game/portrait.go)、[assets.go](../febuildergba-go/internal/game/assets.go) | 有界 LZ77、4bpp 图块、调色板、头像、职业卡、道具图标和已知头像扩展 |
| 页面与组件 | [pages](../febuildergba-go/internal/pages)、[components](../febuildergba-go/internal/components) | 记录导航、搜索、多选、字段矩阵、批量预览、素材预览、表定位和十六进制编辑 |
| 共享编辑状态 | [state.go](../febuildergba-go/internal/editor/state.go) | 每窗口独立会话、忙碌与错误状态、撤销重做及草稿修订 |
| 系统边界 | [app](../febuildergba-go/internal/app) | 独立窗口、异步文件对话框、关闭确认、外部修改检测、菜单和模拟器启动 |
| 显示语言 | [locales](../febuildergba-go/internal/locales) | MyGo catalogs、系统语言匹配、英文回退、全应用切换及偏好持久化 |
| 原生能力原型 | [ui-probe](../febuildergba-go/cmd/ui-probe/main.go) | 像素缩放与拾取、输入法输入框、10,000 行代码编辑器 |
| 旧版来源索引 | [legacy-inventory](../febuildergba-go/cmd/legacy-inventory/main.go) | 扫描旧版项目和配置，生成来源及静态调用关系 CSV |

领域核心不导入 MyGo。普通字段及原始字节操作保持等长；表搬迁把复制、引用更新和必要的扩容纳入同一文档操作，一次撤销恢复字节、指针和原文件长度。FE6 人物表包含隐藏的 ID 0，职业／道具表包含解析所需的结束记录。手动表定位只改变当前窗口的解析设置；当前搬迁入口要求恢复自动定位。有效数值立即修改内存文档，未完成输入及组合文字保留为草稿。普通文本草稿使用文本撤销，数值字段使用 ROM 历史。

空间层只更新显式选择且值已核实的引用；表操作从游戏规则中选择引用，不扫描任意指针形状的字节。默认在末尾追加；核心 API 也接受注明来源、与已知表和引用等保护区不重叠的可用区间。界面尚未提供区间声明或全局引用查询。旧区保留，未识别引用仍指向旧副本，不保证未知补丁代码会使用新副本。

核心测试使用修正后的合成标头、表指针与布局，调用实际文档、会话和 LZ77 实现。覆盖三个版本共九种表搬迁、扩容历史、过期预览、引用及区间冲突、容量上限和保存重开；不运行 UI 或模拟器。测试范围及限制见[测试说明](../febuildergba-go/tests/README.md)。

## 当前缺口

- 来源索引已生成，完整行为叶项、动态入口及外部格式仍需盘点。阶段 0 尚未全部完成。
- FE7、更多字库与补丁字段、全局引用查询、表记录数扩展、旧区回收、文本／图像写入、章节与事件编辑等按[覆盖矩阵](coverage.md)继续实现。元数据层设计已确认，按具体功能需求接入。
- 外部文件只在保存及聚焦时检查；备份和历史不自动清理，除语言外的编辑偏好未持久化。
- 模拟器集成只有启动能力，跳章及内存调试尚未实现。

当前实现尚未达到旧版完整功能覆盖。
