# 开发说明

命令在仓库根目录执行，由 [justfile](../justfile) 维护；执行 `just` 查看全部说明。代码修改约束见 [AGENTS](../AGENTS.md)，测试数据与组织见[测试说明](../tests/README.md)。

## 环境与命令

使用 [go.mod](../go.mod) 指定的 Go 版本和 MyGo 工具。MyGo 通过 `go tool mygo` 调用，lint 与 Go 格式化需要 PATH 中的 `golangci-lint`，文档格式化使用 `oxfmt`。不需要旧版 Windows/.NET 运行环境。生成旧版来源索引前，先初始化固定子模块：

```sh
git submodule update --init FEBuilderGBA
```

| 任务                  | 命令                               |
| --------------------- | ---------------------------------- |
| 启动并打开 ROM        | `just run /path/to/FE8U.gba`       |
| 开发模式与热重编译    | `just dev -lang zh-Hans`           |
| 常规检查              | `just check`                       |
| 运行相关测试          | `just test -run TestName -count=1` |
| 数据竞争检查          | `just test-race`                   |
| 本地真实 ROM 集成检查 | `just test-integration`            |
| 格式化                | `just fmt`                         |
| 生成翻译 API          | `just generate`                    |
| 生成旧版来源索引      | `just inventory`                   |
| 原生控件原型          | `just ui-probe`                    |
| 构建应用和安装镜像    | `just build`                       |

`just check` 包含 lint、Go/MyGo vet、翻译检查、默认测试，以及默认和 `CGO_ENABLED=0` 两种构建检查。race 和真实 ROM 检查按需单独执行。集成命令禁用缓存，缺少本地 ROM 时由测试明确报告跳过。

`just lint` 使用项目配置的自动修复；`just fmt` 会格式化仓库并排除旧版子模块。执行后应检查 diff。仅格式化文档时可直接向 `oxfmt` 传入所改文件。

## 开发启动

开发模式（`just dev` 或未打包的 `go run .`）会记住成功打开的 ROM 路径。没有显式路径参数时，重编译或重启后自动恢复这些文件；显式参数优先，另存为后更新记录。

记录由 [dev_session.go](../internal/app/dev_session.go) 原子写入 `os.UserCacheDir()/febuildergba-mygo/dev-session.json`，macOS 通常位于用户 Library 的 Caches 目录。生产构建及 `MYGO_ENV=production` 不读写此记录。恢复仅涉及文件路径，未保存的内存修改不会自动写入 ROM。

## 国际化

[mygo.json](../mygo.json) 将 `internal/locales` 配置为翻译目录：`en.json` 是源语言，`zh-Hans.json` 是简体中文。静态文案使用生成的类型化函数；[labels.go](../internal/locales/labels.go) 将动态字段 ID 映射到 catalog key，[errtext.go](../internal/components/errtext.go) 将结构化错误码映射为文案。

语言匹配、回退、复数和原生控件翻译由 MyGo 处理。界面语言不影响游戏编码、字段 ID 或 ROM 输出；旧编辑器 UI 文案无需迁移。

修改 JSON 后执行 `just generate` 和 `just i18n-check`。`catalog_gen.go` 与 JSON 一起纳入版本管理，普通 `go build` 不需要预先生成；MyGo 的 dev 和 build 会重新生成。

临时预览语言可使用：

```sh
MYGO_LOCALE=zh-Hans go run .
MYGO_LOCALE=en go run .
```

该环境变量不会改写保存的语言偏好。

## 打包

执行 `just build`，输出位置由 `mygo.json` 决定。当前 macOS arm64 应用可通过以下命令打开：

```sh
open build/darwin-arm64/FEBuilderGBA.app
```

当前产物使用本地签名，尚未发行公证。平台与分发的实现状态见[覆盖矩阵](coverage.md#待展开的旧版功能组)。
