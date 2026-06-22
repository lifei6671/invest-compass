import { useMemo, useState } from "react";
import { App as AntApp } from "antd";
import { InfoCircleFilled, SafetyCertificateOutlined } from "@ant-design/icons";
import type { DataSourceStatus, HotIndustry, MentionedStock, NewsFilters, NewsItem, SentimentSummary } from "./types";
import { NewsFilterCard } from "./components/NewsFilterCard";
import { NewsListCard } from "./components/NewsListCard";
import { NewsSidebarPanel } from "./components/NewsSidebarPanel";
import { openExternalURL, searchNews, type DocumentSearchItem } from "../../services/coreClient";

const pageSize = 10;
const initialNewsFilters: NewsFilters = {
  keyword: "",
  stock: "全部股票",
  source: "全部来源",
  industry: "全部行业",
  timeRange: "近 7 天",
};
const hotKeywords: string[] = [];
const hotIndustries: HotIndustry[] = [];
const mentionedStocks: MentionedStock[] = [];
const dataSourceStatuses: DataSourceStatus[] = [];
const sentimentSummary: SentimentSummary = {
  positive: { count: 0, percent: 0 },
  neutral: { count: 0, percent: 0 },
  negative: { count: 0, percent: 0 },
  summary: "暂无资讯情绪统计",
};

export function NewsCenterPage() {
  const { message } = AntApp.useApp();
  const [filters, setFilters] = useState<NewsFilters>(initialNewsFilters);
  const [sortMode, setSortMode] = useState("按最新");
  const [currentPage, setCurrentPage] = useState(1);
  const [searchItems, setSearchItems] = useState<NewsItem[]>([]);
  const [remoteSearchMode, setRemoteSearchMode] = useState(false);
  const [isSearching, setIsSearching] = useState(false);

  const filteredItems = useMemo(() => {
    if (remoteSearchMode) {
      return searchItems;
    }
    return [];
  }, [filters, remoteSearchMode, searchItems]);

  const visibleItems = filteredItems.slice((currentPage - 1) * pageSize, currentPage * pageSize);
  const totalCount = filteredItems.length;

  const updateFilters = (patch: Partial<NewsFilters>) => {
    setFilters((current) => ({ ...current, ...patch }));
    setRemoteSearchMode(false);
    setSearchItems([]);
    setCurrentPage(1);
  };

  const handleHotKeywordClick = (keyword: string) => {
    updateFilters({ keyword });
    message.success("已应用热门关键词");
  };

  const handleSortModeChange = (value: string) => {
    setSortMode(value);
    if (value === "按热度") {
      message.info("热度排序待接入");
    }
    if (value === "按相关性") {
      message.info("相关性排序待接入");
    }
  };

  const handleCopySummary = async (item: NewsItem) => {
    await navigator.clipboard.writeText(item.summary);
    message.success("摘要已复制");
  };

  const handleRefresh = async () => {
    const keyword = filters.keyword.trim();
    if (!keyword) {
      setRemoteSearchMode(false);
      setSearchItems([]);
      message.info("请输入关键词后搜索资讯中心");
      return;
    }
    try {
      setIsSearching(true);
      const results = await searchNews({
        keyword,
        symbols: [],
        limit: 20,
        offset: 0,
        sort: "relevance",
      });
      setSearchItems(results.filter((item) => item.doc_type === "news").map(mapSearchNewsItem));
      setRemoteSearchMode(true);
      setCurrentPage(1);
      message.success("资讯搜索已更新");
    } catch (error) {
      message.error(error instanceof Error ? error.message : "资讯搜索失败");
    } finally {
      setIsSearching(false);
    }
  };

  const handleOpenOriginal = async (item: NewsItem) => {
    if (!item.url) {
      message.info("暂无原文链接");
      return;
    }
    try {
      await openExternalURL(item.url);
    } catch (error) {
      message.error(error instanceof Error ? error.message : "打开原文失败");
    }
  };

  return (
    <main className="news-center-page">
      <header className="news-center-title-row">
        <div>
          <h1>资讯中心</h1>
          <p>聚合个股新闻、市场新闻、行业事件与研究线索</p>
        </div>
      </header>

      <NewsFilterCard
        filters={filters}
        hotKeywords={hotKeywords}
        onChange={updateFilters}
        onHotKeywordClick={handleHotKeywordClick}
        onRefresh={handleRefresh}
        isRefreshing={isSearching}
      />

      <div className="news-center-content-grid">
        <NewsListCard
          items={visibleItems}
          totalCount={totalCount}
          sortMode={sortMode}
          currentPage={currentPage}
          onSortModeChange={handleSortModeChange}
          onViewSwitch={() => message.info("视图切换待接入")}
          onPageChange={setCurrentPage}
          onOpenOriginal={handleOpenOriginal}
          onAddContext={() => message.success("已加入 AI 分析上下文")}
          onCopySummary={handleCopySummary}
          emptyDescription="仅搜索资讯中心，暂无匹配资讯"
        />
        <NewsSidebarPanel
          industries={hotIndustries}
          mentionedStocks={mentionedStocks}
          sentiment={sentimentSummary}
          statuses={dataSourceStatuses}
          onCleanCache={() => message.success("资讯缓存已清理")}
        />
      </div>

      <NewsRiskNotice />
    </main>
  );
}

function mapSearchNewsItem(item: DocumentSearchItem): NewsItem {
  const tags = Array.from(new Set([item.symbol, ...item.highlights].map((tag) => tag.trim()).filter(Boolean)));
  return {
    id: item.doc_uid,
    source: item.source || "资讯",
    timeLabel: formatSearchNewsTime(item.source_time),
    title: item.title,
    summary: item.summary,
    tags,
    url: documentSearchRefURL(item.ref_id),
  };
}

function formatSearchNewsTime(value: string): string {
  const date = new Date(value);
  if (!value || Number.isNaN(date.getTime())) {
    return "时间未知";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function documentSearchRefURL(refID: string): string | undefined {
  return /^https?:\/\//i.test(refID) ? refID : undefined;
}

function NewsRiskNotice() {
  return (
    <div className="settings-basic-risk-notice news-center-risk-notice">
      <div className="settings-basic-risk-left">
        <InfoCircleFilled />
        <span>历史资讯仅供研究参考，请结合原始数据独立判断。</span>
      </div>
      <div className="settings-basic-risk-right">
        <SafetyCertificateOutlined />
        <span>仅供研究，不构成投资建议。</span>
      </div>
    </div>
  );
}
