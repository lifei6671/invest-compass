/* @vitest-environment jsdom */

import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import type { FormEvent } from "react";
import { expect, test, vi } from "vitest";
import type { SchedulerJob } from "../../services/scheduler";
import { SchedulerBackfillDialog, type SchedulerBackfillFormState } from "./SchedulerBackfillDialog";

test("SchedulerBackfillDialog 渲染受控补偿表单并派发提交", () => {
  const value: SchedulerBackfillFormState = {
    jobId: "7",
    dateFrom: "2026-06-19",
    dateTo: "2026-06-19",
    symbols: "600000.SH",
  };
  const jobs: SchedulerJob[] = [
    { id: 7, name: "A 股行情刷新", cron_type: "cn_a_share_quote_refresh" },
    { id: 8, name: "市场新闻刷新", cron_type: "market_news_refresh" },
  ];
  const onChange = vi.fn();
  const onSubmit = vi.fn((event: FormEvent<HTMLFormElement>) => event.preventDefault());

  render(
    <SchedulerBackfillDialog
      value={value}
      jobs={jobs}
      actionPending=""
      onChange={onChange}
      onSubmit={onSubmit}
    />,
  );

  expect(screen.getByLabelText("任务")).toHaveValue("7");
  expect(screen.getByLabelText("开始日期")).toHaveValue("2026-06-19");
  expect(screen.getByLabelText("结束日期")).toHaveValue("2026-06-19");
  expect(screen.getByLabelText("Symbol，可选")).toHaveValue("600000.SH");

  fireEvent.change(screen.getByLabelText("任务"), { target: { value: "8" } });
  fireEvent.change(screen.getByLabelText("Symbol，可选"), { target: { value: "000001.SZ" } });
  fireEvent.click(screen.getByRole("button", { name: "生成补偿" }));

  expect(onChange).toHaveBeenCalledWith({ ...value, jobId: "8" });
  expect(onChange).toHaveBeenCalledWith({ ...value, symbols: "000001.SZ" });
  expect(onSubmit).toHaveBeenCalledTimes(1);
});
