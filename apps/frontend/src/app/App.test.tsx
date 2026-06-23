/* @vitest-environment jsdom */

import { clearMocks, mockIPC } from "@tauri-apps/api/mocks";
import "@testing-library/jest-dom/vitest";
import "../test/setupDom";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import { App, AppErrorBoundary } from "./App";
import * as AppModule from "./App";
import { appDatePickerLocale } from "../lib/antdLocale";
import { useDashboardStore } from "../stores/dashboardStore";

Object.defineProperty(window, "matchMedia", {
  writable: true,
  value: vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })),
});

const originalGetComputedStyle = window.getComputedStyle.bind(window);
Object.defineProperty(window, "getComputedStyle", {
  writable: true,
  value: (element: Element) => originalGetComputedStyle(element),
});

const dialogOpenMock = vi.hoisted(() => vi.fn());
const notificationIsPermissionGrantedMock = vi.hoisted(() => vi.fn());
const notificationRequestPermissionMock = vi.hoisted(() => vi.fn());
const notificationSendMock = vi.hoisted(() => vi.fn());

vi.mock("@tauri-apps/plugin-dialog", () => ({
  open: dialogOpenMock,
}));

vi.mock("@tauri-apps/plugin-notification", () => ({
  isPermissionGranted: notificationIsPermissionGrantedMock,
  requestPermission: notificationRequestPermissionMock,
  sendNotification: notificationSendMock,
}));

afterEach(() => {
  clearMocks();
  useDashboardStore.setState({ state: null, loading: false, error: null, lastLoadedAt: null });
  dialogOpenMock.mockReset();
  notificationIsPermissionGrantedMock.mockReset();
  notificationRequestPermissionMock.mockReset();
  notificationSendMock.mockReset();
  cleanup();
  window.location.hash = "";
  vi.useRealTimers();
});

const dashboardFixture = {
  watchlist: { up_count: 2, down_count: 1, flat_count: 0 },
  recent_reports: [
    { id: 1, title: "浦发银行分析", symbol: "600000.SH", risk_summary: "注意估值波动" },
  ],
  recent_tasks: [{ id: "task-1", title: "生成分析报告", status: "SUCCESS", progress: 100 }],
  market_news: [{ title: "A 股早盘新闻", source: "eastmoney", published_at: "2026-06-19T09:30:00Z" }],
  risk_tips: ["仅作研究辅助，不构成投资建议"],
  provider_statuses: [{ name: "market", source: "sina", available: true, last_error: "" }],
};

const emptyDashboardFixture = {
  watchlist: { up_count: 0, down_count: 0, flat_count: 0 },
  recent_reports: [],
  recent_tasks: [],
  market_news: [],
  risk_tips: ["仅作研究辅助，不构成投资建议"],
  provider_statuses: [{ name: "market", source: "unconfigured", available: false, last_error: "provider_unconfigured" }],
};

const basicSettingsGetPayload = {
  keys: ["app.theme", "app.language", "market.default", "quote.refresh_interval", "kline.default_period", "kline.default_adjust"],
};

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

test("首版主导航和路由范围只包含 MVP 页面", () => {
  const routeModule = AppModule as typeof AppModule & {
    APP_NAV_ITEMS?: unknown;
    APP_ROUTE_PATHS?: unknown;
  };

  expect(routeModule.APP_NAV_ITEMS).toEqual([
    { path: "/", label: "总览" },
    { path: "/watchlist", label: "自选股" },
    { path: "/analysis", label: "AI 分析" },
    { path: "/reports", label: "报告历史" },
    { path: "/news", label: "资讯中心" },
    { path: "/tasks", label: "任务历史" },
    { path: "/settings", label: "设置" },
  ]);
  expect(routeModule.APP_ROUTE_PATHS).toEqual([
    "/",
    "/watchlist",
    "/stocks/:symbol",
    "/news",
    "/scheduler",
    "/analysis",
    "/analysis/running",
    "/reports",
    "/reports/:reportId",
    "/tasks",
    "/settings",
    "/ai-settings",
  ]);
});

test("App 默认不伪装 core health，刷新后通过 typed invoke service 展示状态", async () => {
  mockIPC((command) => {
    if (command === "dashboard_summary") {
      return { code: 0, message: "ok", data: dashboardFixture };
    }
    if (command === "watchlist_list") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "market_quote") {
      return { code: 0, message: "ok", data: { symbol: "000001.SH", price: 3181.3, change: -12.18, change_percent: -0.38 } };
    }
    expect(command).toBe("core_health");
    return {
      code: 0,
      message: "ok",
      data: {
        status: "ok",
        version: "0.1.0",
      },
    };
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByText("自选股涨跌分布")).toBeInTheDocument();
  });
  expect(screen.queryByText(/本地核心服务已连接/)).not.toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: /刷新/ }));

  await waitFor(() => {
    expect(screen.getByText(/本地核心服务已连接/)).toBeInTheDocument();
  });
  expect(screen.getByText(/版本\s*0\.1\.0/)).toBeInTheDocument();
});

test("App 提供基础路由外壳和错误边界", async () => {
  mockIPC((command) => {
    if (command === "dashboard_summary") {
      return { code: 0, message: "ok", data: dashboardFixture };
    }
    if (command === "watchlist_list") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "market_quote") {
      return { code: 0, message: "ok", data: { symbol: "000001.SH", price: 3181.3, change: -12.18, change_percent: -0.38 } };
    }
    return {
      code: 0,
      message: "ok",
      data: {
        status: "ok",
        version: "0.1.0",
      },
    };
  });

  render(<App />);

  expect(screen.getByRole("navigation", { name: "主导航" })).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "总览" })).toHaveAttribute("href", "#/");
  expect(screen.getByRole("link", { name: "自选股" })).toHaveAttribute("href", "#/watchlist");
  expect(screen.getByRole("link", { name: "AI 分析" })).toHaveAttribute("href", "#/analysis");
  expect(screen.getByRole("link", { name: "资讯中心" })).toHaveAttribute("href", "#/news");

  expect(screen.queryByText(/本地核心服务已连接/)).not.toBeInTheDocument();
});

test("Dashboard 首页展示真实数据空态且不拉取伪数据", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByText("自选股涨跌分布")).toBeInTheDocument();
  });
  expect(screen.getByPlaceholderText("搜索股票名称 / 代码 / 拼音")).toBeInTheDocument();
  expect(screen.getByText("今日热点")).toBeInTheDocument();
  expect(screen.getByText("最近分析报告")).toBeInTheDocument();
  expect(screen.getByText("最近任务状态")).toBeInTheDocument();
  expect(screen.getByText("上涨")).toBeInTheDocument();
  expect(screen.getByText("下跌")).toBeInTheDocument();
  expect(screen.getByText("平盘")).toBeInTheDocument();
  expect(screen.queryByText("暂无市场指数数据")).not.toBeInTheDocument();
  expect(screen.getByText("暂无热点数据")).toBeInTheDocument();
  expect(screen.queryByText("宁德时代深度分析")).not.toBeInTheDocument();
  expect(screen.queryByText("A股 已收盘")).not.toBeInTheDocument();
  expect(screen.queryByText("上证指数")).not.toBeInTheDocument();
  expect(screen.queryByText("宁德时代（300750）深度分析报告")).not.toBeInTheDocument();
  expect(screen.getByText("仅供研究，不构成投资建议。")).toBeInTheDocument();
  expect(screen.getByText("AI 生成内容仅供研究参考，请结合公开披露信息独立判断。")).toBeInTheDocument();
  expect(document.querySelector(".ant-badge-count")).not.toBeInTheDocument();
  expect(calls.map((call) => call.command)).not.toContain("dashboard_summary");
  expect(calls.map((call) => call.command)).not.toContain("watchlist_list");
  expect(calls.map((call) => call.command)).not.toContain("market_kline");
});

test("Dashboard 首页默认不展示伪造市场状态时间", async () => {
  mockIPC((command) => {
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByText("自选股涨跌分布")).toBeInTheDocument();
  });
  expect(screen.queryByText("A股 已收盘")).not.toBeInTheDocument();
  expect(screen.queryByText((content) => content.includes("2025") && content.includes("15:30:00"))).not.toBeInTheDocument();
});

test("顶部搜索通过后端股票搜索跳转到首个真实结果", async () => {
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
      case "dashboard_summary":
        return { code: 0, message: "ok", data: emptyDashboardFixture };
      case "watchlist_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "market_quote": {
        const args = payload as { symbol?: string };
        return { code: 0, message: "ok", data: { symbol: args.symbol, price: 0, change_percent: 0 } };
      }
      case "stock_search":
        return {
          code: 0,
          message: "ok",
          data: [{ symbol: "600000.SH", name: "浦发银行", code: "600000", market: "CN", exchange: "SH" }],
        };
      case "market_kline":
        return { code: 0, message: "ok", data: { items: [] } };
      case "market_indicators":
        return { code: 0, message: "ok", data: { symbol: "600000.SH", period: "day", adjust: "qfq", indicators: {} } };
      case "news_list":
        return { code: 0, message: "ok", data: { items: [] } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  const search = await screen.findByPlaceholderText("搜索股票名称 / 代码 / 拼音");
  fireEvent.change(search, { target: { value: "浦发银行" } });
  fireEvent.keyDown(search, { key: "Enter", code: "Enter" });

  await waitFor(() => {
    expect(window.location.hash).toBe("#/stocks/600000.SH");
  });
  expect(calls).toContainEqual({ command: "stock_search", payload: { keyword: "浦发银行" } });
  expect(calls.map((call) => call.command)).not.toContain("search_reports");
  expect(calls.map((call) => call.command)).not.toContain("search_news");
  expect(calls.map((call) => call.command)).not.toContain("search_watchlist_notes");
  expect(calls.map((call) => call.command)).not.toContain("search_global");
  expect(screen.queryByText("查看全部搜索结果")).not.toBeInTheDocument();
  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "未选择股票" })).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: /返回/ }));
  await waitFor(() => {
    expect(window.location.hash).toBe("#/");
  });
});

