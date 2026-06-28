/* @vitest-environment jsdom */

import { clearMocks, mockIPC } from "@tauri-apps/api/mocks";
import "@testing-library/jest-dom/vitest";
import "../../test/setupDom";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { App as AntApp, ConfigProvider } from "antd";
import { MemoryRouter, useLocation } from "react-router-dom";
import { afterEach, expect, test, vi } from "vitest";
import { AnalysisPage } from "./AnalysisPage";
import { APP_FONT } from "../../styles/fonts";

afterEach(() => {
  clearMocks();
  cleanup();
  vi.useRealTimers();
});

function renderPage(initialEntry = "/analysis") {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <ConfigProvider theme={{ token: { fontFamily: APP_FONT, colorPrimary: "#1677ff" } }}>
        <AntApp>
          <AnalysisPage />
        </AntApp>
      </ConfigProvider>
    </MemoryRouter>,
  );
}

test("股票选择输入使用去抖搜索，避免每次输入都请求 sidecar", async () => {
  vi.useFakeTimers();
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "ai_config_list") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "prompt_templates_list") {
      return { code: 0, message: "ok", data: { items: promptTemplates() } };
    }
    if (command === "stock_search") {
      return { code: 0, message: "ok", data: [{ symbol: "CN:SH:600584", name: "长电科技", code: "600584" }] };
    }
    throw new Error(`unexpected command ${command}`);
  });

  renderPage();

  const stockInput = screen.getAllByRole("combobox")[0];
  fireEvent.change(stockInput, { target: { value: "长" } });
  fireEvent.change(stockInput, { target: { value: "长电" } });
  fireEvent.change(stockInput, { target: { value: "长电科" } });

  expect(calls.filter((call) => call.command === "stock_search")).toHaveLength(0);
  await act(async () => {
    await vi.advanceTimersByTimeAsync(299);
  });
  expect(calls.filter((call) => call.command === "stock_search")).toHaveLength(0);

  await act(async () => {
    await vi.advanceTimersByTimeAsync(1);
  });

  expect(calls.filter((call) => call.command === "stock_search")).toEqual([
    { command: "stock_search", payload: { keyword: "长电科" } },
  ]);
});

test("股票上下文预览在 K 线失败时仍展示已加载的基础信息和行情", async () => {
  mockIPC((command) => {
    if (command === "ai_config_list") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "prompt_templates_list") {
      return { code: 0, message: "ok", data: { items: promptTemplates() } };
    }
    if (command === "stock_search") {
      return { code: 0, message: "ok", data: [{ symbol: "CN:SH:600584", name: "长电科技", code: "600584" }] };
    }
    if (command === "stock_profile") {
      return {
        code: 0,
        message: "ok",
        data: {
          symbol: "CN:SH:600584",
          name: "长电科技",
          full_name: "江苏长电科技股份有限公司",
          industry: "半导体",
          list_date: "2003-06-03",
        },
      };
    }
    if (command === "market_quote") {
      return {
        code: 0,
        message: "ok",
        data: {
          symbol: "CN:SH:600584",
          price: 94.7,
          change_amount: 8.61,
          change_percent: 10.00000000000002,
          total_market_cap: 15168794104,
          float_market_cap: 9876543210,
          quote_time: "2026-06-24T15:00:00+08:00",
        },
      };
    }
    if (command === "market_kline") {
      throw new Error("sidecar http error: HTTP status server error (502 Bad Gateway)");
    }
    if (command === "market_indicators") {
      return { code: 0, message: "ok", data: { symbol: "CN:SH:600584", period: "day", adjust: "qfq", indicators: { ma5: 90.2 } } };
    }
    if (command === "news_list") {
      return { code: 0, message: "ok", data: { items: [{ id: 1, title: "长电科技先进封装订单增长" }] } };
    }
    throw new Error(`unexpected command ${command}`);
  });

  renderPage("/analysis?symbol=CN:SH:600584");

  expect(await screen.findByText("江苏长电科技股份有限公司")).toBeInTheDocument();
  expect(screen.getByText("半导体")).toBeInTheDocument();
  expect(screen.getByText("94.7")).toBeInTheDocument();
  expect(screen.getByText("+8.61")).toBeInTheDocument();
  expect(screen.getByText("+10.00%")).toBeInTheDocument();
  expect(screen.getByText("151.69亿")).toBeInTheDocument();
  expect(screen.getByText("98.77亿")).toBeInTheDocument();
  expect(screen.getByText("长电科技先进封装订单增长")).toBeInTheDocument();
});

