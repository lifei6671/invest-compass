import { useCallback, useEffect, useMemo, useState } from "react";
import { App as AntApp } from "antd";
import { InfoCircleFilled, SafetyCertificateOutlined } from "@ant-design/icons";
import type { DataSourceStatus, HotIndustry, MentionedStock, NewsFilters, NewsItem, SentimentSummary } from "./types";
import { NewsFilterCard } from "./components/NewsFilterCard";
import { NewsListCard } from "./components/NewsListCard";
import { NewsSidebarPanel } from "./components/NewsSidebarPanel";
import {
  newsHotTopics,
  newsMarket,
  newsStats,
  openExternalURL,
  searchNews,
  type DocumentSearchItem,
  type NewsHotTopicsResult,
  type NewsItem as CoreNewsItem,
  type NewsStatsResult,
} from "../../services/coreClient";
import { DEFAULT_PAGE_SIZE } from "../../lib/pagination";

const initialNewsFilters: NewsFilters = {
  keyword: "",
  stock: "全部股票",
  source: "全部来源",
  industry: "全部行业",
  timeRange: "近 7 天",
};
const hotKeywords: string[] = [];
const dataSourceStatuses: DataSourceStatus[] = [];
const emptySentimentSummary: SentimentSummary = {
  positive: { count: 0, percent: 0 },
  neutral: { count: 0, percent: 0 },
  negative: { count: 0, percent: 0 },
  summary: "暂无资讯情绪统计",
};
const defaultMarket = "CN";
const newsQueryLimit = 80;

