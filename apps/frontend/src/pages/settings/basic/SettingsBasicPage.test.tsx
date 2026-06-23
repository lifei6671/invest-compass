/* @vitest-environment jsdom */

import { clearMocks, mockIPC } from "@tauri-apps/api/mocks";
import "@testing-library/jest-dom/vitest";
import "../../../test/setupDom";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { App as AntApp } from "antd";
import { afterEach, expect, test, vi } from "vitest";
import { SettingsBasicPage } from "./SettingsBasicPage";
import { useDashboardStore } from "../../../stores/dashboardStore";

const dialogOpenMock = vi.hoisted(() => vi.fn());

vi.mock("@tauri-apps/plugin-dialog", () => ({
  open: dialogOpenMock,
}));

const defaultAIConfig = {
  id: 1,
  name: "DeepSeek",
  provider: "openai-compatible",
  base_url: "https://api.deepseek.com",
  api_key_ref: "local-vault://ai-config/deepseek-1",
  masked_api_key: "sk-...seek",
  has_api_key: true,
  model_name: "DeepSeek-V3",
  temperature: 0.2,
  max_tokens: 4096,
  timeout_seconds: 120,
  stream_enabled: true,
  is_default: true,
};

afterEach(() => {
  clearMocks();
  useDashboardStore.setState({ state: null, loading: false, error: null, lastLoadedAt: null });
  dialogOpenMock.mockReset();
  cleanup();
});

test("基础设置页展示搜索索引状态并通过固定 scope 触发报告索引重建", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "settings_get":
        return { code: 0, message: "ok", data: { items: [] } };
      case "workspace_get":
        return { code: 0, message: "ok", data: { path: "/Users/demo/Documents/Invest Compass" } };
      case "ai_config_list":
        return { code: 0, message: "ok", data: { items: [defaultAIConfig] } };
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
      payload: { payload: { scope: "reports" } },
    });
  });
  await waitFor(() => {
    expect(screen.getByText("索引重建任务已创建：task-search-1，可在任务历史查看")).toBeInTheDocument();
  });
  expect(calls.map((call) => call.command)).not.toContain("search_global");
});

test("基础设置页初次读取搜索索引遇到 sidecar 瞬断时会重试一次", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  let searchStatusAttempts = 0;
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "settings_get":
        return { code: 0, message: "ok", data: { items: [] } };
      case "workspace_get":
        return { code: 0, message: "ok", data: { path: "/Users/demo/Documents/Invest Compass" } };
      case "ai_config_list":
        return { code: 0, message: "ok", data: { items: [defaultAIConfig] } };
      case "cache_stats":
        return { code: 0, message: "ok", data: { total_bytes: 0, items: [] } };
      case "search_status":
        searchStatusAttempts += 1;
        if (searchStatusAttempts === 1) {
          throw new Error("sidecar http error: error sending request for url (http://127.0.0.1:54005/api/search/status)");
        }
        return {
          code: 0,
          message: "ok",
          data: {
            fts5_status: "available",
            gse_status: "fallback",
            search_status: "need_rebuild",
            active_stock_batch_id: "",
            active_document_batch_id: "",
            running_rebuild_task_id: "",
            stock_index_count: 0,
            report_index_count: 0,
            news_index_count: 0,
            watchlist_note_index_count: 0,
            last_rebuild_at: "",
            tokenizer_name: "simple",
            tokenizer_version: "1",
            dictionary_hash: "builtin",
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
    expect(screen.getByText("NEED_REBUILD")).toBeInTheDocument();
  });
  expect(screen.getByRole("button", { name: /重建全部索引/ })).toBeEnabled();
  expect(screen.getByRole("button", { name: /重建报告索引/ })).toBeDisabled();
  expect(screen.getByRole("button", { name: /重建新闻索引/ })).toBeDisabled();
  expect(searchStatusAttempts).toBe(2);
  expect(useDashboardStore.getState().error).toBeNull();
});

test("基础设置页读取真实工作区路径并回显", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "settings_get":
        return { code: 0, message: "ok", data: { items: [] } };
      case "workspace_get":
        return { code: 0, message: "ok", data: { path: "/Users/demo/Documents/Invest Compass" } };
      case "ai_config_list":
        return { code: 0, message: "ok", data: { items: [defaultAIConfig] } };
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
            active_stock_batch_id: "",
            active_document_batch_id: "",
            running_rebuild_task_id: "",
            stock_index_count: 0,
            report_index_count: 0,
            news_index_count: 0,
            watchlist_note_index_count: 0,
            last_rebuild_at: "",
            tokenizer_name: "simple",
            tokenizer_version: "1",
            dictionary_hash: "builtin",
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
    expect(screen.getByDisplayValue("/Users/demo/Documents/Invest Compass")).toBeInTheDocument();
  });
  expect(calls.some((call) => call.command === "workspace_get")).toBe(true);
});