test("URL 指定股票未精确匹配时不静默选择搜索第一项", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "ai_config_list") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "prompt_templates_list") {
      return { code: 0, message: "ok", data: { items: promptTemplates() } };
    }
    if (command === "stock_search") {
      return { code: 0, message: "ok", data: [{ symbol: "CN:SH:600000", name: "浦发银行", code: "600000" }] };
    }
    throw new Error(`unexpected command ${command}`);
  });

  renderPage("/analysis?symbol=CN:SH:600584");

  await waitFor(() => {
    expect(calls).toContainEqual({ command: "stock_search", payload: { keyword: "CN:SH:600584" } });
  });
  await act(async () => {
    await new Promise((resolve) => window.setTimeout(resolve, 50));
  });
  expect(calls.map((call) => call.command)).not.toContain("stock_profile");
  expect(calls.map((call) => call.command)).not.toContain("market_quote");
  expect(screen.queryByText("浦发银行")).not.toBeInTheDocument();
});

test("股票上下文预览按 A 股口径展示百分比和涨跌颜色", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "ai_config_list") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "prompt_templates_list") {
      return { code: 0, message: "ok", data: { items: promptTemplates() } };
    }
    if (command === "stock_search") {
      return { code: 0, message: "ok", data: [{ symbol: "CN:SZ:301217", name: "铜冠铜箔", code: "301217" }] };
    }
    if (command === "stock_profile") {
      return {
        code: 0,
        message: "ok",
        data: {
          symbol: "CN:SZ:301217",
          name: "铜冠铜箔",
          full_name: "安徽铜冠铜箔集团股份有限公司",
          industry: "元器件",
          list_date: "20220127",
        },
      };
    }
    if (command === "market_quote") {
      return {
        code: 0,
        message: "ok",
        data: {
          symbol: "CN:SZ:301217",
          price: 180.67,
          change_amount: -6.02,
          change_percent: -3.22,
          open: 180.98,
          high: 187.58,
          low: 176,
          amount: 6011000000,
          volume: 32983150,
          turnover_rate: 3.98,
          total_market_cap: 14977800000,
          float_market_cap: 14977800000,
          quote_time: "2026-06-25T11:30:00+08:00",
        },
      };
    }
    if (command === "market_kline") {
      const closes = Array.from({ length: 61 }, (_, index) => {
        if (index === 40) {
          return 120;
        }
        if (index === 60) {
          return 110;
        }
        return 100;
      });
      return {
        code: 0,
        message: "ok",
        data: {
          items: closes.map((close, index) => ({
            symbol: "CN:SZ:301217",
            period: "day",
            adjust: "qfq",
            trade_date: `2026-04-${String(index + 1).padStart(2, "0")}`,
            open: close,
            high: close,
            low: close,
            close,
          })),
        },
      };
    }
    if (command === "market_indicators") {
      return { code: 0, message: "ok", data: { symbol: "CN:SZ:301217", period: "day", adjust: "qfq", indicators: {} } };
    }
    if (command === "news_list") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    throw new Error(`unexpected command ${command}`);
  });

  renderPage("/analysis?symbol=CN:SZ:301217");

  expect(await screen.findByText("安徽铜冠铜箔集团股份有限公司")).toBeInTheDocument();
  expect(screen.getByText("-6.02")).toHaveClass("analysis-down");
  expect(screen.getByText("-3.22%")).toHaveClass("analysis-down");
  expect(screen.getByText("3.98%")).toBeInTheDocument();
  expect(screen.getAllByText("149.78亿").length).toBeGreaterThanOrEqual(2);
  expect(screen.queryByText("398.00%")).not.toBeInTheDocument();
  expect(screen.getByText("-8.33%")).toHaveClass("analysis-down");
  expect(screen.getByText("10.00%")).toHaveClass("analysis-up");
  expect(calls).toContainEqual({ command: "market_quote", payload: { symbol: "CN:SZ:301217", forceRefresh: true } });
});

