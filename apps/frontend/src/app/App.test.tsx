/* @vitest-environment jsdom */

import { clearMocks, mockIPC } from "@tauri-apps/api/mocks";
import "@testing-library/jest-dom/vitest";
import "../test/setupDom";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import { App, AppErrorBoundary } from "./App";
import * as AppModule from "./App";
import { appDatePickerLocale } from "../lib/antdLocale";

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

test("App 启动后通过 typed invoke service 展示 core health 状态", async () => {
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

  expect(screen.getByText("正在连接本地核心服务")).toBeInTheDocument();

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

  await waitFor(() => {
    expect(screen.getByText(/本地核心服务已连接/)).toBeInTheDocument();
  });
});

test("Dashboard 首页按截图结构展示总览桌面", async () => {
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
  expect(screen.getByText("A股 已收盘")).toBeInTheDocument();
  expect(screen.getByText("上证指数")).toBeInTheDocument();
  expect(screen.getByText("深证成指")).toBeInTheDocument();
  expect(screen.getByText("创业板指")).toBeInTheDocument();
  expect(screen.getByText("恒生指数")).toBeInTheDocument();
  expect(screen.getByText("今日热点")).toBeInTheDocument();
  expect(screen.getByText("最近分析报告")).toBeInTheDocument();
  expect(screen.getByText("最近任务状态")).toBeInTheDocument();
  expect(screen.getByText("上涨")).toBeInTheDocument();
  expect(screen.getByText("下跌")).toBeInTheDocument();
  expect(screen.getByText("平盘")).toBeInTheDocument();
  expect(screen.getByText("行业热点")).toBeInTheDocument();
  expect(screen.getByText("半导体")).toBeInTheDocument();
  expect(screen.getByText("宁德时代（300750）深度分析报告")).toBeInTheDocument();
  expect(screen.getByText("宁德时代深度分析")).toBeInTheDocument();
  expect(screen.getByText("仅供研究，不构成投资建议。")).toBeInTheDocument();
  expect(screen.getByText("AI 生成内容仅供研究参考，请结合公开披露信息独立判断。")).toBeInTheDocument();
  expect(document.querySelector(".ant-badge-count")).not.toBeInTheDocument();
  expect(calls.map((call) => call.command)).not.toContain("dashboard_summary");
  expect(calls.map((call) => call.command)).not.toContain("watchlist_list");
  expect(calls.map((call) => call.command)).not.toContain("market_kline");
});

test("Dashboard 首页使用本地 mock 状态驱动顶部市场状态", async () => {
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
  expect(screen.getByText("A股 已收盘")).toBeInTheDocument();
  expect(screen.getByText("数据更新：")).toBeInTheDocument();
  expect(screen.getByText((content) => content.includes("2025") && content.includes("15:30:00"))).toBeInTheDocument();
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
  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "生益科技" })).toBeInTheDocument();
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

test("自选股页面展示本地 mock 卡片并支持筛选和本地删除", async () => {
  window.location.hash = "#/watchlist";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "自选股" })).toBeInTheDocument();
  });
  expect(screen.getByRole("radio", { name: /卡片视图/ })).toBeChecked();
  expect(screen.getByText("贵州茅台")).toBeInTheDocument();
  expect(screen.getByText("600519.SH")).toBeInTheDocument();
  expect(screen.getByText("宁德时代")).toBeInTheDocument();
  expect(screen.getByText("共 56 条")).toBeInTheDocument();
  expect(screen.getByText("自选概览")).toBeInTheDocument();
  expect(screen.getByText("市场分布")).toBeInTheDocument();
  expect(screen.queryByText("买入")).not.toBeInTheDocument();
  expect(screen.queryByText("卖出")).not.toBeInTheDocument();

  fireEvent.change(screen.getByPlaceholderText("搜索自选股"), { target: { value: "光模块" } });
  expect(screen.getByText("新易盛")).toBeInTheDocument();
  expect(screen.getByText("中际旭创")).toBeInTheDocument();
  expect(screen.queryByText("贵州茅台")).not.toBeInTheDocument();

  fireEvent.change(screen.getByPlaceholderText("搜索自选股"), { target: { value: "" } });
  const maotaiCard = screen.getByText("贵州茅台").closest("article");
  expect(maotaiCard).toBeTruthy();
  fireEvent.click(within(maotaiCard as HTMLElement).getByRole("button", { name: /删除/ }));
  expect(screen.queryByText("贵州茅台")).not.toBeInTheDocument();
  expect(calls).toEqual([]);
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

