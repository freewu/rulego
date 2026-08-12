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
| just | 1.58.0 双平台：Windows 版 `D:\env\bin\just-1.58.0-x86_64-pc-windows-msvc\just.exe`；Linux 版 `/mnt/c/Users/24358/.local/bin/just`（WSL bash 直接 `just`）。justfile 按 `os()` 自动切换 shell：Windows→cmd、WSL/Linux→sh |
| Go 代理 | `GOPROXY=https://goproxy.cn,direct`（proxy.golang.org 被墙；justfile 已内置默认） |
| 本地 git 身份 | 已配置：freewu / freewu@users.noreply.github.com（仓库级） |

## 提交前检查

- Go 代码：`gofmt` 格式、`go vet ./...`、`go test ./...` 全绿
- 前端 JS：`node --check <file>` 语法校验、`just test-frontend`（Blockly→Lua 生成测试）
- 工作区干净（`git status` 无未提交改动）

## just 跨平台注意事项

- **Windows 终端**（cmd/PowerShell）直接输 `just` 即可（走 Windows 版，默认 cmd shell）
- **WSL bash** 直接输 `just`（走 Linux 版，默认 sh shell）；平台差异由 justfile 中 `os()` 判断处理
- ⚠️ **不要从 WSL 里启动 cmd 再跑 just**：WSL 启动的 cmd 进程会继承 `SHELL=/usr/bin/bash`，just 会误用 bash shell 执行 cmd 分支命令（如 `clean`）导致失败
- 若需在 Windows 侧验证 just，直接打开 cmd/PowerShell 窗口操作

## 项目要点速览

- 可视化规则引擎：Blockly 前端生成 Lua，Go 后端执行，规则 JSON 存储
- 后端：`internal/{config,rule,lua,server}`，入口 `cmd/rulego`
- 前端：`web/`（Blockly vendor 在 `web/vendor/blockly/`，离线可用）
- 自定义积木：`web/js/blocks.js` + `web/js/lua_gen.js`（Blockly 11 用 `forBlock` 注册生成器）
- 「事件」积木选项需与后端 `internal/rule.TriggerTypes` 保持一致
- 完整说明见 `README.md`
