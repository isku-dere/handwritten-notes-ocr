# 手写笔记 OCR 离线桌面版

这是一个基于 `Go + PaddleOCR + 内嵌前端` 的本地离线应用：

- 启动一个本机专用界面
- 在浏览器里选择或拖拽手写笔记图片
- Go 程序调用本机 `PaddleOCR`
- 自动整理成 Markdown

当前版本不依赖任何外部 API，也不会把图片上传到在线服务。

## 桌面版形态

- 前端页面已嵌入 Go 二进制
- OCR Python 脚本已嵌入 Go 二进制，启动时自动释放到临时目录
- 服务只监听 `127.0.0.1`
- 程序启动后会自动打开本机界面

这意味着分发时你主要需要：

1. `server.exe`
2. 本机 Python 环境
3. 本机安装好的 `paddleocr` 和 `paddlepaddle`

## 运行要求

1. Go 1.25+，仅在源码运行或重新编译时需要
2. Python 3.10+，并且命令行可直接调用
3. 安装 PaddleOCR 依赖

```bash
pip install -r requirements.txt
```

如果你的 Python 可执行文件不是 `python`，启动前设置：

```bash
set OCR_PYTHON_BIN=C:\Path\To\python.exe
```

## 启动方式

源码运行：

```bash
go run ./cmd/server
```

编译可执行文件：

```bash
go build -o note-ocr.exe ./cmd/server
```

启动后界面会自动打开：

[http://127.0.0.1:8080](http://127.0.0.1:8080)

## 可配置环境变量

- `PORT`: 本地端口，默认 `8080`
- `OCR_PYTHON_BIN`: Python 可执行文件，默认 `python`
- `OCR_SCRIPT_PATH`: 可选，手动指定外部 OCR 脚本路径
- `OCR_LANG`: PaddleOCR 语言，默认 `ch`
- `OPEN_BROWSER`: 是否启动后自动打开界面，默认 `1`
- `OCR_ONLINE_API_URL`: 可选，配置远端 OCR API 地址
- `OCR_ONLINE_API_TOKEN`: 可选，配置远端 OCR Bearer Token

## OCR 链路

默认情况下，程序优先使用项目内 `.venv` 的本地 PaddleOCR。

如果配置了 `OCR_ONLINE_API_URL`，程序会先尝试调用在线 OCR 接口；在线接口失败时，会回退到本地 PaddleOCR。

当前已兼容 AI Studio 这类调用格式：

- 请求头：`Authorization: token <TOKEN>`
- 请求体字段：`file`、`fileType`、`useDocOrientationClassify`、`useDocUnwarping`、`useChartRecognition`
- 响应读取：`result.layoutParsingResults[*].markdown.text`

## 接口

虽然这是桌面版，本地程序仍然暴露本机接口供界面调用：

### `GET /api/health`

用于检查程序是否启动成功。

### `POST /api/ocr`

表单字段：

- `image`: 图片文件

返回示例：

```json
{
  "fileName": "note.jpg",
  "markdown": "# 会议记录\n\n## 待办\n\n- 整理方案\n",
  "rawText": "会议记录\n待办\n整理方案",
  "lines": [
    {
      "text": "会议记录",
      "score": 0.97,
      "box": [[0, 0], [10, 0], [10, 10], [0, 10]]
    }
  ]
}
```

## 当前限制

- 这仍然需要本机安装 Python 和 PaddleOCR 运行库，不是零依赖 EXE。
- Markdown 整理逻辑目前是启发式规则。
- 手写体识别准确率主要取决于图片质量和 PaddleOCR 模型表现。

## 下一步可继续做

1. 直接打包成真正的原生窗口版，例如 `Wails`
2. 增加多页笔记合并
3. 增加 Markdown 文件导出
4. 增加图片预处理和旋转校正
