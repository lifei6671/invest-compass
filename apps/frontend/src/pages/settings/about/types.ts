import type { ReactNode } from "react";

export type AppInfoItem = {
  key: string;
  label: string;
  value: string;
  status?: "normal" | "warning" | "error";
  icon?: ReactNode;
};

export type UpdateInfo = {
  currentVersion: string;
  latestVersion: string;
  updateStatus: "latest" | "available" | "failed";
  releaseDate: string;
};

export type LicenseInfo = {
  status: "FREE" | "TRIAL" | "PRO" | "EXPIRED";
  description: string;
  licenseType: string;
  expiresAt: string;
};

export type ResourceLink = {
  title: string;
  description: string;
  actionText: string;
  type: "license" | "manual" | "logs";
};

export const appInfoItems: AppInfoItem[] = [
  {
    key: "desktop",
    label: "桌面端框架",
    value: "Tauri v2",
  },
  {
    key: "frontend",
    label: "前端技术",
    value: "React + TypeScript",
  },
  {
    key: "core",
    label: "核心服务",
    value: "Go Core 已连接",
    status: "normal",
  },
  {
    key: "database",
    label: "数据库",
    value: "SQLite 正常",
    status: "normal",
  },
  {
    key: "workspace",
    label: "当前工作区",
    value: "C:\\Users\\InvestCompass\\Documents\\InvestCompass",
  },
  {
    key: "logs",
    label: "日志目录",
    value: "C:\\Users\\InvestCompass\\AppData\\Local\\InvestCompass\\logs",
  },
];

export const updateInfo: UpdateInfo = {
  currentVersion: "v0.1.0",
  latestVersion: "v0.1.0",
  updateStatus: "latest",
  releaseDate: "2025-05-18",
};

export const licenseInfo: LicenseInfo = {
  status: "FREE",
  description: "首版仅展示授权状态，不提供激活流程与功能限制。",
  licenseType: "个人非商用",
  expiresAt: "—",
};

export const resourceLinks: ResourceLink[] = [
  {
    title: "开源许可证",
    description: "本项目遵循开源许可证协议，欢迎查看许可信息。",
    actionText: "查看 LICENSE",
    type: "license",
  },
  {
    title: "用户手册",
    description: "查看使用指南、功能说明与常见问题。",
    actionText: "打开用户手册",
    type: "manual",
  },
  {
    title: "日志与诊断",
    description: "导出应用日志，用于问题排查与反馈。",
    actionText: "导出日志",
    type: "logs",
  },
];
