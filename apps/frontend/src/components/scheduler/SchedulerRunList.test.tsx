/* @vitest-environment jsdom */

import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { expect, test, vi } from "vitest";
import type { SchedulerJob, SchedulerRun } from "../../services/scheduler";
import { SchedulerRunList, type SchedulerRunFilters } from "./SchedulerRunList";

test("SchedulerRunList 展示运行记录、过滤控件和执行详情", () => {
  const jobs: SchedulerJob[] = [
    { id: 7, name: "A 股行情刷新", cron_type: "cn_a_share_quote_refresh" },
  ];
  const runs: SchedulerRun[] = [
    {
      id: 11,
      run_key: "scheduler_job:7:2026-06-19:missed_today",
      trigger_type: "missed_today",
      status: "success",
      target_date: "2026-06-19",
      started_at: "2026-06-19T10:00:00Z",
      finished_at: "2026-06-19T10:00:02Z",
      fetched_count: 2,
      written_count: 2,
      source: "startup_restore",
      scope_key: "CN",
      cron_type: "cn_a_share_quote_refresh",
      data_type: "quote",
    },
    {
      id: 12,
      run_key: "scheduler_job:7:2026-06-18:catchup_gap",
      trigger_type: "catchup_gap",
      status: "failed",
      target_date: "2026-06-18",
      error_message: "provider unavailable",
    },
  ];
  const filters: SchedulerRunFilters = {
    jobId: "0",
    status: "",
    triggerType: "",
  };
  const onChangeJob = vi.fn();
  const onChangeStatus = vi.fn();
  const onChangeTriggerType = vi.fn();
  const onViewDetail = vi.fn();

  render(
    <SchedulerRunList
      jobs={jobs}
      runs={runs}
      filters={filters}
      selectedRun={runs[0]}
      actionPending=""
      onChangeJob={onChangeJob}
      onChangeStatus={onChangeStatus}
      onChangeTriggerType={onChangeTriggerType}
      onViewDetail={onViewDetail}
    />,
  );

  expect(screen.getAllByText("scheduler_job:7:2026-06-19:missed_today").length).toBeGreaterThan(0);
  expect(screen.getByText("provider unavailable")).toBeInTheDocument();
  expect(screen.getByText("startup_restore")).toBeInTheDocument();
  expect(screen.getAllByText("2/2").length).toBeGreaterThan(0);

  fireEvent.change(screen.getByLabelText("执行任务"), { target: { value: "7" } });
  fireEvent.change(screen.getByLabelText("执行状态"), { target: { value: "failed" } });
  fireEvent.change(screen.getByLabelText("触发类型"), { target: { value: "catchup_gap" } });
  fireEvent.click(screen.getAllByRole("button", { name: "查看详情" })[0]);

  expect(onChangeJob).toHaveBeenCalledWith("7");
  expect(onChangeStatus).toHaveBeenCalledWith("failed");
  expect(onChangeTriggerType).toHaveBeenCalledWith("catchup_gap");
  expect(onViewDetail).toHaveBeenCalledWith(runs[0]);
});

test("SchedulerRunList 在过滤后无记录时展示空状态", () => {
  render(
    <SchedulerRunList
      jobs={[]}
      runs={[{ id: 12, run_key: "run-failed", trigger_type: "catchup_gap", status: "failed" }]}
      filters={{ jobId: "0", status: "success", triggerType: "" }}
      selectedRun={null}
      actionPending=""
      onChangeJob={vi.fn()}
      onChangeStatus={vi.fn()}
      onChangeTriggerType={vi.fn()}
      onViewDetail={vi.fn()}
    />,
  );

  expect(screen.getByText("暂无执行记录")).toBeInTheDocument();
});