test("Dashboard 首页不展示旧版 Provider 异常空态", async () => {
  mockIPC((command) => {
    if (command === "dashboard_summary") {
      return { code: 0, message: "ok", data: emptyDashboardFixture };
    }
    if (command === "watchlist_list") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "market_quote") {
      return { code: 0, message: "ok", data: { symbol: "000001.SH", price: 0, change: 0, change_percent: 0 } };
    }
    return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByText("自选股涨跌分布")).toBeInTheDocument();
  });
  expect(screen.getByText("最近分析报告")).toBeInTheDocument();
  expect(screen.getByText("最近任务状态")).toBeInTheDocument();
  expect(screen.queryByText("provider_unconfigured")).not.toBeInTheDocument();
});

test("自选股页面默认展示空态并支持备注范围搜索", async () => {
  window.location.hash = "#/watchlist";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "search_watchlist_notes") {
      return {
        code: 0,
        message: "ok",
        data: [
          {
            doc_uid: "watchlist_note:1",
            doc_type: "watchlist_note",
            ref_id: "1",
            symbol: "CN:SZ:300308",
            title: "中际旭创",
            summary: "北美客户订单",
            source: "watchlist_note",
            source_time: "2026-06-22T10:22:00Z",
            score: 1,
            highlights: ["光模块"],
          },
        ],
      };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "自选股" })).toBeInTheDocument();
  });
  expect(screen.getByRole("radio", { name: /卡片视图/ })).toBeChecked();
  expect(screen.getByText("暂无匹配自选股")).toBeInTheDocument();
  expect(screen.queryByText("贵州茅台")).not.toBeInTheDocument();
  expect(screen.queryByText("宁德时代")).not.toBeInTheDocument();
  expect(screen.getByText("自选概览")).toBeInTheDocument();
  expect(screen.getByText("市场分布")).toBeInTheDocument();
  expect(screen.queryByText("买入")).not.toBeInTheDocument();
  expect(screen.queryByText("卖出")).not.toBeInTheDocument();

  const searchInput = screen.getByPlaceholderText("搜索自选股");
  fireEvent.change(searchInput, { target: { value: "北美客户" } });
  fireEvent.keyDown(searchInput, { key: "Enter", code: "Enter" });
  await waitFor(() => {
    expect(screen.getByText("中际旭创")).toBeInTheDocument();
  });
  expect(screen.getByText("北美客户订单")).toBeInTheDocument();
  expect(calls).toEqual([
    {
      command: "search_watchlist_notes",
      payload: { payload: { keyword: "北美客户", symbols: [], limit: 20, offset: 0, sort: "relevance" } },
    },
  ]);
  expect(calls.map((call) => call.command)).not.toContain("search_global");
  expect(calls.map((call) => call.command)).not.toContain("search_news");
});

test("自选股页面可切换表格视图且不暴露交易入口", async () => {
  window.location.hash = "#/watchlist";
  mockIPC((command) => {
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "自选股" })).toBeInTheDocument();
  });
  expect(screen.getByRole("radio", { name: /卡片视图/ })).toBeChecked();
  fireEvent.click(screen.getByRole("radio", { name: /表格视图/ }));
  expect(screen.getByRole("radio", { name: /表格视图/ })).toBeChecked();
  expect(screen.getByRole("columnheader", { name: "股票名称" })).toBeInTheDocument();
  expect(screen.queryByText("券商账户")).not.toBeInTheDocument();
  expect(screen.queryByText("下单")).not.toBeInTheDocument();
});

test("个股详情页展示空态工作台且不作为左侧菜单入口", async () => {
  window.location.hash = "#/stocks/CN%3ASH%3A600183";
  mockIPC((command) => {
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "未选择股票" })).toBeInTheDocument();
  });
  expect(screen.queryByRole("link", { name: /个股详情/ })).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: /返回/ })).toBeInTheDocument();
  expect(screen.getAllByText("暂无").length).toBeGreaterThan(0);
  expect(screen.getByRole("heading", { name: "K线图" })).toBeInTheDocument();
  expect(screen.getByText("暂无K线数据")).toBeInTheDocument();
  expect(screen.getByText("暂无基础信息")).toBeInTheDocument();
  expect(screen.getByText("暂无技术指标")).toBeInTheDocument();
  expect(screen.getByText("暂无新闻资讯")).toBeInTheDocument();
  expect(screen.queryByText("生益科技：一季度归母净利润同比增长18.35% 产品结构持续优化")).not.toBeInTheDocument();
  expect(screen.queryByText("买入")).not.toBeInTheDocument();
  expect(screen.queryByText("卖出")).not.toBeInTheDocument();
  expect(screen.queryByText("下单")).not.toBeInTheDocument();
  expect(screen.queryByText("券商账户")).not.toBeInTheDocument();
});

test("个股详情页本地交互只展示占位提示状态", async () => {
  window.location.hash = "#/stocks/CN%3ASH%3A600183";
  mockIPC((command) => {
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "未选择股票" })).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: "周K" }));
  expect(screen.getByRole("button", { name: "周K" })).toHaveClass("text-[#1677ff]");
  fireEvent.click(screen.getByText("MACD"));
  expect(screen.getByText("MACD")).toHaveClass("text-[#1677ff]");
  fireEvent.click(screen.getByText("AI分析摘要"));
  expect(screen.getAllByText("内容待接入").length).toBeGreaterThan(0);
});

test("模型配置页保存 API Key 后只展示脱敏字段并清空明文输入", async () => {
  window.location.hash = "#/ai-settings";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    const args = payload as any;
    calls.push({ command, payload });
    switch (command) {
      case "ai_config_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "prompt_templates_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "ai_config_save":
        return {
          code: 0,
          message: "ok",
          data: {
            config: {
              ...args.payload,
              id: 3,
              api_key_ref: "local-vault://ai-config/custom-3",
              masked_api_key: "sk-...cret",
              has_api_key: true,
              api_key: undefined,
            },
          },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "模型配置" })).toBeInTheDocument();
  });
  fireEvent.change(screen.getByLabelText("配置名称"), { target: { value: "自定义接入点" } });
  fireEvent.change(screen.getByLabelText("接入点"), { target: { value: "https://llm.example.com" } });
  fireEvent.change(screen.getByLabelText("模型名称"), { target: { value: "gpt-4.1-mini" } });
  fireEvent.change(screen.getByLabelText("API Key"), { target: { value: "sk-live-secret" } });
  fireEvent.click(screen.getByRole("button", { name: "保存模型配置" }));

  await waitFor(() => {
    expect(screen.getByText("sk-...cret")).toBeInTheDocument();
  });
  expect(screen.queryByText("sk-live-secret")).not.toBeInTheDocument();
  expect(screen.queryByDisplayValue("sk-live-secret")).not.toBeInTheDocument();
  expect(calls.some((call) => call.command === "ai_config_save" && call.payload?.payload?.api_key === "sk-live-secret")).toBe(true);
});

test("模型连通性测试失败时不把密钥文本展示到页面", async () => {
  window.location.hash = "#/ai-settings";
  mockIPC((command) => {
    switch (command) {
      case "ai_config_list":
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
      case "prompt_templates_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "ai_config_test":
        return { code: 50201, message: "upstream failed sk-live-secret", data: null };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByText("OpenAI 主配置")).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: "测试 OpenAI 主配置" }));

  await waitFor(() => {
    expect(screen.getByText(/upstream failed/)).toBeInTheDocument();
  });
  expect(screen.queryByText("sk-live-secret")).not.toBeInTheDocument();
});

test("Prompt 模板页拒绝未支持变量且不会提交创建 command", async () => {
  window.location.hash = "#/ai-settings";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "ai_config_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "prompt_templates_list":
        return { code: 0, message: "ok", data: { items: [] } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "Prompt 模板" })).toBeInTheDocument();
  });
  fireEvent.change(screen.getByLabelText("模板名称"), { target: { value: "非法变量模板" } });
  fireEvent.change(screen.getByLabelText("模板内容"), { target: { value: "分析 {{unsupported_var}}" } });
  fireEvent.click(screen.getByRole("button", { name: "保存 Prompt 模板" }));

  await waitFor(() => {
    expect(screen.getByText("变量 unsupported_var 不在首版白名单")).toBeInTheDocument();
  });
  expect(calls.some((call) => call.command === "prompt_templates_create")).toBe(false);
});

