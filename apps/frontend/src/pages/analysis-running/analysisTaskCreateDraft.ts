import type { AnalysisTaskResult } from "../../services/coreClient";

type AnalysisTaskCreateDraft = () => Promise<AnalysisTaskResult>;

const drafts = new Map<string, AnalysisTaskCreateDraft>();

// putAnalysisTaskCreateDraft 保存一次性的任务创建闭包，避免把 api_key_ref 和持仓输入放进 router state。
export function putAnalysisTaskCreateDraft(createTask: AnalysisTaskCreateDraft): string {
  const id = typeof crypto !== "undefined" && "randomUUID" in crypto
    ? crypto.randomUUID()
    : `${Date.now()}-${Math.random().toString(36).slice(2)}`;
  drafts.set(id, createTask);
  return id;
}

// takeAnalysisTaskCreateDraft 只允许运行页消费一次，避免旧闭包在内存里长期持有敏感参数。
export function takeAnalysisTaskCreateDraft(id: string): AnalysisTaskCreateDraft | null {
  const draft = drafts.get(id);
  drafts.delete(id);
  return draft ?? null;
}
