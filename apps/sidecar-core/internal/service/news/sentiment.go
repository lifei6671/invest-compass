package news

import (
	"sort"
	"strings"
)

const (
	sentimentPositive = "positive"
	sentimentNeutral  = "neutral"
	sentimentNegative = "negative"
)

var positiveFinanceWords = map[string]float64{
	"涨": 1.0, "上涨": 2.0, "涨停": 3.0, "牛市": 3.0, "反弹": 2.0, "新高": 2.5,
	"利好": 2.5, "增持": 2.0, "买入": 2.0, "推荐": 1.5, "看多": 2.0,
	"盈利": 2.0, "增长": 2.0, "超预期": 2.5, "强劲": 1.5, "回升": 1.5,
	"复苏": 2.0, "突破": 2.0, "创新高": 3.0, "回暖": 1.5, "上扬": 1.5,
	"利好消息": 3.0, "收益增长": 2.5, "利润增长": 2.5, "业绩优异": 2.5,
	"潜力股": 2.0, "绩优股": 2.0, "强势": 1.5, "走高": 1.5, "攀升": 1.5,
	"大涨": 2.5, "飙升": 3.0, "井喷": 3.0, "暴涨": 3.0,
	"回购股份": 2.5, "回购进展": 2.5, "回购进展情况": 2.5, "股份回购": 2.5,
	"员工持股计划": 2.0,
}

var negativeFinanceWords = map[string]float64{
	"跌": 2.0, "下跌": 2.0, "跌停": 3.0, "熊市": 3.0, "回调": 2.5, "新低": 2.5,
	"利空": 2.5, "减持": 2.0, "卖出": 2.0, "看空": 2.0, "亏损": 2.5,
	"下滑": 2.0, "萎缩": 2.0, "不及预期": 2.5, "疲软": 1.5, "恶化": 2.0,
	"衰退": 2.0, "跌破": 2.0, "创新低": 3.0, "走弱": 2.5, "下挫": 2.5,
	"利空消息": 3.0, "收益下降": 2.5, "利润下滑": 2.5, "业绩不佳": 2.5,
	"风险": 2.0, "弱势": 2.5, "走低": 2.5, "缩量": 2.5,
	"大跌": 2.5, "暴跌": 3.0, "崩盘": 3.0, "跳水": 3.0, "重挫": 3.0,
	"跌超": 2.5, "跌逾": 2.5, "跌近": 3.0, "回吐": 3.0, "转跌": 3.0,
	"拟减持": 2.5, "减持计划": 2.5, "计划减持": 2.5,
	"解禁": 2.5, "限售股上市流通": 2.5, "限售股解禁": 2.5,
}

var sentimentNegationWords = []string{"不", "没", "无", "非", "未", "别", "勿"}

var sentimentDegreeWords = map[string]float64{
	"非常": 1.8, "极其": 2.2, "太": 1.8, "很": 1.5, "比较": 0.8, "稍微": 0.6,
	"有点": 0.7, "显著": 1.5, "大幅": 1.8, "急剧": 2.0, "轻微": 0.6,
	"小幅": 0.7, "逾": 1.8, "超": 1.8,
}

var sentimentTransitionWords = []string{"但是", "然而", "不过", "却", "可是"}

var neutralMarketPhrases = []string{
	"涨跌互现",
	"涨跌不一",
	"小幅波动",
	"窄幅震荡",
	"震荡整理",
	"影响有限",
	"维持稳定",
	"基本持平",
	"整体平稳",
	"波动不大",
}

// SentimentResult 是新闻情绪标签的规则分析结果。
type SentimentResult struct {
	Label       string
	Description string
	Score       float64
}

type sentimentTerm struct {
	Word     string
	Value    float64
	Polarity float64
}

// AnalyzeSentiment 使用金融词典规则分析新闻情绪，不调用外部模型。
func AnalyzeSentiment(text string) SentimentResult {
	text = strings.TrimSpace(text)
	if text == "" {
		return sentimentResult(sentimentNeutral, 0)
	}
	score := scoreSentimentDocument(text)
	switch {
	case score > 1.0:
		return sentimentResult(sentimentPositive, score)
	case score < -1.0:
		return sentimentResult(sentimentNegative, score)
	default:
		return sentimentResult(sentimentNeutral, score)
	}
}

// scoreSentimentDocument 按新闻文本行加权打分；首行通常是标题，权重略高于摘要。
func scoreSentimentDocument(text string) float64 {
	lines := strings.Split(text, "\n")
	score := 0.0
	weightedTitle := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		weight := 1.0
		if !weightedTitle {
			weight = 1.4
			weightedTitle = true
		}
		score += scoreSentimentSegment(line) * weight
	}
	return score
}

