/* @vitest-environment jsdom */

import "@testing-library/jest-dom/vitest";
import "../test/setupDom";
import { cleanup, render } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import { autoRefreshSettingsChangedEvent, useAutoRefresh } from "./useAutoRefresh";

const settingsGetMock = vi.hoisted(() => vi.fn());

vi.mock("../services/coreClient", () => ({
  settingsGet: settingsGetMock,
}));

afterEach(() => {
  cleanup();
  settingsGetMock.mockReset();
  vi.useRealTimers();
});

function AutoRefreshProbe(props: { onRefresh: () => Promise<void> | void }) {
  useAutoRefresh(props.onRefresh);
  return <div>probe</div>;
}

test("按基础设置中的行情刷新间隔定时触发刷新", async () => {
  vi.useFakeTimers();
  const onRefresh = vi.fn();
  settingsGetMock.mockResolvedValue({ items: [{ key: "quote.refresh_interval", value: "15s" }] });

  render(<AutoRefreshProbe onRefresh={onRefresh} />);

  await Promise.resolve();
  expect(settingsGetMock).toHaveBeenCalledWith(["quote.refresh_interval", "data_source.quote_refresh_interval"]);
  expect(onRefresh).not.toHaveBeenCalled();

  await vi.advanceTimersByTimeAsync(15_000);

  expect(onRefresh).toHaveBeenCalledTimes(1);
});

test("优先按数据源设置中的行情刷新间隔定时触发刷新", async () => {
  vi.useFakeTimers();
  const onRefresh = vi.fn();
  settingsGetMock.mockResolvedValue({
    items: [
      { key: "quote.refresh_interval", value: "120s" },
      { key: "data_source.quote_refresh_interval", value: "15s" },
    ],
  });

  render(<AutoRefreshProbe onRefresh={onRefresh} />);

  await Promise.resolve();
  expect(settingsGetMock).toHaveBeenCalledWith(["quote.refresh_interval", "data_source.quote_refresh_interval"]);

  await vi.advanceTimersByTimeAsync(15_000);

  expect(onRefresh).toHaveBeenCalledTimes(1);
});

test("刷新间隔设置为 manual 时不启动定时刷新", async () => {
  vi.useFakeTimers();
  const onRefresh = vi.fn();
  settingsGetMock.mockResolvedValue({ items: [{ key: "quote.refresh_interval", value: "manual" }] });

  render(<AutoRefreshProbe onRefresh={onRefresh} />);

  await Promise.resolve();
  expect(settingsGetMock).toHaveBeenCalledWith(["quote.refresh_interval", "data_source.quote_refresh_interval"]);
  await vi.advanceTimersByTimeAsync(120_000);

  expect(onRefresh).not.toHaveBeenCalled();
});

test("刷新间隔设置变更后重装定时器", async () => {
  vi.useFakeTimers();
  const onRefresh = vi.fn();
  settingsGetMock
    .mockResolvedValueOnce({ items: [{ key: "quote.refresh_interval", value: "15s" }] })
    .mockResolvedValueOnce({ items: [{ key: "quote.refresh_interval", value: "manual" }] });

  render(<AutoRefreshProbe onRefresh={onRefresh} />);

  await Promise.resolve();
  await vi.advanceTimersByTimeAsync(15_000);
  expect(onRefresh).toHaveBeenCalledTimes(1);

  window.dispatchEvent(new CustomEvent(autoRefreshSettingsChangedEvent));
  await Promise.resolve();
  await vi.advanceTimersByTimeAsync(60_000);

  expect(settingsGetMock).toHaveBeenCalledTimes(2);
  expect(onRefresh).toHaveBeenCalledTimes(1);
});

test("上一次刷新未完成时不并发启动下一次刷新", async () => {
  vi.useFakeTimers();
  const refreshResolver: { current?: () => void } = {};
  const onRefresh = vi.fn(
    () =>
      new Promise<void>((resolve) => {
        refreshResolver.current = resolve;
      }),
  );
  settingsGetMock.mockResolvedValue({ items: [{ key: "quote.refresh_interval", value: "15s" }] });

  render(<AutoRefreshProbe onRefresh={onRefresh} />);
  await Promise.resolve();
  expect(settingsGetMock).toHaveBeenCalledWith(["quote.refresh_interval", "data_source.quote_refresh_interval"]);

  await vi.advanceTimersByTimeAsync(30_000);
  expect(onRefresh).toHaveBeenCalledTimes(1);

  if (!refreshResolver.current) {
    throw new Error("refresh promise resolver was not captured");
  }
  refreshResolver.current();
  await vi.advanceTimersByTimeAsync(15_000);

  expect(onRefresh).toHaveBeenCalledTimes(2);
});
