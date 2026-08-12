package rule

import (
	"encoding/json"
	"testing"
	"time"
)

func validRule() *Rule {
	return &Rule{
		ID:      "r_001",
		Name:    "库存告警",
		Trigger: "data.updated",
		Lua:     "function main(ctx) end",
	}
}

func TestValidate_OK(t *testing.T) {
	if err := validRule().Validate(); err != nil {
		t.Fatalf("合法规则校验失败: %v", err)
	}
}

func TestValidate_Errors(t *testing.T) {
	cases := []struct {
		name  string
		mut   func(*Rule)
		check string
	}{
		{"空ID", func(r *Rule) { r.ID = "" }, "ID"},
		{"非法ID", func(r *Rule) { r.ID = "bad id!" }, "ID"},
		{"空名称", func(r *Rule) { r.Name = "  " }, "名称"},
		{"空触发", func(r *Rule) { r.Trigger = "" }, "触发"},
		{"未知触发", func(r *Rule) { r.Trigger = "foo.bar" }, "触发"},
		{"空Lua", func(r *Rule) { r.Lua = "" }, "Lua"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := validRule()
			c.mut(r)
			if err := r.Validate(); err == nil {
				t.Fatalf("应校验失败: %s", c.name)
			}
		})
	}
}

func TestNormalizeAndTouch(t *testing.T) {
	r := &Rule{Name: "x", Trigger: "data.created", Lua: "function main(ctx) end"}
	r.Normalize()
	if r.ID == "" {
		t.Error("Normalize 应生成 ID")
	}
	if r.CreatedAt.IsZero() || r.UpdatedAt.IsZero() {
		t.Error("Normalize 应设置时间戳")
	}
	if r.Version != 1 {
		t.Errorf("初始版本 = %d, want 1", r.Version)
	}
	before := r.Version
	r.Touch()
	if r.Version != before+1 {
		t.Errorf("Touch 后版本 = %d, want %d", r.Version, before+1)
	}
}

func TestJSONRoundTrip(t *testing.T) {
	r := validRule()
	r.Workspace = json.RawMessage(`{"blocks":{"languageVersion":0}}`)
	r.Normalize()
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var back Rule
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if back.ID != r.ID || back.Name != r.Name {
		t.Errorf("JSON 往返不一致: %+v vs %+v", back, r)
	}
	if string(back.Workspace) != `{"blocks":{"languageVersion":0}}` {
		t.Errorf("workspace 丢失: %s", back.Workspace)
	}
}

func TestTriggerTypes(t *testing.T) {
	if len(TriggerTypes) == 0 {
		t.Error("触发事件列表不能为空")
	}
	// 保证所有触发类型在列表内可被校验通过
	for _, tt := range TriggerTypes {
		r := validRule()
		r.Trigger = tt
		if err := r.Validate(); err != nil {
			t.Errorf("触发类型 %s 校验失败: %v", tt, err)
		}
	}
}

func TestTimestampFormat(t *testing.T) {
	r := validRule()
	r.Normalize()
	data, _ := json.Marshal(r)
	var m map[string]interface{}
	_ = json.Unmarshal(data, &m)
	if _, ok := m["created_at"].(string); !ok {
		t.Errorf("created_at 应为字符串: %v", m["created_at"])
	}
	_, err := time.Parse(time.RFC3339, m["created_at"].(string))
	if err != nil {
		t.Errorf("created_at 不是 RFC3339: %v", err)
	}
}
