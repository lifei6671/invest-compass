import {
  schedulerJobTypes as coreSchedulerJobTypes,
  schedulerJobsBackfill as coreSchedulerJobsBackfill,
  schedulerJobsDelete as coreSchedulerJobsDelete,
  schedulerJobsGet as coreSchedulerJobsGet,
  schedulerJobsList as coreSchedulerJobsList,
  schedulerJobsRunNow as coreSchedulerJobsRunNow,
  schedulerJobsSave as coreSchedulerJobsSave,
  schedulerJobsSetEnabled as coreSchedulerJobsSetEnabled,
  schedulerRefreshSymbol as coreSchedulerRefreshSymbol,
  schedulerRunsGet as coreSchedulerRunsGet,
  schedulerRunsList as coreSchedulerRunsList,
  schedulerRunsTrigger as coreSchedulerRunsTrigger,
  schedulerStatus as coreSchedulerStatus,
  type SchedulerBackfillPayload,
  type SchedulerBackfillResult,
  type SchedulerJob,
  type SchedulerJobPayload,
  type SchedulerJobsList,
  type SchedulerJobType,
  type SchedulerJobTypes,
  type SchedulerRefreshSymbolPayload,
  type SchedulerRefreshSymbolResult,
  type SchedulerRun,
  type SchedulerRunNowPayload,
  type SchedulerRunsList,
  type SchedulerRunsListPayload,
  type SchedulerStatus,
  type SchedulerTriggerPayload,
} from "./coreClient";

export type {
  SchedulerBackfillPayload,
  SchedulerBackfillResult,
  SchedulerJob,
  SchedulerJobPayload,
  SchedulerJobsList,
  SchedulerJobType,
  SchedulerJobTypes,
  SchedulerRefreshSymbolPayload,
  SchedulerRefreshSymbolResult,
  SchedulerRun,
  SchedulerRunNowPayload,
  SchedulerRunsList,
  SchedulerRunsListPayload,
  SchedulerStatus,
  SchedulerTriggerPayload,
};

/// scheduler service 是调度 UI 的单一前端入口，只委托固定 Rust command，不保存凭据或拼接外部数据源。
export async function schedulerJobsList(): Promise<SchedulerJobsList> {
  return coreSchedulerJobsList();
}

/// 读取单个调度任务配置。
export async function schedulerJobsGet(id: number): Promise<SchedulerJob> {
  return coreSchedulerJobsGet(id);
}

/// 读取后端注册的调度任务类型元数据。
export async function schedulerJobTypes(): Promise<SchedulerJobTypes> {
  return coreSchedulerJobTypes();
}

/// 创建或更新调度任务配置。
export async function schedulerJobsSave(payload: SchedulerJobPayload): Promise<SchedulerJob> {
  return coreSchedulerJobsSave(payload);
}

/// 启用或停用调度任务。
export async function schedulerJobsSetEnabled(id: number, enabled: boolean): Promise<{ id: number; enabled: boolean }> {
  return coreSchedulerJobsSetEnabled(id, enabled);
}

/// 软删除调度任务。
export async function schedulerJobsDelete(id: number): Promise<{ id: number }> {
  return coreSchedulerJobsDelete(id);
}

/// 立即执行一次调度任务。
export async function schedulerJobsRunNow(payload: SchedulerRunNowPayload): Promise<SchedulerRun> {
  return coreSchedulerJobsRunNow(payload);
}

/// 发起有边界的手动补偿抓取。
export async function schedulerJobsBackfill(payload: SchedulerBackfillPayload): Promise<SchedulerBackfillResult> {
  return coreSchedulerJobsBackfill(payload);
}

/// 读取调度执行记录列表。
export async function schedulerRunsList(payload: SchedulerRunsListPayload): Promise<SchedulerRunsList> {
  return coreSchedulerRunsList(payload);
}

/// 读取单条调度执行记录。
export async function schedulerRunsGet(id: number): Promise<SchedulerRun> {
  return coreSchedulerRunsGet(id);
}

/// 手动触发一次调度执行。
export async function schedulerRunsTrigger(payload: SchedulerTriggerPayload): Promise<SchedulerRun> {
  return coreSchedulerRunsTrigger(payload);
}

/// 读取调度摘要状态。
export async function schedulerStatus(): Promise<SchedulerStatus> {
  return coreSchedulerStatus();
}

/// 手动刷新某只股票数据，并复用统一调度执行队列。
export async function schedulerRefreshSymbol(payload: SchedulerRefreshSymbolPayload): Promise<SchedulerRefreshSymbolResult> {
  return coreSchedulerRefreshSymbol(payload);
}
