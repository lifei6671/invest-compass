import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { App as AntApp } from "antd";
import { InfoCircleFilled, SafetyCertificateOutlined } from "@ant-design/icons";
import { useLocation } from "react-router-dom";
import type { DataSourceStatus, HotIndustry, MentionedStock, NewsFilters, NewsItem, SentimentSummary } from "./types";
import { NewsFilterCard, type NewsStockOption } from "./components/NewsFilterCard";
import { NewsListCard } from "./components/NewsListCard";
import { NewsSidebarPanel } from "./components/NewsSidebarPanel";
import {
  newsHotTopics,
  newsList,
  newsMarket,
  newsStats,
  openExternalURL,
  searchNews,
  stockSearch,
  watchlistList,
  type DocumentSearchItem,
  type NewsHotTopicsResult,
  type NewsItem as CoreNewsItem,
  type NewsStatsResult,
  type StockSearchResult,
  type WatchlistItem,
} from "../../services/coreClient";
import { DEFAULT_PAGE_SIZE } from "../../lib/pagination";

const initialNewsFilters: NewsFilters = {
  keyword: "",
  stock: "全部股票",
  stockSymbol: "",
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
  const location = useLocation();
  const initialRouteStock = useMemo(() => routeStockFromLocation(location), [location]);
  const [filters, setFilters] = useState<NewsFilters>(() => ({
    ...initialNewsFilters,
    ...(initialRouteStock ? { stock: initialRouteStock.label, stockSymbol: initialRouteStock.value } : {}),
  }));
  const [sortMode, setSortMode] = useState("按最新");
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE);
  const [searchItems, setSearchItems] = useState<NewsItem[]>([]);
  const [remoteSearchMode, setRemoteSearchMode] = useState(false);
  const [isSearching, setIsSearching] = useState(false);
  const [watchlistStockOptions, setWatchlistStockOptions] = useState<NewsStockOption[]>([]);
  const [searchedStockOptions, setSearchedStockOptions] = useState<NewsStockOption[]>([]);
  const [hotIndustries, setHotIndustries] = useState<HotIndustry[]>([]);
  const [mentionedStocks, setMentionedStocks] = useState<MentionedStock[]>([]);
  const [sentimentSummary, setSentimentSummary] = useState<SentimentSummary>(emptySentimentSummary);
  const [hotTopicsUpdatedAt, setHotTopicsUpdatedAt] = useState<string>();
  const [statsUpdatedAt, setStatsUpdatedAt] = useState<string>();
  const newsRequestSeqRef = useRef(0);

  const filteredItems = useMemo(() => {
    if (remoteSearchMode) {
      return searchItems;
    }
    return searchItems.filter((item) => {
      if (filters.source !== "全部来源" && item.source !== filters.source) {
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

  const stockOptions = useMemo(
    () => mergeStockOptions([{ value: "全部股票", label: "全部股票" }, initialRouteStock, ...watchlistStockOptions, ...searchedStockOptions]),
    [initialRouteStock, searchedStockOptions, watchlistStockOptions],
  );

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
    const requestSeq = nextNewsRequestSeq(newsRequestSeqRef);
    try {
      setIsSearching(true);
      const result = await newsMarket({
        market: defaultMarket,
        limit: newsQueryLimit,
        ...(options?.forceRefresh ? { forceRefresh: true } : {}),
      });
      if (!isLatestNewsRequest(newsRequestSeqRef, requestSeq)) {
        return;
      }
      setSearchItems(result.items.map(mapCoreNewsItem));
      setRemoteSearchMode(false);
      setCurrentPage(1);
    } catch (error) {
      if (isLatestNewsRequest(newsRequestSeqRef, requestSeq)) {
        message.error(error instanceof Error ? error.message : "市场资讯加载失败");
      }
    } finally {
      if (isLatestNewsRequest(newsRequestSeqRef, requestSeq)) {
        setIsSearching(false);
      }
    }
  }, [message]);

  const loadStockNews = useCallback(async (symbol: string) => {
    const requestSeq = nextNewsRequestSeq(newsRequestSeqRef);
    try {
      setIsSearching(true);
      const result = await newsList({ symbol, limit: newsQueryLimit });
      if (!isLatestNewsRequest(newsRequestSeqRef, requestSeq)) {
        return;
      }
      setSearchItems(result.items.map(mapCoreNewsItem));
      setRemoteSearchMode(false);
      setCurrentPage(1);
    } catch (error) {
      if (isLatestNewsRequest(newsRequestSeqRef, requestSeq)) {
        message.error(error instanceof Error ? error.message : "个股资讯加载失败");
      }
    } finally {
      if (isLatestNewsRequest(newsRequestSeqRef, requestSeq)) {
        setIsSearching(false);
      }
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
    if (filters.stockSymbol) {
      void loadStockNews(filters.stockSymbol);
    } else {
      void loadMarketNews();
    }
    void loadNewsInsights();
  }, [filters.stockSymbol, loadMarketNews, loadNewsInsights, loadStockNews]);

  useEffect(() => {
    watchlistList()
      .then((result) => setWatchlistStockOptions(result.items.map(mapWatchlistStockOption)))
      .catch(() => {
        setWatchlistStockOptions([]);
      });
  }, []);

  useEffect(() => {
    if (!initialRouteStock) {
      return;
    }
    setFilters((current) => {
      if (current.stockSymbol === initialRouteStock.value) {
        return current;
      }
      return { ...current, stock: initialRouteStock.label, stockSymbol: initialRouteStock.value };
    });
  }, [initialRouteStock]);

  const handleRefresh = async () => {
    const keyword = filters.keyword.trim();
    const stockSymbol = filters.stockSymbol.trim();
    if (!keyword && stockSymbol) {
      await loadStockNews(stockSymbol);
      await loadNewsInsights();
      return;
    }
    if (!keyword) {
      await loadMarketNews({ forceRefresh: true });
      await loadNewsInsights();
      return;
    }
    const requestSeq = nextNewsRequestSeq(newsRequestSeqRef);
    try {
      setIsSearching(true);
      const results = await searchNews({
        keyword,
        symbols: stockSymbol ? [stockSymbol] : [],
        limit: newsQueryLimit,
        offset: 0,
        sort: "relevance",
      });
      if (!isLatestNewsRequest(newsRequestSeqRef, requestSeq)) {
        return;
      }
      setSearchItems(results.filter((item) => item.doc_type === "news").map(mapSearchNewsItem));
      setRemoteSearchMode(true);
      setCurrentPage(1);
      message.success("资讯搜索已更新");
    } catch (error) {
      if (isLatestNewsRequest(newsRequestSeqRef, requestSeq)) {
        message.error(error instanceof Error ? error.message : "资讯搜索失败");
      }
    } finally {
      if (isLatestNewsRequest(newsRequestSeqRef, requestSeq)) {
        setIsSearching(false);
      }
    }
  };

  const handleStockSearch = async (keyword: string) => {
    const normalizedKeyword = keyword.trim();
    if (!normalizedKeyword) {
      setSearchedStockOptions([]);
      return;
    }
    try {
      const results = await stockSearch(normalizedKeyword);
      setSearchedStockOptions(results.map(mapSearchStockOption));
    } catch (error) {
      message.error(error instanceof Error ? error.message : "股票搜索失败");
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
        stockOptions={stockOptions}
        onChange={updateFilters}
        onStockSearch={handleStockSearch}
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

type RouteStockState = {
  newsStock?: {
    symbol?: string;
    name?: string;
    code?: string;
  };
};

function routeStockFromLocation(location: ReturnType<typeof useLocation>): NewsStockOption | null {
  const routeState = location.state as RouteStockState | null;
  const stateStock = routeState?.newsStock;
  if (stateStock?.symbol) {
    return {
      value: stateStock.symbol,
      label: stateStock.name || stateStock.code || stateStock.symbol,
    };
  }
  const query = new URLSearchParams(location.search);
  const symbol = query.get("symbol")?.trim();
  if (!symbol) {
    return null;
  }
  return {
    value: symbol,
    label: query.get("name")?.trim() || query.get("code")?.trim() || symbol,
  };
}

function mergeStockOptions(options: Array<NewsStockOption | null>) {
  const seen = new Set<string>();
  const result: NewsStockOption[] = [];
  for (const option of options) {
    if (!option || seen.has(option.value)) {
      continue;
    }
    seen.add(option.value);
    result.push(option);
  }
  return result;
}

function nextNewsRequestSeq(ref: { current: number }) {
  ref.current += 1;
  return ref.current;
}

function isLatestNewsRequest(ref: { current: number }, requestSeq: number) {
  return ref.current === requestSeq;
}

function mapWatchlistStockOption(item: WatchlistItem): NewsStockOption {
  return {
    value: item.symbol,
    label: item.name || item.code || item.symbol,
  };
}

function mapSearchStockOption(item: StockSearchResult): NewsStockOption {
  return {
    value: item.symbol,
    label: item.name || item.code || item.symbol,
  };
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
    sentiment: normalizeSentiment(item.sentiment),
    url: normalizeNewsURL(item.url),
  };
}

function mapSearchNewsItem(item: DocumentSearchItem): NewsItem {
  const tags = Array.from(new Set([item.symbol, ...(item.tags ?? [])].map((tag) => tag.trim()).filter(Boolean)));
  return {
    id: item.doc_uid,
    source: item.source || "资讯",
    timeLabel: formatNewsTime(item.source_time),
    title: item.title,
    summary: item.summary,
    tags,
    sentiment: normalizeSentiment(item.sentiment ?? undefined),
    url: normalizeNewsURL(item.url ?? undefined),
  };
}

function normalizeSentiment(value?: string): NewsItem["sentiment"] {
  if (value === "positive" || value === "neutral" || value === "negative") {
    return value;
  }
  return undefined;
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
  const positiveCount = stats.sentiment_positive_count ?? 0;
  const neutralCount = stats.sentiment_neutral_count ?? 0;
  const negativeCount = stats.sentiment_negative_count ?? 0;
  const total = Math.max(1, positiveCount + neutralCount + negativeCount);
  return {
    positive: { count: positiveCount, percent: Math.round((positiveCount / total) * 100) },
    neutral: { count: neutralCount, percent: Math.round((neutralCount / total) * 100) },
    negative: { count: negativeCount, percent: Math.round((negativeCount / total) * 100) },
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
