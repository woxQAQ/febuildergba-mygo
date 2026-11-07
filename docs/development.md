# 开发说明

命令在仓库根目录执行，由 [justfile](../justfile) 维护；执行 `just --list` 查看每个 recipe 的用途。检查选择和停止条件见 [AGENTS](../AGENTS.md#命令与验证)。

## 环境与命令

仓库提供 `devenv.nix` 和 `devenv.yaml`，通过 `devenv.lock` 固定 Nix 输入。安装 Nix 和 devenv 后，在仓库根目录进入开发环境：

```sh
devenv shell
just dev
```

也可直接运行 `devenv shell just dev`。使用 direnv 时，首次执行 `direnv allow`，之后进入仓库自动加载 `.envrc`。

环境提供 Go 1.27、gopls、just、golangci-lint 和 oxfmt，并设置 `GOTOOLCHAIN=local`。Linux 下额外设置 `LD_LIBRARY_PATH`，供 MyGo 的 purego 后端按库名加载 GTK 3、GLib、Pango、Cairo 和 OpenGL 等原生依赖；NixOS 图形驱动通过系统的 `/run/opengl-driver/lib` 使用。环境还向 `XDG_DATA_DIRS` 前置 GTK 3 和桌面 GSettings schema 路径，保留已有数据目录，供原生文件选择器读取设置定义。运行编辑器仍需要可用的图形桌面会话。macOS 不设置这些 Linux 专用路径，使用系统原生框架。

`justfile` 为全部 recipe 及其子进程导出 `GOTOOLCHAIN=local`。直接运行 Go、语言服务器或其他 Go 工具时，按 [AGENTS 的工具链约束](../AGENTS.md#命令与验证)设置相同值。提升 `go.mod` 要求后，先通过 Nix 更新本地工具链；不足的本地版本会报错，不会自动下载补齐。既有终端、语言服务器和开发进程需要重新启动以使用更新后的工具。

可用以下命令核对当前工具：

```sh
GOTOOLCHAIN=local go version
golangci-lint version
just --version
oxfmt --version
```

MyGo 版本由 `go.mod` 锁定，recipes 通过 `go tool mygo` 调用。lint 与 Go 格式化使用 `golangci-lint`，文档格式化使用 `oxfmt`。不需要旧版 Windows/.NET 运行环境。生成旧版来源索引前，先初始化固定子模块：

```sh
git submodule update --init FEBuilderGBA
```

| 任务               | Recipe 入口与参数示例                                              |
| ------------------ | ------------------------------------------------------------------ |
| 启动并打开 ROM     | [run](../justfile)：`just run /path/to/FE8U.gba`                   |
| 开发模式与热重编译 | [dev](../justfile)：`just dev -lang zh-Hans`                       |
| 常规检查           | [check](../justfile)                                               |
| 运行相关测试       | [test](../justfile)：`just test -run TestName -count=1`            |
| 可选 ROM 集成      | [test-integration](../justfile)                                    |
| 数据竞争检查       | [test-race](../justfile)；范围见 [AGENTS](../AGENTS.md#命令与验证) |
| 格式化             | [fmt](../justfile)                                                 |
| 更新翻译           | [i18n](../justfile)                                                |
| 生成旧版来源索引   | [inventory](../justfile)                                           |
| 原生控件原型       | [ui-probe](../justfile)                                            |
| 构建应用和安装镜像 | [build](../justfile)                                               |

[lint recipe](../justfile) 的 depguard 按 `.golangci.yml` 检查[导入边界](architecture.md#依赖方向)。自动检查与人工审查的边界见[架构约束](architecture.md#约束)。gosec 同时检查生产代码、测试和合成样本构造，包括整数转换的 G115 规则。优先用匹配的类型、明确范围校验或原生位字段编码表达约束；静态分析无法证明的格式语义使用局部 `#nosec`，注明规则与可核对的依据。清理抑制时须确认去除注释后的实际诊断，不能由常规 lint 通过推断注释仍有必要。

[lint](../justfile) 使用项目配置的自动修复；[fmt](../justfile) 格式化仓库并排除旧版子模块。局部格式化时，可向 `golangci-lint fmt` 传入所改包、向 `oxfmt` 传入所改文档，以遵循[保留工作区修改的约定](../AGENTS.md#开始修改)。

## 开发启动

开发模式（[dev recipe](../justfile) 或未打包的 `GOTOOLCHAIN=local go run .`）会记住成功打开的 ROM 路径。没有显式路径参数时，重编译或重启后自动恢复这些文件；显式参数优先，另存为后更新记录。

记录由 `internal/app/dev_session.go` 原子写入 `os.UserCacheDir()/febuildergba-mygo/dev-session.json`，macOS 通常位于用户 Library 的 Caches 目录。生产构建及 `MYGO_ENV=production` 不读写此记录。恢复仅涉及文件路径，未保存的内存修改不会自动写入 ROM。

## 日志与诊断

应用启动后同时向标准错误流（文本）、磁盘文件（JSON Lines）与内存缓冲写入日志。默认路径为 `os.UserCacheDir()/febuildergba-mygo/logs/febuildergba.jsonl`；与开发恢复的 `dev-session.json` 共享父级缓存目录。环境变量控制配置，不新增 CLI 参数：

| 环境变量              | 取值与默认行为                                                                          |
| --------------------- | --------------------------------------------------------------------------------------- |
| `FEBUILDER_LOG_LEVEL` | `debug`、`info`、`warn`、`error`；未设置或非法时，开发模式为 `debug`，生产构建为 `info` |
| `FEBUILDER_LOG_DIR`   | 日志目录路径；未设置时使用上述缓存路径                                                  |

文件默认在下一条记录将超过 1 MiB 时轮转，保留当前文件与 `.1`、`.2`、`.3` 三个历史世代，`.1` 最新；重新启动会继续已有文件并按当前大小轮转。单条记录超过上限时完整保存，下一条记录再轮转，避免截断错误原因树。新建目录与文件权限分别为 `0700` 和 `0600`。

缓存路径解析、目录创建、文件打开或运行期写入／轮转失败时，应用继续运行，向标准错误流和内存记录一次警告；本次会话停用失败的文件输出。内存默认保留最近 200 条已通过级别过滤的结构化记录，文件降级警告始终保留。`Sink.Buffer.Snapshot()` 提供与内部数据隔离的只读快照，每条记录的内存序号按接收顺序递增，缓冲淘汰后不复用；`Entry.JSON()` 与磁盘输出使用同一结构化表示，内存序号不写入文件格式。从“查看 → 应用日志”或 Cmd/Ctrl+Shift+L 打开全应用共享的独立日志窗口，支持最低级别及全文筛选、暂停／继续、复制所选或筛选结果的 JSONL，以及清空当前视图。显示筛选不改变采集级别，暂停不停止采集，清空不删除日志缓冲或文件。查看器只读当前进程的近期记录，不读取轮转历史文件；功能状态见[覆盖矩阵](coverage.md)。

日志包含文件路径、识别版本、大小、修订与结构化错误原因，不包含完整 ROM、PNG 或正文草稿。错误分级、去重和事件字段见[错误设计](error-design.md#日志采集)，包和线程边界见[架构](architecture.md#应用诊断)。

## 国际化

`mygo.json` 将 `internal/locales` 配置为翻译目录：`en.json` 是源语言，`zh-Hans.json` 是简体中文。静态文案使用生成的类型化函数；`internal/locales/labels.go` 将动态字段 ID 映射到 catalog key，`internal/components/errtext.go` 将结构化错误码映射为文案。

语言匹配、回退、复数和原生控件翻译由 MyGo 处理。界面语言不影响游戏编码、字段 ID 或 ROM 输出；旧编辑器 UI 文案无需迁移。

翻译修改使用 [i18n recipe](../justfile)，生成文件约定见 [AGENTS](../AGENTS.md#命令与验证)。普通构建读取已纳入版本管理的 `catalog_gen.go`，无需预先生成；MyGo 的 dev 和 build 会重新生成。

临时预览语言可使用：

```sh
MYGO_LOCALE=zh-Hans just run
MYGO_LOCALE=en just run
```

该环境变量不会改写保存的语言偏好。

## 打包

使用 [build recipe](../justfile)，输出位置由 `mygo.json` 决定。当前 macOS arm64 应用可通过以下命令打开：

```sh
open build/darwin-arm64/FEBuilderGBA.app
```

当前产物使用本地签名，尚未发行公证。平台与分发的实现状态见[覆盖矩阵](coverage.md#待展开的旧版功能组)。
