// 生成示例规则 JSON（examples/rules/*.json）与用例模板（examples/templates/*.json）：
// 使用真实 Blockly 序列化格式
const path = require("path");
const fs = require("fs");
const { JSDOM } = require("jsdom");
const dom = new JSDOM("<!DOCTYPE html><html><body></body></html>");
globalThis.window = dom.window;
globalThis.document = dom.window.document;
globalThis.navigator = dom.window.navigator;
globalThis.DOMParser = dom.window.DOMParser;
globalThis.XMLSerializer = dom.window.XMLSerializer;
globalThis.Node = dom.window.Node;

// 与后端 internal/rule.EngineVersion 保持一致
const ENGINE_VERSION = "1.1.0";

const web = path.resolve(__dirname, "..");
const Blockly = require(path.join(web, "vendor/blockly/blockly_compressed.js"));
globalThis.Blockly = Blockly;
Blockly.libraryBlocks = require(path.join(web, "vendor/blockly/blocks_compressed.js"));
Blockly.Lua = require(path.join(web, "vendor/blockly/lua_compressed.js")).luaGenerator;
const messages = require(path.join(web, "vendor/blockly/msg/zh-hans.js"));
Blockly.Msg = Object.assign({}, Blockly.Msg || {}, messages);
require(path.join(web, "js/blocks.js"));
require(path.join(web, "js/lua_gen.js"));

function workspaceJson(build) {
  const ws = new Blockly.Workspace();
  build(ws);
  return Blockly.serialization.workspaces.save(ws);
}

function ruleJson(id, name, desc, trigger, workspace, enabled = true) {
  const ws = new Blockly.Workspace();
  Blockly.serialization.workspaces.load(workspace, ws);
  const lua = Blockly.Lua.workspaceToCode(ws);
  const now = new Date().toISOString();
  // 固定时间戳与 block ID，保证每次生成结果可复现（git diff 干净）
  const TS = "2026-08-12T00:00:00Z";
  const idMap = new Map();
  let n = 0;
  (function stable(obj) {
    if (Array.isArray(obj)) { obj.forEach(stable); return; }
    if (obj && typeof obj === "object") {
      for (const k of Object.keys(obj)) {
        if (k === "id" && typeof obj[k] === "string") {
          if (!idMap.has(obj[k])) idMap.set(obj[k], "blk_" + n++);
          obj[k] = idMap.get(obj[k]);
        } else stable(obj[k]);
      }
    }
  })(workspace);
  return {
    id,
    name,
    description: desc,
    enabled,
    version: 1,
    engine_version: ENGINE_VERSION,
    trigger,
    workspace,
    lua,
    created_at: TS,
    updated_at: TS,
  };
}

// ---------- 规则一：库存告警 ----------
const invWs = workspaceJson((ws) => {
  const trigger = ws.newBlock("rule_trigger");
  trigger.setFieldValue("data.updated", "EVENT");

  const ifb = ws.newBlock("controls_if");
  const cmp = ws.newBlock("logic_compare");
  cmp.setFieldValue("LT", "OP");
  const ctxGet = ws.newBlock("ctx_get");
  ctxGet.setFieldValue("stock", "FIELD");
  const num10 = ws.newBlock("math_number");
  num10.setFieldValue(10, "NUM");
  cmp.getInput("A").connection.connect(ctxGet.outputConnection);
  cmp.getInput("B").connection.connect(num10.outputConnection);
  ifb.getInput("IF0").connection.connect(cmp.outputConnection);

  const logBlock = ws.newBlock("rule_log");
  logBlock.setFieldValue("warn", "LEVEL");
  const textMsg = ws.newBlock("text");
  textMsg.setFieldValue("库存不足", "TEXT");
  logBlock.getInput("MSG").connection.connect(textMsg.outputConnection);
  ifb.getInput("DO0").connection.connect(logBlock.previousConnection);

  const alertBlock = ws.newBlock("rule_alert");
  alertBlock.setFieldValue("email", "CHANNEL");
  const textMsg2 = ws.newBlock("text");
  textMsg2.setFieldValue("低库存告警", "TEXT");
  alertBlock.getInput("MSG").connection.connect(textMsg2.outputConnection);
  ifb.getInput("DO0").connection.connect(alertBlock.previousConnection);

  trigger.getInput("BODY").connection.connect(ifb.previousConnection);
});

// ---------- 规则二：订单金额校验 ----------
const orderWs = workspaceJson((ws) => {
  const trigger = ws.newBlock("rule_trigger");
  trigger.setFieldValue("data.created", "EVENT");

  const ifb = ws.newBlock("controls_if");
  const cmp = ws.newBlock("logic_compare");
  cmp.setFieldValue("GT", "OP");
  const ctxAmount = ws.newBlock("ctx_get");
  ctxAmount.setFieldValue("amount", "FIELD");
  const num1000 = ws.newBlock("math_number");
  num1000.setFieldValue(1000, "NUM");
  cmp.getInput("A").connection.connect(ctxAmount.outputConnection);
  cmp.getInput("B").connection.connect(num1000.outputConnection);
  ifb.getInput("IF0").connection.connect(cmp.outputConnection);

  const setFlag = ws.newBlock("ctx_set");
  setFlag.setFieldValue("need_review", "FIELD");
  const boolTrue = ws.newBlock("logic_boolean");
  boolTrue.setFieldValue("TRUE", "BOOL");
  setFlag.getInput("VALUE").connection.connect(boolTrue.outputConnection);
  ifb.getInput("DO0").connection.connect(setFlag.previousConnection);

  const alertBlock = ws.newBlock("rule_alert");
  alertBlock.setFieldValue("webhook", "CHANNEL");
  const textMsg = ws.newBlock("text");
  textMsg.setFieldValue("大额订单需人工审核", "TEXT");
  alertBlock.getInput("MSG").connection.connect(textMsg.outputConnection);
  ifb.getInput("DO0").connection.connect(alertBlock.previousConnection);

  trigger.getInput("BODY").connection.connect(ifb.previousConnection);
});

