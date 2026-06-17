package logger

import (
	"regexp"
	"strings"
)

// RedactedValue 是所有敏感字段进入日志、错误和导出内容前的统一替换值。
const RedactedValue = "[REDACTED]"

var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)("?(authorization|proxy-authorization)"?\s*:\s*)"?[^"\r\n,}]+`),
	regexp.MustCompile(`(?i)("?(api[_-]?key|apikey|license[_-]?key|licensekey|proxy[_-]?password|proxypassword)"?\s*:\s*)"?[^"\r\n,}]+`),
	regexp.MustCompile(`(?i)(authorization|proxy-authorization)\s*:\s*[^\r\n]+`),
	regexp.MustCompile(`(?i)(api[_-]?key|apikey|license[_-]?key|licensekey|proxy[_-]?password|proxypassword)\s*[:=]\s*[^\r\n,;]+`),
	regexp.MustCompile(`(?i)(position[_-]?snapshot|position[_-]?input|holding[_-]?input|portfolio)\s*[:=]\s*[^\r\n]+`),
}

// RedactText 对日志、错误和导出文本做统一脱敏，避免密钥和用户持仓输入泄露。
func RedactText(value string) string {
	redacted := value
	for _, pattern := range sensitivePatterns {
		redacted = pattern.ReplaceAllStringFunc(redacted, redactMatchedField)
	}
	return redacted
}

// RedactError 将错误转成安全字符串，供 slog 字段和错误响应复用。
func RedactError(err error) string {
	if err == nil {
		return ""
	}
	return RedactText(err.Error())
}

// ExportLogText 拼接日志行并在导出前执行二次脱敏。
func ExportLogText(lines []string) string {
	return RedactText(strings.Join(lines, "\n"))
}

// redactMatchedField 保留字段名并替换字段值，便于排障时知道哪个字段被脱敏。
func redactMatchedField(value string) string {
	for index, char := range value {
		if char == ':' || char == '=' {
			return value[:index+1] + " " + RedactedValue
		}
	}
	return RedactedValue
}