test("AI 分析页展示空态工作台并只使用本地交互", async () => {
  window.location.hash = "#/analysis";
  const calls: Array<{ command: string; payload?: any }> = [];
  const writeText = vi.fn<(text: string) => Promise<void>>(() => Promise.resolve());
  Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "AI 分析" })).toBeInTheDocument();
  });
  expect(screen.getByText("基于行情、K线、新闻与技术指标生成研究报告")).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "AI 分析" })).toHaveAttribute("aria-current", "page");
  expect(screen.getByRole("button", { name: /返回自选/ })).toBeInTheDocument();

  expect(screen.getByText("分析配置")).toBeInTheDocument();
  expect(screen.getByText("股票选择")).toBeInTheDocument();
  expect(screen.queryByDisplayValue(/生益科技/)).not.toBeInTheDocument();
  expect(screen.getByText("个股综合分析")).toBeInTheDocument();
  expect(screen.getByText("DeepSeek (DeepSeek-V3)")).toBeInTheDocument();
  expect(screen.getByText("管理模板")).toBeInTheDocument();

  expect(screen.getByText("可选持仓上下文")).toBeInTheDocument();
  expect(screen.getByText("成本价（元）")).toBeInTheDocument();
  expect(screen.getByText("股数（股）")).toBeInTheDocument();
  expect(screen.getByText("风险偏好")).toBeInTheDocument();
  expect(screen.getByText("仅用于本次分析上下文，不落库")).toBeInTheDocument();

  expect(screen.getByText("数据上下文预览")).toBeInTheDocument();
  expect(screen.getByText("基础信息摘要")).toBeInTheDocument();
  expect(screen.getByText("最新行情摘要")).toBeInTheDocument();
  expect(screen.getByText("K线概况（日K）")).toBeInTheDocument();
  expect(screen.getByText("技术指标摘要")).toBeInTheDocument();
  expect(screen.getByText("相关新闻摘要")).toBeInTheDocument();
  expect(screen.queryByText("广东生益科技股份有限公司")).not.toBeInTheDocument();
  expect(screen.queryByText("生益科技：一季度归母净利润同比增长18.35%，产品结构持续优化")).not.toBeInTheDocument();

  expect(screen.getByText("输出预览")).toBeInTheDocument();
  expect(screen.getAllByText("暂无输出内容").length).toBeGreaterThan(0);
  fireEvent.mouseDown(screen.getByText("Markdown"));
  fireEvent.click(screen.getAllByRole("option", { name: "纯文本" })[0]);
  expect(screen.getAllByText("暂无输出内容").length).toBeGreaterThan(0);

  const stopButton = screen.getByRole("button", { name: /停止生成/ });
  expect(stopButton).toBeDisabled();
  fireEvent.click(screen.getByRole("button", { name: /保存报告/ }));
  fireEvent.click(screen.getByRole("button", { name: /复制 Markdown/ }));
  expect(writeText).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: /导出 Markdown/ }));
  fireEvent.click(screen.getByText("管理模板"));
  fireEvent.click(screen.getByText("查看更多新闻 >"));
  fireEvent.click(screen.getByLabelText("全屏预览"));

  expect(screen.getByText("AI 输出需区分事实、推断和观点，仅供研究参考。")).toBeInTheDocument();
  expect(screen.getAllByText("仅供研究，不构成投资建议。").length).toBeGreaterThan(0);

  fireEvent.click(screen.getByRole("button", { name: /开始分析/ }));
  await waitFor(() => {
    expect(window.location.hash).toBe("#/analysis/running");
    expect(screen.getByRole("heading", { name: "AI 分析任务" })).toBeInTheDocument();
  });

  expect(screen.queryByText("下单")).not.toBeInTheDocument();
  expect(screen.queryByText("券商账户")).not.toBeInTheDocument();
  expect(screen.queryByText("自动交易")).not.toBeInTheDocument();
  expect(calls).toEqual([]);
});

test("AI 分析生成中页面展示空任务态并只使用本地交互", async () => {
  window.location.hash = "#/analysis/running";
  const calls: Array<{ command: string; payload?: any }> = [];
  const writeText = vi.fn<(text: string) => Promise<void>>(() => Promise.resolve());
  Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "AI 分析任务" })).toBeInTheDocument();
  });
  expect(screen.getByRole("link", { name: "AI 分析" })).toHaveAttribute("aria-current", "page");
  expect(screen.getByText("AI 分析任务进行中，请稍候...")).toBeInTheDocument();
  expect(screen.getByText("CANCELLED")).toBeInTheDocument();
  expect(screen.getAllByText("暂无").length).toBeGreaterThan(0);
  expect(screen.getByRole("button", { name: /返回/ })).toBeInTheDocument();

  expect(screen.getByText("任务步骤")).toBeInTheDocument();
  expect(screen.queryByText("校验股票代码")).not.toBeInTheDocument();

  expect(screen.getByText("流式输出")).toBeInTheDocument();
  expect(screen.getByText("自动滚动")).toBeInTheDocument();
  expect(screen.getByText("暂无流式输出")).toBeInTheDocument();

  expect(screen.getByText("任务日志")).toBeInTheDocument();
  expect(screen.queryByText("TASK_STARTED")).not.toBeInTheDocument();

  const autoScrollSwitch = screen.getByRole("switch");
  expect(autoScrollSwitch).toBeChecked();
  fireEvent.click(autoScrollSwitch);
  expect(autoScrollSwitch).not.toBeChecked();
  fireEvent.click(screen.getByRole("button", { name: /^清\s*空$/ }));
  fireEvent.click(screen.getByRole("button", { name: "清空日志" }));
  fireEvent.click(screen.getByRole("button", { name: /返回/ }));

  const stopButton = screen.getByRole("button", { name: /停止生成/ });
  expect(stopButton).toBeDisabled();
  fireEvent.click(stopButton);
  expect(stopButton).toBeDisabled();
  fireEvent.click(screen.getByRole("button", { name: /后台运行/ }));
  fireEvent.click(screen.getByRole("button", { name: /复制当前内容/ }));
  expect(writeText).not.toHaveBeenCalled();

  expect(screen.getByText("任务完成后可在报告历史中查看完整内容。")).toBeInTheDocument();
  expect(screen.getByText("AI 输出需区分事实、推断和观点，仅供研究参考，不构成投资建议。")).toBeInTheDocument();
  expect(screen.getAllByText("仅供研究，不构成投资建议。").length).toBeGreaterThan(0);
  expect(screen.queryByText("下单")).not.toBeInTheDocument();
  expect(screen.queryByText("券商账户")).not.toBeInTheDocument();
  expect(screen.queryByText("自动交易")).not.toBeInTheDocument();
  expect(calls).toEqual([]);
});

test("分析报告历史页面展示空态并按报告范围搜索", async () => {
  window.location.hash = "#/reports";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "search_reports") {
      return {
        code: 0,
        message: "ok",
        data: [
          {
            doc_uid: "report:2",
            doc_type: "report",
            ref_id: "2",
            symbol: "CN:SZ:002409",
            title: "雅克科技 技术面分析",
            summary: "趋势反转失败；量能不足；关键位失守",
            source: "analysis_report",
            source_time: "2025-05-20T14:42:00Z",
            score: 1,
            highlights: ["雅克科技"],
          },
        ],
      };
    }
    if (command === "report_delete") {
      const deletePayload = payload as { id: number };
      return {
        code: 0,
        message: "ok",
        data: { id: deletePayload.id },
      };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "分析报告历史" })).toBeInTheDocument();
  });

  expect(screen.getByText("报告列表")).toBeInTheDocument();
  expect(screen.queryByText("生益科技 个股综合分析")).not.toBeInTheDocument();
  expect(screen.queryByText("雅克科技 技术面分析")).not.toBeInTheDocument();
  expect(screen.getByText("报告统计")).toBeInTheDocument();
  expect(screen.getByText("常用模型 TOP 5")).toBeInTheDocument();
  expect(screen.getByText("分析类型分布")).toBeInTheDocument();
  expect(screen.queryByText("下单")).not.toBeInTheDocument();
  expect(screen.queryByText("券商账户")).not.toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: /新建分析/ }));
  await waitFor(() => {
    expect(window.location.hash).toBe("#/analysis");
  });
  window.location.hash = "#/reports";
  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "分析报告历史" })).toBeInTheDocument();
  });

  fireEvent.change(screen.getByPlaceholderText("输入股票名称 / 代码 / 拼音"), { target: { value: "雅克" } });
  fireEvent.click(screen.getByLabelText("查询报告"));
  await waitFor(() => {
    expect(screen.getByText("雅克科技 技术面分析")).toBeInTheDocument();
  });
  expect(screen.getByText("趋势反转失败；量能不足；关键位失守")).toBeInTheDocument();
  expect(screen.queryByText("生益科技 个股综合分析")).not.toBeInTheDocument();
  fireEvent.click(screen.getByLabelText("删除 雅克科技 技术面分析"));
  await waitFor(() => {
    expect(screen.queryByText("雅克科技 技术面分析")).not.toBeInTheDocument();
  });

  fireEvent.click(screen.getByLabelText("重置筛选"));
  await waitFor(() => {
    expect(screen.queryByText("雅克科技 技术面分析")).not.toBeInTheDocument();
  });

  expect(screen.queryByRole("button", { name: "复制 生益科技 个股综合分析" })).not.toBeInTheDocument();
  expect(calls).toEqual([
    {
      command: "search_reports",
      payload: { payload: { keyword: "雅克", symbols: [], limit: 20, offset: 0, sort: "relevance" } },
    },
    { command: "report_delete", payload: { id: 2 } },
  ]);
});

test("资讯中心页面展示空态并按资讯范围搜索", async () => {
  window.location.hash = "#/news";
  const writeText = vi.fn(() => Promise.resolve());
  Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "search_news") {
      return {
        code: 0,
        message: "ok",
        data: [
          {
            doc_uid: "news:optical",
            doc_type: "news",
            ref_id: "https://news.example.com/optical",
            symbol: "CN:SZ:300308",
            title: "光模块厂商订单增长，AI 算力需求延续",
            summary: "机构认为北美 AI 算力资本开支仍处高位。",
            source: "同花顺资讯",
            source_time: "2026-06-22T10:22:00Z",
            score: 1,
            highlights: ["光模块"],
          },
        ],
      };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "资讯中心" })).toBeInTheDocument();
  });

  expect(screen.getByText("聚合个股新闻、市场新闻、行业事件与研究线索")).toBeInTheDocument();
  expect(screen.getByText("资讯列表")).toBeInTheDocument();
  expect(screen.getByText("（共 0 条）")).toBeInTheDocument();
  expect(screen.getByText("热点观察")).toBeInTheDocument();
  expect(screen.getByText("数据源状态")).toBeInTheDocument();
  expect(screen.getByText("仅搜索资讯中心，暂无匹配资讯")).toBeInTheDocument();
  expect(screen.queryByText("生益科技：公司高端覆铜板产品订单饱满，持续提升 AI 服务器领域份额")).not.toBeInTheDocument();
  expect(screen.queryByText("下单")).not.toBeInTheDocument();
  expect(screen.queryByText("券商账户")).not.toBeInTheDocument();
  expect(screen.queryByText("公告专用入口")).not.toBeInTheDocument();

  fireEvent.change(screen.getByPlaceholderText("输入关键词，支持标题/摘要"), { target: { value: "光模块" } });
  fireEvent.click(screen.getByRole("button", { name: /刷新资讯/ }));
  await waitFor(() => {
    expect(screen.getByText("光模块厂商订单增长，AI 算力需求延续")).toBeInTheDocument();
  });
  fireEvent.click(screen.getAllByRole("button", { name: /查看原文/ })[0]);
  fireEvent.click(screen.getAllByRole("button", { name: /加入上下文/ })[0]);
  fireEvent.click(screen.getAllByRole("button", { name: /复制摘要/ })[0]);
  await waitFor(() => {
    expect(writeText).toHaveBeenCalledWith(expect.stringContaining("北美 AI 算力"));
  });
  fireEvent.click(screen.getByRole("button", { name: /清理缓存/ }));

  await waitFor(() => {
    expect(calls.filter((call) => call.command === "search_news")).toEqual([
      {
        command: "search_news",
        payload: { payload: { keyword: "光模块", symbols: [], limit: 20, offset: 0, sort: "relevance" } },
      },
    ]);
  });
});

