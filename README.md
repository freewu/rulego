# rulego — 基于 Go 和 Blockly 的可视化规则引擎

通过 **Blockly 拖拽积木** 可视化地编排业务规则，自动生成 **Lua 代码**，由 **Go 后端** 提供配置管理与规则执行服务。规则本身以 **JSON 格式** 存储，可随时还原回拖拽界面继续编辑。

## 核心特性

- 🧩 **可视化编辑**：Blockly 拖拽积木（事件触发、条件判断、循环、上下文读写、日志、告警），中文界面
- ⚡ **Lua 生成**：积木一键生成 `function main(ctx) ... end` 形式的 Lua 规则脚本
- 📄 **JSON 规则格式**：每条规则一个 JSON 文件，包含 Blockly 工作区（可还原编辑）与生成的 Lua 代码
- 🛡️ **沙箱执行**：基于 gopher-lua 的受限运行时，不开放 `os/io/debug` 库，支持执行超时
- 🗄️ **配置管理**：YAML 配置文件 + `RULEGO_*` 环境变量覆盖 + 默认值三级机制
- 🔌 **REST API**：规则 CRUD、语法校验、在线执行测试
- 📦 **离线可用**：Blockly 已 vendor 到 `web/vendor/`，无需 CDN

## 项目结构

```
rulego/
├── cmd/rulego/               # 服务启动入口
├── internal/
│   ├── config/               # 配置管理（YAML + 环境变量 + 默认值）
│   ├── rule/                 # 规则模型（JSON）与文件存储
│   ├── lua/                  # gopher-lua 沙箱执行引擎
│   └── server/               # HTTP API 与前端静态资源托管
├── web/                      # Blockly 前端
│   ├── index.html
│   ├── js/                   # 自定义积木、Lua 生成器、页面逻辑
│   ├── vendor/blockly/       # Blockly 本地化（离线可用）
│   └── test/                 # 前端 Lua 生成验证（Node + jsdom）
├── examples/rules/           # 示例规则（JSON）
├── config.example.yaml       # 配置示例
└── go.mod
```

## 快速开始

### 1. 启动服务

```bash
go build -o rulego ./cmd/rulego
./rulego -c config.example.yaml
# 或使用默认配置直接启动：./rulego
```

启动后浏览器打开 **http://localhost:8080**：

```
=== rulego 可视化规则引擎 ===
server: 0.0.0.0:8080
static_dir: web
rules_dir: data/rules
lua_timeout: 5s
API 与前端地址: http://0.0.0.0:8080
```

### 2. 编辑规则

1. 从左侧工具箱拖入「事件」积木（规则入口），选择触发事件类型
2. 组合「逻辑」「循环」「数学」「文本」「上下文」「规则动作」等积木
3. 点击 **⚡ 生成 Lua** 预览代码，**💾 保存规则** 以 JSON 存入后端
4. 在「执行测试」页签填入输入数据 JSON，点击 **▶ 执行规则** 查看日志、告警与返回值

### 3. 规则 JSON 格式

每条规则保存为一个 JSON 文件（默认目录 `data/rules/`）：

```json
{
  "id": "rule_inventory",
  "name": "库存告警",
  "description": "库存低于阈值时发送告警",
  "enabled": true,
  "version": 3,
  "trigger": "data.updated",
  "workspace": { "...": "Blockly 工作区序列化，用于还原拖拽界面" },
  "lua": "function main(ctx)\n  if ctx['stock'] < 10 then\n    alert('email', tostring('低库存告警'))\n  end\nend",
  "created_at": "2026-08-12T07:46:17Z",
  "updated_at": "2026-08-12T07:46:32Z"
}
```

## REST API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET  | `/api/health` | 健康检查 |
| GET  | `/api/triggers` | 支持的触发事件类型列表 |
| GET  | `/api/rules` | 规则列表 |
| POST | `/api/rules` | 新建规则（ID 为空自动生成） |
| GET  | `/api/rules/{id}` | 查询规则 |
| PUT  | `/api/rules/{id}` | 更新规则（版本自增） |
| DELETE | `/api/rules/{id}` | 删除规则 |
| POST | `/api/rules/{id}/run` | 执行规则，`{"data":{...}}` 作为 ctx 输入 |
| POST | `/api/validate` | Lua 语法校验，`{"lua":"..."}` |

执行示例：

```bash
curl -X POST http://localhost:8080/api/rules/rule_inventory/run \
  -H "Content-Type: application/json" \
  -d '{"data":{"stock":5}}'
# => {"outputs":["[WARN] 库存不足"],"alerts":[{"channel":"email","message":"低库存告警"}],"return":null,"duration_ms":0}
```

## 配置管理

配置文件为 YAML，支持三级覆盖（优先级从低到高）：

1. **内置默认值**：`0.0.0.0:8080`、`web`、`data/rules`、5s 超时
2. **配置文件**：`-c config.yaml`
3. **环境变量**：`RULEGO_SERVER_PORT=9090`、`RULEGO_LUA_TIMEOUT_SECONDS=10` 等

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  static_dir: "web"
storage:
  rules_dir: "data/rules"
lua:
  timeout_seconds: 5
```

## Lua 规则脚本约定

- 规则必须是 `function main(ctx) ... end` 形式，由 Blockly 自动生成
- `ctx` 为输入上下文表，通过执行接口的 `data` 注入（支持嵌套 map/数组）
- 内置两个宿主函数：
  - `log(level, msg)`：输出日志，显示在执行结果 `outputs` 中
  - `alert(channel, msg)`：发送告警，显示在执行结果 `alerts` 中
- 沙箱限制：仅开放 `base/table/string/math` 库；`dofile/loadfile/load/print` 及 `os/io/debug/package` 均不可用

## 自定义积木

在 `web/js/blocks.js` 中用 `Blockly.defineBlocksWithJsonArray` 定义积木，
在 `web/js/lua_gen.js` 中用 `Blockly.Lua.forBlock["type"]` 编写 Lua 生成器。
注意「事件」积木的下拉选项需与后端 `internal/rule.TriggerTypes` 保持一致。

## 测试

```bash
# Go 后端测试（配置/规则存储/Lua 沙箱/HTTP API）
go test ./...

# 前端 Blockly→Lua 生成测试（Node + jsdom，验证自定义积木与生成器）
cd web && npm install && npm test

# 重新生成示例规则 JSON
cd web && npm run gen-examples
```

## License

MIT
