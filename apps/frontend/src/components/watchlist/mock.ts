export type WatchlistItem = {
  id: number;
  starred: boolean;
  name: string;
  code: string;
  market: "沪市" | "深市" | "港股" | "美股";
  price: string;
  changeAmount: string;
  changePercent: string;
  amount: string;
  turnoverRate: string;
  pe: string;
  industry: string;
  tags: string[];
  note: string;
  updatedAt: string;
  trend: "up" | "down";
};

export const watchlistMockItems: WatchlistItem[] = [
  { id: 1, starred: true, name: "贵州茅台", code: "600519.SH", market: "沪市", price: "1,688.00", changeAmount: "+12.50", changePercent: "+0.75%", amount: "27.58亿", turnoverRate: "0.18%", pe: "21.38", industry: "食品饮料", tags: ["白马", "核心"], note: "长期跟踪", updatedAt: "05-20 15:30", trend: "up" },
  { id: 2, starred: true, name: "宁德时代", code: "300750.SZ", market: "深市", price: "238.50", changeAmount: "-0.80", changePercent: "-0.33%", amount: "35.62亿", turnoverRate: "0.68%", pe: "18.74", industry: "电力设备", tags: ["新能源", "核心"], note: "关注业绩拐点", updatedAt: "05-20 15:30", trend: "down" },
  { id: 3, starred: false, name: "生益科技", code: "600183.SH", market: "沪市", price: "25.68", changeAmount: "+0.36", changePercent: "+1.42%", amount: "8.92亿", turnoverRate: "1.72%", pe: "33.62", industry: "电子", tags: ["PCB"], note: "关注扩产进度", updatedAt: "05-20 15:30", trend: "up" },
  { id: 4, starred: false, name: "雅克科技", code: "002409.SZ", market: "深市", price: "57.21", changeAmount: "+0.52", changePercent: "+0.92%", amount: "6.31亿", turnoverRate: "1.28%", pe: "27.11", industry: "电子化学品", tags: ["半导体"], note: "电子材料平台", updatedAt: "05-20 15:30", trend: "up" },
  { id: 5, starred: false, name: "泰晶科技", code: "603738.SH", market: "沪市", price: "19.14", changeAmount: "-0.12", changePercent: "-0.62%", amount: "4.21亿", turnoverRate: "1.35%", pe: "30.45", industry: "电子", tags: ["元器件"], note: "石英晶振龙头", updatedAt: "05-20 15:30", trend: "down" },
  { id: 6, starred: true, name: "新易盛", code: "300502.SZ", market: "深市", price: "91.75", changeAmount: "+1.90", changePercent: "+2.11%", amount: "12.63亿", turnoverRate: "2.55%", pe: "32.18", industry: "通信设备", tags: ["光模块", "核心"], note: "800G 需求", updatedAt: "05-20 15:30", trend: "up" },
  { id: 7, starred: true, name: "中际旭创", code: "300308.SZ", market: "深市", price: "141.80", changeAmount: "+2.38", changePercent: "+1.71%", amount: "18.76亿", turnoverRate: "2.91%", pe: "29.66", industry: "通信设备", tags: ["光模块", "核心"], note: "北美客户订单", updatedAt: "05-20 15:30", trend: "up" },
  { id: 8, starred: true, name: "沪电股份", code: "002463.SZ", market: "深市", price: "37.68", changeAmount: "+0.21", changePercent: "+0.56%", amount: "7.84亿", turnoverRate: "1.47%", pe: "23.52", industry: "电子", tags: ["PCB", "核心"], note: "服务器 PCB", updatedAt: "05-20 15:30", trend: "up" },
  { id: 9, starred: false, name: "紫光国微", code: "002049.SZ", market: "深市", price: "63.58", changeAmount: "-0.35", changePercent: "-0.55%", amount: "5.13亿", turnoverRate: "0.96%", pe: "50.32", industry: "半导体", tags: ["芯片"], note: "特种芯片领先", updatedAt: "05-20 15:30", trend: "down" },
  { id: 10, starred: true, name: "北方华创", code: "002371.SZ", market: "深市", price: "312.35", changeAmount: "+3.55", changePercent: "+1.15%", amount: "9.41亿", turnoverRate: "1.02%", pe: "42.67", industry: "半导体设备", tags: ["设备", "核心"], note: "国产替代", updatedAt: "05-20 15:30", trend: "up" },
];
