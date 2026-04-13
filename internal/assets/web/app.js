const form = document.getElementById("upload-form");
const input = document.getElementById("image-input");
const dropzone = document.getElementById("dropzone");
const queueList = document.getElementById("queue-list");
const queueCount = document.getElementById("queue-count");
const currentFile = document.getElementById("current-file");
const previewImage = document.getElementById("preview-image");
const previewEmpty = document.getElementById("preview-empty");
const rotateLeftButton = document.getElementById("rotate-left-button");
const rotateRightButton = document.getElementById("rotate-right-button");
const markdownOutput = document.getElementById("markdown-output");
const markdownPreview = document.getElementById("markdown-preview");
const summaryOutput = document.getElementById("summary-output");
const summaryPreview = document.getElementById("summary-preview");
const summaryTitle = document.getElementById("summary-title");
const statusText = document.getElementById("status");
const submitButton = document.getElementById("submit-button");
const copyButton = document.getElementById("copy-button");
const saveButton = document.getElementById("save-button");
const summaryButton = document.getElementById("summary-button");
const summaryCopyButton = document.getElementById("summary-copy-button");
const summarySaveButton = document.getElementById("summary-save-button");
const modeButtons = Array.from(document.querySelectorAll(".mode-button"));
const tipButtons = Array.from(document.querySelectorAll("[data-tip-button]"));
const tipPanels = Array.from(document.querySelectorAll("[data-tip-panel]"));

const STATUS = {
  IDLE: "idle",
  QUEUED: "queued",
  PROCESSING: "processing",
  SUCCESS: "success",
  ERROR: "error",
};

const state = {
  items: [],
  selectedIndex: -1,
  summary: { title: "", markdown: "" },
  viewMode: { single: "source", summary: "source" },
  previewRequestId: 0,
  previewObjectURL: "",
};

input.addEventListener("change", () => appendFiles(Array.from(input.files || [])));
markdownOutput.addEventListener("input", handleMarkdownInput);
summaryOutput.addEventListener("input", () => {
  state.summary.markdown = summaryOutput.value;
  renderSummary();
});
markdownOutput.addEventListener("keydown", handleEditorKeydown);
summaryOutput.addEventListener("keydown", handleEditorKeydown);
rotateLeftButton.addEventListener("click", () => rotateCurrentItem(-90));
rotateRightButton.addEventListener("click", () => rotateCurrentItem(90));

tipButtons.forEach((button) => {
  button.addEventListener("click", (event) => {
    event.stopPropagation();
    const key = button.dataset.tipButton;
    const panel = document.querySelector(`[data-tip-panel="${key}"]`);
    const shouldShow = panel?.classList.contains("is-hidden");
    hideTipPanels();
    if (shouldShow) panel?.classList.remove("is-hidden");
  });
});

document.addEventListener("click", (event) => {
  if (event.target.closest(".editor-tools")) return;
  hideTipPanels();
});

modeButtons.forEach((button) => {
  button.addEventListener("click", () => {
    state.viewMode[button.dataset.target] = button.dataset.mode;
    renderModes();
  });
});

["dragenter", "dragover"].forEach((eventName) => {
  dropzone.addEventListener(eventName, (event) => {
    event.preventDefault();
    dropzone.classList.add("is-dragging");
  });
});

["dragleave", "drop"].forEach((eventName) => {
  dropzone.addEventListener(eventName, (event) => {
    event.preventDefault();
    dropzone.classList.remove("is-dragging");
  });
});

dropzone.addEventListener("drop", (event) => {
  const files = Array.from(event.dataTransfer.files || []).filter((file) => file.type.startsWith("image/"));
  if (!files.length) return;
  mergeInputFiles(files);
  appendFiles(files);
});

form.addEventListener("submit", async (event) => {
  event.preventDefault();
  const pending = state.items.filter((item) => !item.removed && (item.status === STATUS.IDLE || item.status === STATUS.ERROR));
  if (!pending.length) {
    setStatus("当前没有待处理的新图片");
    return;
  }
  await runBatchOCR();
});

