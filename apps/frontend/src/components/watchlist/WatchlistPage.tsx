import { App as AntApp, Button, Input, Modal, Segmented, Tag } from "antd";
import { InfoCircleOutlined, SafetyCertificateOutlined, TableOutlined, AppstoreOutlined } from "@ant-design/icons";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { AddWatchlistModal } from "./AddWatchlistModal";
import { SummaryPanel } from "./SummaryPanel";
import { WatchlistCardGrid } from "./WatchlistCardGrid";
import { WatchlistTableCard } from "./WatchlistTableCard";
import type { WatchlistItem } from "./types";
import {
  marketQuote,
  searchWatchlistNotes,
  watchlistCreate,
  watchlistDelete,
  watchlistList,
  watchlistUpdate,
  type DocumentSearchItem,
  type MarketQuote,
  type StockSearchResult,
  type WatchlistItem as CoreWatchlistItem,
} from "../../services/coreClient";

type ViewMode = "table" | "card";

export function WatchlistPage() {
  const { message } = AntApp.useApp();
  const navigate = useNavigate();
  const [items, setItems] = useState<WatchlistItem[]>([]);
  const [keyword, setKeyword] = useState("");
  const [marketFilter, setMarketFilter] = useState("all");
  const [tagFilter, setTagFilter] = useState("all");
  const [viewMode, setViewMode] = useState<ViewMode>("card");
  const [addOpen, setAddOpen] = useState(false);
  const [remoteSearchMode, setRemoteSearchMode] = useState(false);
  const [searchItems, setSearchItems] = useState<WatchlistItem[]>([]);
  const [isSearching, setIsSearching] = useState(false);
  const [isLoadingList, setIsLoadingList] = useState(false);
  const [listError, setListError] = useState<string | null>(null);
  const [editingItem, setEditingItem] = useState<WatchlistItem | null>(null);
  const [editTags, setEditTags] = useState("");
  const [editNote, setEditNote] = useState("");
  const [isSavingEdit, setIsSavingEdit] = useState(false);

  const loadItems = useCallback(
    async (options?: { notify?: boolean }) => {
      try {
        setIsLoadingList(true);
        setListError(null);
        const list = await watchlistList();
        const rows = await hydrateWatchlistItems(list.items);
        setItems(rows);
        if (options?.notify) {
          message.success("自选股已刷新");
        }
      } catch (error) {
        const text = error instanceof Error ? error.message : "自选股读取失败";
        setListError(text);
        if (options?.notify) {
          message.error(text);
        }
      } finally {
        setIsLoadingList(false);
      }
    },
    [message],
  );

  useEffect(() => {
    void loadItems();
  }, [loadItems]);

  const filteredItems = useMemo(() => {
    if (remoteSearchMode) {
      return searchItems;
    }
    const normalized = keyword.trim().toLowerCase();
    return items.filter((item) => {
      if (marketFilter !== "all" && item.market !== marketFilter) {
        return false;
      }
      if (tagFilter !== "all" && !item.tags.includes(tagFilter)) {
        return false;
      }
      if (!normalized) {
        return true;
      }
      const haystack = [item.name, item.code, item.market, item.industry, item.note, ...item.tags].join(" ").toLowerCase();
      return haystack.includes(normalized);
    });
  }, [items, keyword, marketFilter, remoteSearchMode, searchItems, tagFilter]);
  const totalCount = filteredItems.length;
  const marketOptions = useMemo(
    () => [
      { value: "all", label: "全部市场" },
      ...Array.from(new Set(items.map((item) => item.market))).map((market) => ({ value: market, label: market })),
    ],
    [items],
  );
  const tagOptions = useMemo(
    () => [
      { value: "all", label: "全部标签" },
      ...Array.from(new Set(items.flatMap((item) => item.tags))).map((tag) => ({ value: tag, label: tag })),
    ],
    [items],
  );

  const handleKeywordChange = (value: string) => {
    setKeyword(value);
    setRemoteSearchMode(false);
    setSearchItems([]);
  };

  const removeItem = async (item: WatchlistItem) => {
    try {
      setIsLoadingList(true);
      await watchlistDelete(item.id);
      setItems((current) => current.filter((value) => value.id !== item.id));
      setSearchItems((current) => current.filter((value) => value.id !== item.id));
      message.success("已从自选股移除");
    } catch (error) {
      message.error(error instanceof Error ? error.message : "删除自选股失败");
    } finally {
      setIsLoadingList(false);
    }
  };

  const addItem = async (payload: { stock: StockSearchResult; tags: string[]; note: string }) => {
    const saved = await watchlistCreate({
      symbol: payload.stock.symbol,
      sort_order: items.length + 1,
      tags: payload.tags,
      note: payload.note,
    });
    const row = await hydrateWatchlistItem(saved, payload.stock.name);
    setItems((current) => [row, ...current.filter((item) => item.id !== row.id)]);
    setRemoteSearchMode(false);
    setSearchItems([]);
    setAddOpen(false);
    message.success("已添加到自选股");
  };

  const openEdit = (item: WatchlistItem) => {
    setEditingItem(item);
    setEditTags(item.tags.join("，"));
    setEditNote(item.note);
  };

  const saveEdit = async () => {
    if (!editingItem) {
      return;
    }
    try {
      setIsSavingEdit(true);
      const updated = await watchlistUpdate({
        id: editingItem.id,
        sort_order: editingItem.sortOrder,
        tags: parseTagsInput(editTags),
        note: editNote,
      });
      const row = await hydrateWatchlistItem(updated, editingItem.name);
      setItems((current) => current.map((item) => (item.id === row.id ? row : item)));
      setEditingItem(null);
      message.success("自选股备注已更新");
    } catch (error) {
      message.error(error instanceof Error ? error.message : "更新自选股失败");
    } finally {
      setIsSavingEdit(false);
    }
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
      {listError ? <div className="rounded-lg border border-red-100 bg-red-50 px-4 py-3 text-[14px] text-red-600">自选股读取失败：{listError}</div> : null}
      <div className="flex min-h-0 flex-1 gap-4">
        {viewMode === "table" ? (
          <WatchlistTableCard
            items={filteredItems}
            keyword={keyword}
            onKeywordChange={handleKeywordChange}
            onAdd={() => setAddOpen(true)}
            onDelete={removeItem}
            onEdit={openEdit}
            onView={viewDetail}
            onRefresh={() => void loadItems({ notify: true })}
            onSearch={searchNotes}
            isSearching={isSearching || isLoadingList}
            emptyDescription={remoteSearchMode ? "仅搜索自选备注和标签，暂无匹配自选项" : undefined}
            totalCount={totalCount}
            marketFilter={marketFilter}
            tagFilter={tagFilter}
            marketOptions={marketOptions}
            tagOptions={tagOptions}
            onMarketFilterChange={setMarketFilter}
            onTagFilterChange={setTagFilter}
          />
        ) : (
          <WatchlistCardGrid
            items={filteredItems}
            keyword={keyword}
            onKeywordChange={handleKeywordChange}
            onAdd={() => setAddOpen(true)}
            onDelete={removeItem}
            onEdit={openEdit}
            onView={viewDetail}
            onRefresh={() => void loadItems({ notify: true })}
            onSearch={searchNotes}
            isSearching={isSearching || isLoadingList}
            emptyDescription={remoteSearchMode ? "仅搜索自选备注和标签，暂无匹配自选项" : undefined}
            totalCount={totalCount}
            marketFilter={marketFilter}
            tagFilter={tagFilter}
            marketOptions={marketOptions}
            tagOptions={tagOptions}
            onMarketFilterChange={setMarketFilter}
            onTagFilterChange={setTagFilter}
          />
        )}
        <SummaryPanel />
      </div>
      <AddWatchlistModal open={addOpen} onClose={() => setAddOpen(false)} onConfirm={addItem} />
      <Modal
        centered
        destroyOnHidden
        title="编辑自选股"
        open={Boolean(editingItem)}
        okText="保存"
        cancelText="取消"
        confirmLoading={isSavingEdit}
        onCancel={() => setEditingItem(null)}
        onOk={saveEdit}
      >
        <div className="space-y-4 pt-2">
          <div>
            <div className="mb-2 text-[13px] font-semibold text-[#374151]">标签</div>
            <Input value={editTags} placeholder="多个标签用逗号、空格或顿号分隔" onChange={(event) => setEditTags(event.target.value)} />
            <div className="mt-2 flex flex-wrap gap-2">
              {parseTagsInput(editTags).map((tag) => (
                <Tag key={tag} className="m-0 rounded-md border-0 bg-slate-100 text-slate-600">
                  {tag}
                </Tag>
              ))}
            </div>
          </div>
          <div>
            <div className="mb-2 text-[13px] font-semibold text-[#374151]">备注</div>
            <Input.TextArea rows={4} maxLength={200} showCount value={editNote} placeholder="记录你对该股票的关注点" onChange={(event) => setEditNote(event.target.value.slice(0, 200))} />
          </div>
        </div>
      </Modal>
      <RiskNotice />
    </section>
  );
}

