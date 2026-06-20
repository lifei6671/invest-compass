/* @vitest-environment jsdom */

import { clearMocks, mockIPC } from "@tauri-apps/api/mocks";
import { afterEach, describe, expect, test } from "vitest";
import {
  schedulerJobsBackfill,
  schedulerJobsList,
  schedulerJobsSave,
  schedulerRefreshSymbol,
  schedulerStatus,
} from "./scheduler";

describe("scheduler service", () => {
  afterEach(() => {
    clearMocks();
  });

  test("通过单一 typed service 读取调度状态和任务列表", async () => {
    const calls: Array<{ command: string; payload?: unknown }> = [];
    mockIPC((command, payload) => {
      calls.push({ command, payload });
      if (command === "scheduler_status") {
        return {
          code: 0,
          message: "ok",
          data: { jobs_total: 1, jobs_enabled: 1, queued_runs: 0, running_runs: 0, failed_runs: 0 },
        };
      }
      return {
        code: 0,
        message: "ok",
        data: { items: [{ id: 7, cron_type: "cn_a_share_quote_refresh" }] },
      };
    });

    await expect(schedulerStatus()).resolves.toMatchObject({ jobs_total: 1, jobs_enabled: 1 });
    await expect(schedulerJobsList()).resolves.toEqual({
      items: [{ id: 7, cron_type: "cn_a_share_quote_refresh" }],
    });

    expect(calls).toEqual([
      { command: "scheduler_status", payload: {} },
      { command: "scheduler_jobs_list", payload: {} },
    ]);
  });

  test("调度任务保存和手动补偿保持 payload 原样交给 Rust 白名单 command", async () => {
    const calls: Array<{ command: string; payload?: unknown }> = [];
    mockIPC((command, payload) => {
      calls.push({ command, payload });
      return {
        code: 0,
        message: "ok",
        data: command === "scheduler_jobs_backfill"
          ? { items: [{ run_key: "run-1", trigger_type: "catchup_gap", status: "queued" }] }
          : { id: 7, cron_type: "cn_a_share_quote_refresh" },
      };
    });

    await schedulerJobsSave({
      id: 7,
      name: "A 股开盘行情刷新",
      cron_type: "cn_a_share_quote_refresh",
      cron_expr: "30 9 * * 1-5",
      enabled: true,
      market: "CN",
      timezone: "Asia/Shanghai",
      trade_window: "trading_time",
      scope_json: "{\"symbols\":[\"CN:SH:600519\"]}",
      params_json: "{}",
      catchup_enabled: true,
      catchup_max_days: 5,
      timeout_seconds: 120,
    });
    await schedulerJobsBackfill({
      id: 7,
      dateFrom: "2026-06-17",
      dateTo: "2026-06-19",
      symbols: ["CN:SH:600519"],
    });

    expect(calls).toEqual([
      {
        command: "scheduler_jobs_save",
        payload: {
          payload: {
            id: 7,
            name: "A 股开盘行情刷新",
            cron_type: "cn_a_share_quote_refresh",
            cron_expr: "30 9 * * 1-5",
            enabled: true,
            market: "CN",
            timezone: "Asia/Shanghai",
            trade_window: "trading_time",
            scope_json: "{\"symbols\":[\"CN:SH:600519\"]}",
            params_json: "{}",
            catchup_enabled: true,
            catchup_max_days: 5,
            timeout_seconds: 120,
          },
        },
      },
      {
        command: "scheduler_jobs_backfill",
        payload: {
          payload: {
            id: 7,
            dateFrom: "2026-06-17",
            dateTo: "2026-06-19",
            symbols: ["CN:SH:600519"],
          },
        },
      },
    ]);
  });

  test("调度 service 透传统一错误响应", async () => {
    mockIPC(() => ({
      code: 40002,
      message: "invalid_scheduler_refresh_symbol",
      traceId: "trace-1",
      requestId: "request-1",
      data: null,
    }));

    await expect(
      schedulerRefreshSymbol({
        symbol: "CN:SH:600519",
        data_type: "quote",
        target_date: "2026-06-19",
      }),
    ).rejects.toThrow("invalid_scheduler_refresh_symbol");
  });
});
