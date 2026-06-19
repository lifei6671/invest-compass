package stockdata

import (
	"testing"
)

// TestMapFieldRowsSelectsWantedFields 验证 fields/items 结构能稳定映射为字段 map。
func TestMapFieldRowsSelectsWantedFields(t *testing.T) {
	rows, err := MapFieldRows(
		[]string{"ts_code", "symbol", "name", "industry"},
		[][]any{{"600519.SH", "600519", "贵州茅台", "白酒"}},
		[]string{"ts_code", "name"},
	)
	if err != nil {
		t.Fatalf("MapFieldRows returned error: %v", err)
	}
	if len(rows) != 1 || rows[0]["ts_code"] != "600519.SH" || rows[0]["name"] != "贵州茅台" {
		t.Fatalf("unexpected rows: %+v", rows)
	}
	if _, exists := rows[0]["industry"]; exists {
		t.Fatalf("unexpected unselected field: %+v", rows[0])
	}
}

// TestDecodeJSONPayloadSupportsCallbackAndVariable 验证常见 JSONP/callback 包装无需 JS 引擎也能解析。
func TestDecodeJSONPayloadSupportsCallbackAndVariable(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		options JSONPayloadOptions
	}{
		{name: "callback", payload: `data({"code":0,"name":"贵州茅台"});`, options: JSONPayloadOptions{CallbackName: "data"}},
		{name: "variable", payload: `var list_data = {"code":0,"name":"腾讯控股"};`, options: JSONPayloadOptions{VariableName: "list_data"}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			var decoded map[string]any
			if err := DecodeJSONPayload([]byte(tt.payload), tt.options, &decoded); err != nil {
				t.Fatalf("DecodeJSONPayload returned error: %v", err)
			}
			if decoded["code"].(float64) != 0 || decoded["name"] == "" {
				t.Fatalf("unexpected decoded payload: %+v", decoded)
			}
		})
	}
}

// TestDecodeJSONPayloadRejectsMismatchedCallback 验证 callback 名称不匹配时快速失败。
func TestDecodeJSONPayloadRejectsMismatchedCallback(t *testing.T) {
	var decoded map[string]any
	err := DecodeJSONPayload([]byte(`other({"code":0});`), JSONPayloadOptions{CallbackName: "data"}, &decoded)
	if err == nil {
		t.Fatal("expected callback mismatch error")
	}
}
