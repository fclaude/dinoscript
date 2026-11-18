const { examples, starters, messages } = JSON.parse(document.getElementById("play-config").textContent);
const ui = Object.fromEntries([
  "examples", "file-list", "new-file", "open-file", "save-file", "file-picker", "file-name", "close-file",
  "run", "stop", "source", "line-numbers", "highlighted-code", "console-view", "console-empty",
  "program-output", "diagnostic", "program-input", "answer", "send-answer", "clear-output",
  "run-status", "draft-status", "code-position", "show-error",
].map(id => [id, document.getElementById(id)]));

const language = document.documentElement.lang;
const encoder = new TextEncoder();
const decoder = new TextDecoder("utf-8", { fatal: true, ignoreBOM: true });
const maxSourceBytes = 64 * 1024;
const maxInputBytes = 1024;
const maxEchoCharacters = 64 * 1024;
const storagePrefix = `dinoscript.play.v1.${language}.`;
const filePrefix = `${storagePrefix}file.`;
const activeKey = `${storagePrefix}active`;

let files = [];
let active;
let worker;
let workerReady = false;
let launchTimeout;
let run;
let prompt;
let diagnostic;
let storageWarning = false;

function setStatus(key, state) {
  ui["run-status"].textContent = messages[key];
  ui["run-status"].dataset.state = state ?? "";
}

function localStorageWarning() {
  if (storageWarning) return;
  storageWarning = true;
  ui["draft-status"].textContent = messages.storageError;
}

function store(key, value) {
  try {
    if (value === null) localStorage.removeItem(key);
    else localStorage.setItem(key, value);
  } catch {
    localStorageWarning();
  }
}

function storeFile(file) {
  store(filePrefix + file.id, JSON.stringify(file));
}

function restoreFiles() {
  let selected;
  try {
    selected = localStorage.getItem(activeKey);
    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i);
      if (!key?.startsWith(filePrefix)) continue;
      try {
        const file = JSON.parse(localStorage.getItem(key));
        if (file && key === filePrefix + file.id && typeof file.name === "string" &&
            typeof file.text === "string" && typeof file.saved === "string" &&
            Number.isInteger(file.order) && Number.isInteger(file.color)) {
          files.push(file);
        }
      } catch {
        // A damaged file entry should not prevent other drafts from opening.
      }
    }
  } catch {
    localStorageWarning();
  }
  files.sort((a, b) => a.order - b.order);
  if (files.length === 0) {
    if (selected === null || selected === undefined) {
      for (const key of starters) {
        const { filename, source } = examples[key];
        createFile(filename, source, key);
      }
    } else {
      createFile(messages.untitled, "");
    }
  }
  return files.find(file => file.id === selected) ?? files[0];
}

function createFile(name, text, example) {
  const id = crypto.getRandomValues(new Uint32Array(2)).join("-");
  const last = files.at(-1);
  const order = Math.max(Date.now(), (last?.order ?? 0) + 1);
  const color = last ? (last.color + 1) % 5 : 0;
  const file = { id, name, text, saved: text, color, order, example };
  files.push(file);
  storeFile(file);
  return file;
}

function unusedName(name) {
  let candidate = name;
  const base = name.replace(/\.ds$/i, "");
  for (let i = 2; files.some(file => file.name === candidate); i++) {
    candidate = `${base} ${i}.ds`;
  }
  return candidate;
}

function fileIcon(checked) {
  const icon = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  const symbol = document.createElementNS(icon.namespaceURI, "use");
  icon.setAttribute("aria-hidden", "true");
  symbol.setAttribute("href", checked ? "#check-icon" : "#file-icon");
  icon.append(symbol);
  return icon;
}

function fileLabel(file) {
  return file.text === file.saved ? file.name : `${file.name} *`;
}

