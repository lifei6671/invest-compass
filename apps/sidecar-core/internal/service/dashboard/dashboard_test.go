package dashboard

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/market"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/news"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/report"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/task"
)

// TestBuildSummaryCountsWatchlistDistribution 验证总览页按自选股行情计算涨跌平分布。
func TestBuildSummaryCountsWatchlistDistribution(t *testing.T) {
	summary := BuildSummary(Input{
		WatchlistQuotes: []market.Quote{
			{ChangePercent: 1.2},
			{ChangePercent: -0.4},
			{ChangePercent: 0},
		},
	})

	if summary.Watchlist.UpCount != 1 || summary.Watchlist.DownCount != 1 || summary.Watchlist.FlatCount != 1 {
		t.Fatalf("unexpected watchlist distribution: %+v", summary.Watchlist)
	}
}

// TestBuildSummaryKeepsRecentReportsTasksAndNews 验证总览页只取最近报告、任务和市场新闻。
func TestBuildSummaryKeepsRecentReportsTasksAndNews(t *testing.T) {
	base := time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)
	summary := BuildSummary(Input{
		RecentLimit: 2,
		Reports: []report.Report{
			{ID: 1, TaskID: "task-old", Title: "旧报告", UpdatedAt: base},
			{ID: 2, TaskID: "task-new", Title: "新报告", UpdatedAt: base.Add(2 * time.Hour)},
			{ID: 3, TaskID: "task-mid", Title: "中报告", UpdatedAt: base.Add(time.Hour)},
		},
		Tasks: []task.Task{
			{ID: "task-old", UpdatedAt: base},
			{ID: "task-new", UpdatedAt: base.Add(2 * time.Hour)},
			{ID: "task-mid", UpdatedAt: base.Add(time.Hour)},
		},
		MarketNews: []news.Item{
			{ID: "news-old", Title: "旧新闻", PublishedAt: base},
			{ID: "news-new", Title: "新新闻", PublishedAt: base.Add(2 * time.Hour)},
			{ID: "news-mid", Title: "中新闻", PublishedAt: base.Add(time.Hour)},
		},
	})

	if len(summary.RecentReports) != 2 || summary.RecentReports[0].Title != "新报告" || summary.RecentReports[1].Title != "中报告" {
		t.Fatalf("unexpected recent reports: %+v", summary.RecentReports)
	}
	if len(summary.RecentTasks) != 2 || summary.RecentTasks[0].ID != "task-new" || summary.RecentTasks[1].ID != "task-mid" {
		t.Fatalf("unexpected recent tasks: %+v", summary.RecentTasks)
	}
	if len(summary.MarketNews) != 2 || summary.MarketNews[0].ID != "news-new" || summary.MarketNews[1].ID != "news-mid" {
		t.Fatalf("unexpected market news: %+v", summary.MarketNews)
	}
}

// TestProviderStatusRedactsLastError 验证数据源状态会脱敏最近错误。
func TestProviderStatusRedactsLastError(t *testing.T) {
	summary := BuildSummary(Input{
		ProviderStatuses: []ProviderStatus{
			{Name: "demo", Available: false, LastError: "Authorization: Bearer demo-sensitive-value"},
		},
	})

	if len(summary.ProviderStatuses) != 1 {
		t.Fatalf("expected one provider status, got %+v", summary.ProviderStatuses)
	}
	if strings.Contains(summary.ProviderStatuses[0].LastError, "demo-sensitive-value") {
		t.Fatalf("provider status leaked secret: %+v", summary.ProviderStatuses[0])
	}
}

// TestSummaryJSONDoesNotExposeUnsupportedMVPFields 验证总览聚合不会返回策略、公告、研报或资金流字段。
func TestSummaryJSONDoesNotExposeUnsupportedMVPFields(t *testing.T) {
	summary := BuildSummary(Input{})
	encoded, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal summary: %v", err)
	}
	payload := string(encoded)

	for _, forbidden := range []string{"strategy", "announcement", "research", "fund_flow"} {
		if strings.Contains(payload, forbidden) {
			t.Fatalf("summary exposed unsupported field %q: %s", forbidden, payload)
		}
	}
}

// TestSummaryJSONDoesNotExposeReportInputSnapshot 验证 Dashboard 不回显报告输入快照中的一次性持仓。
func TestSummaryJSONDoesNotExposeReportInputSnapshot(t *testing.T) {
	summary := BuildSummary(Input{
		Reports: []report.Report{{
			ID:            1,
			TaskID:        "task-sensitive",
			Title:         "敏感报告",
			InputSnapshot: `{"user_position":{"cost_price":123.45,"shares":100}}`,
			UpdatedAt:     time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC),
		}},
	})
	encoded, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal summary: %v", err)
	}
	payload := string(encoded)

	for _, forbidden := range []string{"InputSnapshot", "input_snapshot", "user_position", "cost_price"} {
		if strings.Contains(payload, forbidden) {
			t.Fatalf("dashboard summary leaked report snapshot field %q: %s", forbidden, payload)
		}
	}
}
