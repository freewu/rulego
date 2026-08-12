package rule

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(filepath.Join(t.TempDir(), "rules"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestStore_CRUD(t *testing.T) {
	s := newTestStore(t)

	// Create
	r := validRule()
	created, err := s.Create(r)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if created.Version != 1 {
		t.Errorf("版本 = %d, want 1", created.Version)
	}

	// 重复创建报错
	if _, err := s.Create(validRule()); err == nil {
		t.Error("重复 ID 应报错")
	}

	// Get
	got, err := s.Get(r.ID)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if got.Name != r.Name {
		t.Errorf("名称 = %s, want %s", got.Name, r.Name)
	}

	// Update
	up := validRule()
	up.Name = "改名了"
	up.Enabled = true
	updated, err := s.Update(r.ID, up)
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}
	if updated.Name != "改名了" {
		t.Errorf("更新后名称 = %s", updated.Name)
	}
	if updated.Version != 2 {
		t.Errorf("更新后版本 = %d, want 2", updated.Version)
	}
	if !updated.Enabled {
		t.Error("enabled 未更新")
	}

	// List / ListEnabled
	all, _ := s.List()
	if len(all) != 1 {
		t.Errorf("列表长度 = %d, want 1", len(all))
	}
	enabled, _ := s.ListEnabled()
	if len(enabled) != 1 {
		t.Errorf("启用列表长度 = %d, want 1", len(enabled))
	}

	// Delete
	if err := s.Delete(r.ID); err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	if _, err := s.Get(r.ID); err != ErrNotFound {
		t.Errorf("删除后应返回 ErrNotFound, got %v", err)
	}
}

func TestStore_NotFound(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.Get("nope"); err != ErrNotFound {
		t.Errorf("Get 不存在应返回 ErrNotFound, got %v", err)
	}
	if err := s.Delete("nope"); err != ErrNotFound {
		t.Errorf("Delete 不存在应返回 ErrNotFound, got %v", err)
	}
}

func TestStore_Reload(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "rules")
	s1, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	r := validRule()
	if _, err := s1.Create(r); err != nil {
		t.Fatal(err)
	}

	// 重新打开（模拟服务重启），规则应仍在
	s2, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	all, err := s2.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].ID != r.ID {
		t.Errorf("重启后规则丢失: %+v", all)
	}
}

func TestStore_SkipCorruptFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "rules")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "broken.json"), []byte("{not json"), 0o644)
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	all, _ := s.List()
	if len(all) != 0 {
		t.Errorf("损坏文件应被跳过, got %d", len(all))
	}
}

func TestStore_AutoID(t *testing.T) {
	s := newTestStore(t)
	r := &Rule{Name: "自动ID", Trigger: "http.request", Lua: "function main(ctx) end"}
	created, err := s.Create(r)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" {
		t.Error("自动生成 ID 失败")
	}
	if _, err := s.Get(created.ID); err != nil {
		t.Errorf("按自动 ID 查询失败: %v", err)
	}
}