test("个股详情页展示本地 mock 工作台且不作为左侧菜单入口", async () => {
  window.location.hash = "#/stocks/CN%3ASH%3A600183";
  mockIPC((command) => {
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "生益科技" })).toBeInTheDocument();
  });
  expect(screen.queryByRole("link", { name: /个股详情/ })).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: /返回/ })).toBeInTheDocument();
  expect(screen.getByText("CN:SH:600183")).toBeInTheDocument();
  expect(screen.getByText("25.68")).toBeInTheDocument();
  expect(screen.getByText("+1.42%")).toBeInTheDocument();
  expect(screen.getByRole("heading", { name: "K线图" })).toBeInTheDocument();
  expect(screen.getByText("MA5")).toBeInTheDocument();
  expect(screen.getByText("基础信息")).toBeInTheDocument();
  expect(screen.getByText("我的标签与备注")).toBeInTheDocument();
  expect(screen.getByText("研究快捷入口")).toBeInTheDocument();
  expect(screen.getByText("生益科技：一季度归母净利润同比增长18.35% 产品结构持续优化")).toBeInTheDocument();
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
    expect(screen.getByRole("heading", { name: "生益科技" })).toBeInTheDocument();
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

test("AI 分析页展示静态工作台并只使用本地交互", async () => {
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
  const stockInput = screen.getByDisplayValue(/生益科技\s+CN:SH:600183/);
  fireEvent.change(stockInput, { target: { value: "茅台" } });
  await waitFor(() => {
    expect(screen.getByText("贵州茅台")).toBeInTheDocument();
  });
  fireEvent.click(screen.getByText("贵州茅台"));
  expect(screen.getByDisplayValue(/贵州茅台\s+CN:SH:600519/)).toBeInTheDocument();
  expect(screen.getByText("个股综合分析")).toBeInTheDocument();
  expect(screen.getByText("DeepSeek (DeepSeek-V3)")).toBeInTheDocument();
  expect(screen.getByText("默认个股分析模板")).toBeInTheDocument();
  expect(screen.getByText("管理模板")).toBeInTheDocument();

  expect(screen.getByText("可选持仓上下文")).toBeInTheDocument();
  expect(screen.getByText("成本价（元）")).toBeInTheDocument();
  expect(screen.getByText("股数（股）")).toBeInTheDocument();
  expect(screen.getByText("风险偏好")).toBeInTheDocument();
  expect(screen.getByText("仅用于本次分析上下文，不落库")).toBeInTheDocument();

  expect(screen.getByText("数据上下文预览")).toBeInTheDocument();
  expect(screen.getByText("基础信息摘要")).toBeInTheDocument();
  expect(screen.getByText("广东生益科技股份有限公司")).toBeInTheDocument();
  expect(screen.getByText("最新行情摘要")).toBeInTheDocument();
  expect(screen.getByText("25.68")).toBeInTheDocument();
  expect(screen.getAllByText("+1.42%").length).toBeGreaterThan(0);
  expect(screen.getByText("K线概况（日K）")).toBeInTheDocument();
  expect(screen.getByText("+18.65%")).toBeInTheDocument();
  expect(screen.getByText("技术指标摘要")).toBeInTheDocument();
  expect(screen.getByText("相关新闻摘要")).toBeInTheDocument();
  expect(screen.getByText("生益科技：一季度归母净利润同比增长18.35%，产品结构持续优化")).toBeInTheDocument();

  expect(screen.getByText("输出预览")).toBeInTheDocument();
  expect(screen.getByText("生益科技（600183.SH）个股综合分析报告（示例）")).toBeInTheDocument();
  expect(screen.getByText("1. 核心结论")).toBeInTheDocument();
  expect(screen.getByText("6. 后续观察指标")).toBeInTheDocument();
  fireEvent.mouseDown(screen.getByText("Markdown"));
  fireEvent.click(screen.getAllByRole("option", { name: "纯文本" })[0]);
  expect(screen.getByText(/核心结论/)).toBeInTheDocument();

  const stopButton = screen.getByRole("button", { name: /停止生成/ });
  expect(stopButton).toBeDisabled();
  fireEvent.click(screen.getByRole("button", { name: /保存报告/ }));
  fireEvent.click(screen.getByRole("button", { name: /复制 Markdown/ }));
  await waitFor(() => {
    expect(writeText).toHaveBeenCalledWith(expect.stringContaining("个股综合分析报告"));
  });
  fireEvent.click(screen.getByRole("button", { name: /导出 Markdown/ }));
  fireEvent.click(screen.getByText("管理模板"));
  fireEvent.click(screen.getByText("查看更多新闻 >"));
  fireEvent.click(screen.getByLabelText("全屏预览"));

  expect(screen.getByText("AI 输出需区分事实、推断和观点，仅供研究参考。")).toBeInTheDocument();
  expect(screen.getAllByText("仅供研究，不构成投资建议。").length).toBeGreaterThan(0);

  fireEvent.click(screen.getByRole("button", { name: /开始分析/ }));
  await waitFor(() => {
    expect(window.location.hash).toBe("#/analysis/running");
    expect(screen.getByRole("heading", { name: "正在生成：生益科技 个股综合分析" })).toBeInTheDocument();
  });

  expect(screen.queryByText("下单")).not.toBeInTheDocument();
  expect(screen.queryByText("券商账户")).not.toBeInTheDocument();
  expect(screen.queryByText("自动交易")).not.toBeInTheDocument();
  expect(calls).toEqual([]);
});

