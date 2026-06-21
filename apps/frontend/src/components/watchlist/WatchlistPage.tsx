import { App as AntApp, Segmented } from "antd";
import { InfoCircleOutlined, SafetyCertificateOutlined, TableOutlined, AppstoreOutlined } from "@ant-design/icons";
import { useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { AddWatchlistModal } from "./AddWatchlistModal";
import { SummaryPanel } from "./SummaryPanel";
import { WatchlistCardGrid } from "./WatchlistCardGrid";
import { WatchlistTableCard } from "./WatchlistTableCard";
import { watchlistMockItems, type WatchlistItem } from "./mock";
import type { StockSearchResult } from "./mockSearchResults";

type ViewMode = "table" | "card";

export function WatchlistPage() {
  const { message } = AntApp.useApp();
  const navigate = useNavigate();
  const [items, setItems] = useState<WatchlistItem[]>(watchlistMockItems);
  const [keyword, setKeyword] = useState("");
  const [viewMode, setViewMode] = useState<ViewMode>("card");
  const [addOpen, setAddOpen] = useState(false);

  const filteredItems = useMemo(() => {
    const normalized = keyword.trim().toLowerCase();
    if (!normalized) {
      return items;
    }
    return items.filter((item) => {
      const haystack = [item.name, item.code, item.market, item.industry, item.note, ...item.tags].join(" ").toLowerCase();
      return haystack.includes(normalized);
    });
  }, [items, keyword]);

  const removeItem = (item: WatchlistItem) => {
    setItems((current) => current.filter((value) => value.id !== item.id));
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
        industry: payload.stock.industry,
        tags: payload.tags,
        note: payload.note,
        updatedAt: "等待刷新",
        trend: "up",
      },
      ...current,
    ]);
    setAddOpen(false);
    message.success("已添加到自选股");
  };

  const viewDetail = (item: WatchlistItem) => {
    navigate(`/stocks/${encodeURIComponent(detailSymbolFromWatchlist(item))}`, { state: { from: "/watchlist" } });
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
            onKeywordChange={setKeyword}
            onAdd={() => setAddOpen(true)}
            onDelete={removeItem}
            onView={viewDetail}
            onRefresh={() => message.success("已刷新本地 mock 数据")}
          />
        ) : (
          <WatchlistCardGrid
            items={filteredItems}
            keyword={keyword}
            onKeywordChange={setKeyword}
            onAdd={() => setAddOpen(true)}
            onDelete={removeItem}
            onView={viewDetail}
            onRefresh={() => message.success("已刷新本地 mock 数据")}
          />
        )}
        <SummaryPanel />
      </div>
      <AddWatchlistModal open={addOpen} onClose={() => setAddOpen(false)} onConfirm={addItem} />
      <RiskNotice />
    </section>
  );
}

function detailSymbolFromWatchlist(item: WatchlistItem) {
  if (item.code.endsWith(".SH")) {
    return `CN:SH:${item.code.replace(".SH", "")}`;
  }
  if (item.code.endsWith(".SZ")) {
    return `CN:SZ:${item.code.replace(".SZ", "")}`;
  }
  if (item.code.endsWith(".HK")) {
    return `HK:${item.code.replace(".HK", "")}`;
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
