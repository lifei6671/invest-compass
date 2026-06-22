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
          <Button size="small" type="text" aria-label="复制输入快照" icon={<CopyOutlined />} disabled={!snapshot} onClick={onCopySnapshot} />
        </div>

        {snapshot ? (
          <>
            <div className="report-snapshot-group">
              <span className="report-snapshot-label">股票</span>
              <strong>{snapshot.stock.name}</strong>
              <span>{snapshot.stock.symbol}</span>
            </div>

            <div className="report-snapshot-group">
              <span className="report-snapshot-label">市场</span>
              <strong>{snapshot.market.board}</strong>
              <span>{snapshot.market.industry}</span>
            </div>

            <div className="report-snapshot-box">
              <h3>行情摘要（最新）</h3>
              <div className="report-snapshot-quote-grid">
                <span>现价</span>
                <strong className="report-up">{snapshot.quote.price}</strong>
                <span>涨跌幅</span>
                <strong className="report-up">{snapshot.quote.changePercent}</strong>
                <span>成交额</span>
                <strong>{snapshot.quote.amount}</strong>
                <span>换手率</span>
                <strong>{snapshot.quote.turnoverRate}</strong>
              </div>
            </div>

            <div className="report-snapshot-box">
              <h3>技术指标摘要</h3>
              <SnapshotRow label="MA5/10/20" value={snapshot.indicators.ma} />
              <SnapshotRow label="MACD" value={snapshot.indicators.macd} />
              <SnapshotRow label="RSI(12)" value={snapshot.indicators.rsi} />
              <SnapshotRow label="KDJ" value={snapshot.indicators.kdj} />
              <SnapshotRow label="BOLL(20)" value={snapshot.indicators.boll} />
            </div>

            <div className="report-snapshot-box">
              <h3>新闻资讯</h3>
              <SnapshotRow label="相关新闻数量" value={`${snapshot.news.count} 条（${snapshot.news.period}）`} />
            </div>

            <div className="report-snapshot-list">
              <SnapshotRow label="Prompt 模板" value={snapshot.promptTemplate} />
              <SnapshotRow label="使用模型" value={snapshot.model} />
              <SnapshotRow label="温度" value={snapshot.temperature} />
              <SnapshotRow label="最大输出 Token" value={snapshot.maxTokens} />
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

function SnapshotRow(props: { label: string; value: string }) {
  return (
    <div className="report-snapshot-row">
      <span>{props.label}</span>
      <strong>{props.value}</strong>
    </div>
  );
}
