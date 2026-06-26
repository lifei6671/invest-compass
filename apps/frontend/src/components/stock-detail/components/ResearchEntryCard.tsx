import { BarChartOutlined, RightOutlined, RobotOutlined } from "@ant-design/icons";

type ResearchEntryCardProps = {
  onStartFull: () => void;
  onStartTechnical: () => void;
};

export function ResearchEntryCard(props: ResearchEntryCardProps) {
  const entries = [
    {
      title: "发起个股综合分析",
      desc: "基本面 + 技术面 + 行业 + 估值综合分析",
      icon: <RobotOutlined />,
      iconClass: "bg-[#f0ecff] text-[#7c3aed]",
      onClick: props.onStartFull,
    },
    {
      title: "发起技术面分析",
      desc: "趋势、形态、指标等技术面分析",
      icon: <BarChartOutlined />,
      iconClass: "bg-[#eaf3ff] text-[#1677ff]",
      onClick: props.onStartTechnical,
    },
  ];

  return (
    <section className="stock-detail-card px-5 py-4">
      <h2 className="m-0 mb-4 text-[16px] font-semibold leading-6 text-[#111827]">研究快捷入口</h2>
      <div className="space-y-3">
        {entries.map((entry) => (
          <button key={entry.title} type="button" className="flex w-full items-center gap-3 rounded-lg border border-[#e5eaf3] bg-white px-3 py-3 text-left transition hover:border-[#b7d3ff] hover:bg-[#f8fbff]" onClick={entry.onClick}>
            <span className={["flex h-8 w-8 shrink-0 items-center justify-center rounded-md text-[16px]", entry.iconClass].join(" ")}>{entry.icon}</span>
            <span className="min-w-0 flex-1">
              <span className="block text-[14px] font-semibold text-[#1f2937]">{entry.title}</span>
              <span className="mt-0.5 block truncate text-[12px] text-[#8a94a6]">{entry.desc}</span>
            </span>
            <RightOutlined className="text-[13px] text-[#8a94a6]" />
          </button>
        ))}
      </div>
    </section>
  );
}