copyButton.addEventListener("click", async () => {
  const current = getCurrentItem();
  if (!current?.result?.markdown?.trim()) {
    setStatus("当前没有可复制的 Markdown");
    return;
  }
  await copyText(current.result.markdown, `已复制 ${current.file.name}`);
});

saveButton.addEventListener("click", () => {
  const current = getCurrentItem();
  if (!current?.result?.markdown?.trim()) {
    setStatus("当前没有可保存的 Markdown");
    return;
  }
  saveMarkdownFile(current.file.name, current.result.markdown);
  setStatus(`已保存 ${toMarkdownName(current.file.name)}`);
});

summaryCopyButton.addEventListener("click", async () => {
  if (!state.summary.markdown.trim()) {
    setStatus("当前没有可复制的学习笔记");
    return;
  }
  await copyText(state.summary.markdown, "已复制学习笔记");
});

summarySaveButton.addEventListener("click", () => {
  if (!state.summary.markdown.trim()) {
    setStatus("当前没有可保存的学习笔记");
    return;
  }
  const fileName = state.summary.title || "study-note";
  saveMarkdownFile(fileName, state.summary.markdown);
  setStatus(`已保存 ${toMarkdownName(fileName)}`);
});

summaryButton.addEventListener("click", async () => {
  const notes = state.items
    .filter((item) => !item.removed && item.result?.markdown?.trim())
    .map((item) => ({
      fileName: item.file.name,
      markdown: item.result.markdown,
      edited: item.dirty,
    }));

  if (!notes.length) {
    setStatus("没有可用于生成学习笔记的 Markdown 内容");
    return;
  }

  const editedCount = notes.filter((item) => item.edited).length;
  const uneditedCount = notes.length - editedCount;
  summaryButton.disabled = true;

  try {
    const precheck = await precheckStudyNote(notes);
    if (precheck.shouldConfirm) {
      const confirmed = window.confirm(precheck.message || "当前内容较短，是否继续生成学习笔记？");
      if (!confirmed) {
        setStatus("已取消生成学习笔记");
        return;
      }
    }

    state.summary = { title: "生成中...", markdown: "" };
    renderSummary();
    setStatus(`正在生成学习笔记：已编辑 ${editedCount} 条，未编辑 ${uneditedCount} 条`);

    await streamStudyNote(notes, precheck.shouldConfirm);
    setStatus("学习笔记已生成");
  } catch (error) {
    setStatus(error.message || "学习笔记生成失败");
  } finally {
    summaryButton.disabled = false;
  }
});

function appendFiles(files) {
  if (!files.length) {
    setStatus("等待上传");
    return;
  }

  const existingKeys = new Set(state.items.map((item) => fileKey(item.file)));
  const appended = [];
  for (const file of files) {
    const key = fileKey(file);
    if (existingKeys.has(key)) continue;
    existingKeys.add(key);
    appended.push({
      file,
      status: STATUS.IDLE,
      result: null,
      error: "",
      dirty: false,
      durationMs: 0,
      rotation: 0,
      removed: false,
    });
  }

  if (!appended.length) {
    setStatus("所选图片已在文件队列中");
    return;
  }

  state.items = state.items.concat(appended);
  ensureSelection();
  renderQueue();
  renderCurrentPanel();
  renderSummary();
  renderModes();
  setStatus(`已追加 ${appended.length} 个文件，当前队列共 ${state.items.length} 个`);
}

