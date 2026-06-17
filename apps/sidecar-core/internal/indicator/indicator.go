package indicator

import (
	"math"
)

// ErrorCode 是技术指标计算失败时对外稳定的错误码。
type ErrorCode string

const (
	// ErrorInvalidPeriod 表示指标周期参数非法。
	ErrorInvalidPeriod ErrorCode = "invalid_indicator_period"
	// ErrorInsufficientData 表示输入数据不足以计算目标指标。
	ErrorInsufficientData ErrorCode = "insufficient_indicator_data"
	// ErrorInvalidInput 表示输入价格或参数不符合计算前置条件。
	ErrorInvalidInput ErrorCode = "invalid_indicator_input"
)

// IndicatorError 表示技术指标计算失败，并携带稳定错误码。
type IndicatorError struct {
	Code ErrorCode
}

// Error 返回稳定错误码字符串，避免把原始行情数据拼进错误文本。
func (err *IndicatorError) Error() string {
	return string(err.Code)
}

// KLine 是技术指标计算所需的最小 K 线数据。
type KLine struct {
	High  float64
	Low   float64
	Close float64
}

// MACDSeries 表示 MACD 的 DIF、DEA 和柱状值序列。
type MACDSeries struct {
	DIF []float64
	DEA []float64
	Bar []float64
}

// KDJSeries 表示 KDJ 的 K、D、J 三条序列。
type KDJSeries struct {
	K []float64
	D []float64
	J []float64
}

// BOLLSeries 表示布林线的中轨、上轨和下轨序列。
type BOLLSeries struct {
	Middle []float64
	Upper  []float64
	Lower  []float64
}

// MA 计算简单移动平均线，数据不足的位置使用 NaN 表达空结果语义。
func MA(values []float64, period int) ([]float64, error) {
	if err := requirePeriod(period); err != nil {
		return nil, err
	}
	if len(values) < period {
		return nil, newIndicatorError(ErrorInsufficientData)
	}

	result := nanSeries(len(values))
	var sum float64
	for index, value := range values {
		sum += value
		if index >= period {
			sum -= values[index-period]
		}
		if index >= period-1 {
			result[index] = sum / float64(period)
		}
	}
	return result, nil
}

// EMA 计算指数移动平均线，首个点直接使用首个输入值作为初始 EMA。
func EMA(values []float64, period int) ([]float64, error) {
	if err := requirePeriod(period); err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, newIndicatorError(ErrorInsufficientData)
	}

	result := make([]float64, len(values))
	multiplier := 2 / float64(period+1)
	result[0] = values[0]
	for index := 1; index < len(values); index++ {
		result[index] = (values[index]-result[index-1])*multiplier + result[index-1]
	}
	return result, nil
}

// MACD 计算 DIF、DEA 和柱状值，柱状值按常见行情软件口径乘以 2。
func MACD(values []float64, fastPeriod int, slowPeriod int, signalPeriod int) (MACDSeries, error) {
	if fastPeriod <= 0 || slowPeriod <= 0 || signalPeriod <= 0 || fastPeriod >= slowPeriod {
		return MACDSeries{}, newIndicatorError(ErrorInvalidPeriod)
	}
	fastEMA, err := EMA(values, fastPeriod)
	if err != nil {
		return MACDSeries{}, err
	}
	slowEMA, err := EMA(values, slowPeriod)
	if err != nil {
		return MACDSeries{}, err
	}

	dif := make([]float64, len(values))
	for index := range values {
		dif[index] = fastEMA[index] - slowEMA[index]
	}

	dea, err := EMA(dif, signalPeriod)
	if err != nil {
		return MACDSeries{}, err
	}
	bar := make([]float64, len(values))
	for index := range values {
		bar[index] = 2 * (dif[index] - dea[index])
	}

	return MACDSeries{DIF: dif, DEA: dea, Bar: bar}, nil
}

// RSI 计算相对强弱指标，数据不足的位置使用 NaN 表达空结果语义。
func RSI(values []float64, period int) ([]float64, error) {
	if err := requirePeriod(period); err != nil {
		return nil, err
	}
	if len(values) <= period {
		return nil, newIndicatorError(ErrorInsufficientData)
	}

	result := nanSeries(len(values))
	for index := period; index < len(values); index++ {
		var gainSum float64
		var lossSum float64
		for cursor := index - period + 1; cursor <= index; cursor++ {
			change := values[cursor] - values[cursor-1]
			if change > 0 {
				gainSum += change
			} else {
				lossSum -= change
			}
		}
		if lossSum == 0 {
			result[index] = 100
			continue
		}
		rs := gainSum / lossSum
		result[index] = 100 - 100/(1+rs)
	}
	return result, nil
}