test("桌面能力读取真实状态并保存到对应后端能力", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "settings_get":
        return {
          code: 0,
          message: "ok",
          data: { items: [{ key: "window.close_to_tray", value: "false" }] },
        };
      case "workspace_get":
        return { code: 0, message: "ok", data: { path: "/Users/demo/Documents/Invest Compass" } };
      case "autostart_get":
        return { enabled: false };
      case "autostart_set":
        return { enabled: true };
      case "settings_set":
        return { code: 0, message: "ok", data: { saved_keys: ["window.close_to_tray"] } };
      case "ai_config_list":
        return { code: 0, message: "ok", data: { items: [defaultAIConfig] } };
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
            active_stock_batch_id: "",
            active_document_batch_id: "",
            running_rebuild_task_id: "",
            stock_index_count: 0,
            report_index_count: 0,
            news_index_count: 0,
            watchlist_note_index_count: 0,
            last_rebuild_at: "",
            tokenizer_name: "simple",
            tokenizer_version: "1",
            dictionary_hash: "builtin",
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
    expect(calls.some((call) => call.command === "autostart_get")).toBe(true);
  });
  const switches = screen.getAllByRole("switch");
  expect(switches[2]).not.toBeChecked();
  expect(switches[3]).not.toBeChecked();

  fireEvent.click(switches[2]);
  fireEvent.click(switches[3]);

  await waitFor(() => {
    expect(calls).toContainEqual({ command: "autostart_set", payload: { enabled: true } });
  });
  expect(calls).toContainEqual({
    command: "settings_set",
    payload: { payload: { items: [{ key: "window.close_to_tray", value: "true" }] } },
  });
});

test("清理缓存前展示确认弹窗并在确认后刷新统计", async () => {
  let cacheStatsCalls = 0;
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "settings_get":
        return { code: 0, message: "ok", data: { items: [] } };
      case "workspace_get":
        return { code: 0, message: "ok", data: { path: "/Users/demo/Documents/Invest Compass" } };
      case "autostart_get":
        return { enabled: false };
      case "ai_config_list":
        return { code: 0, message: "ok", data: { items: [defaultAIConfig] } };
      case "cache_stats":
        cacheStatsCalls += 1;
        return {
          code: 0,
          message: "ok",
          data: {
            total_bytes: cacheStatsCalls === 1 ? 1572864 : 0,
            items: cacheStatsCalls === 1
              ? [
                  { target: "quote", label: "行情缓存", bytes: 1048576, cleanable: true },
                  { target: "report", label: "报告数据", bytes: 524288, cleanable: false },
                ]
              : [],
          },
        };
      case "cache_clean":
        return { code: 0, message: "ok", data: { cleaned_targets: ["quote"] } };
      case "search_status":
        return {
          code: 0,
          message: "ok",
          data: {
            fts5_status: "available",
            gse_status: "fallback",
            search_status: "ready",
            active_stock_batch_id: "",
            active_document_batch_id: "",
            running_rebuild_task_id: "",
            stock_index_count: 0,
            report_index_count: 0,
            news_index_count: 0,
            watchlist_note_index_count: 0,
            last_rebuild_at: "",
            tokenizer_name: "simple",
            tokenizer_version: "1",
            dictionary_hash: "builtin",
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
    expect(screen.getByText("1.5 MB")).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: /清理缓存/ }));

  await waitFor(() => {
    expect(screen.getByText("确认清理缓存")).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: /取\s*消|取消/ }));
  expect(calls.map((call) => call.command)).not.toContain("cache_clean");

  fireEvent.click(screen.getByRole("button", { name: /清理缓存/ }));
  await waitFor(() => {
    expect(screen.getByText("确认清理缓存")).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: "确认清理" }));

  await waitFor(() => {
    expect(calls).toContainEqual({
      command: "cache_clean",
      payload: { payload: { targets: ["quote"] } },
    });
  });
  await waitFor(() => {
    expect(screen.getAllByText("0 MB").length).toBeGreaterThan(0);
  });
});

