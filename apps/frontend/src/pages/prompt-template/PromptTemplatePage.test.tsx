/* @vitest-environment jsdom */

import { clearMocks, mockIPC } from "@tauri-apps/api/mocks";
import "@testing-library/jest-dom/vitest";
import "../../test/setupDom";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { App as AntApp } from "antd";
import { afterEach, expect, test } from "vitest";
import { PromptTemplatePage } from "./PromptTemplatePage";

afterEach(() => {
  clearMocks();
  cleanup();
});

test("Prompt 模板页加载时从后端读取模板列表并渲染首个模板", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "prompt_templates_list") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: 7,
              name: "技术面模板",
              type: "technical",
              description: "技术指标分析",
              content: "分析 {{stock_name}} 的 {{indicators}}",
              variables: ["stock_name", "indicators"],
              is_builtin: false,
              updated_at: "2026-06-22T10:00:00Z",
            },
          ],
        },
      };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <AntApp>
      <PromptTemplatePage />
    </AntApp>,
  );

  await waitFor(() => {
    expect(screen.getByDisplayValue("技术面模板")).toBeInTheDocument();
  });
  expect(screen.getByText("技术指标分析")).toBeInTheDocument();
  expect(screen.getByDisplayValue("分析 {{stock_name}} 的 {{indicators}}")).toBeInTheDocument();
  expect(calls).toContainEqual({ command: "prompt_templates_list", payload: {} });
});

test("Prompt 模板页保存新模板时调用真实创建命令", async () => {
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "prompt_templates_list") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "prompt_templates_create") {
      return {
        code: 0,
        message: "ok",
        data: {
          id: 8,
          ...(payload as any).payload,
          variables: ["stock_name"],
          is_builtin: false,
        },
      };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <AntApp>
      <PromptTemplatePage />
    </AntApp>,
  );

  await waitFor(() => {
    expect(calls).toContainEqual({ command: "prompt_templates_list", payload: {} });
  });
  fireEvent.change(screen.getByLabelText("模板名称"), { target: { value: "自定义个股模板" } });
  fireEvent.change(screen.getByLabelText("Prompt 内容"), { target: { value: "请分析 {{stock_name}}" } });
  fireEvent.click(screen.getByRole("button", { name: "保存" }));

  await waitFor(() => {
    expect(calls.some((call) => call.command === "prompt_templates_create")).toBe(true);
  });
  expect(calls).toContainEqual({
    command: "prompt_templates_create",
    payload: {
      payload: {
        name: "自定义个股模板",
        type: "stock_full",
        description: "",
        content: "请分析 {{stock_name}}",
      },
    },
  });
});

test("Prompt 模板页选择列表模板时调用详情命令", async () => {
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "prompt_templates_list") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: 7,
              name: "综合模板",
              type: "stock_full",
              description: "列表摘要",
              content: "列表内容",
              variables: [],
              is_builtin: false,
            },
            {
              id: 8,
              name: "深度模板",
              type: "stock_full",
              description: "列表摘要",
              content: "列表内容",
              variables: [],
              is_builtin: false,
            },
          ],
        },
      };
    }
    if (command === "prompt_templates_get") {
      return {
        code: 0,
        message: "ok",
        data: {
          id: (payload as any).id,
          name: "深度模板",
          type: "stock_full",
          description: "详情描述",
          content: "详情内容 {{stock_name}}",
          variables: ["stock_name"],
          is_builtin: false,
        },
      };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <AntApp>
      <PromptTemplatePage />
    </AntApp>,
  );

  await waitFor(() => {
    expect(screen.getByText("深度模板")).toBeInTheDocument();
  });
  fireEvent.click(screen.getByText("深度模板"));

  await waitFor(() => {
    expect(screen.getByDisplayValue("详情内容 {{stock_name}}")).toBeInTheDocument();
  });
  expect(calls).toContainEqual({ command: "prompt_templates_get", payload: { id: 8 } });
});

test("Prompt 全屏编辑弹窗使用独立滚动容器而不是页面滚动", async () => {
  mockIPC((command) => {
    if (command === "prompt_templates_list") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: 7,
              name: "技术面分析",
              type: "technical",
              description: "",
              content: Array.from({ length: 40 }, (_, index) => `第 ${index + 1} 行 Prompt 内容`).join("\n"),
              variables: [],
              is_builtin: false,
            },
          ],
        },
      };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <AntApp>
      <PromptTemplatePage />
    </AntApp>,
  );

  await waitFor(() => {
    expect(screen.getByDisplayValue(/第 40 行 Prompt 内容/)).toBeInTheDocument();
  });
  fireEvent.click(screen.getAllByRole("button", { name: "全屏编辑" })[0]);

  const dialog = screen.getByRole("dialog");
  expect(dialog.querySelector(".prompt-editor-fullscreen-content")).toBeInTheDocument();
  expect(dialog.closest(".prompt-editor-fullscreen-modal")).toBeInTheDocument();
});

