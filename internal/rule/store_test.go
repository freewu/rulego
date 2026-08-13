package rule

import (
	"errors"
	"fmt"
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

// ---------- 版本历史 ----------

func TestStore_Versions(t *testing.T) {
	s := newTestStore(t)

	// 创建 → v1 快照
	r := validRule()
	if _, err := s.Create(r); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListVersions(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Version != 1 {
		t.Fatalf("创建后版本历史 = %+v, want [v1]", list)
	}

	// 更新两次 → 历史 v1, v2
	up1 := validRule()
	up1.Name = "第二次"
	if _, err := s.Update(r.ID, up1); err != nil {
		t.Fatal(err)
	}
	up2 := validRule()
	up2.Name = "第三次"
	if _, err := s.Update(r.ID, up2); err != nil {
		t.Fatal(err)
	}

	cur, _ := s.Get(r.ID)
	if cur.Version != 3 {
		t.Errorf("当前版本 = %d, want 3", cur.Version)
	}
	// 历史 = 过去版本快照（不含当前主文件）
	list, _ = s.ListVersions(r.ID)
	if len(list) != 2 {
		t.Fatalf("版本历史数量 = %d, want 2: %+v", len(list), list)
	}
	if list[0].Version != 1 || list[1].Version != 2 {
		t.Errorf("版本历史 = %+v, want [v1 v2]", list)
	}

	// 读取历史版本内容
	v1, err := s.GetVersion(r.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if v1.Name != validRule().Name {
		t.Errorf("v1 名称 = %s, want %s", v1.Name, validRule().Name)
	}
	v2, _ := s.GetVersion(r.ID, 2)
	if v2.Name != "第二次" {
		t.Errorf("v2 名称 = %s, want 第二次", v2.Name)
	}
	// 当前版本也按版本号读取（读主文件）
	v3, _ := s.GetVersion(r.ID, 3)
	if v3.Name != "第三次" {
		t.Errorf("v3(当前) 名称 = %s, want 第三次", v3.Name)
	}

	// 不存在的版本
	if _, err := s.GetVersion(r.ID, 99); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetVersion(99) err = %v, want ErrNotFound", err)
	}
}

func TestStore_RestoreVersion(t *testing.T) {
	s := newTestStore(t)
	r := validRule()
	if _, err := s.Create(r); err != nil {
		t.Fatal(err)
	}
	up1 := validRule()
	up1.Name = "v2 内容"
	if _, err := s.Update(r.ID, up1); err != nil {
		t.Fatal(err)
	}
	up2 := validRule()
	up2.Name = "v3 内容"
	if _, err := s.Update(r.ID, up2); err != nil {
		t.Fatal(err)
	}

	// 回滚到 v1
	restored, err := s.RestoreVersion(r.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Name != validRule().Name {
		t.Errorf("回滚后名称 = %s", restored.Name)
	}
	if restored.Version != 4 {
		t.Errorf("回滚后版本 = %d, want 4（自增为新版本）", restored.Version)
	}

	// 回滚内容成为当前版本（v4），历史完整
	v4, err := s.GetVersion(r.ID, 4)
	if err != nil {
		t.Fatal(err)
	}
	if v4.Name != validRule().Name {
		t.Errorf("v4(当前) 名称 = %s, want 回滚前 v1 内容", v4.Name)
	}
	cur, _ := s.Get(r.ID)
	if cur.Name != validRule().Name {
		t.Errorf("当前名称 = %s", cur.Name)
	}
	list, _ := s.ListVersions(r.ID)
	if len(list) != 3 {
		t.Errorf("历史版本数量 = %d, want 3（v1 v2 v3）: %+v", len(list), list)
	}
}

func TestStore_VersionLimit(t *testing.T) {
	s, err := NewStore(filepath.Join(t.TempDir(), "rules"), WithMaxVersions(2))
	if err != nil {
		t.Fatal(err)
	}
	r := validRule()
	if _, err := s.Create(r); err != nil {
		t.Fatal(err)
	}
	// 连续更新 5 次（版本到 6）
	for i := 0; i < 5; i++ {
		up := validRule()
		up.Name = fmt.Sprintf("v%d", i+2)
		if _, err := s.Update(r.ID, up); err != nil {
			t.Fatal(err)
		}
	}
	list, _ := s.ListVersions(r.ID)
	if len(list) != 2 {
		t.Fatalf("裁剪后版本数量 = %d, want 2: %+v", len(list), list)
	}
	// 保留的是最近的历史快照 v4, v5（v6 为当前版本不在历史中）
	if list[0].Version != 4 || list[1].Version != 5 {
		t.Errorf("保留的版本 = %+v, want [v4 v5]", list)
	}
	// 最旧的已被淘汰
	if _, err := s.GetVersion(r.ID, 1); !errors.Is(err, ErrNotFound) {
		t.Errorf("v1 应已被淘汰, err = %v", err)
	}
}

func TestStore_Diff(t *testing.T) {
	s := newTestStore(t)
	r := validRule()
	if _, err := s.Create(r); err != nil {
		t.Fatal(err)
	}
	up := validRule()
	up.Name = "改名了"
	up.Description = "新增描述"
	if _, err := s.Update(r.ID, up); err != nil {
		t.Fatal(err)
	}

	ops, err := s.Diff(r.ID, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) == 0 {
		t.Fatal("diff 应为空")
	}
	// 找到 name 修改操作
	found := false
	for _, op := range ops {
		if op.Path == "/name" && op.Type == "replace" {
			found = true
		}
	}
	if !found {
		t.Errorf("diff 缺少 name 修改: %+v", ops)
	}

	// 相同版本 diff 为空
	ops2, err := s.Diff(r.ID, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(ops2) != 0 {
		t.Errorf("相同版本 diff = %+v, want 空", ops2)
	}

	// 不存在的版本报错
	if _, err := s.Diff(r.ID, 1, 99); !errors.Is(err, ErrNotFound) {
		t.Errorf("Diff(1,99) err = %v, want ErrNotFound", err)
	}
}

func TestStore_DeleteVersions(t *testing.T) {
	s := newTestStore(t)
	r := validRule()
	if _, err := s.Create(r); err != nil {
		t.Fatal(err)
	}
	up := validRule()
	up.Name = "v2"
	if _, err := s.Update(r.ID, up); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(r.ID); err != nil {
		t.Fatal(err)
	}
	// 版本历史目录应一并清理
	dir := s.versionsDir(r.ID)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("删除规则后版本目录应不存在: %v", err)
	}
}
