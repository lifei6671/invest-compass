import { CheckOutlined, LoadingOutlined } from "@ant-design/icons";
import type { TaskStep } from "../types";

type TaskStepTimelineProps = {
  steps: TaskStep[];
};

export function TaskStepTimeline({ steps }: TaskStepTimelineProps) {
  return (
    <section className="analysis-running-card analysis-running-step-card">
      <h2 className="analysis-running-card-title">任务步骤</h2>
      <div className="analysis-running-step-list">
        {steps.map((step, index) => (
          <div key={step.id} className={`analysis-running-step analysis-running-step-${step.status}`}>
            <div className="analysis-running-step-rail">
              <span className="analysis-running-step-dot">
                {step.status === "success" ? <CheckOutlined /> : step.id}
              </span>
              {index < steps.length - 1 ? <span className="analysis-running-step-line" /> : null}
            </div>
            <div className="analysis-running-step-content">
              <div className="analysis-running-step-title-row">
                <h3>{step.title}</h3>
                {step.time ? <time>{step.time}</time> : null}
              </div>
              <p>{step.description}</p>
            </div>
            {step.status === "running" ? <LoadingOutlined className="analysis-running-step-spinner" /> : null}
          </div>
        ))}
      </div>
    </section>
  );
}
