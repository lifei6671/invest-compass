/* @vitest-environment jsdom */

import { clearMocks, mockIPC } from "@tauri-apps/api/mocks";
import { afterEach, describe, expect, test } from "vitest";
import {
  aiConfigDelete,
  aiConfigList,
  aiConfigSave,
  aiConfigTest,
  analysisTaskCancel,
  analysisTaskCreate,
  analysisTaskSubscribe,
  appBootStatus,
  CoreClientError,
  coreHealth,
  dashboardSummary,
  marketIndicators,
  marketKline,
  marketQuote,
  newsList,
  newsMarket,
  notificationsClearRead,
  notificationsList,
  notificationsMarkAllRead,
  notificationsMarkRead,
  notificationsUnreadCount,
  openExternalURL,
  promptTemplatesCreate,
  promptTemplatesDelete,
  promptTemplatesGet,
  promptTemplatesList,
  promptTemplatesUpdate,
  reportDelete,
  reportGet,
  reportList,
  cacheClean,
  cacheStats,
  checkUpdate,
  exportLogs,
  providersStatus,
  schedulerJobTypes,
  schedulerJobsBackfill,
  schedulerJobsDelete,
  schedulerJobsGet,
  schedulerJobsList,
  schedulerJobsRunNow,
  schedulerJobsSave,
  schedulerJobsSetEnabled,
  schedulerRefreshSymbol,
  schedulerRunsGet,
  schedulerRunsList,
  schedulerRunsTrigger,
  schedulerStatus,
  searchNews,
  searchRebuild,
  searchReports,
  searchStatus,
  searchWatchlistNotes,
  stockProfile,
  stockSearch,
  settingsGet,
  settingsSet,
  taskEvents,
  taskGet,
  taskList,
  watchlistCreate,
  watchlistDelete,
  watchlistList,
  watchlistUpdate,
  workspaceGet,
  workspaceSet,
} from "./coreClient";

