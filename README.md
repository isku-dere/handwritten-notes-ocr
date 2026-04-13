# Handwritten Notes OCR

[English](./README.en.md)

一个基于 `Go + PaddleOCR + 内嵌 Web UI` 的离线优先手写笔记 OCR 应用，支持可选的在线 OCR 回退链路，以及基于 Qwen 的学习笔记生成。

它的目标是将拍照得到的手写笔记转成可编辑的 Markdown，再进一步整理、纠错并汇总成结构化电子学习笔记。

## 功能特性

- 批量上传与批量 OCR 处理
- 并发 OCR 过程中的文件队列即时刷新
- 每张图片保留独立 Markdown 结果并支持手工编辑
- Markdown 源码 / 预览双模式
- OCR 前图片旋转
- 文件队列软移除 / 恢复
- 单图结果与学习笔记结果支持本地导出 Markdown
- 基于当前页面全部已识别 Markdown 生成学习笔记
- 在调用 LLM 前做短内容预检查，避免不必要的 token 消耗
- 前端静态资源嵌入 Go 二进制

## 技术栈

- 后端：Go
- OCR：
  - 本地：通过 Python 调用 PaddleOCR
  - 可选：在线 OCR 接口作为优先或回退链路
- LLM：通过 OpenAI 兼容接口调用 Qwen
- 前端：HTML、CSS、原生 JavaScript
- 通信：
  - REST 接口用于上传与控制
  - SSE 用于批量 OCR 进度与学习笔记流式输出

## 工作流程

1. 上传一张或多张手写笔记图片。
2. 前端可在提交前对图片进行旋转。
3. Go 后端并发处理 OCR 请求。
4. 每张图片都会生成一份独立的 Markdown，可单独查看和编辑。
5. 程序收集当前页面中所有未被移除的 Markdown 结果。
6. 在调用 LLM 前，后端先进行一次短内容预检查。
7. 如果内容足够，Qwen 会输出一版修正后的电子学习笔记 Markdown。

## 项目结构

```text
cmd/server/                 Go 程序入口
internal/app/               服务、路由、配置、运行时
internal/ocr/               OCR 客户端
internal/notes/             学习笔记提示词与预检查逻辑
internal/llm/               Qwen 客户端
internal/markdown/          OCR 结果转 Markdown
internal/assets/            嵌入资源与 OCR 辅助脚本
internal/assets/web/        前端界面
```

## 本地运行

### 环境要求

- Go `1.25+`
- Python `3.10+`
- 当前主要在 Windows 环境下验证

### 安装 Python 依赖

```bash
pip install -r requirements.txt
```

### 直接运行源码

```bash
go run ./cmd/server
```

### 构建可执行文件

```bash
go build -o handwritten-notes-ocr.exe ./cmd/server
```

默认访问地址：

[http://127.0.0.1:8080](http://127.0.0.1:8080)

## 环境变量

建议使用本地环境变量，或使用不会进入 Git 的本地 `.env` 文件。

示例见 `.env.example`。

- `PORT`：本地服务端口，默认 `8080`
- `OPEN_BROWSER`：启动时是否自动打开浏览器，默认 `1`
- `OCR_PYTHON_BIN`：Python 可执行文件路径
- `OCR_SCRIPT_PATH`：可选，自定义 OCR 脚本路径
- `OCR_LANG`：OCR 语言，默认 `ch`
- `OCR_CONCURRENCY`：后端 OCR 并发数，默认 `3`
- `OCR_ONLINE_API_URL`：可选，在线 OCR 接口地址
- `OCR_ONLINE_API_TOKEN`：在线 OCR 接口令牌
- `QWEN_API_KEY`：Qwen API Key
- `QWEN_BASE_URL`：Qwen 兼容接口的基础地址
- `QWEN_MODEL`：Qwen 模型名

## 接口概览

### 健康检查

- `GET /api/health`

### OCR

- `POST /api/ocr`
- `POST /api/ocr/batch`
- `POST /api/ocr/batch/stream`

### 学习笔记

- `POST /api/notes/precheck`
- `POST /api/notes/summarize`
- `POST /api/notes/summarize/stream`

## 安全说明

- 真实 API Key 不应提交到仓库。
- `.env` 和 `.env.*` 已被 Git 忽略。
- 仓库中只应保留 `.env.example`。

## 当前限制

- 本地 OCR 仍然依赖可用的 Python 运行环境和 PaddleOCR 安装。
- 手写体识别效果较大程度依赖图片质量与 handwriting 可辨识度。
- 当前 Markdown 预览是轻量渲染，不是完整 Markdown 引擎。
- 学习笔记阶段严格约束在已提供的 Markdown 内容内，不以补充外部知识为目标。

## 这个项目的展示价值

- 将本地 OCR、在线 OCR 和 LLM 后处理整合在一条工作流中
- 使用 Go 并发处理批量 OCR
- 使用 SSE 同时支持 OCR 进度流和 LLM 输出流
- 在生成学习笔记前保留可编辑的单图 OCR 结果
- 设计目标是离线优先工具，而不是纯云端演示项目

## 后续规划

- 更完整的 Markdown 渲染能力
- 更细的日志与性能分段统计
- 原生桌面应用打包
- 批量 Markdown 结果 ZIP 导出
- 更稳健的学习笔记生成前质量判断
