/* @vitest-environment jsdom */

import { clearMocks, mockIPC } from "@tauri-apps/api/mocks";
import "@testing-library/jest-dom/vitest";
import "../../test/setupDom";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { App as AntApp, ConfigProvider } from "antd";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { afterEach, expect, test, vi } from "vitest";
import { WatchlistPage } from "./WatchlistPage";
import { APP_FONT } from "../../styles/fonts";

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

afterEach(() => {
  clearMocks();
  cleanup();
  vi.useRealTimers();
});

function renderPage() {
  return render(
    <MemoryRouter initialEntries={["/watchlist"]}>
      <ConfigProvider theme={{ token: { fontFamily: APP_FONT, colorPrimary: "#1677ff" } }}>
        <AntApp>
          <WatchlistPage />
        </AntApp>
      </ConfigProvider>
    </MemoryRouter>,
  );
}

test("自选股页面默认不展示伪造自选数据", () => {
  mockIPC((command) => {
    if (command === "watchlist_list") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    throw new Error(`unexpected command ${command}`);
  });

  renderPage();

  expect(screen.getByRole("heading", { name: "自选股" })).toBeInTheDocument();
  expect(screen.getByText("跟踪重点标的、行情变化与研究入口")).toBeInTheDocument();
  expect(screen.getByRole("radio", { name: /表格视图/ })).not.toBeChecked();
  expect(screen.getByRole("radio", { name: /卡片视图/ })).toBeChecked();

  expect(screen.queryByText("贵州茅台")).not.toBeInTheDocument();
  expect(screen.queryByText("宁德时代")).not.toBeInTheDocument();
  expect(screen.queryByText("北方华创")).not.toBeInTheDocument();
  expect(screen.getByText("暂无匹配自选股")).toBeInTheDocument();

  expect(screen.queryByText("自选概览")).not.toBeInTheDocument();
  expect(screen.queryByText("市场分布")).not.toBeInTheDocument();
  expect(screen.getByText("列表数据仅供研究参考，实际行情请以数据源更新为准。")).toBeInTheDocument();
  expect(screen.getByText("仅供研究，不构成投资建议。")).toBeInTheDocument();
  expect(screen.queryByText("买入")).not.toBeInTheDocument();
  expect(screen.queryByText("卖出")).not.toBeInTheDocument();
  expect(screen.queryByText("下单")).not.toBeInTheDocument();
});

test("自选股页面兼容后端返回空标签和空备注", async () => {
  mockIPC((command, payload) => {
    const args = payload as { symbol?: string };
    if (command === "watchlist_list") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: 7,
              symbol: "CN:SH:600001",
              sort_order: 1,
              tags: null,
              note: null,
              name: "空标签股票",
              code: "600001",
              market: "CN",
              exchange: "SH",
              industry: "银行",
              created_at: "2026-06-25T10:00:00+08:00",
              quote: {
                symbol: "CN:SH:600001",
                price: 8.88,
                change_amount: 0.11,
                change_percent: 1.23,
                turnover_rate: 0,
                pe: 0,
                quote_time: "2026-06-25T10:00:00+08:00",
              },
            },
          ],
        },
      };
    }
    if (command === "market_quote") {
      return {
        code: 0,
        message: "ok",
        data: {
          symbol: args.symbol,
          price: 8.88,
          change_amount: 0,
          change_percent: 0,
          quote_time: "2026-06-25T10:00:00+08:00",
        },
      };
    }
    if (command === "market_kline") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    throw new Error(`unexpected command ${command}`);
  });

  renderPage();

  await waitFor(() => expect(screen.getByText("空标签股票")).toBeInTheDocument());
  expect(screen.getByText("600001.SH")).toBeInTheDocument();
  expect(screen.getByText("银行")).toBeInTheDocument();
  const stockCard = screen.getByText("空标签股票").closest("article");
  expect(stockCard).not.toBeNull();
  expect(within(stockCard as HTMLElement).getAllByText("--").length).toBeGreaterThanOrEqual(2);
  expect(within(stockCard as HTMLElement).queryByText("0.00%")).not.toBeInTheDocument();
});

