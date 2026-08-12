# rulego — 基于 Go 和 Blockly 的可视化规则引擎

> 通过 **Blockly 拖拽积木** 可视化编排业务规则，自动生成 **Lua 代码**，由 **Go 后端** 提供配置管理与规则执行服务。规则以 **JSON 格式** 存储，可随时还原回拖拽界面继续编辑。

## 特性

- 🧩 **可视化编辑** — Blockly 拖拽积木（事件触发 / 条件判断 / 循环 / 上下文读写 / 日志 / 告警），中文界面
- ⚡ **Lua 代码生成** — 积木一键生成 `function main(ctx) ... end` 规则脚本
- 📄 **JSON 规则格式** — 每条规则一个 JSON 文件，包含 Blockly 工作区（可还原）与 Lua 代码
- 🛡️ **沙箱执行** — 基于 gopher-lua 的受限运行时：不开放 `os/io/debug`，移除 `dofile/loadfile/load/print`，支持抢占式超时终止
- 🗄️ **配置管理** — YAML 配置文件 + `RULEGO_*` 环境变量覆盖 + 内置默认值，三级机制
- 🔌 **REST API** — 规则 CRUD、Lua 语法校验、在线执行测试
- 📦 **离线可用** — Blockly 已 vendor 到 `web/vendor/`（约 1.2MB），无需 CDN
- ✅ **完整测试** — Go 单元测试（配置/存储/沙箱/API）+ Node jsdom 前端生成测试

## 架构

```
┌───────────────────────── 浏览器 ─────────────────────────┐
│  Blockly 拖拽界面 (web/)                                  │
│  自定义积木 → Lua 生成器 → 规则 JSON (workspace + lua)     │
└───────────────┬──────────────────────────────────────────┘
                │ HTTP /api
┌───────────────▼──────────────────────────────────────────┐
│  Go 服务 (cmd/rulego)                                     │
│  ├─ internal/config   配置管理 (YAML + 环境变量 + 默认值)  │
│  ├─ internal/rule     规则模型 + JSON 文件存储             │
│  ├─ internal/lua      gopher-lua 沙箱执行引擎              │
│  └─ internal/server   REST API + 静态资源托管              │
└───────────────────────────────────────────────────────────┘
```

## 快速开始

### 1. 启动服务

```bash
go build -o rulego ./cmd/rulego
./rulego -c config.example.yaml   # 或 just run
```

浏览器打开 **http://localhost:8080**：

```
=== rulego 可视化规则引擎 ===
server: 0.0.0.0:8080
static_dir: web
rules_dir: data/rules
lua_timeout: 5s
API 与前端地址: http://0.0.0.0:8080
```

### 2. 用 Blockly 编辑一条规则

1. 从左侧工具箱拖入 **「事件」积木**（规则入口），选择触发事件类型（如 `数据更新 data.updated`）
2. 组合 **「规则动作」**（设置上下文 / 记录日志 / 发送告警）、**「逻辑」**、**「数学」** 等积木
3. 点击 **⚡ 生成 Lua** 预览生成的代码
4. 填写规则名称，点击 **💾 保存规则** — 以 JSON 形式存入后端
5. 切到 **「执行测试」** 页签，填入输入数据 JSON，点击 **▶ 执行规则** 查看日志、告警与返回值

### 3. 通过 API 执行规则

```bash
# 新建规则（可从 examples/rules/ 导入）
curl -X POST http://localhost:8080/api/rules \
  -H "Content-Type: application/json" \
  -d @examples/rules/inventory_alert.json

# 执行规则（data 作为 ctx 输入）
curl -X POST http://localhost:8080/api/rules/rule_inventory/run \
  -H "Content-Type: application/json" \
  -d '{"data":{"stock":5}}'

# => {"outputs":["[WARN] 库存不足"],"alerts":[{"channel":"email","message":"低库存告警"}],"return":null,"duration_ms":0}
```

## 规则 JSON 格式

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

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | string | 规则唯一 ID（`r_` 前缀自动生成，或自定义，≤64 字符） |
| `name` | string | 规则名称（必填） |
| `description` | string | 规则描述 |
| `enabled` | bool | 是否启用 |
| `version` | int | 版本号，每次更新自增 |
| `trigger` | string | 触发事件类型（见 `/api/triggers`） |
| `workspace` | object | Blockly 工作区序列化，用于还原可视化编辑 |
| `lua` | string | 生成的 Lua 规则脚本（必填） |
| `created_at` / `updated_at` | string | RFC3339 时间戳 |

## REST API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/health` | 健康检查 |
| GET | `/api/triggers` | 支持的触发事件类型列表 |
| GET | `/api/rules` | 规则列表 |
| POST | `/api/rules` | 新建规则（ID 为空自动生成） |
| GET | `/api/rules/{id}` | 查询规则 |
| PUT | `/api/rules/{id}` | 更新规则（版本自增） |
| DELETE | `/api/rules/{id}` | 删除规则 |
| POST | `/api/rules/{id}/run` | 执行规则，`{"data":{...}}` 作为 ctx 输入 |
| POST | `/api/validate` | Lua 语法校验，`{"lua":"..."}` |

## Lua 规则脚本约定

