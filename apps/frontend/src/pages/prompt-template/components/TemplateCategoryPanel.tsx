import { PlusOutlined } from "@ant-design/icons";
import { Button, Menu } from "antd";
import type { MenuProps } from "antd";
import { getPromptCategoryIcon } from "./promptCategoryIcons";
import type {
  PromptTemplate,
  PromptTemplateCategory,
  PromptTemplateCategoryType,
} from "../types";

type TemplateCategoryPanelProps = {
  categories: PromptTemplateCategory[];
  selectedCategoryId: PromptTemplateCategoryType;
  selectedTemplateId: string;
  onAddCategory: () => void;
  onToggleCategory: (categoryId: PromptTemplateCategoryType) => void;
  onSelectTemplate: (template: PromptTemplate) => void;
};

export function TemplateCategoryPanel(props: TemplateCategoryPanelProps) {
  const templateById = new Map<string, PromptTemplate>();

  props.categories.forEach((category) => {
    category.templates?.forEach((template) => {
      templateById.set(template.id, template);
    });
  });

  const menuItems: MenuProps["items"] = props.categories.map((category) => {
    const label = (
      <span className="prompt-category-menu-label">
        <span>{category.name}</span>
        <span className="prompt-category-count">{category.count}</span>
      </span>
    );

    if (!category.templates?.length) {
      return {
        key: category.id,
        icon: getPromptCategoryIcon(category),
        label,
      };
    }

    return {
      key: category.id,
      icon: getPromptCategoryIcon(category),
      label,
      children: category.templates.map((template) => ({
        key: template.id,
        label: (
          <span className="prompt-template-menu-label">
            <span>{template.name}</span>
          </span>
        ),
      })),
    };
  });

  const openKeys = props.categories.filter((category) => category.expanded && category.templates?.length).map((category) => category.id);

  const handleOpenChange: MenuProps["onOpenChange"] = (keys) => {
    const openKeySet = new Set(keys);
    props.categories.forEach((category) => {
      if (!category.templates?.length) {
        return;
      }

      const shouldOpen = openKeySet.has(category.id);
      if (Boolean(category.expanded) !== shouldOpen) {
        props.onToggleCategory(category.id);
      }
    });
  };

  const handleMenuClick: MenuProps["onClick"] = ({ key }) => {
    const template = templateById.get(key);
    if (template) {
      props.onSelectTemplate(template);
      return;
    }

    props.onToggleCategory(key as PromptTemplateCategoryType);
  };

  return (
    <section className="prompt-card prompt-category-panel">
      <header className="prompt-card-header">
        <h2>模板分类</h2>
        <Button aria-label="新增分类" size="small" className="prompt-icon-button" icon={<PlusOutlined />} onClick={props.onAddCategory} />
      </header>
      <div className="prompt-category-list">
        <Menu
          className="prompt-category-menu"
          inlineIndent={16}
          items={menuItems}
          mode="inline"
          openKeys={openKeys}
          selectedKeys={props.selectedTemplateId ? [props.selectedTemplateId] : [props.selectedCategoryId]}
          onClick={handleMenuClick}
          onOpenChange={handleOpenChange}
        />
      </div>
    </section>
  );
}