test("自选股页面按基础设置定时提交全量后台刷新且不直接请求远端行情", async () => {
  vi.useFakeTimers();
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "settings_get") {
      return { code: 0, message: "ok", data: { items: [{ key: "quote.refresh_interval", value: "15s" }] } };
    }
    if (command === "watchlist_list") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: 5,
              symbol: "CN:SZ:000001",
              sort_order: 2,
              tags: ["观察"],
              note: "原备注",
              quote: {
                symbol: "CN:SZ:000001",
                price: 12.34,
                change_amount: -0.12,
                change_percent: -0.96,
                quote_time: "2026-06-23T15:00:00+08:00",
              },
            },
          ],
        },
      };
    }
    if (command === "watchlist_refresh") {
      return { code: 0, message: "ok", data: { accepted: true, total: 1 } };
    }
    throw new Error(`unexpected command ${command}`);
  });

  renderPage();
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
  expect(screen.getAllByText("000001.SZ").length).toBeGreaterThan(0);
  expect(calls.filter((call) => call.command === "watchlist_list")).toHaveLength(1);
  expect(calls.map((call) => call.command)).not.toContain("market_quote");
  expect(calls.map((call) => call.command)).not.toContain("market_kline");

  await vi.advanceTimersByTimeAsync(15_000);

  expect(calls).toContainEqual({
    command: "watchlist_refresh",
    payload: { payload: { symbols: ["CN:SZ:000001"] } },
  });
  expect(calls.map((call) => call.command)).not.toContain("market_quote");
  expect(calls.map((call) => call.command)).not.toContain("market_kline");
});

test("自选股页面进入时只渲染本地列表，批量刷新才请求行情和走势图", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "watchlist_list") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: 6,
              symbol: "CN:SH:600000",
              sort_order: 1,
              tags: ["银行"],
              note: "本地备注",
              name: "浦发银行",
              code: "600000",
              market: "CN",
              exchange: "SH",
              created_at: "2026-06-24T09:00:00+08:00",
              quote: {
                symbol: "CN:SH:600000",
                price: 9.12,
                change_amount: 0.08,
                change_percent: 0.88,
                amount: 98000000,
                turnover_rate: 0.6,
                pe: 5.4,
                quote_time: "2026-06-23T15:00:00+08:00",
              },
              trend_points: [8.92, 9.03, 9.12],
            },
          ],
        },
      };
    }
    if (command === "watchlist_refresh") {
      return { code: 0, message: "ok", data: { accepted: true, total: 1 } };
    }
    throw new Error(`unexpected command ${command}`);
  });

  renderPage();

  await waitFor(() => expect(screen.getByText("浦发银行")).toBeInTheDocument());
  expect(screen.getByText("本地备注")).toBeInTheDocument();
  expect(screen.getAllByText("600000.SH").length).toBeGreaterThan(0);
  expect(calls.map((call) => call.command)).toContain("watchlist_list");
  expect(calls.map((call) => call.command)).not.toContain("market_quote");
  expect(calls.map((call) => call.command)).not.toContain("market_kline");
  expect(screen.getByText("9.12")).toBeInTheDocument();
  expect(screen.getByText("0.60%")).toBeInTheDocument();
  expect(screen.getByText("5.4")).toBeInTheDocument();
  expect(screen.getByRole("img", { name: "上涨走势" })).toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: /批量刷新/ }));
  await waitFor(() => expect(calls.map((call) => call.command)).toContain("watchlist_refresh"));
  expect(calls).toContainEqual({ command: "watchlist_refresh", payload: { payload: { symbols: ["CN:SH:600000"] } } });
  expect(calls.map((call) => call.command)).not.toContain("market_quote");
  expect(calls.map((call) => call.command)).not.toContain("market_kline");
});

