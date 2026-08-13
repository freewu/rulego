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
# 注意：MSYS bash 的 PATH 不含当前目录，执行本地二进制必须带 ./ 前缀
bin := if os() == "windows" { "./rulego.exe" } else { "./rulego" }
rm_cmd := "rm -f rulego rulego.exe && rm -rf data"
# 结束服务（平台分支）：
# - Windows（Git Bash sh）：taskkill 在 PATH 中；用 //F //IM 避免 MSYS 把 /F 转成路径；
#   rulego.exe 与无扩展名 rulego 都匹配，覆盖 interop 启动的无扩展名进程
# - WSL/Linux：PATH 无 taskkill，经 cmd.exe 包装执行；pkill 兜底杀 WSL 侧直接启动的进程；
#   [r]ulego 正则技巧避免匹配到自身 shell
# 进程不存在时静默忽略
stop_cmd := if os() == "windows" {
    "taskkill //F //IM rulego.exe 2>/dev/null; taskkill //F //IM rulego 2>/dev/null; true"
} else {
    "cmd.exe /c \"taskkill /F /IM rulego.exe >nul 2>&1 & taskkill /F /IM rulego >nul 2>&1\"; pkill -f \"[r]ulego -c\" 2>/dev/null; true"
}

# 无参数执行 `just` 时列出可用命令
@default:
    @just --list

# 构建
build:
    {{go}} build -o {{bin}} ./cmd/rulego

# 运行（先构建，Ctrl+C 停止）
run: build
    @{{bin}} -c config.example.yaml

# 结束服务（停止 rulego 进程；进程不存在时静默忽略）
stop:
    @{{stop_cmd}}
    @echo "已停止 rulego 服务（可运行 just run 重新启动）"

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
