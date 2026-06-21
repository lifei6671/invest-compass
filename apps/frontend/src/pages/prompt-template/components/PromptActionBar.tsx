import { CopyOutlined, DeleteOutlined, PlusOutlined, ReloadOutlined, SaveOutlined } from "@ant-design/icons";
import { Button } from "antd";

type PromptActionBarProps = {
  onCreate: () => void;
  onSave: () => void;
  onCopy: () => void;
  onReset: () => void;
  onDelete: () => void;
};

export function PromptActionBar(props: PromptActionBarProps) {
  return (
    <div className="prompt-action-bar">
      <Button type="primary" className="prompt-primary-button" icon={<PlusOutlined />} onClick={props.onCreate}>
        新建模板
      </Button>
      <Button type="primary" className="prompt-primary-button" icon={<SaveOutlined />} onClick={props.onSave}>
        保存
      </Button>
      <Button className="prompt-secondary-button" icon={<CopyOutlined />} onClick={props.onCopy}>
        复制
      </Button>
      <Button className="prompt-secondary-button" icon={<ReloadOutlined />} onClick={props.onReset}>
        恢复默认
      </Button>
      <Button danger className="prompt-danger-button" icon={<DeleteOutlined />} onClick={props.onDelete}>
        删除
      </Button>
    </div>
  );
}