test("自选股页面分页切页生效且页大小选项统一", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  const watchlistItems = Array.from({ length: 11 }, (_, index) => {
    const id = index + 1;
    return {
      id,
      symbol: `CN:SH:6000${String(id).padStart(2, "0")}`,
      sort_order: id,
      tags: [],
      note: "",
      name: `分页股票${String(id).padStart(2, "0")}`,
      code: `6000${String(id).padStart(2, "0")}`,
      market: "CN",
      exchange: "SH",
      created_at: `2026-06-${String(30 - id).padStart(2, "0")}T09:00:00+08:00`,
    };
  });
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    const args = payload as { symbol?: string };
    if (command === "watchlist_list") {
      return { code: 0, message: "ok", data: { items: watchlistItems } };
    }
    if (command === "market_quote") {
      return {
        code: 0,
        message: "ok",
        data: {
          symbol: args.symbol,
          price: 10,
          change_amount: 0.1,
          change_percent: 1,
          quote_time: "2026-06-25T10:00:00+08:00",
        },
      };
    }
    if (command === "market_kline") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "watchlist_refresh") {
      return { code: 0, message: "ok", data: { accepted: true, total: 1 } };
    }
    throw new Error(`unexpected command ${command}`);
  });

  renderPage();

  fireEvent.click(screen.getByRole("radio", { name: /表格视图/ }));
  await waitFor(() => expect(screen.getByText("分页股票01")).toBeInTheDocument());
  expect(screen.queryByText("分页股票11")).not.toBeInTheDocument();

  fireEvent.click(screen.getByText("2"));

  await waitFor(() => expect(screen.getByText("分页股票11")).toBeInTheDocument());
  expect(screen.queryByText("分页股票01")).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: /批量刷新/ }));
  await waitFor(() => {
    expect(calls).toContainEqual({
      command: "watchlist_refresh",
      payload: { payload: { symbols: ["CN:SH:600011"] } },
    });
  });

  fireEvent.mouseDown(screen.getAllByRole("combobox").at(-1)!);

  for (const option of ["10 条/页", "20 条/页", "30 条/页", "40 条/页", "50 条/页"]) {
    expect(screen.getAllByText(option).length).toBeGreaterThan(0);
  }
});