test("AI 分析生成中页面展示 mock 运行态并只使用本地交互", async () => {
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
    expect(screen.getByRole("heading", { name: "正在生成：生益科技 个股综合分析" })).toBeInTheDocument();
  });
  expect(screen.getByRole("link", { name: "AI 分析" })).toHaveAttribute("aria-current", "page");
  expect(screen.getByText("AI 分析任务进行中，请稍候...")).toBeInTheDocument();
  expect(screen.getByText("RUNNING")).toBeInTheDocument();
  expect(screen.getByText("task_20250520_153012_abcd1234")).toBeInTheDocument();
  expect(screen.getByText("00:01:42")).toBeInTheDocument();
  expect(screen.getByText("DeepSeek-V3")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: /返回/ })).toBeInTheDocument();

  expect(screen.getByText("任务步骤")).toBeInTheDocument();
  expect(screen.getByText("校验股票代码")).toBeInTheDocument();
  expect(screen.getByText("拉取基础信息")).toBeInTheDocument();
  expect(screen.getByText("拉取行情与K线")).toBeInTheDocument();
  expect(screen.getByText("计算技术指标")).toBeInTheDocument();
  expect(screen.getByText("计算中...（约 20%）")).toBeInTheDocument();
  expect(screen.getAllByText("拉取新闻资讯").length).toBeGreaterThan(0);
  expect(screen.getAllByText("构建 Prompt").length).toBeGreaterThan(0);
  expect(screen.getByText("调用 AI 模型")).toBeInTheDocument();
  expect(screen.getByText("保存分析报告")).toBeInTheDocument();

  expect(screen.getByText("流式输出")).toBeInTheDocument();
  expect(screen.getByText("（正在生成中...）")).toBeInTheDocument();
  expect(screen.getByText("自动滚动")).toBeInTheDocument();
  expect(screen.getByText("生益科技（600183.SH）个股综合分析")).toBeInTheDocument();
  expect(screen.getByText("1. 核心结论（生成中...）")).toBeInTheDocument();
  expect(screen.getByText("3. 技术面观察（生成中...）")).toBeInTheDocument();
  expect(screen.getByText("内容持续生成中...")).toBeInTheDocument();

  expect(screen.getByText("任务日志")).toBeInTheDocument();
  expect(screen.getByText("TASK_STARTED")).toBeInTheDocument();
  expect(screen.getAllByText("TASK_PROGRESS").length).toBeGreaterThan(0);
  expect(screen.getAllByText("TASK_CHUNK").length).toBe(3);
  expect(screen.getByText("任务已创建，准备开始执行分析")).toBeInTheDocument();
  expect(screen.getByText("流式输出：技术面观察段落")).toBeInTheDocument();
  expect(screen.getByText("+512 字")).toBeInTheDocument();

  const autoScrollSwitch = screen.getByRole("switch");
  expect(autoScrollSwitch).toBeChecked();
  fireEvent.click(autoScrollSwitch);
  expect(autoScrollSwitch).not.toBeChecked();
  fireEvent.click(screen.getByRole("button", { name: /^清\s*空$/ }));
  fireEvent.click(screen.getByRole("button", { name: "清空日志" }));
  fireEvent.click(screen.getByRole("button", { name: /返回/ }));

  const stopButton = screen.getByRole("button", { name: /停止生成/ });
  expect(stopButton).not.toBeDisabled();
  fireEvent.click(stopButton);
  expect(stopButton).toBeDisabled();
  fireEvent.click(screen.getByRole("button", { name: /后台运行/ }));
  fireEvent.click(screen.getByRole("button", { name: /复制当前内容/ }));
  await waitFor(() => {
    expect(writeText).toHaveBeenCalledWith(expect.stringContaining("生益科技（600183.SH）个股综合分析"));
  });

  expect(screen.getByText("任务完成后可在报告历史中查看完整内容。")).toBeInTheDocument();
  expect(screen.getByText("AI 输出需区分事实、推断和观点，仅供研究参考，不构成投资建议。")).toBeInTheDocument();
  expect(screen.getAllByText("仅供研究，不构成投资建议。").length).toBeGreaterThan(0);
  expect(screen.queryByText("下单")).not.toBeInTheDocument();
  expect(screen.queryByText("券商账户")).not.toBeInTheDocument();
  expect(screen.queryByText("自动交易")).not.toBeInTheDocument();
  expect(calls).toEqual([]);
});