async function runBatchOCR() {
  submitButton.disabled = true;
  const pendingIndexes = state.items
    .map((item, index) => ({ item, index }))
    .filter(({ item }) => !item.removed && (item.status === STATUS.IDLE || item.status === STATUS.ERROR))
    .map(({ index }) => index);

  for (const index of pendingIndexes) {
    updateItem(index, {
      status: STATUS.PROCESSING,
      error: "",
      durationMs: 0,
    });
  }

  ensureSelection();
  renderQueue();
  renderCurrentPanel();
  setStatus(`正在批量识别 ${pendingIndexes.length} 个文件...`);

  try {
    let successCount = 0;
    await processBatchItemsStream(pendingIndexes.map((index) => state.items[index]), ({ index, result }) => {
      const targetIndex = pendingIndexes[index];
      if (typeof targetIndex !== "number") return;

      if (result.error) {
        updateItem(targetIndex, {
          status: STATUS.ERROR,
          error: result.error,
          durationMs: result.durationMs || 0,
        });
      } else {
        successCount += 1;
        updateItem(targetIndex, {
          status: STATUS.SUCCESS,
          result: {
            ...result,
            markdown: result.markdown || state.items[targetIndex].result?.markdown || "",
          },
          error: "",
          dirty: false,
          durationMs: result.durationMs || 0,
        });
      }

      renderQueue();
      renderCurrentPanel();
      setStatus(`已完成 ${successCount} / ${pendingIndexes.length}`);
    });

    setStatus(`批量处理完成：成功 ${successCount} / ${pendingIndexes.length}`);
  } catch (error) {
    pendingIndexes.forEach((index) => {
      if (state.items[index].status === STATUS.PROCESSING) {
        updateItem(index, {
          status: STATUS.ERROR,
          error: error.message || "批量处理失败",
          durationMs: 0,
        });
      }
    });
    renderQueue();
    renderCurrentPanel();
    setStatus(error.message || "批量处理失败");
  } finally {
    submitButton.disabled = false;
  }
}

async function processBatchItemsStream(items, onItem) {
  const body = new FormData();
  for (const item of items) {
    const uploadFile = await buildUploadFile(item.file, item.rotation || 0);
    body.append("images", uploadFile);
  }

  const response = await fetch("/api/ocr/batch/stream", {
    method: "POST",
    body,
  });
  if (!response.ok || !response.body) {
    const payload = await response.json().catch(() => ({}));
    throw new Error(payload.error || "批量处理失败");
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  while (true) {
    const { value, done } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });

    let boundary = buffer.indexOf("\n\n");
    while (boundary !== -1) {
      const eventBlock = buffer.slice(0, boundary);
      buffer = buffer.slice(boundary + 2);
      const event = parseSSEBlock(eventBlock);
      if (event?.name === "item") onItem(event.payload);
      if (event?.name === "error") throw new Error(event.payload.error || "批量处理失败");
      boundary = buffer.indexOf("\n\n");
    }
  }
}

async function precheckStudyNote(notes) {
  const response = await fetch("/api/notes/precheck", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ notes }),
  });
  if (!response.ok) {
    const payload = await response.json().catch(() => ({}));
    throw new Error(payload.error || "学习笔记预检查失败");
  }
  return response.json();
}

async function streamStudyNote(notes, force) {
  const response = await fetch("/api/notes/summarize/stream", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ notes, force }),
  });
  if (!response.ok || !response.body) {
    const payload = await response.json().catch(() => ({}));
    throw new Error(payload.error || "学习笔记生成失败");
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let streamedJSON = "";

  while (true) {
    const { value, done } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });

    let boundary = buffer.indexOf("\n\n");
    while (boundary !== -1) {
      const eventBlock = buffer.slice(0, boundary);
      buffer = buffer.slice(boundary + 2);
      const event = parseSSEBlock(eventBlock);
      if (event?.name === "delta") {
        streamedJSON += event.payload.text || "";
        state.summary.markdown = extractMarkdownFromJSONStream(streamedJSON);
        summaryTitle.textContent = extractTitleFromJSONStream(streamedJSON) || "正在生成...";
        renderSummary();
      }
      if (event?.name === "confirm_required") {
        throw new Error(event.payload.message || "当前内容较短，请确认后再生成");
      }
      if (event?.name === "final") {
        state.summary = {
          title: event.payload.title || "",
          markdown: event.payload.markdown || "",
        };
        renderSummary();
        renderModes();
      }
      if (event?.name === "error") throw new Error(event.payload.error || "学习笔记生成失败");
      boundary = buffer.indexOf("\n\n");
    }
  }
}