test("自选股页面支持通过股票搜索新增并切换表格", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "watchlist_list") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "stock_search") {
      return {
        code: 0,
        message: "ok",
        data: [
          {
            symbol: "HK:00700",
            name: "腾讯控股",
            market: "港股",
            exchange: "HKEX",
            currency: "HKD",
          },
        ],
      };
    }
    if (command === "watchlist_create") {
      return {
        code: 0,
        message: "ok",
        data: {
          id: 9,
          symbol: "HK:00700",
          sort_order: 1,
          tags: ["核心标的", "长期跟踪", "消费"],
          note: "港股互联网观察",
        },
      };
    }
    if (command === "market_quote") {
      return {
        code: 0,
        message: "ok",
        data: {
          symbol: "HK:00700",
          price: 380,
          change_amount: 3.4,
          change_percent: 0.9,
          amount: 120000000,
          turnover_rate: 1.2,
          pe: 18.6,
          quote_time: "2026-06-23T15:00:00+08:00",
        },
      };
    }
    if (command === "market_kline") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    throw new Error(`unexpected command ${command}`);
  });

  renderPage();

  fireEvent.click(screen.getByRole("radio", { name: /表格视图/ }));
  expect(screen.getByRole("radio", { name: /表格视图/ })).toBeChecked();
  expect(screen.getByRole("columnheader", { name: "股票名称" })).toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: "+ 添加自选" }));
  const addDialog = screen.getByRole("dialog");
  expect(within(addDialog).getByRole("heading", { name: "添加自选股" })).toBeInTheDocument();
  expect(within(addDialog).queryByText("核心标的")).not.toBeInTheDocument();
  expect(within(addDialog).queryByText("长期跟踪")).not.toBeInTheDocument();
  expect(within(addDialog).queryByText("消费")).not.toBeInTheDocument();
  fireEvent.change(within(addDialog).getByPlaceholderText("输入股票名称、代码或拼音，例如：茅台 / 600519 / maotai"), { target: { value: "腾讯" } });
  fireEvent.keyDown(within(addDialog).getByPlaceholderText("输入股票名称、代码或拼音，例如：茅台 / 600519 / maotai"), { key: "Enter", code: "Enter" });
  await waitFor(() => expect(within(addDialog).getByText("腾讯控股")).toBeInTheDocument());
  const tencentRow = within(addDialog).getByRole("row", { name: /腾讯控股/ });
  fireEvent.click(within(tencentRow).getByRole("button", { name: /添\s*加/ }));
  fireEvent.change(within(addDialog).getByPlaceholderText("添加备注，记录你对该股票的关注点..."), { target: { value: "港股互联网观察" } });
  fireEvent.click(within(addDialog).getByRole("button", { name: "确认添加" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  expect(screen.getByText("腾讯控股")).toBeInTheDocument();
  expect(screen.getByText("港股互联网观察")).toBeInTheDocument();
  expect(calls).toContainEqual({ command: "stock_search", payload: { keyword: "腾讯" } });
  expect(calls).toContainEqual({
    command: "watchlist_create",
    payload: {
        payload: {
          symbol: "HK:00700",
          sort_order: 1,
          tags: [],
          note: "港股互联网观察",
        },
      },
  });
});

test("自选股页面编辑和删除调用真实自选股命令", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "watchlist_list") {
      return {
        code: 0,
        message: "ok",
        data: { items: [{ id: 5, symbol: "CN:SZ:000001", sort_order: 2, tags: ["观察"], note: "原备注" }] },
      };
    }
    if (command === "market_quote") {
      return {
        code: 0,
        message: "ok",
        data: {
          symbol: "CN:SZ:000001",
          price: 12.34,
          change_amount: -0.12,
          change_percent: -0.96,
          amount: 260000000,
          turnover_rate: 1.8,
          pe: 6.5,
          quote_time: "2026-06-23T15:00:00+08:00",
        },
      };
    }
    if (command === "market_kline") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "watchlist_update") {
      return {
        code: 0,
        message: "ok",
        data: { id: 5, symbol: "CN:SZ:000001", sort_order: 2, tags: ["核心", "银行"], note: "更新备注" },
      };
    }
    if (command === "watchlist_delete") {
      return { code: 0, message: "ok", data: { id: 5 } };
    }
    throw new Error(`unexpected command ${command}`);
  });

  renderPage();

  fireEvent.click(screen.getByRole("radio", { name: /表格视图/ }));
  await waitFor(() => expect(screen.getAllByText("000001.SZ").length).toBeGreaterThan(0));

  fireEvent.click(screen.getByRole("button", { name: "编辑 000001.SZ" }));
  const editDialog = screen.getByRole("dialog");
  expect(within(editDialog).getByText("编辑自选股")).toBeInTheDocument();
  fireEvent.change(within(editDialog).getByPlaceholderText("多个标签用逗号、空格或顿号分隔"), { target: { value: "核心，银行" } });
  fireEvent.change(within(editDialog).getByPlaceholderText("记录你对该股票的关注点"), { target: { value: "更新备注" } });
  fireEvent.click(within(editDialog).getByRole("button", { name: /保\s*存/ }));

  await waitFor(() => {
    expect(calls).toContainEqual({
      command: "watchlist_update",
      payload: { payload: { id: 5, sort_order: 2, tags: ["核心", "银行"], note: "更新备注" } },
    });
  });
  expect(screen.getByText("更新备注")).toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: "删除 000001.SZ" }));
  await waitFor(() => {
    expect(calls).toContainEqual({ command: "watchlist_delete", payload: { id: 5 } });
  });
  expect(screen.queryByText("更新备注")).not.toBeInTheDocument();
});

