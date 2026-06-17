package prompt

import (
	"errors"
	"testing"
	"time"
)

// TestValidateTemplateAcceptsSupportedTypesAndVariables 验证首版模板类型和变量白名单。
func TestValidateTemplateAcceptsSupportedTypesAndVariables(t *testing.T) {
	template := Template{
		Name:    "个股综合分析",
		Type:    TemplateStockFull,
		Content: "分析 {{stock_name}} {{stock_code}} {{market}} {{quote}} {{kline_summary}} {{indicators}} {{news}}，语言：{{analysis_language}}",
	}

	if err := ValidateTemplate(template); err != nil {
		t.Fatalf("ValidateTemplate returned error: %v", err)
	}
}

// TestValidateTemplateRejectsUnsupportedType 验证未来模板类型不能进入首版模板页。
func TestValidateTemplateRejectsUnsupportedType(t *testing.T) {
	err := ValidateTemplate(Template{
		Name:    "持仓分析",
		Type:    TemplateType("portfolio"),
		Content: "分析 {{stock_name}}",
	})

	assertPromptErrorCode(t, err, ErrorUnsupportedTemplateType)
}

// TestValidateTemplateRejectsUnsupportedVariables 验证公告、研报、持仓等未支持变量保存失败。
func TestValidateTemplateRejectsUnsupportedVariables(t *testing.T) {
	err := ValidateTemplate(Template{
		Name:    "未来变量",
		Type:    TemplateCustom,
		Content: "分析 {{stock_name}} {{announcements}} {{reports}} {{portfolio}}",
	})

	assertPromptErrorCode(t, err, ErrorUnsupportedVariable)
}

// TestExtractVariablesKeepsStableOrder 验证变量提取按首次出现顺序返回且去重。
func TestExtractVariablesKeepsStableOrder(t *testing.T) {
	variables := ExtractVariables("{{stock_name}} {{stock_code}} {{stock_name}} {{ news }}")

	want := []Variable{VariableStockName, VariableStockCode, VariableNews}
	if len(variables) != len(want) {
		t.Fatalf("expected %d variables, got %d: %+v", len(want), len(variables), variables)
	}
	for index := range want {
		if variables[index] != want[index] {
			t.Fatalf("index %d expected %q, got %q", index, want[index], variables[index])
		}
	}
}

