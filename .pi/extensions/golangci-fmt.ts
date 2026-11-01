import { type ExtensionAPI, withFileMutationQueue } from "@earendil-works/pi-coding-agent";
import { realpath } from "node:fs/promises";
import { homedir } from "node:os";
import { extname, isAbsolute, relative, resolve, sep } from "node:path";

// gpt6 series models like to write codes without poor format, so write this extension to hook
// the edit tool and format the result.
export default function (pi: ExtensionAPI) {
  pi.on("tool_result", async (event, ctx) => {
    if (event.toolName !== "edit" || event.isError || typeof event.input.path !== "string") {
      return;
    }

    const inputPath = event.input.path.replace(/^@/, "");
    if (extname(inputPath) !== ".go") return;

    let message: string;
    try {
      const expandedPath = inputPath.startsWith("~/")
        ? resolve(homedir(), inputPath.slice(2))
        : inputPath;
      const filePath = await realpath(resolve(ctx.cwd, expandedPath));
      const projectPath = relative(await realpath(ctx.cwd), filePath);
      const firstPart = projectPath.split(sep)[0];
      if (isAbsolute(projectPath) || firstPart === ".." || firstPart === "FEBuilderGBA") {
        return;
      }

      const result = await withFileMutationQueue(filePath, () =>
        pi.exec("env", ["GOTOOLCHAIN=local", "golangci-lint", "fmt", "--", filePath], {
          cwd: ctx.cwd,
          timeout: 30_000,
          signal: ctx.signal,
        }),
      );
      const output = [result.stderr, result.stdout].filter(Boolean).join("\n").trim();
      if (result.killed || result.code !== 0) {
        const reason = result.killed ? "超时或已取消" : `退出码 ${result.code}`;
        throw new Error(`${reason}${output ? `\n${output.slice(0, 4000)}` : ""}`);
      }
      // fmt can report formatter failures as warnings while exiting successfully.
      message = output
        ? `编辑已完成；golangci-lint fmt 返回以下输出，格式化结果需检查：\n${output.slice(0, 4000)}`
        : `golangci-lint fmt 已格式化 ${event.input.path}；再次编辑前请读取最新内容。`;
    } catch (error) {
      const reason = error instanceof Error ? error.message : String(error);
      message = `编辑已完成，但 golangci-lint fmt 失败：${reason}`;
    }

    return {
      content: [...event.content, { type: "text", text: message }],
    };
  });
}
