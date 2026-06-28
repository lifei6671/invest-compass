import { InfoCircleOutlined, LeftOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { App as AntApp, Button } from "antd";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { AnalysisActionBar } from "./components/AnalysisActionBar";
import { AnalysisConfigCard } from "./components/AnalysisConfigCard";
import { DataContextPreview } from "./components/DataContextPreview";
import { OptionalContextCard } from "./components/OptionalContextCard";
import { OutputPreviewCard } from "./components/OutputPreviewCard";
import { emptyContextSummary, initialAnalysisConfig, initialHoldingContext } from "./defaults";
import type { AnalysisConfig, AnalysisModelOption, AnalysisPromptOption, AnalysisType, ContextSummary, OptionalHoldingContext, OutputFormat, SelectedStock } from "./types";
import {
  aiConfigList,
  analysisTaskCreate,
  marketIndicators,
  marketKline,
  marketQuote,
  newsList,
  promptTemplatesList,
  stockProfile,
  stockSearch,
  type AIConfig,
  type AnalysisTaskCreatePayload,
  type MarketIndicatorsResult,
  type MarketKlineItem,
  type MarketQuote,
  type NewsItem,
  type PromptTemplate,
  type PromptTemplateType,
} from "../../services/coreClient";
import { putAnalysisTaskCreateDraft } from "../analysis-running/analysisTaskCreateDraft";

export function AnalysisPage() {
  const { message } = AntApp.useApp();
  const navigate = useNavigate();
  const location = useLocation();
  const [config, setConfig] = useState<AnalysisConfig>(initialAnalysisConfig);
  const [holdingContext, setHoldingContext] = useState<OptionalHoldingContext>(initialHoldingContext);
  const [outputFormat, setOutputFormat] = useState<OutputFormat>("Markdown");
  const [models, setModels] = useState<AIConfig[]>([]);
  const [prompts, setPrompts] = useState<PromptTemplate[]>([]);
  const [stockOptions, setStockOptions] = useState<SelectedStock[]>([]);
  const [contextSummary, setContextSummary] = useState<ContextSummary>(emptyContextSummary);
  const [contextReady, setContextReady] = useState(false);
  const [loadingModels, setLoadingModels] = useState(true);
  const [loadingPrompts, setLoadingPrompts] = useState(true);
  const [searchingStocks, setSearchingStocks] = useState(false);
  const [generating, setGenerating] = useState(false);
  const [creatingTask, setCreatingTask] = useState(false);
  const stockSearchTimerRef = useRef<number | null>(null);
  const stockSearchSeqRef = useRef(0);
  const selectedPromptType = promptTypeForAnalysisType(config.analysisType);
  const selectedModel = models.find((model) => String(model.id) === config.aiModel);
  const selectedPrompt = prompts.find((prompt) => String(prompt.id) === config.promptTemplate && prompt.type === selectedPromptType);
  const previewMarkdown = useMemo(
    () => buildPromptPreview(selectedPrompt, config.stock, contextSummary, holdingContext),
    [config.stock, contextSummary, holdingContext, selectedPrompt],
  );
  const previewPlainText = previewMarkdown;
  const markdownPreviewOutput = previewMarkdown.trim();
  const hasPreviewOutput = Boolean(markdownPreviewOutput);

  const modelOptions = useMemo<AnalysisModelOption[]>(
    () =>
      models.map((model) => ({
        id: model.id,
        label: `${model.name} (${model.model_name})`,
        apiKeyRef: model.api_key_ref,
        disabled: !model.has_api_key,
      })),
    [models],
  );
  const promptOptions = useMemo<AnalysisPromptOption[]>(
    () =>
      prompts
        .filter((prompt) => prompt.type === selectedPromptType)
        .map((prompt) => ({ id: prompt.id, label: prompt.name, type: prompt.type })),
    [prompts, selectedPromptType],
  );

  useEffect(() => {
    let active = true;
    setLoadingModels(true);
    aiConfigList()
      .then((result) => {
        if (!active) {
          return;
        }
        setModels(result.items);
        const next = result.items.find((item) => item.is_default && item.has_api_key) ?? result.items.find((item) => item.has_api_key);
        if (next) {
          setConfig((current) => ({ ...current, aiModel: String(next.id) }));
        }
      })
      .catch((cause) => {
        if (active) {
          message.error(cause instanceof Error ? cause.message : "模型配置加载失败");
        }
      })
      .finally(() => {
        if (active) {
          setLoadingModels(false);
        }
      });
    return () => {
      active = false;
    };
  }, [message]);

  useEffect(() => {
    let active = true;
    setLoadingPrompts(true);
    promptTemplatesList()
      .then((result) => {
        if (!active) {
          return;
        }
        setPrompts(result.items);
        setConfig((current) => {
          const next = selectPromptForAnalysisType(result.items, current.analysisType);
          return next ? { ...current, promptTemplate: String(next.id) } : current;
        });
      })
      .catch((cause) => {
        if (active) {
          message.error(cause instanceof Error ? cause.message : "Prompt 模板加载失败");
        }
      })
      .finally(() => {
        if (active) {
          setLoadingPrompts(false);
        }
      });
    return () => {
      active = false;
    };
  }, [message]);

  useEffect(() => {
    if (!prompts.length) {
      return;
    }
    const currentPrompt = prompts.find((prompt) => String(prompt.id) === config.promptTemplate);
    if (currentPrompt?.type === selectedPromptType) {
      return;
    }
    const nextPrompt = selectPromptForAnalysisType(prompts, config.analysisType);
    setConfig((current) => {
      const nextPromptId = nextPrompt ? String(nextPrompt.id) : "";
      if (current.promptTemplate === nextPromptId) {
        return current;
      }
      return { ...current, promptTemplate: nextPromptId };
    });
  }, [config.analysisType, config.promptTemplate, prompts, selectedPromptType]);

  useEffect(() => {
    const query = new URLSearchParams(location.search);
    const analysisType = query.get("analysisType")?.trim();
    if (analysisType === "technical") {
      setConfig((current) => ({ ...current, analysisType: "技术面分析" }));
    } else if (analysisType === "stock_full") {
      setConfig((current) => ({ ...current, analysisType: "个股综合分析" }));
    } else if (analysisType === "fundamental") {
      setConfig((current) => ({ ...current, analysisType: "基本面分析" }));
    } else if (analysisType === "news") {
      setConfig((current) => ({ ...current, analysisType: "消息面分析" }));
    }
    const symbol = query.get("symbol")?.trim();
    if (!symbol) {
      return;
    }
    let active = true;
    setSearchingStocks(true);
    stockSearch(symbol)
      .then((items) => {
        if (!active) {
          return;
        }
        const options = items.map((item) => ({ name: item.name, symbol: item.symbol, code: item.code }));
        setStockOptions(options);
        const selected = options.find((item) => analysisSymbolsReferToSameStock(item.symbol, symbol));
        if (selected) {
          setConfig((current) => ({ ...current, stock: selected }));
        } else {
          message.error("未找到匹配股票");
        }
      })
      .catch((cause) => {
        if (active) {
          message.error(cause instanceof Error ? cause.message : "股票搜索失败");
        }
      })
      .finally(() => {
        if (active) {
          setSearchingStocks(false);
        }
      });
    return () => {
      active = false;
    };
  }, [location.search, message]);

  useEffect(() => {
    if (!config.stock.symbol) {
      setContextSummary(emptyContextSummary);
      setContextReady(false);
      return;
    }
    let active = true;
    setContextReady(false);
    Promise.allSettled([
      stockProfile(config.stock.symbol),
      marketQuote(config.stock.symbol, { forceRefresh: true }),
      marketKline({ symbol: config.stock.symbol, period: "day", adjust: "qfq", limit: 120 }),
      marketIndicators({ symbol: config.stock.symbol, period: "day", adjust: "qfq", limit: 120, indicators: ["ma", "rsi", "macd", "kdj", "boll"] }),
      newsList({ symbol: config.stock.symbol, limit: 20 }),
    ])
      .then(([profileResult, quoteResult, klineResult, indicatorsResult, newsResult]) => {
        if (!active) {
          return;
        }

        const profile = profileResult.status === "fulfilled" ? profileResult.value : null;
        const quote = quoteResult.status === "fulfilled" ? quoteResult.value : null;
        const klineItems = klineResult.status === "fulfilled" ? klineResult.value.items : [];
        const indicators = indicatorsResult.status === "fulfilled" ? indicatorsResult.value : null;
        const newsItems = newsResult.status === "fulfilled" ? newsResult.value.items : [];
        const hasAnyContext = Boolean(profile || quote || klineItems.length || indicators || newsItems.length);

        if (!hasAnyContext) {
          setContextSummary(emptyContextSummary);
          setContextReady(false);
          message.error("股票上下文加载失败");
          return;
        }

        setContextSummary(
          buildContextSummary(
            profile?.full_name || profile?.name || config.stock.name || config.stock.symbol,
            profile?.industry,
            profile?.list_date,
            quote,
            klineItems,
            indicators,
            newsItems,
          ),
        );
        setContextReady(Boolean(profile || quote));
      });
    return () => {
      active = false;
    };
  }, [config.stock, message]);

  useEffect(() => {
    return () => {
      if (stockSearchTimerRef.current !== null) {
        window.clearTimeout(stockSearchTimerRef.current);
      }
    };
  }, []);

  const searchStocks = useCallback((keyword: string) => {
    const nextKeyword = keyword.trim();
    if (stockSearchTimerRef.current !== null) {
      window.clearTimeout(stockSearchTimerRef.current);
      stockSearchTimerRef.current = null;
    }
    if (!nextKeyword) {
      stockSearchSeqRef.current += 1;
      setStockOptions([]);
      setSearchingStocks(false);
      return;
    }

    const seq = stockSearchSeqRef.current + 1;
    stockSearchSeqRef.current = seq;
    stockSearchTimerRef.current = window.setTimeout(() => {
      setSearchingStocks(true);
      stockSearch(nextKeyword)
        .then((items) => {
          if (stockSearchSeqRef.current === seq) {
            setStockOptions(items.map((item) => ({ name: item.name, symbol: item.symbol, code: item.code })));
          }
        })
        .catch((cause) => {
          if (stockSearchSeqRef.current === seq) {
            setStockOptions([]);
            message.error(cause instanceof Error ? cause.message : "股票搜索失败");
          }
        })
        .finally(() => {
          if (stockSearchSeqRef.current === seq) {
            setSearchingStocks(false);
          }
        });
    }, 300);
  }, [message]);

  const copyMarkdown = async () => {
    if (!markdownPreviewOutput) {
      message.info("暂无可复制的预览内容");
      return;
    }
    try {
      await navigator.clipboard.writeText(markdownPreviewOutput);
      message.success("Markdown 已复制");
    } catch (cause) {
      message.error(cause instanceof Error ? cause.message : "Markdown 复制失败");
    }
  };

  const exportMarkdown = () => {
    if (!markdownPreviewOutput) {
      message.info("暂无可导出的预览内容");
      return;
    }
    const url = URL.createObjectURL(new Blob([markdownPreviewOutput], { type: "text/markdown;charset=utf-8" }));
    const link = document.createElement("a");
    link.href = url;
    link.download = analysisPreviewFileName(config.stock.symbol);
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
    message.success("Markdown 已导出");
  };

  const startAnalysis = () => {
    if (!selectedModel?.has_api_key) {
      message.error("请先配置可用 AI 模型");
      return;
    }
    if (!selectedPrompt) {
      message.error("请先选择 Prompt 模板");
      return;
    }
    if (!config.stock.symbol || !contextReady) {
      message.error("请先选择股票并加载行情上下文");
      return;
    }
    if (!isExecutableAnalysisType(selectedPromptType)) {
      message.error("当前分析类型暂未接入任务执行");
      return;
    }
    const createPayload: AnalysisTaskCreatePayload = {
      symbol: config.stock.symbol,
      analysis_type: selectedPromptType,
      ai_config_id: selectedModel.id,
      api_key_ref: selectedModel.api_key_ref,
      prompt_template_id: selectedPrompt.id,
      user_position: buildUserPositionPayload(holdingContext),
    };
    const createDraftId = putAnalysisTaskCreateDraft(() => analysisTaskCreate(createPayload));
    setCreatingTask(true);
    navigate("/analysis/running?creating=1", {
      state: {
        stockName: config.stock.name,
        stockCode: config.stock.symbol,
        analysisType: selectedPromptType,
        analysisTypeLabel: config.analysisType,
        createDraftId,
      },
    });
  };

  return (
    <section className="analysis-page">
      <header className="analysis-page-header">
        <div>
          <h1>AI 分析</h1>
          <p>基于行情、K线、新闻与技术指标生成研究报告</p>
        </div>
        <Button className="analysis-back-button" icon={<LeftOutlined />} onClick={() => navigate("/watchlist")}>
          返回自选
        </Button>
      </header>
      <div className="analysis-workspace">
        <div className="analysis-left-column">
          <AnalysisConfigCard
            value={config}
            modelOptions={modelOptions}
            promptOptions={promptOptions}
            stockOptions={stockOptions}
            loadingModels={loadingModels}
            loadingPrompts={loadingPrompts}
            searchingStocks={searchingStocks}
            onChange={setConfig}
            onSearchStock={searchStocks}
            onManageTemplate={() => navigate("/settings", { state: { initialActiveTab: "prompt-template" } })}
          />
          <OptionalContextCard value={holdingContext} onChange={setHoldingContext} />
        </div>
        <DataContextPreview
          value={contextSummary}
          onViewMoreNews={() => {
            if (!config.stock.symbol) {
              navigate("/news");
              return;
            }
            navigate("/news", {
              state: {
                newsStock: {
                  symbol: config.stock.symbol,
                  name: config.stock.name,
                  code: config.stock.code,
                },
              },
            });
          }}
        />
        <OutputPreviewCard
          markdown={previewMarkdown}
          plainText={previewPlainText}
          format={outputFormat}
          onFormatChange={setOutputFormat}
        />
      </div>
      <AnalysisActionBar
        generating={generating}
        startLoading={creatingTask}
        startDisabled={!selectedModel?.has_api_key || !selectedPrompt || !isExecutableAnalysisType(selectedPromptType) || creatingTask || generating}
        stopDisabled={creatingTask || !generating}
        saveDisabled
        copyDisabled={!hasPreviewOutput}
        exportDisabled={!hasPreviewOutput}
        onStart={startAnalysis}
        onStop={() => {
          setGenerating(false);
          message.info("已停止生成");
        }}
        onSave={() => message.info("暂无可保存的分析报告")}
        onCopy={() => void copyMarkdown()}
        onExport={exportMarkdown}
      />
      <AnalysisRiskNotice />
    </section>
  );
}

function analysisPreviewFileName(symbol: string) {
  const normalized = symbol.trim().replace(/[^A-Za-z0-9]+/g, "-").replace(/^-+|-+$/g, "");
  return `${normalized || "analysis"}-analysis-preview.md`;
}

function buildContextSummary(
  companyName: string,
  industry: string | undefined,
  listDate: string | undefined,
  quote: MarketQuote | null,
  klines: MarketKlineItem[],
  indicators: MarketIndicatorsResult | null,
  newsItems: NewsItem[],
): ContextSummary {
  const closes = klines.map((item) => item.close).filter((value) => Number.isFinite(value));
  return {
    basicInfo: {
      companyName: companyName || "暂无",
      industry: industry || "暂无",
      listDate: listDate || "暂无",
      totalMarketCap: formatAmountUnit(quote?.total_market_cap),
      floatMarketCap: formatAmountUnit(quote?.float_market_cap),
    },
    quote: {
      price: formatNumber(quote?.price),
      changeAmount: formatSignedNumber(quote?.change_amount),
      changePercent: formatPercent(quote?.change_percent, true),
      open: formatNumber(quote?.open),
      high: formatNumber(quote?.high),
      low: formatNumber(quote?.low),
      amount: formatAmountUnit(quote?.amount),
      volume: formatNumber(quote?.volume),
      turnoverRate: formatPercent(quote?.turnover_rate),
      updateTime: quote?.quote_time || "待加载",
    },
    kline: {
      change20d: formatKlineChange(closes, 20),
      change60d: formatKlineChange(closes, 60),
      ytdChange: "暂无",
      ma20: formatMovingAverage(closes, 20),
      ma60: formatMovingAverage(closes, 60),
      ma120: formatMovingAverage(closes, 120),
      dailyKlines: formatDailyKlines(klines),
    },
    indicators: {
      ma: formatIndicatorGroup(indicators?.indicators?.["ma"], ["ma5"]),
      macd: formatIndicatorGroup(indicators?.indicators?.["macd"], ["dif", "dea", "bar"]),
      rsi: formatIndicatorGroup(indicators?.indicators?.["rsi"], ["rsi6"]),
      kdj: formatIndicatorGroup(indicators?.indicators?.["kdj"], ["k", "d", "j"]),
      boll: formatIndicatorGroup(indicators?.indicators?.["boll"], ["middle", "upper", "lower"]),
    },
    news: {
      count: newsItems.length,
      period: newsItems.length ? "最近资讯" : "暂无新闻",
      items: newsItems.slice(0, 3).map(formatNewsSummaryItem),
    },
  };
}

function formatNewsSummaryItem(item: NewsItem) {
  const title = item.title.trim();
  const summary = item.summary?.trim();
  if (!summary || summary === title) {
    return title;
  }
  return `${title}：${summary}`;
}

function promptTypeForAnalysisType(analysisType: AnalysisType): Extract<PromptTemplateType, "stock_full" | "technical" | "fundamental" | "news"> {
  switch (analysisType) {
    case "技术面分析":
      return "technical";
    case "基本面分析":
      return "fundamental";
    case "消息面分析":
      return "news";
    case "个股综合分析":
    default:
      return "stock_full";
  }
}

function selectPromptForAnalysisType(prompts: PromptTemplate[], analysisType: AnalysisType) {
  const promptType = promptTypeForAnalysisType(analysisType);
  return prompts.find((item) => item.type === promptType);
}

function isExecutableAnalysisType(promptType: PromptTemplateType): promptType is "stock_full" | "technical" {
  return promptType === "stock_full" || promptType === "technical";
}

function buildUserPositionPayload(context: OptionalHoldingContext) {
  const costPrice = Number(context.costPrice);
  const shares = Number(context.shares);
  if (!Number.isFinite(costPrice) || costPrice <= 0 || !Number.isFinite(shares) || shares <= 0) {
    return null;
  }
  return {
    cost_price: costPrice,
    shares,
    risk_level: context.riskPreference,
  };
}

function buildPromptPreview(
  template: PromptTemplate | undefined,
  stock: SelectedStock,
  context: ContextSummary,
  holding: OptionalHoldingContext,
) {
  if (!template || !stock.symbol) {
    return "";
  }
  const content = template.content.trim();
  if (!content) {
    return "";
  }
  const dataMissing = "【数据缺失】当前上下文未提供该项数据，分析时不得自行补全。";
  const stockName = stock.name || context.basicInfo.companyName || dataMissing;
  const stockCode = stock.code || stock.symbol || dataMissing;
  const quoteSummary = [
    `现价 ${context.quote.price}`,
    `涨跌额 ${context.quote.changeAmount}`,
    `涨跌幅 ${context.quote.changePercent}`,
    `成交额 ${context.quote.amount}`,
    `换手率 ${context.quote.turnoverRate}`,
  ].join("；");
  const klineSummary = [
    `近20日涨跌幅 ${context.kline.change20d}`,
    `近60日涨跌幅 ${context.kline.change60d}`,
    `20日均线 ${context.kline.ma20}`,
    `60日均线 ${context.kline.ma60}`,
  ].join("；");
  const indicatorSummary = [
    `MA ${context.indicators.ma}`,
    `MACD ${context.indicators.macd}`,
    `RSI ${context.indicators.rsi}`,
    `KDJ ${context.indicators.kdj}`,
    `BOLL ${context.indicators.boll}`,
  ].join("；");
  const newsSummary = context.news.items.length ? context.news.items.join("；") : dataMissing;
  const replacements: Record<string, string> = {
    stock_name: stockName,
    stock_code: stockCode,
    market: marketFromSymbol(stock.symbol),
    quote: quoteSummary,
    current_price: context.quote.price,
    kline_summary: klineSummary,
    daily_klines: context.kline.dailyKlines || dataMissing,
    indicators: indicatorSummary,
    news: newsSummary,
    news_summary: newsSummary,
    market_news: newsSummary,
    industry_news: newsSummary,
    fundamental_summary: `公司全称 ${context.basicInfo.companyName}；所属行业 ${context.basicInfo.industry}；上市日期 ${context.basicInfo.listDate}`,
    financial_summary: dataMissing,
    valuation_summary: `总市值 ${context.basicInfo.totalMarketCap}；流通市值 ${context.basicInfo.floatMarketCap}`,
    forecast_summary: dataMissing,
    industry_summary: context.basicInfo.industry,
    user_position: userPositionPreview(holding, dataMissing),
    risk_preference: holding.riskPreference,
    industry: context.basicInfo.industry,
    data_asof: context.quote.updateTime,
    context_sources: "股票基础资料、行情快照、K线、技术指标、相关新闻",
    context_quality: "基于当前已加载数据生成，缺失字段已显式标注。",
    analysis_language: "简体中文",
    prompt_key: template.key || template.name,
    prompt_version: typeof template.version === "number" ? String(template.version) : "1",
  };
  return content.replace(/\{\{\s*([a-zA-Z0-9_]+)\s*\}\}/g, (match, variable: string) => replacements[variable] ?? `【变量缺失: ${variable}】`);
}

function userPositionPreview(context: OptionalHoldingContext, fallback: string) {
  if (!context.costPrice && !context.shares) {
    return fallback;
  }
  return [`成本价 ${context.costPrice || "未填写"}`, `股数 ${context.shares || "未填写"}`, `风险偏好 ${context.riskPreference}`].join("；");
}

function marketFromSymbol(symbol: string) {
  if (symbol.includes(":SH") || symbol.endsWith(".SH")) {
    return "沪市";
  }
  if (symbol.includes(":SZ") || symbol.endsWith(".SZ")) {
    return "深市";
  }
  return symbol || "暂无";
}

function analysisSymbolsReferToSameStock(left: string, right: string) {
  return normalizeAnalysisSymbol(left) === normalizeAnalysisSymbol(right);
}

function normalizeAnalysisSymbol(symbol: string) {
  const value = symbol.trim().toUpperCase();
  const cnSymbol = /^CN:(SH|SZ|BJ):(\d+)$/.exec(value);
  if (cnSymbol) {
    return `${cnSymbol[2]}.${cnSymbol[1]}`;
  }
  const dotSymbol = /^(\d+)\.(SH|SZ|BJ)$/.exec(value);
  if (dotSymbol) {
    return `${dotSymbol[1]}.${dotSymbol[2]}`;
  }
  return value;
}

function formatNumber(value: number | undefined) {
  return typeof value === "number" && Number.isFinite(value) ? Number(value.toFixed(2)).toString() : "暂无";
}

function formatSignedNumber(value: number | undefined) {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    return "暂无";
  }
  const formatted = Number(Math.abs(value).toFixed(2)).toString();
  return value > 0 ? `+${formatted}` : value < 0 ? `-${formatted}` : "0";
}

