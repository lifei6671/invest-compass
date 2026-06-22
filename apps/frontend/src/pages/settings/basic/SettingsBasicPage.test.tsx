/* @vitest-environment jsdom */

import { clearMocks, mockIPC } from "@tauri-apps/api/mocks";
import "@testing-library/jest-dom/vitest";
import "../../../test/setupDom";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { App as AntApp } from "antd";
import { afterEach, expect, test } from "vitest";
import { SettingsBasicPage } from "./SettingsBasicPage";

afterEach(() => {
  clearMocks();
  cleanup();
});

test("基础设置页展示搜索索引状态并通过固定 scope 触发报告索引重建", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "cache_stats":
        return { code: 0, message: "ok", data: { total_bytes: 0, items: [] } };
      case "search_status":
        return {
          code: 0,
          message: "ok",
          data: {
            fts5_status: "available",
            gse_status: "fallback",
            search_status: "ready",
            active_stock_batch_id: "stock-ready-1",
            active_document_batch_id: "doc-ready-1",
            running_rebuild_task_id: "",
            stock_index_count: 12,
            report_index_count: 3,
            news_index_count: 4,
            watchlist_note_index_count: 5,
            last_rebuild_at: "2026-06-22T13:30:00Z",
            tokenizer_name: "simple",
            tokenizer_version: "1",
            dictionary_hash: "builtin",
          },
        };
      case "search_rebuild":
        return {
          code: 0,
          message: "ok",
          data: {
            task_id: "task-search-1",
            scope: "reports",
            stock_batch_id: "stock-ready-1",
            document_batch_id: "doc-building-1",
          },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(
    <AntApp>
      <SettingsBasicPage />
    </AntApp>,
  );

  await waitFor(() => {
    expect(screen.getByText("搜索索引")).toBeInTheDocument();
  });
  expect(screen.getByText("FTS5")).toBeInTheDocument();
  expect(screen.getByText("AVAILABLE")).toBeInTheDocument();
  expect(screen.getByText("FALLBACK")).toBeInTheDocument();
  expect(screen.getByText("READY")).toBeInTheDocument();
  expect(screen.getByText("股票索引数量")).toBeInTheDocument();
  expect(screen.getByText("12")).toBeInTheDocument();
  expect(screen.getByText("报告索引数量")).toBeInTheDocument();
  expect(screen.getByText("3")).toBeInTheDocument();
  expect(screen.getByText("新闻索引数量")).toBeInTheDocument();
  expect(screen.getByText("4")).toBeInTheDocument();
  expect(screen.getByText("自选备注索引数量")).toBeInTheDocument();
  expect(screen.getByText("5")).toBeInTheDocument();
  expect(screen.getByText("2026-06-22T13:30:00Z")).toBeInTheDocument();
  expect(screen.getByText("simple@1")).toBeInTheDocument();
  expect(screen.getByText("builtin")).toBeInTheDocument();
  expect(screen.getByText("stock-ready-1")).toBeInTheDocument();
  expect(screen.getByText("doc-ready-1")).toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: /重建报告索引/ }));

  await waitFor(() => {
    expect(calls).toContainEqual({
      command: "search_rebuild",
      payload: { payload: { scope: "reports", force: false } },
    });
  });
  expect(calls.map((call) => call.command)).not.toContain("search_global");
});
