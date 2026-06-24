package prompt

import (
	"context"
	"io/fs"
	"testing"
	"testing/fstest"
)

// TestLoadBuiltinPromptTemplatesParsesFrontMatter 验证内置 Prompt Markdown front matter 会被解析为只读模板模型。
func TestLoadBuiltinPromptTemplatesParsesFrontMatter(t *testing.T) {
	templates, err := LoadBuiltinPromptTemplates(fstest.MapFS{
		"builtin/stock_full.md": &fstest.MapFile{Data: []byte(`---
key: builtin_stock_full
name: 个股综合分析
type: stock_full
version: 2
is_builtin: true
builtin_locked: true
variables:
  - stock_name
  - prompt_key
---

请分析 {{stock_name}}，模板 {{prompt_key}}。
`)},
	})
	if err != nil {
		t.Fatalf("LoadBuiltinPromptTemplates returned error: %v", err)
	}
	if len(templates) != 1 {
		t.Fatalf("expected one template, got %+v", templates)
	}
	template := templates[0]
	if template.Key != "builtin_stock_full" ||
		template.Name != "个股综合分析" ||
		template.Type != TemplateStockFull ||
		template.Version != 2 ||
		!template.IsBuiltin ||
		!template.BuiltinLocked ||
		template.Source != TemplateSourceBuiltin {
		t.Fatalf("unexpected builtin template: %+v", template)
	}
	if template.Checksum == "" {
		t.Fatalf("expected checksum to be calculated: %+v", template)
	}
	if len(template.Variables) != 2 || template.Variables[0] != VariableStockName || template.Variables[1] != VariablePromptKey {
		t.Fatalf("unexpected variables: %+v", template.Variables)
	}
}

// TestLoadPackagedBuiltinPromptTemplatesIncludesRequiredKeys 验证仓库内打包的内置模板包含本期要求的 5 个模板。
func TestLoadPackagedBuiltinPromptTemplatesIncludesRequiredKeys(t *testing.T) {
	templates, err := LoadPackagedBuiltinPromptTemplates()
	if err != nil {
		t.Fatalf("LoadPackagedBuiltinPromptTemplates returned error: %v", err)
	}
	keys := make(map[string]struct{}, len(templates))
	for _, template := range templates {
		keys[template.Key] = struct{}{}
		if !template.IsBuiltin || !template.BuiltinLocked || template.Source != TemplateSourceBuiltin {
			t.Fatalf("packaged builtin template must be locked builtin: %+v", template)
		}
	}
	for _, key := range []string{
		"builtin_system_common",
		"builtin_stock_full",
		"builtin_technical",
		"builtin_fundamental",
		"builtin_news",
	} {
		if _, ok := keys[key]; !ok {
			t.Fatalf("missing packaged builtin prompt key %q, got %+v", key, keys)
		}
	}
}

// TestSeedBuiltinPromptTemplatesCreatesAndUpdatesByChecksum 验证 seed 按 key 创建内置模板，并在 checksum 变化时更新系统模板。
func TestSeedBuiltinPromptTemplatesCreatesAndUpdatesByChecksum(t *testing.T) {
	store := &fakeBuiltinPromptStore{
		items: map[string]Template{
			"builtin_technical": {
				ID:            9,
				Key:           "builtin_technical",
				Name:          "旧技术模板",
				Type:          TemplateTechnical,
				Content:       "旧内容 {{stock_name}}",
				Version:       1,
				Checksum:      "old-checksum",
				IsBuiltin:     true,
				BuiltinLocked: true,
				Source:        TemplateSourceBuiltin,
			},
			"user_custom": {
				ID:       10,
				Key:      "user_custom",
				Name:     "用户模板",
				Type:     TemplateCustom,
				Content:  "用户内容 {{stock_name}}",
				Source:   TemplateSourceUser,
				Checksum: "user-checksum",
			},
		},
	}
	templates, err := LoadBuiltinPromptTemplates(fstest.MapFS{
		"builtin/technical.md": &fstest.MapFile{Data: []byte(`---
key: builtin_technical
name: 技术面分析
type: technical
version: 2
is_builtin: true
builtin_locked: true
variables:
  - stock_name
---

新技术内容 {{stock_name}}
`)},
		"builtin/news.md": &fstest.MapFile{Data: []byte(`---
key: builtin_news
name: 消息面分析
type: news
version: 1
is_builtin: true
builtin_locked: true
variables:
  - stock_name
  - news
---

消息面内容 {{stock_name}} {{news}}
`)},
	})
	if err != nil {
		t.Fatalf("load builtin templates: %v", err)
	}

	if err := SeedBuiltinPromptTemplates(context.Background(), store, templates); err != nil {
		t.Fatalf("SeedBuiltinPromptTemplates returned error: %v", err)
	}

	technical := store.items["builtin_technical"]
	if technical.ID != 9 || technical.Name != "技术面分析" || technical.Version != 2 || technical.Checksum == "old-checksum" {
		t.Fatalf("expected existing builtin technical template to be updated, got %+v", technical)
	}
	if _, ok := store.items["builtin_news"]; !ok {
		t.Fatalf("expected builtin_news to be created, got %+v", store.items)
	}
	if store.items["user_custom"].Content != "用户内容 {{stock_name}}" {
		t.Fatalf("user template must not be overwritten: %+v", store.items["user_custom"])
	}
}

type fakeBuiltinPromptStore struct {
	items map[string]Template
}

// GetPromptTemplateByKey 按 key 返回 fake store 中的模板。
func (store *fakeBuiltinPromptStore) GetPromptTemplateByKey(_ context.Context, key string) (Template, bool, error) {
	template, ok := store.items[key]
	return template, ok, nil
}

// SavePromptTemplate 保存 fake store 模板，模拟 DAO 按 ID 保持已有记录。
func (store *fakeBuiltinPromptStore) SavePromptTemplate(_ context.Context, template Template) error {
	if template.ID == 0 {
		template.ID = int64(len(store.items) + 1)
	}
	store.items[template.Key] = template
	return nil
}

var _ fs.FS = fstest.MapFS{}