test("分析报告详情页面无报告时展示空态并可返回列表", async () => {
  window.location.hash = "#/reports/report-1";
  const writeText = vi.fn(() => Promise.resolve());
  Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "报告详情" })).toBeInTheDocument();
  });

  expect(screen.getByText("暂无报告详情，请从报告历史选择已生成的报告")).toBeInTheDocument();
  expect(screen.queryByText("核心结论")).not.toBeInTheDocument();
  expect(screen.getByText("风险声明")).toBeInTheDocument();
  expect(screen.queryByText("下单")).not.toBeInTheDocument();
  expect(screen.queryByText("券商账户")).not.toBeInTheDocument();

  expect(writeText).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: /返回列表/ }));

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "分析报告历史" })).toBeInTheDocument();
  });

  expect(calls).toEqual([]);
});

test("分析报告详情页面按路由 ID 读取真实报告", async () => {
  window.location.hash = "#/reports/3";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "report_get") {
      return {
        code: 0,
        message: "ok",
        data: {
          id: 3,
          task_id: "task-report-3",
          symbol: "CN:SH:600519",
          title: "贵州茅台 个股综合分析",
          analysis_type: "stock_full",
          model_name: "gpt-4.1-mini",
          content_markdown: "## 核心结论\n\n仅作研究辅助，不构成投资建议。",
          risk_summary: "估值波动风险",
          created_at: "2026-06-22T09:00:00Z",
          updated_at: "2026-06-22T10:00:00Z",
        },
      };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "贵州茅台 个股综合分析" })).toBeInTheDocument();
  });
  expect(screen.getAllByText(/核心结论/).length).toBeGreaterThan(0);
  expect(screen.getByText(/仅作研究辅助，不构成投资建议/)).toBeInTheDocument();
  expect(screen.getByText(/估值波动风险/)).toBeInTheDocument();
  expect(calls).toEqual([{ command: "report_get", payload: { id: 3 } }]);
});

test("任务历史页面默认展示空态且不暴露假任务", async () => {
  window.location.hash = "#/tasks";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "任务历史" })).toBeInTheDocument();
  });
  expect(screen.getByText("跟踪 AI 分析、行情刷新、资讯同步和缓存清理任务")).toBeInTheDocument();
  expect(screen.getByText("运行中任务")).toBeInTheDocument();
  expect(screen.getByText("今日成功")).toBeInTheDocument();
  expect(screen.getByText("失败任务")).toBeInTheDocument();
  expect(screen.queryByText("task_20250520_152834_abcd1234")).not.toBeInTheDocument();
  expect(screen.queryByText("生益科技 个股综合分析")).not.toBeInTheDocument();
  expect(screen.queryByText("事件流")).not.toBeInTheDocument();
  expect(calls).toEqual([]);
});

test("任务历史页面筛选和重置保持空态", async () => {
  window.location.hash = "#/tasks";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "任务历史" })).toBeInTheDocument();
  });

  fireEvent.change(screen.getByPlaceholderText("搜索任务标题 / 任务 ID / 股票代码"), {
    target: { value: "雅克科技" },
  });
  fireEvent.click(screen.getByRole("button", { name: /查\s*询/ }));
  expect(screen.queryByText("雅克科技 技术面分析")).not.toBeInTheDocument();

  expect(appDatePickerLocale.lang.shortWeekDays).toEqual(["日", "一", "二", "三", "四", "五", "六"]);
  expect(appDatePickerLocale.lang.shortMonths).toContain("6月");

  fireEvent.click(screen.getByRole("button", { name: /重\s*置/ }));
  expect(screen.queryByText("生益科技 个股综合分析")).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: /取消/ })).not.toBeInTheDocument();
  expect(calls).toEqual([]);
});

