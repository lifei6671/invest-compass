/* @vitest-environment jsdom */

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

test("自选股页面默认展示截图规格中的 mock 卡片、统计和风险提示", () => {
  renderPage();

  expect(screen.getByRole("heading", { name: "自选股" })).toBeInTheDocument();
  expect(screen.getByText("跟踪重点标的、行情变化与研究入口")).toBeInTheDocument();
  expect(screen.getByRole("radio", { name: /表格视图/ })).not.toBeChecked();
  expect(screen.getByRole("radio", { name: /卡片视图/ })).toBeChecked();

  expect(screen.getByText("贵州茅台")).toBeInTheDocument();
  expect(screen.getByText("宁德时代")).toBeInTheDocument();
  expect(screen.getByText("北方华创")).toBeInTheDocument();
  expect(screen.getByText("1,688.00")).toHaveClass("text-[#ff4d4f]");
  expect(screen.getByText("238.50")).toHaveClass("text-[#16a34a]");
  expect(screen.getAllByLabelText("上涨走势").length).toBeGreaterThan(0);
  expect(screen.getAllByLabelText("下跌走势").length).toBeGreaterThan(0);

  expect(screen.getByText("自选概览")).toBeInTheDocument();
  expect(screen.getByText("56")).toBeInTheDocument();
  expect(screen.getByText("市场分布")).toBeInTheDocument();
  expect(screen.getByText("列表数据仅供研究参考，实际行情请以数据源更新为准。")).toBeInTheDocument();
  expect(screen.getByText("仅供研究，不构成投资建议。")).toBeInTheDocument();
  expect(screen.queryByText("买入")).not.toBeInTheDocument();
  expect(screen.queryByText("卖出")).not.toBeInTheDocument();
  expect(screen.queryByText("下单")).not.toBeInTheDocument();
});

test("自选股页面支持卡片搜索、删除、切换表格和本地新增", async () => {
  renderPage();

  fireEvent.change(screen.getByPlaceholderText("搜索自选股"), { target: { value: "光模块" } });
  expect(screen.getByText("新易盛")).toBeInTheDocument();
  expect(screen.getByText("中际旭创")).toBeInTheDocument();
  expect(screen.queryByText("贵州茅台")).not.toBeInTheDocument();

  fireEvent.change(screen.getByPlaceholderText("搜索自选股"), { target: { value: "" } });
  const maotaiCard = screen.getByText("贵州茅台").closest("article");
  expect(maotaiCard).toBeTruthy();
  fireEvent.click(within(maotaiCard as HTMLElement).getByRole("button", { name: /删除/ }));

  expect(screen.queryByText("贵州茅台")).not.toBeInTheDocument();
  expect(screen.getByText("宁德时代")).toBeInTheDocument();

  fireEvent.click(screen.getByRole("radio", { name: /表格视图/ }));
  expect(screen.getByRole("radio", { name: /表格视图/ })).toBeChecked();
  expect(screen.getByRole("columnheader", { name: "股票名称" })).toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: "+ 添加自选" }));
  const addDialog = screen.getByRole("dialog");
  expect(within(addDialog).getByRole("heading", { name: "添加自选股" })).toBeInTheDocument();
  const tencentRow = within(addDialog).getByRole("row", { name: /腾讯控股/ });
  fireEvent.click(within(tencentRow).getByRole("button", { name: /添\s*加/ }));
  fireEvent.change(within(addDialog).getByPlaceholderText("添加备注，记录你对该股票的关注点..."), { target: { value: "港股互联网观察" } });
  fireEvent.click(within(addDialog).getByRole("button", { name: "确认添加" }));
  await waitFor(() => expect(screen.getByText("腾讯控股")).toBeInTheDocument());
  expect(screen.getByText("港股互联网观察")).toBeInTheDocument();
});
