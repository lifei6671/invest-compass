/* @vitest-environment jsdom */

import { clearMocks, mockIPC } from "@tauri-apps/api/mocks";
import "@testing-library/jest-dom/vitest";
import "../../../test/setupDom";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { App as AntApp } from "antd";
import { afterEach, expect, test } from "vitest";
import { useDashboardStore } from "../../../stores/dashboardStore";
import { ModelConfigPage } from "./ModelConfigPage";

afterEach(() => {
  clearMocks();
  useDashboardStore.setState({
    state: null,
    loading: false,
    error: null,
    lastLoadedAt: null,
  });
  cleanup();
});

test("模型配置页从真实 ai_config_list 渲染脱敏配置列表", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "ai_config_list") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: 1,
              name: "DeepSeek 主配置",
              provider: "deepseek",
              base_url: "https://api.deepseek.com",
              api_key_ref: "local-vault://ai-config/deepseek-1",
              masked_api_key: "sk-...seek",
              has_api_key: true,
              model_name: "DeepSeek-V3",
              temperature: 0.2,
              max_tokens: 4096,
              timeout_seconds: 120,
              stream_enabled: true,
              is_default: true,
            },
          ],
        },
      };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <AntApp>
      <ModelConfigPage showTabs={false} />
    </AntApp>,
  );

  await waitFor(() => {
    expect(screen.getByText("DeepSeek 主配置")).toBeInTheDocument();
  });
  expect(screen.getByText("https://api.deepseek.com")).toBeInTheDocument();
  expect(screen.getByText("DeepSeek-V3")).toBeInTheDocument();
  expect(screen.getByText("sk-...seek")).toBeInTheDocument();
  expect(screen.getByText("（DeepSeek）")).toBeInTheDocument();
  expect(screen.queryByText("sk-live-secret")).not.toBeInTheDocument();
  expect(calls).toContainEqual({ command: "ai_config_list", payload: {} });
});

test("模型配置页读取到 sidecar 掉线时同步标记全局 core 未连接", async () => {
  useDashboardStore.setState({
    state: {
      health: { status: "ok", version: "0.1.0" },
      summary: {
        watchlist: { up_count: 0, down_count: 0, flat_count: 0 },
        recent_reports: [],
        recent_tasks: [],
        market_news: [],
        risk_tips: [],
        provider_statuses: [{ name: "market", source: "sina", available: true, last_error: "" }],
      },
      indexQuotes: [],
      indexTrends: {},
      watchlistRows: [],
    },
    loading: false,
    error: null,
    lastLoadedAt: "2026-06-22T00:00:00.000Z",
  });
  mockIPC((command) => {
    if (command === "ai_config_list") {
      throw "core sidecar is not running";
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <AntApp>
      <ModelConfigPage showTabs={false} />
    </AntApp>,
  );

  await waitFor(() => {
    expect(useDashboardStore.getState().state).toBeNull();
  });
  expect(useDashboardStore.getState().error).toBe("core sidecar is not running");
});

test("模型配置页新增配置调用 ai_config_save 且只回显脱敏 Key", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "ai_config_list") {
      return {
        code: 0,
        message: "ok",
        data: { items: [] },
      };
    }
    if (command === "ai_config_save") {
      return {
        code: 0,
        message: "ok",
        data: {
          config: {
            ...(payload as any).payload,
            id: 3,
            api_key_ref: "local-vault://ai-config/custom-3",
            masked_api_key: "sk-...cret",
            has_api_key: true,
            api_key: undefined,
          },
        },
      };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <AntApp>
      <ModelConfigPage showTabs={false} />
    </AntApp>,
  );

  await waitFor(() => {
    expect(calls).toContainEqual({ command: "ai_config_list", payload: {} });
  });
  fireEvent.click(screen.getByRole("button", { name: "+ 新建配置" }));
  fireEvent.change(screen.getByLabelText("配置名称"), { target: { value: "自定义接入点" } });
  fireEvent.change(screen.getByLabelText("Base URL"), { target: { value: "https://llm.example.com" } });
  fireEvent.change(screen.getByLabelText("模型名"), { target: { value: "gpt-4.1-mini" } });
  fireEvent.change(screen.getByLabelText("API Key"), { target: { value: "sk-live-secret" } });
  fireEvent.click(screen.getByRole("button", { name: "保存配置" }));

  await waitFor(() => {
    expect(screen.getByText("sk-...cret")).toBeInTheDocument();
  });
  expect(screen.queryByText("sk-live-secret")).not.toBeInTheDocument();
  await waitFor(() => {
    expect(screen.queryByDisplayValue("sk-live-secret")).not.toBeInTheDocument();
  });
  expect(calls.some((call) => call.command === "ai_config_save" && (call.payload as any)?.payload?.api_key === "sk-live-secret")).toBe(true);
});

