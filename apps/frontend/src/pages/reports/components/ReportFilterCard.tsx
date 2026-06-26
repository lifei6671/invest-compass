import { Button, DatePicker, Input, Select } from "antd";
import { CalendarOutlined, DownOutlined, SearchOutlined } from "@ant-design/icons";
import { appDatePickerLocale } from "../../../lib/antdLocale";
import type { ReportFilters } from "../types";
import type { FormEvent } from "react";

type ReportFilterCardProps = {
  filters: ReportFilters;
  modelOptions: string[];
  onChange: (patch: Partial<ReportFilters>) => void;
  onReset: () => void;
  onQuery: () => void;
};

const analysisTypeOptions = ["全部类型", "个股综合分析", "技术面分析", "财务分析", "持仓分析"];
const statusOptions = ["全部状态", "成功", "失败", "生成中"];
const { RangePicker } = DatePicker;

export function ReportFilterCard({ filters, modelOptions, onChange, onReset, onQuery }: ReportFilterCardProps) {
  const updateDateRange = (start: string, end: string) => {
    onChange({
      dateRangeStart: start,
      dateRangeEnd: end,
      dateRangeLabel: `${start || "---- -- --"} ~ ${end || "---- -- --"}`,
    });
  };

  const handleDateInput = (event: FormEvent<HTMLDivElement>) => {
    const inputs = Array.from(event.currentTarget.querySelectorAll("input"));
    const nextStart = inputs[0]?.value || filters.dateRangeStart;
    const nextEnd = inputs[1]?.value || filters.dateRangeEnd;
    updateDateRange(nextStart, nextEnd);
  };

  return (
    <section className="report-filter-card">
      <div className="report-filter-grid">
        <label className="report-filter-field">
          <span>股票搜索</span>
          <Input
            value={filters.keyword}
            onChange={(event) => onChange({ keyword: event.target.value })}
            placeholder="输入股票名称 / 代码 / 拼音"
            suffix={<SearchOutlined />}
          />
        </label>
        <label className="report-filter-field">
          <span>分析类型</span>
          <Select
            value={filters.analysisType}
            onChange={(value) => onChange({ analysisType: value as ReportFilters["analysisType"] })}
            options={analysisTypeOptions.map((value) => ({ value, label: value }))}
          />
        </label>
        <label className="report-filter-field">
          <span>使用模型</span>
          <Select value={filters.model} onChange={(value) => onChange({ model: value })} options={modelOptions.map((value) => ({ value, label: value }))} />
        </label>
        <label className="report-filter-field report-filter-date-field">
          <span>时间范围</span>
          <div onInput={handleDateInput}>
            <RangePicker
              key={`${filters.dateRangeStart}-${filters.dateRangeEnd}`}
              className="report-range-picker"
              format="YYYY-MM-DD"
              locale={appDatePickerLocale}
              allowClear={false}
              inputReadOnly={false}
              placement="bottomLeft"
              placeholder={[filters.dateRangeStart || "开始日期", filters.dateRangeEnd || "结束日期"]}
              prefix={<CalendarOutlined />}
              suffixIcon={<DownOutlined />}
              separator={<span className="report-range-separator">~</span>}
              onChange={(_, dateStrings) => updateDateRange(dateStrings[0] ?? "", dateStrings[1] ?? "")}
            />
          </div>
        </label>
        <label className="report-filter-field">
          <span>报告状态</span>
          <Select value={filters.status} onChange={(value) => onChange({ status: value as ReportFilters["status"] })} options={statusOptions.map((value) => ({ value, label: value }))} />
        </label>
      </div>

      <div className="report-filter-actions">
        <div className="report-filter-action-group">
          <Button aria-label="重置筛选" onClick={onReset}>
            重置
          </Button>
          <Button type="primary" aria-label="查询报告" onClick={onQuery}>
            查询
          </Button>
        </div>
      </div>
    </section>
  );
}
