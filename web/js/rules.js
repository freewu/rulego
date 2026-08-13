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
        <button class="btn history" data-id="${esc(r.id)}">历史</button>
        <button class="btn json" data-id="${esc(r.id)}">JSON</button>
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

// ---------- 版本历史 ----------
let versionRuleId = ""; // 当前打开版本弹窗的规则 ID

// 打开版本历史弹窗
async function openVersions(id) {
  versionRuleId = id;
  const rule = allRules.find((r) => r.id === id);
  document.getElementById("version-modal-title").textContent =
    `版本历史 · ${rule ? rule.name : id}`;
  document.getElementById("version-modal").hidden = false;
  document.getElementById("diff-result").hidden = true;
  const tbody = document.getElementById("version-tbody");
  tbody.innerHTML = `<tr><td colspan="4" class="empty">加载中…</td></tr>`;
  try {
    const versions = await api("GET", `/api/rules/${encodeURIComponent(id)}/versions`);
    renderVersions(versions, rule ? rule.version : 0);
    fillDiffSelects(versions, rule ? rule.version : 0);
  } catch (e) {
    tbody.innerHTML = `<tr><td colspan="4" class="empty">加载失败: ${esc(e.message)}</td></tr>`;
  }
}

function renderVersions(versions, current) {
  const tbody = document.getElementById("version-tbody");
  if (!versions.length) {
    tbody.innerHTML = `<tr><td colspan="4" class="empty">暂无历史版本（当前为 v${current}）</td></tr>`;
    return;
  }
  // 当前版本显示在最前
  const rows = [
    `<tr class="row-current">
      <td><code>v${current}</code> <span class="tag on">当前</span></td>
      <td class="cell-time">${fmtTime(versions.length ? "" : "")}</td>
      <td>-</td>
      <td class="cell-ops">
        <button class="btn json" data-v="${current}">查看 JSON</button>
      </td>
    </tr>`,
    ...versions
      .map(
        (v) => `
    <tr>
      <td><code>v${v.version}</code></td>
      <td class="cell-time">${fmtTime(v.saved_at)}</td>
      <td>${(v.size / 1024).toFixed(1)} KB</td>
      <td class="cell-ops">
        <button class="btn json" data-v="${v.version}">查看 JSON</button>
        <button class="btn restore" data-v="${v.version}">回滚</button>
      </td>
    </tr>`
      )
      .join(""),
  ];
  tbody.innerHTML = rows.join("");
}

function fillDiffSelects(versions, current) {
  const opts = [
    `<option value="${current}">v${current}（当前）</option>`,
    ...versions
      .slice()
      .reverse()
      .map((v) => `<option value="${v.version}">v${v.version}</option>`),
  ];
  document.getElementById("diff-v1").innerHTML = opts.join("");
  document.getElementById("diff-v2").innerHTML = opts.join("");
  // 默认 v1=最早，v2=当前
  const all = versions.slice();
  if (all.length) {
    document.getElementById("diff-v1").value = all[0].version;
  }
  document.getElementById("diff-v2").value = current;
}

// 版本 JSON 比对
async function diffVersions() {
  const v1 = document.getElementById("diff-v1").value;
  const v2 = document.getElementById("diff-v2").value;
  if (v1 === v2) {
    toast("请选择两个不同的版本进行比对", "err");
    return;
  }
  const result = document.getElementById("diff-result");
  const tbody = document.getElementById("diff-tbody");
  result.hidden = false;
  tbody.innerHTML = `<tr><td colspan="3" class="empty">比对中…</td></tr>`;
  try {
    const data = await api(
      "GET",
      `/api/rules/${encodeURIComponent(versionRuleId)}/diff?v1=${v1}&v2=${v2}`
    );
    const patch = data.patch || [];
    document.getElementById("diff-summary").textContent =
      `v${data.v1} → v${data.v2}：共 ${patch.length} 处差异`;
    if (!patch.length) {
      tbody.innerHTML = `<tr><td colspan="3" class="empty">两个版本内容一致</td></tr>`;
      return;
    }
    tbody.innerHTML = patch
      .map((op) => {
        const val =
          op.value === undefined
            ? ""
            : typeof op.value === "string"
            ? esc(op.value.slice(0, 120))
            : esc(JSON.stringify(op.value).slice(0, 120));
        const opLabel = { add: "新增", remove: "删除", replace: "修改", move: "移动", copy: "复制", test: "校验" }[op.op] || op.op;
        const cls = op.op === "remove" ? "op-del" : op.op === "add" ? "op-add" : "op-replace";
        return `<tr><td><span class="op-tag ${cls}">${opLabel}</span></td><td><code>${esc(op.path || "/")}</code></td><td class="cell-val">${val}</td></tr>`;
      })
      .join("");
  } catch (e) {
    tbody.innerHTML = `<tr><td colspan="3" class="empty">比对失败: ${esc(e.message)}</td></tr>`;
  }
}

