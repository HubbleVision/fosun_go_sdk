package client

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// KLine K线数据
type KLine struct {
	Time     time.Time // 时间
	Open     float64   // 开盘价
	High     float64   // 最高价
	Low      float64   // 最低价
	Close    float64   // 收盘价
	Vol      float64   // 成交量
	Turnover float64   // 成交额
	Tor      float64   // 换手率(%)
	PClose   float64   // 前收盘价
}

// KLineType K线类型
type KLineType string

const (
	KLineType1Min  KLineType = "min1"
	KLineType5Min  KLineType = "min5"
	KLineType15Min KLineType = "min15"
	KLineType30Min KLineType = "min30"
	KLineType60Min KLineType = "min60"
	KLineTypeDay   KLineType = "day"
	KLineTypeWeek  KLineType = "week"
	KLineTypeMonth KLineType = "month"
)

// ParseKLineTime 解析K线时间
// day/week/month: YYYYMMDD 格式 (如 20260325)
// minute: YYYYMMDDHHMM 格式 (如 202604010945)
func ParseKLineTime(timeStr string, ktype KLineType) (time.Time, error) {
	timeStr = strings.TrimSpace(timeStr)

	switch ktype {
	case KLineTypeDay, KLineTypeWeek, KLineTypeMonth:
		// 日/周/月: YYYYMMDD
		if len(timeStr) >= 8 {
			year, _ := strconv.Atoi(timeStr[0:4])
			month, _ := strconv.Atoi(timeStr[4:6])
			day, _ := strconv.Atoi(timeStr[6:8])
			return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local), nil
		}
	case KLineType1Min, KLineType5Min, KLineType15Min, KLineType30Min, KLineType60Min:
		// 分钟: YYYYMMDDHHMM
		if len(timeStr) >= 12 {
			year, _ := strconv.Atoi(timeStr[0:4])
			month, _ := strconv.Atoi(timeStr[4:6])
			day, _ := strconv.Atoi(timeStr[6:8])
			hour, _ := strconv.Atoi(timeStr[8:10])
			min, _ := strconv.Atoi(timeStr[10:12])
			return time.Date(year, time.Month(month), day, hour, min, 0, 0, time.Local), nil
		}
		// 尝试作为HHMM格式
		if len(timeStr) == 4 {
			hour, _ := strconv.Atoi(timeStr[0:2])
			min, _ := strconv.Atoi(timeStr[2:4])
			now := time.Now()
			return time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, time.Local), nil
		}
	}

	return time.Time{}, fmt.Errorf("无法解析时间格式: %s, 类型: %s", timeStr, ktype)
}

// ConvertPrice 转换价格
// 港股价格单位是分，需要除以1000转为港币
// A股价格单位是分，需要除以100转为人民币
// 美股价格单位是美分，需要除以10000转为美元
func ConvertPrice(price float64, market string) float64 {
	switch market {
	case "hk":
		return price / 1000 // 港分 -> 港币
	case "us":
		return price / 10000 // 美分 -> 美元
	case "sh", "sz", "bj":
		return price / 100 // 分 -> 元
	default:
		return price
	}
}

// ParseKLineResponse 解析K线响应数据
func ParseKLineResponse(resp map[string]interface{}, ktype string) ([]KLine, error) {
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("无效的响应数据")
	}

	dataList, ok := data["data"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("无效的data字段")
	}

	klines := make([]KLine, 0, len(dataList))
	for _, item := range dataList {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		timeVal, _ := itemMap["time"].(float64)
		timeStr := fmt.Sprintf("%.0f", timeVal)

		parsedTime, err := ParseKLineTime(timeStr, KLineType(ktype))
		if err != nil {
			// 如果解析失败，使用当前时间
			parsedTime = time.Now()
		}

		kline := KLine{
			Time:     parsedTime,
			Open:     getFloat64(itemMap["open"]),
			High:     getFloat64(itemMap["high"]),
			Low:      getFloat64(itemMap["low"]),
			Close:    getFloat64(itemMap["close"]),
			Vol:      getFloat64(itemMap["vol"]),
			Turnover: getFloat64(itemMap["turnover"]),
			Tor:      getFloat64(itemMap["tor"]),
			PClose:   getFloat64(itemMap["pClose"]),
		}
		klines = append(klines, kline)
	}

	return klines, nil
}

// NewKLineWithMarket 使用市场转换价格
func NewKLineWithMarket(resp map[string]interface{}, market, ktype string) ([]KLine, error) {
	klines, err := ParseKLineResponse(resp, ktype)
	if err != nil {
		return nil, err
	}

	// 转换价格单位
	for i := range klines {
		klines[i].Open = ConvertPrice(klines[i].Open, market)
		klines[i].High = ConvertPrice(klines[i].High, market)
		klines[i].Low = ConvertPrice(klines[i].Low, market)
		klines[i].Close = ConvertPrice(klines[i].Close, market)
		klines[i].PClose = ConvertPrice(klines[i].PClose, market)
	}

	return klines, nil
}

// pow10 计算 10^n
func pow10(n int) float64 {
	result := 1.0
	for i := 0; i < n; i++ {
		result *= 10
	}
	return result
}

func getFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	}
	return 0
}
