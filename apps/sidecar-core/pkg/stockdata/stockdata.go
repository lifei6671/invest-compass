package stockdata

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Record 是数据源解析后的松散字段记录，由上层 Provider 再转换到业务模型。
type Record map[string]any

// JSONPayloadOptions 描述 JSONP 或 JavaScript 变量包裹体的解包方式。
type JSONPayloadOptions struct {
	CallbackName string
	VariableName string
}

// MapFieldRows 将字段名数组和二维行数据映射为 map 列表，适配 Tushare fields/items 这类响应。
func MapFieldRows(fields []string, items [][]any, selectedFields []string) ([]Record, error) {
	fieldIndexes := make(map[string]int, len(fields))
	for index, field := range fields {
		name := strings.TrimSpace(field)
		if name == "" {
			return nil, fmt.Errorf("stockdata field name is empty")
		}
		fieldIndexes[name] = index
	}

	wantedFields := selectedFields
	if len(wantedFields) == 0 {
		wantedFields = fields
	}
	indexes := make([]struct {
		name  string
		index int
	}, 0, len(wantedFields))
	for _, field := range wantedFields {
		name := strings.TrimSpace(field)
		index, ok := fieldIndexes[name]
		if !ok {
			return nil, fmt.Errorf("stockdata field %q not found", name)
		}
		indexes = append(indexes, struct {
			name  string
			index int
		}{name: name, index: index})
	}

	rows := make([]Record, 0, len(items))
	for rowIndex, item := range items {
		row := make(Record, len(indexes))
		for _, field := range indexes {
			if field.index >= len(item) {
				return nil, fmt.Errorf("stockdata row %d missing field %q", rowIndex, field.name)
			}
			row[field.name] = item[field.index]
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// DecodeJSONPayload 解码普通 JSON、callback(json) 或 var name = json; 形式的响应体。
func DecodeJSONPayload(payload []byte, options JSONPayloadOptions, target any) error {
	if target == nil {
		return errors.New("stockdata missing json payload target")
	}
	trimmed := strings.TrimSpace(string(payload))
	if trimmed == "" {
		return errors.New("stockdata empty json payload")
	}

	jsonText, err := unwrapJSONPayload(trimmed, options)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(jsonText), target); err != nil {
		return fmt.Errorf("stockdata decode json payload failed: %w", err)
	}
	return nil
}

// unwrapJSONPayload 从常见 JS 包裹体中提取 JSON 文本，避免为了简单 callback 引入 JS 引擎。
func unwrapJSONPayload(payload string, options JSONPayloadOptions) (string, error) {
	if options.CallbackName != "" {
		prefix := options.CallbackName + "("
		if !strings.HasPrefix(payload, prefix) {
			return "", fmt.Errorf("stockdata callback %q not found", options.CallbackName)
		}
		inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(payload, prefix), ";"))
		if !strings.HasSuffix(inner, ")") {
			return "", fmt.Errorf("stockdata callback %q is not closed", options.CallbackName)
		}
		return strings.TrimSpace(strings.TrimSuffix(inner, ")")), nil
	}

	if options.VariableName != "" {
		prefix := "var " + options.VariableName
		if !strings.HasPrefix(payload, prefix) {
			return "", fmt.Errorf("stockdata variable %q not found", options.VariableName)
		}
		assignment := strings.TrimSpace(strings.TrimPrefix(payload, prefix))
		if !strings.HasPrefix(assignment, "=") {
			return "", fmt.Errorf("stockdata variable %q assignment not found", options.VariableName)
		}
		return strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(assignment, "="), ";")), nil
	}

	return payload, nil
}
