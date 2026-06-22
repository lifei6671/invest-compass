package search

import "testing"

// TestBuildPinyinTextGeneratesFullAndInitials 验证股票名称能生成稳定全拼和首字母。
func TestBuildPinyinTextGeneratesFullAndInitials(t *testing.T) {
	cases := []struct {
		name         string
		wantFull     string
		wantInitials string
	}{
		{name: "贵州茅台", wantFull: "guizhoumaotai", wantInitials: "gzmt"},
		{name: "重庆啤酒", wantFull: "chongqingpijiu", wantInitials: "cqpj"},
		{name: "长城汽车", wantFull: "changchengqiche", wantInitials: "ccqc"},
		{name: "兴业银行", wantFull: "xingyeyinhang", wantInitials: "xyyh"},
	}

	for _, item := range cases {
		got := BuildPinyinText(item.name, nil)
		if got.Full != item.wantFull || got.Initials != item.wantInitials {
			t.Fatalf("%s pinyin mismatch: got %+v", item.name, got)
		}
	}
}

// TestBuildPinyinTextUsesOverride 验证多音字 override 优先于默认拼音库结果。
func TestBuildPinyinTextUsesOverride(t *testing.T) {
	overrides := map[string]PinyinOverride{
		"重庆啤酒": {Full: "chongqingpijiu", Initials: "cqpj"},
	}

	got := BuildPinyinText("重庆啤酒", overrides)
	if got.Full != "chongqingpijiu" || got.Initials != "cqpj" {
		t.Fatalf("expected override pinyin, got %+v", got)
	}
}