test("选择工作区目录时先预检再确认迁移", async () => {
  dialogOpenMock.mockResolvedValue("/Users/demo/Documents/Invest Compass Next");
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "settings_get":
        return { code: 0, message: "ok", data: { items: [] } };
      case "workspace_get":
        return { code: 0, message: "ok", data: { path: "/Users/demo/Documents/Invest Compass" } };
      case "workspace_migration_plan":
        return {
          code: 0,
          message: "ok",
          data: {
            current_path: "/Users/demo/Documents/Invest Compass",
            target_path: "/Users/demo/Documents/Invest Compass Next",
            can_migrate: true,
            reason: "",
            warnings: [],
            source_size_bytes: 1048576,
            available_space_bytes: 1073741824,
            target_exists: true,
            target_empty: true,
            will_create_target: false,
          },
        };
      case "workspace_migrate":
        return {
          code: 0,
          message: "ok",
          data: {
            path: "/Users/demo/Documents/Invest Compass Next",
            migrated_files: ["invest-compass.sqlite3", "logs"],
            backup_path: "/Users/demo/Documents/Invest Compass/backups/workspace-migration-1",
          },
        };
      case "ai_config_list":
        return { code: 0, message: "ok", data: { items: [defaultAIConfig] } };
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
            active_stock_batch_id: "",
            active_document_batch_id: "",
            running_rebuild_task_id: "",
            stock_index_count: 0,
            report_index_count: 0,
            news_index_count: 0,
            watchlist_note_index_count: 0,
            last_rebuild_at: "",
            tokenizer_name: "simple",
            tokenizer_version: "1",
            dictionary_hash: "builtin",
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
    expect(screen.getByDisplayValue("/Users/demo/Documents/Invest Compass")).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: "选择目录" }));

  await waitFor(() => {
    expect(screen.getByText("确认迁移工作区")).toBeInTheDocument();
  });
  expect(screen.getByText("1.0 MB")).toBeInTheDocument();
  expect(screen.getByText("1.0 GB")).toBeInTheDocument();
  expect(calls).toContainEqual({
    command: "workspace_migration_plan",
    payload: { payload: { target_path: "/Users/demo/Documents/Invest Compass Next" } },
  });

  fireEvent.click(screen.getByRole("button", { name: "确认迁移" }));

  await waitFor(() => {
    expect(calls).toContainEqual({
      command: "workspace_migrate",
      payload: {
        payload: {
          target_path: "/Users/demo/Documents/Invest Compass Next",
          include_cache: false,
          create_backup: true,
        },
      },
    });
  });
  await waitFor(() => {
    expect(screen.getByDisplayValue("/Users/demo/Documents/Invest Compass Next")).toBeInTheDocument();
  });
});

test("工作区迁移预检不通过时不执行迁移", async () => {
  dialogOpenMock.mockResolvedValue("/Users/demo/Documents/Invest Compass Existing");
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "settings_get":
        return { code: 0, message: "ok", data: { items: [] } };
      case "workspace_get":
        return { code: 0, message: "ok", data: { path: "/Users/demo/Documents/Invest Compass" } };
      case "workspace_migration_plan":
        return {
          code: 0,
          message: "ok",
          data: {
            current_path: "/Users/demo/Documents/Invest Compass",
            target_path: "/Users/demo/Documents/Invest Compass Existing",
            can_migrate: false,
            reason: "目标目录非空，默认不会覆盖已有工作区",
            warnings: [],
            source_size_bytes: 0,
            available_space_bytes: 1073741824,
            target_exists: true,
            target_empty: false,
            will_create_target: false,
          },
        };
      case "ai_config_list":
        return { code: 0, message: "ok", data: { items: [defaultAIConfig] } };
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
            active_stock_batch_id: "",
            active_document_batch_id: "",
            running_rebuild_task_id: "",
            stock_index_count: 0,
            report_index_count: 0,
            news_index_count: 0,
            watchlist_note_index_count: 0,
            last_rebuild_at: "",
            tokenizer_name: "simple",
            tokenizer_version: "1",
            dictionary_hash: "builtin",
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
    expect(screen.getByDisplayValue("/Users/demo/Documents/Invest Compass")).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: "选择目录" }));

  await waitFor(() => {
    expect(screen.getAllByText("目标目录非空，默认不会覆盖已有工作区").length).toBeGreaterThan(0);
  });
  expect(calls.map((call) => call.command)).not.toContain("workspace_migrate");
});

