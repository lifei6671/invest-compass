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
  mockIPC((command) => {
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
  await waitFor(() => expect(screen.getByText("腾讯控股")).toBeInTheDocument());
  expect(screen.getByText("港股互联网观察")).toBeInTheDocument();
});

test("自选股页回车搜索时只调用 search_watchlist_notes 并展示备注范围结果", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
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
