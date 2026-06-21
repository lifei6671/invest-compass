import type { TopModelItem } from "../types";

type TopModelsCardProps = {
  models: TopModelItem[];
};

export function TopModelsCard({ models }: TopModelsCardProps) {
  return (
    <section className="report-side-card top-models-card">
      <div className="report-side-card-title">
        <h3>常用模型 TOP 5</h3>
      </div>
      <div className="top-model-list">
        {models.map((model, index) => (
          <div className="top-model-row" key={model.name}>
            <div className="top-model-line">
              <span className="top-model-rank">{index + 1}</span>
              <span className="top-model-name">{model.name}</span>
              <span className="top-model-meta">
                {model.count} <em>({model.percent}%)</em>
              </span>
            </div>
            <div className="top-model-progress">
              <span style={{ width: `${model.percent * 2.8}%` }} />
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}