test("Prompt 模板页编辑和删除已有模板时调用真实命令", async () => {
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "prompt_templates_list") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: 9,
              name: "原始模板",
              type: "custom",
              description: "原始描述",
              content: "原始内容 {{stock_name}}",
              variables: ["stock_name"],
              is_builtin: false,
            },
          ],
        },
      };
    }
    if (command === "prompt_templates_update") {
      return {
        code: 0,
        message: "ok",
        data: {
          id: (payload as any).payload.id,
          name: (payload as any).payload.name,
          type: (payload as any).payload.type,
          description: (payload as any).payload.description,
          content: (payload as any).payload.content,
          variables: ["stock_name"],
          is_builtin: false,
        },
      };
    }
    if (command === "prompt_templates_delete") {
      return { code: 0, message: "ok", data: { id: (payload as any).id } };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <AntApp>
      <PromptTemplatePage />
    </AntApp>,
  );

  await waitFor(() => {
    expect(screen.getByDisplayValue("原始模板")).toBeInTheDocument();
  });
  fireEvent.change(screen.getByLabelText("模板名称"), { target: { value: "更新模板" } });
  fireEvent.change(screen.getByLabelText("Prompt 内容"), { target: { value: "更新内容 {{stock_name}}" } });
  fireEvent.click(screen.getByRole("button", { name: "保存" }));

  await waitFor(() => {
    expect(calls.some((call) => call.command === "prompt_templates_update")).toBe(true);
  });
  expect(calls).toContainEqual({
    command: "prompt_templates_update",
    payload: {
      payload: {
        id: 9,
        name: "更新模板",
        type: "custom",
        description: "原始描述",
        content: "更新内容 {{stock_name}}",
      },
    },
  });

  fireEvent.click(screen.getByRole("button", { name: "删除" }));

  await waitFor(() => {
    expect(calls).toContainEqual({ command: "prompt_templates_delete", payload: { id: 9 } });
  });
});

test("Prompt 模板页选中锁定内置模板时禁止直接编辑和删除", async () => {
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "prompt_templates_list") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: 11,
              key: "builtin_stock_full",
              name: "内置综合分析",
              type: "stock_full",
              description: "首版内置模板",
              content: "请分析 {{stock_name}}，并标注 {{data_asof}}",
              variables: ["stock_name", "data_asof"],
              is_builtin: true,
              builtin_locked: true,
              version: 2,
              checksum: "sha256:builtin",
              source: "builtin",
            },
          ],
        },
      };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <AntApp>
      <PromptTemplatePage />
    </AntApp>,
  );

  await waitFor(() => {
    expect(screen.getByDisplayValue("内置综合分析")).toBeDisabled();
  });
  expect(screen.getByLabelText("Prompt 内容")).toHaveAttribute("readonly");
  expect(screen.getByLabelText("Prompt 内容")).not.toBeDisabled();
  expect(screen.getByRole("button", { name: "保存" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "删除" })).toBeDisabled();

  fireEvent.click(screen.getByRole("button", { name: "保存" }));
  fireEvent.click(screen.getByRole("button", { name: "删除" }));

  expect(calls.map((call) => call.command)).toEqual(["prompt_templates_list"]);
});

test("Prompt 模板页支持将内置模板复制为可编辑自定义草稿", async () => {
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "prompt_templates_list") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: 12,
              key: "builtin_technical",
              name: "内置技术分析",
              type: "technical",
              description: "首版内置模板",
              content: "请分析 {{stock_name}} 的 {{indicators}}",
              variables: ["stock_name", "indicators"],
              is_builtin: true,
              builtin_locked: true,
              version: 1,
              checksum: "sha256:technical",
              source: "builtin",
            },
          ],
        },
      };
    }
    if (command === "prompt_templates_create") {
      return {
        code: 0,
        message: "ok",
        data: {
          id: 21,
          ...(payload as any).payload,
          variables: ["stock_name", "indicators"],
          is_builtin: false,
          builtin_locked: false,
          source: "user",
          version: 1,
        },
      };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <AntApp>
      <PromptTemplatePage />
    </AntApp>,
  );

  await waitFor(() => {
    expect(screen.getByDisplayValue("内置技术分析")).toBeDisabled();
  });

  fireEvent.click(screen.getByRole("button", { name: "创建自定义副本" }));

  await waitFor(() => {
    expect(screen.getByDisplayValue("内置技术分析 副本")).not.toBeDisabled();
  });
  expect(screen.getByLabelText("Prompt 内容")).not.toBeDisabled();

  fireEvent.click(screen.getByRole("button", { name: "保存" }));

  await waitFor(() => {
    expect(calls.some((call) => call.command === "prompt_templates_create")).toBe(true);
  });
  expect(calls).toContainEqual({
    command: "prompt_templates_create",
    payload: {
      payload: {
        name: "内置技术分析 副本",
        type: "technical",
        description: "首版内置模板",
        content: "请分析 {{stock_name}} 的 {{indicators}}",
      },
    },
  });
});
