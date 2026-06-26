import { CalendarOutlined, DownOutlined, SearchOutlined } from "@ant-design/icons";
import { Button, DatePicker, Input, Select } from "antd";
import type { FormEvent } from "react";
import { appDatePickerLocale } from "../../../lib/antdLocale";
import { taskStatusLabels, type TaskFilters, type TaskStatus, type TaskType } from "../types";

type TaskFilterCardProps = {
  filters: TaskFilters;
  onChange: (patch: Partial<TaskFilters>) => void;
  onReset: () => void;
  onQuery: () => void;
};

const taskTypeOptions: Array<TaskFilters["taskType"]> = ["全部类型", "AI 分析", "资讯同步", "行情刷新", "缓存清理", "数据重建"];
const taskStatusOptions: Array<TaskFilters["status"]> = ["全部状态", "RUNNING", "SUCCESS", "FAILED", "CANCELLED"];
const { RangePicker } = DatePicker;

export function TaskFilterCard({ filters, onChange, onReset, onQuery }: TaskFilterCardProps) {
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
    <section className="task-filter-card">
      <div className="task-filter-grid">
        <label className="task-filter-field">
          <span>任务类型</span>
          <Select<TaskFilters["taskType"]>
            value={filters.taskType}
            options={taskTypeOptions.map((value) => ({ label: value, value }))}
            onChange={(taskType: "全部类型" | TaskType) => onChange({ taskType })}
          />
        </label>
        <label className="task-filter-field">
          <span>任务状态</span>
          <Select<TaskFilters["status"]>
            value={filters.status}
            options={taskStatusOptions.map((value) => ({
              label: value === "全部状态" ? value : taskStatusLabels[value],
              value,
            }))}
            onChange={(status: "全部状态" | TaskStatus) => onChange({ status })}
          />
        </label>
        <label className="task-filter-field task-filter-date-field">
          <span>时间范围</span>
          <div onInput={handleDateInput}>
            <RangePicker
              key={`${filters.dateRangeStart}-${filters.dateRangeEnd}`}
              className="task-range-picker"
              format="YYYY-MM-DD"
              locale={appDatePickerLocale}
              allowClear={false}
              inputReadOnly={false}
              placement="bottomLeft"
              placeholder={[filters.dateRangeStart || "开始日期", filters.dateRangeEnd || "结束日期"]}
              prefix={<CalendarOutlined />}
              suffixIcon={<DownOutlined />}
              separator={<span className="task-range-separator">~</span>}
              onChange={(_, dateStrings) => updateDateRange(dateStrings[0] ?? "", dateStrings[1] ?? "")}
            />
          </div>
        </label>
        <label className="task-filter-field task-filter-keyword-field">
          <span>关键词搜索</span>
          <Input
            value={filters.keyword}
            placeholder="搜索任务标题 / 任务 ID / 股票代码"
            suffix={<SearchOutlined />}
            onChange={(event) => onChange({ keyword: event.target.value })}
          />
        </label>
      </div>
      <div className="task-filter-actions">
        <Button onClick={onReset}>重置</Button>
        <Button type="primary" onClick={onQuery}>
          查询
        </Button>
      </div>
    </section>
  );
}
