import {
  ApiOutlined,
  AppstoreOutlined,
  BellOutlined,
  BookOutlined,
  BulbOutlined,
  CloudOutlined,
  CodeOutlined,
  CompassOutlined,
  DatabaseOutlined,
  ExperimentOutlined,
  FileTextOutlined,
  FireOutlined,
  FolderOutlined,
  LineChartOutlined,
  ProfileOutlined,
  RocketOutlined,
  SettingOutlined,
  StarOutlined,
  TagsOutlined,
  ToolOutlined,
} from "@ant-design/icons";
import type { ReactNode } from "react";
import type {
  BuiltinPromptTemplateCategoryType,
  PromptCategoryIconType,
  PromptTemplateCategory,
} from "../types";

export type PromptCategoryIconOption = {
  label: string;
  value: PromptCategoryIconType;
  icon: ReactNode;
};

export const promptCategoryIconOptions: PromptCategoryIconOption[] = [
  { label: "文件夹", value: "folder", icon: <FolderOutlined aria-hidden /> },
  { label: "文件", value: "file", icon: <FileTextOutlined aria-hidden /> },
  { label: "图表", value: "chart", icon: <LineChartOutlined aria-hidden /> },
  { label: "代码", value: "code", icon: <CodeOutlined aria-hidden /> },
  { label: "设置", value: "setting", icon: <SettingOutlined aria-hidden /> },
  { label: "书籍", value: "book", icon: <BookOutlined aria-hidden /> },
  { label: "数据库", value: "database", icon: <DatabaseOutlined aria-hidden /> },
  { label: "通知", value: "bell", icon: <BellOutlined aria-hidden /> },
  { label: "火箭", value: "rocket", icon: <RocketOutlined aria-hidden /> },
  { label: "云端", value: "cloud", icon: <CloudOutlined aria-hidden /> },
  { label: "星标", value: "star", icon: <StarOutlined aria-hidden /> },
  { label: "标签", value: "tags", icon: <TagsOutlined aria-hidden /> },
  { label: "接口", value: "api", icon: <ApiOutlined aria-hidden /> },
  { label: "应用", value: "appstore", icon: <AppstoreOutlined aria-hidden /> },
  { label: "罗盘", value: "compass", icon: <CompassOutlined aria-hidden /> },
  { label: "实验", value: "experiment", icon: <ExperimentOutlined aria-hidden /> },
  { label: "热点", value: "fire", icon: <FireOutlined aria-hidden /> },
  { label: "灵感", value: "bulb", icon: <BulbOutlined aria-hidden /> },
  { label: "文档", value: "profile", icon: <ProfileOutlined aria-hidden /> },
  { label: "工具", value: "tool", icon: <ToolOutlined aria-hidden /> },
];

const iconByType = new Map(promptCategoryIconOptions.map((option) => [option.value, option.icon]));

const builtinIconByCategory: Record<BuiltinPromptTemplateCategoryType, PromptCategoryIconType> = {
  system: "setting",
  stock_full: "chart",
  technical: "code",
  fundamental: "profile",
  news: "fire",
  custom: "folder",
};

export function getPromptCategoryIcon(category: PromptTemplateCategory) {
  const iconType =
    category.iconType ?? builtinIconByCategory[category.id as BuiltinPromptTemplateCategoryType] ?? "folder";
  return iconByType.get(iconType) ?? <FolderOutlined aria-hidden />;
}