// scoreSentimentSegment 处理单段文本，转折词之后的情绪权重更高。
func scoreSentimentSegment(text string) float64 {
	score := scoreSentimentText(text)
	for _, transition := range sentimentTransitionWords {
		if index := strings.Index(text, transition); index >= 0 {
			preScore := scoreSentimentText(text[:index])
			postScore := scoreSentimentText(text[index+len(transition):]) * 1.5
			score = preScore + postScore
			break
		}
	}
	return score
}

// SentimentDescription 返回情绪标签的中文展示文案。
func SentimentDescription(label string) string {
	switch label {
	case sentimentPositive:
		return "看涨"
	case sentimentNegative:
		return "看跌"
	case sentimentNeutral:
		return "中性"
	default:
		return ""
	}
}

// scoreSentimentText 计算文本命中金融情绪词后的加权分值。
func scoreSentimentText(text string) float64 {
	score := 0.0
	occupied := neutralPhraseOccupied(text)
	for _, term := range sortedSentimentTerms() {
		score += termScore(text, term, occupied)
	}
	return score
}

// sortedSentimentTerms 合并正负词典并按长词优先排序，避免“上涨”同时命中“涨”。
func sortedSentimentTerms() []sentimentTerm {
	terms := make([]sentimentTerm, 0, len(positiveFinanceWords)+len(negativeFinanceWords))
	for word, value := range positiveFinanceWords {
		terms = append(terms, sentimentTerm{Word: word, Value: value, Polarity: 1})
	}
	for word, value := range negativeFinanceWords {
		terms = append(terms, sentimentTerm{Word: word, Value: value, Polarity: -1})
	}
	sort.SliceStable(terms, func(left int, right int) bool {
		if len(terms[left].Word) != len(terms[right].Word) {
			return len(terms[left].Word) > len(terms[right].Word)
		}
		return terms[left].Word < terms[right].Word
	})
	return terms
}

// neutralPhraseOccupied 标记中性或混合行情短语，避免其中的“涨”“跌”被拆开计分。
func neutralPhraseOccupied(text string) []bool {
	occupied := make([]bool, len(text))
	for _, phrase := range neutralMarketPhrases {
		start := 0
		for {
			index := strings.Index(text[start:], phrase)
			if index < 0 {
				break
			}
			absoluteIndex := start + index
			markOccupied(occupied, absoluteIndex, absoluteIndex+len(phrase))
			start = absoluteIndex + len(phrase)
		}
	}
	return occupied
}

// termScore 根据词频、否定词和程度词返回单个情绪词分值。
func termScore(text string, term sentimentTerm, occupied []bool) float64 {
	score := 0.0
	start := 0
	for {
		index := strings.Index(text[start:], term.Word)
		if index < 0 {
			break
		}
		absoluteIndex := start + index
		end := absoluteIndex + len(term.Word)
		if !rangeOccupied(occupied, absoluteIndex, end) {
			multiplier := degreeMultiplier(text, absoluteIndex)
			if hasNegationBefore(text, absoluteIndex) {
				multiplier = -multiplier
			}
			score += term.Value * term.Polarity * multiplier
			markOccupied(occupied, absoluteIndex, end)
		}
		start = end
	}
	return score
}

// rangeOccupied 判断当前命中是否已经被更长词或中性短语占用。
func rangeOccupied(occupied []bool, start int, end int) bool {
	for index := start; index < end && index < len(occupied); index++ {
		if occupied[index] {
			return true
		}
	}
	return false
}

// markOccupied 标记已计分文本范围，保证同一片段只贡献一次情绪分。
func markOccupied(occupied []bool, start int, end int) {
	for index := start; index < end && index < len(occupied); index++ {
		occupied[index] = true
	}
}

// degreeMultiplier 读取情绪词前的程度副词权重。
func degreeMultiplier(text string, wordIndex int) float64 {
	prefix := nearbyPrefix(text, wordIndex)
	for word, multiplier := range sentimentDegreeWords {
		if strings.HasSuffix(prefix, word) {
			return multiplier
		}
	}
	return 1
}

// hasNegationBefore 判断情绪词前是否存在紧邻否定词。
func hasNegationBefore(text string, wordIndex int) bool {
	prefix := nearbyPrefix(text, wordIndex)
	for _, word := range sentimentNegationWords {
		if strings.HasSuffix(prefix, word) {
			return true
		}
	}
	return false
}

// nearbyPrefix 返回情绪词前的短上下文，避免远距离词语误影响当前情绪词。
func nearbyPrefix(text string, wordIndex int) string {
	start := wordIndex - 12
	if start < 0 {
		start = 0
	}
	return text[start:wordIndex]
}

// sentimentResult 创建稳定的情绪结果。
func sentimentResult(label string, score float64) SentimentResult {
	return SentimentResult{Label: label, Description: SentimentDescription(label), Score: score}
}
