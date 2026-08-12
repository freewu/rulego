// Package server 提供规则引擎的 HTTP API 服务与前端静态资源托管。
package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
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
	s.mux.HandleFunc("GET /api/rules/{id}", s.handleGetRule)
	s.mux.HandleFunc("PUT /api/rules/{id}", s.handleUpdateRule)
	s.mux.HandleFunc("DELETE /api/rules/{id}", s.handleDeleteRule)
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
