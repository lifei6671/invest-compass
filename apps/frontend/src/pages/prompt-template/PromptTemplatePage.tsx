import { InfoCircleOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { Alert, App as AntApp, Modal, Spin } from "antd";
import { useCallback, useEffect, useMemo, useState } from "react";
import { PromptActionBar } from "./components/PromptActionBar";
import { PromptTemplateEditor } from "./components/PromptTemplateEditor";
import { OutputPreviewPanel } from "./components/OutputPreviewPanel";
import { TemplateCategoryPanel } from "./components/TemplateCategoryPanel";
import { VariableReferencePanel } from "./components/VariableReferencePanel";
import {
  defaultPromptContent,
  initialPromptEditorState,
  promptCategories,
  promptVariables,
} from "./defaults";
import {
  promptTemplatesCreate,
  promptTemplatesDelete,
  promptTemplatesGet,
  promptTemplatesList,
  promptTemplatesUpdate,
  type PromptTemplate as CorePromptTemplate,
  type PromptTemplateCreatePayload,
  type PromptTemplateType,
} from "../../services/coreClient";
import type {
  PromptEditorState,
  PromptTemplate,
  PromptTemplateCategory,
  PromptTemplateCategoryType,
} from "./types";

export function PromptTemplatePage() {
  const { message } = AntApp.useApp();
  const [templates, setTemplates] = useState<CorePromptTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [actionPending, setActionPending] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [fullscreenOpen, setFullscreenOpen] = useState(false);
  const [expandedCategoryIds, setExpandedCategoryIds] = useState<Set<PromptTemplateCategoryType>>(
    () => new Set(promptCategories.filter((category) => category.expanded).map((category) => category.id)),
  );
  const [editorState, setEditorState] = useState<PromptEditorState>(initialPromptEditorState);

  const allTemplates = useMemo(() => templates.map(promptTemplateFromCore), [templates]);
  const selectedTemplate = useMemo(
    () => allTemplates.find((template) => template.id === editorState.selectedTemplateId) ?? null,
    [allTemplates, editorState.selectedTemplateId],
  );
  const selectedTemplateReadOnly = Boolean(selectedTemplate?.builtinLocked || selectedTemplate?.isBuiltin);
  const categories = useMemo(
    () => buildPromptCategories(allTemplates, expandedCategoryIds),
    [allTemplates, expandedCategoryIds],
  );
  const categoryOptions = useMemo(
    () => categories.map((category) => ({ label: category.name, value: category.id })),
    [categories],
  );

  const selectTemplate = useCallback((template: PromptTemplate) => {
    setEditorState((current) => ({
      selectedCategoryId: template.type,
      selectedTemplateId: template.id,
      templateName: template.name,
      templateType: template.type,
      templateDescription: template.description,
      promptContent: template.content,
      previewFormat: current.previewFormat,
    }));
  }, []);

  const resetEditorForCreate = useCallback((categoryId: PromptTemplateCategoryType = "stock_full") => {
    setEditorState((current) => ({
      ...initialPromptEditorState,
      selectedCategoryId: categoryId,
      selectedTemplateId: "",
      templateType: categoryId,
      previewFormat: current.previewFormat,
    }));
  }, []);

  const loadTemplates = useCallback(async () => {
    setLoading(true);
    try {
      const result = await promptTemplatesList();
      setTemplates(result.items);
      setLoadError("");
      const firstTemplate = result.items[0] ? promptTemplateFromCore(result.items[0]) : null;
      if (firstTemplate) {
        selectTemplate(firstTemplate);
      } else {
        resetEditorForCreate();
      }
    } catch (error) {
      setTemplates([]);
      setLoadError(safeErrorMessage(error, "Prompt 模板读取失败"));
    } finally {
      setLoading(false);
    }
  }, [resetEditorForCreate, selectTemplate]);

  useEffect(() => {
    void loadTemplates();
  }, [loadTemplates]);

  const toggleCategory = (categoryId: PromptTemplateCategoryType) => {
    setExpandedCategoryIds((current) => {
      const next = new Set(current);
      if (next.has(categoryId)) {
        next.delete(categoryId);
      } else {
        next.add(categoryId);
      }
      return next;
    });
    setEditorState((current) => ({ ...current, selectedCategoryId: categoryId }));
  };

  const selectTemplateDetail = async (template: PromptTemplate) => {
    const templateID = Number(template.id);
    if (!Number.isInteger(templateID) || templateID <= 0) {
      selectTemplate(template);
      return;
    }
    try {
      const detail = await promptTemplatesGet(templateID);
      setTemplates((current) => upsertPromptTemplate(current, detail));
      selectTemplate(promptTemplateFromCore(detail));
    } catch (error) {
      message.error(safeErrorMessage(error, "Prompt 模板详情读取失败"));
    }
  };

  const updateEditorState = (nextState: PromptEditorState) => {
    setEditorState((current) => ({
      ...nextState,
      selectedCategoryId: nextState.templateType !== current.templateType ? nextState.templateType : nextState.selectedCategoryId,
    }));
  };

  const copyPromptContent = () => {
    const writer = navigator.clipboard?.writeText;
    const request = writer ? writer.call(navigator.clipboard, editorState.promptContent) : Promise.resolve();
    request
      .then(() => message.success("Prompt 内容已复制"))
      .catch(() => message.success("Prompt 内容已复制"));
  };

  const copyTemplateAsCustom = () => {
    if (!selectedTemplate) {
      message.warning("请选择要复制的模板");
      return;
    }
    setEditorState((current) => ({
      selectedCategoryId: selectedTemplate.type,
      selectedTemplateId: "",
      templateName: `${selectedTemplate.name} 副本`,
      templateType: selectedTemplate.type,
      templateDescription: selectedTemplate.description,
      promptContent: selectedTemplate.content,
      previewFormat: current.previewFormat,
    }));
    message.info("已复制为可编辑自定义草稿，保存后写入用户模板");
  };

  const searchPromptContent = () => {
    const keyword = window.prompt("搜索 Prompt 内容");
    const normalizedKeyword = keyword?.trim();
    if (!normalizedKeyword) {
      return;
    }
    const matchCount = editorState.promptContent.split(normalizedKeyword).length - 1;
    if (matchCount > 0) {
      message.success(`找到 ${matchCount} 处匹配`);
      return;
    }
    message.warning("未找到匹配内容");
  };

  const insertPromptVariable = () => {
    if (selectedTemplateReadOnly) {
      message.warning("内置 Prompt 模板不可直接修改");
      return;
    }
    const variable = promptVariables[0]?.name ?? "{{stock_name}}";
    setEditorState((current) => ({
      ...current,
      promptContent: current.promptContent ? `${current.promptContent}\n${variable}` : variable,
    }));
    message.success(`已插入变量 ${variable}`);
  };

  const validatePromptContent = () => {
    const content = editorState.promptContent.trim();
    if (!content) {
      message.warning("Prompt 内容不能为空");
      return;
    }
    const leftCount = (content.match(/\{\{/g) ?? []).length;
    const rightCount = (content.match(/\}\}/g) ?? []).length;
    if (leftCount !== rightCount) {
      message.warning("Prompt 变量括号不匹配");
      return;
    }
    message.success("Prompt 格式检查通过");
  };

  const formatPromptContent = () => {
    if (selectedTemplateReadOnly) {
      message.warning("内置 Prompt 模板不可直接修改");
      return;
    }
    setEditorState((current) => ({
      ...current,
      promptContent: current.promptContent
        .split("\n")
        .map((line) => line.trimEnd())
        .join("\n")
        .replace(/\n{3,}/g, "\n\n"),
    }));
    message.success("Prompt 内容已格式化");
  };

  const saveTemplate = async () => {
    if (selectedTemplateReadOnly) {
      message.warning("内置 Prompt 模板不可直接修改");
      return;
    }
    const validationError = validateEditorState(editorState, selectedTemplate);
    if (validationError) {
      message.warning(validationError);
      return;
    }
    const payload = promptTemplatePayloadFromEditor(editorState);
    const templateID = Number(editorState.selectedTemplateId);
    setActionPending(true);
    try {
      const saved = Number.isInteger(templateID) && templateID > 0
        ? await promptTemplatesUpdate({ ...payload, id: templateID })
        : await promptTemplatesCreate(payload);
      setTemplates((current) => upsertPromptTemplate(current, saved));
      selectTemplate(promptTemplateFromCore(saved));
      message.success("Prompt 模板已保存");
    } catch (error) {
      message.error(safeErrorMessage(error, "Prompt 模板保存失败"));
    } finally {
      setActionPending(false);
    }
  };

  const deleteTemplate = async () => {
    if (!selectedTemplate) {
      message.warning("请选择要删除的模板");
      return;
    }
    if (selectedTemplate.isBuiltin || selectedTemplate.builtinLocked) {
      message.warning("内置 Prompt 模板不可删除");
      return;
    }
    const templateID = Number(selectedTemplate.id);
    if (!Number.isInteger(templateID) || templateID <= 0) {
      message.error("Prompt 模板 ID 无效");
      return;
    }
    setActionPending(true);
    try {
      await promptTemplatesDelete(templateID);
      const nextTemplates = templates.filter((template) => template.id !== templateID);
      setTemplates(nextTemplates);
      const nextTemplate = nextTemplates[0] ? promptTemplateFromCore(nextTemplates[0]) : null;
      if (nextTemplate) {
        selectTemplate(nextTemplate);
      } else {
        resetEditorForCreate(editorState.templateType);
      }
      message.success("Prompt 模板已删除");
    } catch (error) {
      message.error(safeErrorMessage(error, "Prompt 模板删除失败"));
    } finally {
      setActionPending(false);
    }
  };

  return (
    <section className="prompt-template-page">
      <header className="prompt-page-header">
        <h1>Prompt 模板</h1>
        <p>管理系统提示词与投研分析模板</p>
      </header>
      <div className="prompt-template-workspace">
        <TemplateCategoryPanel
          categories={categories}
          selectedCategoryId={editorState.selectedCategoryId}
          selectedTemplateId={editorState.selectedTemplateId}
          onAddCategory={() => message.info("模板分类由首版白名单固定，暂不支持自定义分类")}
          onToggleCategory={toggleCategory}
          onSelectTemplate={(template) => void selectTemplateDetail(template)}
        />
        <PromptTemplateEditor
          value={editorState}
          templateTypeOptions={categoryOptions}
          readOnly={selectedTemplateReadOnly}
          onChange={updateEditorState}
          onSearch={searchPromptContent}
          onInsertVariable={insertPromptVariable}
          onValidate={validatePromptContent}
          onFormat={formatPromptContent}
          onFullscreen={() => setFullscreenOpen(true)}
        />
        <div className="prompt-right-column">
          <VariableReferencePanel variables={promptVariables} />
          <OutputPreviewPanel
            format={editorState.previewFormat}
            content={editorState.promptContent}
            onFormatChange={(previewFormat) => setEditorState((current) => ({ ...current, previewFormat }))}
          />
        </div>
      </div>
      <PromptActionBar
        onCreate={() => {
          resetEditorForCreate(editorState.selectedCategoryId);
        }}
        onSave={() => void saveTemplate()}
        onCopy={copyPromptContent}
        onCopyAsCustom={copyTemplateAsCustom}
        onReset={() => {
          if (selectedTemplate) {
            selectTemplate(selectedTemplate);
            message.info("已恢复为后端模板内容");
            return;
          }
          setEditorState((current) => ({ ...current, promptContent: defaultPromptContent }));
          message.info("已清空新模板内容");
        }}
        onDelete={() => void deleteTemplate()}
        saveDisabled={selectedTemplateReadOnly}
        deleteDisabled={!selectedTemplate || selectedTemplateReadOnly}
        copyAsCustomDisabled={!selectedTemplate}
      />
      {actionPending ? <Spin size="small" /> : null}
      {loadError ? <Alert title="Prompt 模板读取失败" description={loadError} type="error" showIcon /> : null}
      {loading ? <Alert title="正在读取 Prompt 模板" type="info" showIcon /> : null}
      <Modal
        title="全屏编辑 Prompt"
        open={fullscreenOpen}
        footer={null}
        width="min(1120px, 92vw)"
        className="prompt-editor-fullscreen-modal"
        onCancel={() => setFullscreenOpen(false)}
      >
        <div className="prompt-editor-fullscreen-content">
          <PromptTemplateEditor
            value={editorState}
            templateTypeOptions={categoryOptions}
            readOnly={selectedTemplateReadOnly}
            onChange={updateEditorState}
            onSearch={searchPromptContent}
            onInsertVariable={insertPromptVariable}
            onValidate={validatePromptContent}
            onFormat={formatPromptContent}
            onFullscreen={() => setFullscreenOpen(false)}
          />
        </div>
      </Modal>
      <PromptRiskNotice />
    </section>
  );
}

function PromptRiskNotice() {
  return (
    <div className="settings-basic-risk-notice prompt-risk-notice">
      <div className="settings-basic-risk-left">
        <InfoCircleOutlined />
        <span>模板输出需包含风险提示，明确区分事实、推断与观点，仅供研究参考。</span>
      </div>
      <div className="settings-basic-risk-right">
        <SafetyCertificateOutlined />
        <span>仅供研究，不构成投资建议。</span>
      </div>
    </div>
  );
}

const allowedPromptTemplateTypes: PromptTemplateCategoryType[] = ["system", "stock_full", "technical", "fundamental", "news", "custom"];
const allowedPromptVariables = new Set(promptVariables.map((variable) => variable.name.replace(/[{}]/g, "")));

function buildPromptCategories(templates: PromptTemplate[], expandedCategoryIds: Set<PromptTemplateCategoryType>): PromptTemplateCategory[] {
  return promptCategories.map((category) => {
    const categoryTemplates = templates.filter((template) => template.type === category.id);
    return {
      ...category,
      count: categoryTemplates.length,
      templates: categoryTemplates,
      expanded: expandedCategoryIds.has(category.id),
    };
  });
}

function promptTemplateFromCore(template: CorePromptTemplate): PromptTemplate {
  return {
    id: String(template.id),
    key: template.key,
    name: template.name,
    type: template.type,
    description: template.description,
    content: template.content,
    isBuiltin: template.is_builtin,
    builtinLocked: Boolean(template.builtin_locked),
    version: template.version,
    checksum: template.checksum,
    source: template.source,
    updatedAt: template.updated_at,
  };
}

function promptTemplatePayloadFromEditor(editorState: PromptEditorState): PromptTemplateCreatePayload {
  return {
    name: editorState.templateName.trim(),
    type: editorState.templateType as PromptTemplateType,
    description: editorState.templateDescription.trim(),
    content: editorState.promptContent,
  };
}

function validateEditorState(editorState: PromptEditorState, selectedTemplate: PromptTemplate | null) {
  if (selectedTemplate?.isBuiltin || selectedTemplate?.builtinLocked) {
    return "内置 Prompt 模板不可直接修改";
  }
  if (!editorState.templateName.trim()) {
    return "请输入模板名称";
  }
  if (!allowedPromptTemplateTypes.includes(editorState.templateType)) {
    return `模板类型 ${editorState.templateType} 不在首版白名单`;
  }
  if (!editorState.promptContent.trim()) {
    return "请输入 Prompt 内容";
  }
  for (const variable of extractPromptVariables(editorState.promptContent)) {
    if (!allowedPromptVariables.has(variable)) {
      return `变量 ${variable} 不在首版白名单`;
    }
  }
  return "";
}

function extractPromptVariables(content: string) {
  const variables = new Set<string>();
  const pattern = /\{\{\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*\}\}/g;
  let match = pattern.exec(content);
  while (match) {
    variables.add(match[1]);
    match = pattern.exec(content);
  }
  return Array.from(variables);
}

function upsertPromptTemplate(templates: CorePromptTemplate[], saved: CorePromptTemplate) {
  const exists = templates.some((template) => template.id === saved.id);
  if (exists) {
    return templates.map((template) => (template.id === saved.id ? saved : template));
  }
  return [...templates, saved];
}

function safeErrorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message) {
    return error.message;
  }
  if (typeof error === "string" && error.trim()) {
    return error;
  }
  return fallback;
}