test("分析报告历史页面展示筛选表格统计并支持本地交互", async () => {
  window.location.hash = "#/reports";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "分析报告历史" })).toBeInTheDocument();
  });

  expect(screen.getByText("查看、筛选、复制和导出历史投研报告")).toBeInTheDocument();
  expect(screen.getByText("报告列表")).toBeInTheDocument();
  expect(screen.getAllByText("生益科技 个股综合分析").length).toBeGreaterThan(0);
  expect(screen.getByText("雅克科技 技术面分析")).toBeInTheDocument();
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
  expect(screen.getByText("雅克科技 技术面分析")).toBeInTheDocument();
  expect(screen.queryByText("生益科技 个股综合分析")).not.toBeInTheDocument();

  const startDateInput = screen.getByPlaceholderText("2025-04-20");
  fireEvent.input(startDateInput, { target: { value: "2025-05-21" } });
  fireEvent.click(screen.getByLabelText("查询报告"));
  expect(screen.queryByText("雅克科技 技术面分析")).not.toBeInTheDocument();

  fireEvent.click(screen.getByLabelText("重置筛选"));
  expect(screen.getAllByText("生益科技 个股综合分析").length).toBeGreaterThan(0);

  expect(screen.queryByRole("button", { name: "复制 生益科技 个股综合分析" })).not.toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: "删除 生益科技 个股综合分析" }));
  await waitFor(() => {
    expect(screen.queryByText("生益科技 个股综合分析")).not.toBeInTheDocument();
  });
  expect(calls).toEqual([]);
});