test("模型配置页保存失败时保留本次输入 API Key", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "ai_config_list") {
      return {
        code: 0,
        message: "ok",
        data: { items: [] },
      };
    }
    if (command === "ai_config_save") {
      throw "sidecar save failed";
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <AntApp>
      <ModelConfigPage showTabs={false} />
    </AntApp>,
  );

  await waitFor(() => {
    expect(calls).toContainEqual({ command: "ai_config_list", payload: {} });
  });
  fireEvent.click(screen.getByRole("button", { name: "+ 新建配置" }));
  fireEvent.change(screen.getByLabelText("配置名称"), { target: { value: "DeepSeek" } });
  fireEvent.change(screen.getByLabelText("Base URL"), { target: { value: "https://api.deepseek.com" } });
  fireEvent.change(screen.getByLabelText("模型名"), { target: { value: "deepseek-v4-flash" } });
  fireEvent.change(screen.getByLabelText("API Key"), { target: { value: "sk-live-secret" } });
  fireEvent.click(screen.getByRole("button", { name: "保存配置" }));

  await waitFor(() => {
    expect(calls.some((call) => call.command === "ai_config_save")).toBe(true);
  });
  expect(screen.getByLabelText("API Key")).toHaveValue("sk-live-secret");
  expect(screen.queryByText("sk-live-secret")).not.toBeInTheDocument();
});

test("模型配置页默认模型和删除操作调用真实命令", async () => {
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "ai_config_list") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: 2,
              name: "OpenAI 主配置",
              provider: "openai-compatible",
              base_url: "https://api.openai.com",
              api_key_ref: "local-vault://ai-config/openai-2",
              masked_api_key: "sk-...abcd",
              has_api_key: true,
              model_name: "gpt-4.1-mini",
              temperature: 0.2,
              max_tokens: 4096,
              timeout_seconds: 120,
              stream_enabled: true,
              is_default: false,
            },
          ],
        },
      };
    }
    if (command === "ai_config_save") {
      const args = payload as { payload: Record<string, unknown> };
      return {
        code: 0,
        message: "ok",
        data: { config: { ...args.payload, is_default: true } },
      };
    }
    if (command === "ai_config_delete") {
      const args = payload as { payload: { id: number } };
      return { code: 0, message: "ok", data: { deleted_id: args.payload.id } };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <AntApp>
      <ModelConfigPage showTabs={false} />
    </AntApp>,
  );

  await waitFor(() => {
    expect(screen.getByText("OpenAI 主配置")).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: "设为默认模型 OpenAI 主配置" }));
  await waitFor(() => {
    expect(calls.some((call) => call.command === "ai_config_save" && call.payload?.payload?.id === 2 && call.payload.payload.is_default === true)).toBe(true);
  });

  fireEvent.click(screen.getByRole("button", { name: "删除 OpenAI 主配置" }));
  const deleteButtons = screen.getAllByRole("button", { name: /删\s*除/ });
  fireEvent.click(deleteButtons[deleteButtons.length - 1]);
  await waitFor(() => {
    expect(calls).toContainEqual({ command: "ai_config_delete", payload: { payload: { id: 2 } } });
  });
});

test("模型配置页默认模型可再次点击取消", async () => {
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "ai_config_list") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: 6,
              name: "DeepSeek 默认配置",
              provider: "deepseek",
              base_url: "https://api.deepseek.com",
              api_key_ref: "local-vault://ai-config/deepseek-6",
              masked_api_key: "sk-...seek",
              has_api_key: true,
              model_name: "deepseek-chat",
              temperature: 0.2,
              max_tokens: 4096,
              timeout_seconds: 120,
              stream_enabled: true,
              is_default: true,
            },
          ],
        },
      };
    }
    if (command === "ai_config_save") {
      const args = payload as { payload: Record<string, unknown> };
      return { code: 0, message: "ok", data: { config: args.payload } };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <AntApp>
      <ModelConfigPage showTabs={false} />
    </AntApp>,
  );

  await waitFor(() => {
    expect(screen.getByText("DeepSeek 默认配置")).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: "取消默认模型 DeepSeek 默认配置" }));

  await waitFor(() => {
    expect(calls.some((call) => call.command === "ai_config_save" && call.payload?.payload?.id === 6 && call.payload.payload.is_default === false)).toBe(true);
  });
  await waitFor(() => {
    expect(screen.getByRole("button", { name: "设为默认模型 DeepSeek 默认配置" })).toBeInTheDocument();
  });
});

