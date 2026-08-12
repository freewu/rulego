// Package lua 封装 gopher-lua 运行时，提供规则 Lua 代码的沙箱执行能力：
//   - 只开放安全的基础库（base/table/string/math/utf8），不开放 os/io/debug/package
//   - 移除 base 库中危险函数（dofile/loadfile/load/print），防止读写文件
//   - 注入 ctx 表作为规则输入上下文
//   - 提供 log / alert 两个 Go 回调，供规则向宿主输出日志与告警
//   - 支持执行超时控制
package lua

import (
	"context"
	"fmt"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// Alert 表示规则通过 alert() 产生的告警记录。
type Alert struct {
	Channel string `json:"channel"`
	Message string `json:"message"`
}

// Result 是规则执行结果。
type Result struct {
	Outputs    []string    `json:"outputs"`     // log() 输出
	Alerts     []Alert     `json:"alerts"`      // alert() 产生的告警
	Return     interface{} `json:"return"`      // 规则返回值
	DurationMS int64       `json:"duration_ms"` // 执行耗时
	Error      string      `json:"error,omitempty"`
}

// Runtime 是规则 Lua 执行器。
type Runtime struct {
	timeout time.Duration
}

// NewRuntime 创建执行器，timeout 为单次执行的超时时间。
func NewRuntime(timeout time.Duration) *Runtime {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Runtime{timeout: timeout}
}

// Check 只做语法编译检查，不执行。
func (rt *Runtime) Check(code string) error {
	L := newSandboxState()
	defer L.Close()
	if _, err := L.LoadString(code); err != nil {
		return fmt.Errorf("Lua 语法错误: %w", err)
	}
	return nil
}

// Exec 在沙箱中执行规则 Lua 代码。
// code 应为 `function main(ctx) ... end` 形式，本函数负责调用 main(ctx)。
// data 作为 ctx 表注入，规则内可通过 log()/alert() 向宿主输出。
func (rt *Runtime) Exec(ctx context.Context, code string, data map[string]interface{}) (*Result, error) {
	start := time.Now()
	res := &Result{}

	L := newSandboxState()
	defer L.Close()

	// 超时控制：通过 gopher-lua 的 SetContext 实现可抢占式终止
	execCtx, cancel := context.WithTimeout(ctx, rt.timeout)
	defer cancel()
	L.SetContext(execCtx)

	// log(level, msg) —— 追加到 Outputs
	L.SetGlobal("log", L.NewFunction(func(L *lua.LState) int {
		level := strings.ToUpper(L.CheckString(1))
		msg := L.CheckString(2)
		res.Outputs = append(res.Outputs, fmt.Sprintf("[%s] %s", level, msg))
		return 0
	}))
	// alert(channel, msg) —— 追加到 Alerts
	L.SetGlobal("alert", L.NewFunction(func(L *lua.LState) int {
		channel := L.CheckString(1)
		msg := L.CheckString(2)
		res.Alerts = append(res.Alerts, Alert{Channel: channel, Message: msg})
		return 0
	}))

	fn, err := L.LoadString(code)
	if err != nil {
		return nil, fmt.Errorf("Lua 语法错误: %w", err)
	}
	L.Push(fn)
	if err := L.PCall(0, lua.MultRet, nil); err != nil {
		return nil, fmt.Errorf("加载规则失败: %w", err)
	}

	mainFn := L.GetGlobal("main")
	if mainFn.Type() != lua.LTFunction {
		return nil, fmt.Errorf("规则必须定义 main(ctx) 函数")
	}

	// 注入 ctx 表并调用 main(ctx)
	ctxTable := toLuaTable(L, data)
	L.Push(mainFn)
	L.Push(ctxTable)

	if err := L.PCall(1, 1, nil); err != nil {
		res.Error = err.Error()
		if execCtx.Err() != nil {
			res.Error = fmt.Sprintf("执行超时（超过 %s）", rt.timeout)
		}
		return res, fmt.Errorf("%s", res.Error)
	}
	if ret := L.Get(-1); ret != lua.LNil {
		res.Return = fromLuaValue(ret)
	}

	res.DurationMS = time.Since(start).Milliseconds()
	return res, nil
}

// newSandboxState 创建受限的 Lua 状态机。
func newSandboxState() *lua.LState {
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	// 只开放安全库
	for _, pair := range []struct {
		n string
		f lua.LGFunction
	}{
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		{lua.StringLibName, lua.OpenString},
		{lua.MathLibName, lua.OpenMath},
	} {
		L.Push(L.NewFunction(pair.f))
		L.Push(lua.LString(pair.n))
		L.Call(1, 0)
	}
	// 移除危险函数
	for _, name := range []string{"dofile", "loadfile", "load", "print"} {
		L.SetGlobal(name, lua.LNil)
	}
	return L
}

// toLuaTable 将 Go map 递归转换为 Lua 表。
func toLuaTable(L *lua.LState, data map[string]interface{}) *lua.LTable {
	t := L.NewTable()
	if data == nil {
		return t
	}
	for k, v := range data {
		t.RawSetString(k, toLuaValue(L, v))
	}
	return t
}

func toLuaValue(L *lua.LState, v interface{}) lua.LValue {
	switch val := v.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(val)
	case string:
		return lua.LString(val)
	case int:
		return lua.LNumber(val)
	case int64:
		return lua.LNumber(val)
	case float64:
		return lua.LNumber(val)
	case float32:
		return lua.LNumber(val)
	case []interface{}:
		t := L.NewTable()
		for i, item := range val {
			t.RawSetInt(i+1, toLuaValue(L, item))
		}
		return t
	case map[string]interface{}:
		t := L.NewTable()
		for k, item := range val {
			t.RawSetString(k, toLuaValue(L, item))
		}
		return t
	default:
		return lua.LString(fmt.Sprintf("%v", val))
	}
}

// fromLuaValue 将 Lua 返回值转换为可 JSON 序列化的 Go 值。
func fromLuaValue(v lua.LValue) interface{} {
	switch val := v.(type) {
	case *lua.LNilType:
		return nil
	case lua.LBool:
		return bool(val)
	case lua.LNumber:
		return float64(val)
	case lua.LString:
		return string(val)
	case *lua.LTable:
		// 优先尝试数组
		n := val.Len()
		if n > 0 {
			arr := make([]interface{}, 0, n)
			for i := 1; i <= n; i++ {
				arr = append(arr, fromLuaValue(val.RawGetInt(i)))
			}
			return arr
		}
		m := map[string]interface{}{}
		val.ForEach(func(k, item lua.LValue) {
			m[fmt.Sprintf("%v", fromLuaValue(k))] = fromLuaValue(item)
		})
		return m
	default:
		return fmt.Sprintf("%v", val)
	}
}