test("资讯中心页面展示筛选列表右侧观察并支持本地交互", async () => {
  window.location.hash = "#/news";
  const writeText = vi.fn(() => Promise.resolve());
  Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "资讯中心" })).toBeInTheDocument();
  });

  expect(screen.getByText("聚合个股新闻、市场新闻、行业事件与研究线索")).toBeInTheDocument();
  expect(screen.getByText("资讯列表")).toBeInTheDocument();
  expect(screen.getByText("（共 218 条）")).toBeInTheDocument();
  expect(screen.getByText("热点观察")).toBeInTheDocument();
  expect(screen.getByText("数据源状态")).toBeInTheDocument();
  expect(screen.getByText("生益科技：公司高端覆铜板产品订单饱满，持续提升 AI 服务器领域份额")).toBeInTheDocument();
  expect(screen.getByText("英伟达 Blackwell 需求强劲，光模块厂商迎来新一轮订单增长")).toBeInTheDocument();
  expect(screen.queryByText("下单")).not.toBeInTheDocument();
  expect(screen.queryByText("券商账户")).not.toBeInTheDocument();
  expect(screen.queryByText("公告专用入口")).not.toBeInTheDocument();

  fireEvent.change(screen.getByPlaceholderText("输入关键词，支持标题/摘要"), { target: { value: "光模块" } });
  expect(screen.getByText("英伟达 Blackwell 需求强劲，光模块厂商迎来新一轮订单增长")).toBeInTheDocument();
  expect(screen.queryByText("生益科技：公司高端覆铜板产品订单饱满，持续提升 AI 服务器领域份额")).not.toBeInTheDocument();

  fireEvent.click(screen.getByText("PCB"));
  expect(screen.getByText("PCB 板块震荡走强，服务器需求拉动高端板材景气度")).toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: /刷新资讯/ }));
  fireEvent.click(screen.getAllByRole("button", { name: /查看原文/ })[0]);
  fireEvent.click(screen.getAllByRole("button", { name: /加入上下文/ })[0]);
  fireEvent.click(screen.getAllByRole("button", { name: /复制摘要/ })[0]);
  await waitFor(() => {
    expect(writeText).toHaveBeenCalledWith(expect.stringContaining("AI 服务器"));
  });
  fireEvent.click(screen.getByRole("button", { name: /清理缓存/ }));

  expect(calls).toEqual([]);
});

test("分析报告详情页面展示报告正文、输入快照并支持本地交互", async () => {
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
    expect(screen.getByRole("heading", { name: "生益科技 CN:SH:600183 投研分析" })).toBeInTheDocument();
  });

  expect(screen.getByText("目录")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "1. 核心结论" })).toBeInTheDocument();
  expect(screen.getByRole("heading", { name: "1. 核心结论" })).toBeInTheDocument();
  expect(screen.getByRole("heading", { name: "4. 基本面观察" })).toBeInTheDocument();
  expect(screen.getByText("输入快照")).toBeInTheDocument();
  expect(screen.getByText("风险声明")).toBeInTheDocument();
  expect(screen.queryByText("下单")).not.toBeInTheDocument();
  expect(screen.queryByText("券商账户")).not.toBeInTheDocument();

  fireEvent.click(screen.getByLabelText("收藏报告"));
  fireEvent.click(screen.getByRole("button", { name: /复制 Markdown/ }));
  await waitFor(() => {
    expect(writeText).toHaveBeenCalledWith(expect.stringContaining("核心结论"));
  });
  fireEvent.click(screen.getByRole("button", { name: /导出 Markdown/ }));
  fireEvent.click(screen.getByRole("button", { name: /重新分析/ }));
  fireEvent.click(screen.getByRole("button", { name: /删除/ }));
  fireEvent.click(screen.getByRole("button", { name: "2. 当前行情状态" }));
  fireEvent.click(screen.getByRole("button", { name: /返回列表/ }));

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "分析报告历史" })).toBeInTheDocument();
  });

  expect(calls).toEqual([]);
});

