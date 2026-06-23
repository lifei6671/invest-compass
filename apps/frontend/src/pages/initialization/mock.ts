import type { InitializationState } from "./types";

export const mockInitializationState: InitializationState = {
  progress: 68,
  currentStepId: "sqlite_migration",
  steps: [
    { id: "core_sidecar", index: 1, title: "启动 Go Core Sidecar", status: "completed", badgeText: "已完成" },
    { id: "workspace_permission", index: 2, title: "检查本地工作区与目录权限", status: "completed", badgeText: "已完成" },
    { id: "sqlite_migration", index: 3, title: "SQLite 数据库迁移 / 重建索引", status: "running", badgeText: "进行中" },
    { id: "tokenizer", index: 4, title: "加载分词器（GSE / 全文检索词典）", status: "pending", badgeText: "等待中" },
    { id: "data_source", index: 5, title: "初始化数据源配置", status: "pending", badgeText: "等待中" },
    { id: "base_cache", index: 6, title: "同步基础行情快照与资讯缓存", status: "pending", badgeText: "等待中" },
    { id: "enter_workspace", index: 7, title: "完成基础检查并进入工作台", status: "pending", badgeText: "等待中" },
  ],
  taskDetail: {
    taskId: "5d3c7b7e-2f6a-4e2f-a8a6-3e9f1c6b7d92",
    elapsed: "00:01:24",
    currentStage: "SQLite 数据库迁移 / 重建索引",
    remaining: "00:00:37",
  },
  logs: [
    { id: "1", time: "15:29:41", status: "success", message: "sidecar ready" },
    { id: "2", time: "15:29:42", status: "success", message: "workspace opened: InvestCompass" },
    { id: "3", time: "15:29:43", status: "running", message: "sqlite migrate start" },
    { id: "4", time: "15:29:44", status: "running", message: "rebuilding index: quotes, news_items" },
    { id: "5", time: "15:29:45", status: "pending", message: "waiting tokenizer initialization" },
  ],
};
