import { InfoCircleOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { App as AntApp, Input, Modal, Select, Space } from "antd";
import { useMemo, useState } from "react";
import { PromptActionBar } from "./components/PromptActionBar";
import { PromptTemplateEditor } from "./components/PromptTemplateEditor";
import { OutputPreviewPanel } from "./components/OutputPreviewPanel";
import { TemplateCategoryPanel } from "./components/TemplateCategoryPanel";
import { VariableReferencePanel } from "./components/VariableReferencePanel";
import {
  promptCategoryIconOptions,
  type PromptCategoryIconOption,
} from "./components/promptCategoryIcons";
import {
  defaultPromptContent,
  initialPromptEditorState,
  promptCategories,
  promptVariables,
} from "./defaults";
import type {
  PromptCategoryIconType,
  PromptEditorState,
  PromptTemplate,
  PromptTemplateCategory,
  PromptTemplateCategoryType,
} from "./types";

export function PromptTemplatePage() {
  const { message } = AntApp.useApp();
  const [categories, setCategories] = useState<PromptTemplateCategory[]>(promptCategories);
  const [editorState, setEditorState] = useState<PromptEditorState>(initialPromptEditorState);
  const [categoryModalOpen, setCategoryModalOpen] = useState(false);
  const [newCategoryName, setNewCategoryName] = useState("");
  const [newCategoryIcon, setNewCategoryIcon] = useState<PromptCategoryIconType>("folder");
  const [templateModalOpen, setTemplateModalOpen] = useState(false);
  const [newTemplateName, setNewTemplateName] = useState("");
  const [newTemplateCategoryId, setNewTemplateCategoryId] = useState<PromptTemplateCategoryType>(initialPromptEditorState.selectedCategoryId);

  const allTemplates = useMemo(
    () => categories.flatMap((category) => category.templates ?? []),
    [categories],
  );
  const selectedTemplate = useMemo(
    () => allTemplates.find((template) => template.id === editorState.selectedTemplateId) ?? allTemplates[0],
    [allTemplates, editorState.selectedTemplateId],
  );
  const categoryOptions = useMemo(
    () => categories.map((category) => ({ label: category.name, value: category.id })),
    [categories],
  );

  const toggleCategory = (categoryId: PromptTemplateCategoryType) => {
    setCategories((current) =>
      current.map((category) =>
        category.id === categoryId ? { ...category, expanded: !category.expanded } : category,
      ),
    );
    setEditorState((current) => ({ ...current, selectedCategoryId: categoryId }));
    message.info("切换模板分类");
  };

  const selectTemplate = (template: PromptTemplate) => {
    setEditorState({
      selectedCategoryId: template.type,
      selectedTemplateId: template.id,
      templateName: template.name,
      templateType: template.type,
      templateDescription: template.description,
      promptContent: template.content,
      previewFormat: editorState.previewFormat,
    });
  };

  const copyPromptContent = () => {
    const writer = navigator.clipboard?.writeText;
    const request = writer ? writer.call(navigator.clipboard, editorState.promptContent) : Promise.resolve();
    request
      .then(() => message.success("Prompt 内容已复制"))
      .catch(() => message.success("Prompt 内容已复制"));
  };

  const createCategory = () => {
    const name = newCategoryName.trim();
    if (!name) {
      message.warning("请输入分类名称");
      return;
    }
    const id = `custom-category-${Date.now()}`;
    setCategories((current) => [
      ...current,
      {
        id,
        name,
        count: 0,
        iconType: newCategoryIcon,
        templates: [],
        expanded: true,
      },
    ]);
    setEditorState((current) => ({ ...current, selectedCategoryId: id }));
    setNewTemplateCategoryId(id);
    setNewCategoryName("");
    setNewCategoryIcon("folder");
    setCategoryModalOpen(false);
    message.success("分类已创建");
  };

  const createTemplate = () => {
    const name = newTemplateName.trim();
    if (!name) {
      message.warning("请输入模板名称");
      return;
    }
    const category = categories.find((item) => item.id === newTemplateCategoryId) ?? categories[0];
    const template: PromptTemplate = {
      id: `local-template-${Date.now()}`,
      name,
      type: category.id,
      description: "",
      content: defaultPromptContent,
      isBuiltin: false,
    };
    setCategories((current) =>
      current.map((item) =>
        item.id === category.id
          ? {
              ...item,
              count: item.count + 1,
              expanded: true,
              templates: [...(item.templates ?? []), template],
            }
          : item,
      ),
    );
    setEditorState((current) => ({
      ...current,
      selectedCategoryId: category.id,
      selectedTemplateId: template.id,
      templateName: template.name,
      templateType: template.type,
      templateDescription: template.description,
      promptContent: template.content,
    }));
    setNewTemplateName("");
    setNewTemplateCategoryId(category.id);
    setTemplateModalOpen(false);
    message.success("模板已创建");
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
          onAddCategory={() => setCategoryModalOpen(true)}
          onToggleCategory={toggleCategory}
          onSelectTemplate={selectTemplate}
        />
        <PromptTemplateEditor
          value={editorState}
          templateTypeOptions={categoryOptions}
          onChange={setEditorState}
          onToolAction={(action) => message.info(action)}
          onFormat={() => message.info("格式化待接入")}
          onFullscreen={() => message.info("全屏编辑待接入")}
        />
        <div className="prompt-right-column">
          <VariableReferencePanel variables={promptVariables} />
          <OutputPreviewPanel
            format={editorState.previewFormat}
            content={editorState.promptContent}
            onFormatChange={(previewFormat) => setEditorState((current) => ({ ...current, previewFormat }))}
            onFullscreen={() => message.info("全屏预览待接入")}
          />
        </div>
      </div>
      <PromptActionBar
        onCreate={() => {
          setNewTemplateCategoryId(editorState.selectedCategoryId);
          setTemplateModalOpen(true);
        }}
        onSave={() => message.success("模板已保存")}
        onCopy={copyPromptContent}
        onReset={() => {
          setEditorState((current) => ({ ...current, promptContent: selectedTemplate?.content ?? defaultPromptContent }));
          message.info("恢复默认模板待接入");
        }}
        onDelete={() => message.warning("删除模板待接入")}
      />
      <PromptRiskNotice />
      <Modal
        title="新建分类"
        open={categoryModalOpen}
        okText="创建分类"
        cancelText="取消"
        onOk={createCategory}
        onCancel={() => setCategoryModalOpen(false)}
      >
        <div className="prompt-create-modal-fields">
          <label className="prompt-category-combo-field">
            <span>分类名称</span>
            <Space.Compact block className="prompt-category-combo">
              <Select
                className="prompt-category-icon-select"
                value={newCategoryIcon}
                options={promptCategoryIconOptions.map((option) => ({
                  value: option.value,
                  label: <PromptCategoryIconLabel option={option} />,
                }))}
                popupMatchSelectWidth={220}
                onChange={setNewCategoryIcon}
              />
              <Input
                className="prompt-category-name-input"
                value={newCategoryName}
                placeholder="请输入分类名称"
                onChange={(event) => setNewCategoryName(event.target.value)}
              />
            </Space.Compact>
          </label>
        </div>
      </Modal>
      <Modal
        title="新建模板"
        open={templateModalOpen}
        okText="创建模板"
        cancelText="取消"
        onOk={createTemplate}
        onCancel={() => setTemplateModalOpen(false)}
      >
        <div className="prompt-create-modal-fields">
          <label>
            <span>模板名称</span>
            <Input value={newTemplateName} placeholder="请输入模板名称" onChange={(event) => setNewTemplateName(event.target.value)} />
          </label>
          <label>
            <span>模板分类</span>
            <Select value={newTemplateCategoryId} options={categoryOptions} onChange={setNewTemplateCategoryId} />
          </label>
        </div>
      </Modal>
    </section>
  );
}

function PromptCategoryIconLabel({ option }: { option: PromptCategoryIconOption }) {
  return (
    <span className="prompt-category-icon-option">
      {option.icon}
      <span>{option.label}</span>
    </span>
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
