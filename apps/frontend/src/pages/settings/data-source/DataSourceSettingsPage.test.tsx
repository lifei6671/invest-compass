/* @vitest-environment jsdom */

import "@testing-library/jest-dom/vitest";
import "../../../test/setupDom";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { App as AntApp } from "antd";
import { afterEach, expect, test, vi } from "vitest";
import { autoRefreshSettingsChangedEvent } from "../../../hooks/useAutoRefresh";
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
const cacheCleanMock = vi.hoisted(() => vi.fn(async () => ({ cleaned_targets: ["quote", "kline", "news", "search"], reclaimed_bytes: 6_144 })));
const schedulerStatusMock = vi.hoisted(() =>
  vi.fn(async () => ({
    jobs_total: 5,
    jobs_enabled: 3,
    queued_runs: 1,
    running_runs: 2,
    failed_runs: 4,
  })),
);
const dataSourceCredentialsListMock = vi.hoisted(() =>
  vi.fn(async () => ({
    providers: [
      {
        id: "tdx",
        name: "通达信",
        capability: "K线 / 分钟级K线（MAC链路）",
        status: "normal",
        authType: "none",
        iconType: "tdx",
      },
      {
        id: "alpha-vantage",
        name: "Alpha Vantage",
        capability: "海外行情",
        status: "not_configured",
        authType: "api_key",
        iconType: "alpha",
      },
    ],
    configs: {
      "alpha-vantage": {
        providerId: "alpha-vantage",
        providerName: "Alpha Vantage",
        capability: "海外行情",
        authType: "api_key",
        baseUrl: "https://www.alphavantage.co",
        credentialStatus: "not_configured",
        timeoutSeconds: 15,
        rateLimitPerMinute: 5,
        maskedCredential: "",
        note: "",
      },
    },
    selectedProviderId: "alpha-vantage",
    testTargets: [{ label: "真实连接测试", value: "connectivity" }],
    testResult: { status: "untested", messages: ["尚未执行真实连接测试"] },
    overview: { configuredCount: 0, expiringSoonCount: 0, expiredCount: 0 },
    healthItems: [{ name: "Alpha Vantage", status: "failed", rateLimitText: "未配置" }],
    operationLogs: [],
  })),
);
const dataSourceCredentialsSaveMock = vi.hoisted(() =>
  vi.fn(async () => ({
    config: {
      providerId: "alpha-vantage",
      providerName: "Alpha Vantage",
      capability: "海外行情",
      authType: "api_key",
      baseUrl: "https://www.alphavantage.co",
      credentialStatus: "normal",
      timeoutSeconds: 15,
      rateLimitPerMinute: 5,
      maskedCredential: "sk-****7890",
      note: "",
    },
  })),
);
const dataSourceCredentialsClearMock = vi.hoisted(() => vi.fn());
const dataSourceCredentialsTestMock = vi.hoisted(() => vi.fn());

vi.mock("../../../services/coreClient", async () => {
  const actual = await vi.importActual<typeof import("../../../services/coreClient")>("../../../services/coreClient");
  return {
    ...actual,
    settingsGet: settingsGetMock,
    settingsSet: settingsSetMock,
    providersStatus: providersStatusMock,
    cacheStats: cacheStatsMock,
    cacheClean: cacheCleanMock,
    schedulerStatus: schedulerStatusMock,
    dataSourceCredentialsList: dataSourceCredentialsListMock,
    dataSourceCredentialsSave: dataSourceCredentialsSaveMock,
    dataSourceCredentialsClear: dataSourceCredentialsClearMock,
    dataSourceCredentialsTest: dataSourceCredentialsTestMock,
  };
});

