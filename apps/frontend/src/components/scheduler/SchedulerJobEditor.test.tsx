/* @vitest-environment jsdom */

import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import type { FormEvent } from "react";
import { expect, test, vi } from "vitest";
import type { SchedulerJobType } from "../../services/scheduler";
import { SchedulerJobEditor, type SchedulerJobFormState } from "./SchedulerJobEditor";

test("SchedulerJobEditor 渲染受控任务表单并派发编辑操作", () => {
  const value: SchedulerJobFormState = {
    id: 0,
    name: "A 股开盘行情刷新",
    cronType: "cn_a_share_quote_refresh",
    cronExpr: "30 9 * * 1-5",
    enabled: true,
    market: "CN",
    timezone: "Asia/Shanghai",
    tradeWindow: "09:30-15:00",
    symbols: "",
    paramsJSON: "{}",
    catchupEnabled: true,
    catchupMaxDays: "5",
    timeoutSeconds: "120",
  };
  const types: SchedulerJobType[] = [
    { cron_type: "cn_a_share_quote_refresh", label: "A 股行情刷新", default_cron_expr: "30 9 * * 1-5", default_window: "09:30-15:00", market: "CN" },
    { cron_type: "market_news_refresh", label: "市场新闻刷新", default_cron_expr: "0 8 * * 1-5", default_window: "08:00-18:00", market: "CN" },
  ];
  const onChange = vi.fn();
  const onSelectType = vi.fn();
  const onSubmit = vi.fn((event: FormEvent<HTMLFormElement>) => event.preventDefault());
  const onReset = vi.fn();

  render(
    <SchedulerJobEditor
      value={value}
      types={types}
      actionPending=""
      onChange={onChange}
      onSelectType={onSelectType}
      onSubmit={onSubmit}
      onReset={onReset}
    />,
  );

  expect(screen.getByLabelText("名称")).toHaveValue("A 股开盘行情刷新");
  expect(screen.getByLabelText("类型")).toHaveValue("cn_a_share_quote_refresh");
  expect(screen.getByLabelText("Cron")).toHaveValue("30 9 * * 1-5");

  fireEvent.change(screen.getByLabelText("名称"), { target: { value: "新闻刷新" } });
  fireEvent.change(screen.getByLabelText("类型"), { target: { value: "market_news_refresh" } });
  fireEvent.click(screen.getByLabelText("启用补偿"));
  fireEvent.click(screen.getByRole("button", { name: "创建任务" }));
  fireEvent.click(screen.getByRole("button", { name: "重置" }));

  expect(onChange).toHaveBeenCalledWith({ ...value, name: "新闻刷新" });
  expect(onSelectType).toHaveBeenCalledWith("market_news_refresh");
  expect(onChange).toHaveBeenCalledWith({ ...value, catchupEnabled: false });
  expect(onSubmit).toHaveBeenCalledTimes(1);
  expect(onReset).toHaveBeenCalledTimes(1);
});