function formatPercent(value: number | undefined, signed = false) {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    return "暂无";
  }
  const formatted = `${Math.abs(value).toFixed(2)}%`;
  if (!signed) {
    return value < 0 ? `-${formatted}` : formatted;
  }
  return value > 0 ? `+${formatted}` : value < 0 ? `-${formatted}` : "0.00%";
}

function formatAmountUnit(value: number | undefined) {
  if (typeof value !== "number" || !Number.isFinite(value) || value <= 0) {
    return "暂无";
  }
  if (Math.abs(value) >= 100000000) {
    return `${(value / 100000000).toFixed(2)}亿`;
  }
  if (Math.abs(value) >= 10000) {
    return `${(value / 10000).toFixed(2)}万`;
  }
  return Number(value.toFixed(2)).toString();
}

function formatMovingAverage(values: number[], windowSize: number) {
  if (values.length < windowSize) {
    return "暂无";
  }
  const slice = values.slice(-windowSize);
  const average = slice.reduce((sum, value) => sum + value, 0) / windowSize;
  return average.toFixed(2);
}

function formatKlineChange(values: number[], windowSize: number) {
  if (values.length <= windowSize) {
    return "暂无";
  }
  const base = values[values.length - windowSize - 1];
  const latest = values.at(-1);
  if (typeof base !== "number" || typeof latest !== "number" || !Number.isFinite(base) || !Number.isFinite(latest) || base === 0) {
    return "暂无";
  }
  return `${(((latest - base) / base) * 100).toFixed(2)}%`;
}