test("模型配置页测试连接调用真实 ai_config_test 并跨重新挂载保留状态", async () => {
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "ai_config_list") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: 2,
              name: "OpenAI 主配置",
              provider: "openai-compatible",
              base_url: "https://api.openai.com",
              api_key_ref: "local-vault://ai-config/openai-2",
              masked_api_key: "sk-...abcd",
              has_api_key: true,
              model_name: "gpt-4.1-mini",
              temperature: 0.2,
              max_tokens: 4096,
              timeout_seconds: 120,
              stream_enabled: true,
              is_default: true,
            },
          ],
        },
      };
    }
    if (command === "ai_config_test") {
      return {
        code: 0,
        message: "ok",
        data: {
          ok: true,
          provider: "openai-compatible",
          model: "gpt-4.1-mini",
          message: "ok",
        },
      };
    }
    throw new Error(`unexpected command ${command}`);
  });

  const view = render(
    <AntApp>
      <ModelConfigPage showTabs={false} />
    </AntApp>,
  );

  await waitFor(() => {
    expect(screen.getByText("OpenAI 主配置")).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: "测试连接 OpenAI 主配置" }));

  await waitFor(() => {
    expect(calls).toContainEqual({
      command: "ai_config_test",
      payload: { payload: { id: 2, api_key_ref: "local-vault://ai-config/openai-2" } },
    });
  });
  await waitFor(() => {
    expect(screen.getByText("正常")).toBeInTheDocument();
  });
  view.unmount();

  render(
    <AntApp>
      <ModelConfigPage showTabs={false} />
    </AntApp>,
  );

  await waitFor(() => {
    expect(screen.getByText("OpenAI 主配置")).toBeInTheDocument();
  });
  expect(screen.getByText("正常")).toBeInTheDocument();
});

test("模型配置页测试连接中禁用当前测试按钮避免重复触发", async () => {
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "ai_config_list") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: 5,
              name: "DeepSeek 主配置",
              provider: "openai-compatible",
              base_url: "https://api.deepseek.com",
              api_key_ref: "local-vault://ai-config/deepseek-5",
              masked_api_key: "sk-...seek",
              has_api_key: true,
              model_name: "deepseek-chat",
              temperature: 0.2,
              max_tokens: 4096,
              timeout_seconds: 120,
              stream_enabled: true,
              is_default: true,
            },
          ],
        },
      };
    }
    if (command === "ai_config_test") {
      return new Promise(() => {});
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <AntApp>
      <ModelConfigPage showTabs={false} />
    </AntApp>,
  );

  await waitFor(() => {
    expect(screen.getByText("DeepSeek 主配置")).toBeInTheDocument();
  });
  const testButton = screen.getByRole("button", { name: "测试连接 DeepSeek 主配置" });
  fireEvent.click(testButton);

  await waitFor(() => {
    expect(screen.getByText("测试中")).toBeInTheDocument();
  });
  expect(testButton).toBeDisabled();
  fireEvent.click(testButton);
  expect(calls.filter((call) => call.command === "ai_config_test")).toHaveLength(1);
});

test("模型配置页测试连接失败时脱敏错误消息", async () => {
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "ai_config_list") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: 4,
              name: "故障配置",
              provider: "openai-compatible",
              base_url: "https://api.example.com",
              api_key_ref: "local-vault://ai-config/fail-4",
              masked_api_key: "sk-...fail",
              has_api_key: true,
              model_name: "gpt-test",
              temperature: 0.2,
              max_tokens: 4096,
              timeout_seconds: 60,
              stream_enabled: false,
              is_default: false,
            },
          ],
        },
      };
    }
    if (command === "ai_config_test") {
      return {
        code: 50201,
        message: "上游请求失败 sk-live-secret",
        data: null,
      };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <AntApp>
      <ModelConfigPage showTabs={false} />
    </AntApp>,
  );

  await waitFor(() => {
    expect(screen.getByText("故障配置")).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: "测试连接 故障配置" }));

  await waitFor(() => {
    expect(calls.some((call) => call.command === "ai_config_test")).toBe(true);
  });
  await waitFor(() => {
    expect(screen.getByText("连接失败")).toBeInTheDocument();
  });
  expect(screen.queryByText("sk-live-secret")).not.toBeInTheDocument();
});
