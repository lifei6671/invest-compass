package news

import "testing"

// TestAnalyzeSentimentClassifiesFinancialNews 验证金融新闻情绪标签按规则输出稳定分类。
func TestAnalyzeSentimentClassifiesFinancialNews(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "positive", text: "公司订单增长超预期，行业景气度回升", want: "positive"},
		{name: "negative", text: "海外需求下滑，公司业绩不及预期", want: "negative"},
		{name: "neutral", text: "公司召开年度股东大会，审议常规议案", want: "neutral"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AnalyzeSentiment(tt.text)
			if got.Label != tt.want {
				t.Fatalf("expected %s, got %+v", tt.want, got)
			}
		})
	}
}

// TestAnalyzeSentimentClassifiesCorporateActionAnnouncements 验证减持、解禁、回购和员工持股计划这类公司行为公告不被归为中性。
func TestAnalyzeSentimentClassifiesCorporateActionAnnouncements(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "reduction plan is negative", text: "股东拟减持公司股份不超过2%", want: "negative"},
		{name: "unlock shares is negative", text: "首次公开发行限售股上市流通公告，解禁股份占总股本15%", want: "negative"},
		{name: "repurchase progress is positive", text: "关于第六期以集中竞价交易方式回购股份的回购进展情况", want: "positive"},
		{name: "employee stock ownership plan is positive", text: "关于第二期员工持股计划第二次持有人会议决议公告", want: "positive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AnalyzeSentiment(tt.text)
			if got.Label != tt.want {
				t.Fatalf("expected %s for %q, got %+v", tt.want, tt.text, got)
			}
		})
	}
}

// TestAnalyzeSentimentWeightsPostTransition 验证转折词之后的情绪权重更高。
func TestAnalyzeSentimentWeightsPostTransition(t *testing.T) {
	got := AnalyzeSentiment("早盘一度上涨，但是订单下滑且业绩不及预期")
	if got.Label != "negative" {
		t.Fatalf("expected post-transition negative sentiment, got %+v", got)
	}
}

// TestAnalyzeSentimentTreatsMixedMarketPhrasesAsNeutral 验证涨跌互现和小幅波动这类混合表述不会被单个涨跌字误判。
func TestAnalyzeSentimentTreatsMixedMarketPhrasesAsNeutral(t *testing.T) {
	tests := []string{
		"三大指数涨跌互现，市场小幅波动",
		"板块涨跌不一，整体影响有限",
	}
	for _, text := range tests {
		got := AnalyzeSentiment(text)
		if got.Label != "neutral" {
			t.Fatalf("expected neutral sentiment for %q, got %+v", text, got)
		}
	}
}

// TestAnalyzeSentimentPrefersLongTerms 验证长词优先，避免上涨同时命中“涨”导致分数虚高。
func TestAnalyzeSentimentPrefersLongTerms(t *testing.T) {
	got := AnalyzeSentiment("股价上涨")
	if got.Label != "positive" {
		t.Fatalf("expected positive sentiment, got %+v", got)
	}
	if got.Score != 2.8 {
		t.Fatalf("expected only 上涨 score to be counted, got %+v", got)
	}
}

// TestAnalyzeSentimentWeightsTitleLine 验证标题行作为新闻核心信息，权重高于摘要背景噪音。
func TestAnalyzeSentimentWeightsTitleLine(t *testing.T) {
	got := AnalyzeSentiment("公司订单超预期\n此前市场担忧需求下滑")
	if got.Label != "positive" {
		t.Fatalf("expected title-weighted positive sentiment, got %+v", got)
	}
}
