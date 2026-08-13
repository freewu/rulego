package rule

import (
	"path/filepath"
	"testing"
)

func TestSeedExamplesIfEmpty(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "rules"))
	if err != nil {
		t.Fatal(err)
	}

	// 空存储 → 初始化 5 条内置案例
	if err := SeedExamplesIfEmpty(store); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	list, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 5 {
		t.Fatalf("案例规则数 = %d, want 5", len(list))
	}
	for _, r := range list {
		if r.EngineVersion != EngineVersion {
			t.Errorf("案例 %s engine_version = %q, want %q", r.ID, r.EngineVersion, EngineVersion)
		}
		if r.Workspace == nil || len(r.Workspace) == 0 {
			t.Errorf("案例 %s 缺少 workspace", r.ID)
		}
		if r.Lua == "" {
			t.Errorf("案例 %s 缺少 lua", r.ID)
		}
	}

	// 再次调用（已有规则）→ 不重复初始化
	if err := SeedExamplesIfEmpty(store); err != nil {
		t.Fatal(err)
	}
	list2, _ := store.List()
	if len(list2) != 5 {
		t.Fatalf("重复初始化后规则数 = %d, want 5", len(list2))
	}
}
