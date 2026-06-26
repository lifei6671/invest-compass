/* @vitest-environment jsdom */

import "@testing-library/jest-dom/vitest";
import "../../../test/setupDom";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { App as AntApp } from "antd";
import { afterEach, expect, test, vi } from "vitest";
import { ProxySettingsPage } from "./ProxySettingsPage";

const settingsGetMock = vi.hoisted(() => vi.fn(async (): Promise<{ items: Array<{ key: string; value: string }> }> => ({ items: [] })));
const settingsSetMock = vi.hoisted(() => vi.fn(async (): Promise<{ saved_keys: string[] }> => ({ saved_keys: [] })));
const proxyConnectionTestMock = vi.hoisted(() =>
  vi.fn(async () => ({
    result: {
      ok: true,
      target: "baidu",
      status_code: 200,
      duration_ms: 128,
      checked_at: "2026-06-24T12:00:00Z",
      message: "ok",
    },
  })),
);

vi.mock("../../../services/coreClient", async () => {
  const actual = await vi.importActual<typeof import("../../../services/coreClient")>("../../../services/coreClient");
  return {
    ...actual,
    proxyConnectionTest: proxyConnectionTestMock,
    settingsGet: settingsGetMock,
    settingsSet: settingsSetMock,
  };
});

afterEach(() => {
  settingsGetMock.mockReset();
  settingsSetMock.mockReset();
  proxyConnectionTestMock.mockReset();
  settingsGetMock.mockResolvedValue({ items: [] });
  settingsSetMock.mockResolvedValue({ saved_keys: [] });
  proxyConnectionTestMock.mockResolvedValue({
    result: {
      ok: true,
      target: "baidu",
      status_code: 200,
      duration_ms: 128,
      checked_at: "2026-06-24T12:00:00Z",
      message: "ok",
    },
  });
  vi.useRealTimers();
  cleanup();
});

test("代理页把 HTTP 和 SOCKS 合并为手动代理且提供不使用代理模式", async () => {
  render(
    <AntApp>
      <ProxySettingsPage />
    </AntApp>,
  );

  expect(screen.getByRole("button", { name: /系统代理/ })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: /不使用代理/ })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: /手动代理/ })).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: /HTTP 代理/ })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: /SOCKS5 代理/ })).not.toBeInTheDocument();
});

