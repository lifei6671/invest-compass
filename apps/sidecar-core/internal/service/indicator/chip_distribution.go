package indicator

import (
	"math"
	"sort"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

const (
	// DefaultChipDistributionBins 是筹码分布默认分箱数，适合桌面端图表直接渲染。
	DefaultChipDistributionBins = 80
	// MaxChipDistributionBins 限制单次计算的最大分箱数，避免异常请求造成无意义开销。
	MaxChipDistributionBins = 300
)

// ChipKLine 是筹码分布计算需要的 K 线输入。
//
// TurnoverRate 使用百分比口径，例如 5.2 表示 5.2%。当前项目的 K 线缓存尚未持久化
// 换手率，后续接入日线换手率来源后可填充该字段；缺失时保持 0，算法会退化为
// 基于成交量和价格区间的成本分布近似。
type ChipKLine struct {
	Open         float64
	High         float64
	Low          float64
	Close        float64
	Volume       float64
	Amount       float64
	TurnoverRate float64
}

// ChipDistributionOptions 描述筹码分布的计算参数。
type ChipDistributionOptions struct {
	BinCount int
}

// ChipBin 表示某个价格分箱内的相对筹码量和占比。
type ChipBin struct {
	Price  float64
	Volume float64
	Ratio  float64
}

// ChipDistributionResult 表示可供后续个股详情页或 AI 分析上下文使用的筹码分布结果。
//
// 该结果是基于历史 K 线的工程近似，不代表交易所或第三方数据源发布的真实筹码数据。
// 后续进入 UI 或 Prompt 时必须保留“基于历史 K 线近似估算”的展示说明。
type ChipDistributionResult struct {
	SampleSize   int
	BinCount     int
	CurrentPrice float64
	AverageCost  float64
	ProfitRatio  float64
	MinPrice     float64
	MaxPrice     float64
	TotalVolume  float64
	Items        []ChipBin
}

// TopN 返回筹码占比最高的 n 个价格分箱，调用方可用于摘要展示主要成本密集区。
func (result ChipDistributionResult) TopN(n int) []ChipBin {
	if n <= 0 || len(result.Items) == 0 {
		return nil
	}
	items := make([]ChipBin, len(result.Items))
	copy(items, result.Items)
	sort.SliceStable(items, func(left int, right int) bool {
		return items[left].Ratio > items[right].Ratio
	})
	if n > len(items) {
		n = len(items)
	}
	return items[:n]
}

// ChipDistribution 基于历史 K 线近似估算筹码分布。
//
// 算法意图：
// 1. 先用历史最高/最低价确定价格分箱。
// 2. 每根 K 线按换手率衰减旧筹码，缺失换手率时不衰减。
// 3. 将当日成交量按成本中枢分配到 high-low 覆盖的分箱中。
//
// 这是为后续个股详情页“筹码分布图”和 AI 技术分析摘要预留的纯计算能力；当前不新增
// API、Rust command、数据库字段或首版页面入口。
func ChipDistribution(klines []ChipKLine, options ChipDistributionOptions) (ChipDistributionResult, error) {
	if len(klines) == 0 {
		return ChipDistributionResult{}, newIndicatorError(xerr.IndicatorInsufficientData)
	}

	binCount, err := normalizeChipBinCount(options.BinCount)
	if err != nil {
		return ChipDistributionResult{}, err
	}

	minPrice, maxPrice, err := chipPriceRange(klines)
	if err != nil {
		return ChipDistributionResult{}, err
	}
	if minPrice == maxPrice {
		maxPrice = minPrice * 1.001
	}
	width := (maxPrice - minPrice) / float64(binCount)
	if width <= 0 || !isFinite(width) {
		return ChipDistributionResult{}, newIndicatorError(xerr.IndicatorInvalidInput)
	}

	volumes := make([]float64, binCount)
	for _, kline := range klines {
		if isEmptyChipKLine(kline) {
			continue
		}
		if err := validateChipKLine(kline); err != nil {
			return ChipDistributionResult{}, err
		}
		turnover, err := normalizeTurnoverRate(kline.TurnoverRate)
		if err != nil {
			return ChipDistributionResult{}, err
		}
		decayChipVolumes(volumes, turnover)

		low, high, ok := normalizedChipBarRange(kline)
		if !ok {
			return ChipDistributionResult{}, newIndicatorError(xerr.IndicatorInvalidInput)
		}
		center := chipCostCenter(kline, low, high)
		addChipVolume(volumes, minPrice, width, low, high, kline.Volume, center)
	}

	totalVolume := sumFloat64(volumes)
	if totalVolume <= 0 || !isFinite(totalVolume) {
		return ChipDistributionResult{}, newIndicatorError(xerr.IndicatorInvalidInput)
	}

	currentPrice := latestChipPrice(klines)
	items := make([]ChipBin, 0, binCount)
	var averageCost float64
	var profitVolume float64
	for index, volume := range volumes {
		price := minPrice + (float64(index)+0.5)*width
		ratio := volume / totalVolume
		items = append(items, ChipBin{
			Price:  price,
			Volume: volume,
			Ratio:  ratio,
		})
		averageCost += volume * price
		if price <= currentPrice {
			profitVolume += volume
		}
	}

	return ChipDistributionResult{
		SampleSize:   len(klines),
		BinCount:     binCount,
		CurrentPrice: currentPrice,
		AverageCost:  averageCost / totalVolume,
		ProfitRatio:  profitVolume / totalVolume,
		MinPrice:     minPrice,
		MaxPrice:     maxPrice,
		TotalVolume:  totalVolume,
		Items:        items,
	}, nil
}

// normalizeChipBinCount 处理默认分箱数，并拒绝会导致异常计算量的参数。
func normalizeChipBinCount(binCount int) (int, error) {
	if binCount == 0 {
		return DefaultChipDistributionBins, nil
	}
	if binCount < 0 || binCount > MaxChipDistributionBins {
		return 0, newIndicatorError(xerr.IndicatorInvalidInput)
	}
	return binCount, nil
}

// chipPriceRange 从有效 K 线中推导筹码分布价格范围。
func chipPriceRange(klines []ChipKLine) (float64, float64, error) {
	minPrice := math.MaxFloat64
	maxPrice := 0.0
	for _, kline := range klines {
		if isEmptyChipKLine(kline) {
			continue
		}
		if err := validateChipKLine(kline); err != nil {
			return 0, 0, err
		}
		low, high, ok := normalizedChipBarRange(kline)
		if !ok {
			return 0, 0, newIndicatorError(xerr.IndicatorInvalidInput)
		}
		if low < minPrice {
			minPrice = low
		}
		if high > maxPrice {
			maxPrice = high
		}
	}
	if minPrice == math.MaxFloat64 || maxPrice <= 0 {
		return 0, 0, newIndicatorError(xerr.IndicatorInvalidInput)
	}
	return minPrice, maxPrice, nil
}

// isEmptyChipKLine 判断数据源尾部全零占位行，允许这类行不参与筹码分布计算。
func isEmptyChipKLine(kline ChipKLine) bool {
	return kline.Open == 0 &&
		kline.High == 0 &&
		kline.Low == 0 &&
		kline.Close == 0 &&
		kline.Volume == 0 &&
		kline.Amount == 0 &&
		kline.TurnoverRate == 0
}

// validateChipKLine 对非占位 K 线快速失败，避免坏行情被静默折算进筹码图。
func validateChipKLine(kline ChipKLine) error {
	if kline.Volume <= 0 || !isFinite(kline.Volume) {
		return newIndicatorError(xerr.IndicatorInvalidInput)
	}
	if kline.Amount < 0 || !isFinite(kline.Amount) {
		return newIndicatorError(xerr.IndicatorInvalidInput)
	}
	if kline.Open <= 0 || kline.High <= 0 || kline.Low <= 0 || kline.Close <= 0 {
		return newIndicatorError(xerr.IndicatorInvalidInput)
	}
	if !isFinite(kline.Open) || !isFinite(kline.High) || !isFinite(kline.Low) || !isFinite(kline.Close) {
		return newIndicatorError(xerr.IndicatorInvalidInput)
	}
	if kline.High < kline.Low {
		return newIndicatorError(xerr.IndicatorInvalidInput)
	}
	return nil
}

// normalizedChipBarRange 返回单根 K 线有效的最低价和最高价。
func normalizedChipBarRange(kline ChipKLine) (float64, float64, bool) {
	low := kline.Low
	high := kline.High
	if low <= 0 || high <= 0 || !isFinite(low) || !isFinite(high) {
		return 0, 0, false
	}
	return low, high, true
}

// normalizeTurnoverRate 将百分比换手率转换为 0-1 的衰减比例。
func normalizeTurnoverRate(turnoverRate float64) (float64, error) {
	if turnoverRate < 0 || !isFinite(turnoverRate) {
		return 0, newIndicatorError(xerr.IndicatorInvalidInput)
	}
	turnover := turnoverRate / 100
	if turnover > 0.98 {
		return 0.98, nil
	}
	return turnover, nil
}

// decayChipVolumes 用换手率衰减历史筹码，缺失换手率时保持历史筹码不变。
func decayChipVolumes(volumes []float64, turnover float64) {
	if turnover <= 0 {
		return
	}
	remain := 1 - turnover
	for index := range volumes {
		volumes[index] *= remain
	}
}

// chipCostCenter 估算单根 K 线的成交成本中枢，优先使用成交额/成交量推导 VWAP。
func chipCostCenter(kline ChipKLine, low float64, high float64) float64 {
	if kline.Amount > 0 && kline.Volume > 0 {
		vwap := kline.Amount / kline.Volume
		if isFinite(vwap) && vwap > 0 {
			return clampFloat64(vwap, low, high)
		}
	}
	if kline.Open > 0 && kline.Close > 0 && isFinite(kline.Open) && isFinite(kline.Close) {
		return clampFloat64((high+low+kline.Open+kline.Close)/4, low, high)
	}
	if kline.Close > 0 && isFinite(kline.Close) {
		return clampFloat64((high+low+kline.Close)/3, low, high)
	}
	return (high + low) / 2
}

// addChipVolume 将单根 K 线成交量按成本中枢分配到价格分箱。
func addChipVolume(volumes []float64, minPrice float64, width float64, low float64, high float64, volume float64, center float64) {
	start := chipBinIndex(low, minPrice, width, len(volumes))
	end := chipBinIndex(high, minPrice, width, len(volumes))
	if end < start {
		start, end = end, start
	}

	span := high - low
	if span <= 0 {
		volumes[start] += volume
		return
	}

	center = clampFloat64(center, low, high)
	sigma := math.Max(span*0.18, 0.000001)
	weights := make([]float64, end-start+1)
	var weightSum float64
	for index := start; index <= end; index++ {
		price := minPrice + (float64(index)+0.5)*width
		if price < low || price > high {
			continue
		}
		distance := (price - center) / sigma
		weight := math.Exp(-0.5 * distance * distance)
		weights[index-start] = weight
		weightSum += weight
	}

	if weightSum <= 0 {
		addUniformChipVolume(volumes, start, end, volume)
		return
	}
	for index := start; index <= end; index++ {
		volumes[index] += volume * weights[index-start] / weightSum
	}
}

// addUniformChipVolume 在成本核无法计算时将成交量平均分配到覆盖分箱。
func addUniformChipVolume(volumes []float64, start int, end int, volume float64) {
	count := float64(end - start + 1)
	if count <= 0 {
		return
	}
	add := volume / count
	for index := start; index <= end; index++ {
		volumes[index] += add
	}
}

// chipBinIndex 将价格映射到分箱下标，并收敛到合法范围。
func chipBinIndex(price float64, minPrice float64, width float64, binCount int) int {
	index := int(math.Floor((price - minPrice) / width))
	if index < 0 {
		return 0
	}
	if index >= binCount {
		return binCount - 1
	}
	return index
}

// latestChipPrice 返回最新一根有有效价格的 K 线价格，收盘价缺失时回退到最高价。
func latestChipPrice(klines []ChipKLine) float64 {
	for index := len(klines) - 1; index >= 0; index-- {
		kline := klines[index]
		if kline.Close > 0 && isFinite(kline.Close) {
			return kline.Close
		}
		if kline.High > 0 && isFinite(kline.High) {
			return kline.High
		}
	}
	return 0
}

// clampFloat64 将浮点数收敛到闭区间。
func clampFloat64(value float64, minValue float64, maxValue float64) float64 {
	return math.Min(maxValue, math.Max(minValue, value))
}

// isFinite 判断浮点数是否为可参与计算的有限值。
func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

// sumFloat64 计算浮点数组总和。
func sumFloat64(values []float64) float64 {
	var sum float64
	for _, value := range values {
		sum += value
	}
	return sum
}