test("设置中心基础设置页展示真实空态并支持基础交互", async () => {
  window.location.hash = "#/settings";
  const calls: Array<{ command: string; payload?: any }> = [];
  const writeText = vi.fn<(text: string) => Promise<void>>(() => Promise.resolve());
  Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "core_health") {
      return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
    }
    if (command === "providers_status") {
      return { code: 0, message: "ok", data: [{ name: "market", source: "sina", available: true, last_error: "" }] };
    }
    if (command === "settings_get") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "ai_config_list") {
      return { code: 0, message: "ok", data: { items: [defaultAIConfig] } };
    }
    if (command === "cache_stats") {
      return {
        code: 0,
        message: "ok",
        data: {
          total_bytes: 1_572_864,
          items: [
            { target: "quote", bytes: 1_048_576, label: "行情缓存", cleanable: true },
            { target: "task_logs", bytes: 524_288, label: "任务日志", cleanable: true },
          ],
        },
      };
    }
    if (command === "cache_clean") {
      const cleanPayload = payload as { payload?: { targets?: string[] } } | undefined;
      return { code: 0, message: "ok", data: { cleaned_targets: cleanPayload?.payload?.targets ?? [] } };
    }
    if (command === "search_status") {
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
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByText("应用基础设置")).toBeInTheDocument();
  });
  expect(screen.getByRole("tab", { name: "基础设置" })).toHaveAttribute("aria-selected", "true");
  expect(screen.queryByRole("tab", { name: "通知设置" })).not.toBeInTheDocument();
  expect(screen.queryByRole("tab", { name: "工作区设置" })).not.toBeInTheDocument();
  expect(screen.queryByRole("tab", { name: "缓存管理" })).not.toBeInTheDocument();
  expect(screen.queryByRole("tab", { name: "开机自启" })).not.toBeInTheDocument();
  expect(screen.queryByRole("tab", { name: "检查更新" })).not.toBeInTheDocument();
  expect(screen.getByText("配置应用的基本行为与偏好设置")).toBeInTheDocument();
  expect(screen.getByText("默认 AI 模型")).toBeInTheDocument();
  expect(screen.getByText("DeepSeek / DeepSeek-V3")).toBeInTheDocument();
  expect(screen.getByText("行情刷新频率")).toBeInTheDocument();
  expect(screen.getByText("默认 K 线周期")).toBeInTheDocument();
  expect(screen.getByText("默认复权类型")).toBeInTheDocument();
  expect(screen.getByText("管理应用的工作区路径与数据存储位置")).toBeInTheDocument();
  expect(screen.getByText("配置任务与系统通知的接收方式")).toBeInTheDocument();
  expect(screen.getByText("配置桌面端行为与系统集成能力")).toBeInTheDocument();
  await waitFor(() => {
    expect(screen.getAllByText("1.5 MB")).toHaveLength(2);
  });
  expect(screen.getByText("行情缓存")).toBeInTheDocument();
  expect(screen.getByText("任务日志")).toBeInTheDocument();
  expect(screen.getByText("搜索索引")).toBeInTheDocument();
  expect(screen.getByText("simple@1")).toBeInTheDocument();
  expect(screen.getByText("stock-ready-1")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: /重建报告索引/ })).toBeInTheDocument();
  expect(screen.getByText("系统代理")).toBeInTheDocument();
  expect(screen.queryByText("其他设置")).not.toBeInTheDocument();
  expect(screen.queryByText("帮助我们改进产品（不会收集个人信息）")).not.toBeInTheDocument();
  expect(screen.getByText("敏感信息会脱敏保存；日志导出前将自动清理 API Key 与代理密码。")).toBeInTheDocument();

  const switches = screen.getAllByRole("switch");
  expect(switches).toHaveLength(4);
  expect(switches[0]).toBeChecked();
  expect(switches[1]).toBeChecked();
  expect(switches[2]).not.toBeChecked();
  expect(switches[3]).toBeChecked();

  fireEvent.click(screen.getByRole("button", { name: "选择目录" }));
  fireEvent.click(screen.getByRole("button", { name: "打开目录" }));
  fireEvent.click(screen.getByRole("button", { name: /清理缓存/ }));
  await waitFor(() => {
    expect(screen.getByText("确认清理缓存")).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: "确认清理" }));
  await waitFor(() => {
    expect(calls.filter((call) => call.command === "cache_clean")).toHaveLength(1);
  });
  fireEvent.click(screen.getByRole("button", { name: /编辑代理设置/ }));
  fireEvent.click(screen.getByRole("tab", { name: "模型设置" }));
  expect(screen.getByRole("tab", { name: "模型设置" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByText("Provider 列表")).toBeInTheDocument();
  expect(screen.getByText("模型配置列表")).toBeInTheDocument();
  expect(screen.queryByText("默认 OpenAI")).not.toBeInTheDocument();

  fireEvent.click(screen.getByRole("tab", { name: "Prompt 配置" }));
  expect(screen.getByRole("tab", { name: "Prompt 配置" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByRole("tab", { name: "关于应用" })).toBeInTheDocument();
  expect(screen.getByRole("heading", { name: "Prompt 模板" })).toBeInTheDocument();
  expect(screen.getByText("管理系统提示词与投研分析模板")).toBeInTheDocument();
  expect(screen.getByText("模板分类")).toBeInTheDocument();
  expect(screen.queryByText("默认个股分析模板")).not.toBeInTheDocument();
  expect(screen.getByText("变量说明")).toBeInTheDocument();
  expect(screen.getByText("{{stock_name}}")).toBeInTheDocument();
  expect(screen.getByText("{{analysis_language}}")).toBeInTheDocument();
  expect(screen.getByText("输出预览")).toBeInTheDocument();
  expect(screen.getAllByText("暂无预览内容").length).toBeGreaterThan(0);
  fireEvent.click(screen.getByRole("button", { name: "新增分类" }));
  let dialog = screen.getByText("新建分类").closest(".ant-modal") as HTMLElement;
  expect(dialog).toBeTruthy();
  fireEvent.change(within(dialog).getByPlaceholderText("请输入分类名称"), { target: { value: "策略模板" } });
  fireEvent.mouseDown(within(dialog).getByText("文件夹"));
  fireEvent.click(screen.getByRole("option", { name: "file" }));
  fireEvent.click(within(dialog).getByRole("button", { name: "创建分类" }));
  await waitFor(() => {
    expect(screen.getByText("策略模板")).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: /新建模板/ }));
  dialog = screen
    .getAllByText("新建模板")
    .find((element) => element.classList.contains("ant-modal-title"))
    ?.closest(".ant-modal") as HTMLElement;
  expect(dialog).toBeTruthy();
  fireEvent.change(within(dialog).getByPlaceholderText("请输入模板名称"), { target: { value: "技术突破模板" } });
  fireEvent.click(within(dialog).getByRole("button", { name: "创建模板" }));
  await waitFor(() => {
    expect(screen.getByText("技术突破模板")).toBeInTheDocument();
    expect(screen.getAllByDisplayValue("技术突破模板").length).toBeGreaterThan(0);
  });
  fireEvent.change(screen.getByLabelText("Prompt 内容"), { target: { value: "# 测试模板\n{{stock_code}}" } });
  fireEvent.click(screen.getByRole("button", { name: /保存/ }));
  fireEvent.click(screen.getByRole("button", { name: /复制/ }));
  await waitFor(() => {
    expect(writeText).toHaveBeenCalledWith("# 测试模板\n{{stock_code}}");
  });
  fireEvent.click(screen.getByRole("button", { name: "格式化" }));
  fireEvent.click(screen.getByRole("button", { name: "全屏编辑" }));
  fireEvent.mouseDown(screen.getByText("Markdown"));
  fireEvent.click(screen.getAllByRole("option", { name: "纯文本" })[0]);
  expect(screen.queryByText("输入激活码")).not.toBeInTheDocument();
  expect(screen.queryByText("下单")).not.toBeInTheDocument();
  expect(screen.queryByText("券商账户")).not.toBeInTheDocument();

  fireEvent.click(screen.getByRole("tab", { name: "数据源设置" }));
  expect(screen.getByRole("tab", { name: "数据源设置" })).toHaveAttribute("aria-selected", "true");
  fireEvent.click(screen.getByRole("tab", { name: "数据源概览" }));
  expect(screen.getByRole("tab", { name: "数据源概览" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByText("数据源基础设置")).toBeInTheDocument();
  expect(screen.getByText("默认行情源")).toBeInTheDocument();
  expect(screen.getByText("AkShare / EastMoney")).toBeInTheDocument();
  expect(screen.getByText("新闻同步频率")).toBeInTheDocument();
  expect(screen.getByText("启动时自动同步")).toBeInTheDocument();
  expect(screen.getByText("行情数据源")).toBeInTheDocument();
  expect(screen.getByText("资讯与新闻源")).toBeInTheDocument();
  expect(screen.getByText("同步任务策略")).toBeInTheDocument();
  expect(screen.getByText("本地缓存与快照")).toBeInTheDocument();
  expect(screen.getByText("数据源状态摘要")).toBeInTheDocument();
  expect(screen.getByText("数据合规与说明")).toBeInTheDocument();
  expect(screen.getAllByText("EastMoney").length).toBeGreaterThan(0);
  expect(screen.getByText("Alpha Vantage")).toBeInTheDocument();
  expect(screen.getByText("186.4 MB")).toBeInTheDocument();
  expect(screen.getByText("2025-05-20 15:28:41")).toBeInTheDocument();
  expect(screen.getByText("Go Core 数据适配层")).toBeInTheDocument();
  expect(screen.getByText("扩展海外源")).toBeInTheDocument();
  expect(screen.getByText("数据仅用于本地研究与分析展示")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: /测试连接/ }));
  fireEvent.click(screen.getByRole("button", { name: /编辑配置/ }));
  fireEvent.click(screen.getByRole("button", { name: /立即同步/ }));
  fireEvent.click(screen.getByRole("button", { name: /查看日志/ }));
  fireEvent.click(screen.getByRole("button", { name: /查看调度配置/ }));
  fireEvent.click(screen.getByRole("button", { name: /清理缓存/ }));
  fireEvent.click(screen.getByRole("button", { name: /重新检测/ }));
  fireEvent.click(screen.getByRole("button", { name: /查看数据说明/ }));

  fireEvent.click(screen.getByRole("tab", { name: "代理设置" }));
  expect(screen.getByRole("tab", { name: "代理设置" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByText("代理模式")).toBeInTheDocument();
  expect(screen.getByText("系统代理（推荐）")).toBeInTheDocument();
  expect(screen.getByText("当前使用")).toBeInTheDocument();
  expect(screen.getByText("HTTP 代理")).toBeInTheDocument();
  expect(screen.getByText("SOCKS5 代理")).toBeInTheDocument();
  expect(screen.getByText("代理配置")).toBeInTheDocument();
  expect(screen.getByText("当前使用系统代理设置，无需手动配置")).toBeInTheDocument();
  expect(screen.getByText(/系统代理信息由操作系统管理/)).toBeInTheDocument();
  expect(screen.getByText("代理来源")).toBeInTheDocument();
  expect(screen.getByText("操作系统")).toBeInTheDocument();
  expect(screen.getByText("PAC 模式")).toBeInTheDocument();
  expect(screen.getByText("自动检测")).toBeInTheDocument();
  expect(screen.getByText("最后检查时间")).toBeInTheDocument();
  expect(screen.getAllByText("2025-05-20 15:30:00").length).toBeGreaterThan(0);
  expect(screen.getByText("连接测试")).toBeInTheDocument();
  expect(screen.getByText("测试目标")).toBeInTheDocument();
  expect(screen.getByText("响应时间：128 ms")).toBeInTheDocument();
  expect(screen.getByText("绕过代理设置（可选）")).toBeInTheDocument();
  expect(screen.getByPlaceholderText("例如：localhost;127.0.0.1;*.local")).toBeInTheDocument();
  expect(screen.getByText("代理配置仅影响应用访问外部网络的行为，不会修改系统或其他应用的网络设置。")).toBeInTheDocument();
  fireEvent.click(screen.getByText("HTTP 代理"));
  expect(screen.getByText("以下设置仅对当前应用生效，不会修改系统代理设置")).toBeInTheDocument();
  expect(screen.getByText("代理地址")).toBeInTheDocument();
  expect(screen.getByDisplayValue("127.0.0.1")).toBeInTheDocument();
  expect(screen.getByText("端口")).toBeInTheDocument();
  expect(screen.getByDisplayValue("7890")).toBeInTheDocument();
  expect(screen.getByText("协议类型")).toBeInTheDocument();
  expect(screen.getByText("HTTP / HTTPS")).toBeInTheDocument();
  expect(screen.getByText("身份认证")).toBeInTheDocument();
  expect(screen.getByText("用户名")).toBeInTheDocument();
  expect(screen.queryByDisplayValue("invest_user")).not.toBeInTheDocument();
  expect(screen.getByText("密码")).toBeInTheDocument();
  expect(screen.getByText("连接超时")).toBeInTheDocument();
  expect(screen.getByDisplayValue("10")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: /保存代理配置/ }));
  fireEvent.click(screen.getByRole("button", { name: /清空配置/ }));
  fireEvent.click(screen.getByText("SOCKS5 代理"));
  expect(screen.getByText("配置 SOCKS5 代理服务器连接参数")).toBeInTheDocument();
  expect(screen.getByDisplayValue("1080")).toBeInTheDocument();
  expect(screen.getByText("SOCKS 版本")).toBeInTheDocument();
  expect(screen.getAllByText("SOCKS5").length).toBeGreaterThan(0);
  expect(screen.getByText("身份认证")).toBeInTheDocument();
  expect(screen.queryByDisplayValue("invest_user")).not.toBeInTheDocument();
  expect(screen.getByText("密码")).toBeInTheDocument();
  expect(screen.getByText("连接超时")).toBeInTheDocument();
  expect(screen.getByDisplayValue("10")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: /保存代理配置/ }));
  fireEvent.click(screen.getByRole("button", { name: /清空配置/ }));
  fireEvent.click(screen.getByText("系统代理（推荐）"));
  fireEvent.click(screen.getByRole("button", { name: /刷新代理状态/ }));
  fireEvent.click(screen.getByRole("button", { name: /测试连接/ }));
  fireEvent.change(screen.getByPlaceholderText("例如：localhost;127.0.0.1;*.local"), { target: { value: "localhost;127.0.0.1;*.local" } });
  fireEvent.click(screen.getByRole("button", { name: /保存绕过规则/ }));

  fireEvent.click(screen.getByRole("tab", { name: "关于应用" }));
  expect(screen.getByRole("tab", { name: "关于应用" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByText(/投研罗盘\s+Invest Compass/)).toBeInTheDocument();
  expect(screen.getAllByText("v0.1.0").length).toBeGreaterThan(0);
  expect(screen.getByText("本地优先的 AI 投研桌面工作台")).toBeInTheDocument();
  expect(screen.getByText(/投研罗盘是一款面向个人投资者和研究者的 AI 投研助手/)).toBeInTheDocument();
  expect(screen.getByText("应用信息")).toBeInTheDocument();
  expect(screen.getByText("桌面端框架")).toBeInTheDocument();
  expect(screen.getByText("Tauri v2")).toBeInTheDocument();
  expect(screen.getByText("React + TypeScript")).toBeInTheDocument();
  expect(screen.getByText("Go Core 已连接")).toBeInTheDocument();
  expect(screen.getByText("SQLite 正常")).toBeInTheDocument();
  expect(screen.getByText("C:\\Users\\InvestCompass\\Documents\\InvestCompass")).toBeInTheDocument();
  expect(screen.getByText("C:\\Users\\InvestCompass\\AppData\\Local\\InvestCompass\\logs")).toBeInTheDocument();
  expect(screen.getAllByText("检查更新").length).toBeGreaterThan(0);
  expect(screen.getByText("当前已是最新版本")).toBeInTheDocument();
  expect(screen.getByText("2025-05-18")).toBeInTheDocument();
  expect(screen.getByText("授权状态")).toBeInTheDocument();
  expect(screen.getByText("FREE")).toBeInTheDocument();
  expect(screen.getByText("首版仅展示授权状态，不提供激活流程与功能限制。")).toBeInTheDocument();
  expect(screen.getByText("个人非商用")).toBeInTheDocument();
  expect(screen.getByText("开源许可证")).toBeInTheDocument();
  expect(screen.getByText("用户手册")).toBeInTheDocument();
  expect(screen.getByText("日志与诊断")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "检查更新" }));
  fireEvent.click(screen.getByRole("button", { name: /查看发布说明/ }));
  fireEvent.click(screen.getByRole("button", { name: /查看 LICENSE/ }));
  fireEvent.click(screen.getByRole("button", { name: /打开用户手册/ }));
  fireEvent.click(screen.getByRole("button", { name: /导出日志/ }));
  expect(screen.queryByText("输入激活码")).not.toBeInTheDocument();
  expect(screen.queryByText("购买专业版")).not.toBeInTheDocument();
  expect(screen.queryByText("升级 Pro")).not.toBeInTheDocument();
  expect(screen.queryByText("立即更新")).not.toBeInTheDocument();

  expect(calls).toEqual([
    { command: "core_health", payload: {} },
    { command: "providers_status", payload: {} },
    { command: "settings_get", payload: basicSettingsGetPayload },
    { command: "ai_config_list", payload: {} },
    { command: "workspace_get", payload: {} },
    { command: "cache_stats", payload: {} },
    { command: "search_status", payload: {} },
    { command: "autostart_get", payload: {} },
    { command: "settings_get", payload: { keys: ["window.close_to_tray"] } },
    { command: "workspace_open", payload: {} },
    { command: "cache_clean", payload: { payload: { targets: ["quote", "task_logs"] } } },
    { command: "cache_stats", payload: {} },
    { command: "ai_config_list", payload: {} },
  ]);
}, 10_000);

