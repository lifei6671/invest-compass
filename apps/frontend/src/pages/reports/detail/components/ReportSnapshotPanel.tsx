import { CopyOutlined } from "@ant-design/icons";
import { Button } from "antd";
import type { InputSnapshot } from "../types";
import { ReportRiskCard } from "./ReportRiskCard";

type ReportSnapshotPanelProps = {
  snapshot: InputSnapshot | null;
  onCopySnapshot: () => void;
};

export function ReportSnapshotPanel({ snapshot, onCopySnapshot }: ReportSnapshotPanelProps) {
  return (
    <aside className="report-detail-side">
      <section className="report-detail-card report-snapshot-card">
        <div className="report-detail-card-title">
          <h2>输入快照</h2>
          {snapshot ? <Button size="small" type="text" aria-label="复制输入快照" icon={<CopyOutlined />} onClick={onCopySnapshot} /> : null}
        </div>

        {snapshot ? (
          <>
            <RawPromptBox snapshot={snapshot} />
            <BasicSnapshotBox snapshot={snapshot} />
            <StructuredSnapshotSections snapshot={snapshot} />
            <div className="report-snapshot-list">
              <SnapshotRow label="Prompt 模板" value={formatSnapshotValue(snapshot.promptTemplate)} />
              <SnapshotRow label="使用模型" value={formatSnapshotValue(snapshot.model)} />
              <SnapshotRow label="温度" value={formatSnapshotValue(snapshot.temperature)} />
              <SnapshotRow label="最大输出 Token" value={formatSnapshotValue(snapshot.maxTokens)} />
            </div>
          </>
        ) : (
          <div className="report-snapshot-box">
            <h3>暂无输入快照</h3>
            <p>请从已生成的报告记录进入详情页。</p>
          </div>
        )}
      </section>
      <ReportRiskCard />
    </aside>
  );
}

function RawPromptBox(props: { snapshot: InputSnapshot }) {
  const rawPrompt = rawPromptFromSnapshot(props.snapshot);
  if (!rawPrompt) {
    return null;
  }
  return (
    <div className="report-snapshot-box">
      <h3>原始 Prompt</h3>
      <pre className="report-snapshot-prompt">{rawPrompt}</pre>
    </div>
  );
}

function BasicSnapshotBox(props: { snapshot: InputSnapshot }) {
  const rawRows: Array<[string, unknown]> = [
    ["股票代码", props.snapshot.symbol],
    ["分析类型", props.snapshot.analysis_type],
    ["AI 配置 ID", props.snapshot.ai_config_id],
    ["Prompt 模板 ID", props.snapshot.prompt_template_id],
    ["重试来源", props.snapshot.retry_of_task_id],
  ];
  const rows = rawRows.filter(([, value]) => value !== undefined && value !== null && `${value}`.trim() !== "");
  if (rows.length === 0) {
    return null;
  }
  return (
    <div className="report-snapshot-box">
      <h3>任务输入</h3>
      {rows.map(([label, value]) => <SnapshotRow key={label} label={String(label)} value={formatSnapshotValue(value)} />)}
    </div>
  );
}

function StructuredSnapshotSections(props: { snapshot: InputSnapshot }) {
  const { snapshot } = props;
  return (
    <>
      {snapshot.stock ? (
        <div className="report-snapshot-group">
          <span className="report-snapshot-label">股票</span>
          <strong>{formatSnapshotValue(snapshot.stock.name)}</strong>
          <span>{formatSnapshotValue(snapshot.stock.symbol)}</span>
        </div>
      ) : null}

      {snapshot.market ? (
        <div className="report-snapshot-group">
          <span className="report-snapshot-label">市场</span>
          <strong>{formatSnapshotValue(snapshot.market.board)}</strong>
          <span>{formatSnapshotValue(snapshot.market.industry)}</span>
        </div>
      ) : null}

      {snapshot.quote ? (
        <div className="report-snapshot-box">
          <h3>行情摘要（最新）</h3>
          <div className="report-snapshot-quote-grid">
            <span>现价</span>
            <strong className="report-up">{formatSnapshotValue(snapshot.quote.price)}</strong>
            <span>涨跌幅</span>
            <strong className="report-up">{formatSnapshotValue(snapshot.quote.changePercent)}</strong>
            <span>成交额</span>
            <strong>{formatSnapshotValue(snapshot.quote.amount)}</strong>
            <span>换手率</span>
            <strong>{formatSnapshotValue(snapshot.quote.turnoverRate)}</strong>
          </div>
        </div>
      ) : null}

      {snapshot.indicators ? (
        <div className="report-snapshot-box">
          <h3>技术指标摘要</h3>
          <SnapshotRow label="MA5/10/20" value={formatSnapshotValue(snapshot.indicators.ma)} />
          <SnapshotRow label="MACD" value={formatSnapshotValue(snapshot.indicators.macd)} />
          <SnapshotRow label="RSI(12)" value={formatSnapshotValue(snapshot.indicators.rsi)} />
          <SnapshotRow label="KDJ" value={formatSnapshotValue(snapshot.indicators.kdj)} />
          <SnapshotRow label="BOLL(20)" value={formatSnapshotValue(snapshot.indicators.boll)} />
        </div>
      ) : null}

      {snapshot.news ? (
        <div className="report-snapshot-box">
          <h3>新闻资讯</h3>
          <SnapshotRow label="相关新闻数量" value={`${formatSnapshotValue(snapshot.news.count)} 条（${formatSnapshotValue(snapshot.news.period)}）`} />
        </div>
      ) : null}
    </>
  );
}

function SnapshotRow(props: { label: string; value: string }) {
  return (
    <div className="report-snapshot-row">
      <span>{props.label}</span>
      <strong>{props.value}</strong>
    </div>
  );
}

function rawPromptFromSnapshot(snapshot: InputSnapshot): string {
  for (const value of [snapshot.raw_prompt, snapshot.rawPrompt, snapshot.rendered_prompt, snapshot.renderedPrompt, snapshot.prompt]) {
    if (typeof value === "string" && value.trim()) {
      return value.trim();
    }
  }
  return "";
}

function formatSnapshotValue(value: unknown): string {
  if (value === undefined || value === null || value === "") {
    return "—";
  }
  if (typeof value === "string") {
    return value;
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  return JSON.stringify(value);
}
