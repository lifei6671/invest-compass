import { InfoCircleOutlined } from "@ant-design/icons";
import type { PromptVariable } from "../types";

type VariableReferencePanelProps = {
  variables: PromptVariable[];
};

export function VariableReferencePanel({ variables }: VariableReferencePanelProps) {
  return (
    <section className="prompt-card prompt-variable-panel">
      <header className="prompt-card-header prompt-compact-header">
        <h2>变量说明</h2>
        <InfoCircleOutlined />
      </header>
      <div className="prompt-variable-list">
        {variables.map((variable) => (
          <div key={variable.name} className="prompt-variable-row">
            <code>{variable.name}</code>
            <span>{variable.description}</span>
          </div>
        ))}
      </div>
    </section>
  );
}
