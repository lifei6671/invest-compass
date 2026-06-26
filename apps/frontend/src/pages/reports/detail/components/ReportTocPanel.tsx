import { UnorderedListOutlined } from "@ant-design/icons";
import { Button } from "antd";
import type { ReportSection } from "../types";

type ReportTocPanelProps = {
  sections: ReportSection[];
  activeSection: string;
  onSectionClick: (section: ReportSection) => void;
  collapsed: boolean;
  onToggleCollapse: () => void;
};

export function ReportTocPanel(props: ReportTocPanelProps) {
  return (
    <aside className="report-detail-card report-toc-panel">
      <div className="report-detail-card-title">
        <h2>目录</h2>
        <Button
          size="small"
          type="text"
          aria-label={props.collapsed ? "展开目录" : "折叠目录"}
          icon={<UnorderedListOutlined />}
          onClick={props.onToggleCollapse}
        />
      </div>
      {props.collapsed ? null : (
        <nav className="report-toc-list" aria-label="报告目录">
          {props.sections.map((section) => (
            <button
              key={section.id}
              type="button"
              className={section.id === props.activeSection ? "report-toc-item report-toc-item-active" : "report-toc-item"}
              onClick={() => props.onSectionClick(section)}
            >
              <span>{section.index}.</span>
              {section.title}
            </button>
          ))}
        </nav>
      )}
    </aside>
  );
}
