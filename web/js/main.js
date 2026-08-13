/**
 * rulego 前端主逻辑：初始化 Blockly 工作区、保存/加载规则、执行测试。
 */

// ---------- 工具函数 ----------
async function api(method, path, body) {
  const opts = { method, headers: {} };
  if (body !== undefined) {
    opts.headers["Content-Type"] = "application/json";
    opts.body = JSON.stringify(body);
  }
  const res = await fetch(path, opts);
  let data = null;
  try {
    data = await res.json();
  } catch (e) {
    /* 非 JSON 响应 */
  }
  if (!res.ok) {
    const msg = (data && data.error) || `请求失败 (${res.status})`;
    throw new Error(msg);
  }
  return data;
}

let toastTimer = null;
function toast(msg, type = "") {
  const el = document.getElementById("toast");
  el.textContent = msg;
  el.className = "show " + type;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => (el.className = ""), 2600);
}

// ---------- 工具箱 ----------
const TOOLBOX = {
  kind: "categoryToolbox",
  contents: [
    {
      kind: "category",
      name: "事件",
      colour: "290",
      contents: [{ kind: "block", type: "rule_trigger" }],
    },
    {
      kind: "category",
      name: "规则动作",
      colour: "20",
      contents: [
        { kind: "block", type: "ctx_set" },
        { kind: "block", type: "rule_log" },
        { kind: "block", type: "rule_alert" },
      ],
    },
    {
      kind: "category",
      name: "上下文",
      colour: "160",
      contents: [{ kind: "block", type: "ctx_get" }],
    },
    {
      kind: "category",
      name: "逻辑",
      colour: "210",
      contents: [
        { kind: "block", type: "controls_if" },
        { kind: "block", type: "logic_compare" },
        { kind: "block", type: "logic_operation" },
        { kind: "block", type: "logic_negate" },
        { kind: "block", type: "logic_boolean" },
        { kind: "block", type: "logic_null" },
        { kind: "block", type: "logic_ternary" },
      ],
    },
    {
      kind: "category",
      name: "循环",
      colour: "120",
      contents: [
        { kind: "block", type: "controls_repeat_ext" },
        { kind: "block", type: "controls_whileUntil" },
        { kind: "block", type: "controls_for" },
        { kind: "block", type: "controls_forEach" },
      ],
    },
    {
      kind: "category",
      name: "数学",
      colour: "230",
      contents: [
        { kind: "block", type: "math_number" },
        { kind: "block", type: "math_arithmetic" },
        { kind: "block", type: "math_single" },
        { kind: "block", type: "math_round" },
        { kind: "block", type: "math_modulo" },
        { kind: "block", type: "math_compare" },
      ],
    },
    {
      kind: "category",
      name: "文本",
      colour: "160",
      contents: [
        { kind: "block", type: "text" },
        { kind: "block", type: "text_join" },
        { kind: "block", type: "text_append" },
        { kind: "block", type: "text_length" },
        { kind: "block", type: "text_isEmpty" },
      ],
    },
    { kind: "category", name: "变量", custom: "VARIABLE", colour: "330" },
  ],
};

// ---------- 全局状态 ----------
let workspace = null;
let currentRuleId = "";

// ---------- 初始化 Blockly ----------
function initBlockly() {
  workspace = Blockly.inject("blocklyDiv", {
    toolbox: TOOLBOX,
    trashcan: true,
    zoom: {
      controls: true,
      wheel: true,
      startScale: 0.9,
      maxScale: 2,
      minScale: 0.4,
    },
    grid: { spacing: 24, length: 4, colour: "#e5e7eb", snap: true },
    renderer: "zelos",
    theme: Blockly.Themes.Classic,
    move: { scrollbars: true, drag: true, wheel: true },
  });
  workspace.addChangeListener(() => {
    // 任何改动后清空当前 ID 标记（防止误覆盖已有规则？不，保留 ID 以便原地更新）
    // 这里仅更新启停文案
    document.getElementById("rule-enabled-text").textContent =
      document.getElementById("rule-enabled").checked ? "已启用" : "已停用";
  });
}

// ---------- 生成 Lua ----------
function generateLua() {
  const code = Blockly.Lua.workspaceToCode(workspace);
  document.getElementById("lua-output").value = code;
  return code;
}

