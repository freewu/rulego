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

func TestRuleEngineVersion(t *testing.T) {
	s := newTestServer(t)
	rec, m := doReq(t, s, "POST", "/api/rules", sampleRule())
	if rec.Code != http.StatusCreated {
		t.Fatalf("创建状态码 = %d", rec.Code)
	}
	if m["engine_version"] != rule.EngineVersion {
		t.Errorf("engine_version = %v, want %s", m["engine_version"], rule.EngineVersion)
	}
	// 更新时 body 不带 engine_version 应保留旧值
	up := sampleRule()
	up.Name = "v2"
	rec2, m2 := doReq(t, s, "PUT", "/api/rules/r_test", up)
	if rec2.Code != http.StatusOK || m2["engine_version"] != rule.EngineVersion {
		t.Errorf("更新后 engine_version = %v", m2["engine_version"])
	}
}

func TestExportRules(t *testing.T) {
	s := newTestServer(t)
	doReq(t, s, "POST", "/api/rules", sampleRule())

	// 导出全部
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/rules/export", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("导出全部状态码 = %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "rules_export.json") {
		t.Errorf("缺少下载头: %v", rec.Header().Get("Content-Disposition"))
	}
	if !strings.Contains(rec.Body.String(), "r_test") {
		t.Errorf("导出全部缺少规则: %s", rec.Body.String())
	}

	// 导出单条
	rec2, m2 := doReq(t, s, "POST", "/api/rules/r_test/export", nil)
	if rec2.Code != http.StatusOK || m2["id"] != "r_test" {
		t.Errorf("导出单条 = %d %v", rec2.Code, m2)
	}
	if !strings.Contains(rec2.Header().Get("Content-Disposition"), "r_test.json") {
		t.Errorf("单条导出缺少下载头: %v", rec2.Header().Get("Content-Disposition"))
	}
}

func TestImportRules(t *testing.T) {
	s := newTestServer(t)
	doReq(t, s, "POST", "/api/rules", sampleRule())

	// 导入数组：r_new 新增 + r_test 覆盖
	newRule := sampleRule()
	newRule.ID = "r_new"
	newRule.Name = "导入的新规则"
	upRule := sampleRule()
	upRule.Name = "被覆盖的规则"
	rec, m := doReq(t, s, "POST", "/api/rules/import", []*rule.Rule{newRule, upRule})
	if rec.Code != http.StatusOK {
		t.Fatalf("导入状态码 = %d, body=%s", rec.Code, rec.Body.String())
	}
	if int(m["imported"].(float64)) != 1 || int(m["updated"].(float64)) != 1 || int(m["skipped"].(float64)) != 0 {
		t.Errorf("导入统计 = %v", m)
	}

	// mode=skip：已存在的跳过
	rec2, m2 := doReq(t, s, "POST", "/api/rules/import?mode=skip", upRule)
	if rec2.Code != http.StatusOK || int(m2["skipped"].(float64)) != 1 {
		t.Errorf("skip 导入统计 = %v", m2)
	}

	// 非法规则计入 failed
	bad := sampleRule()
	bad.ID = "r_bad_import"
	bad.Lua = ""
	rec3, m3 := doReq(t, s, "POST", "/api/rules/import", bad)
	if rec3.Code != http.StatusOK || int(m3["imported"].(float64)) != 0 {
		t.Errorf("非法导入统计 = %v", m3)
	}
	failed, ok := m3["failed"].([]interface{})
	if !ok || len(failed) != 1 {
		t.Errorf("failed = %v", m3["failed"])
	}

	// 空内容报错
	rec4 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec4, httptest.NewRequest("POST", "/api/rules/import", strings.NewReader("[]")))
	if rec4.Code != http.StatusBadRequest {
		t.Errorf("空导入状态码 = %d, want 400", rec4.Code)
	}
}

func TestDuplicateRule(t *testing.T) {
	s := newTestServer(t)
	doReq(t, s, "POST", "/api/rules", sampleRule())

	rec, m := doReq(t, s, "POST", "/api/rules/r_test/duplicate", nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("复制状态码 = %d, body=%s", rec.Code, rec.Body.String())
	}
	id, _ := m["id"].(string)
	if !strings.Contains(id, "_copy_") {
		t.Errorf("复制 ID = %s, want 含 _copy_", id)
	}
	if !strings.Contains(m["name"].(string), "副本") {
		t.Errorf("复制名称 = %v", m["name"])
	}
	if m["enabled"] != false {
		t.Errorf("副本应默认停用, enabled=%v", m["enabled"])
	}

	rec2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec2, httptest.NewRequest("GET", "/api/rules", nil))
	if !strings.Contains(rec2.Body.String(), "_copy_") {
		t.Errorf("列表应包含副本: %s", rec2.Body.String())
	}
}

