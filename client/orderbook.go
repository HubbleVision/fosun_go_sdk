package client

import "fmt"

// OrderbookItem 十档买卖盘档位
type OrderbookItem struct {
	Price float64 // 价格（转换后）
	Vol   float64 // 委托量
}

// OrderbookResponse 十档买卖盘响应
type OrderbookResponse struct {
	Code  string          // 证券代码
	Bid   []OrderbookItem // 买盘（10档）
	Ask   []OrderbookItem // 卖盘（10档）
	Power int             // 价格精度
}

// QueryOrderbook 查询十档买卖盘
// market: "sh"/"sz"/"hk"/"us"/"bj"
// code: 纯证券代码（如 600519, 00700, AAPL）
func (c *OpenAPIClient) QueryOrderbook(market, code string) (*OrderbookResponse, error) {
	fullCode := market + code

	params := map[string]string{
		"code":   fullCode,
		"market": market,
	}

	resp, err := c.Get("/v1/market/secu/orderbook", params)
	if err != nil {
		return nil, err
	}

	return ParseOrderbookResponse(resp, market)
}

// ParseOrderbookResponse 解析十档买卖盘响应
func ParseOrderbookResponse(resp map[string]interface{}, market string) (*OrderbookResponse, error) {
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("无效的响应数据")
	}

	code, _ := data["code"].(string)
	power := int(getFloat64(data["power"]))

	parseItems := func(key string) []OrderbookItem {
		list, ok := data[key].([]interface{})
		if !ok {
			return nil
		}
		items := make([]OrderbookItem, 0, len(list))
		for _, v := range list {
			m, ok := v.(map[string]interface{})
			if !ok {
				continue
			}
			price := getFloat64(m["price"])
			if power > 0 {
				price = price / pow10(power)
			} else {
				price = ConvertPrice(price, market)
			}
			items = append(items, OrderbookItem{
				Price: price,
				Vol:   getFloat64(m["vol"]),
			})
		}
		return items
	}

	return &OrderbookResponse{
		Code:  code,
		Bid:   parseItems("bid"),
		Ask:   parseItems("ask"),
		Power: power,
	}, nil
}
