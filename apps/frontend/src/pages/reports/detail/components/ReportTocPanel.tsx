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
    <aside className={props.collapsed ? "report-detail-card report-toc-panel report-toc-panel-collapsed" : "report-detail-card report-toc-panel"}>
      <div className="report-detail-card-title">
        {props.collapsed ? null : <h2>目录</h2>}
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
              className={[
                section.id === props.activeSection ? "report-toc-item report-toc-item-active" : "report-toc-item",
                `report-toc-level-${section.level}`,
              ].join(" ")}
              onClick={() => props.onSectionClick(section)}
            >
              {sectionTitle(section)}
            </button>
          ))}
        </nav>
      )}
    </aside>
  );
}

function sectionTitle(section: ReportSection): string {
  return /^\d+([.)、]|\.\d+)/.test(section.title) ? section.title : `${section.index}. ${section.title}`;
}
