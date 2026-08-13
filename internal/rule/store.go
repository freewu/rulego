package rule

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Store 基于文件系统的规则存储：每个规则一个 JSON 文件，存放在 RulesDir 下。
// 版本历史以快照文件形式存放在 RulesDir/.versions/{id}/v{n}.json。
// 并发安全（内部有互斥锁）。
type Store struct {
	mu          sync.RWMutex
	dir         string
	index       map[string]string // id -> 磁盘上的绝对路径
	maxVersions int               // 每条规则保留的历史版本数量（0 = 不限制）
	changed     bool
}

// Option 是 Store 的构造选项。
type Option func(*Store)

// WithMaxVersions 设置每条规则保留的历史版本数量；超出时自动淘汰最旧版本。0 表示不限制。
func WithMaxVersions(n int) Option {
	return func(s *Store) {
		s.maxVersions = n
	}
}

// NewStore 创建规则存储，并扫描已有规则文件建立索引。
func NewStore(dir string, opts ...Option) (*Store, error) {
	s := &Store{dir: dir, index: map[string]string{}}
	for _, opt := range opts {
		opt(s)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("创建规则目录失败: %w", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("读取规则目录失败: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		r, err := s.readFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[store] 跳过损坏的规则文件 %s: %v\n", path, err)
			continue
		}
		s.index[r.ID] = path
	}
	return s, nil
}

func (s *Store) filePath(id string) string {
	return filepath.Join(s.dir, id+".json")
}

func (s *Store) readFile(path string) (*Rule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Rule
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// List 返回全部规则（按 ID 排序）。
func (s *Store) List() ([]*Rule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]string, 0, len(s.index))
	for id := range s.index {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	rules := make([]*Rule, 0, len(ids))
	for _, id := range ids {
		r, err := s.readFile(s.index[id])
		if err != nil {
			continue
		}
		rules = append(rules, r)
	}
	return rules, nil
}

// ListEnabled 返回所有启用的规则。
func (s *Store) ListEnabled() ([]*Rule, error) {
	all, err := s.List()
	if err != nil {
		return nil, err
	}
	out := all[:0]
	for _, r := range all {
		if r.Enabled {
			out = append(out, r)
		}
	}
	return out, nil
}

// Get 按 ID 查询规则。
func (s *Store) Get(id string) (*Rule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	path, ok := s.index[id]
	if !ok {
		return nil, ErrNotFound
	}
	return s.readFile(path)
}

// Create 新增规则，ID 为空时自动生成，并保存 v1 版本快照。
func (s *Store) Create(r *Rule) (*Rule, error) {
	r.Normalize()
	if err := r.Validate(); err != nil {
		return nil, err
	}
	if _, err := s.Get(r.ID); err == nil {
		return nil, fmt.Errorf("规则 %s 已存在", r.ID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.writeFileLocked(r); err != nil {
		return nil, err
	}
	s.index[r.ID] = s.filePath(r.ID)
	if err := s.saveVersionLocked(r.ID, r.Version, r); err != nil {
		return nil, err
	}
	s.trimVersionsLocked(r.ID)
	return r, nil
}

// Update 更新规则：ID 不变，版本自增；更新前先将当前版本保存为历史快照。
func (s *Store) Update(id string, r *Rule) (*Rule, error) {
	if id == "" {
		return nil, errors.New("规则 ID 不能为空")
	}
	old, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	// 保留不可变字段
	r.ID = old.ID
	r.CreatedAt = old.CreatedAt
	r.Version = old.Version
	// 保留/补全软件版本号
	if r.EngineVersion == "" {
		r.EngineVersion = old.EngineVersion
	}
	if r.EngineVersion == "" {
		r.EngineVersion = EngineVersion
	}
	if err := r.Validate(); err != nil {
		return nil, err
	}
	r.Touch()
	s.mu.Lock()
	defer s.mu.Unlock()
	// 更新前保存旧版本快照（old.Version 对应的内容，即当前主文件）
	if err := s.saveVersionLocked(id, old.Version, old); err != nil {
		return nil, err
	}
	if err := s.writeFileLocked(r); err != nil {
		return nil, err
	}
	s.trimVersionsLocked(id)
	return r, nil
}

// Delete 删除规则（连同版本历史目录）。
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path, ok := s.index[id]
	if !ok {
		return ErrNotFound
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	delete(s.index, id)
	// 清理版本历史目录
	if err := os.RemoveAll(s.versionsDir(id)); err != nil {
		return err
	}
	return nil
}

func (s *Store) writeFileLocked(r *Rule) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化规则失败: %w", err)
	}
	tmp := s.filePath(r.ID) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.filePath(r.ID))
}

// ErrNotFound 表示规则不存在。
var ErrNotFound = errors.New("规则不存在")