function parseSSEBlock(block) {
  if (!block.trim()) return null;
  const lines = block.split("\n");
  let eventName = "message";
  const dataLines = [];

  for (const line of lines) {
    if (line.startsWith("event:")) {
      eventName = line.slice(6).trim();
    } else if (line.startsWith("data:")) {
      dataLines.push(line.slice(5).trim());
    }
  }

  const dataText = dataLines.join("\n");
  return {
    name: eventName,
    payload: dataText ? JSON.parse(dataText) : {},
  };
}

function handleMarkdownInput() {
  const current = getCurrentItem();
  if (!current || !current.result) return;
  current.result.markdown = markdownOutput.value;
  current.dirty = true;
  renderQueue();
  renderCurrentPanel();
}

function mergeInputFiles(newFiles) {
  const transfer = new DataTransfer();
  const existing = Array.from(input.files || []);
  const allFiles = existing.concat(newFiles);
  const seen = new Set();

  for (const file of allFiles) {
    const key = fileKey(file);
    if (seen.has(key)) continue;
    seen.add(key);
    transfer.items.add(file);
  }
  input.files = transfer.files;
}

function fileKey(file) {
  return [file.name, file.size, file.lastModified].join(":");
}

function updateItem(index, patch) {
  state.items[index] = { ...state.items[index], ...patch };
}

function renderQueue() {
  queueList.innerHTML = "";
  const activeItems = state.items.filter((item) => !item.removed);
  const processedCount = activeItems.filter((item) => item.status === STATUS.SUCCESS || item.status === STATUS.ERROR).length;
  queueCount.textContent = activeItems.length ? `${processedCount}/${activeItems.length} 已完成` : "0 项";

  if (!state.items.length) {
    const empty = document.createElement("div");
    empty.className = "queue-empty";
    empty.textContent = "还没有待处理文件";
    queueList.appendChild(empty);
    return;
  }

  state.items.forEach((item, index) => {
    const card = document.createElement("div");
    card.className = ["queue-item", index === state.selectedIndex ? "is-active" : "", `is-${item.status}`, item.removed ? "is-removed" : ""]
      .filter(Boolean)
      .join(" ");

    const mainButton = document.createElement("button");
    mainButton.type = "button";
    mainButton.className = "queue-item-main";
    mainButton.disabled = item.removed;
    mainButton.title = item.file.name;
    mainButton.addEventListener("click", () => {
      if (item.removed) return;
      state.selectedIndex = index;
      renderQueue();
      renderCurrentPanel();
    });

    const title = document.createElement("span");
    title.className = "queue-item-title";
    title.textContent = item.file.name;
    title.title = item.file.name;

    const meta = document.createElement("span");
    meta.className = "queue-item-meta";
    meta.textContent = getStatusLabel(item);

    const duration = document.createElement("span");
    duration.className = "queue-item-duration";
    duration.textContent = item.durationMs > 0 ? formatDuration(item.durationMs) : "--";

    const actions = document.createElement("div");
    actions.className = "queue-item-actions";

    const toggleButton = document.createElement("button");
    toggleButton.type = "button";
    toggleButton.className = "queue-toggle-button";
    toggleButton.textContent = item.removed ? "恢复" : "移除";
    toggleButton.title = item.removed ? "恢复到队列" : "从队列中移除";
    toggleButton.addEventListener("click", () => toggleRemoved(index));

    actions.appendChild(toggleButton);
    mainButton.append(title, meta, duration);
    card.append(mainButton, actions);
    queueList.appendChild(card);
  });
}

