/* @vitest-environment jsdom */

import "@testing-library/jest-dom/vitest";
import "../../../test/setupDom";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import { PromptTemplateEditor } from "./PromptTemplateEditor";
import type { PromptEditorState } from "../types";

const editorState: PromptEditorState = {
  selectedCategoryId: "stock_full",
  selectedTemplateId: "1",
  templateName: "内置模板",
  templateType: "stock_full",
  templateDescription: "",
  promptContent: "这是一行很长的 Prompt 内容，用来验证编辑区和预览区都应该按容器宽度自动换行，而不是横向裁切。",
  previewFormat: "Markdown",
};

afterEach(() => {
  cleanup();
});

test("Prompt 编辑区启用软换行以匹配预览区阅读方式", () => {
  render(
    <PromptTemplateEditor
      value={editorState}
      templateTypeOptions={[{ label: "个股综合模板", value: "stock_full" }]}
      onChange={vi.fn()}
      onToolAction={vi.fn()}
      onFormat={vi.fn()}
      onFullscreen={vi.fn()}
    />,
  );

  const editor = screen.getByLabelText("Prompt 内容");
  expect(editor).toHaveAttribute("wrap", "soft");
  expect(editor).toHaveClass("prompt-editor-textarea-wrap");
});

test("Prompt 编辑区 textarea 使用边框盒尺寸避免底部内容被父容器裁切", () => {
  render(
    <PromptTemplateEditor
      value={editorState}
      templateTypeOptions={[{ label: "个股综合模板", value: "stock_full" }]}
      onChange={vi.fn()}
      onToolAction={vi.fn()}
      onFormat={vi.fn()}
      onFullscreen={vi.fn()}
    />,
  );

  expect(screen.getByLabelText("Prompt 内容")).toHaveClass("prompt-editor-textarea-fit");
});

test("Prompt 编辑区底部保留可滚动安全区避免最后一行贴边裁切", () => {
  render(
    <PromptTemplateEditor
      value={editorState}
      templateTypeOptions={[{ label: "个股综合模板", value: "stock_full" }]}
      onChange={vi.fn()}
      onToolAction={vi.fn()}
      onFormat={vi.fn()}
      onFullscreen={vi.fn()}
    />,
  );

  const editor = screen.getByLabelText("Prompt 内容");
  expect(editor).toHaveClass("prompt-editor-textarea-bottom-safe");
  expect(editor.closest(".prompt-editor-body")).toHaveClass("prompt-editor-body-scroll-safe");
});

test("Prompt 编辑器高度对齐右侧预览组合底部且保持内部滚动", () => {
  render(
    <PromptTemplateEditor
      value={editorState}
      templateTypeOptions={[{ label: "个股综合模板", value: "stock_full" }]}
      onChange={vi.fn()}
      onToolAction={vi.fn()}
      onFormat={vi.fn()}
      onFullscreen={vi.fn()}
    />,
  );

  const editor = screen.getByLabelText("Prompt 内容");
  expect(editor.closest(".prompt-editor-card")).toHaveClass("prompt-editor-card-aligned");
  expect(editor.closest(".prompt-editor-shell")).toHaveClass("prompt-editor-shell-aligned");
  expect(editor.closest(".prompt-editor-body")).toHaveClass("prompt-editor-body-scrollable");
  expect(editor).toHaveClass("prompt-editor-textarea-scrollable");
});

test("Prompt 编辑区内容不参与外层卡片高度计算", () => {
  render(
    <PromptTemplateEditor
      value={{
        ...editorState,
        promptContent: Array.from({ length: 60 }, (_, index) => `第 ${index + 1} 行测试内容`).join("\n"),
      }}
      templateTypeOptions={[{ label: "个股综合模板", value: "stock_full" }]}
      onChange={vi.fn()}
      onToolAction={vi.fn()}
      onFormat={vi.fn()}
      onFullscreen={vi.fn()}
    />,
  );

  const editor = screen.getByLabelText("Prompt 内容");
  expect(editor.closest(".prompt-editor-body")).toHaveClass("prompt-editor-body-contained");
  expect(editor.previousElementSibling).toHaveClass("prompt-line-numbers-contained");
});

test("只读 Prompt 内容区保持可滚动而不是禁用控件", () => {
  render(
    <PromptTemplateEditor
      value={editorState}
      readOnly
      templateTypeOptions={[{ label: "个股综合模板", value: "stock_full" }]}
      onChange={vi.fn()}
      onToolAction={vi.fn()}
      onFormat={vi.fn()}
      onFullscreen={vi.fn()}
    />,
  );

  const editor = screen.getByLabelText("Prompt 内容");
  expect(editor).toHaveAttribute("readonly");
  expect(editor).not.toBeDisabled();
});