test("打开工作区目录通过固定 Rust command 执行", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "settings_get":
        return { code: 0, message: "ok", data: { items: [] } };
      case "workspace_get":
        return { code: 0, message: "ok", data: { path: "/Users/demo/Documents/Invest Compass" } };
      case "workspace_open":
        return { opened: true };
      case "ai_config_list":
        return { code: 0, message: "ok", data: { items: [defaultAIConfig] } };
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
            active_stock_batch_id: "",
            active_document_batch_id: "",
            running_rebuild_task_id: "",
            stock_index_count: 0,
            report_index_count: 0,
            news_index_count: 0,
            watchlist_note_index_count: 0,
            last_rebuild_at: "",
            tokenizer_name: "simple",
            tokenizer_version: "1",
            dictionary_hash: "builtin",
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
    expect(screen.getByDisplayValue("/Users/demo/Documents/Invest Compass")).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: "打开目录" }));

  await waitFor(() => {
    expect(calls).toContainEqual({ command: "workspace_open", payload: {} });
  });
});

test("基础设置切换后立即保存普通配置和默认模型", async () => {
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    const args = payload as any;
    calls.push({ command, payload });
    switch (command) {
      case "settings_get":
        return { code: 0, message: "ok", data: { items: [{ key: "market.default", value: "CN" }] } };
      case "workspace_get":
        return { code: 0, message: "ok", data: { path: "/Users/demo/Documents/Invest Compass" } };
      case "ai_config_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              defaultAIConfig,
              { ...defaultAIConfig, id: 2, name: "Qwen", model_name: "qwen-max", is_default: false },
            ],
          },
        };
      case "settings_set":
        return { code: 0, message: "ok", data: { saved_keys: args?.payload?.items?.map((item: { key: string }) => item.key) ?? [] } };
      case "ai_config_save":
        return { code: 0, message: "ok", data: { config: args.payload } };
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
            active_stock_batch_id: "",
            active_document_batch_id: "",
            running_rebuild_task_id: "",
            stock_index_count: 0,
            report_index_count: 0,
            news_index_count: 0,
            watchlist_note_index_count: 0,
            last_rebuild_at: "",
            tokenizer_name: "simple",
            tokenizer_version: "1",
            dictionary_hash: "builtin",
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
    expect(screen.getByText("DeepSeek / DeepSeek-V3")).toBeInTheDocument();
  });

  openBasicSettingSelect("默认市场");
  clickSelectOption("港股");
  await waitFor(() => {
    expect(calls).toContainEqual({
      command: "settings_set",
      payload: { payload: { items: [{ key: "market.default", value: "HK" }] } },
    });
  });

  openBasicSettingSelect("默认 AI 模型");
  clickSelectOption("Qwen / qwen-max");
  await waitFor(() => {
    expect(calls.some((call) => call.command === "ai_config_save" && call.payload?.payload?.id === 2 && call.payload.payload.is_default === true)).toBe(true);
  });
});

test("基础设置初始化读取失败时合并全局错误提示", async () => {
  useDashboardStore.setState({
    state: {
      health: { status: "ok", version: "0.1.0" },
      summary: {
        watchlist: { up_count: 0, down_count: 0, flat_count: 0 },
        recent_reports: [],
        recent_tasks: [],
        market_news: [],
        risk_tips: [],
        provider_statuses: [],
      },
      indexQuotes: [],
      indexTrends: {},
      watchlistRows: [],
    },
    loading: false,
    error: null,
    lastLoadedAt: "2026-06-22T12:00:00Z",
  });
  mockIPC((command) => {
    switch (command) {
      case "settings_get":
      case "workspace_get":
      case "cache_stats":
      case "search_status":
        throw "core sidecar is not running";
      case "ai_config_list":
        return { code: 0, message: "ok", data: { items: [] } };
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
    expect(screen.getAllByText(/读取失败，请稍后重试/)).toHaveLength(1);
  });
  expect(screen.queryByText("设置页数据读取失败，请稍后重试")).not.toBeInTheDocument();
  expect(useDashboardStore.getState().state).toBeNull();
  expect(useDashboardStore.getState().error).toBe("core sidecar is not running");
});

function openBasicSettingSelect(label: string) {
  const field = screen.getByText(label).closest(".settings-basic-field");
  const select = field?.querySelector(".ant-select");
  if (!select) {
    throw new Error(`missing select for ${label}`);
  }
  fireEvent.mouseDown(select);
}

function clickSelectOption(label: string) {
  const option = Array.from(document.querySelectorAll(".ant-select-item-option")).find((item) => item.textContent === label);
  if (!option) {
    throw new Error(`missing option ${label}`);
  }
  fireEvent.click(option);
}
