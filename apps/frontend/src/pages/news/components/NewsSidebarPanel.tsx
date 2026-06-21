import type { DataSourceStatus, HotIndustry, MentionedStock, SentimentSummary } from "../types";
import { DataSourceStatusCard } from "./DataSourceStatusCard";
import { HotObservationCard } from "./HotObservationCard";

type NewsSidebarPanelProps = {
  industries: HotIndustry[];
  mentionedStocks: MentionedStock[];
  sentiment: SentimentSummary;
  statuses: DataSourceStatus[];
  onCleanCache: () => void;
};

export function NewsSidebarPanel(props: NewsSidebarPanelProps) {
  return (
    <aside className="news-sidebar-panel">
      <HotObservationCard industries={props.industries} mentionedStocks={props.mentionedStocks} sentiment={props.sentiment} />
      <DataSourceStatusCard statuses={props.statuses} onCleanCache={props.onCleanCache} />
    </aside>
  );
}