function renderFiles() {
  const list = document.createDocumentFragment();
  for (const file of files) {
    const item = document.createElement("li");
    const button = document.createElement("button");
    const label = document.createElement("span");
    button.type = "button";
    button.className = `file-button tab-${file.color}`;
    button.dataset.file = file.id;
    if (file === active) button.setAttribute("aria-current", "true");
    button.title = file.name;
    label.textContent = fileLabel(file);
    button.append(fileIcon(file === active), label);
    button.addEventListener("click", () => showFile(file, true));
    item.append(button);
    list.append(item);
  }
  ui["file-list"].replaceChildren(list);
  const selected = ui["file-list"].querySelector("[aria-current]");
  if (selected && ui["file-list"].scrollWidth > ui["file-list"].clientWidth) {
    selected.scrollIntoView({ block: "nearest", inline: "nearest" });
  }
}

function showFile(file, focus = false) {
  if (active !== file) {
    if (active) {
      active.selection = [ui.source.selectionStart, ui.source.selectionEnd];
      active.scroll = [ui.source.scrollLeft, ui.source.scrollTop];
    }
    if (run) stopRun();
    clearOutput();
    setStatus("ready");
    active = file;
    ui.source.value = file.text;
    ui.source.setSelectionRange(...(file.selection ?? [0, 0]));
    [ui.source.scrollLeft, ui.source.scrollTop] = file.scroll ?? [0, 0];
    updateCode();
    updateFileLabel();
    renderFiles();
    store(activeKey, file.id);
  }
  if (focus) ui.source.focus({ preventScroll: true });
}

function updateFileLabel() {
  ui["file-name"].textContent = fileLabel(active);
  ui["file-name"].title = active.name;
  const currentLabel = ui["file-list"].querySelector("[aria-current] span");
  if (currentLabel) currentLabel.textContent = fileLabel(active);
}

function openExample(key, focus) {
  const example = examples[key];
  if (!Object.hasOwn(examples, key)) return;
  const existing = files.find(file => file.example === key && file.text === example.source);
  showFile(existing ?? createFile(unusedName(example.filename), example.source, key), focus);
}

const keywords = new Set("let while do end function output if then else true false and or not".split(" "));
const builtins = new Set(["print", "readint", "pause"]);
const tokens = /#[^\n]*|"(?:[^"\\\n]|\\[^\n])*"?|[a-zA-Z_][a-zA-Z_0-9]*|[0-9]+(?:\.[0-9]*)?(?:[eE][+-]?[0-9]+)?/g;

function updateCode() {
  const source = ui.source.value;
  const highlighted = document.createDocumentFragment();
  let position = 0;
  for (const match of source.matchAll(tokens)) {
    const token = match[0];
    let style;
    if (token[0] === "#") style = "comment";
    else if (token[0] === '"') style = "string-value";
    else if (keywords.has(token)) style = "keyword";
    else if (builtins.has(token)) style = "call";
    else if (/^[0-9]/.test(token)) style = "number";
    if (!style) continue;
    highlighted.append(source.slice(position, match.index));
    const span = document.createElement("span");
    span.className = `syntax-${style}`;
    span.textContent = token;
    highlighted.append(span);
    position = match.index + token.length;
  }
  highlighted.append(source.slice(position), "\n");
  ui["highlighted-code"].replaceChildren(highlighted);
  const count = source.split("\n").length;
  ui["line-numbers"].textContent = Array.from({ length: count }, (_, index) => index + 1).join("\n");
  syncScroll();
  updatePosition();
  updateDiagnosticButton();
}

function syncScroll() {
  const { scrollLeft, scrollTop } = ui.source;
  ui["highlighted-code"].style.transform = `translate(${-scrollLeft}px, ${-scrollTop}px)`;
  ui["line-numbers"].style.transform = `translateY(${-scrollTop}px)`;
}

function updatePosition() {
  const lines = ui.source.value.slice(0, ui.source.selectionDirection === "backward"
    ? ui.source.selectionStart : ui.source.selectionEnd).split("\n");
  ui["code-position"].textContent = messages.position
    .replace("{line}", lines.length).replace("{column}", Array.from(lines.at(-1)).length + 1);
}

function clearOutput() {
  ui["program-output"].replaceChildren();
  ui.diagnostic.hidden = true;
  ui.diagnostic.textContent = "";
  ui["console-empty"].querySelector("p").textContent = messages.empty;
  ui["console-empty"].hidden = false;
  ui["clear-output"].disabled = true;
  diagnostic = undefined;
  if (run) run.echoCharacters = 0;
  updateDiagnosticButton();
}

