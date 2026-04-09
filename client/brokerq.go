package client

import "fmt"

// BrokerqItem 五档买卖盘档位
type BrokerqItem struct {
	Price      float64 // 价格（转换后）
	Vol        float64 // 委托量
	OrderCount int     // 委托笔数
}

// BrokerqResponse 五档买卖盘响应
type BrokerqResponse struct {
	Code  string        // 证券代码
	Bid   []BrokerqItem // 买盘（5档，按价格降序）
	Ask   []BrokerqItem // 卖盘（5档，按价格升序）
	Power int           // 价格精度
}

// QueryBrokerq 查询五档买卖盘
// market: "sh"/"sz"/"hk"/"us"/"bj"
// code: 纯证券代码（如 600519, 00700, AAPL）
func (c *OpenAPIClient) QueryBrokerq(market, code string) (*BrokerqResponse, error) {
	fullCode := market + code

	params := map[string]string{
		"code":   fullCode,
		"market": market,
	}

	resp, err := c.Get("/v1/market/secu/brokerq", params)
	if err != nil {
		return nil, err
	}

	return ParseBrokerqResponse(resp, market)
}

// ParseBrokerqResponse 解析五档买卖盘响应
func ParseBrokerqResponse(resp map[string]interface{}, market string) (*BrokerqResponse, error) {
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("无效的响应数据")
	}

	code, _ := data["code"].(string)
	power := int(getFloat64(data["power"]))

	parseItems := func(key string) []BrokerqItem {
		list, ok := data[key].([]interface{})
		if !ok {
			return nil
		}
		items := make([]BrokerqItem, 0, len(list))
		for _, v := range list {
			m, ok := v.(map[string]interface{})
			if !ok {
				continue
			}
			price := getFloat64(m["price"])
			// 优先使用 power 动态换算，否则 fallback 到 ConvertPrice
			if power > 0 {
				price = price / pow10(power)
			} else {
				price = ConvertPrice(price, market)
			}
			items = append(items, BrokerqItem{
				Price:      price,
				Vol:        getFloat64(m["vol"]),
				OrderCount: int(getFloat64(m["orderCount"])),
			})
		}
		return items
	}

	return &BrokerqResponse{
		Code:  code,
		Bid:   parseItems("bid"),
		Ask:   parseItems("ask"),
		Power: power,
	}, nil
}