- 规则必须是 `function main(ctx) ... end` 形式，由 Blockly 自动生成
- `ctx` 为输入上下文表，通过执行接口的 `data` 注入（支持嵌套 map / 数组）
- 内置两个宿主函数：

| 函数 | 说明 | 执行结果中的位置 |
|------|------|------------------|
| `log(level, msg)` | 输出日志 | `outputs` 数组 |
| `alert(channel, msg)` | 发送告警 | `alerts` 数组（channel + message） |

- **沙箱限制**：仅开放 `base/table/string/math` 库；`dofile/loadfile/load/print` 及 `os/io/debug/package` 均不可用；单条规则有执行超时（默认 5s，可配置）

## 配置管理

配置文件为 YAML，三级覆盖（优先级从低到高）：

1. **内置默认值** — `0.0.0.0:8080` / `web` / `data/rules` / 5s 超时
2. **配置文件** — `./rulego -c config.yaml`
3. **环境变量** — `RULEGO_SERVER_PORT=9090`、`RULEGO_LUA_TIMEOUT_SECONDS=10` 等

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

## 项目结构

```
rulego/
├── cmd/rulego/            # 服务启动入口
├── internal/
│   ├── config/            # 配置管理（YAML + 环境变量 + 默认值）
│   ├── rule/              # 规则模型（JSON）与文件存储
│   ├── lua/               # gopher-lua 沙箱执行引擎
│   └── server/            # HTTP API 与前端静态资源托管
├── web/                   # Blockly 前端
│   ├── index.html
│   ├── js/                # 自定义积木 / Lua 生成器 / 页面逻辑
│   ├── vendor/blockly/    # Blockly 本地化（离线可用）
│   └── test/              # 前端 Lua 生成验证（Node + jsdom）
├── examples/rules/        # 示例规则（JSON）
├── scripts/push.sh        # 提交推送脚本（Windows git）
├── config.example.yaml    # 配置示例
├── justfile              # 常用命令（just 版，替代 Makefile）
```

## 自定义积木开发

新增一个积木需要两步：

**1. 定义积木**（`web/js/blocks.js`）：

```js
Blockly.defineBlocksWithJsonArray([
  {
    type: "my_action",
    message0: "执行动作 %1",
    args0: [{ type: "field_input", name: "TEXT", text: "hello" }],
    previousStatement: null,
    nextStatement: null,
    colour: 20,
  },
]);
```

**2. 编写 Lua 生成器**（`web/js/lua_gen.js`，Blockly 11 使用 `forBlock` 注册表）：

```js
Blockly.Lua.forBlock["my_action"] = function (block) {
  const text = block.getFieldValue("TEXT");
  return `myAction(${Blockly.Lua.quote_(text)})\n`;
};
```

> ⚠️ 「事件」积木（`rule_trigger`）的下拉选项需与后端 `internal/rule.TriggerTypes` 保持一致；若新增宿主函数（如 `myAction`），需同步在 `internal/lua/runtime.go` 中注入。

## 开发约定

- **每次完成一轮改动后提交并推送到远程**：`./scripts/push.sh "commit message"`（或 `git add -A && git commit -m "..." && git push`）
- Go 代码遵循 `gofmt` 格式，提交前运行 `go vet ./...` 与 `go test ./...`
- 前端脚本修改后用 `node --check` 做语法校验，并运行 `just test-frontend`（Node + jsdom 生成测试）

## 测试

```bash
# Go 后端测试（配置 / 规则存储 / Lua 沙箱 / HTTP API）
just test

# 前端 Blockly → Lua 生成测试（验证自定义积木与生成器）
cd web && npm install
just test-frontend

# 重新生成示例规则 JSON
just examples
```

## 常见问题

**Q: just 命令在 Windows 和 WSL 下都能用吗？**
**Q: just 命令在 Windows 和 WSL 下都能用吗？**
可以。justfile 跨平台：Windows 终端与 WSL/Linux 通用，平台差异（工具链路径、可执行文件后缀）由 `os()` 判断自动处理。注意 just 1.58 在 Windows 上**优先使用 `SHELL` 环境变量**确定 shell，若它指向 unix 路径（如 Git Bash 会话继承的 `/usr/bin/sh`）会报 `could not find the shell sh`。已配置 PowerShell profile 将 `SHELL` 强制为 `D:\Program Files\Git\bin\sh.exe` 并补齐 PATH；报错时**关闭并重新打开 PowerShell** 即可。当前会话临时修复：`$env:SHELL = "D:\Program Files\Git\bin\sh.exe"`。

**Q: 本地 git push 报 `gnutls_handshake() failed` / 连接超时？**
本环境（WSL + 代理）下 WSL 版 git 推送 HTTPS 大包易失败，请改用 Windows 版 git：
`"/mnt/d/Program Files/Git/cmd/git.exe" push origin main`，或直接使用 `./scripts/push.sh`。

**Q: 前端页面样式错乱 / 积木空白？**
确认 `web/vendor/blockly/` 存在（Blockly 已本地化，勿删除）；若修改过 `web/js/blocks.js`，浏览器需强制刷新（Ctrl+F5）。

**Q: 规则执行报「执行超时」？**
规则内存在死循环或耗时操作。在 `config.yaml` 的 `lua.timeout_seconds` 调整超时，或优化规则逻辑（沙箱内不建议做重型计算）。

## License

MIT
