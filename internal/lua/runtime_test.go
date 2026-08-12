package lua

import (
	"context"
	"testing"
	"time"
)

const baseRule = `function main(ctx)
	log("info", "hello " .. tostring(ctx["name"]))
	if ctx["stock"] < 10 then
		alert("email", "库存不足: " .. tostring(ctx["stock"]))
	end
end
`

func TestExec_Basic(t *testing.T) {
	rt := NewRuntime(5 * time.Second)
	res, err := rt.Exec(context.Background(), baseRule, map[string]interface{}{
		"name":  "warehouse-a",
		"stock": 3,
	})
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}
	if len(res.Outputs) != 1 || res.Outputs[0] != "[INFO] hello warehouse-a" {
		t.Errorf("Outputs = %v", res.Outputs)
	}
	if len(res.Alerts) != 1 {
		t.Fatalf("Alerts = %v, want 1", res.Alerts)
	}
	if res.Alerts[0].Channel != "email" || res.Alerts[0].Message != "库存不足: 3" {
		t.Errorf("Alert = %+v", res.Alerts[0])
	}
	if res.DurationMS < 0 {
		t.Error("耗时不能为负")
	}
}

func TestExec_NoAlert(t *testing.T) {
	rt := NewRuntime(5 * time.Second)
	res, err := rt.Exec(context.Background(), baseRule, map[string]interface{}{
		"name":  "warehouse-b",
		"stock": 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Alerts) != 0 {
		t.Errorf("库存充足不应告警: %v", res.Alerts)
	}
}

func TestExec_SyntaxError(t *testing.T) {
	rt := NewRuntime(5 * time.Second)
	_, err := rt.Exec(context.Background(), "function main(ctx", nil)
	if err == nil {
		t.Fatal("语法错误应报错")
	}
}

func TestExec_MissingMain(t *testing.T) {
	rt := NewRuntime(5 * time.Second)
	_, err := rt.Exec(context.Background(), "local x = 1", nil)
	if err == nil || err.Error() != "规则必须定义 main(ctx) 函数" {
		t.Fatalf("缺失 main 应报错, got %v", err)
	}
}

func TestExec_ReturnValue(t *testing.T) {
	code := `function main(ctx)
		return ctx["a"] + ctx["b"]
	end`
	rt := NewRuntime(5 * time.Second)
	res, err := rt.Exec(context.Background(), code, map[string]interface{}{"a": 2, "b": 3})
	if err != nil {
		t.Fatal(err)
	}
	if res.Return != float64(5) {
		t.Errorf("Return = %v, want 5", res.Return)
	}
}

func TestExec_Timeout(t *testing.T) {
	code := `function main(ctx)
		while true do end
	end`
	rt := NewRuntime(100 * time.Millisecond)
	_, err := rt.Exec(context.Background(), code, nil)
	if err == nil {
		t.Fatal("死循环应超时")
	}
}

func TestExec_SandboxBlocksFileIO(t *testing.T) {
	// os / io / dofile / loadfile / print 均不可用
	code := `function main(ctx)
		os.remove("/tmp/x")
	end`
	rt := NewRuntime(5 * time.Second)
	_, err := rt.Exec(context.Background(), code, nil)
	if err == nil {
		t.Fatal("应阻止 os 库调用")
	}

	code2 := `function main(ctx)
		dofile("/etc/passwd")
	end`
	if _, err := rt.Exec(context.Background(), code2, nil); err == nil {
		t.Fatal("应阻止 dofile 调用")
	}
}

func TestCheck(t *testing.T) {
	rt := NewRuntime(5 * time.Second)
	if err := rt.Check("function main(ctx) end"); err != nil {
		t.Errorf("合法代码校验失败: %v", err)
	}
	if err := rt.Check("function main(ctx"); err == nil {
		t.Error("非法代码应校验失败")
	}
}

func TestNestedData(t *testing.T) {
	code := `function main(ctx)
		local u = ctx["user"]
		log("info", u["name"] .. ":" .. tostring(u["age"]))
	end`
	rt := NewRuntime(5 * time.Second)
	res, err := rt.Exec(context.Background(), code, map[string]interface{}{
		"user": map[string]interface{}{"name": "张三", "age": 30},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Outputs) != 1 || res.Outputs[0] != "[INFO] 张三:30" {
		t.Errorf("嵌套数据输出 = %v", res.Outputs)
	}
}
