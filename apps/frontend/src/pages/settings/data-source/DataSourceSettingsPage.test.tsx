/* @vitest-environment jsdom */

import "@testing-library/jest-dom/vitest";
import "../../../test/setupDom";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { App as AntApp } from "antd";
import { afterEach, expect, test, vi } from "vitest";
import { DataSourceSettingsPage } from "./DataSourceSettingsPage";

const settingsGetMock = vi.hoisted(() => vi.fn(async () => ({ items: [] })));
const settingsSetMock = vi.hoisted(() => vi.fn(async () => ({ saved_keys: [] })));
const providersStatusMock = vi.hoisted(() =>
  vi.fn(async () => ({
    items: [
      { name: "行情 Provider", source: "EastMoney", available: true, last_error: "" },
      { name: "新闻 Provider", source: "财联社", available: false, last_error: "credential expired" },
    ],
  })),
);
const cacheStatsMock = vi.hoisted(() =>
  vi.fn(async () => ({
    total_bytes: 6_144,
    items: [
      { target: "quote", bytes: 2_048, label: "行情缓存" },
      { target: "news", bytes: 4_096, label: "新闻缓存" },
    ],
  })),
);
const schedulerStatusMock = vi.hoisted(() =>
  vi.fn(async () => ({
    jobs_total: 5,
    jobs_enabled: 3,
    queued_runs: 1,
    running_runs: 2,
    failed_runs: 4,
  })),
);

vi.mock("../../../services/coreClient", async () => {
  const actual = await vi.importActual<typeof import("../../../services/coreClient")>("../../../services/coreClient");
  return {
    ...actual,
    settingsGet: settingsGetMock,
    settingsSet: settingsSetMock,
    providersStatus: providersStatusMock,
    cacheStats: cacheStatsMock,
    schedulerStatus: schedulerStatusMock,
  };
});

afterEach(() => {
  settingsGetMock.mockClear();
  settingsSetMock.mockClear();
  providersStatusMock.mockClear();
  cacheStatsMock.mockClear();
  schedulerStatusMock.mockClear();
  cleanup();
});

test("数据源概览从真实 Provider、缓存和调度状态派生展示", async () => {
  render(
    <AntApp>
      <DataSourceSettingsPage />
    </AntApp>,
  );

  await waitFor(() => {
    expect(providersStatusMock).toHaveBeenCalledTimes(1);
  });
  expect(cacheStatsMock).toHaveBeenCalledTimes(1);
  expect(schedulerStatusMock).toHaveBeenCalledTimes(1);

  expect(screen.getByText("EastMoney")).toBeInTheDocument();
  expect(screen.getByText("财联社")).toBeInTheDocument();
  expect(screen.getAllByText("异常").length).toBeGreaterThan(0);
  expect(screen.getByText("2 KB")).toBeInTheDocument();
  expect(screen.getByText("4 KB")).toBeInTheDocument();
  expect(screen.getByText("已启用任务")).toBeInTheDocument();
  expect(screen.getByText("3 / 5")).toBeInTheDocument();
  expect(screen.getByText("排队 / 运行")).toBeInTheDocument();
  expect(screen.getByText("1 / 2")).toBeInTheDocument();
  expect(screen.getByText("失败任务")).toBeInTheDocument();
  expect(screen.getByText("4")).toBeInTheDocument();
});

test("数据源概览未接入动作统一提示待接入而不是假成功", async () => {
  render(
    <AntApp>
      <DataSourceSettingsPage />
    </AntApp>,
  );

  await waitFor(() => {
    expect(screen.getByText("行情数据源")).toBeInTheDocument();
  });
  await waitFor(() => {
    expect(providersStatusMock).toHaveBeenCalledTimes(1);
  });

  fireEvent.click(screen.getByRole("button", { name: /测试连接/ }));
  fireEvent.click(screen.getByRole("button", { name: /立即同步/ }));
  fireEvent.click(screen.getByRole("button", { name: /清理缓存/ }));
  fireEvent.click(screen.getByRole("button", { name: /重新检测/ }));

  await waitFor(() => {
    expect(screen.getByText("数据源连接测试待接入")).toBeInTheDocument();
  });
  expect(screen.getByText("新闻同步待接入")).toBeInTheDocument();
  expect(screen.getByText("数据源缓存清理待接入")).toBeInTheDocument();
  await waitFor(() => {
    expect(providersStatusMock).toHaveBeenCalledTimes(2);
  });

  expect(screen.queryByText("数据源连接测试完成")).not.toBeInTheDocument();
  expect(screen.queryByText("新闻同步任务已提交")).not.toBeInTheDocument();
  expect(screen.queryByText("缓存清理完成")).not.toBeInTheDocument();
  expect(screen.queryByText("数据源状态检测完成")).not.toBeInTheDocument();
});