async function validateLua(code) {
  const statusEl = document.getElementById("validate-status");
  if (!code.trim()) {
    statusEl.textContent = "";
    statusEl.className = "validate-status";
    return;
  }
  try {
    const r = await api("POST", "/api/validate", { lua: code });
    if (r.valid) {
      statusEl.textContent = "✓ Lua 语法正确";
      statusEl.className = "validate-status ok";
    } else {
      statusEl.textContent = "✗ " + (r.error || "语法错误");
      statusEl.className = "validate-status err";
    }
  } catch (e) {
    statusEl.textContent = "✗ 校验失败: " + e.message;
    statusEl.className = "validate-status err";
  }
}

// ---------- 保存规则 ----------
async function saveRule() {
  const name = document.getElementById("rule-name").value.trim();
  if (!name) {
    toast("请填写规则名称", "err");
    return;
  }
  const code = generateLua();
  if (!code.includes("function main(ctx)")) {
    toast("请先拖入「事件」积木作为规则入口", "err");
    return;
  }
  const event = findTriggerEvent(workspace) || "data.updated";
  const workspaceJson = Blockly.serialization.workspaces.save(workspace);

  const payload = {
    id: currentRuleId || undefined,
    name,
    description: document.getElementById("rule-desc").value.trim(),
    enabled: document.getElementById("rule-enabled").checked,
    trigger: event,
    workspace: workspaceJson,
    lua: code,
  };

  try {
    let saved;
    if (currentRuleId) {
      saved = await api("PUT", `/api/rules/${currentRuleId}`, payload);
      toast("规则已更新", "ok");
    } else {
      saved = await api("POST", "/api/rules", payload);
      toast("规则已保存", "ok");
    }
    currentRuleId = saved.id;
    document.getElementById("rule-id").value = saved.id;
    document.getElementById("rule-enabled").checked = saved.enabled;
    document.getElementById("rule-enabled-text").textContent = saved.enabled
      ? "已启用"
      : "已停用";
  } catch (e) {
    toast("保存失败: " + e.message, "err");
  }
}

// ---------- 执行规则 ----------
async function runRule() {
  if (!currentRuleId) {
    toast("请先保存规则再执行", "err");
    return;
  }
  let data = {};
  try {
    const raw = document.getElementById("test-input").value.trim();
    data = raw ? JSON.parse(raw) : {};
  } catch (e) {
    toast("输入数据不是合法 JSON: " + e.message, "err");
    return;
  }
  const out = document.getElementById("test-output");
  out.textContent = "执行中…";
  try {
    const res = await api("POST", `/api/rules/${currentRuleId}/run`, { data });
    const lines = [];
    lines.push("=== 执行结果 ===");
    lines.push(`耗时: ${res.duration_ms} ms`);
    if (res.return !== undefined && res.return !== null) {
      lines.push(`返回值: ${JSON.stringify(res.return)}`);
    }
    lines.push(`日志 (${(res.outputs || []).length}):`);
    (res.outputs || []).forEach((l) => lines.push("  " + l));
    lines.push(`告警 (${(res.alerts || []).length}):`);
    (res.alerts || []).forEach((a) =>
      lines.push(`  [${a.channel}] ${a.message}`)
    );
    if (res.error) lines.push("错误: " + res.error);
    out.textContent = lines.join("\n");
    out.style.color = res.error ? "#dc2626" : "#0f172a";
  } catch (e) {
    out.textContent = "执行失败: " + e.message;
    out.style.color = "#dc2626";
  }
}

// ---------- 载入规则到工作区 ----------
async function loadRule(id) {
  try {
    const r = await api("GET", `/api/rules/${id}`);
    if (r.workspace) {
      Blockly.serialization.workspaces.load(r.workspace, workspace);
    } else {
      workspace.clear();
    }
    currentRuleId = r.id;
    document.getElementById("rule-id").value = r.id;
    document.getElementById("rule-name").value = r.name || "";
    document.getElementById("rule-desc").value = r.description || "";
    document.getElementById("rule-enabled").checked = !!r.enabled;
    document.getElementById("rule-enabled-text").textContent = r.enabled
      ? "已启用"
      : "已停用";
    document.getElementById("lua-output").value = r.lua || "";
    document.getElementById("json-editor").value = JSON.stringify(r, null, 2);
    toast(`已载入规则「${r.name}」`, "ok");
    switchTab("info");
  } catch (e) {
    toast("载入失败: " + e.message, "err");
  }
}

