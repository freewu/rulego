package rule

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/wI2L/jsondiff"
)

// VersionInfo 是版本历史列表中的一条记录。
type VersionInfo struct {
	Version int       `json:"version"`
	SavedAt time.Time `json:"saved_at"`
	Size    int64     `json:"size"`
}

// versionsDir 返回规则版本历史目录：RulesDir/.versions/{id}。
func (s *Store) versionsDir(id string) string {
	return filepath.Join(s.dir, ".versions", id)
}

// versionPath 返回某规则某版本快照文件的路径。
func (s *Store) versionPath(id string, v int) string {
	return filepath.Join(s.versionsDir(id), fmt.Sprintf("v%d.json", v))
}

// saveVersionLocked 保存某版本快照（调用方须持写锁）。
// 同一版本号重复保存时直接覆盖（正常情况下版本号单调递增，不会重复）。
func (s *Store) saveVersionLocked(id string, v int, r *Rule) error {
	if v < 1 {
		return nil
	}
	dir := s.versionsDir(id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建版本目录失败: %w", err)
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化版本快照失败: %w", err)
	}
	path := s.versionPath(id, v)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// trimVersionsLocked 裁剪版本历史：超过 maxVersions（>0）时删除最旧版本快照。
func (s *Store) trimVersionsLocked(id string) {
	if s.maxVersions <= 0 {
		return
	}
	dir := s.versionsDir(id)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	vers := make([]int, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "v") || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(e.Name(), "v"), ".json"))
		if err != nil {
			continue
		}
		vers = append(vers, n)
	}
	sort.Ints(vers)
	// 保留最近 maxVersions 个，删除更旧的
	if len(vers) > s.maxVersions {
		for _, v := range vers[:len(vers)-s.maxVersions] {
			_ = os.Remove(s.versionPath(id, v))
		}
	}
}

// ListVersions 返回规则的历史版本列表（按版本号升序，不含当前版本）。
func (s *Store) ListVersions(id string) ([]VersionInfo, error) {
	if _, err := s.Get(id); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	dir := s.versionsDir(id)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []VersionInfo{}, nil
		}
		return nil, err
	}
	list := make([]VersionInfo, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "v") || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(e.Name(), "v"), ".json"))
		if err != nil {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		var r Rule
		if snap, err := s.readFile(s.versionPath(id, n)); err == nil {
			r = *snap
			list = append(list, VersionInfo{
				Version: n,
				SavedAt: r.UpdatedAt,
				Size:    info.Size(),
			})
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Version < list[j].Version })
	return list, nil
}

// GetVersion 返回规则指定版本的内容：历史版本读快照，当前版本读主文件。
func (s *Store) GetVersion(id string, version int) (*Rule, error) {
	if _, err := s.Get(id); err != nil {
		return nil, err
	}
	if version < 1 {
		return nil, fmt.Errorf("无效版本号: %d", version)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if version == s.currentVersionLocked(id) {
		return s.readFile(s.filePath(id))
	}
	path := s.versionPath(id, version)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: 版本 v%d 不存在", ErrNotFound, version)
		}
		return nil, err
	}
	return s.readFile(path)
}

// RestoreVersion 回滚规则到指定历史版本：以该版本内容作为新版本保存
// （版本号自增，回滚前的当前版本先入历史）。
func (s *Store) RestoreVersion(id string, version int) (*Rule, error) {
	snap, err := s.GetVersion(id, version)
	if err != nil {
		return nil, err
	}
	// 复用 Update：把历史版本内容作为新版本写入（版本自增、旧版本先入历史）
	return s.Update(id, snap)
}

// Diff 比较规则两个版本（v1 为基准、v2 为目标）的 JSON 差异，返回 RFC 6902 JSON Patch。
// 任一版本不存在时返回错误。
func (s *Store) Diff(id string, v1, v2 int) ([]jsondiff.Operation, error) {
	a, err := s.versionJSON(id, v1)
	if err != nil {
		return nil, err
	}
	b, err := s.versionJSON(id, v2)
	if err != nil {
		return nil, err
	}
	return jsondiff.CompareJSON(a, b)
}

// versionJSON 返回规则指定版本的原始 JSON 字节（当前版本 = 主文件）。
func (s *Store) versionJSON(id string, v int) ([]byte, error) {
	if _, err := s.Get(id); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if v < 1 {
		return nil, fmt.Errorf("无效版本号: %d", v)
	}
	path := s.filePath(id)
	if v != s.currentVersionLocked(id) {
		path = s.versionPath(id, v)
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("%w: 版本 v%d 不存在", ErrNotFound, v)
			}
			return nil, err
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// currentVersionLocked 返回规则当前版本号（读主文件，调用方须持锁）。
func (s *Store) currentVersionLocked(id string) int {
	r, err := s.readFile(s.filePath(id))
	if err != nil {
		return 0
	}
	return r.Version
}