// ---------- 用例模板（Blockly 工作区构建） ----------

// 模板一：定时健康检查（timer.interval 触发 + 日志）
const healthWs = workspaceJson((ws) => {
  const trigger = ws.newBlock("rule_trigger");
  trigger.setFieldValue("timer.interval", "EVENT");

  const logBlock = ws.newBlock("rule_log");
  logBlock.setFieldValue("info", "LEVEL");
  const textMsg = ws.newBlock("text");
  textMsg.setFieldValue("定时任务执行中", "TEXT");
  logBlock.getInput("MSG").connection.connect(textMsg.outputConnection);

  trigger.getInput("BODY").connection.connect(logBlock.previousConnection);
});

// 模板二：HTTP 请求访问日志（http.request 触发 + 日志）
const httpWs = workspaceJson((ws) => {
  const trigger = ws.newBlock("rule_trigger");
  trigger.setFieldValue("http.request", "EVENT");

  const logBlock = ws.newBlock("rule_log");
  logBlock.setFieldValue("info", "LEVEL");
  const join = ws.newBlock("text_join");
  const method = ws.newBlock("ctx_get");
  method.setFieldValue("method", "FIELD");
  const suffix = ws.newBlock("text");
  suffix.setFieldValue(" 请求", "TEXT");
  join.getInput("ADD0").connection.connect(method.outputConnection);
  join.getInput("ADD1").connection.connect(suffix.outputConnection);
  logBlock.getInput("MSG").connection.connect(join.outputConnection);

  trigger.getInput("BODY").connection.connect(logBlock.previousConnection);
});

// 模板三：数据删除通知（data.deleted 触发 + 短信告警）
const delWs = workspaceJson((ws) => {
  const trigger = ws.newBlock("rule_trigger");
  trigger.setFieldValue("data.deleted", "EVENT");

  const alertBlock = ws.newBlock("rule_alert");
  alertBlock.setFieldValue("sms", "CHANNEL");
  const textMsg = ws.newBlock("text");
  textMsg.setFieldValue("数据被删除，请确认", "TEXT");
  alertBlock.getInput("MSG").connection.connect(textMsg.outputConnection);

  trigger.getInput("BODY").connection.connect(alertBlock.previousConnection);
});

// 案例规则定义：示例（examples/rules）与模板（examples/templates）
// 同时输出到 internal/rule/seed/ 供 go:embed 内嵌，首次启动时自动初始化
const outDir = path.join(__dirname, "..", "..", "examples", "rules");
const tplDir = path.join(__dirname, "..", "..", "examples", "templates");
const seedDir = path.join(__dirname, "..", "..", "internal", "rule", "seed");

const examples = [
  ["inventory_alert.json", ruleJson("rule_inventory", "库存告警", "库存低于阈值时发送告警", "data.updated", invWs)],
  ["order_review.json", ruleJson("rule_order_review", "大额订单审核", "订单金额超过 1000 时标记人工审核", "data.created", orderWs)],
];
const templates = [
  ["health_check.json", ruleJson("rule_tpl_health_check", "模板：定时健康检查", "定时触发记录运行日志，可用于定时任务与心跳上报", "timer.interval", healthWs)],
  ["http_request_log.json", ruleJson("rule_tpl_http_log", "模板：HTTP 请求日志", "记录 HTTP 请求的方法与路径，输入数据需包含 method、path 字段", "http.request", httpWs)],
  ["delete_notify.json", ruleJson("rule_tpl_delete_notify", "模板：数据删除通知", "数据被删除时发送短信告警", "data.deleted", delWs)],
];

function writeAll(dir, files) {
  fs.mkdirSync(dir, { recursive: true });
  for (const [name, rule] of files) {
    fs.writeFileSync(path.join(dir, name), JSON.stringify(rule, null, 2));
  }
}

writeAll(outDir, examples);
writeAll(tplDir, templates);
writeAll(seedDir, examples.concat(templates)); // 全部作为内置案例（启动自动初始化）

console.log("已生成示例规则:");
console.log("  examples/rules/inventory_alert.json");
console.log("  examples/rules/order_review.json");
console.log("已生成用例模板:");
console.log("  examples/templates/health_check.json");
console.log("  examples/templates/http_request_log.json");
console.log("  examples/templates/delete_notify.json");
console.log("已生成内置案例（启动自动初始化）:");
console.log("  internal/rule/seed/");

// 打印库存告警的 Lua 供参考
console.log("\n===== inventory_alert.lua =====");
console.log(ruleJson("x", "x", "", "data.updated", invWs).lua);
