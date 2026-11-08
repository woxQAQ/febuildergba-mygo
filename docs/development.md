# 开发说明

环境、启动与复核操作见本文。验证选择和停止条件由 [AGENTS](../AGENTS.md#命令与验证)规定；recipe 的实现、参数和包范围查 [justfile](../justfile)，用 `just --list` 浏览命令。

## 环境与命令

Nix 输入由 `devenv.nix`、`devenv.yaml` 与 `devenv.lock` 管理，Go 和 MyGo 版本要求查 `go.mod`。安装 Nix 和 devenv 后，在仓库根目录启动：

```nu
devenv shell
just dev
```

也可直接执行 `devenv shell just dev`。使用 direnv 时，首次执行 `direnv allow`，之后进入仓库自动加载 `.envrc`。

Linux 的开发环境设置 `LD_LIBRARY_PATH`，供 MyGo 的 purego 后端加载 GTK 3、GLib、Pango、Cairo 和 OpenGL；NixOS 图形驱动使用系统 `/run/opengl-driver/lib`。`XDG_DATA_DIRS` 前置 GTK 3 和桌面 GSettings schema，同时保留已有目录，供原生文件选择器读取设置。运行应用需要图形桌面会话。macOS 使用系统原生框架，不设置这些 Linux 路径。

工具链要求和 `GOTOOLCHAIN=local` 约定见 [AGENTS](../AGENTS.md#命令与验证)。升级 `go.mod` 后，通过 Nix 更新工具链，并重启既有终端、语言服务器和开发进程。排查版本时可执行：

```nu
with-env { GOTOOLCHAIN: local } { go version }
golangci-lint version
just --version
oxfmt --version
```

MyGo CLI 通过 `go tool mygo` 使用锁定依赖。新版运行不依赖旧 Windows/.NET 环境；需要查阅旧源码或生成来源索引时，先初始化固定子模块：

```nu
git submodule update --init FEBuilderGBA
```

`just lint` 带自动修复，`just fmt` 会格式化仓库并排除旧版子模块。已有其他修改时，局部使用 `golangci-lint fmt` 处理所改包、`oxfmt` 处理所改文档，并检查最终 diff。

静态检查也覆盖测试与合成样本构造。遇到 gosec 的整数转换等诊断时，优先使用匹配类型、范围校验或原生位字段编码；静态分析不能表达的格式语义才用局部 `#nosec`，附可核对的依据。删除抑制前，确认去除注释后的实际诊断，常规 lint 通过不能证明抑制仍有必要。

## 依赖边界验证

修改 `.golangci.yml` 的 depguard 规则时，在隔离副本验证当前规则，避免探针污染业务源码：

1. 为受影响边界构造允许和禁止的导入，覆盖包根、子包，以及普通源码和测试文件。
2. 先确认允许探针通过，再确认禁止探针产生目标 depguard 诊断；解析、编译或其他规则失败不能充当禁止导入的证据。
3. 删除隔离副本，检查工作区只保留所需规则变更，报告覆盖的边界与结果。

依赖方向及自动检查不能证明的约束见[架构](architecture.md#约束)。

## 原生界面复核

UI 复核的触发条件、临时性和停止规则见 [AGENTS](../AGENTS.md#命令与验证)。需要复现时：

1. 限定受影响流程与待核对结果，再按 `go.mod` 中的版本阅读 MyGo API；维护实践见 [mygo-maintenance](../.agents/skills/mygo-maintenance/SKILL.md)，项目约束优先。
2. 使用临时入口或临时 `ui.NewTester` 代码。真实 ROM 只操作副本；原生启动使用 `MYGO_ENV=production` 或隔离用户缓存，保护[开发恢复记录](#开发启动)。
3. 复现具体交互，检查目标控件、输入值和相关草稿。按改动涉及的维度检查键盘、焦点、窄窗口、缩放、语言或主题；仅 `TextInputActive()` 或找到同名元素不足以证明目标控件获得焦点。
4. 移除临时代码和入口，核对 diff。报告当次输入、操作与观察，永久回归保障由领域工作流测试承担。

线程与历史路由的实现入口见[界面与线程](architecture.md#界面与线程)，通用视觉和交互规则见 [DESIGN](DESIGN.md)。

## 开发启动

开发模式会记住成功打开的 ROM 路径。没有显式路径参数时，热重编译或重启后自动恢复；显式参数优先，另存为后更新记录。

`internal/app/dev_session.go` 将路径原子写入 `os.UserCacheDir()/febuildergba-mygo/dev-session.json`。生产构建及 `MYGO_ENV=production` 不读写它。恢复仅涉及路径，未保存的内存修改不会自动写入 ROM。

## 日志与诊断

应用同时向标准错误流（文本）、磁盘（JSON Lines）与内存写日志。默认文件为 `os.UserCacheDir()/febuildergba-mygo/logs/febuildergba.jsonl`，与开发恢复记录共享父级缓存目录。

| 环境变量              | 取值与默认行为                                                                          |
| --------------------- | --------------------------------------------------------------------------------------- |
| `FEBUILDER_LOG_LEVEL` | `debug`、`info`、`warn`、`error`；未设置或非法时，开发模式用 `debug`，生产构建用 `info` |
| `FEBUILDER_LOG_DIR`   | 日志目录；未设置时使用上述缓存路径                                                      |

文件在下一条记录将超过 1 MiB 时轮转，保留当前文件和 `.1`、`.2`、`.3`，`.1` 最新；重启继续已有文件。超限的单条记录完整保存，下一条再轮转，以保留错误原因树。新建目录和文件权限分别为 `0700` 和 `0600`。

缓存路径、目录、文件或轮转失败时，应用继续运行，向标准错误流和内存记录一次警告，本次会话停用失败的文件输出。内存默认保留最近 200 条通过级别过滤的记录，文件降级警告始终保留。

[应用日志窗口](user-guide.md#应用日志)提供近期记录的筛选、阅读和 JSONL 复制。日志包含路径、版本、大小、修订与结构化原因，不包含完整 ROM、PNG 或正文草稿。错误分级、去重和字段见[错误设计](error-design.md#日志采集)，缓冲、输出及线程边界见[架构](architecture.md#应用诊断)。

## 国际化

翻译目录和源语言由 `mygo.json` 指定。静态文案使用生成的类型化函数；`internal/locales/labels.go` 映射动态字段 ID，`internal/components/errtext.go` 映射结构化错误码。

语言匹配、回退、复数和原生控件翻译由 MyGo 处理。界面语言与游戏编码、字段 ID、ROM 输出独立；旧编辑器 UI 文案不迁移。

修改翻译后按 [AGENTS](../AGENTS.md#命令与验证)执行 `just i18n`。普通构建读取已纳入版本管理的 `catalog_gen.go`，MyGo 的 dev 和 build 会重新生成。

临时预览不同语言可使用：

```nu
with-env { MYGO_LOCALE: "zh-Hans" } { just run }
with-env { MYGO_LOCALE: "en" } { just run }
```

该环境变量不改写已保存的语言偏好。

## 图标

界面图标来自 [@iconify-json](https://iconify.design/) 图标集：`package.json` 在 `devDependencies` 中声明需要的集（如 `@iconify-json/feather`），`just icons` 安装依赖并由 `cmd/iconify-gen` 把每个集生成为 `internal/components/icons/<集>_gen.go`：集是一个命名空间变量，图标是其 `ui.SVG` 字段，按 `icons.<Set>.<Icon>` 引用（kebab-case 转 CamelCase）。生成文件与 `pnpm-lock.yaml` 一起提交，`node_modules/` 不入库。

生成器以集合级 24×24 网格为默认值并保留每个图标的 viewBox 覆盖；使用 rotate／flip 变换或别名（alias）的条目会报错而不是静默渲染错误，`hidden` 条目跳过。接入需要这些能力的集合时，先扩展生成器。

## 打包

使用 `just build`；平台、应用名称和产物位置查 `mygo.json`，附加参数查 MyGo CLI 帮助。平台与分发状态维护在[覆盖矩阵](coverage.md#待展开的旧版功能组)。