function formatDailyKlines(klines: MarketKlineItem[]) {
  if (!klines.length) {
    return "";
  }
  return klines
    .slice(-60)
    .map(
      (item) =>
        `date=${item.trade_date} open=${formatKlineNumber(item.open)} high=${formatKlineNumber(item.high)} low=${formatKlineNumber(item.low)} close=${formatKlineNumber(item.close)} volume=${formatKlineInteger(item.volume)} amount=${formatKlineInteger(item.amount)}`,
    )
    .join("\n");
}

function formatKlineNumber(value: number | undefined) {
  return typeof value === "number" && Number.isFinite(value) ? value.toFixed(2) : "暂无";
}

function formatKlineInteger(value: number | undefined) {
  return typeof value === "number" && Number.isFinite(value) ? Math.round(value).toString() : "暂无";
}

function formatIndicatorGroup(indicators: unknown, keys: string[]) {
  if (!indicators || typeof indicators !== "object") {
    return "暂无";
  }
  const source = indicators as Record<string, unknown>;
  const values = keys
    .map((key) => latestFiniteValue(source[key]))
    .filter((value): value is number => typeof value === "number" && Number.isFinite(value))
    .map((value) => value.toFixed(2));
  return values.length ? values.join(" / ") : "暂无";
}

function latestFiniteValue(value: unknown) {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  if (!Array.isArray(value)) {
    return undefined;
  }
  for (let index = value.length - 1; index >= 0; index -= 1) {
    const item = value[index];
    if (typeof item === "number" && Number.isFinite(item)) {
      return item;
    }
  }
  return undefined;
}

function AnalysisRiskNotice() {
  return (
    <div className="settings-basic-risk-notice analysis-risk-notice">
      <div className="settings-basic-risk-left">
        <InfoCircleOutlined />
        <span>AI 输出需区分事实、推断和观点，仅供研究参考。</span>
      </div>
      <div className="settings-basic-risk-right">
        <SafetyCertificateOutlined />
        <span>仅供研究，不构成投资建议。</span>
      </div>
    </div>
  );
}
