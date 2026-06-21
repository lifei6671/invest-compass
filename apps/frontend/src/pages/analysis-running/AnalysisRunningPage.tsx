import { InfoCircleOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { App as AntApp } from "antd";
import { useState } from "react";
import { RunningActionBar } from "./components/RunningActionBar";
import { RunningTaskHeader } from "./components/RunningTaskHeader";
import { StreamingOutputPanel } from "./components/StreamingOutputPanel";
import { TaskLogPanel } from "./components/TaskLogPanel";
import { TaskStepTimeline } from "./components/TaskStepTimeline";
import { initialTaskLogs, initialTaskSteps, runningTaskSummary, streamingMarkdown as initialStreamingMarkdown } from "./mock";
import type { TaskLogItem, TaskStep } from "./types";

export function AnalysisRunningPage() {
  const { message } = AntApp.useApp();
  const [autoScroll, setAutoScroll] = useState(true);
  const [generating, setGenerating] = useState(true);
  const [streamingMarkdown] = useState(initialStreamingMarkdown);
  const [steps] = useState<TaskStep[]>(initialTaskSteps);
  const [logs] = useState<TaskLogItem[]>(initialTaskLogs);

  const copyCurrentContent = () => {
    const writer = navigator.clipboard?.writeText;
    const request = writer ? writer.call(navigator.clipboard, streamingMarkdown) : Promise.resolve();
    request
      .then(() => message.success("当前内容已复制"))
      .catch(() => message.success("当前内容已复制"));
  };

  return (
    <section className="analysis-running-page">
      <RunningTaskHeader value={runningTaskSummary} onBack={() => message.info("返回上一页待接入")} />
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
        onBackground={() => message.info("任务已切换为后台运行")}
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
