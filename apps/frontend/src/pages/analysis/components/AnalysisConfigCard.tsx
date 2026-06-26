import { CloseOutlined, DownOutlined } from "@ant-design/icons";
import { AutoComplete, Select } from "antd";
import { useEffect, useMemo, useState } from "react";
import { analysisTypes } from "../defaults";
import type { AIModel, AnalysisConfig, AnalysisModelOption, AnalysisPromptOption, AnalysisType, SelectedStock } from "../types";

type AnalysisConfigCardProps = {
  value: AnalysisConfig;
  modelOptions: AnalysisModelOption[];
  promptOptions: AnalysisPromptOption[];
  stockOptions: SelectedStock[];
  loadingModels?: boolean;
  loadingPrompts?: boolean;
  searchingStocks?: boolean;
  onChange: (value: AnalysisConfig) => void;
  onSearchStock: (keyword: string) => void;
  onManageTemplate: () => void;
};

export function AnalysisConfigCard(props: AnalysisConfigCardProps) {
  const [stockInput, setStockInput] = useState(displayStock(props.value.stock));
  const filteredStocks = useMemo(() => {
    const keyword = stockInput.trim().toLowerCase();
    if (!keyword) {
      return props.stockOptions;
    }
    return props.stockOptions.filter((item) => {
      const haystack = [item.name, item.symbol, item.code].join(" ").toLowerCase();
      return haystack.includes(keyword);
    });
  }, [props.stockOptions, stockInput]);

  useEffect(() => {
    setStockInput(displayStock(props.value.stock));
  }, [props.value.stock]);

  const selectStock = (value: string) => {
    const stock = props.stockOptions.find((item) => displayStock(item) === value);
    if (!stock) {
      return;
    }
    setStockInput(displayStock(stock));
    props.onChange({
      ...props.value,
      stock: {
        name: stock.name,
        symbol: stock.symbol,
        code: stock.code,
      },
    });
  };

  return (
    <section className="analysis-card analysis-config-card">
      <h2 className="analysis-card-title">分析配置</h2>
      <div className="analysis-form-stack">
        <label className="analysis-field">
          <span>股票选择</span>
          <AutoComplete
            allowClear={{ clearIcon: <CloseOutlined /> }}
            className="analysis-control analysis-stock-autocomplete"
            filterOption={false}
            placeholder="输入股票名称 / 代码 / 拼音"
            suffixIcon={<DownOutlined />}
            value={stockInput}
            options={filteredStocks.map((item) => ({
              value: displayStock(item),
              label: (
                <span className="analysis-stock-option">
                  <span>
                    <strong>{item.name}</strong>
                    <small>{item.code}</small>
                  </span>
                  <em>{item.symbol}</em>
                </span>
              ),
            }))}
            onChange={setStockInput}
            onSearch={props.onSearchStock}
            onSelect={selectStock}
            notFoundContent={props.searchingStocks ? "搜索中..." : "暂无匹配股票"}
          />
        </label>
        <label className="analysis-field">
          <span>分析类型</span>
          <Select
            className="analysis-control"
            value={props.value.analysisType}
            options={analysisTypes.map((item) => ({ value: item, label: item }))}
            onChange={(analysisType: AnalysisType) => props.onChange({ ...props.value, analysisType })}
          />
        </label>
        <label className="analysis-field">
          <span>AI 模型</span>
          <Select
            className="analysis-control"
            value={props.value.aiModel}
            loading={props.loadingModels}
            placeholder="请选择已配置模型"
            options={props.modelOptions.map((item) => ({ value: String(item.id), label: item.label, disabled: item.disabled }))}
            onChange={(aiModel: AIModel) => props.onChange({ ...props.value, aiModel })}
          />
        </label>
        <label className="analysis-field">
          <span>Prompt 模板</span>
          <Select
            className="analysis-control"
            value={props.value.promptTemplate}
            loading={props.loadingPrompts}
            placeholder="请选择 Prompt 模板"
            options={props.promptOptions.map((item) => ({ value: String(item.id), label: item.label }))}
            onChange={(promptTemplate: string) => props.onChange({ ...props.value, promptTemplate })}
          />
        </label>
      </div>
      <button className="analysis-link-button analysis-template-link" type="button" onClick={props.onManageTemplate}>
        管理模板
      </button>
    </section>
  );
}

function displayStock(stock: SelectedStock) {
  if (!stock.name && !stock.symbol) {
    return "";
  }
  return `${stock.name}    ${stock.symbol}`;
}
