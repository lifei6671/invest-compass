import { useMemo, useState } from "react";
import { App as AntApp } from "antd";
import { InfoCircleFilled, SafetyCertificateOutlined } from "@ant-design/icons";
import {
  dataSourceStatuses,
  hotIndustries,
  hotKeywords,
  initialNewsFilters,
  mentionedStocks,
  newsItems,
  sentimentSummary,
} from "./mock";
import type { NewsFilters, NewsItem } from "./types";
import { NewsFilterCard } from "./components/NewsFilterCard";
import { NewsListCard } from "./components/NewsListCard";
import { NewsSidebarPanel } from "./components/NewsSidebarPanel";

const pageSize = 10;
const mockTotalCount = 218;

export function NewsCenterPage() {
  const { message } = AntApp.useApp();
  const [filters, setFilters] = useState<NewsFilters>(initialNewsFilters);
  const [sortMode, setSortMode] = useState("按最新");
  const [currentPage, setCurrentPage] = useState(1);

  const filteredItems = useMemo(() => {
    const keyword = filters.keyword.trim().toLowerCase();
    return newsItems.filter((item) => {
      const keywordMatched =
        !keyword ||
        item.title.toLowerCase().includes(keyword) ||
        item.summary.toLowerCase().includes(keyword) ||
        item.tags.some((tag) => tag.toLowerCase().includes(keyword));
      const stockMatched = filters.stock === "全部股票" || item.tags.includes(filters.stock);
      const sourceMatched = filters.source === "全部来源" || item.source === filters.source;
      const industryMatched = filters.industry === "全部行业" || item.tags.some((tag) => tag.includes(filters.industry));
      return keywordMatched && stockMatched && sourceMatched && industryMatched;
    });
  }, [filters]);

  const visibleItems = filteredItems.slice((currentPage - 1) * pageSize, currentPage * pageSize);

  const updateFilters = (patch: Partial<NewsFilters>) => {
    setFilters((current) => ({ ...current, ...patch }));
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
        onRefresh={() => message.success("资讯已刷新")}
      />

      <div className="news-center-content-grid">
        <NewsListCard
          items={visibleItems}
          totalCount={mockTotalCount}
          sortMode={sortMode}
          currentPage={currentPage}
          onSortModeChange={handleSortModeChange}
          onViewSwitch={() => message.info("视图切换待接入")}
          onPageChange={setCurrentPage}
          onOpenOriginal={() => message.info("查看原文待接入")}
          onAddContext={() => message.success("已加入 AI 分析上下文")}
          onCopySummary={handleCopySummary}
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