test("数据源设置凭据管理页展示脱敏凭据并仅使用本地交互", async () => {
  window.location.hash = "#/settings";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "core_health") {
      return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
    }
    if (command === "providers_status") {
      return { code: 0, message: "ok", data: [{ name: "market", source: "sina", available: true, last_error: "" }] };
    }
    if (command === "settings_get") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "ai_config_list") {
      return { code: 0, message: "ok", data: { items: [defaultAIConfig] } };
    }
    if (command === "cache_stats") {
      return { code: 0, message: "ok", data: { total_bytes: 0, items: [] } };
    }
    if (command === "search_status") {
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
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByText("应用基础设置")).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("tab", { name: "数据源设置" }));

  expect(screen.getByRole("tab", { name: "数据说明" })).toHaveAttribute("aria-selected", "true");
  fireEvent.click(screen.getByRole("tab", { name: "凭据管理" }));
  expect(screen.getByRole("tab", { name: "凭据管理" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByText("Provider 列表")).toBeInTheDocument();
  expect(screen.getByText("凭据配置")).toBeInTheDocument();
  expect(screen.getByText("连接测试")).toBeInTheDocument();
  expect(screen.getByText("安全与存储说明")).toBeInTheDocument();
  expect(screen.getByText("已配置凭据概览")).toBeInTheDocument();
  expect(screen.getByText("调用限制与健康状态")).toBeInTheDocument();
  expect(screen.getByText("凭据操作日志")).toBeInTheDocument();

  ["EastMoney", "AkShare", "Alpha Vantage", "财联社", "雪球", "Custom HTTP"].forEach((name) => {
    expect(screen.getAllByText(name).length).toBeGreaterThan(0);
  });
  expect(screen.getByDisplayValue("财联社")).toBeInTheDocument();
  expect(screen.getByText("快讯 / 行业事件 / 日历")).toBeInTheDocument();
  expect(screen.getByDisplayValue("https://www.cls.cn")).toBeInTheDocument();
  expect(screen.getByPlaceholderText("2025-06-30 23:59")).toBeInTheDocument();
  expect(screen.getByDisplayValue("uid=****; token=****; session=****")).toBeInTheDocument();
  expect(screen.queryByText("Authorization")).not.toBeInTheDocument();
  expect(screen.queryByText("Proxy-Authorization")).not.toBeInTheDocument();
  expect(screen.queryByText("买入")).not.toBeInTheDocument();
  expect(screen.queryByText("卖出")).not.toBeInTheDocument();
  expect(screen.queryByText("券商账户")).not.toBeInTheDocument();
  expect(screen.queryByText("授权激活")).not.toBeInTheDocument();

  fireEvent.click(screen.getByText("Alpha Vantage"));
  expect(screen.getByDisplayValue("Alpha Vantage")).toBeInTheDocument();
  expect(screen.getAllByText("海外行情").length).toBeGreaterThan(0);
  fireEvent.mouseDown(screen.getAllByRole("combobox")[0]);
  fireEvent.click(screen.getByRole("option", { name: "Cookie" }));
  expect(screen.getAllByText("Cookie").length).toBeGreaterThan(0);
  fireEvent.change(screen.getByLabelText("Base URL"), { target: { value: "https://api.example.test" } });
  expect(screen.getByDisplayValue("https://api.example.test")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "保存凭据" }));
  fireEvent.click(screen.getAllByRole("button", { name: /测试连接/ })[0]);
  fireEvent.click(screen.getByRole("button", { name: /重新测试/ }));
  fireEvent.click(screen.getByRole("button", { name: "清除凭据" }));
  expect(screen.getAllByText("确认清除 Alpha Vantage 的凭据？").length).toBeGreaterThan(0);
  fireEvent.click(screen.getByRole("button", { name: "确认清除" }));
  await waitFor(() => {
    expect(screen.getAllByText("未配置").length).toBeGreaterThan(0);
  });

  fireEvent.click(screen.getByRole("tab", { name: "数据源概览" }));
  expect(screen.getByRole("tab", { name: "数据源概览" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByText("数据源基础设置")).toBeInTheDocument();
  expect(screen.getByText("默认行情源")).toBeInTheDocument();
  expect(screen.queryByRole("tab", { name: "Provider 配置" })).not.toBeInTheDocument();
  expect(screen.queryByRole("tab", { name: "同步策略" })).not.toBeInTheDocument();

  expect(calls).toEqual([
    { command: "core_health", payload: {} },
    { command: "providers_status", payload: {} },
    { command: "settings_get", payload: basicSettingsGetPayload },
    { command: "ai_config_list", payload: {} },
    { command: "workspace_get", payload: {} },
    { command: "cache_stats", payload: {} },
    { command: "search_status", payload: {} },
    { command: "autostart_get", payload: {} },
    { command: "settings_get", payload: { keys: ["window.close_to_tray"] } },
  ]);
}, 10_000);

