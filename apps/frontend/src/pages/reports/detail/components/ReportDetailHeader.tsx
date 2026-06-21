import { Button, Tag } from "antd";
import {
  CopyOutlined,
  DeleteOutlined,
  DownloadOutlined,
  LeftOutlined,
  ReloadOutlined,
  StarFilled,
  StarOutlined,
} from "@ant-design/icons";
import type { ReportDetail } from "../types";

type ReportDetailHeaderProps = {
  report: ReportDetail;
  onBack: () => void;
  onCopy: () => void;
  onExport: () => void;
  onReanalyze: () => void;
  onDelete: () => void;
  onFavoriteToggle: () => void;
};

export function ReportDetailHeader(props: ReportDetailHeaderProps) {
  const favoriteIcon = props.report.favorite ? <StarFilled /> : <StarOutlined />;

  return (
    <header className="report-detail-header">
      <div className="report-detail-header-main">
        <div className="report-detail-title-row">
          <h1>{props.report.title}</h1>
          <Button
            type="text"
            className="report-detail-favorite"
            aria-label={props.report.favorite ? "取消收藏报告" : "收藏报告"}
            icon={favoriteIcon}
            onClick={props.onFavoriteToggle}
          />
        </div>
        <div className="report-detail-meta">
          <span>
            分析类型 <Tag className="report-detail-meta-tag">{props.report.analysisType}</Tag>
          </span>
          <span>
            使用模型 <Tag className="report-detail-meta-tag">{props.report.model}</Tag>
          </span>
          <span>生成时间 {props.report.generatedAt}</span>
          <span>任务 ID {props.report.taskId}</span>
          <span>数据更新时间 {props.report.dataUpdatedAt}</span>
        </div>
      </div>
      <div className="report-detail-actions">
        <Button icon={<LeftOutlined />} onClick={props.onBack}>
          返回列表
        </Button>
        <Button icon={<CopyOutlined />} onClick={props.onCopy}>
          复制 Markdown
        </Button>
        <Button icon={<DownloadOutlined />} onClick={props.onExport}>
          导出 Markdown
        </Button>
        <Button type="primary" icon={<ReloadOutlined />} onClick={props.onReanalyze}>
          重新分析
        </Button>
        <Button danger icon={<DeleteOutlined />} onClick={props.onDelete}>
          删除
        </Button>
      </div>
    </header>
  );
}
