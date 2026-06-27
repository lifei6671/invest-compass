/* @vitest-environment jsdom */

import "@testing-library/jest-dom/vitest";
import "../../../test/setupDom";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { App as AntApp } from "antd";
import { afterEach, expect, test, vi } from "vitest";
import { ChartControlBar } from "./ChartControlBar";

afterEach(() => {
  cleanup();
});

test("指标设置齿轮展示迁移后的指标分组", async () => {
  const onIndicatorChange = vi.fn();
  render(
    <AntApp>
      <ChartControlBar
        activePeriod="minute"
        activeAdjust="qfq"
        activeIndicators={["MA", "VOL", "MACD", "KDJ"]}
        onPeriodChange={vi.fn()}
        onAdjustChange={vi.fn()}
        onIndicatorChange={onIndicatorChange}
      />
    </AntApp>,
  );

  fireEvent.click(screen.getByRole("button", { name: "指标设置" }));

  expect(await screen.findByText("指标库")).toBeInTheDocument();
  expect(screen.getByText("趋势")).toBeInTheDocument();
  expect(screen.getByText("波动")).toBeInTheDocument();
  expect(screen.getByText("动量")).toBeInTheDocument();
  expect(screen.getByText("量价")).toBeInTheDocument();
  expect(screen.getByText("强度")).toBeInTheDocument();

  const bollButtons = screen.getAllByRole("button", { name: "BOLL" });
  fireEvent.click(bollButtons[bollButtons.length - 1]);
  expect(onIndicatorChange).toHaveBeenCalledWith("BOLL");

  fireEvent.click(screen.getByRole("button", { name: "SAR" }));
  fireEvent.click(screen.getByRole("button", { name: "CCI" }));
  fireEvent.click(screen.getByRole("button", { name: "AO" }));
  fireEvent.click(screen.getByRole("button", { name: "TRIX" }));
  fireEvent.click(screen.getByRole("button", { name: "ROC" }));
  fireEvent.click(screen.getByRole("button", { name: "PVT" }));
  fireEvent.click(screen.getByRole("button", { name: "ADX" }));

  expect(onIndicatorChange).toHaveBeenCalledWith("SAR");
  expect(onIndicatorChange).toHaveBeenCalledWith("CCI");
  expect(onIndicatorChange).toHaveBeenCalledWith("AO");
  expect(onIndicatorChange).toHaveBeenCalledWith("TRIX");
  expect(onIndicatorChange).toHaveBeenCalledWith("ROC");
  expect(onIndicatorChange).toHaveBeenCalledWith("PVT");
  expect(onIndicatorChange).toHaveBeenCalledWith("ADX");

  fireEvent.click(screen.getByRole("button", { name: "KAMA" }));
  fireEvent.click(screen.getByRole("button", { name: "Aroon" }));

  expect(onIndicatorChange).toHaveBeenCalledWith("KAMA");
  expect(onIndicatorChange).toHaveBeenCalledWith("AROON");
});

test("指标按钮悬停展示作用和计算方式", async () => {
  render(
    <AntApp>
      <ChartControlBar
        activePeriod="day"
        activeAdjust="qfq"
        activeIndicators={["MA"]}
        onPeriodChange={vi.fn()}
        onAdjustChange={vi.fn()}
        onIndicatorChange={vi.fn()}
      />
    </AntApp>,
  );

  fireEvent.click(screen.getByRole("button", { name: "指标设置" }));
  fireEvent.mouseEnter(await screen.findByRole("button", { name: "KAMA" }));

  expect(await screen.findByText(/作用：根据行情噪声自适应快慢的均线/)).toBeInTheDocument();
  expect(screen.getByText(/计算：效率比 ER 调整平滑系数/)).toBeInTheDocument();
});
