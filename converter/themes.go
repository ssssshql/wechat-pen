package converter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ThemePackFile is the on-disk format for external theme packs (themes/*.json).
type ThemePackFile struct {
	// ID is the style key used by API/CLI (e.g. "my-brand"). Prefer lowercase kebab-case.
	ID string `json:"id"`
	// Name is shown in UI.
	Name string `json:"name"`
	// Description is a short one-line summary for UI / AI.
	Description string `json:"description,omitempty"`
	// Primary is the default primary color if user does not override.
	Primary string `json:"primary,omitempty"`
	// Base is the outer <section> inline style template.
	// Supports placeholders: {{fontSize}} {{lineHeight}} {{primary}}
	Base string `json:"base,omitempty"`
	// PreCode styles <code> inside <pre>.
	PreCode string `json:"preCode,omitempty"`
	// Tags maps HTML tag names to inline style templates.
	// Special keys: "p" for paragraphs (supports {{gap}} {{fontSize}} {{lineHeight}} {{align}} {{indent}} {{primary}}).
	Tags map[string]string `json:"tags,omitempty"`
	// Extends chooses a built-in pack as base before applying overrides.
	// One of: simple|magazine|tech|warm|dark|fresh|pink|mono|academic|chin
	// Empty defaults to simple.
	Extends string `json:"extends,omitempty"`
}

type themeRegistry struct {
	mu    sync.RWMutex
	byID  map[string]ThemePackFile
	order []string
}

var externalThemes = &themeRegistry{byID: map[string]ThemePackFile{}}

// LoadThemesDir loads all *.json theme packs from dir.
// Missing dir is not an error.
func LoadThemesDir(dir string) (int, error) {
	if strings.TrimSpace(dir) == "" {
		return 0, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	loaded := map[string]ThemePackFile{}
	var order []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".json") {
			continue
		}
		path := filepath.Join(dir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			return 0, fmt.Errorf("read theme %s: %w", path, err)
		}
		var pack ThemePackFile
		if err := json.Unmarshal(raw, &pack); err != nil {
			return 0, fmt.Errorf("parse theme %s: %w", path, err)
		}
		if pack.ID == "" {
			pack.ID = strings.TrimSuffix(name, filepath.Ext(name))
		}
		pack.ID = normalizeThemeID(pack.ID)
		if pack.ID == "" {
			continue
		}
		if pack.Name == "" {
			pack.Name = pack.ID
		}
		// validate not colliding with reserved builtin names silently override external only map
		loaded[pack.ID] = pack
		order = append(order, pack.ID)
	}

	externalThemes.mu.Lock()
	externalThemes.byID = loaded
	externalThemes.order = order
	externalThemes.mu.Unlock()
	return len(loaded), nil
}

// ListExternalThemes returns loaded external theme packs in stable order.
func ListExternalThemes() []ThemePackFile {
	externalThemes.mu.RLock()
	defer externalThemes.mu.RUnlock()
	out := make([]ThemePackFile, 0, len(externalThemes.order))
	for _, id := range externalThemes.order {
		if p, ok := externalThemes.byID[id]; ok {
			out = append(out, p)
		}
	}
	return out
}

// GetExternalTheme returns a pack by id.
func GetExternalTheme(id string) (ThemePackFile, bool) {
	externalThemes.mu.RLock()
	defer externalThemes.mu.RUnlock()
	p, ok := externalThemes.byID[normalizeThemeID(id)]
	return p, ok
}

// SaveThemePack writes a theme pack JSON into dir and reloads the registry.
// If pack.ID is empty, it is derived from pack.Name.
func SaveThemePack(dir string, pack ThemePackFile) (ThemePackFile, error) {
	if strings.TrimSpace(dir) == "" {
		return ThemePackFile{}, fmt.Errorf("themes dir is empty")
	}
	if pack.ID == "" {
		pack.ID = pack.Name
	}
	pack.ID = normalizeThemeID(pack.ID)
	if pack.ID == "" {
		return ThemePackFile{}, fmt.Errorf("theme id is required")
	}
	if isBuiltinStyleID(pack.ID) {
		return ThemePackFile{}, fmt.Errorf("cannot overwrite builtin theme %q", pack.ID)
	}
	if pack.Name == "" {
		pack.Name = pack.ID
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ThemePackFile{}, err
	}
	path := filepath.Join(dir, pack.ID+".json")
	raw, err := json.MarshalIndent(pack, "", "  ")
	if err != nil {
		return ThemePackFile{}, err
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return ThemePackFile{}, err
	}
	if _, err := LoadThemesDir(dir); err != nil {
		return ThemePackFile{}, err
	}
	return pack, nil
}

// DeleteThemePack removes themes/<id>.json and reloads the registry.
func DeleteThemePack(dir, id string) error {
	id = normalizeThemeID(id)
	if id == "" {
		return fmt.Errorf("theme id is required")
	}
	if isBuiltinStyleID(id) {
		return fmt.Errorf("cannot delete builtin theme %q", id)
	}
	path := filepath.Join(dir, id+".json")
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("theme %q not found", id)
		}
		return err
	}
	_, err := LoadThemesDir(dir)
	return err
}

func isBuiltinStyleID(id string) bool {
	switch normalizeThemeID(id) {
	case "simple", "magazine", "tech", "warm", "dark", "fresh", "pink", "mono", "academic", "chin":
		return true
	default:
		return false
	}
}

func normalizeThemeID(id string) string {
	id = strings.TrimSpace(strings.ToLower(id))
	id = strings.ReplaceAll(id, " ", "-")
	return id
}

func packFromExternal(cfg Config, pack ThemePackFile) stylePack {
	extends := StylePack(strings.ToLower(strings.TrimSpace(pack.Extends)))
	if extends == "" {
		extends = StyleSimple
	}
	// Build builtin base without recursing into custom/external.
	baseCfg := cfg
	baseCfg.Style = extends
	// Avoid external id being treated as builtin
	switch extends {
	case StyleMagazine, StyleTech, StyleWarm, StyleDark, StyleFresh, StylePink, StyleMono, StyleAcademic, StyleChin, StyleSimple:
	default:
		baseCfg.Style = StyleSimple
	}
	p := builtinPack(baseCfg)

	primary := cfg.PrimaryColor
	if primary == "" {
		primary = pack.Primary
	}
	if primary == "" {
		primary = defaultPrimary(baseCfg.Style)
	}

	if strings.TrimSpace(pack.Base) != "" {
		p.Base = pack.Base
	}
	if strings.TrimSpace(pack.PreCode) != "" {
		p.PreCode = pack.PreCode
	}
	if p.Tags == nil {
		p.Tags = map[string]string{}
	}
	for k, v := range pack.Tags {
		k = strings.TrimSpace(strings.ToLower(k))
		if k == "" || strings.TrimSpace(v) == "" {
			continue
		}
		switch k {
		case "base":
			p.Base = v
		case "pre_code", "precode":
			p.PreCode = v
		default:
			p.Tags[k] = v
		}
	}
	// Ensure primary available for later substitution via cfg
	_ = primary
	return p
}

func builtinPack(cfg Config) stylePack {
	primary := cfg.PrimaryColor
	if primary == "" {
		primary = defaultPrimary(cfg.Style)
	}
	switch cfg.Style {
	case StyleMagazine:
		return magazinePack(primary)
	case StyleTech:
		return techPack(primary)
	case StyleWarm:
		return warmPack(primary)
	case StyleDark:
		return darkPack(primary)
	case StyleFresh:
		return freshPack(primary)
	case StylePink:
		return pinkPack(primary)
	case StyleMono:
		return monoPack(primary)
	case StyleAcademic:
		return academicPack(primary)
	case StyleChin:
		return chinPack(primary)
	default:
		return simplePack(primary)
	}
}

// BuiltinStyleMeta is UI metadata for built-in packs.
type BuiltinStyleMeta struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Primary     string `json:"primary"`
	Builtin     bool   `json:"builtin"`
}

// ListBuiltinStyles returns built-in style metadata.
func ListBuiltinStyles() []BuiltinStyleMeta {
	return []BuiltinStyleMeta{
		{ID: "simple", Name: "简约", Description: "左边框章节 · 常规公众号排版", Primary: "#07c160", Builtin: true},
		{ID: "tech", Name: "科技", Description: "色条标题 · 暗底代码 · 胶囊标签", Primary: "#3b82f6", Builtin: true},
		{ID: "magazine", Name: "杂志", Description: "全居中衬线 · 无边框引用 · 装饰分割", Primary: "#c23a2b", Builtin: true},
		{ID: "warm", Name: "暖色", Description: "胶囊小标题 · 虚线分隔 · 软阴影图", Primary: "#d97706", Builtin: true},
		{ID: "dark", Name: "暗黑", Description: "整卡深色阅读 · 高亮描边", Primary: "#58a6ff", Builtin: true},
		{ID: "fresh", Name: "清新", Description: "实心色条 h2 · 浅绿清单底板", Primary: "#059669", Builtin: true},
		{ID: "pink", Name: "粉色", Description: "气泡式引用 · 圆角列表卡片", Primary: "#ec4899", Builtin: true},
		{ID: "mono", Name: "极简灰", Description: "等宽正文 · 线稿感 · 灰度图片", Primary: "#6b7280", Builtin: true},
		{ID: "academic", Name: "学术", Description: "论文体 · 居中标题 · 表注风格", Primary: "#1d4ed8", Builtin: true},
		{ID: "chin", Name: "中国风", Description: "宣纸底 · 双线标题 · 中文序号", Primary: "#c2410c", Builtin: true},
	}
}