test("选择股票后右侧输出预览展示已渲染的 Prompt 内容", async () => {
  mockIPC((command) => {
    if (command === "ai_config_list") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "prompt_templates_list") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: 11,
              name: "个股综合模板",
              type: "stock_full",
              description: "",
              content: "# {{stock_name}} 投研分析\n\n现价：{{current_price}}\n行业：{{industry}}\n日K：{{daily_klines}}\n相关新闻：{{news_summary}}\n风险偏好：{{risk_preference}}",
              variables: ["stock_name", "current_price", "industry", "daily_klines", "news_summary", "risk_preference"],
              is_builtin: true,
            },
          ],
        },
      };
    }
    if (command === "stock_search") {
      return { code: 0, message: "ok", data: [{ symbol: "CN:SZ:301217", name: "铜冠铜箔", code: "301217" }] };
    }
    if (command === "stock_profile") {
      return {
        code: 0,
        message: "ok",
        data: {
          symbol: "CN:SZ:301217",
          name: "铜冠铜箔",
          full_name: "安徽铜冠铜箔集团股份有限公司",
          industry: "元器件",
          list_date: "20220127",
        },
      };
    }
    if (command === "market_quote") {
      return {
        code: 0,
        message: "ok",
        data: {
          symbol: "CN:SZ:301217",
          price: 183.27,
          change_amount: -3.42,
          change_percent: -1.83,
          quote_time: "2026-06-25T16:29:00+08:00",
        },
      };
    }
    if (command === "market_kline") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            { symbol: "CN:SZ:301217", period: "day", adjust: "qfq", trade_date: "2026-06-24", open: 180.1, high: 185.2, low: 178.6, close: 183.27, volume: 120000, amount: 21992400 },
            { symbol: "CN:SZ:301217", period: "day", adjust: "qfq", trade_date: "2026-06-25", open: 183.4, high: 186.5, low: 181.2, close: 184.9, volume: 135000, amount: 24961500 },
          ],
        },
      };
    }
    if (command === "market_indicators") {
      return { code: 0, message: "ok", data: { symbol: "CN:SZ:301217", period: "day", adjust: "qfq", indicators: {} } };
    }
    if (command === "news_list") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: 1,
              title: "铜冠铜箔扩产项目进展顺利",
              summary: "项目已完成关键设备调试，产能释放进度符合预期。",
            },
          ],
        },
      };
    }
    throw new Error(`unexpected command ${command}`);
  });

  renderPage("/analysis?symbol=CN:SZ:301217");

  expect(await screen.findByText("安徽铜冠铜箔集团股份有限公司")).toBeInTheDocument();
  expect(await screen.findByRole("heading", { name: "铜冠铜箔 投研分析" })).toBeInTheDocument();
  expect(screen.getByText("现价：183.27")).toBeInTheDocument();
  expect(screen.getByText("行业：元器件")).toBeInTheDocument();
  expect(screen.getByText(/date=2026-06-24 open=180.10 high=185.20 low=178.60 close=183.27 volume=120000 amount=21992400/)).toBeInTheDocument();
  expect(screen.queryByText(/变量缺失: daily_klines/)).not.toBeInTheDocument();
  expect(screen.getByText("相关新闻：铜冠铜箔扩产项目进展顺利：项目已完成关键设备调试，产能释放进度符合预期。")).toBeInTheDocument();
  expect(screen.getByText("风险偏好：中等")).toBeInTheDocument();
  expect(screen.queryByText("暂无输出内容")).not.toBeInTheDocument();
});

test("从相关新闻摘要进入资讯中心时带上当前分析股票", async () => {
  let currentRoute = "";
  let currentState: unknown = null;
  function RouteProbe() {
    const location = useLocation();
    currentRoute = `${location.pathname}${location.search}`;
    currentState = location.state;
    return null;
  }
  mockIPC((command) => {
    if (command === "ai_config_list") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "prompt_templates_list") {
      return { code: 0, message: "ok", data: { items: promptTemplates() } };
    }
    if (command === "stock_search") {
      return { code: 0, message: "ok", data: [{ symbol: "CN:SZ:301217", name: "铜冠铜箔", code: "301217" }] };
    }
    if (command === "stock_profile") {
      return { code: 0, message: "ok", data: { symbol: "CN:SZ:301217", name: "铜冠铜箔", full_name: "安徽铜冠铜箔集团股份有限公司", industry: "元器件" } };
    }
    if (command === "market_quote") {
      return { code: 0, message: "ok", data: { symbol: "CN:SZ:301217", price: 183.27 } };
    }
    if (command === "market_kline") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "market_indicators") {
      return { code: 0, message: "ok", data: { symbol: "CN:SZ:301217", period: "day", adjust: "qfq", indicators: {} } };
    }
    if (command === "news_list") {
      return { code: 0, message: "ok", data: { items: [{ id: 1, title: "铜冠铜箔扩产项目进展顺利" }] } };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <MemoryRouter initialEntries={["/analysis?symbol=CN:SZ:301217"]}>
      <ConfigProvider theme={{ token: { fontFamily: APP_FONT, colorPrimary: "#1677ff" } }}>
        <AntApp>
          <AnalysisPage />
          <RouteProbe />
        </AntApp>
      </ConfigProvider>
    </MemoryRouter>,
  );

  await screen.findByText("安徽铜冠铜箔集团股份有限公司");
  fireEvent.click(screen.getByRole("button", { name: /查看更多新闻/ }));

  await waitFor(() => {
    expect(currentRoute).toBe("/news");
  });
  expect(currentState).toEqual({
    newsStock: {
      symbol: "CN:SZ:301217",
      name: "铜冠铜箔",
      code: "301217",
    },
  });
});

