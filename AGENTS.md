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
| just | 1.58.0 双平台：Windows 版 `D:\env\bin\just-1.58.0-x86_64-pc-windows-msvc\just.exe`；Linux 版 `/mnt/c/Users/24358/.local/bin/just`（WSL bash 直接 `just`）。justfile 按 `os()` 分支工具链/可执行文件；**just 1.58 在 Windows 上默认 shell 是 `sh`（Git Bash）** |
| Go 代理 | `GOPROXY=https://goproxy.cn,direct`（proxy.golang.org 被墙；justfile 已内置默认） |
| 本地 git 身份 | 已配置：freewu / freewu@users.noreply.github.com（仓库级） |

## 提交前检查

- Go 代码：`gofmt` 格式、`go vet ./...`、`go test ./...` 全绿
- 前端 JS：`node --check <file>` 语法校验、`just test-frontend`（Blockly→Lua 生成测试）
- 工作区干净（`git status` 无未提交改动）

## just 跨平台注意事项

- **Windows 终端**（cmd/PowerShell）直接输 `just` 即可（Windows 版 just）；无参数 `just` 列出可用命令
- **WSL bash** 直接输 `just`（Linux 版 just）
- **just 1.58 在 Windows 上优先用 `SHELL` 环境变量**确定 shell：若 SHELL 是 unix 路径（如 `/usr/bin/sh`，Git Bash 会话继承）会报 `could not find the shell sh: program not found`（与 PATH 无关）
- 已配置 PowerShell profile（`Documents\WindowsPowerShell\Microsoft.PowerShell_profile.ps1`）：将 `SHELL` 强制设为 `D:\Program Files\Git\bin\sh.exe` 并补齐 Git\bin 到 PATH；若仍报错 → 关闭重开 PowerShell，或临时执行 `$env:SHELL = "D:\Program Files\Git\bin\sh.exe"`
- build 产物名按平台：Windows 生成 `rulego.exe`、其他生成 `rulego`（`go build -o` 指定名字时不会自动加 .exe）；MSYS bash 的 PATH 不含当前目录，命令中本地二进制必须带 `./` 前缀（bin 变量已处理）
- 平台差异（go/node 路径、`rulego.exe` vs `./rulego`）由 justfile 中 `os()` 判断处理；所有命令统一 sh 语法

## 项目要点速览

- 可视化规则引擎：Blockly 前端生成 Lua，Go 后端执行，规则 JSON 存储
- 后端：`internal/{config,rule,lua,server}`，入口 `cmd/rulego`
- 前端：`web/`（Blockly vendor 在 `web/vendor/blockly/`，离线可用）
- 自定义积木：`web/js/blocks.js` + `web/js/lua_gen.js`（Blockly 11 用 `forBlock` 注册生成器）
- 「事件」积木选项需与后端 `internal/rule.TriggerTypes` 保持一致
- 完整说明见 `README.md`
