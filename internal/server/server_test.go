package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"rulego/internal/lua"
	"rulego/internal/rule"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	store, err := rule.NewStore(filepath.Join(t.TempDir(), "rules"))
	if err != nil {
		t.Fatal(err)
	}
	rt := lua.NewRuntime(5 * time.Second)
	return New(&Config{StaticDir: t.TempDir()}, store, rt)
}

func doReq(t *testing.T, s *Server, method, path string, body interface{}) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.SetPathValue("id", strings.TrimPrefix(strings.Split(path, "?")[0], "/api/rules/"))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	var m map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &m)
	return rec, m
}

func sampleRule() *rule.Rule {
	return &rule.Rule{
		ID:      "r_test",
		Name:    "测试规则",
		Trigger: "data.updated",
		Lua:     "function main(ctx)\n log(\"info\", \"stock=\" .. tostring(ctx[\"stock\"]))\n if ctx[\"stock\"] < 10 then alert(\"sms\", \"low\") end\nend",
		Enabled: true,
	}
}

func TestHealth(t *testing.T) {
	s := newTestServer(t)
	rec, m := doReq(t, s, "GET", "/api/health", nil)
	if rec.Code != http.StatusOK || m["status"] != "ok" {
		t.Errorf("health = %d %v", rec.Code, m)
	}
}

func TestTriggers(t *testing.T) {
	s := newTestServer(t)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/triggers", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("triggers 状态码 = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "data.updated") {
		t.Errorf("triggers 缺少 data.updated: %s", rec.Body.String())
	}
}

func TestRulesCRUD(t *testing.T) {
	s := newTestServer(t)

	// Create
	rec, m := doReq(t, s, "POST", "/api/rules", sampleRule())
	if rec.Code != http.StatusCreated {
		t.Fatalf("创建状态码 = %d, body=%s", rec.Code, rec.Body.String())
	}
	if m["id"] != "r_test" {
		t.Errorf("创建 id = %v", m["id"])
	}

	// List
	rec2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec2, httptest.NewRequest("GET", "/api/rules", nil))
	if rec2.Code != http.StatusOK {
		t.Fatalf("列表状态码 = %d", rec2.Code)
	}
	if !strings.Contains(rec2.Body.String(), "测试规则") {
		t.Errorf("列表缺少规则: %s", rec2.Body.String())
	}

	// Get
	rec3, m3 := doReq(t, s, "GET", "/api/rules/r_test", nil)
	if rec3.Code != http.StatusOK || m3["name"] != "测试规则" {
		t.Errorf("获取规则 = %d %v", rec3.Code, m3)
	}

	// Update
	up := sampleRule()
	up.Name = "更新后的规则"
	rec4, m4 := doReq(t, s, "PUT", "/api/rules/r_test", up)
	if rec4.Code != http.StatusOK || m4["name"] != "更新后的规则" {
		t.Errorf("更新规则 = %d %v", rec4.Code, m4)
	}
	if int(m4["version"].(float64)) != 2 {
		t.Errorf("更新后版本 = %v", m4["version"])
	}

	// Delete
	rec5, _ := doReq(t, s, "DELETE", "/api/rules/r_test", nil)
	if rec5.Code != http.StatusOK {
		t.Errorf("删除状态码 = %d", rec5.Code)
	}
	rec6, _ := doReq(t, s, "GET", "/api/rules/r_test", nil)
	if rec6.Code != http.StatusNotFound {
		t.Errorf("删除后查询状态码 = %d, want 404", rec6.Code)
	}
}

func TestRunRule(t *testing.T) {
	s := newTestServer(t)
	doReq(t, s, "POST", "/api/rules", sampleRule())

	rec, m := doReq(t, s, "POST", "/api/rules/r_test/run", map[string]interface{}{
		"data": map[string]interface{}{"stock": 5},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("执行状态码 = %d, body=%s", rec.Code, rec.Body.String())
	}
	outputs, ok := m["outputs"].([]interface{})
	if !ok || len(outputs) != 1 || outputs[0] != "[INFO] stock=5" {
		t.Errorf("outputs = %v", m["outputs"])
	}
	alerts, ok := m["alerts"].([]interface{})
	if !ok || len(alerts) != 1 {
		t.Errorf("alerts = %v", m["alerts"])
	}
	if m["error"] != nil && m["error"] != "" {
		t.Errorf("不应有错误: %v", m["error"])
	}
}

func TestRunRule_NotFound(t *testing.T) {
	s := newTestServer(t)
	rec, _ := doReq(t, s, "POST", "/api/rules/nope/run", map[string]interface{}{})
	if rec.Code != http.StatusNotFound {
		t.Errorf("不存在规则执行状态码 = %d, want 404", rec.Code)
	}
}

func TestRunRule_BadLua(t *testing.T) {
	s := newTestServer(t)
	bad := sampleRule()
	bad.ID = "r_bad"
	bad.Lua = "function main(ctx" // 语法错误
	doReq(t, s, "POST", "/api/rules", bad)

	rec, _ := doReq(t, s, "POST", "/api/rules/r_bad/run", map[string]interface{}{
		"data": map[string]interface{}{},
	})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("坏 Lua 执行状态码 = %d, want 422", rec.Code)
	}
}

func TestValidate(t *testing.T) {
	s := newTestServer(t)
	rec, m := doReq(t, s, "POST", "/api/validate", map[string]interface{}{
		"lua": "function main(ctx) end",
	})
	if rec.Code != http.StatusOK || m["valid"] != true {
		t.Errorf("合法 Lua 校验 = %d %v", rec.Code, m)
	}
	rec2, m2 := doReq(t, s, "POST", "/api/validate", map[string]interface{}{
		"lua": "function main(ctx",
	})
	if rec2.Code != http.StatusOK || m2["valid"] != false {
		t.Errorf("非法 Lua 校验 = %v", m2)
	}
}

func TestCreateRule_Validation(t *testing.T) {
	s := newTestServer(t)
	rec, m := doReq(t, s, "POST", "/api/rules", map[string]interface{}{
		"id": "r_x", "name": "", "trigger": "data.updated", "lua": "function main(ctx) end",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("非法规则状态码 = %d, want 400, body=%s", rec.Code, rec.Body.String())
	}
	if m["error"] == "" {
		t.Error("应返回错误信息")
	}
}
