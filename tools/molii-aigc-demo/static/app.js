(() => {
  "use strict";

  const $ = (selector, root = document) => root.querySelector(selector);
  const $$ = (selector, root = document) => Array.from(root.querySelectorAll(selector));
  const TERMINAL = new Set(["succeeded", "failed", "canceled", "cancelled"]);
  const labels = {
    model: "模型", prompt: "提示词", content: "内容序列", generate_audio: "生成音频",
    resolution: "分辨率", ratio: "画面比例", duration: "时长（秒）", watermark: "添加水印",
    tools: "联网搜索", aspect_ratio: "画面比例", quality: "质量档位", n: "输出数量", image: "输入图片",
    images: "输入图片列表", video: "输入视频", url: "公开素材 URL", asset_type: "素材类型",
    name: "素材名称", id: "素材 ID"
  };
  const operationLabels = {
    "seedance.video.generate": "视频生成", "seedance.asset.create": "创建素材",
    "seedance.asset.get": "查询素材", "seedance.asset.delete": "删除素材",
    "grok.image.generate": "图片生成", "grok.image.edit": "图片编辑",
    "grok.video.generate": "视频生成", "grok.video.edit": "视频编辑"
  };
  const providerViews = [
    { id: "seedance", label: "Seedance", subtitle: "视频生成" },
    { id: "grok-image", label: "Grok Image", subtitle: "图片生成 / 编辑" },
    { id: "grok-video", label: "Grok Video", subtitle: "视频生成 / 编辑" },
    { id: "seedance-assets", label: "Seedance", subtitle: "临时素材" }
  ];

  const state = {
    csrf: "", environments: [], activeEnvironmentId: "", catalog: [], providerView: "seedance",
    modelId: "doubao-seedance-2-0-260128", operationId: "seedance.video.generate", values: {},
    content: [{ type: "text", text: "", role: "" }], preview: null, previewFormat: "json",
    runs: [], selectedRun: null, previewTimer: 0, pollTimer: 0, historyTimer: 0, versionTimer: 0, instanceId: "", previewSequence: 0,
    loading: { bootstrap: true, preview: false, submit: false, history: false }
  };

  const fallbackCatalog = [
    seedanceModel("doubao-seedance-2-0-260128", "Seedance 2.0", ["480p", "720p", "1080p", "4k"]),
    seedanceModel("doubao-seedance-2-0-fast-260128", "Seedance 2.0 Fast", ["480p", "720p"]),
    grokImageModel("grok-imagine-image", "Grok Imagine Image"),
    grokImageModel("grok-imagine-image-quality", "Grok Imagine Image Quality"),
    grokImageModel("grok-imagine-image-2.0", "Grok Imagine Image 2.0"),
    grokVideoModel("grok-imagine-video", "Grok Imagine Video", false, true),
    grokVideoModel("grok-imagine-video-1.5", "Grok Imagine Video 1.5", true, false),
  ];

  function field(name, type, options = {}) { return { name, type, label: labels[name], ...options }; }
  function assetOperations() {
    return [
      { id: "seedance.asset.create", label: "创建临时素材", method: "POST", path: "/v1/assets", fields: [field("url", "url", { required: true }), field("asset_type", "select", { required: true, options: ["image", "video", "audio"] }), field("name", "text", { required: true, maximum: 80 })] },
      { id: "seedance.asset.get", label: "查询临时素材", method: "GET", path: "/v1/assets/{id}", fields: [field("id", "text", { required: true })] },
      { id: "seedance.asset.delete", label: "删除临时素材", method: "DELETE", path: "/v1/assets/{id}", fields: [field("id", "text", { required: true })] }
    ];
  }
  function seedanceModel(id, label, resolutions) {
    const generate = { id: "seedance.video.generate", label: "视频生成", method: "POST", path: "/v1/video/generations", async: true, generation: true, fields: [
      field("model", "select", { required: true }), field("prompt", "textarea"), field("content", "array", { item_type: "seedance_content" }),
      field("generate_audio", "boolean", { default: true }), field("resolution", "select", { default: "720p", options: resolutions }),
      field("ratio", "select", { default: "adaptive", options: ["16:9", "4:3", "1:1", "3:4", "9:16", "21:9", "adaptive"] }),
      field("duration", "integer", { default: 5, minimum: -1, maximum: 15 }), field("watermark", "boolean", { default: false }),
      field("tools", "array", { item_type: "web_search" })
    ] };
    return { id, label, provider: "seedance", kind: "video", operations: [generate, ...assetOperations()] };
  }
  function grokImageModel(id, label) {
    const common = [field("model", "select", { required: true }), field("prompt", "textarea", { required: true, maximum: 10000 }), field("aspect_ratio", "select", { default: "16:9", options: ["1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3", "2:1", "1:2", "19.5:9", "9:19.5", "20:9", "9:20", "auto"] }), field("resolution", "select", { default: "1k", options: ["1k", "2k"] })];
    if (id === "grok-imagine-image-2.0") common.push(field("quality", "select", { default: "medium", options: ["low", "medium"] }));
    common.push(field("n", "integer", { default: 1, minimum: 1, maximum: 4 }));
    return { id, label, provider: "grok", kind: "image", operations: [
      { id: "grok.image.generate", label: "图片生成", method: "POST", path: "/v1/images/generations", generation: true, fields: common },
      { id: "grok.image.edit", label: "图片编辑", method: "POST", path: "/v1/images/edits", generation: true, fields: [...common, field("image", "media"), field("images", "array", { item_type: "media" })] }
    ] };
  }
  function grokVideoModel(id, label, imageRequired, editable) {
    const resolutions = imageRequired ? ["480p", "720p", "1080p"] : ["480p", "720p"];
    const operations = [{ id: "grok.video.generate", label: "视频生成", method: "POST", path: "/v1/videos", async: true, generation: true, fields: [field("model", "select", { required: true }), field("prompt", "textarea", { required: true, maximum: 10000 }), field("image", "media", { required: imageRequired }), field("duration", "integer", { default: 5, minimum: 1, maximum: 15 }), field("aspect_ratio", "text", { default: "16:9", maximum: 32 }), field("resolution", "select", { default: "480p", options: resolutions })] }];
    if (editable) operations.push({ id: "grok.video.edit", label: "视频编辑", method: "POST", path: "/v1/videos/edits", async: true, generation: true, fields: [field("model", "select", { required: true }), field("prompt", "textarea", { required: true, maximum: 10000 }), field("video", "media", { required: true })] });
    return { id, label, provider: "grok", kind: "video", operations };
  }

  class APIError extends Error {
    constructor(message, status, payload) { super(message); this.name = "APIError"; this.status = status; this.payload = payload; }
  }
  async function api(path, options = {}) {
    const method = (options.method || "GET").toUpperCase();
    const headers = new Headers(options.headers || {});
    headers.set("Accept", "application/json");
    if (state.csrf && !["GET", "HEAD", "OPTIONS"].includes(method)) headers.set("X-CSRF-Token", state.csrf);
    const request = { method, headers, credentials: "same-origin", cache: "no-store", signal: options.signal };
    if (options.body !== undefined) {
      headers.set("Content-Type", "application/json");
      request.body = JSON.stringify(options.body);
    }
    let response;
    try { response = await fetch(path, request); }
    catch (error) { throw new APIError(error.name === "AbortError" ? "请求已取消" : "无法连接到 Demo 服务", 0, null); }
    const contentType = response.headers.get("content-type") || "";
    let payload = null;
    try { payload = contentType.includes("json") ? await response.json() : await response.text(); } catch (_) { /* empty response */ }
    if (!response.ok) {
      const message = payload?.message || payload?.error?.message || payload?.error || (typeof payload === "string" && payload) || `请求失败（HTTP ${response.status}）`;
      throw new APIError(String(message), response.status, payload);
    }
    return payload;
  }
  function unwrap(payload) {
    if (payload && typeof payload === "object" && !Array.isArray(payload) && payload.data !== undefined) return payload.data;
    return payload;
  }

  async function monitorInstance() {
    try {
      const response = await fetch("/api/version", { credentials: "same-origin", cache: "no-store" });
      if (response.ok) {
        const data = await response.json();
        const next = String(data?.instance_id || "");
        if (state.instanceId && next && state.instanceId !== next) { window.location.reload(); return; }
        if (next) state.instanceId = next;
      }
    } catch (_) { /* The watcher may be rebuilding; retry when the service returns. */ }
    state.versionTimer = window.setTimeout(monitorInstance, 2500);
  }

  function currentEnvironment() { return state.environments.find(item => String(item.id) === String(state.activeEnvironmentId)); }
  function visibleModels() {
    const models = state.catalog.length ? state.catalog : fallbackCatalog;
    if (state.providerView === "seedance" || state.providerView === "seedance-assets") return models.filter(model => model.provider === "seedance");
    if (state.providerView === "grok-image") return models.filter(model => model.provider === "grok" && model.kind === "image");
    return models.filter(model => model.provider === "grok" && model.kind === "video");
  }
  function currentModel() { return visibleModels().find(model => model.id === state.modelId) || visibleModels()[0]; }
  function visibleOperations(model = currentModel()) {
    if (!model) return [];
    if (state.providerView === "seedance-assets") return (model.operations || []).filter(op => op.id.startsWith("seedance.asset."));
    if (state.providerView === "seedance") return (model.operations || []).filter(op => op.id === "seedance.video.generate");
    return model.operations || [];
  }
  function currentOperation() { return visibleOperations().find(op => op.id === state.operationId) || visibleOperations()[0]; }
  function valuesKey() { return `${state.modelId}:${state.operationId}`; }
  function values() {
    if (!state.values[valuesKey()]) state.values[valuesKey()] = defaultValues(currentOperation());
    return state.values[valuesKey()];
  }
  function defaultValues(operation) {
    const result = {};
    for (const item of operation?.fields || []) {
      if (item.name === "model" || item.name === "content") continue;
      if (item.default !== undefined) result[item.name] = item.default;
      else if (item.type === "boolean") result[item.name] = false;
      else if (item.name === "tools") result[item.name] = false;
      else result[item.name] = "";
    }
    return result;
  }
  function ensureSelection() {
    const models = visibleModels();
    if (!models.some(model => model.id === state.modelId)) state.modelId = models[0]?.id || "";
    const operations = visibleOperations();
    if (!operations.some(op => op.id === state.operationId)) state.operationId = operations[0]?.id || "";
  }

  function renderSelectors() {
    $("#provider-tabs").innerHTML = providerViews.map(view => `<button type="button" class="${view.id === state.providerView ? "active" : ""}" data-provider="${view.id}" aria-pressed="${view.id === state.providerView}"><strong>${escapeHTML(view.label)}</strong><span>${escapeHTML(view.subtitle)}</span></button>`).join("");
    $("#model-tabs").innerHTML = visibleModels().map(model => `<button type="button" role="tab" class="${model.id === state.modelId ? "active" : ""}" data-model="${escapeAttr(model.id)}" aria-selected="${model.id === state.modelId}" title="${escapeAttr(model.id)}">${escapeHTML(model.label || model.id)}</button>`).join("");
    $("#operation-tabs").innerHTML = visibleOperations().map(op => `<button type="button" role="tab" class="${op.id === state.operationId ? "active" : ""}" data-operation="${escapeAttr(op.id)}" aria-selected="${op.id === state.operationId}">${escapeHTML(operationLabels[op.id] || op.label || op.id)}</button>`).join("");
  }

  function renderForm() {
    const operation = currentOperation();
    const currentValues = values();
    const fields = (operation?.fields || []).filter(item => item.name !== "model" && item.name !== "content");
    $("#parameter-fields").innerHTML = fields.map(item => renderField(item, currentValues[item.name])).join("");
    const isSeedanceGeneration = operation?.id === "seedance.video.generate";
    $("#content-editor").hidden = !isSeedanceGeneration;
    if (isSeedanceGeneration) renderContent();
    updateSubmitState();
  }

  function renderField(item, value) {
    const name = escapeAttr(item.name);
    const label = escapeHTML(labels[item.name] || item.label || item.name);
    const required = item.required ? " required" : "";
    const requiredMark = item.required ? " <span aria-hidden=\"true\">*</span>" : "";
    const description = item.description ? `<small>${escapeHTML(localizeDescription(item.description))}</small>` : "";
    const wide = item.type === "textarea" || item.type === "url" || item.type === "media" || item.type === "array" ? " wide" : "";
    if (item.type === "boolean" || item.name === "tools") {
      const checked = Boolean(value) ? " checked" : "";
      const help = item.name === "tools" ? "使用 web_search 工具" : (item.name === "generate_audio" ? "随视频生成音轨" : "在生成结果中添加水印");
      return `<label class="field checkbox-field${wide}"><span>${label}<small>${help}</small></span><span class="switch"><input type="checkbox" name="${name}"${checked}><i aria-hidden="true"></i></span></label>`;
    }
    if (item.type === "select") {
      const options = item.options || [];
      return `<label class="field${wide}"><span>${label}${requiredMark}</span><select name="${name}"${required}>${options.map(option => `<option value="${escapeAttr(option)}"${String(value) === String(option) ? " selected" : ""}>${escapeHTML(optionLabel(item.name, option))}</option>`).join("")}</select>${description}</label>`;
    }
    if (item.type === "textarea") {
      return `<label class="field wide"><span>${label}${requiredMark}</span><textarea name="${name}" maxlength="${item.maximum || 10000}"${required} placeholder="描述你希望生成的画面、动作、镜头与风格">${escapeHTML(value ?? "")}</textarea>${description}</label>`;
    }
    if (item.type === "media") return renderMediaField(item, value);
    if (item.type === "array" && item.item_type === "media") {
      return `<label class="field wide"><span>${label}${requiredMark}</span><textarea name="${name}"${required} placeholder="每行一个公开 URL">${escapeHTML(Array.isArray(value) ? value.map(mediaDisplayValue).join("\n") : (value || ""))}</textarea><small>每行一个公开 URL，图片编辑合计支持 1–3 张</small></label>`;
    }
    const inputType = item.type === "integer" ? "number" : (item.type === "url" ? "url" : "text");
    const min = item.minimum !== undefined ? ` min="${item.minimum}"` : "";
    const max = item.maximum !== undefined ? ` max="${item.maximum}"` : "";
    const special = item.name === "duration" && state.operationId === "seedance.video.generate" ? `<small>-1 表示智能时长，或填写 4–15</small>` : description;
    return `<label class="field${wide}"><span>${label}${requiredMark}</span><input type="${inputType}" name="${name}" value="${escapeAttr(value ?? "")}"${min}${max}${required} placeholder="${escapeAttr(placeholderFor(item.name))}">${special}</label>`;
  }

  function renderMediaField(item, value) {
    const normalized = normalizeMedia(value);
    return `<div class="field wide media-field" data-media-field="${escapeAttr(item.name)}"><span>${escapeHTML(labels[item.name] || item.label || item.name)}${item.required ? " *" : ""}</span><div class="media-input-row"><input type="url" name="${escapeAttr(item.name)}" value="${escapeAttr(normalized.value)}"${item.required ? " required" : ""} placeholder="https://…"></div><small>Demo 不保存上传文件正文，请提供可访问的公开 URL</small></div>`;
  }

  function renderContent() {
    if (!state.content.length) state.content.push({ type: "text", text: "", role: "" });
    $("#content-items").innerHTML = state.content.map((item, index) => {
      const isText = item.type === "text";
      const roles = rolesFor(item.type);
      const value = isText ? item.text : item.url;
      return `<div class="content-item" data-content-index="${index}"><span class="drag-handle" aria-hidden="true">⋮⋮</span><label><span class="sr-only">内容类型</span><select data-content-key="type"><option value="text"${item.type === "text" ? " selected" : ""}>文本</option><option value="image_url"${item.type === "image_url" ? " selected" : ""}>图片 URL</option><option value="video_url"${item.type === "video_url" ? " selected" : ""}>视频 URL</option><option value="audio_url"${item.type === "audio_url" ? " selected" : ""}>音频 URL</option></select></label><label><span class="sr-only">内容值</span>${isText ? `<textarea data-content-key="text" placeholder="提示文本">${escapeHTML(value || "")}</textarea>` : `<input data-content-key="url" type="text" value="${escapeAttr(value || "")}" placeholder="https://… 或 asset://…">`}</label><label class="content-role"><span class="sr-only">素材角色</span><select data-content-key="role"${isText ? " disabled" : ""}>${roles.map(role => `<option value="${escapeAttr(role.value)}"${role.value === (item.role || "") ? " selected" : ""}>${escapeHTML(role.label)}</option>`).join("")}</select></label><button class="content-remove" type="button" data-remove-content aria-label="删除第 ${index + 1} 项"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 7h16M9 7V4h6v3m-8 0 1 13h8l1-13M10 11v5M14 11v5"/></svg></button></div>`;
    }).join("");
  }

  function rolesFor(type) {
    if (type === "image_url") return [{ value: "first_frame", label: "首帧" }, { value: "last_frame", label: "尾帧" }, { value: "reference_image", label: "参考图片" }];
    if (type === "video_url") return [{ value: "reference_video", label: "参考视频" }];
    if (type === "audio_url") return [{ value: "reference_audio", label: "参考音频" }];
    return [{ value: "", label: "无角色" }];
  }

  function collectPayload() {
    const operation = currentOperation();
    const form = $("#request-form");
    const formData = new FormData(form);
    const payload = {};
    if ((operation?.fields || []).some(item => item.name === "model")) payload.model = state.modelId;
    for (const item of operation?.fields || []) {
      if (item.name === "model" || item.name === "content") continue;
      if (item.type === "boolean") { payload[item.name] = Boolean(form.elements[item.name]?.checked); continue; }
      if (item.name === "tools") { if (form.elements[item.name]?.checked) payload.tools = [{ type: "web_search" }]; continue; }
      let raw = formData.get(item.name);
      if (item.type === "integer" && raw !== "") raw = Number(raw);
      if (item.type === "media") raw = collectMedia(item.name, raw);
      if (item.type === "array" && item.item_type === "media") raw = String(raw || "").split(/\n+/).map(value => value.trim()).filter(Boolean).map(value => mediaFromString(value));
      if (raw !== "" && !(Array.isArray(raw) && raw.length === 0)) payload[item.name] = raw;
    }
    if (operation?.id === "seedance.video.generate") {
      payload.content = state.content.map(item => {
        if (item.type === "text") return { type: "text", text: (item.text || "").trim() };
        const mediaKey = item.type;
        return { type: item.type, [mediaKey]: { url: (item.url || "").trim() }, role: item.role || defaultRole(item.type) };
      }).filter(item => item.type === "text" ? item.text : item[item.type]?.url);
    }
    return payload;
  }
  function collectMedia(name, raw) {
    const trimmed = String(raw || "").trim();
    return trimmed ? { url: trimmed } : "";
  }
  function mediaFromString(value) { return { url: value }; }
  function normalizeMedia(value) {
    if (value && typeof value === "object") return { value: value.url || "" };
    return { value: String(value || "") };
  }
  function mediaDisplayValue(value) { return typeof value === "string" ? value : value?.url || ""; }
  function defaultRole(type) { return type === "image_url" ? "first_frame" : type === "video_url" ? "reference_video" : "reference_audio"; }

  function validatePayload(payload) {
    const operation = currentOperation();
    const errors = [];
    for (const item of operation?.fields || []) {
      if (!item.required || item.name === "model") continue;
      const value = payload[item.name];
      if (value === undefined || value === null || value === "" || (Array.isArray(value) && !value.length)) errors.push(`请填写${labels[item.name] || item.label || item.name}`);
    }
    for (const item of operation?.fields || []) {
      const value = payload[item.name];
      if (value === undefined || value === null || value === "") continue;
      if (item.type === "integer" && (!Number.isInteger(value) || (item.minimum !== undefined && value < item.minimum) || (item.maximum !== undefined && value > item.maximum))) errors.push(`${labels[item.name] || item.label || item.name}超出允许范围`);
      if (["text", "textarea"].includes(item.type) && item.maximum && String(value).length > item.maximum) errors.push(`${labels[item.name] || item.label || item.name}不能超过 ${item.maximum} 个字符`);
    }
    if (operation?.id === "grok.image.edit") {
      const inputCount = (payload.image ? 1 : 0) + (payload.images?.length || 0);
      if (inputCount < 1 || inputCount > 3) errors.push("图片编辑需要 1–3 张输入图片");
    }
    if (operation?.id === "seedance.video.generate") {
      if (!String(payload.prompt || "").trim() && !payload.content?.some(item => item.type === "text" || item.type === "image_url" || item.type === "video_url")) errors.push("提示词、参考图片或参考视频至少填写一项");
      const counts = payload.content.reduce((acc, item) => { acc[item.type] = (acc[item.type] || 0) + 1; return acc; }, {});
      if ((counts.image_url || 0) > 9 || (counts.video_url || 0) > 3 || (counts.audio_url || 0) > 3) errors.push("参考素材数量超过限制");
      const frame = payload.content.some(item => ["first_frame", "last_frame"].includes(item.role));
      const reference = payload.content.some(item => ["reference_image", "reference_video", "reference_audio"].includes(item.role));
      if (frame && reference) errors.push("帧模式与多模态参考素材不能混用");
      const firstCount = payload.content.filter(item => item.role === "first_frame").length;
      const lastCount = payload.content.filter(item => item.role === "last_frame").length;
      if (frame && (firstCount !== 1 || lastCount > 1)) errors.push("帧模式需要一张首帧，且最多一张尾帧");
      const audioCount = counts.audio_url || 0;
      if (audioCount && !(counts.image_url || counts.video_url)) errors.push("参考音频需要同时提供参考图片或视频");
      if (payload.duration !== -1 && (payload.duration < 4 || payload.duration > 15)) errors.push("Seedance 时长必须为 -1 或 4–15 秒");
    }
    return errors;
  }

  function requestDescriptor() {
    const payload = collectPayload();
    const environmentId = state.activeEnvironmentId;
    return {
      environment_id: environmentId,
      model: state.modelId,
      operation: state.operationId,
      parameters: payload
    };
  }

  async function updatePreview(immediate = false) {
    clearTimeout(state.previewTimer);
    if (!immediate) { state.previewTimer = window.setTimeout(() => updatePreview(true), 280); return; }
    const descriptor = requestDescriptor();
    const errors = validatePayload(descriptor.parameters);
    if (!state.activeEnvironmentId) {
      state.preview = localPreview(descriptor);
      renderPreview("请选择环境以获取服务端预览", "error");
      return;
    }
    if (errors.length) {
      state.preview = localPreview(descriptor);
      renderPreview(errors[0], "error");
      return;
    }
    const sequence = ++state.previewSequence;
    state.loading.preview = true;
    setPreviewStatus("正在预估", "loading");
    try {
      const response = unwrap(await api("/api/preview", { method: "POST", body: descriptor }));
      if (sequence !== state.previewSequence) return;
      state.preview = response || localPreview(descriptor);
      renderPreview();
    } catch (error) {
      if (sequence !== state.previewSequence) return;
      state.preview = localPreview(descriptor);
      renderPreview(error.message, "error");
    } finally { if (sequence === state.previewSequence) state.loading.preview = false; }
  }

  function localPreview(descriptor) {
    const operation = currentOperation();
    const env = currentEnvironment();
    const path = interpolatePath(operation?.path || "", descriptor.parameters);
    const body = ["GET", "DELETE"].includes(operation?.method) ? null : descriptor.parameters;
    const url = `${(env?.base_url || "${MOLII_BASE_URL}").replace(/\/$/, "")}${path}`;
    const curlParts = [`curl -X ${operation?.method || "POST"}`, `'${url}'`, `-H 'Authorization: Bearer $MOLII_API_KEY'`, `-H 'Content-Type: application/json'`];
    if (body) curlParts.push(`--data '${JSON.stringify(body)}'`);
    return { request_json: body || {}, curl: curlParts.join(" \\\n  "), estimated_amount: null, currency: "CNY" };
  }

  function renderPreview(note = "", kind = "ready") {
    const preview = state.preview || {};
    const json = parseMaybeJSON(preview.request_json ?? preview.body ?? preview.prepared?.body ?? preview.request ?? preview.parameters ?? collectPayload());
    const curl = preview.curl || preview.curl_command || localPreview(requestDescriptor()).curl;
    const text = state.previewFormat === "curl" ? String(curl || "") : JSON.stringify(json ?? {}, null, 2);
    $("#request-preview code").innerHTML = state.previewFormat === "json" ? syntaxHighlight(text) : escapeHTML(text);
    $("#preview-empty").hidden = Boolean(text);
    const estimated = firstDefined(preview.estimated_amount, preview.estimated_cost, preview.estimate?.amount, preview.billing?.amount, preview.estimated_billing?.amount);
    renderBilling({ estimated, currency: preview.currency || preview.estimate?.currency || preview.billing?.currency || "CNY", formula: preview.formula || preview.estimate?.formula || preview.estimate?.reason || preview.billing?.formula, actual: state.selectedRun?.actual_amount, delta: state.selectedRun?.delta_amount });
    setPreviewStatus(note || "预览已更新", kind);
  }

  function renderBilling({ estimated, actual, delta, currency = "CNY", formula } = {}) {
    $("#estimated-cost").textContent = estimated === undefined || estimated === null || estimated === "" ? "待同步" : money(estimated, currency);
    $("#actual-cost").textContent = actual === undefined || actual === null || actual === "" ? "待同步" : money(actual, currency);
    $("#cost-delta").textContent = delta === undefined || delta === null || delta === "" ? "—" : money(delta, currency, true);
    $("#billing-formula").textContent = formula || "价格由所选环境的 /api/pricing 动态计算";
    const ready = estimated !== undefined && estimated !== null && estimated !== "";
    $("#billing-state").textContent = ready ? "已预估" : "待预估";
    $("#billing-state").classList.toggle("ready", ready);
  }

  async function submitRun(event) {
    event.preventDefault();
    hideMessage("#form-message");
    const descriptor = requestDescriptor();
    const errors = validatePayload(descriptor.parameters);
    if (!state.activeEnvironmentId) errors.unshift("请先添加并选择一个环境");
    if (errors.length) { showMessage("#form-message", errors.join("；")); return; }
    setSubmitting(true);
    try {
      const run = normalizeRun(unwrap(await api("/api/runs", { method: "POST", body: descriptor })));
      state.selectedRun = run;
      upsertRun(run);
      renderHistory(); renderResult(run); startPolling(run);
      toast("请求已提交", "success");
    } catch (error) { showMessage("#form-message", error.message); toast(error.message, "error"); }
    finally { setSubmitting(false); }
  }

  async function loadBootstrap() {
    state.loading.bootstrap = true;
    try {
      const raw = await api("/api/bootstrap");
      const data = unwrap(raw) || {};
      state.csrf = data.csrf_token || data.csrfToken || raw?.csrf_token || "";
      state.environments = arrayFrom(data.environments || data.environment_list);
      state.activeEnvironmentId = String(data.active_environment_id || data.selected_environment_id || data.active_environment?.id || "");
      state.catalog = normalizeCatalog(data.catalog || data.models || data.catalog?.models);
      if (!state.catalog.length) state.catalog = fallbackCatalog;
      if (data.runs || data.history) state.runs = arrayFrom(data.runs || data.history).map(normalizeRun);
      renderEnvironments(); ensureSelection(); renderSelectors(); renderForm(); await updatePreview(true);
      if (!state.runs.length) await loadHistory(); else renderHistory();
    } catch (error) {
      state.catalog = fallbackCatalog;
      renderEnvironments(); ensureSelection(); renderSelectors(); renderForm(); renderHistory();
      setPreviewStatus("服务暂不可用", "error"); toast(error.message, "error");
    } finally { state.loading.bootstrap = false; updateSubmitState(); }
  }

  async function loadEnvironments() {
    try {
      const data = unwrap(await api("/api/environments"));
      state.environments = arrayFrom(data?.environments || data);
      if (!state.environments.some(item => String(item.id) === String(state.activeEnvironmentId))) state.activeEnvironmentId = "";
      renderEnvironments(); updatePreview();
    } catch (error) { toast(error.message, "error"); }
  }
  function renderEnvironments() {
    const select = $("#environment-select");
    select.innerHTML = state.environments.length ? `${state.activeEnvironmentId ? "" : '<option value="">请选择环境</option>'}${state.environments.map(item => `<option value="${escapeAttr(item.id)}"${String(item.id) === String(state.activeEnvironmentId) ? " selected" : ""}>${escapeHTML(item.name)} · ${escapeHTML(shortHost(item.base_url))}</option>`).join("")}` : '<option value="">尚未配置环境</option>';
    select.value = state.activeEnvironmentId;
    const hasEnv = Boolean(currentEnvironment());
    $("#test-environment").disabled = !hasEnv; $("#edit-environment").disabled = !hasEnv;
    $("#environment-status").textContent = hasEnv ? `${currentEnvironment().key_masked || "密钥已安全保存"} · 服务端持久化` : "配置环境后即可发送测试请求";
  }
  async function selectEnvironment(id) {
    if (!id) return;
    const previous = state.activeEnvironmentId; state.activeEnvironmentId = String(id); renderEnvironments();
    try { await api(`/api/environments/${encodeURIComponent(id)}/select`, { method: "POST" }); $("#environment-status").textContent = "环境已切换并保存"; $("#environment-status").className = "environment-status success"; updatePreview(true); await loadHistory(); }
    catch (error) { state.activeEnvironmentId = previous; renderEnvironments(); toast(error.message, "error"); }
  }

  function openEnvironmentDialog(mode) {
    const editing = mode === "edit";
    const env = editing ? currentEnvironment() : null;
    $("#environment-dialog-title").textContent = editing ? "编辑环境" : "添加环境";
    $("#environment-id").value = env?.id || ""; $("#environment-name").value = env?.name || ""; $("#environment-url").value = env?.base_url || ""; $("#environment-key").value = "";
    $("#environment-key").type = "password"; $("#toggle-key").textContent = "显示";
    $("#environment-key").required = !editing; $("#key-hint").textContent = editing ? "留空表示保留现有密钥" : "密钥仅发送到 Demo 服务并加密保存";
    $("#delete-environment").hidden = !editing; hideMessage("#environment-form-message");
    $("#environment-dialog").showModal(); window.setTimeout(() => $("#environment-name").focus(), 0);
  }
  async function saveEnvironment(event) {
    event.preventDefault();
    const id = $("#environment-id").value;
    const body = { name: $("#environment-name").value.trim(), base_url: $("#environment-url").value.trim() };
    const key = $("#environment-key").value.trim(); if (key) body.api_key = key;
    if (!body.name || !body.base_url || (!id && !key)) { showMessage("#environment-form-message", "请填写名称、Base URL 和 API Key"); return; }
    setButtonBusy("#save-environment", true, "保存中");
    try {
      const saved = unwrap(await api(id ? `/api/environments/${encodeURIComponent(id)}` : "/api/environments", { method: id ? "PUT" : "POST", body }));
      let savedId = saved?.id || id;
      await loadEnvironments();
      if (!savedId) savedId = state.environments.find(item => item.name === body.name && item.base_url === body.base_url)?.id;
      if (savedId) await selectEnvironment(savedId);
      $("#environment-key").value = "";
      $("#environment-dialog").close(); toast(id ? "环境已更新" : "环境已添加", "success");
    } catch (error) { showMessage("#environment-form-message", error.message); }
    finally { setButtonBusy("#save-environment", false, "保存环境"); }
  }
  async function deleteEnvironment() {
    const id = $("#environment-id").value; const env = state.environments.find(item => String(item.id) === id);
    if (!id || !window.confirm(`确定删除环境“${env?.name || id}”吗？历史运行不会被删除。`)) return;
    setButtonBusy("#delete-environment", true, "删除中");
    try { await api(`/api/environments/${encodeURIComponent(id)}`, { method: "DELETE" }); $("#environment-dialog").close(); state.activeEnvironmentId = ""; await loadEnvironments(); toast("环境已删除", "success"); }
    catch (error) { showMessage("#environment-form-message", error.message); }
    finally { setButtonBusy("#delete-environment", false, "删除环境"); }
  }
  async function testEnvironment() {
    const id = state.activeEnvironmentId; if (!id) return;
    const button = $("#test-environment"); button.disabled = true; $("#environment-status").textContent = "正在测试连接…"; $("#environment-status").className = "environment-status";
    try {
      const result = unwrap(await api(`/api/environments/${encodeURIComponent(id)}/test`, { method: "POST" }));
      $("#environment-status").textContent = result?.message || `连接正常${result?.latency_ms ? ` · ${result.latency_ms} ms` : ""}`; $("#environment-status").className = "environment-status success"; toast("环境连接正常", "success");
    } catch (error) { $("#environment-status").textContent = error.message; $("#environment-status").className = "environment-status error"; }
    finally { button.disabled = false; }
  }

  async function loadHistory(silent = false) {
    if (state.loading.history) return;
    state.loading.history = true;
    if (!silent && !state.runs.length) $("#history-list").innerHTML = '<div class="skeleton-card"></div><div class="skeleton-card"></div>';
    try {
      const query = state.activeEnvironmentId ? `?environment_id=${encodeURIComponent(state.activeEnvironmentId)}&limit=100` : "?limit=100";
      const data = unwrap(await api(`/api/runs${query}`));
      state.runs = arrayFrom(data?.runs || data?.items || data).map(normalizeRun);
      renderHistory();
    } catch (error) { if (!silent) { renderHistory(error.message); toast(error.message, "error"); } }
    finally { state.loading.history = false; }
  }
  function renderHistory(error = "") {
    const needle = $("#history-search").value.trim().toLowerCase();
    const runs = state.runs.filter(run => !needle || [run.model, run.id, run.request_id, run.operation].some(value => String(value || "").toLowerCase().includes(needle)));
    const list = $("#history-list");
    if (!runs.length) {
      list.innerHTML = `<div class="empty-state"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 5h16v14H4zM8 9h8M8 13h5"/></svg><strong>${error ? "加载失败" : needle ? "没有匹配结果" : "暂无运行记录"}</strong><p>${escapeHTML(error || (needle ? "试试其他关键词" : "提交一次请求后会在这里保存完整历史"))}</p></div>`;
      return;
    }
    list.innerHTML = runs.map(run => `<button class="history-card${state.selectedRun?.id === run.id ? " active" : ""}" type="button" data-run-id="${escapeAttr(run.id)}"><div class="history-card-top"><strong title="${escapeAttr(run.model)}">${escapeHTML(shortModel(run.model))}</strong><span class="run-status ${escapeAttr(statusClass(run.status))}">${escapeHTML(statusLabel(run.status))}</span></div><div class="history-card-meta"><span>${escapeHTML(operationLabels[run.operation] || run.operation || "API 请求")}</span><time>${escapeHTML(formatDate(run.created_at))}</time></div><div class="history-card-cost"><span>${escapeHTML(shortId(run.request_id || run.id))}</span><strong>${run.actual_amount != null ? money(run.actual_amount, run.currency) : run.estimated_amount != null ? `预估 ${money(run.estimated_amount, run.currency)}` : "待计费"}</strong></div></button>`).join("");
  }

  async function selectRun(id, openDetails = false) {
    let run = state.runs.find(item => String(item.id) === String(id));
    if (!run) return;
    state.selectedRun = run; renderHistory(); renderResult(run);
    try {
      const detail = unwrap(await api(`/api/runs/${encodeURIComponent(id)}`));
      const detailedRun = normalizeRun(detail?.run || detail);
      detailedRun.exchanges = arrayFrom(detail?.exchanges || detailedRun.exchanges);
      state.selectedRun = detailedRun; upsertRun(detailedRun); renderHistory(); renderResult(detailedRun);
      if (openDetails) openRunDialog(detailedRun);
      if (!TERMINAL.has(normalizeStatus(detailedRun.status))) startPolling(detailedRun);
    } catch (error) { toast(error.message, "error"); }
  }

  function renderResult(run) {
    if (!run) return;
    const status = normalizeStatus(run.status); const progress = progressNumber(run);
    $("#result-subtitle").textContent = `${statusLabel(status)} · ${shortId(run.request_id || run.id)}`;
    const inProgress = !TERMINAL.has(status);
    $("#run-progress").hidden = !inProgress && progress <= 0;
    $("#progress-label").textContent = statusLabel(status); $("#progress-value").textContent = `${Math.round(progress)}%`;
    const progressTrack = $(".progress-track");
    progressTrack.setAttribute("aria-valuenow", String(Math.round(progress)));
    progressTrack.className = `progress-track progress-${Math.round(progress / 5) * 5}`;
    $("#cancel-run").hidden = !inProgress; $("#cancel-run").disabled = Boolean(run.cancel_requested);
    renderBilling({ estimated: run.estimated_amount, actual: run.actual_amount, delta: run.delta_amount, currency: run.currency || "CNY", formula: billingFormula(run) });
    renderMedia(run);
  }
  function renderMedia(run) {
    const container = $("#result-media");
    const status = normalizeStatus(run.status);
    if (status === "failed" || status === "canceled" || status === "cancelled") { container.className = "result-media empty"; container.innerHTML = `<svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="9"/><path d="m9 9 6 6M15 9l-6 6"/></svg><span>${escapeHTML(run.error_message || (status === "failed" ? "生成失败" : "Demo 已停止轮询；上游任务可能仍在运行"))}</span>`; return; }
    const video = isVideoRun(run);
    const resultURLs = mediaURLs(run);
    const urls = status === "succeeded" ? (video ? [`/api/runs/${encodeURIComponent(run.id)}/media`] : resultURLs.map((_, index) => `/api/runs/${encodeURIComponent(run.id)}/media?index=${index}`)) : resultURLs;
    if (!urls.length && status !== "succeeded") { container.className = "result-media empty"; container.innerHTML = '<span class="spinner spinner-dark"></span><span>等待媒体结果…</span>'; return; }
    if (!urls.length && status === "succeeded") urls.push(`/api/runs/${encodeURIComponent(run.id)}/media`);
    container.className = "result-media";
    container.innerHTML = `<div class="media-grid">${urls.map((url, index) => `<div class="media-item">${video ? `<video src="${escapeAttr(url)}" controls preload="metadata" aria-label="生成视频"></video>` : `<img src="${escapeAttr(url)}" alt="生成图片 ${index + 1}" loading="lazy">`}<div class="media-actions"><a href="${escapeAttr(mediaDownloadURL(run, url))}" download title="下载媒体" aria-label="下载第 ${index + 1} 个媒体"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3v12m0 0 4-4m-4 4-4-4M5 20h14"/></svg></a></div></div>`).join("")}</div>`;
  }
  function mediaDownloadURL(run, url) { return url.startsWith("/api/runs/") ? `${url}${url.includes("?") ? "&" : "?"}download=1` : url; }

  function startPolling(run) {
    clearTimeout(state.pollTimer);
    if (!run || TERMINAL.has(normalizeStatus(run.status))) return;
    state.pollTimer = window.setTimeout(async () => {
      try {
        const detail = unwrap(await api(`/api/runs/${encodeURIComponent(run.id)}`));
        const fresh = normalizeRun(detail?.run || detail); fresh.exchanges = arrayFrom(detail?.exchanges);
        state.selectedRun = fresh; upsertRun(fresh); renderResult(fresh); renderHistory();
        if (!TERMINAL.has(normalizeStatus(fresh.status))) startPolling(fresh); else { toast(statusLabel(fresh.status), fresh.status === "succeeded" ? "success" : "error"); loadHistory(true); }
      } catch (error) { state.pollTimer = window.setTimeout(() => startPolling(run), 3000); }
    }, 1800);
  }
  async function cancelRun() {
    const run = state.selectedRun; if (!run || TERMINAL.has(normalizeStatus(run.status))) return;
    $("#cancel-run").disabled = true;
    try { const fresh = normalizeRun(unwrap(await api(`/api/runs/${encodeURIComponent(run.id)}/cancel`, { method: "POST" }))); state.selectedRun = fresh; upsertRun(fresh); renderResult(fresh); renderHistory(); toast("已停止 Demo 轮询；上游任务不会被取消", "success"); }
    catch (error) { $("#cancel-run").disabled = false; toast(error.message, "error"); }
  }

  function openRunDialog(run) {
    $("#run-dialog-title").textContent = `运行 ${shortId(run.id)}`;
    const exchanges = arrayFrom(run.exchanges);
    $("#run-dialog-body").innerHTML = `<div class="run-summary"><div><span>状态</span><strong>${escapeHTML(statusLabel(run.status))}</strong></div><div><span>模型</span><strong>${escapeHTML(shortModel(run.model))}</strong></div><div><span>耗时</span><strong>${escapeHTML(runDuration(run))}</strong></div><div><span>实际费用</span><strong>${run.actual_amount != null ? money(run.actual_amount, run.currency) : "待同步"}</strong></div></div>${run.error_message ? `<div class="inline-message">${escapeHTML(run.error_message)}</div>` : ""}<div class="timeline">${exchanges.length ? exchanges.map(renderExchange).join("") : `<div class="empty-state"><strong>暂无交换记录</strong><p>服务端尚未保存该运行的请求或轮询日志</p></div>`}</div>`;
    $("#run-dialog").showModal();
  }
  function renderExchange(exchange, index) {
    const requestBody = parseMaybeJSON(exchange.request_body ?? exchange.request_body_json);
    const responseBody = parseMaybeJSON(exchange.response_body ?? exchange.response_body_json);
    const requestHeaders = parseMaybeJSON(exchange.request_headers ?? exchange.request_headers_json);
    const responseHeaders = parseMaybeJSON(exchange.response_headers ?? exchange.response_headers_json);
    const log = {
      request: { headers: requestHeaders ?? {}, body: requestBody ?? null },
      response: { status: exchange.response_status ?? null, headers: responseHeaders ?? {}, body: responseBody ?? null },
      error: exchange.error || undefined
    };
    return `<article class="timeline-item"><div class="timeline-head"><strong>${escapeHTML(exchange.kind || `交换 ${index + 1}`)}</strong><time>${escapeHTML(formatDate(exchange.started_at || exchange.created_at, true))}</time></div><div class="timeline-meta"><span>${escapeHTML(exchange.method || "HTTP")}</span>${exchange.response_status != null ? `<span>HTTP ${escapeHTML(exchange.response_status)}</span>` : ""}${exchange.duration_ms != null ? `<span>${escapeHTML(exchange.duration_ms)} ms</span>` : ""}<span>${escapeHTML(redactURL(exchange.url || ""))}</span></div><pre class="log-block">${escapeHTML(JSON.stringify(log, null, 2))}</pre></article>`;
  }

  function bindEvents() {
    $("#provider-tabs").addEventListener("click", event => { const button = event.target.closest("[data-provider]"); if (!button) return; persistFormValues(); state.providerView = button.dataset.provider; ensureSelection(); renderSelectors(); renderForm(); updatePreview(); });
    $("#model-tabs").addEventListener("click", event => { const button = event.target.closest("[data-model]"); if (!button) return; persistFormValues(); state.modelId = button.dataset.model; ensureSelection(); renderSelectors(); renderForm(); updatePreview(); });
    $("#operation-tabs").addEventListener("click", event => { const button = event.target.closest("[data-operation]"); if (!button) return; persistFormValues(); state.operationId = button.dataset.operation; renderSelectors(); renderForm(); updatePreview(); });
    $("#request-form").addEventListener("input", () => { persistFormValues(); hideMessage("#form-message"); updatePreview(); });
    $("#request-form").addEventListener("change", event => { if (event.target.matches("[data-media-kind]")) renderForm(); else persistFormValues(); updatePreview(); });
    $("#request-form").addEventListener("submit", submitRun);
    $("#add-content").addEventListener("click", () => { state.content.push({ type: "image_url", url: "", role: "first_frame" }); renderContent(); updatePreview(); });
    $("#content-items").addEventListener("input", updateContentItem);
    $("#content-items").addEventListener("change", updateContentItem);
    $("#content-items").addEventListener("click", event => { const button = event.target.closest("[data-remove-content]"); if (!button) return; const index = Number(button.closest("[data-content-index]").dataset.contentIndex); state.content.splice(index, 1); renderContent(); updatePreview(); });
    $("#environment-select").addEventListener("change", event => selectEnvironment(event.target.value));
    $("#add-environment").addEventListener("click", () => openEnvironmentDialog("add")); $("#edit-environment").addEventListener("click", () => openEnvironmentDialog("edit")); $("#test-environment").addEventListener("click", testEnvironment);
    $("#environment-form").addEventListener("submit", saveEnvironment); $("#delete-environment").addEventListener("click", deleteEnvironment);
    $$('[data-close-modal]').forEach(button => button.addEventListener("click", () => $("#environment-dialog").close()));
    $("#toggle-key").addEventListener("click", () => { const input = $("#environment-key"); input.type = input.type === "password" ? "text" : "password"; $("#toggle-key").textContent = input.type === "password" ? "显示" : "隐藏"; });
    $$("[data-code-tab]").forEach(button => button.addEventListener("click", () => { state.previewFormat = button.dataset.codeTab; $$("[data-code-tab]").forEach(item => { item.classList.toggle("active", item === button); item.setAttribute("aria-selected", item === button ? "true" : "false"); }); renderPreview(); }));
    $("#copy-preview").addEventListener("click", copyPreview); $("#refresh-history").addEventListener("click", () => loadHistory()); $("#refresh-all").addEventListener("click", refreshAll);
    $("#history-search").addEventListener("input", () => renderHistory());
    $("#history-list").addEventListener("click", event => { const card = event.target.closest("[data-run-id]"); if (card) selectRun(card.dataset.runId, true); });
    $("#cancel-run").addEventListener("click", cancelRun); $("[data-close-run]").addEventListener("click", () => $("#run-dialog").close());
    for (const dialog of $$("dialog")) dialog.addEventListener("click", event => { if (event.target === dialog) dialog.close(); });
    $("#environment-dialog").addEventListener("close", () => { $("#environment-key").value = ""; $("#environment-key").type = "password"; });
    document.addEventListener("visibilitychange", () => { if (document.visibilityState === "visible") loadHistory(true); });
  }

  function persistFormValues() {
    const operation = currentOperation(); if (!operation) return;
    const existing = state.values[valuesKey()] || {};
    for (const item of operation.fields || []) {
      if (["model", "content"].includes(item.name)) continue;
      const element = $("#request-form").elements[item.name]; if (!element) continue;
      if (item.type === "boolean" || item.name === "tools") existing[item.name] = Boolean(element.checked);
      else existing[item.name] = element.value;
    }
    state.values[valuesKey()] = existing;
  }
  function updateContentItem(event) {
    const wrapper = event.target.closest("[data-content-index]"); const key = event.target.dataset.contentKey; if (!wrapper || !key) return;
    const item = state.content[Number(wrapper.dataset.contentIndex)]; item[key] = event.target.value;
    if (key === "type") { item.role = defaultRole(item.type); delete item.text; delete item.url; renderContent(); }
    updatePreview();
  }
  async function copyPreview() {
    const text = $("#request-preview").textContent;
    try { await navigator.clipboard.writeText(text); toast("已复制到剪贴板", "success"); }
    catch (_) { const selection = window.getSelection(); const range = document.createRange(); range.selectNodeContents($("#request-preview")); selection.removeAllRanges(); selection.addRange(range); toast("已选中预览内容，请手动复制"); }
  }
  async function refreshAll() { const button = $("#refresh-all"); button.disabled = true; try { await Promise.all([loadEnvironments(), loadHistory()]); await updatePreview(true); toast("数据已刷新", "success"); } finally { button.disabled = false; } }

  function normalizeCatalog(input) {
    if (!input) return [];
    const models = Array.isArray(input) ? input : Array.isArray(input.models) ? input.models : [];
    return models.filter(model => model && model.id).map(model => ({ ...model, operations: arrayFrom(model.operations).map(op => ({ ...op, fields: arrayFrom(op.fields).map(item => ({ ...item, item_type: item.item_type || item.itemType })) })) }));
  }
  function normalizeRun(input) {
    const run = input?.run || input || {};
    const estimatedBilling = parseMaybeJSON(run.estimated_billing || run.estimated_billing_json);
    const actualBilling = parseMaybeJSON(run.actual_billing || run.actual_billing_json);
    return {
      ...run,
      id: String(run.id || run.run_id || ""),
      status: normalizeStatus(run.status || run.state || "pending"),
      progress: firstDefined(run.progress, run.percent, run.percentage),
      estimated_amount: firstDefined(run.estimated_amount, run.estimated_cost, estimatedBilling?.amount),
      actual_amount: firstDefined(run.actual_amount, run.actual_cost, actualBilling?.amount),
      delta_amount: firstDefined(run.delta_amount, run.cost_delta),
      result_json: parseMaybeJSON(run.result_json || run.result), request_json: parseMaybeJSON(run.request_json || run.request),
      exchanges: arrayFrom(input?.exchanges || run.exchanges)
    };
  }
  function upsertRun(run) { const index = state.runs.findIndex(item => item.id === run.id); if (index >= 0) state.runs[index] = { ...state.runs[index], ...run }; else state.runs.unshift(run); }
  function normalizeStatus(status) { const value = String(status || "pending").toLowerCase(); return value === "success" || value === "completed" ? "succeeded" : value === "failure" || value === "error" ? "failed" : value; }
  function statusClass(status) { return normalizeStatus(status).replace(/[^a-z]/g, ""); }
  function statusLabel(status) { return ({ pending: "等待提交", submitted: "已提交", queued: "排队中", polling: "生成中", running: "生成中", processing: "处理中", succeeded: "已完成", failed: "失败", canceled: "已停止轮询", cancelled: "已停止轮询" })[normalizeStatus(status)] || String(status || "未知"); }
  function progressNumber(run) { let value = Number(run.progress ?? 0); if (!Number.isFinite(value)) return 0; if (value > 0 && value <= 1) value *= 100; if (normalizeStatus(run.status) === "succeeded") value = 100; return Math.max(0, Math.min(100, value)); }
  function isVideoRun(run) { return String(run.model || "").includes("video") || String(run.model || "").includes("seedance") || String(run.operation || "").includes("video"); }
  function mediaURLs(run) {
    const data = parseMaybeJSON(run.result_json || run.result) || {};
    const candidates = [run.media_url, run.url, data.url, data.result_url, data.video?.url, data.data?.url, data.data?.video_url, ...(Array.isArray(data.data) ? data.data.map(item => item?.url) : []), ...(Array.isArray(data.images) ? data.images.map(item => item?.url || item) : [])];
    return [...new Set(candidates.filter(value => typeof value === "string" && (/^https?:\/\//.test(value) || value.startsWith("/"))))];
  }
  function billingFormula(run) { const data = parseMaybeJSON(run.actual_billing || run.estimated_billing); return data?.formula || data?.description || (run.actual_amount == null ? "实际账单生成后将通过 /api/log/token 自动同步" : "实际费用已同步"); }
  function runDuration(run) { const start = Date.parse(run.submitted_at || run.created_at); const end = Date.parse(run.completed_at || run.updated_at); return Number.isFinite(start) && Number.isFinite(end) && end >= start ? `${((end - start) / 1000).toFixed(1)} 秒` : "—"; }

  function updateSubmitState() { $("#submit-run").disabled = state.loading.bootstrap || state.loading.submit || !state.activeEnvironmentId || !currentOperation(); }
  function setSubmitting(on) { state.loading.submit = on; $("#submit-run").classList.toggle("loading", on); updateSubmitState(); }
  function setPreviewStatus(text, kind = "") { const status = $("#preview-status"); status.innerHTML = `<i></i>${escapeHTML(text)}`; status.className = `status-dot ${kind}`; }
  function showMessage(selector, text, success = false) { const el = $(selector); el.textContent = text; el.hidden = false; el.classList.toggle("success", success); }
  function hideMessage(selector) { const el = $(selector); el.hidden = true; el.textContent = ""; el.classList.remove("success"); }
  function setButtonBusy(selector, busy, label) { const el = $(selector); el.disabled = busy; el.textContent = label; }
  function toast(message, kind = "") { const el = document.createElement("div"); el.className = `toast ${kind}`; el.textContent = message; $("#toast-region").appendChild(el); window.setTimeout(() => el.remove(), 3500); }
  function money(value, currency = "CNY", signed = false) { const number = Number(value); if (!Number.isFinite(number)) return String(value); const symbol = String(currency).toUpperCase() === "USD" ? "$" : "¥"; return `${signed && number > 0 ? "+" : ""}${symbol}${number.toFixed(number < .01 ? 6 : 4)}`; }
  function formatDate(value, withSeconds = false) { const date = new Date(value); if (!value || Number.isNaN(date.getTime())) return "—"; return new Intl.DateTimeFormat("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", ...(withSeconds ? { second: "2-digit" } : {}) }).format(date); }
  function shortHost(url) { try { return new URL(url).host; } catch (_) { return String(url || "").replace(/^https?:\/\//, ""); } }
  function shortId(id) { const value = String(id || ""); return value.length > 14 ? `${value.slice(0, 8)}…${value.slice(-4)}` : value || "—"; }
  function shortModel(model) { return String(model || "未知模型").replace("doubao-", "").replace("grok-imagine-", "Grok "); }
  function placeholderFor(name) { return ({ url: "https://example.com/media.png", id: "asset_…", name: "用于生成的参考素材", aspect_ratio: "16:9" })[name] || ""; }
  function optionLabel(name, value) { if (name === "asset_type") return ({ image: "图片", video: "视频", audio: "音频" })[value] || value; if (value === "adaptive") return "adaptive（自适应）"; if (value === "auto") return "auto（自动）"; return value; }
  function localizeDescription(value) { return value.replace("Only {\"type\":\"web_search\"} is supported.", "仅支持 web_search 工具").replace("At least prompt or valid content is required.", "至少填写提示词或有效内容"); }
  function parseMaybeJSON(value) { if (value == null || value === "") return value; if (typeof value === "object") return value; if (Array.isArray(value)) return value; try { return JSON.parse(value); } catch (_) { try { return JSON.parse(new TextDecoder().decode(Uint8Array.from(atob(value), char => char.charCodeAt(0)))); } catch (_) { return value; } } }
  function arrayFrom(value) { return Array.isArray(value) ? value : []; }
  function firstDefined(...values) { return values.find(value => value !== undefined && value !== null && value !== ""); }
  function interpolatePath(path, payload) { return String(path || "").replace(/\{([^}]+)\}/g, (_, key) => encodeURIComponent(payload[key] || `{${key}}`)); }
  function redactURL(value) { try { const url = new URL(value); for (const key of [...url.searchParams.keys()]) if (/token|key|secret|sign|auth/i.test(key)) url.searchParams.set(key, "[REDACTED]"); return url.toString(); } catch (_) { return String(value || ""); } }
  function escapeHTML(value) { return String(value ?? "").replace(/[&<>"']/g, char => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[char]); }
  function escapeAttr(value) { return escapeHTML(value).replace(/`/g, "&#96;"); }
  function cssEscape(value) { return window.CSS?.escape ? window.CSS.escape(value) : String(value).replace(/[^a-zA-Z0-9_-]/g, "\\$&"); }
  function syntaxHighlight(text) { return escapeHTML(text).replace(/(&quot;(?:\\u[a-fA-F0-9]{4}|\\[^u]|[^\\&])*&quot;)(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d+)?(?:[eE][+\-]?\d+)?/g, match => { let cls = "json-number"; if (/^&quot;/.test(match)) cls = /:$/.test(match) ? "json-key" : "json-string"; else if (/true|false|null/.test(match)) cls = "json-literal"; return `<span class="${cls}">${match}</span>`; }); }

  bindEvents();
  loadBootstrap();
  monitorInstance();
  state.historyTimer = window.setInterval(() => { if (document.visibilityState === "visible") loadHistory(true); }, 12000);
})();