test("分析页可复制并导出当前 Prompt 预览 Markdown", async () => {
  const writeText = vi.fn(async () => undefined);
  const createObjectURL = vi.fn(() => "blob:analysis-preview");
  const revokeObjectURL = vi.fn();
  const click = vi.fn();
  const appendChild = vi.spyOn(document.body, "appendChild");
  const removeChild = vi.spyOn(document.body, "removeChild");
  const originalCreateElement = document.createElement.bind(document);
  const createElement = vi.spyOn(document, "createElement");
  Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
  Object.defineProperty(URL, "createObjectURL", { value: createObjectURL, configurable: true });
  Object.defineProperty(URL, "revokeObjectURL", { value: revokeObjectURL, configurable: true });
  createElement.mockImplementation((tagName, options) => {
    const element = originalCreateElement(tagName, options);
    if (tagName.toLowerCase() === "a") {
      element.click = click;
    }
    return element;
  });
  mockIPC((command) => {
    if (command === "ai_config_list") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "prompt_templates_list") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: 11,
              name: "个股综合模板",
              type: "stock_full",
              description: "",
              content: "# {{stock_name}} 投研分析\n\n现价：{{current_price}}\n行业：{{industry}}",
              variables: ["stock_name", "current_price", "industry"],
              is_builtin: true,
            },
          ],
        },
      };
    }
    if (command === "stock_search") {
      return { code: 0, message: "ok", data: [{ symbol: "CN:SZ:301217", name: "铜冠铜箔", code: "301217" }] };
    }
    if (command === "stock_profile") {
      return {
        code: 0,
        message: "ok",
        data: {
          symbol: "CN:SZ:301217",
          name: "铜冠铜箔",
          full_name: "安徽铜冠铜箔集团股份有限公司",
          industry: "元器件",
        },
      };
    }
    if (command === "market_quote") {
      return { code: 0, message: "ok", data: { symbol: "CN:SZ:301217", price: 183.27 } };
    }
    if (command === "market_kline") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "market_indicators") {
      return { code: 0, message: "ok", data: { symbol: "CN:SZ:301217", period: "day", adjust: "qfq", indicators: {} } };
    }
    if (command === "news_list") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    throw new Error(`unexpected command ${command}`);
  });

  renderPage("/analysis?symbol=CN:SZ:301217");

  await screen.findByRole("heading", { name: "铜冠铜箔 投研分析" });
  fireEvent.click(screen.getByRole("button", { name: /复制 Markdown/ }));
  fireEvent.click(screen.getByRole("button", { name: /导出 Markdown/ }));

  await waitFor(() => {
    expect(writeText).toHaveBeenCalledWith(expect.stringContaining("# 铜冠铜箔 投研分析"));
  });
  expect(createObjectURL).toHaveBeenCalledWith(expect.any(Blob));
  expect(appendChild).toHaveBeenCalledWith(expect.objectContaining({ download: "CN-SZ-301217-analysis-preview.md" }));
  expect(click).toHaveBeenCalledTimes(1);
  expect(removeChild).toHaveBeenCalledWith(expect.objectContaining({ href: "blob:analysis-preview" }));
  expect(revokeObjectURL).toHaveBeenCalledWith("blob:analysis-preview");

  createElement.mockRestore();
  appendChild.mockRestore();
  removeChild.mockRestore();
});

