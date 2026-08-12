/**
 * rulego 自定义积木的 Lua 代码生成器（Blockly 11 使用 forBlock 注册表）。
 * 生成目标：后端沙箱执行的 `function main(ctx) ... end` 规则脚本。
 * ctx 为输入上下文表，log()/alert() 由后端注入。
 */

// ---------- 事件入口：生成 main 函数外壳 ----------
Blockly.Lua.forBlock["rule_trigger"] = function (block) {
  const body = Blockly.Lua.statementToCode(block, "BODY");
  const code = "function main(ctx)\n" + body + "end\n";
  return code;
};

// ---------- 上下文读取 ----------
Blockly.Lua.forBlock["ctx_get"] = function (block) {
  const field = block.getFieldValue("FIELD") || "";
  const code = `ctx[${Blockly.Lua.quote_(field)}]`;
  return [code, Blockly.Lua.ORDER_ATOMIC];
};

// ---------- 上下文写入 ----------
Blockly.Lua.forBlock["ctx_set"] = function (block) {
  const field = block.getFieldValue("FIELD") || "";
  const value =
    Blockly.Lua.valueToCode(block, "VALUE", Blockly.Lua.ORDER_NONE) || "nil";
  return `ctx[${Blockly.Lua.quote_(field)}] = ${value}\n`;
};

// ---------- 日志输出 ----------
Blockly.Lua.forBlock["rule_log"] = function (block) {
  const level = block.getFieldValue("LEVEL") || "info";
  const msg =
    Blockly.Lua.valueToCode(block, "MSG", Blockly.Lua.ORDER_NONE) || '""';
  return `log(${Blockly.Lua.quote_(level)}, tostring(${msg}))\n`;
};

// ---------- 发送告警 ----------
Blockly.Lua.forBlock["rule_alert"] = function (block) {
  const channel = block.getFieldValue("CHANNEL") || "email";
  const msg =
    Blockly.Lua.valueToCode(block, "MSG", Blockly.Lua.ORDER_NONE) || '""';
  return `alert(${Blockly.Lua.quote_(channel)}, tostring(${msg}))\n`;
};
