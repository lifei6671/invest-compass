import { App as AntApp, Segmented } from "antd";
import { InfoCircleOutlined, SafetyCertificateOutlined, TableOutlined, AppstoreOutlined } from "@ant-design/icons";
import { useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { AddWatchlistModal } from "./AddWatchlistModal";
import { SummaryPanel } from "./SummaryPanel";
import { WatchlistCardGrid } from "./WatchlistCardGrid";
import { WatchlistTableCard } from "./WatchlistTableCard";
import type { WatchlistItem } from "./types";
import { searchWatchlistNotes, type DocumentSearchItem, type StockSearchResult } from "../../services/coreClient";

type ViewMode = "table" | "card";
const localWatchlistTotalCount = 0;

export function WatchlistPage() {
  const { message } = AntApp.useApp();
  const navigate = useNavigate();
  const [items, setItems] = useState<WatchlistItem[]>([]);
  const [keyword, setKeyword] = useState("");
  const [viewMode, setViewMode] = useState<ViewMode>("card");
  const [addOpen, setAddOpen] = useState(false);
  const [remoteSearchMode, setRemoteSearchMode] = useState(false);
  const [searchItems, setSearchItems] = useState<WatchlistItem[]>([]);
  const [isSearching, setIsSearching] = useState(false);

  const filteredItems = useMemo(() => {
    if (remoteSearchMode) {
      return searchItems;
    }
    const normalized = keyword.trim().toLowerCase();
    if (!normalized) {
      return items;
    }
    return items.filter((item) => {
      const haystack = [item.name, item.code, item.market, item.industry, item.note, ...item.tags].join(" ").toLowerCase();
      return haystack.includes(normalized);
    });
  }, [items, keyword, remoteSearchMode, searchItems]);
  const totalCount = remoteSearchMode ? filteredItems.length : localWatchlistTotalCount;

  const handleKeywordChange = (value: string) => {
    setKeyword(value);
    setRemoteSearchMode(false);
    setSearchItems([]);
  };

  const removeItem = (item: WatchlistItem) => {
    setItems((current) => current.filter((value) => value.id !== item.id));
    setSearchItems((current) => current.filter((value) => value.id !== item.id));
    message.success("已从自选股移除");
  };

  const addItem = (payload: { stock: StockSearchResult; tags: string[]; note: string }) => {
    setItems((current) => [
      {
        id: Math.max(0, ...current.map((item) => item.id)) + 1,
        starred: true,
        name: payload.stock.name,
        code: payload.stock.code,
        market: displayMarket(payload.stock),
        price: "--",
        changeAmount: "--",
        changePercent: "--",
        amount: "--",
        turnoverRate: "--",
        pe: "--",
        industry: "未分类",
        tags: payload.tags,
        note: payload.note,
        updatedAt: "等待刷新",
        trend: "up",
      },
      ...current,
    ]);
    setRemoteSearchMode(false);
    setSearchItems([]);
    setAddOpen(false);
    message.success("已添加到自选股");
  };

  const viewDetail = (item: WatchlistItem) => {
    navigate(`/stocks/${encodeURIComponent(detailSymbolFromWatchlist(item))}`, { state: { from: "/watchlist" } });
  };

  const searchNotes = async () => {
    const normalized = keyword.trim();
    if (!normalized) {
      setRemoteSearchMode(false);
      setSearchItems([]);
      message.info("请输入关键词后搜索自选备注和标签");
      return;
    }
    try {
      setIsSearching(true);
      const results = await searchWatchlistNotes({
        keyword: normalized,
        symbols: [],
        limit: 20,
        offset: 0,
        sort: "relevance",
      });
      setSearchItems(results.filter((item) => item.doc_type === "watchlist_note").map(mapWatchlistNoteSearchItem));
      setRemoteSearchMode(true);
      message.success("自选备注搜索已更新");
    } catch (error) {
      message.error(error instanceof Error ? error.message : "自选备注搜索失败");
    } finally {
      setIsSearching(false);
    }
  };

  return (
    <section className="flex min-h-[calc(100vh-120px)] flex-col gap-4">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="m-0 text-[26px] font-semibold leading-9 text-slate-950">自选股</h1>
          <p className="m-0 mt-1 text-[15px] text-slate-500">跟踪重点标的、行情变化与研究入口</p>
        </div>
        <Segmented
          className="watchlist-view-switch rounded-lg border border-[#e5eaf3] bg-white p-1 shadow-[0_1px_2px_rgba(15,23,42,0.03)]"
          value={viewMode}
          onChange={(value) => {
            const next = value as ViewMode;
            setViewMode(next);
          }}
          options={[
            {
              value: "table",
              label: (
                <span className={["inline-flex h-8 w-[112px] items-center justify-center gap-2 rounded-md text-[14px] font-medium transition-colors", viewMode === "table" ? "bg-[#1677ff] text-white" : "text-slate-500 hover:text-[#1677ff]"].join(" ")}>
                  <TableOutlined />
                  表格视图
                </span>
              ),
            },
            {
              value: "card",
              label: (
                <span className={["inline-flex h-8 w-[112px] items-center justify-center gap-2 rounded-md text-[14px] font-medium transition-colors", viewMode === "card" ? "bg-[#1677ff] text-white" : "text-slate-500 hover:text-[#1677ff]"].join(" ")}>
                  <AppstoreOutlined />
                  卡片视图
                </span>
              ),
            },
          ]}
        />
      </div>
      <div className="flex min-h-0 flex-1 gap-4">
        {viewMode === "table" ? (
          <WatchlistTableCard
            items={filteredItems}
            keyword={keyword}
            onKeywordChange={handleKeywordChange}
            onAdd={() => setAddOpen(true)}
            onDelete={removeItem}
            onView={viewDetail}
            onRefresh={() => message.info("自选股刷新待接入真实列表接口")}
            onSearch={searchNotes}
            isSearching={isSearching}
            emptyDescription={remoteSearchMode ? "仅搜索自选备注和标签，暂无匹配自选项" : undefined}
            totalCount={totalCount}
          />
        ) : (
          <WatchlistCardGrid
            items={filteredItems}
            keyword={keyword}
            onKeywordChange={handleKeywordChange}
            onAdd={() => setAddOpen(true)}
            onDelete={removeItem}
            onView={viewDetail}
            onRefresh={() => message.info("自选股刷新待接入真实列表接口")}
            onSearch={searchNotes}
            isSearching={isSearching}
            emptyDescription={remoteSearchMode ? "仅搜索自选备注和标签，暂无匹配自选项" : undefined}
            totalCount={totalCount}
          />
        )}
        <SummaryPanel />
      </div>
      <AddWatchlistModal open={addOpen} onClose={() => setAddOpen(false)} onConfirm={addItem} />
      <RiskNotice />
    </section>
  );
}

function mapWatchlistNoteSearchItem(item: DocumentSearchItem, index: number): WatchlistItem {
  const parsedID = Number(item.ref_id);
  const { code, market } = watchlistDisplaySymbol(item.symbol);
  return {
    id: Number.isFinite(parsedID) && parsedID > 0 ? parsedID : 100000 + index,
    starred: true,
    name: item.title || item.symbol,
    code,
    market,
    price: "--",
    changeAmount: "--",
    changePercent: "--",
    amount: "--",
    turnoverRate: "--",
    pe: "--",
    industry: "自选备注",
    tags: Array.from(new Set((item.highlights.length > 0 ? item.highlights : ["备注"]).filter(Boolean))),
    note: item.summary,
    updatedAt: formatWatchlistSearchTime(item.source_time),
    trend: "up",
  };
}

function watchlistDisplaySymbol(symbol: string): Pick<WatchlistItem, "code" | "market"> {
  const parts = symbol.split(":");
  if (parts.length === 3 && parts[0] === "CN") {
    return {
      code: `${parts[2]}.${parts[1]}`,
      market: parts[1] === "SH" ? "沪市" : "深市",
    };
  }
  if (parts.length === 2 && parts[0] === "HK") {
    return { code: `${parts[1]}.HK`, market: "港股" };
  }
  return { code: symbol, market: "沪市" };
}

function formatWatchlistSearchTime(value: string): string {
  const date = new Date(value);
  if (!value || Number.isNaN(date.getTime())) {
    return "搜索结果";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function detailSymbolFromWatchlist(item: WatchlistItem) {
  if (item.code.endsWith(".SH")) {
    return ["CN", "SH", item.code.replace(".SH", "")].join(":");
  }
  if (item.code.endsWith(".SZ")) {
    return ["CN", "SZ", item.code.replace(".SZ", "")].join(":");
  }
  if (item.code.endsWith(".HK")) {
    return ["HK", item.code.replace(".HK", "")].join(":");
  }
  return item.code;
}

function displayMarket(stock: StockSearchResult): WatchlistItem["market"] {
  if (stock.market === "港股") {
    return "港股";
  }
  if (stock.market === "美股") {
    return "美股";
  }
  return stock.symbol.includes(":SH:") ? "沪市" : "深市";
}

function RiskNotice() {
  return (
    <div className="flex min-h-[52px] items-center justify-between rounded-xl border border-[#b9d6ff] bg-[#f2f7ff] px-5 text-[14px] text-[#155ec8]">
      <div className="flex items-center gap-3 font-medium">
        <InfoCircleOutlined className="text-[18px]" />
        列表数据仅供研究参考，实际行情请以数据源更新为准。
      </div>
      <div className="flex items-center gap-2 text-slate-500">
        <SafetyCertificateOutlined />
        仅供研究，不构成投资建议。
      </div>
    </div>
  );
}
