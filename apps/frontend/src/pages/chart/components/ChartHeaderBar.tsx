import { App as AntdApp, Button, Input, Popover, Tag } from "antd";
import {
  DownOutlined,
  LeftOutlined,
  SearchOutlined,
  StockOutlined,
} from "@ant-design/icons";
import { useEffect, useState, type KeyboardEvent, type ReactNode } from "react";
import type { StockSearchResult } from "../../../services/coreClient";
import type { StockChartQuote } from "../types";

type ChartHeaderBarProps = {
  quote: StockChartQuote;
  onBack: () => void;
  onSearch: (keyword: string) => Promise<StockSearchResult[]>;
  onSearchSelect: (stock: StockSearchResult) => void;
};

export function ChartHeaderBar(props: ChartHeaderBarProps) {
  const { message } = AntdApp.useApp();
  const [keyword, setKeyword] = useState("");
  const [results, setResults] = useState<StockSearchResult[]>([]);
  const [searching, setSearching] = useState(false);
  const [searchError, setSearchError] = useState<string | null>(null);
  const positive = props.quote.changeAmount >= 0;
  const toneClass = positive ? "text-[#ff4d4f]" : "text-[#16a34a]";

  useEffect(() => {
    setKeyword("");
    setResults([]);
    setSearchError(null);
  }, [props.quote.symbol]);

  const submitSearch = async () => {
    const normalized = keyword.trim();
    if (!normalized || searching) {
      return;
    }
    setSearching(true);
    setSearchError(null);
    try {
      const nextResults = await props.onSearch(normalized);
      setResults(nextResults);
      if (nextResults.length === 0) {
        void message.info("未找到匹配股票");
      }
    } catch (error) {
      const text = error instanceof Error ? error.message : "股票搜索失败";
      setSearchError(text);
      setResults([]);
      void message.error(text);
    } finally {
      setSearching(false);
    }
  };

  const handleSearchEnter = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "Enter") {
      event.preventDefault();
      void submitSearch();
    }
  };

  const selectSearchResult = (stock: StockSearchResult) => {
    setKeyword("");
    setResults([]);
    setSearchError(null);
    props.onSearchSelect(stock);
  };

  const searchPopover = (
    <div className="w-[300px] py-1">
      {searchError ? <div className="px-3 py-2 text-[12px] text-[#ff4d4f]">{searchError}</div> : null}
      {results.map((item) => (
        <button
          key={item.symbol}
          type="button"
          aria-label={`${item.name} ${item.code}`}
          className="flex w-full items-center justify-between gap-3 rounded-md px-3 py-2 text-left text-[13px] text-[#374151] hover:bg-[#f6f8fb]"
          onClick={() => selectSearchResult(item)}
        >
          <span className="min-w-0 truncate font-medium text-[#111827]">{item.name}</span>
          <span className="app-number shrink-0 text-[#64748b]">{item.code || item.symbol}</span>
        </button>
      ))}
      {!searchError && results.length === 0 ? <div className="px-3 py-2 text-[12px] text-[#8a94a6]">输入关键词后按 Enter 搜索</div> : null}
    </div>
  );

  return (
    <header className="flex h-16 shrink-0 items-center justify-between border-b border-[#e5eaf3] bg-white px-4">
      <div className="flex min-w-0 items-center gap-5">
        <div className="flex min-w-0 items-center gap-3 border-r border-[#edf1f7] pr-5">
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-[#1677ff] text-white">
            <StockOutlined className="text-[21px]" />
          </div>
          <div className="flex min-w-0 items-center gap-2">
            <span className="truncate text-[20px] font-semibold leading-7 text-[#111827]">{props.quote.name}</span>
            <span className="app-number text-[16px] font-semibold text-[#374151]">{props.quote.code}</span>
            <DownOutlined className="text-[12px] text-[#374151]" />
          </div>
        </div>

        <div className="flex items-baseline gap-3">
          <span className={`app-number text-[24px] font-bold leading-8 ${toneClass}`}>{props.quote.price.toFixed(2)}</span>
          <span className={`app-number text-[16px] font-semibold ${toneClass}`}>
            {positive ? "+" : ""}
            {props.quote.changeAmount.toFixed(2)}
          </span>
          <span className={`app-number text-[16px] font-semibold ${toneClass}`}>
            {positive ? "+" : ""}
            {props.quote.changePercent.toFixed(2)}%
          </span>
        </div>

        <div className="flex items-center gap-3">
          <Tag className="m-0 rounded-md border-0 bg-[#ecfdf3] px-2.5 py-0.5 text-[13px] font-semibold text-[#16a34a]">
            {props.quote.status}
          </Tag>
          <span className="text-[13px] text-[#64748b]">{props.quote.updateTime}</span>
        </div>
      </div>

      <div className="flex shrink-0 items-center gap-3">
        <Popover content={searchPopover} destroyOnHidden open={results.length > 0 || Boolean(searchError)} placement="bottomRight" trigger="click" arrow={false}>
          <Input
            className="h-9 w-[300px] rounded-lg border-[#dfe7f2] bg-white text-[13px]"
            prefix={<SearchOutlined className="text-[#8a94a6]" />}
            suffix={<span className="rounded border border-[#e5eaf3] px-1.5 py-0.5 text-[11px] leading-4 text-[#8a94a6]">⌘ K</span>}
            value={keyword}
            placeholder="搜索股票 / 指数"
            disabled={searching}
            onChange={(event) => {
              setKeyword(event.target.value);
              setSearchError(null);
              setResults([]);
            }}
            onKeyDown={handleSearchEnter}
          />
        </Popover>
        <div className="h-6 w-px bg-[#edf1f7]" />
        <HeaderButton icon={<LeftOutlined />} label="返回" onClick={props.onBack} />
      </div>
    </header>
  );
}

function HeaderButton(props: { icon: ReactNode; label: string; onClick: () => void }) {
  return (
    <Button
      type="text"
      aria-label={props.label}
      className="h-9 rounded-lg px-2.5 text-[13px] font-medium text-[#374151]"
      icon={props.icon}
      onClick={props.onClick}
    >
      {props.label}
    </Button>
  );
}
