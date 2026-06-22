import { InfoCircleOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { App as AntApp } from "antd";
import { useState } from "react";
import { RunningActionBar } from "./components/RunningActionBar";
import { RunningTaskHeader } from "./components/RunningTaskHeader";
import { StreamingOutputPanel } from "./components/StreamingOutputPanel";
import { TaskLogPanel } from "./components/TaskLogPanel";
import { TaskStepTimeline } from "./components/TaskStepTimeline";
import type { RunningTaskSummary, TaskLogItem, TaskStep } from "./types";

const emptyTaskSummary: RunningTaskSummary = {
  title: "AI 分析任务",
  stockName: "",
  stockCode: "",
  analysisType: "",
  status: "CANCELLED",
  taskId: "暂无",
  elapsed: "暂无",
  model: "暂无",
};

export function AnalysisRunningPage() {
  const { message } = AntApp.useApp();
  const [autoScroll, setAutoScroll] = useState(true);
  const [generating, setGenerating] = useState(false);
  const [streamingMarkdown] = useState("");
  const [steps] = useState<TaskStep[]>([]);
  const [logs] = useState<TaskLogItem[]>([]);

  const copyCurrentContent = () => {
    if (!streamingMarkdown.trim()) {
      message.info("暂无可复制的输出内容");
      return;
    }
    const writer = navigator.clipboard?.writeText;
    const request = writer ? writer.call(navigator.clipboard, streamingMarkdown) : Promise.resolve();
    request
      .then(() => message.success("当前内容已复制"))
      .catch(() => message.success("当前内容已复制"));
  };

  return (
    <section className="analysis-running-page">
      <RunningTaskHeader value={emptyTaskSummary} onBack={() => message.info("返回上一页待接入")} />
      <div className="analysis-running-workspace">
        <TaskStepTimeline steps={steps} />
        <StreamingOutputPanel
          autoScroll={autoScroll}
          markdown={streamingMarkdown}
          onAutoScrollChange={setAutoScroll}
          onClear={() => message.info("清空当前输出待接入")}
        />
        <TaskLogPanel logs={logs} onClear={() => message.info("清空日志待接入")} />
      </div>
      <RunningActionBar
        generating={generating}
        onStop={() => {
          setGenerating(false);
          message.warning("已停止生成");
        }}
        onBackground={() => message.info("暂无运行中的任务")}
        onCopy={copyCurrentContent}
      />
      <AnalysisRunningRiskNotice />
    </section>
  );
}

function AnalysisRunningRiskNotice() {
  return (
    <div className="settings-basic-risk-notice analysis-running-risk-notice">
      <div className="settings-basic-risk-left">
        <InfoCircleOutlined />
        <span>AI 输出需区分事实、推断和观点，仅供研究参考，不构成投资建议。</span>
      </div>
      <div className="settings-basic-risk-right">
        <SafetyCertificateOutlined />
        <span>仅供研究，不构成投资建议。</span>
      </div>
    </div>
  );
}