function mapWatchlistNoteSearchItem(item: DocumentSearchItem, index: number): WatchlistItem {
  const parsedID = Number(item.ref_id);
  const { code, market } = watchlistDisplaySymbol(item.symbol);
  return {
    id: Number.isFinite(parsedID) && parsedID > 0 ? parsedID : 100000 + index,
    sourceSymbol: item.symbol,
    sortOrder: index + 1,
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
  if (item.sourceSymbol) {
    return item.sourceSymbol;
  }
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

async function hydrateWatchlistItems(items: CoreWatchlistItem[]) {
  return Promise.all(items.map((item) => hydrateWatchlistItem(item)));
}

async function hydrateWatchlistItem(item: CoreWatchlistItem, displayName?: string): Promise<WatchlistItem> {
  try {
    return mapCoreWatchlistItem(item, await marketQuote(item.symbol), displayName);
  } catch {
    return mapCoreWatchlistItem(item, null, displayName);
  }
}

function mapCoreWatchlistItem(item: CoreWatchlistItem, quote: MarketQuote | null, displayName?: string): WatchlistItem {
  const display = watchlistProfileDisplay(item);
  const changePercent = quote?.change_percent;
  const changeAmount = quote?.change_amount;
  return {
    id: item.id,
    sourceSymbol: item.symbol,
    sortOrder: item.sort_order,
    starred: true,
    name: displayName || item.name || display.code,
    code: display.code,
    market: display.market,
    price: formatQuoteNumber(quote?.price),
    changeAmount: formatSignedQuoteNumber(changeAmount),
    changePercent: formatQuotePercent(changePercent),
    amount: formatAmount(quote?.amount),
    turnoverRate: formatQuotePercent(quote?.turnover_rate, false),
    pe: formatQuoteNumber(quote?.pe),
    industry: item.industry || "未分类",
    tags: item.tags,
    note: item.note,
    updatedAt: formatWatchlistQuoteTime(quote?.quote_time),
    trend: typeof changePercent === "number" && changePercent < 0 ? "down" : "up",
  };
}

function watchlistProfileDisplay(item: CoreWatchlistItem): Pick<WatchlistItem, "code" | "market"> {
  if (item.code) {
    const suffix = item.exchange ? `.${item.exchange}` : "";
    return {
      code: `${item.code}${suffix}`,
      market: marketLabelFromProfile(item.market, item.exchange),
    };
  }
  return watchlistDisplaySymbol(item.symbol);
}

function marketLabelFromProfile(market: string | undefined, exchange: string | undefined): WatchlistItem["market"] {
  if (market === "HK" || exchange === "HK") {
    return "港股";
  }
  if (market === "US" || exchange === "US") {
    return "美股";
  }
  if (exchange === "SZ") {
    return "深市";
  }
  return "沪市";
}

function parseTagsInput(value: string) {
  return Array.from(new Set(value.split(/[，,、\s]+/).map((item) => item.trim()).filter(Boolean)));
}

function formatQuoteNumber(value: number | undefined) {
  if (typeof value !== "number") {
    return "--";
  }
  return new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 2 }).format(value);
}

function formatSignedQuoteNumber(value: number | undefined) {
  if (typeof value !== "number") {
    return "--";
  }
  return `${value > 0 ? "+" : ""}${formatQuoteNumber(value)}`;
}

function formatQuotePercent(value: number | undefined, signed = true) {
  if (typeof value !== "number") {
    return "--";
  }
  return `${signed && value > 0 ? "+" : ""}${value.toFixed(2)}%`;
}

function formatAmount(value: number | undefined) {
  if (typeof value !== "number") {
    return "--";
  }
  if (Math.abs(value) >= 100000000) {
    return `${(value / 100000000).toFixed(2)}亿`;
  }
  if (Math.abs(value) >= 10000) {
    return `${(value / 10000).toFixed(2)}万`;
  }
  return formatQuoteNumber(value);
}

function formatWatchlistQuoteTime(value: string | undefined) {
  if (!value) {
    return "等待刷新";
  }
  return formatWatchlistSearchTime(value);
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
