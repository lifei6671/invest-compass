import { Button } from "antd";
import type { AIContextDataType, AIOutputNature } from "../types";

type AIContextExplanationCardProps = {
  dataTypes: AIContextDataType[];
  outputNatures: AIOutputNature[];
  onViewCredentials: () => void;
};

export function AIContextExplanationCard({ dataTypes, outputNatures, onViewCredentials }: AIContextExplanationCardProps) {
  return (
    <section className="data-description-card data-description-small-card">
      <h2>C. AI 上下文说明</h2>
      <p className="data-description-card-copy">AI 分析会基于以下数据构建上下文，以生成研究结论与解读。</p>
      <div className="data-description-ai-tags">
        {dataTypes.map((type) => (
          <span key={type}>{type}</span>
        ))}
      </div>
      <p className="data-description-nature-title">AI 输出内容按性质区分为：</p>
      <div className="data-description-nature-list">
        {outputNatures.map((item) => (
          <div key={item.type} className="data-description-nature-row">
            <span className={`data-description-nature-dot data-description-nature-${item.color}`} />
            <strong>{item.type}：</strong>
            <span>{item.description}</span>
          </div>
        ))}
      </div>
      <Button className="data-description-link-button data-description-side-link" type="text" onClick={onViewCredentials}>
        查看凭据管理 &gt;
      </Button>
    </section>
  );
}