test("数据源设置数据说明页展示说明模块并仅使用本地交互", async () => {
  window.location.hash = "#/settings";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "core_health") {
      return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
    }
    if (command === "providers_status") {
      return { code: 0, message: "ok", data: [{ name: "market", source: "sina", available: true, last_error: "" }] };
    }
    if (command === "settings_get") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "ai_config_list") {
      return { code: 0, message: "ok", data: { items: [defaultAIConfig] } };
    }
    if (command === "cache_stats") {
      return { code: 0, message: "ok", data: { total_bytes: 0, items: [] } };
    }
    if (command === "search_status") {
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
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByText("应用基础设置")).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("tab", { name: "数据源设置" }));

  expect(screen.getByRole("tab", { name: "数据说明" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByText("数据使用与来源说明")).toBeInTheDocument();
  expect(screen.getByText("说明行情、资讯、缓存与 AI 上下文使用边界")).toBeInTheDocument();
  expect(screen.getByText("适用范围：")).toBeInTheDocument();
  expect(screen.getByText("总览 / 自选股 / 个股详情 / 资讯中心 / AI 分析")).toBeInTheDocument();
  expect(screen.getByText("默认市场：")).toBeInTheDocument();
  expect(screen.getByText("A股")).toBeInTheDocument();
  expect(screen.getByText("行情来源：")).toBeInTheDocument();
  expect(screen.getAllByText("AkShare / EastMoney").length).toBeGreaterThan(0);
  expect(screen.getByText("资讯来源：")).toBeInTheDocument();
  expect(screen.getByText("聚合新闻源 / 个股相关新闻")).toBeInTheDocument();
  expect(screen.getByText("K线数据范围：")).toBeInTheDocument();
  expect(screen.getAllByText("近 5 年").length).toBeGreaterThan(0);
  expect(screen.getByText("数据用途：")).toBeInTheDocument();
  expect(screen.getByText("本地研究展示 / 上下文构建 / 历史快照")).toBeInTheDocument();
  expect(screen.getByText("明确说明：")).toBeInTheDocument();
  expect(screen.getByText("不用于交易执行")).toBeInTheDocument();

  ["A. 数据来源说明", "B. 更新时效说明", "C. AI 上下文说明", "D. 数据合规与边界", "E. 字段说明", "F. 常见问题"].forEach((title) => {
    expect(screen.getByText(title)).toBeInTheDocument();
  });
  expect(screen.getAllByText("行情数据").length).toBeGreaterThan(0);
  expect(screen.getByText("实时行情与分时/盘口数据")).toBeInTheDocument();
  fireEvent.mouseEnter(screen.getByText("实时行情与分时/盘口数据"));
  await waitFor(() => {
    expect(screen.getByRole("tooltip")).toHaveTextContent("实时行情与分时/盘口数据");
  });
  expect(screen.getByText("扩展海外源")).toBeInTheDocument();
  expect(screen.getByText("Alpha Vantage")).toBeInTheDocument();
  expect(screen.getByText("受限")).toBeInTheDocument();
  expect(screen.getByText("行情轮询频率")).toBeInTheDocument();
  expect(screen.getByText("2 秒（盘中）/ 10 秒（非交易时段）")).toBeInTheDocument();
  expect(screen.getByText("AI 分析会基于以下数据构建上下文，以生成研究结论与解读。")).toBeInTheDocument();
  expect(screen.getByText("事实：")).toBeInTheDocument();
  expect(screen.getByText("来自原始数据或公开信息，可直接验证")).toBeInTheDocument();
  expect(screen.getByText("推断：")).toBeInTheDocument();
  expect(screen.getByText("基于数据逻辑与模型推导，存在不确定性")).toBeInTheDocument();
  expect(screen.getByText("观点：")).toBeInTheDocument();
  expect(screen.getByText("模型综合判断与建议，不构成投资建议")).toBeInTheDocument();
  expect(screen.getByText("现价")).toBeInTheDocument();
  expect(screen.getByText("最新成交价格")).toBeInTheDocument();
  expect(screen.getByText("换手率")).toBeInTheDocument();
  expect(screen.getByText("当日成交量 / 流通股本")).toBeInTheDocument();
  expect(screen.getByText("为什么不同页面时间不完全一致？")).toBeInTheDocument();
  expect(screen.getByText("为什么 AI 报告与页面最新行情略有差异？")).toBeInTheDocument();
  expect(screen.getByText("为什么部分资讯需要凭据？")).toBeInTheDocument();
  expect(screen.getByText("敏感凭据仅保存在本地安全存储；日志导出前将自动清理 API Key 与代理密码。")).toBeInTheDocument();
  expect(screen.getByText("仅供研究，不构成投资建议。")).toBeInTheDocument();

  expect(screen.queryByText("买入")).not.toBeInTheDocument();
  expect(screen.queryByText("卖出")).not.toBeInTheDocument();
  expect(screen.queryByText("下单")).not.toBeInTheDocument();
  expect(screen.queryByText("券商账户")).not.toBeInTheDocument();
  expect(screen.queryByText("自动交易")).not.toBeInTheDocument();
  expect(screen.queryByText("收益承诺")).not.toBeInTheDocument();
  expect(screen.queryByText("授权激活")).not.toBeInTheDocument();
  expect(screen.queryByText("真实 API Key")).not.toBeInTheDocument();
  expect(screen.queryByText("真实 Cookie")).not.toBeInTheDocument();
  expect(screen.queryByText("真实 Token")).not.toBeInTheDocument();
  expect(screen.queryByText("Authorization")).not.toBeInTheDocument();
  expect(screen.queryByText("Proxy-Authorization")).not.toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: "查看数据源概览" }));
  fireEvent.click(screen.getByRole("button", { name: "查看 Provider 配置" }));
  fireEvent.click(screen.getByRole("button", { name: "查看凭据管理 >" }));
  fireEvent.click(screen.getByRole("button", { name: "查看更多字段说明 >" }));
  fireEvent.click(screen.getByRole("button", { name: "为什么部分资讯需要凭据？ right" }));
  fireEvent.click(screen.getByRole("button", { name: "查看更多 FAQ >" }));
  expect(screen.queryByRole("tab", { name: "Provider 配置" })).not.toBeInTheDocument();
  expect(screen.queryByRole("tab", { name: "同步策略" })).not.toBeInTheDocument();
  expect(screen.getByRole("tab", { name: "数据说明" })).toHaveAttribute("aria-selected", "true");

  expect(calls).toEqual([
    { command: "core_health", payload: {} },
    { command: "providers_status", payload: {} },
    { command: "settings_get", payload: basicSettingsGetPayload },
    { command: "ai_config_list", payload: {} },
    { command: "workspace_get", payload: {} },
    { command: "cache_stats", payload: {} },
    { command: "search_status", payload: {} },
    { command: "autostart_get", payload: {} },
    { command: "settings_get", payload: { keys: ["window.close_to_tray"] } },
  ]);
}, 10_000);

test("任务调度页面读取真实调度接口并支持立即执行", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.setSystemTime(new Date("2026-06-19T09:00:00+08:00"));
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
      case "scheduler_status":
        return {
          code: 0,
          message: "ok",
          data: { jobs_total: 1, jobs_enabled: 1, queued_runs: 0, running_runs: 0, failed_runs: 0 },
        };
      case "scheduler_job_types":
        return {
          code: 0,
          message: "ok",
          data: { items: [{ cron_type: "cn_a_share_quote_refresh", label: "A 股行情刷新" }] },
        };
      case "scheduler_jobs_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 7,
                name: "A 股开盘行情刷新",
                cron_type: "cn_a_share_quote_refresh",
                cron_expr: "30 9 * * 1-5",
                enabled: true,
                market: "CN",
                last_status: "failed",
                last_error: "market_provider_unconfigured",
              },
            ],
          },
        };
      case "scheduler_runs_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 11,
                run_key: "run-11",
                trigger_type: "missed_today",
                status: "failed",
                target_date: "2026-06-19",
                started_at: "2026-06-19T09:30:00Z",
                finished_at: "2026-06-19T09:31:00Z",
                fetched_count: 1,
                written_count: 0,
              },
            ],
          },
        };
      case "scheduler_runs_get":
        return {
          code: 0,
          message: "ok",
          data: {
            id: 11,
            run_key: "run-11",
            trigger_type: "missed_today",
            status: "failed",
            target_date: "2026-06-19",
            started_at: "2026-06-19T09:30:00Z",
            finished_at: "2026-06-19T09:31:00Z",
            source: "startup_restore",
            scope_key: "CN:SH:600519",
            cron_type: "cn_a_share_quote_refresh",
            data_type: "quote",
            fetched_count: 1,
            written_count: 0,
            error_message: "market_provider_unconfigured",
          },
        };
      case "providers_status":
        return {
          code: 0,
          message: "ok",
          data: [
            { name: "market-provider", source: "sina-tencent-market", available: true },
            { name: "news-provider", source: "sina-live", available: true },
          ],
        };
      case "scheduler_jobs_run_now":
        return {
          code: 0,
          message: "ok",
          data: { id: 12, run_key: "run-12", trigger_type: "user_request", status: "queued" },
        };
      case "scheduler_jobs_set_enabled":
        return { code: 0, message: "ok", data: { id: 7, enabled: false } };
      case "scheduler_jobs_backfill":
        return {
          code: 0,
          message: "ok",
          data: { items: [{ id: 13, run_key: "run-13", trigger_type: "catchup_gap", status: "queued" }] },
        };
      case "scheduler_jobs_save":
        return {
          code: 0,
          message: "ok",
          data: { id: 14, name: "新建调度任务", cron_type: "cn_a_share_quote_refresh", enabled: true },
        };
      case "scheduler_refresh_symbol":
        return {
          code: 0,
          message: "ok",
          data: { items: [{ id: 15, run_key: "run-15", trigger_type: "user_request", status: "queued" }] },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);
  fireEvent.click(screen.getByRole("link", { name: "任务调度" }));

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "任务调度" })).toBeInTheDocument();
  });
  expect(screen.getAllByText("A 股开盘行情刷新").length).toBeGreaterThan(0);
  expect(screen.getByText("2026-06-19 09:30")).toBeInTheDocument();
  expect(screen.getAllByText("failed").length).toBeGreaterThan(0);
  expect(screen.getByText("market_provider_unconfigured")).toBeInTheDocument();
  expect(screen.getAllByText("missed_today").length).toBeGreaterThan(0);
  expect(screen.getByText("2026-06-19T09:30:00Z")).toBeInTheDocument();
  expect(screen.getByText("2026-06-19T09:31:00Z")).toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: "查看详情" }));

  await waitFor(() => {
    expect(calls.some((call) => call.command === "scheduler_runs_get")).toBe(true);
  });
  expect(screen.getByText("执行详情")).toBeInTheDocument();
  expect(screen.getByText("startup_restore")).toBeInTheDocument();
  expect(screen.getByText("CN:SH:600519")).toBeInTheDocument();
  expect(screen.getAllByText("2026-06-19T09:30:00Z").length).toBeGreaterThan(0);
  expect(screen.getAllByText("2026-06-19T09:31:00Z").length).toBeGreaterThan(0);

  fireEvent.click(screen.getByRole("button", { name: "立即执行" }));

  await waitFor(() => {
    expect(calls.some((call) => call.command === "scheduler_jobs_run_now")).toBe(true);
  });

  fireEvent.click(screen.getByRole("button", { name: "停用任务" }));

  await waitFor(() => {
    expect(calls.some((call) => call.command === "scheduler_jobs_set_enabled")).toBe(true);
  });

  fireEvent.change(screen.getByLabelText("名称"), { target: { value: "新建调度任务" } });
  fireEvent.click(screen.getByRole("button", { name: "创建任务" }));

  await waitFor(() => {
    expect(calls.some((call) => call.command === "scheduler_jobs_save")).toBe(true);
  });

  fireEvent.change(screen.getByLabelText("任务"), { target: { value: "7" } });
  fireEvent.change(screen.getAllByLabelText("Symbol，可选")[1], { target: { value: "600000.SH 000001.SZ" } });
  fireEvent.click(screen.getByRole("button", { name: "生成补偿" }));

  await waitFor(() => {
    expect(calls.some((call) => call.command === "scheduler_jobs_backfill")).toBe(true);
  });

  fireEvent.change(screen.getByLabelText("股票代码"), { target: { value: "600000.SH" } });
  fireEvent.click(screen.getByRole("button", { name: "刷新单股" }));

  await waitFor(() => {
    expect(calls.some((call) => call.command === "scheduler_refresh_symbol")).toBe(true);
  });
});