function renderCurrentPanel() {
  ensureSelection();
  const current = getCurrentItem();
  currentFile.textContent = current ? current.file.name : "未选择文件";
  currentFile.title = current ? current.file.name : "";
  void renderPreview(current);

  if (!current) {
    markdownOutput.value = "";
    markdownOutput.placeholder = "识别结果会显示在这里";
  } else if (current.result?.markdown) {
    markdownOutput.value = current.result.markdown;
  } else if (current.status === STATUS.PROCESSING) {
    markdownOutput.value = "";
    markdownOutput.placeholder = "当前文件正在处理中...";
  } else {
    markdownOutput.value = "";
    markdownOutput.placeholder = "当前文件还未开始处理";
  }

  renderModes();
}

function renderSummary() {
  summaryTitle.textContent = state.summary.title || "尚未生成";
  summaryOutput.value = state.summary.markdown || "";
  renderModes();
}

function renderModes() {
  modeButtons.forEach((button) => {
    const active = state.viewMode[button.dataset.target] === button.dataset.mode;
    button.classList.toggle("is-active", active);
  });
  toggleEditorView("single", markdownOutput, markdownPreview, markdownOutput.value);
  toggleEditorView("summary", summaryOutput, summaryPreview, summaryOutput.value);
}

function toggleEditorView(target, textarea, preview, markdown) {
  const isPreview = state.viewMode[target] === "preview";
  textarea.classList.toggle("is-hidden", isPreview);
  preview.classList.toggle("is-hidden", !isPreview);
  if (isPreview) preview.innerHTML = renderMarkdown(markdown || "");
}

async function renderPreview(item) {
  const requestId = ++state.previewRequestId;

  if (!item?.file || item.removed) {
    clearPreviewURL();
    previewImage.removeAttribute("src");
    previewImage.classList.add("is-hidden");
    previewEmpty.classList.remove("is-hidden");
    rotateLeftButton.disabled = true;
    rotateRightButton.disabled = true;
    return;
  }

  rotateLeftButton.disabled = false;
  rotateRightButton.disabled = false;

  try {
    const previewFile = await buildUploadFile(item.file, item.rotation || 0);
    if (requestId !== state.previewRequestId) return;
    clearPreviewURL();
    state.previewObjectURL = URL.createObjectURL(previewFile);
    previewImage.src = state.previewObjectURL;
  } catch {
    if (requestId !== state.previewRequestId) return;
    clearPreviewURL();
    state.previewObjectURL = URL.createObjectURL(item.file);
    previewImage.src = state.previewObjectURL;
  }

  previewImage.classList.remove("is-hidden");
  previewEmpty.classList.add("is-hidden");
}

function clearPreviewURL() {
  if (state.previewObjectURL) {
    URL.revokeObjectURL(state.previewObjectURL);
    state.previewObjectURL = "";
  }
}

function hideTipPanels() {
  tipPanels.forEach((panel) => panel.classList.add("is-hidden"));
}

function getCurrentItem() {
  const item = state.items[state.selectedIndex];
  if (!item || item.removed) return null;
  return item;
}

function ensureSelection() {
  if (state.selectedIndex >= 0 && state.items[state.selectedIndex] && !state.items[state.selectedIndex].removed) return;
  state.selectedIndex = state.items.findIndex((item) => !item.removed);
}

function getStatusLabel(item) {
  if (item.removed) return "已移除";
  switch (item.status) {
    case STATUS.QUEUED:
      return "排队中";
    case STATUS.PROCESSING:
      return "处理中";
    case STATUS.SUCCESS:
      return item.dirty ? "已编辑" : "处理完成";
    case STATUS.ERROR:
      return "处理失败";
    case STATUS.IDLE:
      return item.result?.markdown ? "待重新识别" : "待处理";
    default:
      return "待处理";
  }
}

function formatDuration(durationMs) {
  const seconds = durationMs / 1000;
  if (seconds >= 3600) return `${(seconds / 3600).toFixed(2)}h`;
  if (seconds >= 60) return `${(seconds / 60).toFixed(2)}m`;
  return `${seconds.toFixed(2)}s`;
}

