import type { RunningTaskSummary, TaskLogItem, TaskStep } from "./types";

export const runningTaskSummary: RunningTaskSummary = {
  title: "正在生成：生益科技 个股综合分析",
  stockName: "生益科技",
  stockCode: "600183.SH",
  analysisType: "个股综合分析",
  status: "RUNNING",
  taskId: "task_20250520_153012_abcd1234",
  elapsed: "00:01:42",
  model: "DeepSeek-V3",
};

export const initialTaskSteps: TaskStep[] = [
  {
    id: 1,
    title: "校验股票代码",
    description: "代码格式校验通过",
    status: "success",
    time: "15:30:12",
  },
  {
    id: 2,
    title: "拉取基础信息",
    description: "公司资料、财务摘要等",
    status: "success",
    time: "15:30:18",
  },
  {
    id: 3,
    title: "拉取行情与K线",
    description: "日线行情与历史K线",
    status: "success",
    time: "15:30:31",
  },
  {
    id: 4,
    title: "计算技术指标",
    description: "计算中...（约 20%）",
    status: "running",
    progressText: "约 20%",
  },
  {
    id: 5,
    title: "拉取新闻资讯",
    description: "等待中",
    status: "pending",
  },
  {
    id: 6,
    title: "构建 Prompt",
    description: "等待中",
    status: "pending",
  },
  {
    id: 7,
    title: "调用 AI 模型",
    description: "等待中",
    status: "pending",
  },
  {
    id: 8,
    title: "保存分析报告",
    description: "等待中",
    status: "pending",
  },
];

export const initialTaskLogs: TaskLogItem[] = [
  {
    id: "log-1",
    time: "15:30:12",
    eventType: "TASK_STARTED",
    color: "blue",
    description: "任务已创建，准备开始执行分析",
    expandable: true,
  },
  {
    id: "log-2",
    time: "15:30:12",
    eventType: "TASK_PROGRESS",
    color: "blue",
    description: "开始校验股票代码 CN:SH:600183",
  },
  {
    id: "log-3",
    time: "15:30:18",
    eventType: "TASK_PROGRESS",
    color: "green",
    description: "基础信息拉取完成",
    extra: "耗时 6s",
  },
  {
    id: "log-4",
    time: "15:30:31",
    eventType: "TASK_PROGRESS",
    color: "green",
    description: "行情与 K线数据拉取完成",
    extra: "耗时 13s",
  },
  {
    id: "log-5",
    time: "15:31:00",
    eventType: "TASK_PROGRESS",
    color: "blue",
    description: "开始计算技术指标（约 20%）",
    expandable: true,
  },
  {
    id: "log-6",
    time: "15:31:05",
    eventType: "TASK_CHUNK",
    color: "blue",
    description: "流式输出：核心结论段落",
    extra: "+256 字",
  },
  {
    id: "log-7",
    time: "15:31:28",
    eventType: "TASK_CHUNK",
    color: "blue",
    description: "流式输出：行情状态段落",
    extra: "+384 字",
  },
  {
    id: "log-8",
    time: "15:31:40",
    eventType: "TASK_CHUNK",
    color: "blue",
    description: "流式输出：技术面观察段落",
    extra: "+512 字",
  },
  {
    id: "log-9",
    time: "--:--:--",
    eventType: "TASK_PROGRESS",
    color: "gray",
    description: "拉取新闻资讯",
  },
  {
    id: "log-10",
    time: "--:--:--",
    eventType: "TASK_PROGRESS",
    color: "gray",
    description: "构建 Prompt",
  },
];

export const streamingMarkdown = `# 生益科技（600183.SH）个股综合分析

## 1. 核心结论（生成中...）

- 公司基本面稳健，业务结构持续优化，受益于行业需求回暖和高端产品占比提升。
- ▌

## 2. 当前行情状态（生成中...）

- 目前股价处于近三个月区间震荡上沿。
- 量能较前期有所放大，短期关注突破能否有效延续。

## 3. 技术面观察（生成中...）

- 均线系统：股价运行在 MA5 / MA10 上方，MA20 逐步走平。
- MACD：DIF 上穿 DEA，红柱小幅放大，动能有所增强。
- ▌`;