test("任务历史页面展示本地 mock 并支持详情切换", async () => {
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
  expect(screen.getAllByText("task_20250520_152834_abcd1234").length).toBeGreaterThan(0);
  expect(screen.getAllByText("生益科技 个股综合分析").length).toBeGreaterThan(0);
  expect(screen.getAllByText("运行中").length).toBeGreaterThan(0);
  expect(screen.getAllByText("成功").length).toBeGreaterThan(0);
  expect(screen.queryByText("事件流")).not.toBeInTheDocument();

  fireEvent.click(screen.getAllByText("生益科技 个股综合分析")[0]);
  expect(screen.getByText("事件流")).toBeInTheDocument();
  expect(screen.getByText("TASK_CREATED")).toBeInTheDocument();

  fireEvent.click(screen.getByText("泰晶科技 个股综合分析"));
  expect(screen.getAllByText("task_20250520_123501_pqr2345").length).toBeGreaterThan(0);
  expect(screen.getAllByText("模型调用超时").length).toBeGreaterThan(0);
  expect(calls).toEqual([]);
});

test("任务历史页面支持本地筛选、重置和取消任务", async () => {
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
  expect(screen.getByText("雅克科技 技术面分析")).toBeInTheDocument();

  expect(appDatePickerLocale.lang.shortWeekDays).toEqual(["日", "一", "二", "三", "四", "五", "六"]);
  expect(appDatePickerLocale.lang.shortMonths).toContain("6月");

  fireEvent.click(screen.getByRole("button", { name: /重\s*置/ }));
  expect(screen.getAllByText("生益科技 个股综合分析").length).toBeGreaterThan(0);

  fireEvent.click(screen.getAllByRole("button", { name: /取消/ })[0]);
  expect(screen.getAllByText("已取消").length).toBeGreaterThan(0);
  expect(calls).toEqual([]);
});

