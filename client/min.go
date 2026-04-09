package client

import (
	"fmt"
	"time"
)

// MinData 分时数据
type MinData struct {
	Time     time.Time // 时间
	Price    float64   // 当前价
	Avg      float64   // 均价
	Vol      float64   // 成交量
	Turnover float64   // 成交额
	ChgVal   float64   // 涨跌额
	ChgPct   float64   // 涨跌幅(%)
}

// MinResponse 分时数据响应
type MinResponse struct {
	Code   string    // 证券代码
	Data   []MinData // 分时数据列表
	PClose float64   // 昨收价
}

// QueryMarketMin 查询分时数据
// market: "sh"/"sz"/"hk"/"us"/"bj"
// code: 纯证券代码（如 600519, 00700, AAPL）
func (c *OpenAPIClient) QueryMarketMin(market, code string) (*MinResponse, error) {
	fullCode := market + code

	params := map[string]string{
		"code":   fullCode,
		"market": market,
	}

	resp, err := c.Get("/v1/market/min", params)
	if err != nil {
		return nil, err
	}

	return ParseMinResponse(resp, market)
}

// ParseMinResponse 解析分时数据响应
func ParseMinResponse(resp map[string]interface{}, market string) (*MinResponse, error) {
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("无效的响应数据")
	}

	code, _ := data["code"].(string)
	pClose := ConvertPrice(getFloat64(data["pClose"]), market)

	dataList, ok := data["data"].([]interface{})
	if !ok {
		return &MinResponse{Code: code, PClose: pClose}, nil
	}

	items := make([]MinData, 0, len(dataList))
	for _, item := range dataList {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// 解析时间（YYYYMMDDHHMM 格式）
		timeVal, _ := itemMap["time"].(float64)
		timeStr := fmt.Sprintf("%.0f", timeVal)
		parsedTime, err := ParseKLineTime(timeStr, KLineType1Min)
		if err != nil {
			parsedTime = time.Now()
		}

		items = append(items, MinData{
			Time:     parsedTime,
			Price:    ConvertPrice(getFloat64(itemMap["price"]), market),
			Avg:      ConvertPrice(getFloat64(itemMap["avg"]), market),
			Vol:      getFloat64(itemMap["vol"]),
			Turnover: getFloat64(itemMap["turnover"]),
			ChgVal:   ConvertPrice(getFloat64(itemMap["chgVal"]), market),
			ChgPct:   getFloat64(itemMap["chgPct"]),
		})
	}

	return &MinResponse{
		Code:   code,
		Data:   items,
		PClose: pClose,
	}, nil
}