// ---------- 版本历史 API ----------

func TestVersionsAPI(t *testing.T) {
	s := newTestServer(t)
	doReq(t, s, "POST", "/api/rules", sampleRule())

	// 更新两次产生历史
	up1 := sampleRule()
	up1.Name = "v2"
	doReq(t, s, "PUT", "/api/rules/r_test", up1)
	up2 := sampleRule()
	up2.Name = "v3"
	rec, m := doReq(t, s, "PUT", "/api/rules/r_test", up2)
	if rec.Code != http.StatusOK || int(m["version"].(float64)) != 3 {
		t.Fatalf("更新到 v3 = %d %v", rec.Code, m)
	}

	// 版本列表：历史 v1 v2（不含当前 v3）
	rec2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec2, httptest.NewRequest("GET", "/api/rules/r_test/versions", nil))
	if rec2.Code != http.StatusOK {
		t.Fatalf("版本列表状态码 = %d", rec2.Code)
	}
	var list []map[string]interface{}
	_ = json.Unmarshal(rec2.Body.Bytes(), &list)
	if len(list) != 2 || int(list[0]["version"].(float64)) != 1 || int(list[1]["version"].(float64)) != 2 {
		t.Errorf("版本列表 = %v, want [v1 v2]", list)
	}

	// 获取指定版本
	rec3, m3 := doReq(t, s, "GET", "/api/rules/r_test/versions/1", nil)
	if rec3.Code != http.StatusOK || m3["name"] != "测试规则" {
		t.Errorf("获取 v1 = %d %v", rec3.Code, m3)
	}
	// 当前版本也可按版本号获取
	rec4, m4 := doReq(t, s, "GET", "/api/rules/r_test/versions/3", nil)
	if rec4.Code != http.StatusOK || m4["name"] != "v3" {
		t.Errorf("获取 v3(当前) = %d %v", rec4.Code, m4)
	}
	// 不存在的版本 → 404
	rec5, _ := doReq(t, s, "GET", "/api/rules/r_test/versions/99", nil)
	if rec5.Code != http.StatusNotFound {
		t.Errorf("不存在的版本状态码 = %d, want 404", rec5.Code)
	}
}

func TestRestoreVersionAPI(t *testing.T) {
	s := newTestServer(t)
	doReq(t, s, "POST", "/api/rules", sampleRule())
	up := sampleRule()
	up.Name = "v2 内容"
	doReq(t, s, "PUT", "/api/rules/r_test", up)

	rec, m := doReq(t, s, "POST", "/api/rules/r_test/versions/1/restore", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("回滚状态码 = %d, body=%s", rec.Code, rec.Body.String())
	}
	if m["name"] != "测试规则" {
		t.Errorf("回滚后名称 = %v", m["name"])
	}
	if int(m["version"].(float64)) != 3 {
		t.Errorf("回滚后版本 = %v, want 3", m["version"])
	}
}

func TestRuleDiffAPI(t *testing.T) {
	s := newTestServer(t)
	doReq(t, s, "POST", "/api/rules", sampleRule())
	up := sampleRule()
	up.Name = "改名了"
	doReq(t, s, "PUT", "/api/rules/r_test", up)

	// v1 vs v2
	rec, m := doReq(t, s, "GET", "/api/rules/r_test/diff?v1=1&v2=2", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("diff 状态码 = %d, body=%s", rec.Code, rec.Body.String())
	}
	patch, ok := m["patch"].([]interface{})
	if !ok || len(patch) == 0 {
		t.Fatalf("patch = %v", m["patch"])
	}
	found := false
	for _, op := range patch {
		opm, _ := op.(map[string]interface{})
		if opm["path"] == "/name" && opm["op"] == "replace" {
			found = true
		}
	}
	if !found {
		t.Errorf("diff 缺少 name 修改: %v", patch)
	}

	// 缺少 v1 → 400
	rec2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec2, httptest.NewRequest("GET", "/api/rules/r_test/diff", nil))
	if rec2.Code != http.StatusBadRequest {
		t.Errorf("缺 v1 状态码 = %d, want 400", rec2.Code)
	}

	// 不存在的版本 → 404
	rec3 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec3, httptest.NewRequest("GET", "/api/rules/r_test/diff?v1=1&v2=99", nil))
	if rec3.Code != http.StatusNotFound {
		t.Errorf("diff 不存在版本状态码 = %d, want 404", rec3.Code)
	}
}
