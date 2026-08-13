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
    refreshRuleList();
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

// ---------- 规则列表 ----------
async function refreshRuleList() {
  const ul = document.getElementById("rule-list");
  try {
    const rules = await api("GET", "/api/rules");
    if (!rules.length) {
      ul.innerHTML = '<li class="empty">暂无规则，请先保存一条规则</li>';
      return;
    }
    ul.innerHTML = "";
    rules.forEach((r) => {
      const li = document.createElement("li");
      li.className = "rule-item";
      const created = (r.created_at || "").slice(0, 10);
      li.innerHTML = `
        <div class="rule-head">
          <span class="rule-name">${esc(r.name)}</span>
          <span class="tag ${r.enabled ? "on" : "off"}">${r.enabled ? "启用" : "停用"}</span>
        </div>
        <div class="rule-meta">${esc(r.id)} · ${esc(r.trigger)} · v${r.version}${r.engine_version ? " · " + esc(r.engine_version) : ""} · ${created}</div>
        <div class="rule-ops">
          <button class="btn load" data-id="${esc(r.id)}">载入</button>
          <button class="btn copy" data-id="${esc(r.id)}">复制</button>
          <button class="btn export" data-id="${esc(r.id)}">导出</button>
          <button class="btn toggle" data-id="${esc(r.id)}" data-enabled="${r.enabled}">${r.enabled ? "停用" : "启用"}</button>
          <button class="btn del" data-id="${esc(r.id)}">删除</button>
        </div>`;
      ul.appendChild(li);
    });
  } catch (e) {
    ul.innerHTML = `<li class="empty">加载失败: ${esc(e.message)}</li>`;
  }
}

function esc(s) {
  return String(s).replace(/[&<>"']/g, (c) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;",
  }[c]));
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
    toast(`已载入规则「${r.name}」`, "ok");
    switchTab("lua");
  } catch (e) {
    toast("载入失败: " + e.message, "err");
  }
}

async function toggleRule(id, enabled) {
  try {
    const r = await api("GET", `/api/rules/${id}`);
    r.enabled = enabled;
    await api("PUT", `/api/rules/${id}`, r);
    toast(enabled ? "已启用" : "已停用", "ok");
    refreshRuleList();
  } catch (e) {
    toast("操作失败: " + e.message, "err");
  }
}

async function deleteRule(id) {
  if (!confirm(`确定删除规则 ${id} 吗？`)) return;
  try {
    await api("DELETE", `/api/rules/${id}`);
    if (currentRuleId === id) {
      resetEditor();
    }
    toast("已删除", "ok");
    refreshRuleList();
  } catch (e) {
    toast("删除失败: " + e.message, "err");
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

async function exportAllRules() {
  try {
    const res = await fetch("/api/rules/export");
    if (!res.ok) {
      let msg = `导出失败 (${res.status})`;
      try {
        const d = await res.json();
        if (d.error) msg = d.error;
      } catch (e) { /* ignore */ }
      throw new Error(msg);
    }
    downloadBlob(await res.blob(), "rules_export.json");
    toast("已导出全部规则", "ok");
  } catch (e) {
    toast("导出失败: " + e.message, "err");
  }
}

// ---------- 导入 ----------
function importRuleFile() {
  const input = document.getElementById("file-import");
  input.value = "";
  input.click();
}

async function handleImportFile(e) {
  const file = e.target.files && e.target.files[0];
  if (!file) return;
  const reader = new FileReader();
  reader.onload = async () => {
    try {
      JSON.parse(reader.result); // 先校验 JSON 合法性
      const res = await fetch("/api/rules/import", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: reader.result,
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || `导入失败 (${res.status})`);
      const parts = [];
      if (data.imported) parts.push(`新增 ${data.imported} 条`);
      if (data.updated) parts.push(`更新 ${data.updated} 条`);
      if (data.skipped) parts.push(`跳过 ${data.skipped} 条`);
      if (data.failed && data.failed.length)
        parts.push(`失败 ${data.failed.length} 条`);
      toast("导入完成：" + (parts.join("，") || "无变更"), data.failed && data.failed.length ? "err" : "ok");
      refreshRuleList();
    } catch (err) {
      toast("导入失败: " + err.message, "err");
    }
  };
  reader.readAsText(file, "utf-8");
}

// ---------- 复制 ----------
async function duplicateRule(id) {
  try {
    const r = await api("POST", `/api/rules/${encodeURIComponent(id)}/duplicate`);
    toast(`已复制为「${r.name}」`, "ok");
    refreshRuleList();
  } catch (e) {
    toast("复制失败: " + e.message, "err");
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
  document.getElementById("validate-status").textContent = "";
  document.getElementById("validate-status").className = "validate-status";
  document.getElementById("test-output").textContent = "尚未执行";
  toast("已新建空白规则", "ok");
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
  refreshRuleList();

  document.querySelectorAll(".tab").forEach((t) => {
    t.addEventListener("click", () => switchTab(t.dataset.tab));
  });

  document.getElementById("btn-new").addEventListener("click", resetEditor);
  document.getElementById("btn-import").addEventListener("click", importRuleFile);
  document.getElementById("btn-export").addEventListener("click", () => {
    if (currentRuleId) exportRule(currentRuleId);
    else if (confirm("当前没有已保存的规则，是否导出全部规则？")) exportAllRules();
  });
  document.getElementById("file-import").addEventListener("change", handleImportFile);
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

  // 规则列表按钮（事件委托）
  document.getElementById("rule-list").addEventListener("click", (e) => {
    const btn = e.target.closest("button");
    if (!btn) return;
    const id = btn.dataset.id;
    if (btn.classList.contains("load")) loadRule(id);
    else if (btn.classList.contains("copy")) duplicateRule(id);
    else if (btn.classList.contains("export")) exportRule(id);
    else if (btn.classList.contains("toggle"))
      toggleRule(id, btn.dataset.enabled !== "true");
    else if (btn.classList.contains("del")) deleteRule(id);
  });

  // Ctrl+S 保存
  document.addEventListener("keydown", (e) => {
    if ((e.ctrlKey || e.metaKey) && e.key === "s") {
      e.preventDefault();
      saveRule();
    }
  });
});
