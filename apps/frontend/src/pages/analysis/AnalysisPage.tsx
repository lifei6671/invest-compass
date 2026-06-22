import { InfoCircleOutlined, LeftOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { App as AntApp, Button } from "antd";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { AnalysisActionBar } from "./components/AnalysisActionBar";
import { AnalysisConfigCard } from "./components/AnalysisConfigCard";
import { DataContextPreview } from "./components/DataContextPreview";
import { OptionalContextCard } from "./components/OptionalContextCard";
import { OutputPreviewCard } from "./components/OutputPreviewCard";
import { emptyContextSummary, initialAnalysisConfig, initialHoldingContext } from "./defaults";
import type { AnalysisConfig, OptionalHoldingContext, OutputFormat } from "./types";

export function AnalysisPage() {
  const { message } = AntApp.useApp();
  const navigate = useNavigate();
  const [config, setConfig] = useState<AnalysisConfig>(initialAnalysisConfig);
  const [holdingContext, setHoldingContext] = useState<OptionalHoldingContext>(initialHoldingContext);
  const [outputFormat, setOutputFormat] = useState<OutputFormat>("Markdown");
  const [generating, setGenerating] = useState(false);

  const copyMarkdown = () => {
    message.info("暂无可复制的分析报告");
  };

  return (
    <section className="analysis-page">
      <header className="analysis-page-header">
        <div>
          <h1>AI 分析</h1>
          <p>基于行情、K线、新闻与技术指标生成研究报告</p>
        </div>
        <Button className="analysis-back-button" icon={<LeftOutlined />} onClick={() => message.info("返回自选待接入")}>
          返回自选
        </Button>
      </header>
      <div className="analysis-workspace">
        <div className="analysis-left-column">
          <AnalysisConfigCard value={config} onChange={setConfig} onManageTemplate={() => message.info("Prompt 模板管理待接入")} />
          <OptionalContextCard value={holdingContext} onChange={setHoldingContext} />
        </div>
        <DataContextPreview value={emptyContextSummary} onViewMoreNews={() => message.info("资讯中心待接入")} />
        <OutputPreviewCard
          markdown=""
          plainText=""
          format={outputFormat}
          onFormatChange={setOutputFormat}
          onFullscreen={() => message.info("全屏预览待接入")}
        />
      </div>
      <AnalysisActionBar
        generating={generating}
        onStart={() => {
          setGenerating(true);
          message.success("开始生成 AI 分析报告");
          navigate("/analysis/running");
        }}
        onStop={() => {
          setGenerating(false);
          message.info("已停止生成");
        }}
        onSave={() => message.info("暂无可保存的分析报告")}
        onCopy={copyMarkdown}
        onExport={() => message.info("导出 Markdown 待接入")}
      />
      <AnalysisRiskNotice />
    </section>
  );
}

function AnalysisRiskNotice() {
  return (
    <div className="settings-basic-risk-notice analysis-risk-notice">
      <div className="settings-basic-risk-left">
        <InfoCircleOutlined />
        <span>AI 输出需区分事实、推断和观点，仅供研究参考。</span>
      </div>
      <div className="settings-basic-risk-right">
        <SafetyCertificateOutlined />
        <span>仅供研究，不构成投资建议。</span>
      </div>
    </div>
  );
}