test("任务调度页面支持按执行状态和触发类型过滤运行记录", async () => {
  mockIPC((command) => {
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
      case "scheduler_status":
        return {
          code: 0,
          message: "ok",
          data: { jobs_total: 1, jobs_enabled: 1, queued_runs: 1, running_runs: 0, failed_runs: 1 },
        };
      case "scheduler_job_types":
        return {
          code: 0,
          message: "ok",
          data: { items: [{ cron_type: "cn_a_share_quote_refresh", label: "A 股行情刷新" }] },
        };
      case "scheduler_jobs_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 7,
                name: "A 股开盘行情刷新",
                cron_type: "cn_a_share_quote_refresh",
                cron_expr: "30 9 * * 1-5",
                enabled: true,
                market: "CN",
              },
            ],
          },
        };
      case "scheduler_runs_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              { id: 21, run_key: "run-failed", trigger_type: "missed_today", status: "failed", target_date: "2026-06-19" },
              { id: 22, run_key: "run-queued", trigger_type: "user_request", status: "queued", target_date: "2026-06-19" },
            ],
          },
        };
      case "providers_status":
        return {
          code: 0,
          message: "ok",
          data: [
            { name: "market-provider", source: "sina-tencent-market", available: true },
            { name: "news-provider", source: "sina-live", available: true },
          ],
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);
  fireEvent.click(screen.getByRole("link", { name: "任务调度" }));

  await waitFor(() => {
    expect(screen.getByText("run-failed")).toBeInTheDocument();
  });
  expect(screen.getByText("run-queued")).toBeInTheDocument();

  fireEvent.change(screen.getByLabelText("执行状态"), { target: { value: "failed" } });
  expect(screen.getByText("run-failed")).toBeInTheDocument();
  expect(screen.queryByText("run-queued")).not.toBeInTheDocument();

  fireEvent.change(screen.getByLabelText("触发类型"), { target: { value: "user_request" } });
  expect(screen.getByText("暂无执行记录")).toBeInTheDocument();
});

test("任务调度页面补偿日期超范围时不保留旧成功提示", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
      case "scheduler_status":
        return {
          code: 0,
          message: "ok",
          data: { jobs_total: 1, jobs_enabled: 1, queued_runs: 0, running_runs: 0, failed_runs: 0 },
        };
      case "scheduler_job_types":
        return {
          code: 0,
          message: "ok",
          data: { items: [{ cron_type: "cn_a_share_quote_refresh", label: "A 股行情刷新" }] },
        };
      case "scheduler_jobs_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 7,
                name: "A 股开盘行情刷新",
                cron_type: "cn_a_share_quote_refresh",
                cron_expr: "30 9 * * 1-5",
                enabled: true,
                market: "CN",
              },
            ],
          },
        };
      case "scheduler_runs_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "providers_status":
        return {
          code: 0,
          message: "ok",
          data: [
            { name: "market-provider", source: "sina-tencent-market", available: true },
            { name: "news-provider", source: "sina-live", available: true },
          ],
        };
      case "scheduler_jobs_backfill":
        return {
          code: 0,
          message: "ok",
          data: { items: [{ id: 31, run_key: "run-31", trigger_type: "catchup_gap", status: "queued" }] },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);
  fireEvent.click(screen.getByRole("link", { name: "任务调度" }));

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "任务调度" })).toBeInTheDocument();
  });
  fireEvent.change(screen.getByLabelText("任务"), { target: { value: "7" } });
  fireEvent.change(screen.getByLabelText("开始日期"), { target: { value: "2026-06-01" } });
  fireEvent.change(screen.getByLabelText("结束日期"), { target: { value: "2026-06-01" } });
  fireEvent.click(screen.getByRole("button", { name: "生成补偿" }));

  await waitFor(() => {
    expect(screen.getByText("已生成 1 条补偿执行记录")).toBeInTheDocument();
  });

  fireEvent.change(screen.getByLabelText("结束日期"), { target: { value: "2026-07-01" } });
  fireEvent.click(screen.getByRole("button", { name: "生成补偿" }));

  expect(screen.getByText("单次补偿最多覆盖 30 个自然日")).toBeInTheDocument();
  expect(screen.queryByText("已生成 1 条补偿执行记录")).not.toBeInTheDocument();
  expect(calls.filter((call) => call.command === "scheduler_jobs_backfill")).toHaveLength(1);
});

test("任务调度页面读取执行详情失败时清理旧详情并展示错误", async () => {
  let detailCalls = 0;
  mockIPC((command, payload) => {
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
      case "scheduler_status":
        return {
          code: 0,
          message: "ok",
          data: { jobs_total: 1, jobs_enabled: 1, queued_runs: 0, running_runs: 0, failed_runs: 1 },
        };
      case "scheduler_job_types":
        return {
          code: 0,
          message: "ok",
          data: { items: [{ cron_type: "cn_a_share_quote_refresh", label: "A 股行情刷新" }] },
        };
      case "scheduler_jobs_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 7,
                name: "A 股开盘行情刷新",
                cron_type: "cn_a_share_quote_refresh",
                cron_expr: "30 9 * * 1-5",
                enabled: true,
                market: "CN",
              },
            ],
          },
        };
      case "scheduler_runs_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              { id: 41, run_key: "run-ok", trigger_type: "missed_today", status: "failed", target_date: "2026-06-19" },
              { id: 42, run_key: "run-error", trigger_type: "user_request", status: "failed", target_date: "2026-06-19" },
            ],
          },
        };
      case "providers_status":
        return {
          code: 0,
          message: "ok",
          data: [
            { name: "market-provider", source: "sina-tencent-market", available: true },
            { name: "news-provider", source: "sina-live", available: true },
          ],
        };
      case "scheduler_runs_get":
        detailCalls += 1;
        if ((payload as { id?: number }).id === 42) {
          return { code: "scheduler_run_missing", message: "执行记录不存在", data: null };
        }
        return {
          code: 0,
          message: "ok",
          data: {
            id: 41,
            run_key: "run-ok",
            trigger_type: "missed_today",
            status: "failed",
            source: "startup_restore",
            scope_key: "CN:SH:600519",
          },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);
  fireEvent.click(screen.getByRole("link", { name: "任务调度" }));

  await waitFor(() => {
    expect(screen.getByText("run-ok")).toBeInTheDocument();
  });
  fireEvent.click(screen.getAllByRole("button", { name: "查看详情" })[0]);

  await waitFor(() => {
    expect(screen.getByText("startup_restore")).toBeInTheDocument();
  });

  fireEvent.click(screen.getAllByRole("button", { name: "查看详情" })[1]);

  await waitFor(() => {
    expect(screen.getByText("执行记录不存在")).toBeInTheDocument();
  });
  expect(screen.queryByText("startup_restore")).not.toBeInTheDocument();
  expect(detailCalls).toBe(2);
});

test("任务调度页面展示 Provider 不可用状态并禁用立即执行", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
      case "scheduler_status":
        return {
          code: 0,
          message: "ok",
          data: { jobs_total: 1, jobs_enabled: 1, queued_runs: 0, running_runs: 0, failed_runs: 0 },
        };
      case "scheduler_job_types":
        return {
          code: 0,
          message: "ok",
          data: { items: [{ cron_type: "cn_a_share_quote_refresh", label: "A 股行情刷新" }] },
        };
      case "scheduler_jobs_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 7,
                name: "A 股开盘行情刷新",
                cron_type: "cn_a_share_quote_refresh",
                cron_expr: "30 9 * * 1-5",
                enabled: true,
                market: "CN",
              },
            ],
          },
        };
      case "scheduler_runs_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "providers_status":
        return {
          code: 0,
          message: "ok",
          data: [
            { name: "market-provider", source: "unconfigured", available: false, last_error: "market_provider_unconfigured" },
            { name: "news-provider", source: "sina-live", available: true },
          ],
        };
      case "scheduler_jobs_run_now":
        return {
          code: 0,
          message: "ok",
          data: { id: 12, run_key: "run-12", trigger_type: "user_request", status: "queued" },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);
  fireEvent.click(screen.getByRole("link", { name: "任务调度" }));

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "任务调度" })).toBeInTheDocument();
  });
  expect(calls.some((call) => call.command === "providers_status")).toBe(true);
  expect(screen.getByText("market_provider_unconfigured")).toBeInTheDocument();

  const runNowButton = screen.getByRole("button", { name: "立即执行" });
  expect(runNowButton).toBeDisabled();
  fireEvent.click(runNowButton);
  expect(calls.some((call) => call.command === "scheduler_jobs_run_now")).toBe(false);
});

test("AppErrorBoundary 捕获渲染异常并显示失败状态", () => {
  const consoleError = vi.spyOn(console, "error").mockImplementation(() => undefined);

  function BrokenView(): never {
    throw new Error("render failed");
  }

  try {
    render(
      <AppErrorBoundary>
        <BrokenView />
      </AppErrorBoundary>,
    );

    expect(screen.getByText("界面渲染失败")).toBeInTheDocument();
  } finally {
    consoleError.mockRestore();
  }
});
