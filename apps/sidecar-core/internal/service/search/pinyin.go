package search

import (
	"strings"

	pinyinlib "github.com/mozillazg/go-pinyin"
)

// PinyinText 是搜索索引使用的全拼和首字母结果。
type PinyinText struct {
	Full     string
	Initials string
}

// PinyinOverride 表示人工确认过的多音字拼音覆盖。
type PinyinOverride struct {
	Full     string
	Initials string
}

var defaultPinyinOverrides = map[string]PinyinOverride{
	"重庆啤酒": {Full: "chongqingpijiu", Initials: "cqpj"},
	"长城汽车": {Full: "changchengqiche", Initials: "ccqc"},
	"兴业银行": {Full: "xingyeyinhang", Initials: "xyyh"},
}

// BuildPinyinText 为股票名称、全称或别名生成稳定拼音字段。
func BuildPinyinText(text string, overrides map[string]PinyinOverride) PinyinText {
	if override, ok := lookupPinyinOverride(text, overrides); ok {
		return PinyinText{
			Full:     sanitizePinyin(override.Full),
			Initials: sanitizePinyin(override.Initials),
		}
	}
	syllables := buildPinyinSyllables(text)
	return PinyinText{
		Full:     strings.Join(syllables, ""),
		Initials: buildInitialsFromSyllables(syllables),
	}
}

// lookupPinyinOverride 优先使用调用方 override，其次使用首批内置常见多音字股票名。
func lookupPinyinOverride(text string, overrides map[string]PinyinOverride) (PinyinOverride, bool) {
	if override, ok := overrides[text]; ok {
		return override, true
	}
	override, ok := defaultPinyinOverrides[text]
	return override, ok
}

// buildPinyinSyllables 按 go-pinyin 默认全拼风格提取每个汉字的首个候选音。
func buildPinyinSyllables(text string) []string {
	args := pinyinlib.NewArgs()
	args.Style = pinyinlib.Normal
	args.Heteronym = false

	var syllables []string
	for _, candidates := range pinyinlib.Pinyin(text, args) {
		if len(candidates) == 0 {
			continue
		}
		syllable := sanitizePinyin(candidates[0])
		if syllable != "" {
			syllables = append(syllables, syllable)
		}
	}
	return syllables
}

// buildInitialsFromSyllables 从全拼音节取首字母，符合股票简称拼音首字母搜索习惯。
func buildInitialsFromSyllables(syllables []string) string {
	var builder strings.Builder
	for _, syllable := range syllables {
		for _, r := range syllable {
			builder.WriteRune(r)
			break
		}
	}
	return builder.String()
}

// sanitizePinyin 只保留小写字母和数字，避免拼音字段混入空格和符号。
func sanitizePinyin(text string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(text) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}
