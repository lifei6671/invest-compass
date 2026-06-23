/* @vitest-environment jsdom */

import { clearMocks, mockIPC } from "@tauri-apps/api/mocks";
import "@testing-library/jest-dom/vitest";
import "../../test/setupDom";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { App as AntApp, ConfigProvider } from "antd";
import { MemoryRouter } from "react-router-dom";
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

  expect(screen.getByText("自选概览")).toBeInTheDocument();
  expect(screen.getAllByText("0").length).toBeGreaterThan(0);
  expect(screen.getByText("市场分布")).toBeInTheDocument();
  expect(screen.getByText("列表数据仅供研究参考，实际行情请以数据源更新为准。")).toBeInTheDocument();
  expect(screen.getByText("仅供研究，不构成投资建议。")).toBeInTheDocument();
  expect(screen.queryByText("买入")).not.toBeInTheDocument();
  expect(screen.queryByText("卖出")).not.toBeInTheDocument();
  expect(screen.queryByText("下单")).not.toBeInTheDocument();
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
    throw new Error(`unexpected command ${command}`);
  });

  renderPage();

  fireEvent.click(screen.getByRole("radio", { name: /表格视图/ }));
  expect(screen.getByRole("radio", { name: /表格视图/ })).toBeChecked();
  expect(screen.getByRole("columnheader", { name: "股票名称" })).toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: "+ 添加自选" }));
  const addDialog = screen.getByRole("dialog");
  expect(within(addDialog).getByRole("heading", { name: "添加自选股" })).toBeInTheDocument();
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
  expect(calls).toContainEqual({
    command: "watchlist_create",
    payload: {
      payload: {
        symbol: "HK:00700",
        sort_order: 1,
        tags: ["核心标的", "长期跟踪", "消费"],
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

function clickSelectOption(label: string) {
  const option = Array.from(document.querySelectorAll(".ant-select-item-option")).find((item) => item.textContent === label);
  if (!option) {
    throw new Error(`missing option ${label}`);
  }
  fireEvent.click(option);
}