function rotateCurrentItem(delta) {
  const current = getCurrentItem();
  if (!current) {
    setStatus("请先选择一张图片");
    return;
  }

  current.rotation = normalizeRotation((current.rotation || 0) + delta);
  if (current.result?.markdown) {
    current.status = STATUS.IDLE;
    current.error = "";
    current.durationMs = 0;
    state.summary = { title: "", markdown: "" };
    renderSummary();
    setStatus(`已旋转 ${current.file.name}，保留原 Markdown，等待重新识别`);
  } else {
    setStatus(`已旋转 ${current.file.name}`);
  }

  renderQueue();
  renderCurrentPanel();
}

function toggleRemoved(index) {
  const item = state.items[index];
  item.removed = !item.removed;
  setStatus(item.removed ? `已移除 ${item.file.name}` : `已恢复 ${item.file.name}`);
  ensureSelection();
  renderQueue();
  renderCurrentPanel();
}

function normalizeRotation(rotation) {
  return ((rotation % 360) + 360) % 360;
}

async function buildUploadFile(file, rotation) {
  const normalizedRotation = normalizeRotation(rotation);
  if (normalizedRotation === 0) return file;

  const dataUrl = await readFileAsDataURL(file);
  const image = await loadImage(dataUrl);
  const canvas = document.createElement("canvas");
  const context = canvas.getContext("2d");

  if (normalizedRotation === 90 || normalizedRotation === 270) {
    canvas.width = image.height;
    canvas.height = image.width;
  } else {
    canvas.width = image.width;
    canvas.height = image.height;
  }

  context.translate(canvas.width / 2, canvas.height / 2);
  context.rotate((normalizedRotation * Math.PI) / 180);
  context.drawImage(image, -image.width / 2, -image.height / 2);

  const blob = await new Promise((resolve, reject) => {
    canvas.toBlob((value) => {
      if (value) resolve(value);
      else reject(new Error("旋转图片失败"));
    }, file.type || "image/png");
  });

  return new File([blob], file.name, {
    type: blob.type || file.type,
    lastModified: Date.now(),
  });
}

function readFileAsDataURL(file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(reader.result);
    reader.onerror = () => reject(new Error("读取图片失败"));
    reader.readAsDataURL(file);
  });
}

function loadImage(src) {
  return new Promise((resolve, reject) => {
    const image = new Image();
    image.onload = () => resolve(image);
    image.onerror = () => reject(new Error("加载图片失败"));
    image.src = src;
  });
}

function extractMarkdownFromJSONStream(raw) {
  const keyIndex = raw.indexOf('"markdown"');
  if (keyIndex === -1) return "";
  const colonIndex = raw.indexOf(":", keyIndex);
  const quoteIndex = raw.indexOf('"', colonIndex + 1);
  if (quoteIndex === -1) return "";

  let output = "";
  let escaping = false;
  for (let i = quoteIndex + 1; i < raw.length; i += 1) {
    const char = raw[i];
    if (escaping) {
      if (char === "n") output += "\n";
      else if (char === "r") output += "";
      else if (char === "t") output += "\t";
      else output += char;
      escaping = false;
      continue;
    }
    if (char === "\\") {
      escaping = true;
      continue;
    }
    if (char === '"') break;
    output += char;
  }
  return output;
}

function extractTitleFromJSONStream(raw) {
  const match = raw.match(/"title"\s*:\s*"([^"]*)/);
  return match ? match[1] : "";
}

async function copyText(text, successMessage) {
  try {
    await navigator.clipboard.writeText(text);
    setStatus(successMessage);
  } catch {
    setStatus("复制失败，请手动复制");
  }
}

