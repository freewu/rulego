// Package server 提供规则引擎的 HTTP API 服务与前端静态资源托管。
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"rulego/internal/lua"
	"rulego/internal/rule"
)

// Server 聚合配置、存储与 Lua 运行时。
type Server struct {
	cfg     *Config
	store   *rule.Store
	runtime *lua.Runtime
	mux     *http.ServeMux
	logger  *log.Logger
}

// Config 是 server 包的运行参数。
type Config struct {
	StaticDir string
}

// New 创建 HTTP 服务。
func New(cfg *Config, store *rule.Store, runtime *lua.Runtime) *Server {
	s := &Server{
		cfg:     cfg,
		store:   store,
		runtime: runtime,
		mux:     http.NewServeMux(),
		logger:  log.New(os.Stdout, "[rulego] ", log.LstdFlags),
	}
	s.routes()
	return s
}

// Handler 返回 HTTP 处理器（供启动或测试使用）。
func (s *Server) Handler() http.Handler {
	return logMiddleware(s.mux, s.logger)
}

func (s *Server) routes() {
	// 规则管理 API
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/triggers", s.handleTriggers)
	s.mux.HandleFunc("GET /api/rules", s.handleListRules)
	s.mux.HandleFunc("POST /api/rules", s.handleCreateRule)
	s.mux.HandleFunc("GET /api/rules/export", s.handleExportRules)
	s.mux.HandleFunc("POST /api/rules/import", s.handleImportRules)
	s.mux.HandleFunc("GET /api/rules/{id}", s.handleGetRule)
	s.mux.HandleFunc("PUT /api/rules/{id}", s.handleUpdateRule)
	s.mux.HandleFunc("DELETE /api/rules/{id}", s.handleDeleteRule)
	s.mux.HandleFunc("POST /api/rules/{id}/export", s.handleExportRule)
	s.mux.HandleFunc("POST /api/rules/{id}/duplicate", s.handleDuplicateRule)
	s.mux.HandleFunc("GET /api/rules/{id}/versions", s.handleListVersions)
	s.mux.HandleFunc("GET /api/rules/{id}/versions/{v}", s.handleGetVersion)
	s.mux.HandleFunc("POST /api/rules/{id}/versions/{v}/restore", s.handleRestoreVersion)
	s.mux.HandleFunc("GET /api/rules/{id}/diff", s.handleRuleDiff)
	s.mux.HandleFunc("POST /api/rules/{id}/run", s.handleRunRule)
	s.mux.HandleFunc("POST /api/validate", s.handleValidate)

	// 前端静态资源
	if s.cfg != nil && s.cfg.StaticDir != "" {
		dir := s.cfg.StaticDir
		s.mux.Handle("/", http.FileServer(http.Dir(dir)))
	}
}

// ---------- 处理器 ----------

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleTriggers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, rule.TriggerTypes)
}

func (s *Server) handleListRules(w http.ResponseWriter, r *http.Request) {
	rules, err := s.store.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

func (s *Server) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	var body rule.Rule
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	created, err := s.store.Create(&body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleGetRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	item, err := s.store.Get(id)
	if err != nil {
		if errors.Is(err, rule.ErrNotFound) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleUpdateRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body rule.Rule
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	updated, err := s.store.Update(id, &body)
	if err != nil {
		if errors.Is(err, rule.ErrNotFound) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.Delete(id); err != nil {
		if errors.Is(err, rule.ErrNotFound) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

// ---------- 导出 ----------

// handleExportRules 导出全部规则（JSON 数组，作为文件下载）。
func (s *Server) handleExportRules(w http.ResponseWriter, r *http.Request) {
	rules, err := s.store.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="rules_export.json"`)
	writeJSON(w, http.StatusOK, rules)
}

// handleExportRule 导出单条规则（JSON 文件下载）。
func (s *Server) handleExportRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	item, err := s.store.Get(id)
	if err != nil {
		if errors.Is(err, rule.ErrNotFound) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.json"`, id))
	writeJSON(w, http.StatusOK, item)
}

// ---------- 导入 ----------

// importMode 是导入时对已存在规则的策略。
type importMode int

const (
	modeOverwrite importMode = iota // 已存在则覆盖（默认）
	modeSkip                        // 已存在则跳过
)

// importResult 是导入结果统计。
type importResult struct {
	Imported int      `json:"imported"` // 新增
	Updated  int      `json:"updated"`  // 覆盖更新
	Skipped  int      `json:"skipped"`  // 跳过（已存在且 mode=skip）
	Failed   []string `json:"failed"`   // 失败原因列表
}

// handleImportRules 导入规则：请求体可为单条规则或规则数组。
// 查询参数 mode=overwrite|skip（默认 overwrite）。
func (s *Server) handleImportRules(w http.ResponseWriter, r *http.Request) {
	mode := modeOverwrite
	if r.URL.Query().Get("mode") == "skip" {
		mode = modeSkip
	}

	var raw json.RawMessage
	if err := decodeJSON(r, &raw); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var list []rule.Rule
	if len(raw) > 0 && raw[0] == '[' {
		if err := json.Unmarshal(raw, &list); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("解析规则数组失败: %w", err))
			return
		}
	} else {
		var single rule.Rule
		if err := json.Unmarshal(raw, &single); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("解析规则失败: %w", err))
			return
		}
		list = []rule.Rule{single}
	}

	if len(list) == 0 {
		writeError(w, http.StatusBadRequest, errors.New("导入内容为空"))
		return
	}

	result := importResult{Failed: []string{}}
	for i := range list {
		item := &list[i]
		_, err := s.store.Get(item.ID)
		switch {
		case err == nil: // 已存在
			if mode == modeSkip {
				result.Skipped++
				continue
			}
			if _, err := s.store.Update(item.ID, item); err != nil {
				result.Failed = append(result.Failed, fmt.Sprintf("%s: %v", item.ID, err))
				continue
			}
			result.Updated++
		case errors.Is(err, rule.ErrNotFound): // 新增
			if _, err := s.store.Create(item); err != nil {
				result.Failed = append(result.Failed, fmt.Sprintf("%s: %v", item.ID, err))
				continue
			}
			result.Imported++
		default:
			result.Failed = append(result.Failed, fmt.Sprintf("%s: %v", item.ID, err))
		}
	}
	writeJSON(w, http.StatusOK, result)
}

