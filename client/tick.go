package client

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// TickItem 逐笔成交
type TickItem struct {
	Time      time.Time // 成交时间
	Price     float64   // 成交价（转换后）
	Vol       float64   // 成交量
	Turnover  float64   // 成交额
	Direction int       // 方向: 1=买, 2=卖, 0=中性
}

// TickResponse 逐笔成交响应
type TickResponse struct {
	Code     string     // 证券代码
	LastTime string     // 最后一笔时间戳
	Ticks    []TickItem // 逐笔成交列表
	Power    int        // 价格精度
}

// QueryTick 查询逐笔成交
// market: "sh"/"sz"/"hk"/"us"/"bj"
// code: 纯证券代码（如 600519, 00700, AAPL）
// lastTime: 上一笔时间戳，用于增量拉取（可选）
// count: 返回条数限制（可选，0 表示使用默认值）
func (c *OpenAPIClient) QueryTick(market, code string, lastTime string, count int) (*TickResponse, error) {
	fullCode := market + code

	params := map[string]string{
		"code":   fullCode,
		"market": market,
	}
	if lastTime != "" {
		params["lastTime"] = lastTime
	}
	if count > 0 {
		params["count"] = strconv.Itoa(count)
	}

	resp, err := c.Get("/v1/market/secu/tick", params)
	if err != nil {
		return nil, err
	}

	return ParseTickResponse(resp, market)
}

// ParseTickResponse 解析逐笔成交响应
func ParseTickResponse(resp map[string]interface{}, market string) (*TickResponse, error) {
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("无效的响应数据")
	}

	code, _ := data["code"].(string)
	lastTime, _ := data["lastTime"].(string)
	power := int(getFloat64(data["power"]))

	tickList, ok := data["ticks"].([]interface{})
	if !ok {
		return &TickResponse{Code: code, LastTime: lastTime, Power: power}, nil
	}

	ticks := make([]TickItem, 0, len(tickList))
	for _, v := range tickList {
		m, ok := v.(map[string]interface{})
		if !ok {
			continue
		}

		// 解析时间: 支持字符串 "YYYYMMDDHHMMSSMMM" 或 float64
		tickTime := parseTickTime(m["time"])

		price := getFloat64(m["price"])
		if power > 0 {
			price = price / pow10(power)
		} else {
			price = ConvertPrice(price, market)
		}

		ticks = append(ticks, TickItem{
			Time:      tickTime,
			Price:     price,
			Vol:       getFloat64(m["vol"]),
			Turnover:  getFloat64(m["turnover"]),
			Direction: int(getFloat64(m["direction"])),
		})
	}

	return &TickResponse{
		Code:     code,
		LastTime: lastTime,
		Ticks:    ticks,
		Power:    power,
	}, nil
}

// parseTickTime 解析逐笔成交时间
// 支持 "YYYYMMDDHHMMSSMMM" 字符串格式或 float64 数字
func parseTickTime(v interface{}) time.Time {
	switch val := v.(type) {
	case string:
		return parseTickTimeString(val)
	case float64:
		return parseTickTimeString(fmt.Sprintf("%.0f", val))
	}
	return time.Time{}
}

// parseTickTimeString 解析毫秒级时间字符串 "YYYYMMDDHHMMSSMMM"
func parseTickTimeString(timeStr string) time.Time {
	timeStr = strings.TrimSpace(timeStr)

	// YYYYMMDDHHMMSSMMM (17位)
	if len(timeStr) >= 17 {
		year, _ := strconv.Atoi(timeStr[0:4])
		month, _ := strconv.Atoi(timeStr[4:6])
		day, _ := strconv.Atoi(timeStr[6:8])
		hour, _ := strconv.Atoi(timeStr[8:10])
		min, _ := strconv.Atoi(timeStr[10:12])
		sec, _ := strconv.Atoi(timeStr[12:14])
		msec, _ := strconv.Atoi(timeStr[14:17])
		return time.Date(year, time.Month(month), day, hour, min, sec, msec*int(time.Millisecond), time.Local)
	}

	// Fallback: 尝试 YYYYMMDDHHMMSS (14位)
	if len(timeStr) >= 14 {
		year, _ := strconv.Atoi(timeStr[0:4])
		month, _ := strconv.Atoi(timeStr[4:6])
		day, _ := strconv.Atoi(timeStr[6:8])
		hour, _ := strconv.Atoi(timeStr[8:10])
		min, _ := strconv.Atoi(timeStr[10:12])
		sec, _ := strconv.Atoi(timeStr[12:14])
		return time.Date(year, time.Month(month), day, hour, min, sec, 0, time.Local)
	}

	return time.Time{}
}
