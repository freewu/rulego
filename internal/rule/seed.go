package rule

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
)

//go:embed seed/*.json
var seedFS embed.FS

// SeedExamplesIfEmpty 在存储为空（首次启动）时，将内置案例规则
// （由 web/test/gen_examples.js 生成并内嵌进二进制）写入存储。
// 存储中已有规则时不重复初始化。
func SeedExamplesIfEmpty(store *Store) error {
	existing, err := store.List()
	if err != nil {
		return fmt.Errorf("检查已有规则失败: %w", err)
	}
	if len(existing) > 0 {
		return nil
	}

	entries, err := seedFS.ReadDir("seed")
	if err != nil {
		return fmt.Errorf("读取内置案例失败: %w", err)
	}
	imported := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := seedFS.ReadFile("seed/" + e.Name())
		if err != nil {
			return fmt.Errorf("读取内置案例 %s 失败: %w", e.Name(), err)
		}
		var r Rule
		if err := json.Unmarshal(data, &r); err != nil {
			return fmt.Errorf("解析内置案例 %s 失败: %w", e.Name(), err)
		}
		if _, err := store.Create(&r); err != nil {
			log.Printf("初始化案例规则 %s 失败: %v", e.Name(), err)
			continue
		}
		imported++
	}
	log.Printf("已初始化 %d 条案例规则（存储为空，首次启动）", imported)
	return nil
}
