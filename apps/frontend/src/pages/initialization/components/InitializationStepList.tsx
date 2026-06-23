import { CheckOutlined } from "@ant-design/icons";
import type { InitializationStep, InitializationStepStatus } from "../types";

type InitializationStepListProps = {
  steps: InitializationStep[];
};

export function InitializationStepList(props: InitializationStepListProps) {
  return (
    <section className="overflow-hidden rounded-[10px] border border-[#e5eaf3] bg-white">
      {props.steps.map((step, index) => (
        <div key={step.id} className="relative grid min-h-12 grid-cols-[34px_1fr_auto] items-center gap-3 border-b border-[#edf1f7] px-4 last:border-b-0">
          {index < props.steps.length - 1 ? (
            <span
              aria-hidden="true"
              className={[
                "absolute left-[30px] top-8 h-8 w-px",
                step.status === "completed" ? "bg-[#16a34a]" : step.status === "running" ? "bg-[#1677ff]" : "bg-[#d1d5db]",
              ].join(" ")}
            />
          ) : null}
          <StepMarker step={step} />
          <span className={["text-[14px] font-medium", step.status === "running" ? "text-[#111827]" : "text-[#374151]"].join(" ")}>
            {step.index}.&nbsp; {step.title}
          </span>
          <span className={["rounded-md border px-3 py-1 text-[12px] leading-4", badgeClassName(step.status)].join(" ")}>{step.badgeText}</span>
        </div>
      ))}
    </section>
  );
}

function StepMarker(props: { step: InitializationStep }) {
  if (props.step.status === "completed") {
    return (
      <span className="relative z-[1] flex h-5 w-5 items-center justify-center rounded-full bg-[#16a34a] text-[11px] text-white">
        <CheckOutlined />
      </span>
    );
  }
  if (props.step.status === "running") {
    return (
      <span className="relative z-[1] flex h-5 w-5 items-center justify-center rounded-full border-2 border-[#1677ff] bg-white text-[12px] font-semibold text-[#1677ff]">
        {props.step.index}
      </span>
    );
  }
  return (
    <span className="relative z-[1] flex h-5 w-5 items-center justify-center rounded-full border border-[#cbd5e1] bg-white text-[12px] font-semibold text-[#64748b]">
      {props.step.index}
    </span>
  );
}

function badgeClassName(status: InitializationStepStatus) {
  if (status === "completed") {
    return "border-[#bbf7d0] bg-[#ecfdf3] text-[#16a34a]";
  }
  if (status === "running") {
    return "border-[#bfdbfe] bg-[#eff6ff] text-[#1677ff]";
  }
  if (status === "failed") {
    return "border-[#fecaca] bg-[#fff1f2] text-[#ff4d4f]";
  }
  return "border-[#e5e7eb] bg-[#f8fafc] text-[#64748b]";
}
