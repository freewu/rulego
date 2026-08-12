/**
 * rulego 自定义积木定义
 * 注意：rule_trigger 积木下拉的事件类型需与后端 internal/rule.TriggerTypes 保持一致。
 */
Blockly.defineBlocksWithJsonArray([
  // ---------- 事件入口（规则触发器）----------
  {
    type: "rule_trigger",
    message0: "当事件 %1 触发时",
    args0: [
      {
        type: "field_dropdown",
        name: "EVENT",
        options: [
          ["数据创建 data.created", "data.created"],
          ["数据更新 data.updated", "data.updated"],
          ["数据删除 data.deleted", "data.deleted"],
          ["定时触发 timer.interval", "timer.interval"],
          ["HTTP 请求 http.request", "http.request"],
        ],
      },
    ],
    message1: "%1",
    args1: [{ type: "input_statement", name: "BODY" }],
    colour: 290,
    tooltip: "规则入口：当指定事件发生时执行内部逻辑",
    helpUrl: "",
    hat: "cap",
  },

  // ---------- 上下文读取 ----------
  {
    type: "ctx_get",
    message0: "上下文 %1",
    args0: [{ type: "field_input", name: "FIELD", text: "field" }],
    output: null,
    colour: 160,
    tooltip: "读取输入上下文 ctx 中的字段值",
    helpUrl: "",
  },

  // ---------- 上下文写入 ----------
  {
    type: "ctx_set",
    message0: "设置上下文 %1 为 %2",
    args0: [
      { type: "field_input", name: "FIELD", text: "field" },
      { type: "input_value", name: "VALUE" },
    ],
    previousStatement: null,
    nextStatement: null,
    colour: 160,
    tooltip: "设置输入上下文 ctx 中的字段值",
    helpUrl: "",
  },

  // ---------- 日志输出 ----------
  {
    type: "rule_log",
    message0: "记录日志 级别 %1 内容 %2",
    args0: [
      {
        type: "field_dropdown",
        name: "LEVEL",
        options: [
          ["INFO", "info"],
          ["WARN", "warn"],
          ["ERROR", "error"],
        ],
      },
      { type: "input_value", name: "MSG" },
    ],
    previousStatement: null,
    nextStatement: null,
    colour: 210,
    tooltip: "向宿主输出一条日志（显示在执行结果中）",
    helpUrl: "",
  },

  // ---------- 发送告警 ----------
  {
    type: "rule_alert",
    message0: "发送告警 渠道 %1 内容 %2",
    args0: [
      {
        type: "field_dropdown",
        name: "CHANNEL",
        options: [
          ["邮件 email", "email"],
          ["短信 sms", "sms"],
          ["Webhook", "webhook"],
        ],
      },
      { type: "input_value", name: "MSG" },
    ],
    previousStatement: null,
    nextStatement: null,
    colour: 20,
    tooltip: "通过指定渠道发送告警（显示在执行结果中）",
    helpUrl: "",
  },
]);

/**
 * 从工作区中查找第一个 rule_trigger 积木，返回其事件类型。
 * @param {!Blockly.Workspace} workspace
 * @returns {string}
 */
function findTriggerEvent(workspace) {
  let event = "";
  const blocks = workspace.getAllBlocks(false);
  for (const block of blocks) {
    if (block.type === "rule_trigger") {
      event = block.getFieldValue("EVENT") || "";
      break;
    }
  }
  return event;
}
