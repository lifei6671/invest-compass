/* @vitest-environment jsdom */

import "@testing-library/jest-dom/vitest";
import "../../../test/setupDom";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { App as AntApp } from "antd";
import { afterEach, expect, test } from "vitest";
import { AboutAppPage } from "./AboutAppPage";

afterEach(() => {
  cleanup();
});

test("关于页检查更新和日志导出未接入时不显示假成功", async () => {
  render(
    <AntApp>
      <AboutAppPage />
    </AntApp>,
  );

  fireEvent.click(screen.getByRole("button", { name: "检查更新" }));

  expect(screen.getAllByText("检查更新待接入").length).toBeGreaterThan(0);
  expect(screen.queryByText("当前已是最新版本")).not.toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: /导出日志/ }));

  await waitFor(() => {
    expect(screen.getByText("日志导出待接入")).toBeInTheDocument();
  });
  expect(screen.queryByText("日志导出功能待接入")).not.toBeInTheDocument();
});
