/**
 * rulego 规则管理页（默认首页）逻辑：统计、筛选、列表操作、导入导出。
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

function esc(s) {
  return String(s).replace(/[&<>"']/g, (c) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;",
  }[c]));
}

function fmtTime(iso) {
  if (!iso) return "-";
  return iso.replace("T", " ").slice(0, 16);
}

// ---------- 状态 ----------
let allRules = [];
let currentFilter = "all"; // all | enabled | disabled

// ---------- 列表加载 ----------
async function loadRules() {
  const tbody = document.getElementById("rule-tbody");
  try {
    const rules = await api("GET", "/api/rules");
    allRules = rules;
    renderStats();
    renderList();
  } catch (e) {
    tbody.innerHTML = `<tr><td colspan="8" class="empty">加载失败: ${esc(e.message)}</td></tr>`;
  }
}

function renderStats() {
  const enabled = allRules.filter((r) => r.enabled).length;
  document.getElementById("stat-total").textContent = allRules.length;
  document.getElementById("stat-enabled").textContent = enabled;
  document.getElementById("stat-disabled").textContent = allRules.length - enabled;
}

function renderList() {
  const tbody = document.getElementById("rule-tbody");
  const list =
    currentFilter === "all"
      ? allRules
      : allRules.filter((r) =>
          currentFilter === "enabled" ? r.enabled : !r.enabled
        );

  document.getElementById("list-count").textContent = list.length
    ? `共 ${allRules.length} 条 · 当前显示 ${list.length} 条`
    : "暂无规则";

  if (!list.length) {
    tbody.innerHTML = `<tr><td colspan="8" class="empty">暂无规则，点击右上角「＋ 新建规则」开始，或首次启动时已自动初始化案例规则</td></tr>`;
    return;
  }

  tbody.innerHTML = list
    .map(
      (r) => `
    <tr>
      <td class="cell-name">
        <span class="rule-name">${esc(r.name)}</span>
        ${r.description ? `<div class="cell-desc">${esc(r.description)}</div>` : ""}
      </td>
      <td><code class="cell-id">${esc(r.id)}</code></td>
      <td><span class="cell-trigger">${esc(r.trigger)}</span></td>
      <td>v${r.version}</td>
      <td>${esc(r.engine_version || "-")}</td>
      <td><span class="tag ${r.enabled ? "on" : "off"}">${r.enabled ? "启用" : "停用"}</span></td>
      <td class="cell-time">${fmtTime(r.updated_at)}</td>
      <td class="cell-ops">
        <button class="btn edit" data-id="${esc(r.id)}">编辑</button>
        <button class="btn copy" data-id="${esc(r.id)}">复制</button>
        <button class="btn export" data-id="${esc(r.id)}">导出</button>
        <button class="btn toggle" data-id="${esc(r.id)}" data-enabled="${r.enabled}">${r.enabled ? "停用" : "启用"}</button>
        <button class="btn del" data-id="${esc(r.id)}">删除</button>
      </td>
    </tr>`
    )
    .join("");
}

// ---------- 操作 ----------
function editRule(id) {
  location.href = `/editor.html?id=${encodeURIComponent(id)}`;
}

async function duplicateRule(id) {
  try {
    const r = await api("POST", `/api/rules/${encodeURIComponent(id)}/duplicate`);
    toast(`已复制为「${r.name}」`, "ok");
    loadRules();
  } catch (e) {
    toast("复制失败: " + e.message, "err");
  }
}

async function toggleRule(id, enabled) {
  try {
    const r = await api("GET", `/api/rules/${id}`);
    r.enabled = enabled;
    await api("PUT", `/api/rules/${id}`, r);
    toast(enabled ? "已启用" : "已停用", "ok");
    loadRules();
  } catch (e) {
    toast("操作失败: " + e.message, "err");
  }
}

async function deleteRule(id) {
  if (!confirm(`确定删除规则「${id}」吗？`)) return;
  try {
    await api("DELETE", `/api/rules/${id}`);
    toast("已删除", "ok");
    loadRules();
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
      loadRules();
    } catch (err) {
      toast("导入失败: " + err.message, "err");
    }
  };
  reader.readAsText(file, "utf-8");
}

// ---------- 事件绑定 ----------
document.addEventListener("DOMContentLoaded", () => {
  loadRules();

  document.getElementById("btn-new").addEventListener("click", () => {
    location.href = "/editor.html";
  });
  document.getElementById("btn-import").addEventListener("click", importRuleFile);
  document.getElementById("btn-export").addEventListener("click", exportAllRules);
  document.getElementById("btn-refresh").addEventListener("click", loadRules);
  document.getElementById("file-import").addEventListener("change", handleImportFile);

  // 筛选
  document.querySelectorAll(".filters .filter").forEach((btn) => {
    btn.addEventListener("click", () => {
      currentFilter = btn.dataset.filter;
      document.querySelectorAll(".filters .filter").forEach((b) =>
        b.classList.toggle("active", b === btn)
      );
      renderList();
    });
  });

  // 列表操作（事件委托）
  document.getElementById("rule-tbody").addEventListener("click", (e) => {
    const btn = e.target.closest("button");
    if (!btn) return;
    const id = btn.dataset.id;
    if (btn.classList.contains("edit")) editRule(id);
    else if (btn.classList.contains("copy")) duplicateRule(id);
    else if (btn.classList.contains("export")) exportRule(id);
    else if (btn.classList.contains("toggle"))
      toggleRule(id, btn.dataset.enabled !== "true");
    else if (btn.classList.contains("del")) deleteRule(id);
  });
});
