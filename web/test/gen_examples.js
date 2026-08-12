// 生成示例规则 JSON（examples/rules/*.json）：使用真实 Blockly 序列化格式
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
  return {
    id,
    name,
    description: desc,
    enabled,
    version: 1,
    trigger,
    workspace,
    lua,
    created_at: now,
    updated_at: now,
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

const outDir = path.join(__dirname, "..", "..", "examples", "rules");
fs.mkdirSync(outDir, { recursive: true });
fs.writeFileSync(
  path.join(outDir, "inventory_alert.json"),
  JSON.stringify(ruleJson("rule_inventory", "库存告警", "库存低于阈值时发送告警", "data.updated", invWs), null, 2)
);
fs.writeFileSync(
  path.join(outDir, "order_review.json"),
  JSON.stringify(ruleJson("rule_order_review", "大额订单审核", "订单金额超过 1000 时标记人工审核", "data.created", orderWs), null, 2)
);

console.log("已生成示例规则:");
console.log("  examples/rules/inventory_alert.json");
console.log("  examples/rules/order_review.json");

// 打印库存告警的 Lua 供参考
console.log("\n===== inventory_alert.lua =====");
console.log(ruleJson("x", "x", "", "data.updated", invWs).lua);