function saveMarkdownFile(fileName, markdown) {
  const blob = new Blob([markdown], { type: "text/markdown;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = toMarkdownName(fileName);
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(url);
}

function toMarkdownName(fileName) {
  const normalized = fileName.replace(/[\\/:*?"<>|]/g, "-").replace(/\.[^.]+$/, "");
  return `${normalized || "note"}.md`;
}

function setStatus(message) {
  statusText.textContent = message;
}

function handleEditorKeydown(event) {
  const textarea = event.target;
  const isMac = navigator.platform.toUpperCase().includes("MAC");
  const mod = isMac ? event.metaKey : event.ctrlKey;

  if (event.key === "Tab") {
    event.preventDefault();
    wrapSelection(textarea, "  ", "");
    return;
  }
  if (mod && event.key.toLowerCase() === "b") {
    event.preventDefault();
    wrapSelection(textarea, "**", "**");
    return;
  }
  if (mod && event.key.toLowerCase() === "i") {
    event.preventDefault();
    wrapSelection(textarea, "*", "*");
    return;
  }
  if (event.altKey && event.key === "1") {
    event.preventDefault();
    prefixCurrentLine(textarea, "# ");
    return;
  }
  if (event.altKey && event.key === "2") {
    event.preventDefault();
    prefixCurrentLine(textarea, "## ");
  }
}

function wrapSelection(textarea, before, after) {
  const start = textarea.selectionStart;
  const end = textarea.selectionEnd;
  const value = textarea.value;
  const selected = value.slice(start, end);
  textarea.value = value.slice(0, start) + before + selected + after + value.slice(end);
  textarea.selectionStart = start + before.length;
  textarea.selectionEnd = end + before.length;
  textarea.dispatchEvent(new Event("input", { bubbles: true }));
}

function prefixCurrentLine(textarea, prefix) {
  const start = textarea.selectionStart;
  const value = textarea.value;
  const lineStart = value.lastIndexOf("\n", start - 1) + 1;
  textarea.value = value.slice(0, lineStart) + prefix + value.slice(lineStart);
  textarea.selectionStart = start + prefix.length;
  textarea.selectionEnd = textarea.selectionStart;
  textarea.dispatchEvent(new Event("input", { bubbles: true }));
}

function renderMarkdown(markdown) {
  const lines = markdown.replace(/\r/g, "").split("\n");
  const html = [];
  let inList = false;
  let inBlockquote = false;

  const closeBlocks = () => {
    if (inList) {
      html.push("</ul>");
      inList = false;
    }
    if (inBlockquote) {
      html.push("</blockquote>");
      inBlockquote = false;
    }
  };

  for (const rawLine of lines) {
    const line = rawLine.trimEnd();
    if (!line.trim()) {
      closeBlocks();
      continue;
    }

    const heading = line.match(/^(#{1,6})\s+(.*)$/);
    if (heading) {
      closeBlocks();
      const level = heading[1].length;
      html.push(`<h${level}>${inlineMarkdown(heading[2])}</h${level}>`);
      continue;
    }

    if (line.startsWith("> ")) {
      if (!inBlockquote) {
        closeBlocks();
        html.push("<blockquote>");
        inBlockquote = true;
      }
      html.push(`<p>${inlineMarkdown(line.slice(2))}</p>`);
      continue;
    }

    const list = line.match(/^[-*]\s+(.*)$/);
    if (list) {
      if (!inList) {
        closeBlocks();
        html.push("<ul>");
        inList = true;
      }
      html.push(`<li>${inlineMarkdown(list[1])}</li>`);
      continue;
    }

    closeBlocks();
    html.push(`<p>${inlineMarkdown(line)}</p>`);
  }

  closeBlocks();
  return html.join("");
}

function inlineMarkdown(text) {
  return escapeHtml(text)
    .replace(/\*\*(.+?)\*\*/g, "<strong>$1</strong>")
    .replace(/\*(.+?)\*/g, "<em>$1</em>")
    .replace(/`(.+?)`/g, "<code>$1</code>");
}

function escapeHtml(text) {
  return text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}