test("设置中心基础设置页展示本地 mock 并支持基础交互", async () => {
  window.location.hash = "#/settings";
  const calls: Array<{ command: string; payload?: any }> = [];
  const writeText = vi.fn<(text: string) => Promise<void>>(() => Promise.resolve());
  Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
  mockIPC((command, payload) => {
    calls.push({ command, payload });
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
  expect(screen.getByText("DeepSeek-V3")).toBeInTheDocument();
  expect(screen.getByText("行情刷新频率")).toBeInTheDocument();
  expect(screen.getByText("默认 K 线周期")).toBeInTheDocument();
  expect(screen.getByText("默认复权类型")).toBeInTheDocument();
  expect(screen.getByText("管理应用的工作区路径与数据存储位置")).toBeInTheDocument();
  expect(screen.getByText("配置任务与系统通知的接收方式")).toBeInTheDocument();
  expect(screen.getByText("配置桌面端行为与系统集成能力")).toBeInTheDocument();
  expect(screen.getByText("512.7 MB")).toBeInTheDocument();
  expect(screen.getByText("系统代理")).toBeInTheDocument();
  expect(screen.getByText("帮助我们改进产品（不会收集个人信息）")).toBeInTheDocument();
  expect(screen.getByText("敏感信息会脱敏保存；日志导出前将自动清理 API Key 与代理密码。")).toBeInTheDocument();

  const switches = screen.getAllByRole("switch");
  expect(switches).toHaveLength(6);
  expect(switches[0]).toBeChecked();
  expect(switches[1]).toBeChecked();
  expect(switches[2]).not.toBeChecked();
  expect(switches[3]).toBeChecked();
  expect(switches[4]).toBeChecked();
  expect(switches[5]).toBeChecked();

  fireEvent.click(screen.getByRole("button", { name: "选择目录" }));
  fireEvent.click(screen.getByRole("button", { name: "打开目录" }));
  fireEvent.click(screen.getByRole("button", { name: /清理缓存/ }));
  expect(screen.getAllByText("0 MB")).toHaveLength(2);
  fireEvent.click(screen.getByRole("button", { name: /编辑代理设置/ }));
  fireEvent.click(screen.getByRole("tab", { name: "模型设置" }));
  expect(screen.getByRole("tab", { name: "模型设置" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByText("Provider 列表")).toBeInTheDocument();
  expect(screen.getByText("模型配置列表")).toBeInTheDocument();
  expect(screen.getByText("默认 OpenAI")).toBeInTheDocument();

  fireEvent.click(screen.getByRole("tab", { name: "Prompt 配置" }));
  expect(screen.getByRole("tab", { name: "Prompt 配置" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByRole("tab", { name: "关于应用" })).toBeInTheDocument();
  expect(screen.getByRole("heading", { name: "Prompt 模板" })).toBeInTheDocument();
  expect(screen.getByText("管理系统提示词与投研分析模板")).toBeInTheDocument();
  expect(screen.getByText("模板分类")).toBeInTheDocument();
  expect(screen.getByText("系统模板")).toBeInTheDocument();
  expect(screen.getAllByText("个股分析模板").length).toBeGreaterThan(0);
  expect(screen.getByText("默认个股分析模板")).toBeInTheDocument();
  expect(screen.getByText("深度个股分析模板")).toBeInTheDocument();
  expect(screen.getByText("技术分析模板")).toBeInTheDocument();
  expect(screen.getByText("财务分析模板")).toBeInTheDocument();
  expect(screen.getByText("持仓分析模板")).toBeInTheDocument();
  expect(screen.getByText("市场复盘模板")).toBeInTheDocument();
  expect(screen.getByText("自定义模板")).toBeInTheDocument();
  expect(screen.getByDisplayValue("默认个股分析模板")).toBeInTheDocument();
  expect(screen.getByDisplayValue("面向个股的综合分析模板，包含基本面、技术面与消息面分析框架。")).toBeInTheDocument();
  expect((screen.getByLabelText("Prompt 内容") as HTMLTextAreaElement).value).toContain("{{stock_name}}");
  expect(screen.getByText("变量说明")).toBeInTheDocument();
  expect(screen.getByText("{{stock_name}}")).toBeInTheDocument();
  expect(screen.getByText("{{analysis_language}}")).toBeInTheDocument();
  expect(screen.getByText("输出预览")).toBeInTheDocument();
  expect(screen.getByText("生益科技（600183.SH）个股综合分析")).toBeInTheDocument();
  fireEvent.click(screen.getByText("深度个股分析模板"));
  expect(screen.getByDisplayValue("深度个股分析模板")).toBeInTheDocument();
  fireEvent.change(screen.getByDisplayValue("深度个股分析模板"), { target: { value: "自定义个股分析模板" } });
  expect(screen.getByDisplayValue("自定义个股分析模板")).toBeInTheDocument();
  fireEvent.change(screen.getByLabelText("Prompt 内容"), { target: { value: "# 测试模板\n{{stock_code}}" } });
  fireEvent.click(screen.getByRole("button", { name: /保存/ }));
  fireEvent.click(screen.getByRole("button", { name: /复制/ }));
  await waitFor(() => {
    expect(writeText).toHaveBeenCalledWith("# 测试模板\n{{stock_code}}");
  });
  fireEvent.click(screen.getByRole("button", { name: /恢复默认/ }));
  fireEvent.click(screen.getByRole("button", { name: /delete\s+删除/ }));
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
  fireEvent.click(screen.getByRole("button", { name: "格式化" }));
  fireEvent.click(screen.getByRole("button", { name: "全屏编辑" }));
  fireEvent.mouseDown(screen.getByText("Markdown"));
  fireEvent.click(screen.getAllByRole("option", { name: "纯文本" })[0]);
  expect(screen.getByText(/核心结论/)).toBeInTheDocument();
  expect(screen.queryByText("输入激活码")).not.toBeInTheDocument();
  expect(screen.queryByText("下单")).not.toBeInTheDocument();
  expect(screen.queryByText("券商账户")).not.toBeInTheDocument();

  fireEvent.click(screen.getByRole("tab", { name: "数据源设置" }));
  expect(screen.getByRole("tab", { name: "数据源设置" })).toHaveAttribute("aria-selected", "true");
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
  expect(screen.getByDisplayValue("invest_user")).toBeInTheDocument();
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
  expect(screen.getByDisplayValue("invest_user")).toBeInTheDocument();
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

  expect(calls).toEqual([]);
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
