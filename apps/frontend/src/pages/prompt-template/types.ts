import type { ReactNode } from "react";

export type BuiltinPromptTemplateCategoryType =
  | "system"
  | "stock_full"
  | "technical"
  | "fundamental"
  | "news"
  | "custom";

export type PromptTemplateCategoryType = BuiltinPromptTemplateCategoryType | (string & {});

export type PromptCategoryIconType =
  | "api"
  | "appstore"
  | "bell"
  | "book"
  | "bulb"
  | "chart"
  | "cloud"
  | "code"
  | "compass"
  | "database"
  | "experiment"
  | "file"
  | "fire"
  | "folder"
  | "profile"
  | "rocket"
  | "setting"
  | "star"
  | "tags"
  | "tool";

export type PromptTemplate = {
  id: string;
  key?: string;
  name: string;
  type: PromptTemplateCategoryType;
  description: string;
  content: string;
  isBuiltin: boolean;
  builtinLocked: boolean;
  version?: number;
  checksum?: string;
  source?: string;
  updatedAt?: string;
};

export type PromptTemplateCategory = {
  id: PromptTemplateCategoryType;
  name: string;
  count: number;
  iconType?: PromptCategoryIconType;
  icon?: ReactNode;
  templates?: PromptTemplate[];
  expanded?: boolean;
};

export type PromptVariable = {
  name: string;
  description: string;
  optional?: boolean;
};

export type PromptEditorState = {
  selectedCategoryId: PromptTemplateCategoryType;
  selectedTemplateId: string;
  templateName: string;
  templateType: PromptTemplateCategoryType;
  templateDescription: string;
  promptContent: string;
  previewFormat: "Markdown" | "纯文本";
};
