/* @vitest-environment jsdom */

import "@testing-library/jest-dom/vitest";
import "../../../test/setupDom";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import { OutputPreviewPanel } from "./OutputPreviewPanel";

const longMarkdown = Array.from({ length: 40 }, (_, index) => `第 ${index + 1} 行预览内容`).join("\n");
const longLineMarkdown = "这是一行很长的 Prompt 内容，用来验证预览区应该按容器宽度自动换行，而不是横向裁切。";

afterEach(() => {
  cleanup();
});

test("输出预览内容区域可滚动以查看完整 Prompt", () => {
  render(
    <OutputPreviewPanel
      content={longMarkdown}
      format="Markdown"
      onFormatChange={vi.fn()}
    />,
  );

  expect(screen.getByLabelText("Prompt 输出预览内容")).toHaveClass("prompt-preview-scroll");
});

test("点击全屏预览会打开可滚动预览弹层", async () => {
  render(
    <OutputPreviewPanel
      content={longMarkdown}
      format="Markdown"
      onFormatChange={vi.fn()}
    />,
  );

  fireEvent.click(screen.getByRole("button", { name: "全屏预览" }));

  const dialog = await screen.findByRole("dialog");
  expect(within(dialog).getByText("输出预览")).toBeInTheDocument();
  const fullscreenPreview = screen.getByLabelText("全屏 Prompt 输出预览内容");
  expect(fullscreenPreview).toHaveClass("prompt-preview-fullscreen-scroll");
  expect(within(fullscreenPreview).getByText("第 40 行预览内容")).toBeInTheDocument();
});

test("Markdown 预览正文使用软换行样式以保持与编辑区一致", () => {
  render(
    <OutputPreviewPanel
      content={longLineMarkdown}
      format="Markdown"
      onFormatChange={vi.fn()}
    />,
  );

  expect(screen.getByText(longLineMarkdown)).toHaveClass("prompt-md-wrap-line");
});
