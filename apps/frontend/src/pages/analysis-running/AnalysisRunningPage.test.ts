import { expect, test, vi } from "vitest";
import { createDraftIDFromRouteState } from "./AnalysisRunningPage";
import { putAnalysisTaskCreateDraft, takeAnalysisTaskCreateDraft } from "./analysisTaskCreateDraft";

test("运行页路由解析不会在 render 阶段消费任务 draft", () => {
  const createTask = vi.fn().mockResolvedValue({ task_id: "analysis-1", status: "PENDING" });
  const createDraftId = putAnalysisTaskCreateDraft(createTask);

  expect(createDraftIDFromRouteState({ createDraftId })).toBe(createDraftId);
  expect(createDraftIDFromRouteState({ createDraftId })).toBe(createDraftId);
  expect(takeAnalysisTaskCreateDraft(createDraftId)).toBe(createTask);
});