function appendOutput(text) {
  ui["console-empty"].hidden = true;
  ui["program-output"].append(text);
  ui["console-view"].scrollTop = ui["console-view"].scrollHeight;
  ui["clear-output"].disabled = false;
}

function updateDiagnosticButton() {
  ui["show-error"].hidden = !diagnostic || diagnostic.source !== ui.source.value;
}

function goToError() {
  if (!diagnostic || diagnostic.source !== ui.source.value) return;
  const bytes = encoder.encode(diagnostic.source);
  const start = decoder.decode(bytes.subarray(0, diagnostic.start)).length;
  const end = decoder.decode(bytes.subarray(0, diagnostic.end)).length;
  ui.source.focus();
  ui.source.setSelectionRange(start, end);
  const lineHeight = parseFloat(getComputedStyle(ui.source).lineHeight);
  const top = (diagnostic.line - 1) * lineHeight;
  if (top < ui.source.scrollTop || top + 2 * lineHeight > ui.source.scrollTop + ui.source.clientHeight) {
    ui.source.scrollTop = Math.max(0, top - 2 * lineHeight);
  }
  syncScroll();
  updatePosition();
}

function resetWorker() {
  clearTimeout(launchTimeout);
  worker?.terminate();
  worker = undefined;
  workerReady = false;
}

function requestInput(kind) {
  prompt = kind;
  ui.answer.hidden = kind === "pause";
  ui["send-answer"].textContent = kind === "pause" ? messages.continue : messages.enter;
  ui["program-input"].hidden = false;
  setStatus(kind);
  (kind === "pause" ? ui["send-answer"] : ui.answer).focus({ preventScroll: true });
  ui["program-input"].scrollIntoView({ block: "nearest" });
}

function hideInput() {
  const moveFocus = ui["program-input"].contains(document.activeElement);
  prompt = undefined;
  ui.answer.value = "";
  ui["program-input"].hidden = true;
  if (moveFocus) ui.source.focus({ preventScroll: true });
}

function finishRun(status, message) {
  const focusRun = document.activeElement === ui.stop;
  if (message?.kind === "diagnostic") {
    diagnostic = { ...message, source: run.source };
    ui["console-empty"].hidden = true;
    ui.diagnostic.textContent = message.text;
    ui.diagnostic.hidden = false;
    ui["console-view"].scrollTop = ui["console-view"].scrollHeight;
    ui["clear-output"].disabled = false;
    updateDiagnosticButton();
  } else if (status === "finished" && !ui["program-output"].hasChildNodes()) {
    ui["console-empty"].querySelector("p").textContent = messages.noOutput;
  }
  hideInput();
  run = undefined;
  ui.run.disabled = false;
  ui.stop.disabled = true;
  setStatus(status, status === "fixError" || status === "engineError" ? "error" : undefined);
  if (focusRun) ui.run.focus({ preventScroll: true });
}

function workerError() {
  resetWorker();
  if (run) finishRun("engineError");
}

function sendRun() {
  setStatus("running");
  worker.postMessage({ kind: "run", source: run.source, language });
}

function processMessage(message) {
  switch (message.kind) {
    case "ready":
      clearTimeout(launchTimeout);
      workerReady = true;
      if (run) sendRun();
      break;
    case "output": appendOutput(message.text); break;
    case "number":
    case "pause": requestInput(message.kind); break;
    case "diagnostic": finishRun("fixError", message); break;
    case "done": finishRun("finished"); break;
    case "error": workerError(); break;
  }
}

function beginRun() {
  if (run) return;
  clearOutput();
  run = { source: ui.source.value, echoCharacters: 0 };
  const focusStop = document.activeElement === ui.run;
  ui.run.disabled = true;
  ui.stop.disabled = false;
  if (focusStop) ui.stop.focus({ preventScroll: true });
  if (workerReady) {
    sendRun();
    return;
  }
  setStatus("loading");
  try {
    const next = new Worker(new URL("play-worker.js", import.meta.url));
    worker = next;
    next.onmessage = event => {
      if (worker === next && run) processMessage(event.data);
    };
    next.onerror = next.onmessageerror = () => {
      if (worker === next) workerError();
    };
    launchTimeout = setTimeout(workerError, 20000);
  } catch {
    workerError();
  }
}