// ---------- 导出 ----------
function downloadBlob(blob, filename) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

async function exportRule(id) {
  try {
    const res = await fetch(`/api/rules/${encodeURIComponent(id)}/export`);
    if (!res.ok) {
      let msg = `导出失败 (${res.status})`;
      try {
        const d = await res.json();
        if (d.error) msg = d.error;
      } catch (e) { /* ignore */ }
      throw new Error(msg);
    }
    downloadBlob(await res.blob(), `${id}.json`);
    toast("已导出规则 JSON", "ok");
  } catch (e) {
    toast("导出失败: " + e.message, "err");
  }
}

// ---------- 新建 / 重置 ----------
function resetEditor() {
  workspace.clear();
  currentRuleId = "";
  document.getElementById("rule-id").value = "";
  document.getElementById("rule-name").value = "";
  document.getElementById("rule-desc").value = "";
  document.getElementById("rule-enabled").checked = true;
  document.getElementById("rule-enabled-text").textContent = "已启用";
  document.getElementById("lua-output").value = "";
  document.getElementById("json-editor").value = "";
  document.getElementById("validate-status").textContent = "";
  document.getElementById("validate-status").className = "validate-status";
  document.getElementById("test-output").textContent = "尚未执行";
  toast("已新建空白规则", "ok");
}

// ---------- 应用 JSON（直接编辑规则 JSON 并保存） ----------
async function applyJson() {
  if (!currentRuleId) {
    toast("请先保存规则（或从管理页载入）再编辑 JSON", "err");
    return;
  }
  const raw = document.getElementById("json-editor").value.trim();
  let obj;
  try {
    obj = JSON.parse(raw);
  } catch (e) {
    toast("JSON 格式错误: " + e.message, "err");
    return;
  }
  try {
    const saved = await api("PUT", `/api/rules/${currentRuleId}`, obj);
    toast(`JSON 已保存（v${saved.version}），重新载入…`, "ok");
    loadRule(saved.id);
  } catch (e) {
    toast("保存失败: " + e.message, "err");
  }
}

// ---------- Tab 切换 ----------
function switchTab(name) {
  document.querySelectorAll(".tab").forEach((t) => {
    t.classList.toggle("active", t.dataset.tab === name);
  });
  document.querySelectorAll(".tab-panel").forEach((p) => {
    p.classList.toggle("active", p.id === "tab-" + name);
  });
}

// ---------- 事件绑定 ----------
document.addEventListener("DOMContentLoaded", () => {
  initBlockly();

  // 从管理页跳转进入时自动载入对应规则
  const params = new URLSearchParams(location.search);
  if (params.get("id")) {
    loadRule(params.get("id"));
  }

  document.querySelectorAll(".tab").forEach((t) => {
    t.addEventListener("click", () => switchTab(t.dataset.tab));
  });

  document.getElementById("btn-back").addEventListener("click", () => {
    location.href = "/";
  });
  document.getElementById("btn-new").addEventListener("click", resetEditor);
  document.getElementById("btn-export").addEventListener("click", () => {
    if (currentRuleId) exportRule(currentRuleId);
    else toast("请先保存规则再导出", "err");
  });
  document.getElementById("btn-generate").addEventListener("click", () => {
    const code = generateLua();
    if (!code.trim()) {
      toast("工作区为空，请先拖拽积木", "err");
      return;
    }
    validateLua(code);
    toast("Lua 已生成，可点击右侧「Lua 代码」查看", "ok");
  });
  document.getElementById("btn-save").addEventListener("click", saveRule);
  document.getElementById("btn-run").addEventListener("click", runRule);
  document.getElementById("btn-apply-json").addEventListener("click", applyJson);
  document.getElementById("btn-copy-lua").addEventListener("click", async () => {
    const code = document.getElementById("lua-output").value;
    if (!code) return toast("没有可复制的代码", "err");
    try {
      await navigator.clipboard.writeText(code);
      toast("已复制到剪贴板", "ok");
    } catch (e) {
      document.getElementById("lua-output").select();
      document.execCommand("copy");
      toast("已复制到剪贴板", "ok");
    }
  });

  // Ctrl+S 保存
  document.addEventListener("keydown", (e) => {
    if ((e.ctrlKey || e.metaKey) && e.key === "s") {
      e.preventDefault();
      saveRule();
    }
  });
});