// 查看某版本 JSON（复用 JSON 弹窗，只读态）
async function viewVersionJson(id, v) {
  try {
    const data = await api("GET", `/api/rules/${encodeURIComponent(id)}/versions/${v}`);
    openJsonEditor(id, data, true);
  } catch (e) {
    toast("读取版本失败: " + e.message, "err");
  }
}

// 回滚到某版本
async function restoreVersion(id, v) {
  if (!confirm(`确定将规则「${id}」回滚到 v${v} 吗？\n回滚将保存为新的版本。`)) return;
  try {
    const r = await api("POST", `/api/rules/${encodeURIComponent(id)}/versions/${v}/restore`);
    toast(`已回滚到 v${v}（当前 v${r.version}）`, "ok");
    loadRules();
    openVersions(id);
  } catch (e) {
    toast("回滚失败: " + e.message, "err");
  }
}

// ---------- JSON 查看 / 编辑 ----------
let jsonRuleId = "";
let jsonReadonly = false;

async function openJsonEditor(id, data, readonly) {
  jsonRuleId = id;
  jsonReadonly = !!readonly;
  const rule = data || (allRules.find((r) => r.id === id) || {});
  document.getElementById("json-modal-title").textContent =
    `规则 JSON · ${rule.name || id}`;
  document.getElementById("json-modal").hidden = false;
  const editor = document.getElementById("json-editor");
  if (data) {
    editor.value = JSON.stringify(data, null, 2);
  } else {
    try {
      const full = await api("GET", `/api/rules/${encodeURIComponent(id)}`);
      editor.value = JSON.stringify(full, null, 2);
    } catch (e) {
      toast("读取规则失败: " + e.message, "err");
      return;
    }
  }
  editor.readOnly = jsonReadonly;
  document.getElementById("btn-json-save").style.display = jsonReadonly ? "none" : "";
}

async function saveJsonEditor() {
  if (jsonReadonly) return;
  const raw = document.getElementById("json-editor").value;
  let obj;
  try {
    obj = JSON.parse(raw);
  } catch (e) {
    toast("JSON 格式错误: " + e.message, "err");
    return;
  }
  try {
    const saved = await api("PUT", `/api/rules/${encodeURIComponent(jsonRuleId)}`, obj);
    toast(`JSON 已保存（v${saved.version}）`, "ok");
    closeModal("json-modal");
    loadRules();
  } catch (e) {
    toast("保存失败: " + e.message, "err");
  }
}

function downloadJsonEditor() {
  const raw = document.getElementById("json-editor").value;
  downloadBlob(new Blob([raw], { type: "application/json" }), `${jsonRuleId}.json`);
}

// ---------- 弹窗 ----------
function closeModal(id) {
  document.getElementById(id).hidden = true;
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
    else if (btn.classList.contains("history")) openVersions(id);
    else if (btn.classList.contains("json")) openJsonEditor(id);
    else if (btn.classList.contains("copy")) duplicateRule(id);
    else if (btn.classList.contains("export")) exportRule(id);
    else if (btn.classList.contains("toggle"))
      toggleRule(id, btn.dataset.enabled !== "true");
    else if (btn.classList.contains("del")) deleteRule(id);
  });

  // 版本弹窗内操作（事件委托）
  document.getElementById("version-tbody").addEventListener("click", (e) => {
    const btn = e.target.closest("button");
    if (!btn) return;
    const v = btn.dataset.v;
    if (btn.classList.contains("json")) viewVersionJson(versionRuleId, v);
    else if (btn.classList.contains("restore")) restoreVersion(versionRuleId, v);
  });

  document.getElementById("btn-diff").addEventListener("click", diffVersions);
  document.getElementById("btn-json-save").addEventListener("click", saveJsonEditor);
  document.getElementById("btn-json-download").addEventListener("click", downloadJsonEditor);
  document.querySelectorAll(".modal-close").forEach((btn) => {
    btn.addEventListener("click", () => closeModal(btn.dataset.close));
  });
  document.querySelectorAll(".modal-overlay").forEach((ov) => {
    ov.addEventListener("click", (e) => {
      if (e.target === ov) closeModal(ov.id);
    });
  });
});