// ---------- 复制 ----------

// handleDuplicateRule 复制规则为新规则（新 ID、名称加“副本”后缀）。
func (s *Server) handleDuplicateRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	old, err := s.store.Get(id)
	if err != nil {
		if errors.Is(err, rule.ErrNotFound) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	copy := *old
	copy.ID = fmt.Sprintf("%s_copy_%d", old.ID, time.Now().UnixMilli())
	copy.Name = old.Name + "(副本)"
	copy.Enabled = false // 副本默认停用，避免误触发
	copy.CreatedAt = time.Time{}
	copy.Version = 0
	created, err := s.store.Create(&copy)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// ---------- 版本历史 ----------

// handleListVersions 返回规则的历史版本列表（不含当前版本）。
func (s *Server) handleListVersions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	list, err := s.store.ListVersions(id)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// handleGetVersion 返回规则指定版本的内容（v=当前版本时读主文件）。
func (s *Server) handleGetVersion(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	v, err := strconv.Atoi(r.PathValue("v"))
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("无效版本号: %s", r.PathValue("v")))
		return
	}
	item, err := s.store.GetVersion(id, v)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// handleRestoreVersion 回滚规则到指定版本（版本自增为新版本）。
func (s *Server) handleRestoreVersion(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	v, err := strconv.Atoi(r.PathValue("v"))
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("无效版本号: %s", r.PathValue("v")))
		return
	}
	restored, err := s.store.RestoreVersion(id, v)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, restored)
}

// handleRuleDiff 比较规则两个版本的 JSON 差异（RFC 6902 JSON Patch）。
// 查询参数：v1（基准）、v2（目标），默认比较 v1 与当前版本。
func (s *Server) handleRuleDiff(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cur, err := s.store.Get(id)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	v1, err := strconv.Atoi(r.URL.Query().Get("v1"))
	if err != nil || v1 < 1 {
		writeError(w, http.StatusBadRequest, errors.New("缺少或无效的 v1 参数"))
		return
	}
	v2 := cur.Version
	if q := r.URL.Query().Get("v2"); q != "" {
		v2, err = strconv.Atoi(q)
		if err != nil || v2 < 1 {
			writeError(w, http.StatusBadRequest, errors.New("无效的 v2 参数"))
			return
		}
	}
	patch, err := s.store.Diff(id, v1, v2)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"rule_id": id,
		"v1":      v1,
		"v2":      v2,
		"patch":   patch,
	})
}

// writeStoreErr 统一处理存储层错误 → HTTP 状态码。
func writeStoreErr(w http.ResponseWriter, err error) {
	if errors.Is(err, rule.ErrNotFound) {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeError(w, http.StatusInternalServerError, err)
}

// runRequest 是执行规则的请求体。
type runRequest struct {
	Data map[string]interface{} `json:"data"`
}

func (s *Server) handleRunRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	item, err := s.store.Get(id)
	if err != nil {
		if errors.Is(err, rule.ErrNotFound) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	var req runRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	res, err := s.runtime.Exec(ctx, item.Lua, req.Data)
	code := http.StatusOK
	if err != nil {
		code = http.StatusUnprocessableEntity
	}
	writeJSON(w, code, res)
}

// validateRequest 是 Lua 校验请求体。
type validateRequest struct {
	Lua string `json:"lua"`
}

func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	var req validateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.runtime.Check(req.Lua); err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"valid": false,
			"error": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"valid": true})
}

// ---------- 工具函数 ----------

func decodeJSON(r *http.Request, v interface{}) error {
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil {
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

// logMiddleware 打印请求日志。
func logMiddleware(next http.Handler, logger *log.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		// 静态资源不刷日志
		if !strings.HasPrefix(r.URL.Path, "/vendor/") && !strings.HasPrefix(r.URL.Path, "/css/") {
			logger.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
		}
	})
}

// ensureStaticExists 用于启动时校验静态目录（仅提示）。
func ensureStaticExists(dir string) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return
	}
	if _, err := os.Stat(abs); err != nil {
		log.Printf("[rulego] 警告: 静态目录 %s 不存在，前端页面将无法访问", abs)
	}
}
