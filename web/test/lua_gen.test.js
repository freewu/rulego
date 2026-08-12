// 验证 vendored Blockly + 自定义积木 + Lua 生成器（无浏览器环境，使用 jsdom）
const path = require("path");
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
// Node 模式下压缩包返回各自命名空间，需手动挂载（浏览器 script 标签会自动挂载）
const libraryBlocks = require(path.join(web, "vendor/blockly/blocks_compressed.js"));
Blockly.libraryBlocks = libraryBlocks;
const luaGen = require(path.join(web, "vendor/blockly/lua_compressed.js"));
Blockly.Lua = luaGen.luaGenerator;
const messages = require(path.join(web, "vendor/blockly/msg/zh-hans.js"));
Blockly.Msg = Object.assign({}, Blockly.Msg || {}, messages);
require(path.join(web, "js/blocks.js"));
require(path.join(web, "js/lua_gen.js"));

// ---------- 构造：事件 -> 如果 库存<10 则 日志+告警 ----------
const ws = new Blockly.Workspace();

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

const code = Blockly.Lua.workspaceToCode(ws);
console.log("===== 生成的 Lua 代码 =====");
console.log(code);

// ---------- 校验生成结果 ----------
const expectMain = code.includes("function main(ctx)");
const expectCtx = code.includes("ctx['stock']");
const expectLog = code.includes("log('warn', tostring('库存不足'))");
const expectAlert = code.includes("alert('email', tostring('低库存告警'))");
const expectIf = code.includes("if");
console.log("===== 校验 =====");
console.log("包含 function main(ctx):", expectMain);
console.log("包含 ctx[\"stock\"]:", expectCtx);
console.log("包含 log():", expectLog);
console.log("包含 alert():", expectAlert);
console.log("包含 if:", expectIf);

// ---------- 序列化 JSON 往返 ----------
const json = Blockly.serialization.workspaces.save(ws);
const s = JSON.stringify(json);
const ws2 = new Blockly.Workspace();
Blockly.serialization.workspaces.load(json, ws2);
const code2 = Blockly.Lua.workspaceToCode(ws2);
console.log("===== JSON 往返 =====");
console.log("往返后代码一致:", code.trim() === code2.trim());

const allOk =
  expectMain && expectCtx && expectLog && expectAlert && expectIf &&
  code.trim() === code2.trim();
console.log(allOk ? "\n✅ 全部通过" : "\n❌ 存在失败项");
process.exit(allOk ? 0 : 1);