test("代理页从 settings 读取普通代理字段且不暴露认证代理入口", async () => {
  settingsGetMock.mockResolvedValueOnce({
    items: [
      { key: "proxy.mode", value: "custom" },
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
    expect(screen.getByRole("button", { name: /手动代理/ })).toHaveClass("settings-proxy-mode-option-active");
  });
  expect(screen.getByText("HTTP")).toBeInTheDocument();
  expect(screen.getByDisplayValue("proxy.example")).toBeInTheDocument();
  expect(screen.getByDisplayValue("8081")).toBeInTheDocument();
  expect(screen.getByDisplayValue("localhost;*.local")).toBeInTheDocument();
  expect(screen.getByText("首版手动代理仅支持无认证代理，已保存的代理凭据不会用于运行时请求。")).toBeInTheDocument();
  expect(screen.queryByLabelText("身份认证")).not.toBeInTheDocument();
  expect(screen.queryByDisplayValue("alice")).not.toBeInTheDocument();
  expect(screen.queryByDisplayValue("local-vault://proxy/http-123")).not.toBeInTheDocument();
  expect(container.querySelector('input[type="password"]')).toBeNull();
});

test("代理页保存 HTTP 配置时只保存无认证代理字段", async () => {
  settingsGetMock.mockResolvedValueOnce({
    items: [
      { key: "proxy.mode", value: "custom" },
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
  expect(container.querySelector('input[type="password"]')).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: /保存代理配置/ }));

  await waitFor(() => {
    expect(settingsSetMock).toHaveBeenCalledWith({
      items: [
        { key: "proxy.mode", value: "custom" },
        { key: "proxy.http_url", value: "http://proxy.example:8081" },
        { key: "proxy.socks5_url", value: "" },
        { key: "proxy.no_proxy", value: "localhost;*.local" },
        { key: "proxy.username", value: "" },
      ],
      clear_proxy_credential: true,
    });
  });
  expect(JSON.stringify(settingsSetMock.mock.calls)).not.toContain("local-vault://proxy/http-123");
  expect(JSON.stringify(settingsSetMock.mock.calls)).not.toContain("new-proxy-secret");
});

test("代理页选择不使用代理时保存 none 并清空手动代理地址和凭据引用", async () => {
  render(
    <AntApp>
      <ProxySettingsPage />
    </AntApp>,
  );

  fireEvent.click(screen.getByRole("button", { name: /不使用代理/ }));

  await waitFor(() => {
    expect(settingsSetMock).toHaveBeenCalledWith({
      items: [
        { key: "proxy.mode", value: "none" },
        { key: "proxy.http_url", value: "" },
        { key: "proxy.socks5_url", value: "" },
        { key: "proxy.username", value: "" },
      ],
      clear_proxy_credential: true,
    });
  });
  expect(screen.getByText("当前应用的外部数据请求不会使用系统代理或手动代理。")).toBeInTheDocument();
});

test("代理页清空手动代理配置时切回不使用代理并清空地址", async () => {
  settingsGetMock.mockResolvedValueOnce({
    items: [
      { key: "proxy.mode", value: "custom" },
      { key: "proxy.http_url", value: "http://proxy.example:8081" },
      { key: "proxy.no_proxy", value: "localhost;*.local" },
    ] as Array<{ key: string; value: string }>,
  });

  render(
    <AntApp>
      <ProxySettingsPage />
    </AntApp>,
  );

  await waitFor(() => {
    expect(screen.getByRole("button", { name: /手动代理/ })).toHaveClass("settings-proxy-mode-option-active");
  });
  fireEvent.click(screen.getByRole("button", { name: /清空配置/ }));

  await waitFor(() => {
    expect(settingsSetMock).toHaveBeenCalledWith({
      items: [
        { key: "proxy.mode", value: "none" },
        { key: "proxy.http_url", value: "" },
        { key: "proxy.socks5_url", value: "" },
        { key: "proxy.username", value: "" },
      ],
      clear_proxy_credential: true,
    });
  });
  expect(screen.getByRole("button", { name: /不使用代理/ })).toHaveClass("settings-proxy-mode-option-active");
  expect(screen.getByText("当前应用的外部数据请求不会使用系统代理或手动代理。")).toBeInTheDocument();
});

test("代理页切换到手动代理时清理历史认证代理引用", async () => {
  settingsGetMock.mockResolvedValueOnce({
    items: [
      { key: "proxy.mode", value: "system" },
      { key: "proxy.http_url", value: "http://proxy.example:8081" },
      { key: "proxy.username", value: "alice" },
      { key: "proxy_credential_ref", value: "local-vault://proxy/http-123" },
    ] as Array<{ key: string; value: string }>,
  });

  render(
    <AntApp>
      <ProxySettingsPage />
    </AntApp>,
  );

  await waitFor(() => {
    expect(screen.getByRole("button", { name: /系统代理/ })).toHaveClass("settings-proxy-mode-option-active");
  });
  fireEvent.click(screen.getByRole("button", { name: /手动代理/ }));

  await waitFor(() => {
    expect(settingsSetMock).toHaveBeenCalledWith({
      items: [
        { key: "proxy.mode", value: "custom" },
        { key: "proxy.username", value: "" },
      ],
      clear_proxy_credential: true,
    });
  });
});

test("代理页测试连接走后端命令并展示真实结果", async () => {
  render(
    <AntApp>
      <ProxySettingsPage />
    </AntApp>,
  );

  fireEvent.click(screen.getByRole("button", { name: /测试连接/ }));

  await waitFor(() => {
    expect(proxyConnectionTestMock).toHaveBeenCalledWith({ target: "baidu" });
  });
  expect(await screen.findByText("响应时间：128 ms")).toBeInTheDocument();
  expect(screen.queryByText("真实代理连接测试待接入")).not.toBeInTheDocument();
});
