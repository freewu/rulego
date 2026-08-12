// Package rule 定义规则的数据模型（JSON 格式）与文件存储。
//
// 规则 JSON 结构：
//
//	{
//	  "id":          "r_1629780000",        // 规则唯一 ID
//	  "name":        "库存告警",              // 规则名称
//	  "description": "库存低于阈值时告警",     // 规则描述
//	  "enabled":     true,                  // 是否启用
//	  "version":     1,                     // 版本号（每次保存自增）
//	  "trigger":     "data.updated",        // 触发事件类型
//	  "workspace":   { ... },               // Blockly 工作区 JSON（用于可视化还原）
//	  "lua":         "function main(ctx) ... end", // Blockly 生成的 Lua 代码
//	  "created_at":  "2024-01-01T00:00:00Z",
//	  "updated_at":  "2024-01-01T00:00:00Z"
//	}
package rule

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// TriggerTypes 是后端支持的触发事件类型，前端 Blockly 事件积木的下拉选项与之一致。
var TriggerTypes = []string{
	"data.created",   // 数据创建
	"data.updated",   // 数据更新
	"data.deleted",   // 数据删除
	"timer.interval", // 定时触发
	"http.request",   // HTTP 请求触发
}

// Rule 表示一条可视化规则。
type Rule struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Enabled     bool            `json:"enabled"`
	Version     int             `json:"version"`
	Trigger     string          `json:"trigger"`
	Workspace   json.RawMessage `json:"workspace,omitempty"`
	Lua         string          `json:"lua"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

var idPattern = regexp.MustCompile(`^[a-zA-Z0-9_\-\.]{1,64}$`)

// Validate 校验规则必填字段与 Lua 代码基本合法性（编译检查由 lua 包完成）。
func (r *Rule) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return errors.New("规则 ID 不能为空")
	}
	if !idPattern.MatchString(r.ID) {
		return errors.New("规则 ID 只能包含字母、数字、下划线、中划线、点，长度不超过 64")
	}
	if strings.TrimSpace(r.Name) == "" {
		return errors.New("规则名称不能为空")
	}
	if r.Trigger == "" {
		return errors.New("触发事件不能为空")
	}
	if !contains(TriggerTypes, r.Trigger) {
		return fmt.Errorf("不支持的触发事件: %s", r.Trigger)
	}
	if strings.TrimSpace(r.Lua) == "" {
		return errors.New("Lua 代码不能为空，请先在 Blockly 中拖拽生成")
	}
	return nil
}

// Normalize 补全 ID（若为空则生成）、时间戳等字段。
func (r *Rule) Normalize() {
	now := time.Now().UTC()
	if r.ID == "" {
		r.ID = fmt.Sprintf("r_%d", now.UnixMilli())
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = now
	}
	r.UpdatedAt = now
	if r.Version < 1 {
		r.Version = 1
	}
}

// Touch 更新 UpdatedAt 并自增版本号。
func (r *Rule) Touch() {
	r.Version++
	r.UpdatedAt = time.Now().UTC()
}

// MarshalJSON 自定义序列化，保证时间为 RFC3339。
func (r Rule) MarshalJSON() ([]byte, error) {
	type alias Rule
	return json.Marshal(alias(r))
}

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}
