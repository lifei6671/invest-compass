/* @vitest-environment jsdom */

import "@testing-library/jest-dom/vitest";
import "../../../test/setupDom";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { App as AntApp } from "antd";
import { afterEach, expect, test, vi } from "vitest";
import { ProxySettingsPage } from "./ProxySettingsPage";

const settingsGetMock = vi.hoisted(() => vi.fn(async (): Promise<{ items: Array<{ key: string; value: string }> }> => ({ items: [] })));
const settingsSetMock = vi.hoisted(() => vi.fn(async (): Promise<{ saved_keys: string[] }> => ({ saved_keys: [] })));

vi.mock("../../../services/coreClient", async () => {
  const actual = await vi.importActual<typeof import("../../../services/coreClient")>("../../../services/coreClient");
  return {
    ...actual,
    settingsGet: settingsGetMock,
    settingsSet: settingsSetMock,
  };
});

afterEach(() => {
  settingsGetMock.mockReset();
  settingsSetMock.mockReset();
  settingsGetMock.mockResolvedValue({ items: [] });
  settingsSetMock.mockResolvedValue({ saved_keys: [] });
  vi.useRealTimers();
  cleanup();
});

test("代理页未接入后端时不展示或写入本地假成功结果", async () => {
  vi.useFakeTimers();
  render(
    <AntApp>
      <ProxySettingsPage />
    </AntApp>,
  );

  expect(screen.getByText("连接测试")).toBeInTheDocument();
  expect(screen.queryByText("成功")).not.toBeInTheDocument();
  expect(screen.queryByText(/响应时间：128 ms/)).not.toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: /测试连接/ }));
  await vi.advanceTimersByTimeAsync(1_000);

  expect(screen.queryByText("成功")).not.toBeInTheDocument();
  expect(screen.queryByText(/响应时间：128 ms/)).not.toBeInTheDocument();
  expect(screen.getByText("代理连接测试待接入")).toBeInTheDocument();
});

test("代理页从 settings 读取普通代理字段且不回显代理密码引用", async () => {
  settingsGetMock.mockResolvedValueOnce({
    items: [
      { key: "proxy.mode", value: "http" },
      { key: "proxy.http_url", value: "http://proxy.example:8081" },
      { key: "proxy.no_proxy", value: "localhost;*.local" },
      { key: "proxy.username", value: "alice" },
      { key: "proxy_credential_ref", value: "local-vault://proxy/http-123" },
    ] as Array<{ key: string; value: string }>,
  });

  const { container } = render(
    <AntApp>
      <ProxySettingsPage />
    </AntApp>,
  );

  await waitFor(() => {
    expect(screen.getByRole("button", { name: /HTTP 代理/ })).toHaveClass("settings-proxy-mode-option-active");
  });
  expect(screen.getByDisplayValue("proxy.example")).toBeInTheDocument();
  expect(screen.getByDisplayValue("8081")).toBeInTheDocument();
  expect(screen.getByDisplayValue("alice")).toBeInTheDocument();
  expect(screen.getByDisplayValue("localhost;*.local")).toBeInTheDocument();
  expect(screen.getByText("已保存代理密码，输入新密码可替换")).toBeInTheDocument();
  expect(screen.queryByDisplayValue("local-vault://proxy/http-123")).not.toBeInTheDocument();
  expect((container.querySelector('input[type="password"]') as HTMLInputElement | null)?.value).toBe("");
});

test("代理页保存 HTTP 配置时普通字段走 settingsSet 且密码只走一次性字段", async () => {
  settingsGetMock.mockResolvedValueOnce({
    items: [
      { key: "proxy.mode", value: "http" },
      { key: "proxy.http_url", value: "http://proxy.example:8081" },
      { key: "proxy.no_proxy", value: "localhost;*.local" },
      { key: "proxy.username", value: "alice" },
      { key: "proxy_credential_ref", value: "local-vault://proxy/http-123" },
    ] as Array<{ key: string; value: string }>,
  });

  const { container } = render(
    <AntApp>
      <ProxySettingsPage />
    </AntApp>,
  );

  await waitFor(() => {
    expect(screen.getByDisplayValue("proxy.example")).toBeInTheDocument();
  });
  const passwordInput = container.querySelector('input[type="password"]') as HTMLInputElement;
  fireEvent.change(passwordInput, { target: { value: "new-proxy-secret" } });
  fireEvent.click(screen.getByRole("button", { name: /保存代理配置/ }));

  await waitFor(() => {
    expect(settingsSetMock).toHaveBeenCalledWith({
      items: [
        { key: "proxy.mode", value: "http" },
        { key: "proxy.http_url", value: "http://proxy.example:8081" },
        { key: "proxy.no_proxy", value: "localhost;*.local" },
        { key: "proxy.username", value: "alice" },
      ],
      proxy_password: "new-proxy-secret",
    });
  });
  expect(JSON.stringify(settingsSetMock.mock.calls)).not.toContain("local-vault://proxy/http-123");
});
