# 前端设计更新说明

更新日期：2026-10-26

## 设计目标

为 FEBuilderGBA 原生编辑器重新设计视觉系统，使其：
1. 体现 Fire Emblem 系列的中世纪奇幻与战术 RPG 美学
2. 服务于 ROM 编辑工具的专业性和数据密度需求
3. 避免 AI 生成界面的通用模板痕迹

## 色彩系统变更

### 之前
```go
t.Accent = ui.Hex("#356887")  // 钢蓝色（无特定主题）
```

### 现在
```go
t.Accent = ui.Hex("#C44B2C")     // 焦赭红 - Fire Emblem 标志性的火焰/纹章红
t.Success = ui.Hex("#4A7C59")    // 森林绿 - 成长率和升级系统
t.Warning = ui.Hex("#D4941E")    // 金色 - 稀有道具和关键数据
t.Danger = ui.Hex("#B3332D")     // 深红 - 错误和危险操作
t.TextMuted = ui.Hex("#6B6B6B")  // 中性灰 - 次要信息
```

## 设计理念

### 色彩来源
- **焦赭红 (#C44B2C)**。取自 Fire Emblem 系列的火焰与纹章符号，介于中世纪设定（锈蚀的铁器）和奇幻元素（火魔法、纹章图案）之间

- **森林绿 (#4A7C59)**。用于成长率、属性增长和角色培养相关的状态

- **金色 (#D4941E)**。取自传说武器和稀有道具

- **深红 (#B3332D)**。比强调色更深的红色，专用于错误提示和破坏性操作

### 布局保持
- 保留现有的数据密度和紧凑布局（3px spacing, 4px radius）
- 保持三区结构：左侧列表（330px）+ 右侧编辑区 + 底部字段检查器
- 继续使用等宽数字（`tnum`）和 monospace 字体确保数据对齐

### 避免的设计模式
- 通用 AI 默认：奶油背景、蓝绿渐变、Anthropic 陶土色
- SaaS 卡片套件：统一圆角卡片、柔和阴影、玻璃态
- 模板化排版：全大写标签、装饰性编号、单词强调色

## 技术实现

主题设置集中在 `internal/components/chrome.go` 的 `ApplyTheme`：

```go
func ApplyTheme(c *ui.Context) {
    t := *c.Theme()
    t.Spacing, t.Radius = 3, 4
    // Fire Emblem-themed color system
    t.Accent = ui.Hex("#C44B2C")
    t.Success = ui.Hex("#4A7C59")
    t.Warning = ui.Hex("#D4941E")
    t.Danger = ui.Hex("#B3332D")
    t.TextMuted = ui.Hex("#6B6B6B")
    c.SetTheme(&t)
    // ...
}
```

## 验证方式

构建并运行应用查看效果：

```bash
go build -o build/febuilder .
./build/febuilder

# 或直接运行
go run .
```

重点检查：
1. 当前标签页的强调色
2. 选中记录时的高亮色
3. 字段编辑器的聚焦状态
4. 错误提示的显示颜色
5. 次要信息（文件大小、记录数）的灰色

## 设计文档

完整设计系统文档见 `DESIGN.md`，包含：
- 设计简报和目标受众分析
- 色彩选择的详细理论依据
- 排版层级和字体特性使用
- 布局原则和视觉层级
- 设计规避的通用模板模式