afterEach(() => {
  settingsGetMock.mockClear();
  settingsSetMock.mockClear();
  providersStatusMock.mockClear();
  cacheStatsMock.mockClear();
  cacheCleanMock.mockClear();
  schedulerStatusMock.mockClear();
  dataSourceCredentialsListMock.mockClear();
  dataSourceCredentialsSaveMock.mockClear();
  dataSourceCredentialsClearMock.mockClear();
  dataSourceCredentialsTestMock.mockClear();
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

test("数据源概览将后端 Provider 技术来源说明转成中文展示", async () => {
  providersStatusMock.mockResolvedValueOnce({
    items: [
      {
        name: "sina-tencent-market",
        source: "Sina public quote endpoint, Tencent public kline/minute endpoints, TDX minute kline endpoint, and EastMoney public kline fallback endpoint",
        available: true,
        last_error: "",
      },
      {
        name: "multi-market-news",
        source: "Cailianpress web telegraph endpoint, Sina finance live feed endpoint, Wallstreetcn live news endpoint, TradingView Chinese news endpoint",
        available: true,
        last_error: "",
      },
    ],
  });

  render(
    <AntApp>
      <DataSourceSettingsPage />
    </AntApp>,
  );

  await waitFor(() => {
    expect(screen.getByText("新浪公开行情接口、腾讯 K 线/分时接口、通达信分钟 K 线接口、东方财富 K 线兜底接口")).toBeInTheDocument();
  });
  expect(screen.getByText("财联社电报、新浪财经直播、华尔街见闻快讯、TradingView 中文资讯")).toBeInTheDocument();
  expect(screen.queryByText(/Sina public quote endpoint/)).not.toBeInTheDocument();
  expect(screen.queryByText(/Cailianpress web telegraph endpoint/)).not.toBeInTheDocument();
});

test("数据源概览动作使用真实刷新、清理缓存和已有调度入口", async () => {
  window.location.hash = "#/settings";
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
  fireEvent.click(screen.getByRole("button", { name: /编辑配置/ }));
  expect(screen.getByRole("tab", { name: "凭据管理" })).toHaveAttribute("aria-selected", "true");

  fireEvent.click(screen.getByRole("tab", { name: "数据源概览" }));
  expect(screen.getByRole("button", { name: /立即同步/ })).toBeDisabled();
  fireEvent.click(screen.getByRole("button", { name: /查看日志/ }));
  expect(window.location.hash).toBe("#/scheduler");

  window.location.hash = "#/settings";
  fireEvent.click(screen.getByRole("button", { name: /查看调度配置/ }));
  expect(window.location.hash).toBe("#/scheduler");

  fireEvent.click(screen.getByRole("button", { name: /清理缓存/ }));
  fireEvent.click(screen.getByRole("button", { name: /重新检测/ }));

  await waitFor(() => {
    expect(providersStatusMock).toHaveBeenCalledTimes(4);
  });
  expect(cacheCleanMock).toHaveBeenCalledWith(["quote", "kline", "news", "search"]);

  expect(screen.queryByText("数据源连接测试待接入")).not.toBeInTheDocument();
  expect(screen.queryByText("新闻同步待接入")).not.toBeInTheDocument();
  expect(screen.queryByText("数据源缓存清理待接入")).not.toBeInTheDocument();
  expect(screen.queryByText("数据源连接测试完成")).not.toBeInTheDocument();
  expect(screen.queryByText("新闻同步任务已提交")).not.toBeInTheDocument();
});

test("默认行情源只展示已接入运行时行为的选项并支持保存通达信 K 线源", async () => {
  render(
    <AntApp>
      <DataSourceSettingsPage />
    </AntApp>,
  );

  const sourceSelect = screen.getByLabelText("默认行情源");
  fireEvent.mouseDown(sourceSelect);
  expect(screen.queryByText("AkShare / EastMoney")).not.toBeInTheDocument();
  expect(screen.queryByText("腾讯财经")).not.toBeInTheDocument();
  expect(screen.queryByText("Custom Provider")).not.toBeInTheDocument();
  fireEvent.click(await screen.findByText("通达信（K线）"));

  await waitFor(() => {
    expect(settingsSetMock).toHaveBeenCalledWith({
      items: [{ key: "data_source.default_market_source", value: "tdx" }],
    });
  });
});

test("行情刷新频率保存成功后通知页面级自动刷新重装定时器", async () => {
  const eventListener = vi.fn();
  window.addEventListener(autoRefreshSettingsChangedEvent, eventListener);

  try {
    render(
      <AntApp>
        <DataSourceSettingsPage />
      </AntApp>,
    );

    fireEvent.mouseDown(screen.getByLabelText("行情刷新频率"));
    fireEvent.click(await screen.findByText("手动刷新"));

    await waitFor(() => {
      expect(settingsSetMock).toHaveBeenCalledWith({
        items: [{ key: "data_source.quote_refresh_interval", value: "manual" }],
      });
    });
    expect(eventListener).toHaveBeenCalledTimes(1);
  } finally {
    window.removeEventListener(autoRefreshSettingsChangedEvent, eventListener);
  }
});

test("数据源基础设置不展示尚未接入运行时的保存入口", () => {
  render(
    <AntApp>
      <DataSourceSettingsPage />
    </AntApp>,
  );

  expect(screen.getByLabelText("默认行情源")).toBeInTheDocument();
  expect(screen.getByLabelText("行情刷新频率")).toBeInTheDocument();
  expect(screen.getByLabelText("新闻同步频率")).toBeInTheDocument();
  expect(screen.queryByLabelText("默认资讯源")).not.toBeInTheDocument();
  expect(screen.queryByLabelText("默认市场范围")).not.toBeInTheDocument();
  expect(screen.queryByLabelText("K线数据范围")).not.toBeInTheDocument();
  expect(screen.queryByText("启动时自动同步")).not.toBeInTheDocument();
  expect(screen.queryByText("非交易时段降频")).not.toBeInTheDocument();
});

test("凭据管理页保存后只回显脱敏凭据且不写浏览器存储", async () => {
  const localStorageSetSpy = vi.spyOn(Storage.prototype, "setItem");
  const indexedDbOpenMock = vi.fn();
  const originalIndexedDB = Object.getOwnPropertyDescriptor(globalThis, "indexedDB");
  Object.defineProperty(globalThis, "indexedDB", { value: { open: indexedDbOpenMock }, configurable: true });

  render(
    <AntApp>
      <DataSourceSettingsPage />
    </AntApp>,
  );

  fireEvent.click(screen.getByRole("tab", { name: "凭据管理" }));

  await waitFor(() => {
    expect(dataSourceCredentialsListMock).toHaveBeenCalledTimes(1);
  });
  const credentialInput = await screen.findByLabelText("API Key / 凭据内容");
  fireEvent.change(credentialInput, { target: { value: "sk-live-secret-1234567890" } });
  fireEvent.click(screen.getByRole("button", { name: "保存凭据" }));

  await waitFor(() => {
    expect(dataSourceCredentialsSaveMock).toHaveBeenCalledTimes(1);
  });
  expect(dataSourceCredentialsSaveMock).toHaveBeenCalledWith({
    config: expect.objectContaining({
      providerId: "alpha-vantage",
      maskedCredential: "",
      credentialStatus: "not_configured",
    }),
    credential: "sk-live-secret-1234567890",
  });
  await waitFor(() => {
    expect(screen.getByDisplayValue("sk-****7890")).toBeInTheDocument();
  });
  expect(screen.queryByDisplayValue("sk-live-secret-1234567890")).not.toBeInTheDocument();
  expect(JSON.stringify(dataSourceCredentialsSaveMock.mock.calls)).not.toContain("localStorage");
  expect(localStorageSetSpy).not.toHaveBeenCalled();
  expect(indexedDbOpenMock).not.toHaveBeenCalled();

  localStorageSetSpy.mockRestore();
  if (originalIndexedDB) {
    Object.defineProperty(globalThis, "indexedDB", originalIndexedDB);
  } else {
    Reflect.deleteProperty(globalThis, "indexedDB");
  }
});

test("凭据管理页按 Provider 展示真实连接测试目标", async () => {
  render(
    <AntApp>
      <DataSourceSettingsPage />
    </AntApp>,
  );

  fireEvent.click(screen.getByRole("tab", { name: "凭据管理" }));

  await waitFor(() => {
    expect(dataSourceCredentialsListMock).toHaveBeenCalledTimes(1);
  });
  expect(screen.getByText("通达信")).toBeInTheDocument();
  expect(screen.getByText("K线 / 分钟级K线（MAC链路）")).toBeInTheDocument();
  expect(screen.getByText("海外行情接口（GLOBAL_QUOTE）")).toBeInTheDocument();
  expect(screen.queryByText("真实连接测试")).not.toBeInTheDocument();
});

test("数据说明页链接只跳转已存在子页且不暴露未闭环能力", async () => {
  const windowOpenSpy = vi.spyOn(window, "open").mockImplementation(() => null);

  render(
    <AntApp>
      <DataSourceSettingsPage />
    </AntApp>,
  );

  fireEvent.click(screen.getByRole("tab", { name: "数据说明" }));
  await waitFor(() => {
    expect(screen.getByRole("tab", { name: "数据说明" })).toHaveAttribute("aria-selected", "true");
  });
  expect(screen.getByText("不用于交易执行")).toBeInTheDocument();
  expect(screen.queryByText("下单")).not.toBeInTheDocument();
  expect(screen.queryByText("券商账户")).not.toBeInTheDocument();
  expect(screen.queryByText("自动交易")).not.toBeInTheDocument();
  expect(document.querySelectorAll('a[href^="http"]').length).toBe(0);

  fireEvent.click(screen.getByRole("button", { name: "查看数据源" }));
  expect(screen.getByRole("tab", { name: "数据源概览" })).toHaveAttribute("aria-selected", "true");

  fireEvent.click(screen.getByRole("tab", { name: "数据说明" }));
  fireEvent.click(screen.getByRole("button", { name: "查看数据源概览" }));
  expect(screen.getByRole("tab", { name: "数据源概览" })).toHaveAttribute("aria-selected", "true");

  fireEvent.click(screen.getByRole("tab", { name: "数据说明" }));
  fireEvent.click(screen.getByRole("button", { name: "查看凭据管理 >" }));
  expect(screen.getByRole("tab", { name: "凭据管理" })).toHaveAttribute("aria-selected", "true");

  fireEvent.click(screen.getByRole("tab", { name: "数据说明" }));
  expect(screen.queryByRole("button", { name: "查看更多字段说明 >" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "查看更多 FAQ >" })).not.toBeInTheDocument();
  expect(screen.getByRole("tab", { name: "数据说明" })).toHaveAttribute("aria-selected", "true");
  expect(windowOpenSpy).not.toHaveBeenCalled();

  windowOpenSpy.mockRestore();
});
