package prompt

import (
	"errors"
	"testing"
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