function stopRun() {
  if (!run) return;
  resetWorker();
  finishRun("stopped");
}

ui.source.addEventListener("input", () => {
  active.text = ui.source.value;
  storeFile(active);
  updateFileLabel();
  updateCode();
});
ui.source.addEventListener("scroll", syncScroll);
for (const type of ["select", "click", "keyup", "selectionchange"]) {
  ui.source.addEventListener(type, updatePosition);
}
ui.examples.addEventListener("change", () => {
  openExample(ui.examples.value, true);
  ui.examples.value = "";
});
ui["new-file"].addEventListener("click", () => showFile(createFile(unusedName(messages.untitled), ""), true));
ui["open-file"].addEventListener("click", () => ui["file-picker"].click());
ui["file-picker"].addEventListener("change", async () => {
  const file = ui["file-picker"].files[0];
  ui["file-picker"].value = "";
  if (!file) return;
  if (file.size > maxSourceBytes) {
    setStatus("fileTooBig", "error");
    return;
  }
  try {
    const text = decoder.decode(await file.arrayBuffer()).replace(/^\uFEFF/, "").replace(/\r\n?/g, "\n");
    if (/[\x00-\x08\x0b\x0c\x0e-\x1f]/.test(text)) {
      setStatus("openError", "error");
      return;
    }
    const name = /\.ds$/i.test(file.name) ? file.name : `${file.name}.ds`;
    showFile(createFile(unusedName(name), text), true);
  } catch {
    setStatus("openError", "error");
  }
});
ui["save-file"].addEventListener("click", () => {
  const link = document.createElement("a");
  const url = URL.createObjectURL(new Blob([active.text], { type: "text/plain;charset=utf-8" }));
  link.href = url;
  link.download = active.name;
  document.body.append(link);
  link.click();
  link.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
  active.saved = active.text;
  storeFile(active);
  updateFileLabel();
});
ui["close-file"].addEventListener("click", () => {
  if (active.text !== active.saved && !confirm(messages.closeEdited)) return;
  const index = files.indexOf(active);
  store(filePrefix + active.id, null);
  files.splice(index, 1);
  showFile(files[Math.min(index, files.length - 1)] ?? createFile(messages.untitled, ""), true);
});
ui.run.addEventListener("click", beginRun);
ui.stop.addEventListener("click", stopRun);
ui["clear-output"].addEventListener("click", clearOutput);
ui["show-error"].addEventListener("click", goToError);
ui["program-input"].addEventListener("submit", event => {
  event.preventDefault();
  if (!run || !prompt) return;
  const text = prompt === "number" ? ui.answer.value : "";
  if (encoder.encode(text).length > maxInputBytes) {
    setStatus("inputTooBig", "error");
    return;
  }
  if (prompt === "number" && run.echoCharacters + text.length + 3 <= maxEchoCharacters) {
    appendOutput(`> ${text}\n`);
    run.echoCharacters += text.length + 3;
  }
  hideInput();
  setStatus("running");
  worker.postMessage({ kind: "answer", text });
});
document.addEventListener("keydown", event => {
  if (event.key === "F5" && !event.ctrlKey && !event.metaKey && !event.altKey && !event.shiftKey) {
    event.preventDefault();
    if (!event.repeat) beginRun();
  }
});

showFile(restoreFiles());
const url = new URL(location.href);
const lesson = url.searchParams.get("lesson");
if (lesson && /^\d+$/.test(lesson) && Object.hasOwn(examples, `lesson-${lesson}`)) {
  openExample(`lesson-${lesson}`);
  url.searchParams.delete("lesson");
  history.replaceState(history.state, "", url);
}
ui.source.closest(".code-editor").classList.add("has-highlights");
for (const id of ["examples", "new-file", "open-file", "save-file", "close-file", "run"]) {
  ui[id].disabled = false;
}
