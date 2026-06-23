import type { InitializationState } from "./types";

export const initialBootPendingState: InitializationState = {
  progress: 10,
  currentStepId: "sidecar",
  steps: [
    { id: "sidecar", index: 1, title: "启动 Go Core Sidecar", status: "pending", badgeText: "等待连接" },
    { id: "workspace", index: 2, title: "检查本地工作区与目录权限", status: "pending", badgeText: "等待连接" },
    { id: "sqlite_migration", index: 3, title: "SQLite 数据库迁移 / 重建索引", status: "pending", badgeText: "等待连接" },
    { id: "tokenizer", index: 4, title: "加载分词器（GSE / 全文检索词典）", status: "pending", badgeText: "等待连接" },
    { id: "data_source", index: 5, title: "初始化数据源配置", status: "pending", badgeText: "等待连接" },
    { id: "market_cache", index: 6, title: "同步基础行情快照与资讯缓存", status: "pending", badgeText: "等待连接" },
    { id: "ready", index: 7, title: "完成基础检查并进入工作台", status: "pending", badgeText: "等待连接" },
  ],
  taskDetail: {
    taskId: "等待连接",
    elapsed: "等待连接",
    currentStage: "前端启动",
    remaining: "等待连接",
  },
  logs: [
    { id: "1", time: "--:--:--", status: "running", message: "loading frontend workspace" },
    { id: "2", time: "--:--:--", status: "running", message: "waiting app boot status" },
  ],
};
