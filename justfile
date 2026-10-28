# 命令在 justfile 所在目录执行；参数通过 "$@" 保留原始边界。
set positional-arguments

# 列出可用命令。
default:
    @just --list

# 启动编辑器，可传入一个或多个 ROM 路径。
run *args:
    go run . "$@"

# 开发模式，代码或翻译修改后自动重编译；例如 just dev -lang zh-Hans。
dev *args:
    go tool mygo dev "$@"

# 生成应用和安装包，输出位置由 mygo.json 决定。
build *args:
    go tool mygo build "$@"

# 重新生成类型化翻译 API。
generate:
    go tool mygo generate

# 检查翻译目录。
i18n-check:
    go tool mygo i18n check

# 按 .golangci.yml 格式化 Go 代码。
fmt:
    golangci-lint fmt ./... && oxfmt . "!FEBuilderGBA/**"

# 运行项目配置的静态检查。
lint:
    golangci-lint run ./... --fix

# MyGo 检查包含 go vet ./... 和原生 UI 生命周期检查。
vet:
    go vet ./... && go tool mygo vet .

# 运行所有默认测试；附加参数传给 go test，例如 just test -count=1。
test *args:
    go test "$@" ./...

# 只运行文档、游戏会话、空间和图像核心测试。
test-core *args:
    go test "$@" ./rom ./game ./space ./gba

# 检查文档、游戏会话和空间操作的数据竞争。
test-race *args:
    go test -race "$@" ./rom ./game ./space

# 动态发现 .rom/ 中的 ROM，使用临时副本验证通用流程；禁用缓存。
test-integration *args:
    go test -tags=integration -count=1 -v "$@" ./tests/integration

# 验证默认构建和无 CGO 构建，不生成安装包。
check-build:
    go build ./...
    CGO_ENABLED=0 go build ./...

# 执行常规检查；集成与 race 测试按需单独运行。
check: lint vet i18n-check test check-build

# 生成旧版来源索引，默认写入仓库根目录的 .cache/。
inventory *args:
    go run ./cmd/legacy-inventory "$@"

# 启动原生控件原型。
ui-probe:
    go run ./cmd/ui-probe
