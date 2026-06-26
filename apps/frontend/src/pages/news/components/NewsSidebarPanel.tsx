import type { DataSourceStatus, HotIndustry, MentionedStock, SentimentSummary } from "../types";
import { DataSourceStatusCard } from "./DataSourceStatusCard";
import { HotObservationCard } from "./HotObservationCard";

type NewsSidebarPanelProps = {
  industries: HotIndustry[];
  mentionedStocks: MentionedStock[];
  sentiment: SentimentSummary;
  hotTopicsUpdatedAt?: string;
  statsUpdatedAt?: string;
  statuses: DataSourceStatus[];
};

export function NewsSidebarPanel(props: NewsSidebarPanelProps) {
  return (
    <aside className="news-sidebar-panel">
      <HotObservationCard
        industries={props.industries}
        mentionedStocks={props.mentionedStocks}
        sentiment={props.sentiment}
        updatedAt={props.hotTopicsUpdatedAt ?? props.statsUpdatedAt}
      />
      <DataSourceStatusCard statuses={props.statuses} updatedAt={props.statsUpdatedAt} />
    </aside>
  );
}