test("自选股卡片使用缓存分时点绘制走势且 AI 分析跳转真实分析页", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "watchlist_list") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [{
            id: 11,
            symbol: "CN:SH:600000",
            sort_order: 1,
            tags: ["银行"],
            note: "关注净息差",
            name: "浦发银行",
            code: "600000",
            market: "CN",
            exchange: "SH",
            industry: "银行",
            quote: {
              symbol: "CN:SH:600000",
              price: 9.12,
              change_amount: 0.08,
              change_percent: 0.88,
              amount: 98000000,
              turnover_rate: 0.6,
              pe: 5.4,
              quote_time: "2026-06-23T15:00:00+08:00",
            },
            trend_points: [8.92, 9.03, 9.12],
          }],
        },
      };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <MemoryRouter initialEntries={["/watchlist"]}>
      <ConfigProvider theme={{ token: { fontFamily: APP_FONT, colorPrimary: "#1677ff" } }}>
        <AntApp>
          <Routes>
            <Route path="/watchlist" element={<WatchlistPage />} />
            <Route path="/analysis" element={<LocationProbe />} />
          </Routes>
        </AntApp>
      </ConfigProvider>
    </MemoryRouter>,
  );

  await waitFor(() => expect(screen.getByText("浦发银行")).toBeInTheDocument());
  expect(screen.queryByText("无走势数据")).not.toBeInTheDocument();
  expect(screen.getByRole("img", { name: "上涨走势" })).toBeInTheDocument();
  expect(calls.map((call) => call.command)).not.toContain("market_quote");
  expect(calls.map((call) => call.command)).not.toContain("market_kline");

  fireEvent.click(screen.getByRole("button", { name: /AI分析/ }));
  expect(screen.getByText("analysis:/analysis?symbol=CN%3ASH%3A600000")).toBeInTheDocument();
});

test("自选股详情按钮进入个股详情页", async () => {
  mockIPC((command) => {
    if (command === "watchlist_list") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: 12,
              symbol: "CN:SH:603026",
              sort_order: 1,
              tags: ["材料"],
              note: "关注价格走势",
              name: "石大胜华",
              code: "603026",
              market: "CN",
              exchange: "SH",
              industry: "化工原料",
              quote: {
                symbol: "CN:SH:603026",
                price: 93.78,
                change_amount: -5.42,
                change_percent: -5.46,
                quote_time: "2026-06-25T15:00:00+08:00",
              },
              trend_points: [95.2, 94.1, 93.78],
            },
          ],
        },
      };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <MemoryRouter initialEntries={["/watchlist"]}>
      <ConfigProvider theme={{ token: { fontFamily: APP_FONT, colorPrimary: "#1677ff" } }}>
        <AntApp>
          <Routes>
            <Route path="/watchlist" element={<WatchlistPage />} />
            <Route path="/stocks/:symbol" element={<StockLocationProbe />} />
          </Routes>
        </AntApp>
      </ConfigProvider>
    </MemoryRouter>,
  );

  await waitFor(() => expect(screen.getByText("石大胜华")).toBeInTheDocument());
  fireEvent.click(screen.getByRole("button", { name: /详情/ }));

  expect(screen.getByText("stock:/stocks/CN%3ASH%3A603026")).toBeInTheDocument();
});

test("自选股列表按添加时间倒序展示", async () => {
  mockIPC((command) => {
    if (command === "watchlist_list") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: 1,
              symbol: "CN:SH:600519",
              sort_order: 1,
              tags: ["旧"],
              note: "先添加",
              name: "旧股票",
              code: "600519",
              market: "CN",
              exchange: "SH",
              created_at: "2026-06-22T09:00:00+08:00",
            },
            {
              id: 2,
              symbol: "CN:SZ:300308",
              sort_order: 2,
              tags: ["新"],
              note: "后添加",
              name: "新股票",
              code: "300308",
              market: "CN",
              exchange: "SZ",
              created_at: "2026-06-24T09:00:00+08:00",
            },
          ],
        },
      };
    }
    if (command === "market_quote") {
      return {
        code: 0,
        message: "ok",
        data: {
          symbol: "CN:SH:600519",
          price: 10,
          change_amount: 0,
          change_percent: 0,
          quote_time: "2026-06-24T15:00:00+08:00",
        },
      };
    }
    if (command === "market_kline") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    throw new Error(`unexpected command ${command}`);
  });

  renderPage();

  await waitFor(() => expect(screen.getByText("新股票")).toBeInTheDocument());
  const cards = screen.getAllByRole("article");
  expect(cards).toHaveLength(2);
  expect(cards[0]).toHaveTextContent("新股票");
  expect(cards[1]).toHaveTextContent("旧股票");
});