describe("coreClient", () => {
  afterEach(() => {
    clearMocks();
  });

  test("coreHealth 通过固定 Tauri command 读取 Go core 健康状态", async () => {
    const calls: Array<{ command: string; payload?: unknown }> = [];

    mockIPC((command, payload) => {
      calls.push({ command, payload });
      return {
        code: 0,
        message: "ok",
        data: {
          status: "ok",
          version: "0.1.0",
          dbStatus: "ok",
        },
      };
    });

    await expect(coreHealth()).resolves.toEqual({
      status: "ok",
      version: "0.1.0",
      dbStatus: "ok",
    });
    expect(calls).toEqual([{ command: "core_health", payload: {} }]);
  });

  test("coreHealth 遇到统一错误响应时抛出业务错误", async () => {
    mockIPC(() => ({
      code: 50001,
      message: "sidecar not ready token=secret-token",
      traceId: "trace-1",
      requestId: "request-1",
      data: null,
    }));

    await expect(coreHealth()).rejects.toMatchObject({
      name: "CoreClientError",
      message: "sidecar not ready token=[REDACTED]",
      code: 50001,
      traceId: "trace-1",
      requestId: "request-1",
    });
  });

  test("coreHealth 遇到 Rust command 错误时统一脱敏", async () => {
    mockIPC(() => {
      throw new Error("sidecar http error Authorization: Bearer sk-test-secret-token");
    });

    await expect(coreHealth()).rejects.toMatchObject({
      name: "CoreClientError",
      message: "sidecar http error Authorization: [REDACTED]",
    });
    await expect(coreHealth()).rejects.toBeInstanceOf(CoreClientError);
  });

  test("appBootStatus 通过固定 Tauri command 读取初始化状态", async () => {
    const calls: Array<{ command: string; payload?: unknown }> = [];
    mockIPC((command, payload) => {
      calls.push({ command, payload });
      return {
        code: 0,
        message: "ok",
        data: {
          ready: false,
          progress: 68,
          currentStepId: "sqlite_migration",
          steps: [
            { id: "sidecar", index: 1, title: "启动 Go Core Sidecar", status: "completed", badgeText: "已完成" },
            { id: "sqlite_migration", index: 3, title: "SQLite 数据库迁移 / 重建索引", status: "running", badgeText: "进行中" },
          ],
          taskDetail: {
            taskId: "boot-1",
            elapsed: "00:00:01",
            currentStage: "SQLite 数据库迁移 / 重建索引",
            remaining: "计算中",
          },
          logs: [{ id: "1", time: "15:29:41", status: "success", message: "sidecar ready" }],
        },
      };
    });

    await expect(appBootStatus()).resolves.toMatchObject({
      ready: false,
      progress: 68,
      currentStepId: "sqlite_migration",
    });
    expect(calls).toEqual([{ command: "app_boot_status", payload: {} }]);
  });

  test("dashboardSummary 通过固定 Tauri command 读取总览数据", async () => {
    const calls: Array<{ command: string; payload?: unknown }> = [];
    mockIPC((command, payload) => {
      calls.push({ command, payload });
      return {
        code: 0,
        message: "ok",
        data: {
          watchlist: { up_count: 2, down_count: 1, flat_count: 0 },
          recent_reports: [{ id: 1, title: "浦发银行分析", symbol: "600000.SH", risk_summary: "波动风险" }],
          recent_tasks: [{ id: "task-1", title: "生成分析报告", status: "SUCCESS", progress: 100 }],
          market_news: [{ title: "市场新闻", source: "eastmoney", published_at: "2026-06-19T09:30:00Z" }],
          risk_tips: ["仅作研究辅助，不构成投资建议"],
          provider_statuses: [{ name: "market", source: "sina", available: true, last_error: "" }],
        },
      };
    });

    await expect(dashboardSummary()).resolves.toMatchObject({
      watchlist: { up_count: 2, down_count: 1, flat_count: 0 },
      recent_reports: [{ title: "浦发银行分析" }],
      risk_tips: ["仅作研究辅助，不构成投资建议"],
    });
    expect(calls).toEqual([{ command: "dashboard_summary", payload: {} }]);
  });

  test("dashboardSummary 将后端空列表 null 规范化为空数组", async () => {
    mockIPC(() => ({
      code: 0,
      message: "ok",
      data: {
        watchlist: { up_count: 0, down_count: 0, flat_count: 0 },
        recent_reports: null,
        recent_tasks: null,
        market_news: null,
        risk_tips: null,
        provider_statuses: null,
      },
    }));

    await expect(dashboardSummary()).resolves.toMatchObject({
      recent_reports: [],
      recent_tasks: [],
      market_news: [],
      risk_tips: [],
      provider_statuses: [],
    });
  });

  test("watchlistList 通过固定 Tauri command 读取自选股并保留股票资料字段", async () => {
    const calls: Array<{ command: string; payload?: unknown }> = [];
    mockIPC((command, payload) => {
      calls.push({ command, payload });
      return {
        code: 0,
        message: "ok",
        data: {
          items: [{
            id: 1,
            symbol: "600000.SH",
            sort_order: 10,
            tags: ["银行"],
            note: "低估值观察",
            name: "浦发银行",
            code: "600000",
            market: "CN",
            exchange: "SH",
            industry: "银行",
            concepts: ["低估值", "大金融"],
            list_date: "1999-11-10",
            status: "active",
            full_name: "上海浦东发展银行股份有限公司",
          }],
        },
      };
    });

    await expect(watchlistList()).resolves.toEqual({
      items: [{
        id: 1,
        symbol: "600000.SH",
        sort_order: 10,
        tags: ["银行"],
        note: "低估值观察",
        name: "浦发银行",
        code: "600000",
        market: "CN",
        exchange: "SH",
        industry: "银行",
        concepts: ["低估值", "大金融"],
        list_date: "1999-11-10",
        status: "active",
        full_name: "上海浦东发展银行股份有限公司",
      }],
    });
    expect(calls).toEqual([{ command: "watchlist_list", payload: {} }]);
  });

  test("stockProfile 通过固定 Tauri command 读取个股基础资料", async () => {
    const calls: Array<{ command: string; payload?: unknown }> = [];
    mockIPC((command, payload) => {
      calls.push({ command, payload });
      return {
        code: 0,
        message: "ok",
        data: {
          symbol: "600000.SH",
          name: "浦发银行",
          code: "600000",
          market: "CN",
          exchange: "SH",
          industry: "银行",
          concepts: ["低估值", "大金融"],
          list_date: "1999-11-10",
          status: "active",
          full_name: "上海浦东发展银行股份有限公司",
        },
      };
    });

    await expect(stockProfile("600000.SH")).resolves.toEqual({
      symbol: "600000.SH",
      name: "浦发银行",
      code: "600000",
      market: "CN",
      exchange: "SH",
      industry: "银行",
      concepts: ["低估值", "大金融"],
      list_date: "1999-11-10",
      status: "active",
      full_name: "上海浦东发展银行股份有限公司",
    });
    expect(calls).toEqual([{ command: "stock_profile", payload: { symbol: "600000.SH" } }]);
  });

  test("watchlistCreate/Update/Delete 通过固定 Tauri command 修改自选股", async () => {
    const calls: Array<{ command: string; payload?: unknown }> = [];
    mockIPC((command, payload) => {
      calls.push({ command, payload });
      return {
        code: 0,
        message: "ok",
        data: { id: 3, symbol: "600000.SH", sort_order: 20, tags: ["银行"], note: "观察" },
      };
    });

    await watchlistCreate({ symbol: "600000.SH", sort_order: 20, tags: ["银行"], note: "观察" });
    await watchlistUpdate({ id: 3, sort_order: 30, tags: ["金融"], note: "更新备注" });
    await watchlistDelete(3);

    expect(calls).toEqual([
      {
        command: "watchlist_create",
        payload: { payload: { symbol: "600000.SH", sort_order: 20, tags: ["银行"], note: "观察" } },
      },
      {
        command: "watchlist_update",
        payload: { payload: { id: 3, sort_order: 30, tags: ["金融"], note: "更新备注" } },
      },
      { command: "watchlist_delete", payload: { id: 3 } },
    ]);
  });

  test("stockSearch 和 marketQuote 通过固定 Tauri command 读取真实搜索与行情", async () => {
    const calls: Array<{ command: string; payload?: unknown }> = [];
    mockIPC((command, payload) => {
      calls.push({ command, payload });
      if (command === "stock_search") {
        return {
          code: 0,
          message: "ok",
          data: [{ symbol: "600000.SH", name: "浦发银行", code: "600000", market: "CN", exchange: "SH" }],
        };
      }
      return {
        code: 0,
        message: "ok",
        data: {
          symbol: "600000.SH",
          price: 7.12,
          change_amount: 0.1,
          change_percent: 1.42,
          quote_time: "2026-06-19T10:00:00Z",
          provider: "sina",
        },
      };
    });

    await expect(stockSearch("浦发")).resolves.toEqual([
      { symbol: "600000.SH", name: "浦发银行", code: "600000", market: "CN", exchange: "SH" },
    ]);
    await expect(marketQuote("600000.SH")).resolves.toMatchObject({
      symbol: "600000.SH",
      price: 7.12,
      provider: "sina",
    });
    expect(calls).toEqual([
      { command: "stock_search", payload: { keyword: "浦发" } },
      { command: "market_quote", payload: { symbol: "600000.SH" } },
    ]);
  });

  test("marketKline、marketIndicators、newsList 和 newsMarket 通过固定 Tauri command 读取详情与资讯中心数据", async () => {
    const calls: Array<{ command: string; payload?: unknown }> = [];
    mockIPC((command, payload) => {
      calls.push({ command, payload });
      if (command === "market_kline") {
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                symbol: "600000.SH",
                period: "day",
                adjust: "qfq",
                trade_date: "2026-06-19",
                open: 7.01,
                high: 7.2,
                low: 6.98,
                close: 7.12,
                volume: 1234000,
                amount: 8780000,
                provider: "tencent",
              },
            ],
          },
        };
      }
      if (command === "market_indicators") {
        return {
          code: 0,
          message: "ok",
          data: {
            symbol: "600000.SH",
            period: "day",
            adjust: "qfq",
            indicators: {
              ma: { ma5: [7.12] },
              rsi: [55.1],
            },
          },
        };
      }
      if (command === "news_market") {
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 2,
                source: "eastmoney",
                title: "市场新闻",
                url: "https://example.com/news/2",
                summary: "市场动态",
                published_at: "2026-06-19T10:05:00Z",
                tags: ["市场"],
              },
            ],
          },
        };
      }
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: 1,
              source: "eastmoney",
              title: "浦发银行新闻",
              url: "https://example.com/news/1",
              summary: "银行板块新闻",
              published_at: "2026-06-19T10:00:00Z",
              symbols: ["600000.SH"],
              tags: ["银行"],
            },
          ],
        },
      };
    });

    await expect(
      marketKline({ symbol: "600000.SH", period: "day", adjust: "qfq", limit: 60 }),
    ).resolves.toMatchObject({
      items: [{ symbol: "600000.SH", close: 7.12, provider: "tencent" }],
    });
    await expect(
      marketIndicators({
        symbol: "600000.SH",
        period: "day",
        adjust: "qfq",
        limit: 60,
        indicators: ["ma", "rsi"],
      }),
    ).resolves.toMatchObject({
      symbol: "600000.SH",
      indicators: { ma: { ma5: [7.12] } },
    });
    await expect(newsList({ symbol: "600000.SH", limit: 20 })).resolves.toMatchObject({
      items: [{ title: "浦发银行新闻", url: "https://example.com/news/1" }],
    });
    await expect(newsMarket({ market: "CN", limit: 20 })).resolves.toMatchObject({
      items: [{ title: "市场新闻", url: "https://example.com/news/2" }],
    });

    expect(calls).toEqual([
      {
        command: "market_kline",
        payload: { symbol: "600000.SH", period: "day", adjust: "qfq", limit: 60 },
      },
      {
        command: "market_indicators",
        payload: {
          symbol: "600000.SH",
          period: "day",
          adjust: "qfq",
          limit: 60,
          indicators: ["ma", "rsi"],
        },
      },
      {
        command: "news_list",
        payload: { symbol: "600000.SH", limit: 20 },
      },
      {
        command: "news_market",
        payload: { market: "CN", limit: 20 },
      },
    ]);
  });

  test("openExternalURL 通过固定 command 打开 HTTPS 外链", async () => {
    const calls: Array<{ command: string; payload?: unknown }> = [];
    mockIPC((command, payload) => {
      calls.push({ command, payload });
      return { ok: true };
    });

    await expect(openExternalURL("https://example.com/news/1")).resolves.toEqual({ ok: true });
    expect(calls).toEqual([{ command: "open_external_url", payload: { url: "https://example.com/news/1" } }]);
  });

  test("AI 配置方法通过固定 Tauri command 读写安全展示字段和一次性 API Key", async () => {
    const calls: Array<{ command: string; payload?: unknown }> = [];
    mockIPC((command, payload) => {
      calls.push({ command, payload });
      if (command === "ai_config_list") {
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 1,
                name: "OpenAI 主配置",
                provider: "openai-compatible",
                base_url: "https://api.openai.com",
                api_key_ref: "local-vault://ai-config/openai-1",
                masked_api_key: "sk-...abcd",
                has_api_key: true,
                model_name: "gpt-4.1-mini",
                temperature: 0.2,
                max_tokens: 4096,
                timeout_seconds: 120,
                stream_enabled: true,
                is_default: true,
              },
            ],
          },
        };
      }
      if (command === "ai_config_test") {
        return {
          code: 0,
          message: "ok",
          data: { ok: true, provider: "openai-compatible", model: "gpt-4.1-mini", message: "ok" },
        };
      }
      if (command === "ai_config_delete") {
        return { code: 0, message: "ok", data: { deleted_id: 1 } };
      }
      return {
        code: 0,
        message: "ok",
        data: {
          config: {
            id: 2,
            name: "自定义接入点",
            provider: "openai-compatible",
            base_url: "https://llm.example.com",
            api_key_ref: "local-vault://ai-config/custom-2",
            masked_api_key: "sk-...wxyz",
            has_api_key: true,
            model_name: "gpt-4.1-mini",
            temperature: 0.3,
            max_tokens: 2048,
            timeout_seconds: 90,
            stream_enabled: true,
            is_default: false,
          },
        },
      };
    });

    await expect(aiConfigList()).resolves.toMatchObject({
      items: [{ name: "OpenAI 主配置", masked_api_key: "sk-...abcd", has_api_key: true }],
    });
    await expect(
      aiConfigSave({
        id: 0,
        name: "自定义接入点",
        provider: "openai-compatible",
        base_url: "https://llm.example.com",
        api_key_ref: "",
        masked_api_key: "",
        has_api_key: false,
        api_key: "sk-test-once",
        model_name: "gpt-4.1-mini",
        temperature: 0.3,
        max_tokens: 2048,
        timeout_seconds: 90,
        stream_enabled: true,
        is_default: false,
      }),
    ).resolves.toMatchObject({
      config: { id: 2, masked_api_key: "sk-...wxyz", has_api_key: true },
    });
    await expect(aiConfigTest({ id: 1, api_key_ref: "local-vault://ai-config/openai-1" })).resolves.toEqual({
      ok: true,
      provider: "openai-compatible",
      model: "gpt-4.1-mini",
      message: "ok",
    });
    await expect(aiConfigDelete({ id: 1 })).resolves.toEqual({ deleted_id: 1 });

    expect(calls).toEqual([
      { command: "ai_config_list", payload: {} },
      {
        command: "ai_config_save",
        payload: {
          payload: {
            id: 0,
            name: "自定义接入点",
            provider: "openai-compatible",
            base_url: "https://llm.example.com",
            api_key_ref: "",
            masked_api_key: "",
            has_api_key: false,
            api_key: "sk-test-once",
            model_name: "gpt-4.1-mini",
            temperature: 0.3,
            max_tokens: 2048,
            timeout_seconds: 90,
            stream_enabled: true,
            is_default: false,
          },
        },
      },
      {
        command: "ai_config_test",
        payload: { payload: { id: 1, api_key_ref: "local-vault://ai-config/openai-1" } },
      },
      {
        command: "ai_config_delete",
        payload: { payload: { id: 1 } },
      },
    ]);
  });

  test("Prompt 模板方法通过固定 Tauri command 管理首版模板类型和变量", async () => {
    const calls: Array<{ command: string; payload?: unknown }> = [];
    mockIPC((command, payload) => {
      calls.push({ command, payload });
      if (command === "prompt_templates_list") {
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 7,
                name: "技术面模板",
                type: "technical",
                description: "技术指标分析",
                content: "分析 {{stock_name}} 的 {{indicators}}",
              variables: ["stock_name", "indicators"],
              is_builtin: false,
              builtin_locked: false,
              key: "user_technical",
              version: 1,
              checksum: "sha256:user",
              source: "user",
            },
          ],
        },
      };
      }
      if (command === "prompt_templates_delete") {
        return { code: 0, message: "ok", data: { id: 7 } };
      }
      return {
        code: 0,
        message: "ok",
        data: {
          id: 7,
          name: "技术面模板",
          type: "technical",
          description: "技术指标分析",
          content: "分析 {{stock_name}} 的 {{indicators}}",
          variables: ["stock_name", "indicators"],
          is_builtin: false,
          builtin_locked: false,
          key: "user_technical",
          version: 1,
          checksum: "sha256:user",
          source: "user",
        },
      };
    });

    await expect(promptTemplatesList()).resolves.toMatchObject({
      items: [
        {
          id: 7,
          type: "technical",
          variables: ["stock_name", "indicators"],
          key: "user_technical",
          version: 1,
          checksum: "sha256:user",
          source: "user",
          builtin_locked: false,
        },
      ],
    });
    await expect(promptTemplatesGet(7)).resolves.toMatchObject({ id: 7, name: "技术面模板" });
    await promptTemplatesCreate({
      name: "技术面模板",
      type: "technical",
      description: "技术指标分析",
      content: "分析 {{stock_name}} 的 {{indicators}}",
    });
    await promptTemplatesUpdate({
      id: 7,
      name: "技术面模板",
      type: "technical",
      description: "技术指标分析",
      content: "分析 {{stock_name}} 的 {{indicators}}",
    });
    await expect(promptTemplatesDelete(7)).resolves.toEqual({ id: 7 });

    expect(calls.map((call) => call.command)).toEqual([
      "prompt_templates_list",
      "prompt_templates_get",
      "prompt_templates_create",
      "prompt_templates_update",
      "prompt_templates_delete",
    ]);
    expect(calls[2]?.payload).toEqual({
      payload: {
        name: "技术面模板",
        type: "technical",
        description: "技术指标分析",
        content: "分析 {{stock_name}} 的 {{indicators}}",
      },
    });
    expect(calls[4]?.payload).toEqual({ id: 7 });
  });

  test("分析任务方法通过固定 Tauri command 创建、取消并订阅事件", async () => {
    const calls: Array<{ command: string; payload?: unknown }> = [];
    mockIPC((command, payload) => {
      calls.push({ command, payload });
      if (command === "analysis_task_cancel") {
        return { code: 0, message: "ok", data: { task_id: "analysis-1", status: "CANCELLED" } };
      }
      if (command === "analysis_task_subscribe") {
        return { emitted: 2, last_event_id: 9 };
      }
      return { code: 0, message: "ok", data: { task_id: "analysis-1", status: "PENDING" } };
    });

    await expect(
      analysisTaskCreate({
        symbol: "600000.SH",
        analysis_type: "stock_full",
        ai_config_id: 1,
        api_key_ref: "local-vault://ai-config/openai-1",
        prompt_template_id: 7,
        user_position: { cost_price: 7.01, shares: 100, risk_level: "稳健" },
      }),
    ).resolves.toEqual({ task_id: "analysis-1", status: "PENDING" });
    await expect(analysisTaskCancel("analysis-1")).resolves.toEqual({ task_id: "analysis-1", status: "CANCELLED" });
    await expect(analysisTaskSubscribe("analysis-1", 8)).resolves.toEqual({ emitted: 2, last_event_id: 9 });

    expect(calls).toEqual([
      {
        command: "analysis_task_create",
        payload: {
          payload: {
            symbol: "600000.SH",
            analysis_type: "stock_full",
            ai_config_id: 1,
            api_key_ref: "local-vault://ai-config/openai-1",
            prompt_template_id: 7,
            user_position: { cost_price: 7.01, shares: 100, risk_level: "稳健" },
          },
        },
      },
      { command: "analysis_task_cancel", payload: { taskId: "analysis-1" } },
      { command: "analysis_task_subscribe", payload: { taskId: "analysis-1", afterEventId: 8 } },
    ]);
    expect(JSON.stringify(calls)).not.toContain("resolved_api_key");
  });

  test("任务和报告方法通过固定 Tauri command 读取历史与报告详情", async () => {
    const calls: Array<{ command: string; payload?: unknown }> = [];
    mockIPC((command, payload) => {
      calls.push({ command, payload });
      switch (command) {
        case "task_list":
          return { code: 0, message: "ok", data: { items: [{ id: "analysis-1", type: "ANALYSIS", status: "SUCCESS", title: "浦发分析", progress: 100 }] } };
        case "task_get":
          return { code: 0, message: "ok", data: { id: "analysis-1", type: "ANALYSIS", status: "SUCCESS", title: "浦发分析", progress: 100 } };
        case "task_events":
          return { code: 0, message: "ok", data: { items: [{ id: 2, event: "TASK_CHUNK", data: { content: "阶段观点" } }] } };
        case "report_list":
          return { code: 0, message: "ok", data: { items: [{ id: 3, task_id: "analysis-1", symbol: "600000.SH", title: "浦发分析", risk_summary: "波动风险" }] } };
        case "report_get":
          return { code: 0, message: "ok", data: { id: 3, task_id: "analysis-1", symbol: "600000.SH", title: "浦发分析", content_markdown: "## 结论", risk_summary: "波动风险" } };
        case "report_delete":
          return { code: 0, message: "ok", data: { id: 3 } };
        default:
          throw new Error(`unexpected command ${command}`);
      }
    });

    await expect(taskList(20)).resolves.toMatchObject({ items: [{ id: "analysis-1", status: "SUCCESS" }] });
    await expect(taskGet("analysis-1")).resolves.toMatchObject({ id: "analysis-1", title: "浦发分析" });
    await expect(taskEvents("analysis-1", 1)).resolves.toMatchObject({ items: [{ event: "TASK_CHUNK" }] });
    await expect(reportList()).resolves.toMatchObject({ items: [{ id: 3, title: "浦发分析" }] });
    await expect(reportGet(3)).resolves.toMatchObject({ id: 3, content_markdown: "## 结论" });
    await expect(reportDelete(3)).resolves.toEqual({ id: 3 });

    expect(calls).toEqual([
      { command: "task_list", payload: { limit: 20 } },
      { command: "task_get", payload: { taskId: "analysis-1" } },
      { command: "task_events", payload: { taskId: "analysis-1", afterEventId: 1 } },
      { command: "report_list", payload: {} },
      { command: "report_get", payload: { id: 3 } },
      { command: "report_delete", payload: { id: 3 } },
    ]);
  });

  test("设置中心方法通过固定 Tauri command 读取和保存非敏感配置", async () => {
    const calls: Array<{ command: string; payload?: unknown }> = [];
    mockIPC((command, payload) => {
      calls.push({ command, payload });
      switch (command) {
        case "settings_get":
          return { code: 0, message: "ok", data: { items: [{ key: "proxy_url", value: "http://127.0.0.1:7890" }] } };
        case "settings_set":
          return { code: 0, message: "ok", data: { saved_keys: ["proxy_url", "proxy_credential_ref"] } };
        case "workspace_get":
          return { code: 0, message: "ok", data: { path: "/Users/demo/InvestCompass" } };
        case "workspace_set":
          return { code: 0, message: "ok", data: { path: "/Users/demo/InvestCompass2" } };
        default:
          throw new Error(`unexpected command ${command}`);
      }
    });

    await expect(settingsGet(["proxy_url"])).resolves.toEqual({ items: [{ key: "proxy_url", value: "http://127.0.0.1:7890" }] });
    await expect(
      settingsSet({
        items: [{ key: "proxy_url", value: "http://127.0.0.1:7890" }],
        proxy_password: "secret-once",
        proxy_credential_ref: "",
        clear_proxy_credential: false,
      }),
    ).resolves.toEqual({ saved_keys: ["proxy_url", "proxy_credential_ref"] });
    await expect(workspaceGet()).resolves.toEqual({ path: "/Users/demo/InvestCompass" });
    await expect(workspaceSet("/Users/demo/InvestCompass2")).resolves.toEqual({ path: "/Users/demo/InvestCompass2" });

    expect(calls).toEqual([
      { command: "settings_get", payload: { keys: ["proxy_url"] } },
      {
        command: "settings_set",
        payload: {
          payload: {
            items: [{ key: "proxy_url", value: "http://127.0.0.1:7890" }],
            proxy_password: "secret-once",
            proxy_credential_ref: "",
            clear_proxy_credential: false,
          },
        },
      },
      { command: "workspace_get", payload: {} },
      { command: "workspace_set", payload: { payload: { path: "/Users/demo/InvestCompass2" } } },
    ]);
  });

  test("缓存、数据源、更新和日志方法通过固定 Tauri command 调用桌面能力", async () => {
    const calls: Array<{ command: string; payload?: unknown }> = [];
    mockIPC((command, payload) => {
      calls.push({ command, payload });
      switch (command) {
        case "cache_stats":
          return { code: 0, message: "ok", data: { total_bytes: 30, items: [{ target: "quote", bytes: 30, label: "行情缓存", cleanable: true }] } };
        case "cache_clean":
          return { code: 0, message: "ok", data: { cleaned_targets: ["quote"] } };
        case "providers_status":
          return { code: 0, message: "ok", data: [{ name: "Market", source: "unconfigured", available: false, last_error: "未配置" }] };
        case "check_update":
          return { code: 0, message: "ok", data: { current_version: "0.1.0", latest_version: "0.1.1", has_update: true, release_notes: "修复问题" } };
        case "export_logs":
          return { file_path: "/tmp/invest-compass.log", file_name: "invest-compass.log" };
        default:
          throw new Error(`unexpected command ${command}`);
      }
    });

    await expect(cacheStats()).resolves.toMatchObject({ total_bytes: 30 });
    await expect(cacheClean(["quote"])).resolves.toEqual({ cleaned_targets: ["quote"] });
    await expect(providersStatus()).resolves.toMatchObject({ items: [{ source: "unconfigured" }] });
    await expect(checkUpdate()).resolves.toMatchObject({ has_update: true });
    await expect(exportLogs("/tmp")).resolves.toEqual({ file_path: "/tmp/invest-compass.log", file_name: "invest-compass.log" });

    expect(calls).toEqual([
      { command: "cache_stats", payload: {} },
      { command: "cache_clean", payload: { payload: { targets: ["quote"] } } },
      { command: "providers_status", payload: {} },
      { command: "check_update", payload: {} },
      { command: "export_logs", payload: { targetDir: "/tmp" } },
    ]);
  });

  test("应用内通知方法通过固定 Tauri command 调用后端 API", async () => {
    const calls: Array<{ command: string; payload?: unknown }> = [];
    mockIPC((command, payload) => {
      calls.push({ command, payload });
      switch (command) {
        case "notifications_list":
          return {
            code: 0,
            message: "ok",
            data: {
              items: [{ id: 1, type: "task", level: "success", title: "任务完成", content: "", is_read: false, created_at: "2026-06-23T09:00:00Z" }],
              total: 1,
              limit: 20,
              offset: 0,
            },
          };
        case "notifications_unread_count":
          return { code: 0, message: "ok", data: { count: 1 } };
        case "notifications_mark_read":
        case "notifications_mark_all_read":
        case "notifications_clear_read":
          return { code: 0, message: "ok", data: { ok: true } };
        default:
          throw new Error(`unexpected command ${command}`);
      }
    });

    await expect(notificationsList({ unread_only: false, limit: 20, offset: 0 })).resolves.toMatchObject({ total: 1 });
    await expect(notificationsUnreadCount()).resolves.toEqual({ count: 1 });
    await expect(notificationsMarkRead({ ids: [1] })).resolves.toEqual({ ok: true });
    await expect(notificationsMarkAllRead()).resolves.toEqual({ ok: true });
    await expect(notificationsClearRead()).resolves.toEqual({ ok: true });

    expect(calls).toEqual([
      { command: "notifications_list", payload: { payload: { unread_only: false, limit: 20, offset: 0 } } },
      { command: "notifications_unread_count", payload: {} },
      { command: "notifications_mark_read", payload: { payload: { ids: [1] } } },
      { command: "notifications_mark_all_read", payload: {} },
      { command: "notifications_clear_read", payload: {} },
    ]);
  });

  test("搜索索引管理通过固定 Tauri command 读取状态并触发重建", async () => {
    const calls: Array<{ command: string; payload?: unknown }> = [];
    mockIPC((command, payload) => {
      calls.push({ command, payload });
      switch (command) {
        case "search_status":
          return {
            code: 0,
            message: "ok",
            data: {
              fts5_status: "available",
              search_status: "ready",
              active_stock_batch_id: "stock-ready-1",
              active_document_batch_id: "doc-ready-1",
              running_rebuild_task_id: "",
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

    await expect(searchStatus()).resolves.toMatchObject({
      fts5_status: "available",
      active_stock_batch_id: "stock-ready-1",
      active_document_batch_id: "doc-ready-1",
    });
    await expect(searchRebuild({ scope: "reports" })).resolves.toEqual({
      task_id: "task-search-1",
      scope: "reports",
      stock_batch_id: "stock-ready-1",
      document_batch_id: "doc-building-1",
    });

    expect(calls).toEqual([
      { command: "search_status", payload: {} },
      { command: "search_rebuild", payload: { payload: { scope: "reports" } } },
    ]);
  });

  test("报告历史菜单搜索通过固定 search_reports command 查询报告范围", async () => {
    const calls: Array<{ command: string; payload?: unknown }> = [];
    mockIPC((command, payload) => {
      calls.push({ command, payload });
      return {
        code: 0,
        message: "ok",
        data: [
          {
            doc_uid: "report:7",
            doc_type: "report",
            ref_id: "7",
            symbol: "CN:SH:600519",
            title: "贵州茅台 个股综合分析",
            summary: "消费复苏与估值波动",
            source: "analysis_report",
            source_time: "2026-06-22T09:00:00Z",
            score: 1,
            highlights: ["消费复苏"],
          },
        ],
      };
    });

    await expect(
      searchReports({
        keyword: "茅台",
        symbols: ["CN:SH:600519"],
        limit: 20,
        offset: 0,
        sort: "relevance",
      }),
    ).resolves.toEqual([
      {
        doc_uid: "report:7",
        doc_type: "report",
        ref_id: "7",
        symbol: "CN:SH:600519",
        title: "贵州茅台 个股综合分析",
        summary: "消费复苏与估值波动",
        source: "analysis_report",
        source_time: "2026-06-22T09:00:00Z",
        score: 1,
        highlights: ["消费复苏"],
      },
    ]);

    expect(calls).toEqual([
      {
        command: "search_reports",
        payload: {
          payload: {
            keyword: "茅台",
            symbols: ["CN:SH:600519"],
            limit: 20,
            offset: 0,
            sort: "relevance",
          },
        },
      },
    ]);
  });

  test("资讯中心菜单搜索通过固定 search_news command 查询资讯范围", async () => {
    const calls: Array<{ command: string; payload?: unknown }> = [];
    mockIPC((command, payload) => {
      calls.push({ command, payload });
      return {
        code: 0,
        message: "ok",
        data: [
          {
            doc_uid: "news:9",
            doc_type: "news",
            ref_id: "9",
            symbol: "CN:SZ:300308",
            title: "光模块订单增长",
            summary: "800G 需求拉动",
            source: "财联社",
            source_time: "2026-06-22T10:00:00Z",
            score: 1,
            highlights: ["光模块"],
          },
        ],
      };
    });

    await expect(
      searchNews({
        keyword: "光模块",
        symbols: ["CN:SZ:300308"],
        limit: 20,
        offset: 0,
        sort: "relevance",
      }),
    ).resolves.toEqual([
      {
        doc_uid: "news:9",
        doc_type: "news",
        ref_id: "9",
        symbol: "CN:SZ:300308",
        title: "光模块订单增长",
        summary: "800G 需求拉动",
        source: "财联社",
        source_time: "2026-06-22T10:00:00Z",
        score: 1,
        highlights: ["光模块"],
      },
    ]);

    expect(calls).toEqual([
      {
        command: "search_news",
        payload: {
          payload: {
            keyword: "光模块",
            symbols: ["CN:SZ:300308"],
            limit: 20,
            offset: 0,
            sort: "relevance",
          },
        },
      },
    ]);
  });

  test("自选备注菜单搜索通过固定 search_watchlist_notes command 查询备注范围", async () => {
    const calls: Array<{ command: string; payload?: unknown }> = [];
    mockIPC((command, payload) => {
      calls.push({ command, payload });
      return {
        code: 0,
        message: "ok",
        data: [
          {
            doc_uid: "watchlist_note:7",
            doc_type: "watchlist_note",
            ref_id: "7",
            symbol: "CN:SZ:300308",
            title: "中际旭创",
            summary: "北美客户订单",
            source: "watchlist_note",
            source_time: "2026-06-22T10:00:00Z",
            score: 1,
            highlights: ["光模块"],
          },
        ],
      };
    });

    await expect(
      searchWatchlistNotes({
        keyword: "北美客户",
        symbols: ["CN:SZ:300308"],
        limit: 20,
        offset: 0,
        sort: "relevance",
      }),
    ).resolves.toEqual([
      {
        doc_uid: "watchlist_note:7",
        doc_type: "watchlist_note",
        ref_id: "7",
        symbol: "CN:SZ:300308",
        title: "中际旭创",
        summary: "北美客户订单",
        source: "watchlist_note",
        source_time: "2026-06-22T10:00:00Z",
        score: 1,
        highlights: ["光模块"],
      },
    ]);

    expect(calls).toEqual([
      {
        command: "search_watchlist_notes",
        payload: {
          payload: {
            keyword: "北美客户",
            symbols: ["CN:SZ:300308"],
            limit: 20,
            offset: 0,
            sort: "relevance",
          },
        },
      },
    ]);
  });

  test("schedulerJobsList 通过固定 Tauri command 读取调度任务", async () => {
    const calls: Array<{ command: string; payload?: unknown }> = [];
    mockIPC((command, payload) => {
      calls.push({ command, payload });
      return {
        code: 0,
        message: "ok",
        data: { items: [{ id: 1, cron_type: "cn_a_share_quote_refresh" }] },
      };
    });

    await expect(schedulerJobsList()).resolves.toEqual({
      items: [{ id: 1, cron_type: "cn_a_share_quote_refresh" }],
    });
    expect(calls).toEqual([{ command: "scheduler_jobs_list", payload: {} }]);
  });

  test("schedulerJobTypes 通过固定 Tauri command 读取任务类型元数据", async () => {
    const calls: Array<{ command: string; payload?: unknown }> = [];
    mockIPC((command, payload) => {
      calls.push({ command, payload });
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              cron_type: "cn_a_share_quote_refresh",
              label: "A 股行情刷新",
            },
          ],
        },
      };
    });

    await expect(schedulerJobTypes()).resolves.toEqual({
      items: [
        {
          cron_type: "cn_a_share_quote_refresh",
          label: "A 股行情刷新",
        },
      ],
    });
    expect(calls).toEqual([{ command: "scheduler_job_types", payload: {} }]);
  });

  test("schedulerRunsTrigger 通过共享队列触发用户手动抓取", async () => {
    const calls: Array<{ command: string; payload?: unknown }> = [];
    mockIPC((command, payload) => {
      calls.push({ command, payload });
      return {
        code: 0,
        message: "ok",
        data: {
          run_key: "user_request:cn_a_share_quote_refresh:CN:SH:600519:2026-06-19:1",
          trigger_type: "user_request",
          status: "queued",
        },
      };
    });

    await expect(
      schedulerRunsTrigger({
        job_id: 0,
        cron_type: "cn_a_share_quote_refresh",
        scope_key: "CN:SH:600519",
        target_date: "2026-06-19",
      }),
    ).resolves.toMatchObject({
      trigger_type: "user_request",
      status: "queued",
    });
    expect(calls).toEqual([
      {
        command: "scheduler_runs_trigger",
        payload: {
          payload: {
            job_id: 0,
            cron_type: "cn_a_share_quote_refresh",
            scope_key: "CN:SH:600519",
            target_date: "2026-06-19",
          },
        },
      },
    ]);
  });

  test("schedulerRefreshSymbol 通过固定 Tauri command 刷新单股数据", async () => {
    const calls: Array<{ command: string; payload?: unknown }> = [];
    mockIPC((command, payload) => {
      calls.push({ command, payload });
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              run_key: "user_request:cn_a_share_quote_refresh:CN:SH:600519:2026-06-19:1",
              trigger_type: "user_request",
              status: "queued",
            },
          ],
        },
      };
    });

    await expect(
      schedulerRefreshSymbol({
        symbol: "CN:SH:600519",
        data_type: "quote",
        target_date: "2026-06-19",
      }),
    ).resolves.toMatchObject({
      items: [{ trigger_type: "user_request", status: "queued" }],
    });
    expect(calls).toEqual([
      {
        command: "scheduler_refresh_symbol",
        payload: {
          payload: {
            symbol: "CN:SH:600519",
            data_type: "quote",
            target_date: "2026-06-19",
          },
        },
      },
    ]);
  });

  test("scheduler 管理方法都通过固定 Tauri command 调用", async () => {
    const calls: Array<{ command: string; payload?: unknown }> = [];
    mockIPC((command, payload) => {
      calls.push({ command, payload });
      return {
        code: 0,
        message: "ok",
        data: command === "scheduler_status"
          ? { jobs_total: 1, jobs_enabled: 1, queued_runs: 0, running_runs: 0, failed_runs: 0 }
          : command === "scheduler_runs_list"
            ? { items: [] }
            : command === "scheduler_jobs_backfill"
              ? { items: [] }
              : { id: 7, cron_type: "cn_a_share_quote_refresh", run_key: "run-7", trigger_type: "user_request", status: "queued" },
      };
    });

    await schedulerJobsGet(7);
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
    await schedulerJobsSetEnabled(7, false);
    await schedulerJobsDelete(7);
    await schedulerJobsRunNow({ id: 7, target_date: "2026-06-19", ignore_trade_window: true });
    await schedulerJobsBackfill({
      id: 7,
      dateFrom: "2026-06-17",
      dateTo: "2026-06-19",
      symbols: ["CN:SH:600519"],
    });
    await schedulerRunsList({ job_id: 7, limit: 20 });
    await schedulerRunsGet(9);
    await schedulerStatus();

    expect(calls.map((call) => call.command)).toEqual([
      "scheduler_jobs_get",
      "scheduler_jobs_save",
      "scheduler_jobs_set_enabled",
      "scheduler_jobs_delete",
      "scheduler_jobs_run_now",
      "scheduler_jobs_backfill",
      "scheduler_runs_list",
      "scheduler_runs_get",
      "scheduler_status",
    ]);
    expect(calls[5]?.payload).toEqual({
      payload: {
        id: 7,
        dateFrom: "2026-06-17",
        dateTo: "2026-06-19",
        symbols: ["CN:SH:600519"],
      },
    });
    expect(calls[6]?.payload).toEqual({ jobId: 7, limit: 20 });
  });
});
