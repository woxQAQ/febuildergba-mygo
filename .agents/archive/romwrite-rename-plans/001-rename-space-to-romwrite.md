# Plan 001：将 ROM 写入规划包统一命名为 romwrite

> 已归档：本计划已于 2026-11-08 实施并验证，对应提交 `b019568`，随后按用户要求合并入 `main`。以下保留编写时的执行要求与基线，不再作为活动计划；归档说明与当前文档入口见[归档索引](README.md)。

> **执行说明**：按步骤完成并运行验证，只修改范围内文件。遇到 STOP 条件时报告，不自行扩大范围。本计划由审查者派发，执行代理不要更新 `plans/README.md`；状态由审查者维护。
>
> **漂移检查**：先运行 `git diff --stat 0650147..HEAD -- space romwrite game rom internal tests .golangci.yml justfile docs/architecture.md docs/error-design.md docs/reference/rom-editing.md docs/reference/chapters-and-events.md docs/reference/text-and-images.md`。预期没有输出；若有输出，对照当前状态摘录，任何职责或接口差异先报告。

## 状态

- **优先级**：P3
- **工作量**：M
- **风险**：MED，目录迁移必须同步静态检查范围和测试命令。
- **依赖**：无
- **类别**：tech-debt
- **计划基线**：commit `0650147`，2026-11-08
- **提案状态**：用户已同意实施；通过独立 worktree 执行并审查，不合并到用户分支。
- **触发条件**：当前包名已不足以表达其写入规划职责；本次只做命名迁移。

## 为什么调整

`space` 当前组合追加、替换、搬迁、指针更新、原地覆盖和资源声明检查，职责已经超出空间分配。改成 `romwrite` 后，`romwrite.Builder`、`romwrite.WritePlan` 和 `romwrite.Protection` 可以直接表达 ROM 写入相关机制。`CheckWrite` 只检查四字节指针站点，应改为 `CheckPointerWrite`，与 `CheckRangeWrite` 对应。

`game/edit` 继续承接来源绑定、不可变提交和过期校验；`rom.Document` 继续拥有原子应用及撤销历史。此次不改变这三个包的职责或行为。

## 当前状态与约定

- `space/doc.go:1` 声明 `Package space plans GBA ROM resource writes and relocations without changing the source bytes`。包内有 `doc.go`、`claims.go`、`write.go`、`plan_builder.go` 和四个工作流测试文件，文件名全部保留。
- `space/claims.go:28` 的接口及稳定错误标识如下：

  ```go
  var ErrClaim = fault.Code{
      ID:   "space.claim_conflict",
      Kind: fault.Conflict,
  }

  // CheckWrite verifies that owner may rewrite the pointer site [site, site+4)
  func CheckWrite(claims []Claim, owner Owner, site int) error {
      siteRange := Range{Start: site, End: site + 4}
      // 原有归属校验保留。
  }
  ```

- `space/plan_builder.go` 提供 `Builder`、`Protection`、`PointerSite`；`space/write.go` 提供 `WritePlan` 及搬迁和分配实现。不要借改名拆合文件。
- `game/maps/map_plan.go:70` 是调用约定的例子：

  ```go
  builder := space.NewBuilder(s.image.Data, space.Protection{
      Pointer: func(_ []byte, _ space.Owner, site int) error {
          return s.mapWriteSite(format, site, LayoutField)
      },
  })
  // Replace 后取得 plan。
  commit: edit.NewCommit(draft.binding, LayoutField, plan.Edit)
  ```

  迁移只把 Go 包名和选择器换成 `romwrite`，继续使用默认包名导入，不引入 `space` 别名或兼容转发层。
- 目前有 106 个 Go 文件导入旧路径，包含 59 个生产文件和 47 个测试文件。导入分布在 `space`、`game` 和 `tests/integration`。
- `.golangci.yml:30,37` 的两组匹配为 `${config-path}/{game,rom,space,gba,fault}/**.go`，分别防止领域包导入应用日志和 MyGo/UI；`:101` 的 locales 规则禁止导入旧 `space` 路径。必须替换成新路径并保留匹配语义。
- `justfile` 的 `test-core`、`test-race` 显式列出 `./space`。`check` 依次包含 lint、vet、翻译检查、测试和两种构建；其中 lint 会执行自动修复，因此只在执行 worktree 运行。
- `docs/error-design.md` 明确“操作名是包内的稳定标识……独立于翻译 key 和函数名”。必须保留所有既有 `space.*` 错误码 ID **和操作名字面量**，包括 `space.check_write`。Go 包改名不要求这些字符串同步改名。
- 项目约定：旧版 `FEBuilderGBA/` 只读；领域核心独立于 MyGo；错误使用稳定 `fault.Code` 并保留原因链；内部包无需兼容别名；不逐函数新增单测。Nix 提供工具，直接运行 Go 必须设置 `GOTOOLCHAIN=local`，不安装或下载工具链。
- `docs/development.md` 要求修改 depguard 后在隔离副本验证允许和禁止导入，覆盖包根、子包、普通源码与测试文件；目标 depguard 诊断才算禁止探针成功。