export function NewsCenterPage() {
  const { message } = AntApp.useApp();
  const [filters, setFilters] = useState<NewsFilters>(initialNewsFilters);
  const [sortMode, setSortMode] = useState("按最新");
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE);
  const [searchItems, setSearchItems] = useState<NewsItem[]>([]);
  const [remoteSearchMode, setRemoteSearchMode] = useState(false);
  const [isSearching, setIsSearching] = useState(false);
  const [hotIndustries, setHotIndustries] = useState<HotIndustry[]>([]);
  const [mentionedStocks, setMentionedStocks] = useState<MentionedStock[]>([]);
  const [sentimentSummary, setSentimentSummary] = useState<SentimentSummary>(emptySentimentSummary);
  const [hotTopicsUpdatedAt, setHotTopicsUpdatedAt] = useState<string>();
  const [statsUpdatedAt, setStatsUpdatedAt] = useState<string>();

  const filteredItems = useMemo(() => {
    if (remoteSearchMode) {
      return searchItems;
    }
    return searchItems.filter((item) => {
      if (filters.source !== "全部来源" && item.source !== filters.source) {
        return false;
      }
      if (filters.stock !== "全部股票" && !item.tags.some((tag) => tag.includes(filters.stock))) {
        return false;
      }
      if (filters.industry !== "全部行业" && !item.tags.some((tag) => tag.includes(filters.industry))) {
        return false;
      }
      return true;
    });
  }, [filters, remoteSearchMode, searchItems]);

  const visibleItems = filteredItems.slice((currentPage - 1) * pageSize, currentPage * pageSize);
  const totalCount = filteredItems.length;

  useEffect(() => {
    const maxPage = Math.max(1, Math.ceil(totalCount / pageSize));
    if (currentPage > maxPage) {
      setCurrentPage(maxPage);
    }
  }, [currentPage, pageSize, totalCount]);

  const updateFilters = (patch: Partial<NewsFilters>) => {
    setFilters((current) => ({ ...current, ...patch }));
    setRemoteSearchMode(false);
    setCurrentPage(1);
  };

  const handleHotKeywordClick = (keyword: string) => {
    updateFilters({ keyword });
    message.success("已应用热门关键词");
  };

  const handleSortModeChange = (value: string) => {
    setSortMode(value);
  };

  const handlePageChange = (page: number, nextPageSize: number) => {
    setPageSize(nextPageSize);
    setCurrentPage(nextPageSize === pageSize ? page : 1);
  };

  const handleCopySummary = async (item: NewsItem) => {
    await navigator.clipboard.writeText(item.summary);
    message.success("摘要已复制");
  };

  const loadMarketNews = useCallback(async (options?: { forceRefresh?: boolean }) => {
    try {
      setIsSearching(true);
      const result = await newsMarket({
        market: defaultMarket,
        limit: newsQueryLimit,
        ...(options?.forceRefresh ? { forceRefresh: true } : {}),
      });
      setSearchItems(result.items.map(mapCoreNewsItem));
      setRemoteSearchMode(false);
      setCurrentPage(1);
    } catch (error) {
      message.error(error instanceof Error ? error.message : "市场资讯加载失败");
    } finally {
      setIsSearching(false);
    }
  }, [message]);

  const loadNewsInsights = useCallback(async () => {
    try {
      const [stats, topics] = await Promise.all([
        newsStats({ market: defaultMarket, limit: newsQueryLimit }),
        newsHotTopics({ market: defaultMarket, limit: newsQueryLimit }),
      ]);
      setSentimentSummary(mapNewsStatsToSentiment(stats));
      setHotIndustries(mapHotIndustries(topics));
      setMentionedStocks(mapMentionedStocks(topics));
      setStatsUpdatedAt(formatUpdateTime(stats.latest_published_at));
      setHotTopicsUpdatedAt(formatUpdateTime(topics.updated_at));
    } catch (error) {
      message.error(error instanceof Error ? error.message : "资讯统计加载失败");
    }
  }, [message]);

  useEffect(() => {
    void loadMarketNews();
    void loadNewsInsights();
  }, [loadMarketNews, loadNewsInsights]);

  const handleRefresh = async () => {
    const keyword = filters.keyword.trim();
    if (!keyword) {
      await loadMarketNews({ forceRefresh: true });
      await loadNewsInsights();
      return;
    }
    try {
      setIsSearching(true);
      const results = await searchNews({
        keyword,
        symbols: [],
        limit: newsQueryLimit,
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
          pageSize={pageSize}
          onSortModeChange={handleSortModeChange}
          onPageChange={handlePageChange}
          onOpenOriginal={handleOpenOriginal}
          onCopySummary={handleCopySummary}
          emptyDescription={remoteSearchMode ? "仅搜索资讯中心，暂无匹配资讯" : "暂无市场资讯"}
        />
        <NewsSidebarPanel
          industries={hotIndustries}
          mentionedStocks={mentionedStocks}
          sentiment={sentimentSummary}
          hotTopicsUpdatedAt={hotTopicsUpdatedAt}
          statsUpdatedAt={statsUpdatedAt}
          statuses={dataSourceStatuses}
        />
      </div>

      <NewsRiskNotice />
    </main>
  );
}

function mapCoreNewsItem(item: CoreNewsItem): NewsItem {
  const tags = Array.from(new Set([...(item.symbols ?? []), ...(item.tags ?? [])].map((tag) => tag.trim()).filter(Boolean)));
  return {
    id: String(item.id),
    source: item.source || "资讯",
    timeLabel: formatNewsTime(item.published_at),
    title: item.title,
    summary: item.summary || "",
    tags,
    url: normalizeNewsURL(item.url),
  };
}

function mapSearchNewsItem(item: DocumentSearchItem): NewsItem {
  const tags = Array.from(new Set([item.symbol, ...item.highlights].map((tag) => tag.trim()).filter(Boolean)));
  return {
    id: item.doc_uid,
    source: item.source || "资讯",
    timeLabel: formatNewsTime(item.source_time),
    title: item.title,
    summary: item.summary,
    tags,
    url: normalizeNewsURL(item.ref_id),
  };
}

function formatNewsTime(value?: string): string {
  if (!value) {
    return "时间未知";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "时间未知";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function mapNewsStatsToSentiment(stats: NewsStatsResult): SentimentSummary {
  return {
    positive: { count: 0, percent: 0 },
    neutral: { count: 0, percent: 0 },
    negative: { count: 0, percent: 0 },
    summary: stats.sentiment_summary || "暂未接入情绪分类，当前仅展示新闻缓存数量、来源和标签统计。",
  };
}

function mapHotIndustries(result: NewsHotTopicsResult): HotIndustry[] {
  const maxCount = Math.max(1, ...result.industries.map((item) => item.count));
  return result.industries.map((item, index) => ({
    rank: index + 1,
    name: item.name,
    heat: Math.max(1, Math.round((item.count / maxCount) * 100)),
  }));
}

function mapMentionedStocks(result: NewsHotTopicsResult): MentionedStock[] {
  return result.mentioned_stocks.map((item) => ({
    name: item.symbol,
    count: item.count,
  }));
}

function formatUpdateTime(value?: string): string | undefined {
  if (!value) {
    return undefined;
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return undefined;
  }
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function normalizeNewsURL(value?: string): string | undefined {
  return value && /^https?:\/\//i.test(value) ? value : undefined;
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