test("AI 分析页点击开始后进入创建中页面且不把敏感载荷写入路由状态", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  let currentRoute = "";
  let currentState: unknown = null;
  function RouteProbe() {
    const location = useLocation();
    currentRoute = `${location.pathname}${location.search}`;
    currentState = location.state;
    return null;
  }
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "ai_config_list") {
      return { code: 0, message: "ok", data: { items: [{ id: 1, name: "DeepSeek", provider: "deepseek", model_name: "deepseek-chat", api_key_ref: "local-vault://ai-config/1", has_api_key: true, is_default: true }] } };
    }
    if (command === "prompt_templates_list") {
      return { code: 0, message: "ok", data: { items: [{ id: 11, name: "个股综合模板", type: "stock_full", description: "", content: "分析 {{stock_name}}", variables: ["stock_name"], is_builtin: true }] } };
    }
    if (command === "stock_search") {
      return { code: 0, message: "ok", data: [{ symbol: "CN:SZ:301217", name: "铜冠铜箔", code: "301217" }] };
    }
    if (command === "stock_profile") {
      return { code: 0, message: "ok", data: { symbol: "CN:SZ:301217", name: "铜冠铜箔", full_name: "安徽铜冠铜箔集团股份有限公司", industry: "元器件" } };
    }
    if (command === "market_quote") {
      return { code: 0, message: "ok", data: { symbol: "CN:SZ:301217", price: 183.27 } };
    }
    if (command === "market_kline") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "market_indicators") {
      return { code: 0, message: "ok", data: { symbol: "CN:SZ:301217", period: "day", adjust: "qfq", indicators: {} } };
    }
    if (command === "news_list") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "analysis_task_create") {
      return { code: 0, message: "ok", data: { task_id: "task-created-1" } };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <MemoryRouter initialEntries={["/analysis?symbol=CN:SZ:301217"]}>
      <ConfigProvider theme={{ token: { fontFamily: APP_FONT, colorPrimary: "#1677ff" } }}>
        <AntApp>
          <AnalysisPage />
          <RouteProbe />
        </AntApp>
      </ConfigProvider>
    </MemoryRouter>,
  );

  await screen.findByText("安徽铜冠铜箔集团股份有限公司");
  fireEvent.click(screen.getByRole("button", { name: /开始分析/ }));

  await waitFor(() => {
    expect(currentRoute).toBe("/analysis/running?creating=1");
  });
  expect(calls.some((call) => call.command === "analysis_task_create")).toBe(false);
  expect(JSON.stringify(currentState)).not.toContain("api_key_ref");
  expect(JSON.stringify(currentState)).not.toContain("user_position");
});

test("分析类型切换会联动同分类 Prompt 模板", async () => {
  mockIPC((command) => {
    if (command === "ai_config_list") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "prompt_templates_list") {
      return { code: 0, message: "ok", data: { items: promptTemplates() } };
    }
    throw new Error(`unexpected command ${command}`);
  });

  renderPage();

  expect(await screen.findByText("个股综合模板")).toBeInTheDocument();

  openCombobox(1);
  clickSelectOption("技术面分析");
  await waitFor(() => expect(screen.getByText("技术面模板")).toBeInTheDocument());
  expect(screen.queryByText("个股综合模板")).not.toBeInTheDocument();

  openCombobox(1);
  clickSelectOption("基本面分析");
  await waitFor(() => expect(screen.getByText("基本面模板")).toBeInTheDocument());
  expect(screen.queryByText("技术面模板")).not.toBeInTheDocument();
});

function promptTemplates() {
  return [
    { id: 11, name: "个股综合模板", type: "stock_full", description: "", content: "", variables: [], is_builtin: true },
    { id: 12, name: "技术面模板", type: "technical", description: "", content: "", variables: [], is_builtin: true },
    { id: 13, name: "基本面模板", type: "fundamental", description: "", content: "", variables: [], is_builtin: true },
    { id: 14, name: "消息面模板", type: "news", description: "", content: "", variables: [], is_builtin: true },
  ];
}

function openCombobox(index: number) {
  const combobox = screen.getAllByRole("combobox")[index];
  if (!combobox) {
    throw new Error(`missing combobox ${index}`);
  }
  fireEvent.mouseDown(combobox);
}

function clickSelectOption(label: string) {
  const option = Array.from(document.querySelectorAll(".ant-select-item-option")).find((item) => item.textContent === label);
  if (!option) {
    throw new Error(`missing option ${label}`);
  }
  fireEvent.click(option);
}
