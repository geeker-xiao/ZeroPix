# ZeroPix

一款面向 Windows 的桌面截图工具：框选即拍、就地标注、一键流转。
界面按取景器风格设计，支持浅色与深色主题，快捷键与工具条都可自定义。

[![Build](https://github.com/geeker-xiao/ZeroPix/actions/workflows/build.yml/badge.svg)](https://github.com/geeker-xiao/ZeroPix/actions/workflows/build.yml)
[![Release](https://img.shields.io/github/v/release/geeker-xiao/ZeroPix)](https://github.com/geeker-xiao/ZeroPix/releases)
[![Go](https://img.shields.io/badge/Go-1.24%2B-00ADD8)](https://go.dev)
[![License](https://img.shields.io/github/license/geeker-xiao/ZeroPix)](LICENSE)

## 功能

### 截图

- **区域截图**（默认 `F1`）：拖拽框选，松开后可拖动整体位置、拖八向手柄改尺寸；
  `Enter` 复制、`Ctrl+S` 保存、`Esc` 取消。
- **全屏截图**（默认 `F3`）：跳过框选，直接进入标注界面。
- **贴图**（默认 `F4`）：把剪贴板里的图片贴到屏幕上，可拖动、缩放、置顶。
- 多显示器与高 DPI 缩放会按各自的分辨率取样，副屏截图不会糊。

### 标注

截图完成后的工具条提供以下工具，全部为矢量绘制，撤销/重做可反复调整：

| 类别 | 工具 |
| --- | --- |
| 形状 | 矩形、椭圆、圆形、直线、箭头 |
| 手绘 | 画笔、荧光笔 |
| 遮挡 | 马赛克、模糊、聚光灯 |
| 文字与标记 | 文字、序号、尺寸标注、标尺 |
| 修正 | 橡皮擦、从截图中取色 |
| 历史 | 撤销（`Ctrl+Z`）、重做（`Ctrl+Shift+Z`） |

- **取色**：四个预设色加一个自定义色。自定义色点击后在工具条下方展开取色面板
  （饱和度/明度方块、色相条、HEX 显示），拖到哪个颜色就作用于当前标注，
  取到的颜色会沿用到下一次截图。
- **工具条**：固定两行布局，第二行整行留给颜色与当前 HEX；
  按钮组合可在设置中心按「个性极简 / 默认实用 / 火力全开」三套预设一键切换，
  也可以逐个勾选。

### 截图之后

- **复制 / 保存**：`Enter` 复制到剪贴板，`Ctrl+S` 保存为 PNG。
- **文字识别（OCR）**：内置离线识别模型，框住文字即可取回文本。
- **翻译**：接 OpenAI 兼容接口或内置服务，把识别结果翻成简体中文或英文。
- **智能脱敏**：识别文字中的敏感信息并自动打码。
- **GIF 截图 / 录屏**：把选中区域录成动图或视频。
- **钉图贴屏**：把结果固定在屏幕最上层，方便对照操作。

### 屏幕取色

默认 `F6` 唤出全屏取色，十字准星带放大镜，松开鼠标后弹出取色窗口，
展示 RGB、十进制与 HEX 三种写法并可一键复制；窗口内还能「重新拾取」。

### 设置中心

- 常规：开机自启、界面主题（跟随系统 / 深色 / 浅色）。
- 保存与识别：翻译服务、目标语言、模型地址与密钥。
- 热键：区域截图、全屏截图、贴图、颜色拾取四条快捷键均可录键修改。
- 工具条：三套预设与逐项开关。

配置保存在 `%APPDATA%\zeropix\settings.json`，旧版本（pixmint / zoo_pix）的配置会自动迁移。

## 安装

从 [Releases](https://github.com/geeker-xiao/ZeroPix/releases) 下载：

- `zeropix-setup-<版本>-amd64.exe`：安装包，带开始菜单与卸载项。
- `zeropix.exe`：单文件绿色版，放到任意目录双击即可运行。

系统要求：Windows 10 / Windows 11，64 位。

## 从源码构建

需要 Go 1.24 以上（`go.mod` 里使用了 `tool` 指令）以及一个 C 编译器，
OCR 运行时通过 cgo 绑定，因此 `CGO_ENABLED=1` 是必需的。

```bash
git clone https://github.com/geeker-xiao/ZeroPix.git
cd ZeroPix

# 只编译可执行文件
CGO_ENABLED=1 go build -o zeropix.exe .

# 打出可执行文件与安装包（输出到 build/windows-amd64/）
CGO_ENABLED=1 go tool mygo build -platform windows/amd64
```

Windows 上的 C 编译器可用 MSYS2 的 `mingw-w64-x86_64-gcc`：

```powershell
winget install MSYS2.MSYS2
# 之后把 C:\msys64\mingw64\bin 加入 PATH
gcc --version
```

检查与测试：

```bash
go vet ./...
go test -count=1 ./...
```

## 项目结构

```
main.go        启动、托盘、单实例
capture_*.go   屏幕取样（Windows / 其他平台）
snip.go        截图会话：框选、移动、缩放、输入分发
annotate.go    标注的几何与绘制
toolbar.go     截图后工具条、颜色行、自定义取色面板
picker.go      屏幕取色窗口
dialog.go      通用弹窗与品牌按钮
settings.go    设置中心与 settings.json
brand.go       色板与主题取值
ocr.go         离线文字识别
translate.go   翻译与智能脱敏
gifrec.go      GIF 与视频录制
pin.go         贴图窗口
icons.go       图标
resources/     应用与托盘图标（go:embed）
ocr/           识别模型与字典（go:embed）
```

## 规划

按优先级从近到远，具体版本号会随实际情况调整：

- **0.2**
  - 滚动截图：长网页、长列表一次拍完。
  - 标注再编辑：截图后可回退到标注界面继续修改，而不是只靠撤销。
  - 线条与箭头样式：虚线、粗细档位、端点样式。
  - 保存设置：默认保存目录、文件名模板、PNG / JPEG / WebP 选择。
- **0.3**
  - 多语言界面（简体中文 / English），跟随系统语言。
  - OCR 语言包可选，支持中英混排之外的语种。
  - 录屏编码选项：帧率、码率、是否带鼠标高亮。
  - 截图历史面板：最近若干张可回溯、重新复制或另存。
- **0.4**
  - 图床上传与自定义上传脚本，复制 Markdown / HTML 链接。
  - 插件化工具条：允许用脚本扩展标注动作。
  - 便携模式：配置与历史放在程序目录，适合 U 盘携带。
- **探索中**
  - macOS 与 Linux 版本（截图与窗口层需要按平台重写）。
  - 自动更新与增量升级。
  - 团队共享的脱敏规则库。

## 反馈

- 问题与建议：[Issues](https://github.com/geeker-xiao/ZeroPix/issues)
- 版本变更：[CHANGELOG.md](CHANGELOG.md)

## 许可

[MIT](LICENSE) © 2026 geeker-xiao

可自由使用、修改、分发与商用，保留版权声明与许可声明即可。构建时嵌入的
`ocr/` 资源另有来源与授权：PP-OCRv5 模型来自 PaddleOCR / PaddleX（Apache-2.0），
推理运行时为 onnxruntime（MIT），详见 [ocr/README.txt](ocr/README.txt)。
