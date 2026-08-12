# AGENTS.md — rulego 项目工作约定

本文件会被 pi 在启动时自动加载。请始终遵守以下约定。

## 🔴 核心约定：每轮工作结束必须提交并推送

**每次完成一轮工作（实现功能 / 修复 / 改文档）后，必须：**

1. `git add -A`
2. `git commit -m "规范的中文提交信息"`（说明改了什么、为什么）
3. **push 到远程 `origin/main`**

### 推送方式（重要，本环境特殊）

- 本机为 WSL + Clash 代理（`127.0.0.1:7897`）
- **WSL 版 git 推送 HTTPS 大包会失败**：报 `gnutls_handshake() failed: The TLS connection was non-properly terminated.`
- **必须使用 Windows 版 git 推送**：

  ```bash
  "/mnt/d/Program Files/Git/cmd/git.exe" push origin main
  # 或直接使用封装脚本：
  ./scripts/push.sh "提交信息"
  ```

- 提交本身可以用任一版本 git，推送务必用 Windows 版
- 推送成功后确认：`"/mnt/d/Program Files/Git/cmd/git.exe" ls-remote origin` 应显示最新 commit

## 环境信息

| 项目 | 值 |
|------|-----|
| Go | 1.25.1（Windows 版，`/mnt/c/Users/24358/.g/go/bin/go.exe`） |
| Node | v24.10.0（`/mnt/d/env/nodejs/node.exe`） |
| Go 代理 | `GOPROXY=https://goproxy.cn,direct`（proxy.golang.org 被墙） |
| 本地 git 身份 | 已配置：freewu / freewu@users.noreply.github.com（仓库级） |

## 提交前检查

- Go 代码：`gofmt` 格式、`go vet ./...`、`go test ./...` 全绿
- 前端 JS：`node --check <file>` 语法校验、`cd web && npm test`（Blockly→Lua 生成测试）
- 工作区干净（`git status` 无未提交改动）

## 项目要点速览

- 可视化规则引擎：Blockly 前端生成 Lua，Go 后端执行，规则 JSON 存储
- 后端：`internal/{config,rule,lua,server}`，入口 `cmd/rulego`
- 前端：`web/`（Blockly vendor 在 `web/vendor/blockly/`，离线可用）
- 自定义积木：`web/js/blocks.js` + `web/js/lua_gen.js`（Blockly 11 用 `forBlock` 注册生成器）
- 「事件」积木选项需与后端 `internal/rule.TriggerTypes` 保持一致
- 完整说明见 `README.md`
