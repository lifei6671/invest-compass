/* @vitest-environment jsdom */

import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import type { SchedulerJob } from "../../services/scheduler";
import type { ProviderStatusItem } from "../../services/coreClient";
import { SchedulerJobTable } from "./SchedulerJobTable";

afterEach(() => {
  vi.useRealTimers();
});

test("SchedulerJobTable 展示下次运行时间并按 Provider 状态限制立即执行", () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.setSystemTime(new Date("2026-06-19T09:00:00+08:00"));
  const job: SchedulerJob = {
    id: 7,
    name: "A 股开盘行情刷新",
    cron_type: "cn_a_share_quote_refresh",
    cron_expr: "30 9 * * 1-5",
    enabled: true,
    last_status: "failed",
    last_error: "market_provider_unconfigured",
  };
  const providers: ProviderStatusItem[] = [
    { name: "market-provider", source: "sina-tencent-market", available: false, last_error: "provider_missing_key" },
  ];
  const onRunNow = vi.fn();
  const onEdit = vi.fn();
  const onSetEnabled = vi.fn();
  const onDelete = vi.fn();

  render(
    <SchedulerJobTable
      jobs={[job]}
      providers={providers}
      actionPending=""
      onRunNow={onRunNow}
      onEdit={onEdit}
      onSetEnabled={onSetEnabled}
      onDelete={onDelete}
    />,
  );

  expect(screen.getByText("A 股开盘行情刷新")).toBeInTheDocument();
  expect(screen.getByText("2026-06-19 09:30")).toBeInTheDocument();
  expect(screen.getByText("provider_missing_key")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "立即执行" })).toBeDisabled();

  fireEvent.click(screen.getByRole("button", { name: "编辑" }));
  fireEvent.click(screen.getByRole("button", { name: "停用任务" }));
  fireEvent.click(screen.getByRole("button", { name: "删除任务" }));

  expect(onRunNow).not.toHaveBeenCalled();
  expect(onEdit).toHaveBeenCalledWith(job);
  expect(onSetEnabled).toHaveBeenCalledWith(job, false);
  expect(onDelete).toHaveBeenCalledWith(job);
});
