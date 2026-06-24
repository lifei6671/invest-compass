package prompt

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed builtin/*.md
var packagedBuiltinPrompts embed.FS

// BuiltinPromptStore 是内置模板 seed 依赖的数据访问边界。
type BuiltinPromptStore interface {
	GetPromptTemplateByKey(ctx context.Context, key string) (Template, bool, error)
	SavePromptTemplate(ctx context.Context, template Template) error
}

// LoadPackagedBuiltinPromptTemplates 读取随 Go core 打包的内置 Prompt 模板。
func LoadPackagedBuiltinPromptTemplates() ([]Template, error) {
	return LoadBuiltinPromptTemplates(packagedBuiltinPrompts)
}

// LoadBuiltinPromptTemplates 从 Markdown front matter 读取只读内置模板。
func LoadBuiltinPromptTemplates(fsys fs.FS) ([]Template, error) {
	paths, err := fs.Glob(fsys, "builtin/*.md")
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)

	templates := make([]Template, 0, len(paths))
	for _, path := range paths {
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return nil, err
		}
		template, err := parseBuiltinPromptMarkdown(string(data))
		if err != nil {
			return nil, fmt.Errorf("parse builtin prompt %s: %w", path, err)
		}
		templates = append(templates, template)
	}
	return templates, nil
}

// SeedBuiltinPromptTemplates 按稳定 key 初始化或更新内置模板，不覆盖用户模板内容。
func SeedBuiltinPromptTemplates(ctx context.Context, store BuiltinPromptStore, templates []Template) error {
	for _, template := range templates {
		if err := ValidateTemplate(template); err != nil {
			return err
		}

		existing, ok, err := store.GetPromptTemplateByKey(ctx, template.Key)
		if err != nil {
			return err
		}
		if ok && existing.Source != TemplateSourceBuiltin && !existing.BuiltinLocked {
			continue
		}
		if ok {
			if existing.Checksum == template.Checksum && existing.Version == template.Version {
				continue
			}
			template.ID = existing.ID
			template.CreatedAt = existing.CreatedAt
		}
		if template.CreatedAt.IsZero() {
			template.CreatedAt = time.Now().UTC()
		}
		template.UpdatedAt = time.Now().UTC()
		if err := store.SavePromptTemplate(ctx, template); err != nil {
			return err
		}
	}
	return nil
}

// parseBuiltinPromptMarkdown 将单个 Markdown 文件解析为内置模板领域模型。
func parseBuiltinPromptMarkdown(data string) (Template, error) {
	metadata, content, err := splitFrontMatter(data)
	if err != nil {
		return Template{}, err
	}
	template := Template{
		Key:           metadata["key"],
		Name:          metadata["name"],
		Type:          TemplateType(metadata["type"]),
		Description:   metadata["description"],
		Content:       strings.TrimSpace(content),
		IsBuiltin:     parseBoolDefault(metadata["is_builtin"], true),
		BuiltinLocked: parseBoolDefault(metadata["builtin_locked"], true),
		Version:       parseIntDefault(metadata["version"], 1),
		Source:        metadata["source"],
	}
	if template.Source == "" {
		template.Source = TemplateSourceBuiltin
	}
	template.Variables = ExtractVariables(template.Content)
	template.Checksum = "sha256:" + sha256Hex(template.Content)
	if err := ValidateTemplate(template); err != nil {
		return Template{}, err
	}
	return template, nil
}

// splitFrontMatter 拆分简化 front matter 和正文，内置模板不引入额外 YAML 依赖。
func splitFrontMatter(data string) (map[string]string, string, error) {
	normalized := strings.ReplaceAll(data, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return nil, "", fmt.Errorf("missing front matter")
	}
	rest := strings.TrimPrefix(normalized, "---\n")
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return nil, "", fmt.Errorf("unterminated front matter")
	}
	return parseFrontMatter(rest[:end]), rest[end+5:], nil
}

// parseFrontMatter 解析内置模板使用的 key-value 元数据，列表项由正文变量提取兜底。
func parseFrontMatter(data string) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "  - ") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		result[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return result
}

// parseBoolDefault 解析布尔 front matter，缺省时使用内置模板的安全默认值。
func parseBoolDefault(value string, fallback bool) bool {
	if value == "" {
		return fallback
	}
	return strings.EqualFold(value, "true")
}

// parseIntDefault 解析版本号，非法值退回到调用方给定默认版本。
func parseIntDefault(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

// sha256Hex 计算模板正文 checksum，用于判断内置模板是否需要升级覆盖。
func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