## 所需命令

以下是执行代理内部使用的 shell 命令；所有命令在独立执行 worktree 根目录运行。

| 用途 | 命令 | 成功预期 |
| --- | --- | --- |
| 基线测试 | `GOTOOLCHAIN=local go test ./space ./game/edit` | 两包通过；原工作区审查时已通过（缓存） |
| 迁移工作流 | `just test-core -count=1` | 新 `romwrite` 与 ROM、游戏领域测试全部通过 |
| 常规检查 | `just check` | exit 0；不得保留无关自动修复 |
| 迁移后 race | `just test-race -count=1` | recipe 范围内全部通过，无 race 报告 |
| 边界探针 | `golangci-lint run --enable-only depguard ./romwrite/... ./internal/locales/...` | 允许探针 exit 0；禁止探针必须产生对应文件和目标边界的 depguard 诊断 |
| 补丁空白 | `git diff --check 0650147` | exit 0，无输出 |
| 结果范围 | `git diff --name-status -M 0650147` | 只有以下范围内的命名、路径和文档变更 |

## 修改范围

**允许持久修改**：

1. 将 `space/` 完整迁移为 `romwrite/`，保留文件名与测试用例，调整 `package space` / `space_test`、包注释、示例和接口名称。
2. 在仓库自有 Go 文件中，只修改导入旧包的路径、对应 `space.` 选择器以及 `CheckWrite` 调用；包括 `game/edit/commit_workflow_test.go`。若其他目录中确有这些引用，也按这一窄规则更新。
3. `.golangci.yml`、`justfile` 中直接指向旧包的规则和包范围。
4. 当前文档中的包名、接口名和路径：`docs/architecture.md`、`docs/error-design.md`、`docs/reference/rom-editing.md`、`docs/reference/chapters-and-events.md`、`docs/reference/text-and-images.md`、`tests/README.md`。架构图节点名及模块职责标签可同步为 ROM 写入规划。
5. 临时边界探针仅存在于执行 worktree 或另外的隔离副本中，最终删除，不提交。

**范围外**：

- `game/edit` 的包名、生产源码及职责；`rom.Document` 的行为；任何编码、分配、归属或过期校验算法。
- 所有 `fault.Code.ID`、`fault.Error.Op` 对应的操作名字面量，以及其他数据字符串、固定测试输入和期望。
- `FEBuilderGBA/`、`.agents/archive/` 历史归档、技能文件、依赖与锁文件、翻译文件和生成代码。
- 新增兼容别名、旧包转发层、UI 测试、功能开发、无关整理。

## Git 工作流

- 使用已由审查者准备的独立分支 `advisor/001-romwrite`，基于 `0650147`。
- 可在该 worktree 提交一份迁移提交，遵循现有简洁祈使句风格，例如 `rename space package to romwrite`。
- 不修改、合并、cherry-pick、提交到用户主工作区；不 push 或发布 PR/issue。

## 执行步骤

### 1. 核对基线及引用

运行漂移检查、`git status --short` 和基线测试。检索旧 import path、`package space`、`space.` 及 `CheckWrite`，区分代码选择器、当前文档、稳定错误字符串和历史归档。确认没有现存 `romwrite` 包冲突。

**验证**：漂移和 status 没有输出；`GOTOOLCHAIN=local go test ./space ./game/edit` exit 0；新目录无冲突。

### 2. 原子完成代码、工具与当前文档的命名迁移

移动目录，统一新包名和 import path；将定义、所有调用及说明中的 `CheckWrite` 改为 `CheckPointerWrite`。按上节窄范围更新规则和命令，保留全部既有错误码与操作名字符串。不改其他 exported symbol，不拆合文件，不调整算法和测试断言。

