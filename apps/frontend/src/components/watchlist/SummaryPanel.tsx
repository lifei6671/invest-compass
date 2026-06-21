import { Card, Tooltip } from "antd";
import { AppstoreOutlined, ArrowDownOutlined, ArrowUpOutlined, ClockCircleOutlined, InfoCircleOutlined, PieChartOutlined } from "@ant-design/icons";
import { EChartView } from "../charts/EChartView";
import { APP_FONT } from "../../styles/fonts";

export function SummaryPanel() {
  const option = {
    animation: false,
    color: ["#1677ff", "#ff8a1f"],
    textStyle: { fontFamily: APP_FONT, fontSize: 13 },
    series: [
      {
        type: "pie",
        radius: ["58%", "78%"],
        center: ["50%", "50%"],
        label: { show: false },
        labelLine: { show: false },
        data: [
          { name: "沪市", value: 26 },
          { name: "深市", value: 30 },
        ],
      },
    ],
  };

  return (
    <aside className="w-[220px] shrink-0 space-y-3">
      <Card className="watchlist-side-card rounded-xl border-[#e5eaf3] shadow-[0_4px_18px_rgba(15,23,42,0.04)]">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="m-0 text-[16px] font-medium text-slate-950">自选概览</h2>
          <Tooltip title="基于本地 mock 自选列表展示">
            <InfoCircleOutlined className="text-slate-400" />
          </Tooltip>
        </div>
        <Metric icon={<AppstoreOutlined />} label="总数量" value="56" valueClass="text-[#1677ff]" />
        <Metric icon={<ArrowUpOutlined />} iconClass="bg-[#fff1f0] text-[#ff4d4f]" label="今日上涨" value="28" hint="(50.00%)" valueClass="text-[#ff4d4f]" />
        <Metric icon={<ArrowDownOutlined />} iconClass="bg-[#ecfdf3] text-[#16a34a]" label="今日下跌" value="21" hint="(37.50%)" valueClass="text-[#16a34a]" />
        <Metric dotClass="bg-slate-300" label="平盘数量" value="7" hint="(12.50%)" valueClass="text-slate-400" />
        <Metric icon={<PieChartOutlined />} label="最近分析数" value="12" hint="近7天生成" valueClass="text-[#635bff]" />
        <div className="mt-3 flex items-start gap-3 border-t border-[#edf1f7] pt-4">
          <ClockCircleOutlined className="mt-1 text-slate-500" />
          <div>
            <div className="text-[14px] text-slate-600">最新更新时间</div>
            <div className="app-number mt-1 text-[18px] font-medium text-slate-900">05-20 15:30:00</div>
          </div>
        </div>
      </Card>
      <Card className="watchlist-side-card rounded-xl border-[#e5eaf3] shadow-[0_4px_18px_rgba(15,23,42,0.04)]">
        <h2 className="m-0 text-[16px] font-medium text-slate-950">市场分布</h2>
        <div className="mt-5 flex items-center gap-3">
          {isJSDOM() ? <div aria-label="市场分布图" className="h-[82px] w-[82px] shrink-0 rounded-full border-[13px] border-[#1677ff] border-b-[#ff8a1f] border-r-[#ff8a1f]" /> : <EChartView option={option} style={{ width: 88, height: 88, flexShrink: 0 }} />}
          <div className="min-w-0 flex-1 space-y-3 text-[11px] text-slate-600">
            <Legend color="#1677ff" label="沪市" value="26 (46.43%)" />
            <Legend color="#ff8a1f" label="深市" value="30 (53.57%)" />
          </div>
        </div>
      </Card>
    </aside>
  );
}

function isJSDOM() {
  return typeof navigator !== "undefined" && navigator.userAgent.toLowerCase().includes("jsdom");
}

function Metric(props: { icon?: React.ReactNode; iconClass?: string; dotClass?: string; label: string; value: string; hint?: string; valueClass: string }) {
  return (
    <div className="mb-4 flex items-center justify-between gap-3">
      <div className="flex min-w-0 items-center gap-3">
        {props.icon ? <span className={["flex h-7 w-7 items-center justify-center rounded-md", props.iconClass ?? "bg-[#eef5ff] text-[#1677ff]"].join(" ")}>{props.icon}</span> : <span className={["ml-2 h-2.5 w-2.5 rounded-full", props.dotClass].join(" ")} />}
        <span className="whitespace-nowrap text-[14px] text-slate-600">{props.label}</span>
      </div>
      <div className="text-right">
        <div className={["app-number text-[20px] font-medium leading-6", props.valueClass].join(" ")}>{props.value}</div>
        {props.hint ? <div className="app-number mt-0.5 whitespace-nowrap text-[12px] text-slate-500">{props.hint}</div> : null}
      </div>
    </div>
  );
}

function Legend(props: { color: string; label: string; value: string }) {
  return (
    <div className="grid grid-cols-[8px_24px_1fr] items-center gap-1.5 whitespace-nowrap">
      <span className="h-2.5 w-2.5 rounded-sm" style={{ backgroundColor: props.color }} />
      <span>{props.label}</span>
      <span className="app-number text-slate-500">{props.value}</span>
    </div>
  );
}
