# rulego 常用命令（just 版，替代 Makefile）
set shell := ["bash", "-cu"]

# Go 模块代理（本环境 proxy.golang.org 被墙，默认走 goproxy.cn）
export GOPROXY := env_var_or_default("GOPROXY", "https://goproxy.cn,direct")

# 工具链：优先用 PATH 中的 go/node，WSL 下回退到已知绝对路径
go := shell("command -v go || echo /mnt/c/Users/24358/.g/go/bin/go.exe")
node := shell("command -v node || echo /mnt/d/env/nodejs/node.exe")

# 构建
build:
    {{go}} build -o rulego ./cmd/rulego

# 运行（先构建，Ctrl+C 停止）
run: build
    ./rulego -c config.example.yaml

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
    rm -f rulego
    rm -rf data