当前文档同步包名、接口名、代码路径和职责标签。保留描述空间分配的自然语言及功能 ID，历史归档冻结。不要为消除文本搜索结果而更改真实错误标识。

**验证**：`just test-core -count=1` exit 0。`rg -n 'CheckWrite|package space|febuildergba-mygo/space|\./space' game rom romwrite internal tests .golangci.yml justfile` 无匹配（rg exit 1）。`rg -n '\bspace\.' --glob '*.go' game romwrite tests` 的残余只允许是既有错误码或操作名字面量。`git diff --check 0650147` exit 0。

### 3. 在隔离目录验证迁移后的 depguard

使用执行 worktree 或另外的隔离副本，构造临时探针，覆盖以下四种位置：包根普通 `.go`、包根 `_test.go`、子包普通 `.go`、子包 `_test.go`。允许探针先用标准库或 `fault` 导入，确认上表边界命令通过。

对 `romwrite` 的四种位置分别注入 MyGo（例如 `github.com/egoist/mygo/ui`）及 `internal/applog` 的禁止导入；对 `internal/locales` 的四种位置分别注入 `romwrite` 禁止导入。可批量运行，但必须逐文件确认三类边界均产生目标 depguard 诊断，覆盖 12 个位置/边界组合。编译失败或其他 linter 失败不算成功。必要时关闭 lint issue 去重/数量限制，以取得全部证据。

删除全部临时文件和目录。

**验证**：允许探针 exit 0；禁止探针覆盖 12 个组合并均有目标 depguard 证据；清理后同一边界命令 exit 0，`git status --short` 无探针残留。

### 4. 完成整体验证并提交执行结果

运行 `just check` 和 `just test-race -count=1`。这里 race 用于确认更新后的 recipe 仍实际包含 `romwrite`，不代表 UI 线程验证。检查自动修复没有引入范围外内容；与基线比较，所有 Go 语义变化都必须归因于包/符号改名。

确认目录迁移完整且没有兼容层，当前文档没有旧包引用（稳定错误标识除外），提交该独立分支。不要修改计划索引。

**验证**：所有命令 exit 0；`git diff --check 0650147` 无输出；`git diff --name-status -M 0650147` 范围合规；提交后 `git status --short` 为空。报告提交 SHA 和实际验证结果。

## 测试计划

本次命名迁移不新增行为测试，也不改变现有测试数据和断言。继续使用原 `space` 资源写入工作流和 `game/edit` 捕获提交工作流，覆盖失败原子性、旧字节与独立引用保留、预览隔离、文档/修订/布局失效及撤销重做。额外的 depguard 探针只验证目录迁移没有削弱静态边界，复核后移除。

不启动 UI、模拟器或真实 ROM 集成流程，因为本次不改变界面和领域行为。

## 完成条件

- [ ] `space/` 不存在，`romwrite/` 完整承接原文件，无兼容转发。
- [ ] 定义与调用统一为 `CheckPointerWrite`；旧 Go 包名、路径、选择器与旧函数引用已清理。
- [ ] 稳定错误 ID、操作名字面量、测试样本与断言保持原值。
- [ ] `game/edit` 保留包名及生产实现，只更新其测试对旧包的引用。
- [ ] `.golangci.yml`、两个核心/race recipe 和当前文档全部同步。
- [ ] depguard 允许探针通过，12 个禁止位置/边界组合均得到对应诊断，临时探针全部移除。
- [ ] `just test-core -count=1`、`just check`、`just test-race -count=1` 全部 exit 0。
- [ ] `git diff --check 0650147` exit 0，全部持久变更符合范围。
- [ ] 在执行 worktree 提交并保持干净，由审查者独立复核和维护计划状态。

## STOP 条件

- 基线发生职责或接口漂移，或现存新包造成命名冲突。
- 需要修改算法、历史行为、错误标识、测试断言、依赖、冻结归档或 `game/edit` 生产代码才能通过检查。
- 同一验证在合理修正后仍失败两次；或工具不足且无法使用既有 Nix 环境。
- depguard 不能产生目标诊断，不能用其他失败冒充边界验证。
- 发现范围外文件需要持久修改；先报告位置和原因。

## 维护提示

后续领域继续向 `romwrite` 提供明确声明与保护规则，领域含义和编码留在所属包。审查重点是命名替换完整性、稳定错误字符串保留、默认包名导入以及 depguard 与 recipe 的覆盖范围。本计划不触发新框架、资源回收或包职责重组。