// KDJ 计算随机指标 K、D、J，初始 K/D 使用 50。
func KDJ(klines []KLine, period int) (KDJSeries, error) {
	if err := requirePeriod(period); err != nil {
		return KDJSeries{}, err
	}
	if len(klines) < period {
		return KDJSeries{}, newIndicatorError(ErrorInsufficientData)
	}

	kValues := nanSeries(len(klines))
	dValues := nanSeries(len(klines))
	jValues := nanSeries(len(klines))
	lastK := 50.0
	lastD := 50.0

	for index := period - 1; index < len(klines); index++ {
		low, high := lowHigh(klines[index-period+1 : index+1])
		var rsv float64
		if high == low {
			rsv = 50
		} else {
			rsv = (klines[index].Close - low) / (high - low) * 100
		}

		lastK = 2.0/3.0*lastK + 1.0/3.0*rsv
		lastD = 2.0/3.0*lastD + 1.0/3.0*lastK
		kValues[index] = lastK
		dValues[index] = lastD
		jValues[index] = 3*lastK - 2*lastD
	}

	return KDJSeries{K: kValues, D: dValues, J: jValues}, nil
}

// BOLL 计算布林线，中轨为 MA，上下轨为中轨加减倍数标准差。
func BOLL(values []float64, period int, multiplier float64) (BOLLSeries, error) {
	if err := requirePeriod(period); err != nil {
		return BOLLSeries{}, err
	}
	if len(values) < period {
		return BOLLSeries{}, newIndicatorError(ErrorInsufficientData)
	}
	if multiplier <= 0 {
		return BOLLSeries{}, newIndicatorError(ErrorInvalidInput)
	}

	middle, err := MA(values, period)
	if err != nil {
		return BOLLSeries{}, err
	}
	upper := nanSeries(len(values))
	lower := nanSeries(len(values))
	for index := period - 1; index < len(values); index++ {
		stddev := stddev(values[index-period+1:index+1], middle[index])
		upper[index] = middle[index] + multiplier*stddev
		lower[index] = middle[index] - multiplier*stddev
	}

	return BOLLSeries{Middle: middle, Upper: upper, Lower: lower}, nil
}

// VolumeMA 计算成交量均线，与价格 MA 保持相同的空结果语义。
func VolumeMA(volumes []float64, period int) ([]float64, error) {
	return MA(volumes, period)
}

// ChangePercent 计算涨跌幅百分比。
func ChangePercent(previousClose float64, currentClose float64) (float64, error) {
	if previousClose <= 0 {
		return 0, newIndicatorError(ErrorInvalidInput)
	}
	return (currentClose - previousClose) / previousClose * 100, nil
}

// MaxDrawdown 计算区间最大回撤百分比。
func MaxDrawdown(values []float64) (float64, error) {
	if len(values) == 0 {
		return 0, newIndicatorError(ErrorInsufficientData)
	}
	peak := values[0]
	maxDrawdown := 0.0
	for _, value := range values {
		if value > peak {
			peak = value
		}
		if peak <= 0 {
			return 0, newIndicatorError(ErrorInvalidInput)
		}
		drawdown := (peak - value) / peak * 100
		if drawdown > maxDrawdown {
			maxDrawdown = drawdown
		}
	}
	return maxDrawdown, nil
}

// Volatility 计算区间收益率总体标准差百分比。
func Volatility(values []float64) (float64, error) {
	if len(values) < 2 {
		return 0, newIndicatorError(ErrorInsufficientData)
	}

	returns := make([]float64, 0, len(values)-1)
	for index := 1; index < len(values); index++ {
		if values[index-1] <= 0 {
			return 0, newIndicatorError(ErrorInvalidInput)
		}
		returns = append(returns, (values[index]-values[index-1])/values[index-1]*100)
	}
	mean := average(returns)
	var sumSquares float64
	for _, value := range returns {
		diff := value - mean
		sumSquares += diff * diff
	}
	return math.Sqrt(sumSquares / float64(len(returns))), nil
}

// requirePeriod 校验技术指标周期必须为正整数。
func requirePeriod(period int) error {
	if period <= 0 {
		return newIndicatorError(ErrorInvalidPeriod)
	}
	return nil
}

// newIndicatorError 创建稳定错误码错误。
func newIndicatorError(code ErrorCode) error {
	return &IndicatorError{Code: code}
}

// nanSeries 创建以 NaN 填充的序列，用于表达前置数据不足。
func nanSeries(length int) []float64 {
	values := make([]float64, length)
	for index := range values {
		values[index] = math.NaN()
	}
	return values
}

// lowHigh 返回 KDJ 窗口内的最低价和最高价。
func lowHigh(klines []KLine) (float64, float64) {
	low := klines[0].Low
	high := klines[0].High
	for _, kline := range klines[1:] {
		if kline.Low < low {
			low = kline.Low
		}
		if kline.High > high {
			high = kline.High
		}
	}
	return low, high
}

// stddev 计算总体标准差，匹配常见布林线窗口内标准差口径。
func stddev(values []float64, mean float64) float64 {
	var sumSquares float64
	for _, value := range values {
		diff := value - mean
		sumSquares += diff * diff
	}
	return math.Sqrt(sumSquares / float64(len(values)))
}

// average 计算简单平均值。
func average(values []float64) float64 {
	var sum float64
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}
