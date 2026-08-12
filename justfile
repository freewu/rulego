# rulego 常用命令（跨平台：Windows 终端 / WSL / Linux）
# 平台说明：
# - Windows 终端：Windows 版 just，默认 shell = sh（Git Bash，需 PATH 含 Git\bin）
# - WSL / Linux：Linux 版 just，默认 shell = sh/bash
# 平台差异通过 os() 判断的变量自动处理，同一 justfile 两处通用

# 启用 unstable 特性以支持比较运算符（os() 平台判断）
set unstable
set lists := true

# Go 模块代理（proxy.golang.org 被墙，默认走 goproxy.cn）
export GOPROXY := env_var_or_default("GOPROXY", "https://goproxy.cn,direct")

# 工具链：Windows 终端直接走 PATH 中的 go/node；
# WSL 下 go/node 不在 PATH，使用绝对路径
go := if os() == "windows" { "go" } else { "/mnt/c/Users/24358/.g/go/bin/go.exe" }
node := if os() == "windows" { "node" } else { "/mnt/d/env/nodejs/node.exe" }

# 平台相关命令片段（just 1.58 在 Windows 上默认 shell 是 sh/Git Bash，
# 因此统一使用 sh 语法；仅可执行文件名与工具链路径有平台差异）
bin := if os() == "windows" { "rulego.exe" } else { "./rulego" }
rm_cmd := "rm -f rulego rulego.exe && rm -rf data"

# 无参数执行 `just` 时列出可用命令
@default:
    @just --list

# 构建
build:
    {{go}} build -o rulego ./cmd/rulego

# 运行（先构建，Ctrl+C 停止）
run: build
    @{{bin}} -c config.example.yaml

# 后端测试（配置 / 规则存储 / Lua 沙箱 / HTTP API）
test:
    {{go}} test ./...

# 前端测试（Blockly → Lua 生成验证，Node + jsdom）
test-frontend:
    cd web && {{node}} test/lua_gen.test.js

# 重新生成示例规则 JSON
examples:
    cd web && {{node}} test/gen_examples.js

# 清理构建产物与运行时数据
clean:
    @{{rm_cmd}}
