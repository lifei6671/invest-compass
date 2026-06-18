package dashboard

import (
	"sort"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/market"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/news"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/report"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/task"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
)

const defaultRecentLimit = 5

// Input 是构建 Dashboard summary 所需的首版数据来源。
type Input struct {
	WatchlistQuotes  []market.Quote
	Reports          []report.Report
	Tasks            []task.Task
	MarketNews       []news.Item
	ProviderStatuses []market.ProviderStatus
	RecentLimit      int
}

// Summary 是 Dashboard summary 的首版聚合结果。
type Summary struct {
	Watchlist        WatchlistSummary        `json:"watchlist"`
	RecentReports    []report.Report         `json:"recent_reports"`
	RecentTasks      []task.Task             `json:"recent_tasks"`
	MarketNews       []news.Item             `json:"market_news"`
	RiskTips         []string                `json:"risk_tips"`
	ProviderStatuses []ProviderStatusSummary `json:"provider_statuses"`
}

// WatchlistSummary 是自选股涨跌分布。
type WatchlistSummary struct {
	UpCount   int `json:"up_count"`
	DownCount int `json:"down_count"`
	FlatCount int `json:"flat_count"`
}

// ProviderStatusSummary 是数据源状态的安全展示模型。
type ProviderStatusSummary struct {
	Name      string `json:"name"`
	Source    string `json:"source"`
	Available bool   `json:"available"`
	LastError string `json:"last_error"`
}

// BuildSummary 汇总 Dashboard 首版允许展示的数据。
func BuildSummary(input Input) Summary {
	limit := input.RecentLimit
	if limit <= 0 {
		limit = defaultRecentLimit
	}

	return Summary{
		Watchlist:        buildWatchlistSummary(input.WatchlistQuotes),
		RecentReports:    limitReports(report.UniqueByTaskID(report.VisibleReports(input.Reports)), limit),
		RecentTasks:      limitTasks(task.ListTasksByUpdatedAt(input.Tasks), limit),
		MarketNews:       limitNews(sortNews(input.MarketNews), limit),
		RiskTips:         defaultRiskTips(),
		ProviderStatuses: buildProviderStatuses(input.ProviderStatuses),
	}
}

// buildWatchlistSummary 计算自选股涨跌平分布。
func buildWatchlistSummary(quotes []market.Quote) WatchlistSummary {
	var summary WatchlistSummary
	for _, quote := range quotes {
		switch {
		case quote.ChangePercent > 0:
			summary.UpCount++
		case quote.ChangePercent < 0:
			summary.DownCount++
		default:
			summary.FlatCount++
		}
	}
	return summary
}

// limitReports 截断最近报告列表。
func limitReports(reports []report.Report, limit int) []report.Report {
	if len(reports) <= limit {
		return reports
	}
	return reports[:limit]
}

// limitTasks 截断最近任务列表。
func limitTasks(tasks []task.Task, limit int) []task.Task {
	if len(tasks) <= limit {
		return tasks
	}
	return tasks[:limit]
}

// sortNews 按发布时间倒序返回市场新闻。
func sortNews(items []news.Item) []news.Item {
	sorted := append([]news.Item(nil), items...)
	sort.SliceStable(sorted, func(left int, right int) bool {
		return sorted[left].PublishedAt.After(sorted[right].PublishedAt)
	})
	return sorted
}

// limitNews 截断市场新闻列表。
func limitNews(items []news.Item, limit int) []news.Item {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

// buildProviderStatuses 生成脱敏后的数据源状态列表。
func buildProviderStatuses(statuses []market.ProviderStatus) []ProviderStatusSummary {
	summaries := make([]ProviderStatusSummary, 0, len(statuses))
	for _, status := range statuses {
		summaries = append(summaries, ProviderStatusSummary{
			Name:      status.Name,
			Source:    status.Source,
			Available: status.Available,
			LastError: logger.RedactText(status.LastError),
		})
	}
	return summaries
}

// defaultRiskTips 返回 Dashboard 固定风险提示，避免首页出现投资建议语义。
func defaultRiskTips() []string {
	return []string{
		"仅作研究辅助，不构成投资建议。",
		"行情、新闻和 AI 输出可能存在延迟或错误，请自行核验。",
	}
}
