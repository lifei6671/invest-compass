export type StockSearchResult = {
  id: string;
  name: string;
  symbol: string;
  code: string;
  market: "A股" | "港股" | "美股";
  exchange: string;
  industry: string;
  pinyin: string;
  isAdded: boolean;
};

export const mockSearchResults: StockSearchResult[] = [
  {
    id: "CN:SH:600519",
    name: "贵州茅台",
    symbol: "CN:SH:600519",
    code: "600519.SH",
    market: "A股",
    exchange: "上海证券交易所",
    industry: "食品饮料",
    pinyin: "guizhoumaotai",
    isAdded: true,
  },
  {
    id: "CN:SZ:300750",
    name: "宁德时代",
    symbol: "CN:SZ:300750",
    code: "300750.SZ",
    market: "A股",
    exchange: "深圳证券交易所",
    industry: "电力设备",
    pinyin: "ningdeshidai",
    isAdded: false,
  },
  {
    id: "HK:00700",
    name: "腾讯控股",
    symbol: "HK:00700",
    code: "00700.HK",
    market: "港股",
    exchange: "香港联合交易所",
    industry: "互联网传媒",
    pinyin: "tengxunkonggu",
    isAdded: false,
  },
  {
    id: "US:AAPL",
    name: "Apple Inc.",
    symbol: "US:AAPL",
    code: "AAPL",
    market: "美股",
    exchange: "NASDAQ",
    industry: "消费电子",
    pinyin: "apple",
    isAdded: false,
  },
];
