import { Empty } from "antd";

type ReportMarkdownContentProps = {
  markdown?: string;
};

export function ReportMarkdownContent({ markdown }: ReportMarkdownContentProps) {
  if (!markdown?.trim()) {
    return (
      <article className="report-detail-card report-markdown-content">
        <Empty description="暂无报告正文" />
      </article>
    );
  }

  return (
    <article className="report-detail-card report-markdown-content">
      <pre>{markdown}</pre>
    </article>
  );
}
