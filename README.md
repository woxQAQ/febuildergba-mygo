# FEBuilderGBA 原生编辑器

使用 Go + MyGo 重建的 Fire Emblem GBA ROM 编辑器，首阶段面向 macOS。支持 FE8U、FE8J、FE7U、FE7J 和 FE6；日文适配面向原版 ROM。

目前可以编辑人物、职业、道具、章节配置、文本、头像及道具图标，以及地图绘制与尺寸、部署、FE8 移动路径、部分条件事件、常用事件脚本、商店货物、战斗／死亡对话、支援关系与现有支援对话、职业移动消耗与地形加成。编辑共用撤销重做、保存、另存为和备份。

完整重建仍在进行：图块素材与地形定义、更多图像与音频、补丁管理、重建工具和完整旧版工作流尚未覆盖。逐项状态与尚待盘点的范围见[覆盖矩阵](docs/coverage.md)。

## 快速运行

安装 Nix 和 devenv 后，在仓库根目录进入固定的开发环境并启动：

```sh
devenv shell
just run /path/to/FE8U.gba
```

省略路径时，可通过“文件 → 打开 ROM…”或 Cmd+O 打开文件。每个 ROM 使用独立窗口。基础编辑不需要旧版 C# 运行环境或模拟器。

在该环境中，也可以执行：

```sh
just dev -lang zh-Hans
just build
```

无需进入交互 shell 时，可直接运行 `devenv shell just run /path/to/FE8U.gba`。环境和工具链约定见[开发说明](docs/development.md#环境与命令)。

开发模式会记住成功打开的 ROM 路径，无显式参数时在重启后恢复；未保存的内存修改不会自动写入 ROM。详情见[开发说明](docs/development.md#开发启动)。

## 使用边界

- FE8、FE7 至少需要 16 MiB，FE6 至少需要 8 MiB，文件上限为 32 MiB。
- 识别版本标头不代表支持改版的全部布局；翻译版自定义字库和补丁扩展按具体能力判断。
- 普通数值修改进入内存文档；文本、素材和事件草稿需预览并应用。Cmd+S 才保存到磁盘。
- 资源替换通常追加数据并保留旧资源，连续替换会增长 ROM。
- 本地 macOS 构建尚未发行公证。

## 文档

- [使用指南](docs/user-guide.md)：编辑、保存、草稿、资源替换和模拟器配置。
- [覆盖矩阵](docs/coverage.md)：已实现范围、缺口和验证证据。
- [开发说明](docs/development.md)与 [AGENTS](AGENTS.md)：开发命令及修改约束。
- [文档索引](docs/README.md)：架构、格式参考、设计和历史记录。

完整产品目标见[重建计划](docs/rebuild-plan.md)，所有日常命令见 `justfile`。