// TestCanDeleteTemplateOnlyAllowsCustomOrNonBuiltin 验证删除只对 custom 或非内置模板生效。
func TestCanDeleteTemplateOnlyAllowsCustomOrNonBuiltin(t *testing.T) {
	tests := []struct {
		name     string
		template Template
		want     bool
	}{
		{name: "builtin system cannot delete", template: Template{Type: TemplateSystem, IsBuiltin: true}, want: false},
		{name: "builtin custom can delete", template: Template{Type: TemplateCustom, IsBuiltin: true}, want: true},
		{name: "non builtin stock full can delete", template: Template{Type: TemplateStockFull, IsBuiltin: false}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanDeleteTemplate(tt.template); got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

// TestFilterActiveTemplatesHidesSoftDeleted 验证列表默认不返回已软删除模板。
func TestFilterActiveTemplatesHidesSoftDeleted(t *testing.T) {
	templates := []Template{
		{ID: 1, Name: "active", Type: TemplateCustom},
		{ID: 2, Name: "deleted", Type: TemplateCustom, Deleted: true},
	}

	active := FilterActiveTemplates(templates)

	if len(active) != 1 || active[0].ID != 1 {
		t.Fatalf("expected only active template, got %+v", active)
	}
}

// TestCreateTemplateValidatesAndExtractsVariables 验证创建模板时统一校验并固化变量列表。
func TestCreateTemplateValidatesAndExtractsVariables(t *testing.T) {
	now := time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)

	template, err := CreateTemplate(CreateRequest{
		ID:          9,
		Name:        "自定义综合分析",
		Type:        TemplateCustom,
		Description: "用于个股综合分析",
		Content:     "分析 {{stock_name}} 和 {{ stock_code }}，再看 {{news}}。",
	}, now)
	if err != nil {
		t.Fatalf("CreateTemplate returned error: %v", err)
	}

	if template.ID != 9 || template.Name != "自定义综合分析" || template.Type != TemplateCustom {
		t.Fatalf("unexpected created template: %+v", template)
	}
	if template.CreatedAt != now || template.UpdatedAt != now {
		t.Fatalf("expected timestamps to be %s, got created=%s updated=%s", now, template.CreatedAt, template.UpdatedAt)
	}
	if template.Deleted || !template.DeletedAt.IsZero() {
		t.Fatalf("new template should be active, got %+v", template)
	}

	wantVariables := []Variable{VariableStockName, VariableStockCode, VariableNews}
	if len(template.Variables) != len(wantVariables) {
		t.Fatalf("expected variables %+v, got %+v", wantVariables, template.Variables)
	}
	for index := range wantVariables {
		if template.Variables[index] != wantVariables[index] {
			t.Fatalf("index %d expected variable %q, got %q", index, wantVariables[index], template.Variables[index])
		}
	}
}

// TestUpdateTemplateRejectsBuiltin 验证内置模板只读，不能被更新。
func TestUpdateTemplateRejectsBuiltin(t *testing.T) {
	_, err := UpdateTemplate(Template{
		ID:        1,
		Name:      "内置系统模板",
		Type:      TemplateSystem,
		Content:   "分析 {{stock_name}}",
		IsBuiltin: true,
	}, UpdateRequest{
		Name:    "尝试修改",
		Type:    TemplateSystem,
		Content: "修改 {{stock_name}}",
	}, time.Now())

	assertPromptErrorCode(t, err, ErrorBuiltinTemplateReadOnly)
}

// TestUpdateTemplateValidatesVariables 验证更新模板时同样执行变量白名单校验。
func TestUpdateTemplateValidatesVariables(t *testing.T) {
	_, err := UpdateTemplate(Template{
		ID:      2,
		Name:    "自定义模板",
		Type:    TemplateCustom,
		Content: "分析 {{stock_name}}",
	}, UpdateRequest{
		Name:    "自定义模板",
		Type:    TemplateCustom,
		Content: "分析 {{stock_name}} {{portfolio}}",
	}, time.Now())

	assertPromptErrorCode(t, err, ErrorUnsupportedVariable)
}

// TestDeleteTemplateSoftDeletesAllowedTemplate 验证允许删除的模板只做软删除。
func TestDeleteTemplateSoftDeletesAllowedTemplate(t *testing.T) {
	createdAt := time.Date(2026, 6, 17, 9, 0, 0, 0, time.UTC)
	deletedAt := time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)

	template, err := DeleteTemplate(Template{
		ID:        3,
		Name:      "用户模板",
		Type:      TemplateStockFull,
		Content:   "分析 {{stock_name}}",
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}, deletedAt)
	if err != nil {
		t.Fatalf("DeleteTemplate returned error: %v", err)
	}

	if !template.Deleted || template.DeletedAt != deletedAt || template.UpdatedAt != deletedAt {
		t.Fatalf("expected soft deleted template, got %+v", template)
	}
	if template.CreatedAt != createdAt {
		t.Fatalf("expected created_at to stay %s, got %s", createdAt, template.CreatedAt)
	}
}

// TestDeleteTemplateRejectsReadonlyBuiltin 验证只读内置模板不能删除。
func TestDeleteTemplateRejectsReadonlyBuiltin(t *testing.T) {
	_, err := DeleteTemplate(Template{
		ID:        4,
		Name:      "内置技术模板",
		Type:      TemplateTechnical,
		Content:   "分析 {{stock_name}}",
		IsBuiltin: true,
	}, time.Now())

	assertPromptErrorCode(t, err, ErrorBuiltinTemplateReadOnly)
}

// TestListTemplatesFiltersDeletedAndSorts 验证列表不返回软删除模板，并按更新时间倒序稳定排序。
func TestListTemplatesFiltersDeletedAndSorts(t *testing.T) {
	base := time.Date(2026, 6, 17, 9, 0, 0, 0, time.UTC)
	templates := []Template{
		{ID: 1, Name: "old", Type: TemplateCustom, UpdatedAt: base},
		{ID: 2, Name: "deleted", Type: TemplateCustom, UpdatedAt: base.Add(3 * time.Hour), Deleted: true},
		{ID: 3, Name: "new", Type: TemplateCustom, UpdatedAt: base.Add(2 * time.Hour)},
		{ID: 4, Name: "same time lower id", Type: TemplateCustom, UpdatedAt: base.Add(2 * time.Hour)},
	}

	listed := ListTemplates(templates)

	wantIDs := []int64{3, 4, 1}
	if len(listed) != len(wantIDs) {
		t.Fatalf("expected %d templates, got %d: %+v", len(wantIDs), len(listed), listed)
	}
	for index, wantID := range wantIDs {
		if listed[index].ID != wantID {
			t.Fatalf("index %d expected ID %d, got %+v", index, wantID, listed[index])
		}
	}
}

// assertPromptErrorCode 校验 Prompt 错误码稳定。
func assertPromptErrorCode(t *testing.T, err error, code ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}

	var promptError *Error
	if !errors.As(err, &promptError) {
		t.Fatalf("expected prompt Error, got %T", err)
	}
	if promptError.Code != code {
		t.Fatalf("expected error code %q, got %q", code, promptError.Code)
	}
}