test("自选股页面市场和标签筛选只基于已加载列表本地过滤", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "watchlist_list") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            { id: 1, symbol: "CN:SH:600519", sort_order: 1, tags: ["白酒"], note: "沪市样本" },
            { id: 2, symbol: "HK:00700", sort_order: 2, tags: ["科技"], note: "港股样本" },
          ],
        },
      };
    }
    if (command === "market_quote") {
      const symbol = (payload as { symbol?: string }).symbol;
      return {
        code: 0,
        message: "ok",
        data: {
          symbol,
          price: symbol === "HK:00700" ? 380 : 1680,
          change_amount: 1,
          change_percent: 0.1,
          amount: 1000000,
          turnover_rate: 1,
          pe: 18,
          quote_time: "2026-06-23T15:00:00+08:00",
        },
      };
    }
    if (command === "market_kline") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    throw new Error(`unexpected command ${command}`);
  });

  renderPage();

  await waitFor(() => expect(screen.getAllByText("600519.SH").length).toBeGreaterThan(0));
  expect(screen.getAllByText("00700.HK").length).toBeGreaterThan(0);

  openSelectByIndex(0);
  clickSelectOption("港股");
  expect(screen.queryByText("600519.SH")).not.toBeInTheDocument();
  expect(screen.getAllByText("00700.HK").length).toBeGreaterThan(0);

  openSelectByIndex(2);
  clickSelectOption("科技");
  expect(screen.getAllByText("00700.HK").length).toBeGreaterThan(0);
  expect(calls.map((call) => call.command)).not.toContain("stock_search");
  expect(calls.map((call) => call.command)).not.toContain("search_global");
});

test("自选股页回车搜索时只调用 search_watchlist_notes 并展示备注范围结果", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "watchlist_list") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "search_watchlist_notes") {
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
    }
    throw new Error(`unexpected command ${command}`);
  });

  renderPage();

  const searchInput = screen.getByPlaceholderText("搜索自选股");
  fireEvent.change(searchInput, { target: { value: "北美客户" } });
  fireEvent.keyDown(searchInput, { key: "Enter", code: "Enter" });

  await waitFor(() => {
    expect(calls).toContainEqual({
      command: "search_watchlist_notes",
      payload: {
        payload: {
          keyword: "北美客户",
          symbols: [],
          limit: 20,
          offset: 0,
          sort: "relevance",
        },
      },
    });
  });
  expect(screen.getByText("中际旭创")).toBeInTheDocument();
  expect(screen.getByText("北美客户订单")).toBeInTheDocument();
  expect(calls.map((call) => call.command)).not.toContain("search_reports");
  expect(calls.map((call) => call.command)).not.toContain("search_news");
  expect(calls.map((call) => call.command)).not.toContain("search_global");
});

test("自选备注搜索无结果时说明仅搜索自选备注和标签", async () => {
  mockIPC((command) => {
    if (command === "watchlist_list") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "search_watchlist_notes") {
      return { code: 0, message: "ok", data: [] };
    }
    throw new Error(`unexpected command ${command}`);
  });

  renderPage();

  const searchInput = screen.getByPlaceholderText("搜索自选股");
  fireEvent.change(searchInput, { target: { value: "不存在" } });
  fireEvent.keyDown(searchInput, { key: "Enter", code: "Enter" });

  await waitFor(() => {
    expect(screen.getByText("仅搜索自选备注和标签，暂无匹配自选项")).toBeInTheDocument();
  });
});

function openSelectByIndex(index: number) {
  const select = screen.getAllByRole("combobox")[index];
  if (!select) {
    throw new Error(`missing select index ${index}`);
  }
  fireEvent.mouseDown(select);
}

function deferredCoreResponse() {
  let resolve!: (value: unknown) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<unknown>((innerResolve, innerReject) => {
    resolve = innerResolve;
    reject = innerReject;
  });
  return { promise, resolve, reject };
}

function LocationProbe() {
  const location = useLocation();
  return <div>{`analysis:${location.pathname}${location.search}`}</div>;
}

function StockLocationProbe() {
  const location = useLocation();
  return <div>{`stock:${location.pathname}${location.search}`}</div>;
}

function clickSelectOption(label: string) {
  const option = Array.from(document.querySelectorAll(".ant-select-item-option")).find((item) => item.textContent === label);
  if (!option) {
    throw new Error(`missing option ${label}`);
  }
  fireEvent.click(option);
}
