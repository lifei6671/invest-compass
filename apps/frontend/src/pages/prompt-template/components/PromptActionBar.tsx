import { CopyOutlined, DeleteOutlined, PlusOutlined, ReloadOutlined, SaveOutlined } from "@ant-design/icons";
import { Button } from "antd";

type PromptActionBarProps = {
  onCreate: () => void;
  onSave: () => void;
  onCopy: () => void;
  onCopyAsCustom: () => void;
  onReset: () => void;
  onDelete: () => void;
  saveDisabled?: boolean;
  deleteDisabled?: boolean;
  copyAsCustomDisabled?: boolean;
};

export function PromptActionBar(props: PromptActionBarProps) {
  return (
    <div className="prompt-action-bar">
      <Button aria-label="新建模板" type="primary" className="prompt-primary-button" icon={<PlusOutlined />} onClick={props.onCreate}>
        新建模板
      </Button>
      <Button
        aria-label="保存"
        type="primary"
        className="prompt-primary-button"
        icon={<SaveOutlined />}
        disabled={props.saveDisabled}
        onClick={props.onSave}
      >
        保存
      </Button>
      <Button aria-label="复制" className="prompt-secondary-button" icon={<CopyOutlined />} onClick={props.onCopy}>
        复制
      </Button>
      <Button
        aria-label="创建自定义副本"
        className="prompt-secondary-button"
        icon={<CopyOutlined />}
        disabled={props.copyAsCustomDisabled}
        onClick={props.onCopyAsCustom}
      >
        创建自定义副本
      </Button>
      <Button aria-label="恢复默认" className="prompt-secondary-button" icon={<ReloadOutlined />} onClick={props.onReset}>
        恢复默认
      </Button>
      <Button
        aria-label="删除"
        danger
        className="prompt-danger-button"
        icon={<DeleteOutlined />}
        disabled={props.deleteDisabled}
        onClick={props.onDelete}
      >
        删除
      </Button>
    </div>
  );
}
