package client

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// TestQueryKline_Integration 集成测试：获取真实K线数据
// 需要设置环境变量：
//   - FOSUN_BASE_URL: API基础地址
//   - FOSUN_API_KEY: API密钥
//   - FSOPENAPI_CLIENT_PRIVATE_KEY: 客户端私钥
//   - FSOPENAPI_SERVER_PUBLIC_KEY: 服务端公钥
//
// 或者使用配置文件路径环境变量：
//   - FOSUN_CONFIG_PATH: 配置文件路径
//
// 运行测试: go test -v -run TestQueryKline_Integration
func TestQueryKline_Integration(t *testing.T) {
	// 跳过短测试，这是一个需要真实API的集成测试
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	client, err := createTestClient(t)
	if err != nil {
		t.Skipf("无法创建测试客户端: %v (请设置必要的环境变量)", err)
		return
	}

	// 测试获取A股日K线
	t.Run("A股日K线", func(t *testing.T) {
		// 获取贵州茅台 (600519) 最近10条日K线
		klines, err := client.QueryKline("600519", string(KLineTypeDay), 10, 0, 0)
		if err != nil {
			t.Fatalf("获取K线失败: %v", err)
		}

		if len(klines) == 0 {
			t.Fatal("返回的K线数据为空")
		}

		// 验证K线数据
		for i, k := range klines {
			t.Logf("K线[%d]: 时间=%s, 开=%.2f, 高=%.2f, 低=%.2f, 收=%.2f, 量=%.0f",
				i, k.Time.Format("2006-01-02"), k.Open, k.High, k.Low, k.Close, k.Vol)

			// 基本验证
			if k.Open <= 0 || k.Close <= 0 {
				t.Errorf("K线[%d] 价格无效: 开=%.2f, 收=%.2f", i, k.Open, k.Close)
			}
			if k.High < k.Low {
				t.Errorf("K线[%d] 最高价小于最低价: 高=%.2f, 低=%.2f", i, k.High, k.Low)
			}
		}
	})

	// 测试获取分钟K线
	t.Run("A股分钟K线", func(t *testing.T) {
		// 获取平安银行 (000001) 最近5条1分钟K线
		klines, err := client.QueryKlineByMarket("sz", "000001", string(KLineType1Min), 5, 0, 0)
		if err != nil {
			t.Fatalf("获取分钟K线失败: %v", err)
		}

		if len(klines) == 0 {
			t.Log("分钟K线数据为空（可能非交易时间）")
			return
		}

		for i, k := range klines {
			t.Logf("分钟K线[%d]: 时间=%s, 开=%.2f, 收=%.2f",
				i, k.Time.Format("2006-01-02 15:04"), k.Open, k.Close)
		}
	})

	// 测试港股K线
	t.Run("港股日K线", func(t *testing.T) {
		// 获取腾讯控股 (00700) 最近5条日K线
		klines, err := client.QueryKlineByMarket("hk", "00700", string(KLineTypeDay), 5, 0, 0)
		if err != nil {
			t.Logf("获取港股K线失败（可能没有权限）: %v", err)
			return
		}

		for i, k := range klines {
			t.Logf("港股K线[%d]: 时间=%s, 开=%.2f, 收=%.2f",
				i, k.Time.Format("2006-01-02"), k.Open, k.Close)
		}
	})
}

// TestQueryKline_WithTimeRange 测试带时间范围的K线查询
func TestQueryKline_WithTimeRange(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	client, err := createTestClient(t)
	if err != nil {
		t.Skipf("无法创建测试客户端: %v", err)
		return
	}

	// 计算30天前的时间戳（毫秒）
	now := time.Now()
	endTime := now.UnixNano() / int64(time.Millisecond)
	startTime := now.AddDate(0, 0, -30).UnixNano() / int64(time.Millisecond)

	klines, err := client.QueryKline("600519", string(KLineTypeDay), 30, startTime, endTime)
	if err != nil {
		t.Fatalf("获取K线失败: %v", err)
	}

	t.Logf("获取到 %d 条K线数据", len(klines))

	// 验证时间范围
	for i, k := range klines {
		if k.Time.Before(now.AddDate(0, 0, -31)) || k.Time.After(now) {
			t.Errorf("K线[%d] 时间超出范围: %s", i, k.Time.Format("2006-01-02"))
		}
	}
}

// TestQueryKline_WithRateLimit 测试带限速的K线查询
func TestQueryKline_WithRateLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	// 创建带限速的客户端
	client, err := createTestClientWithRateLimit(t, 5, 2) // 5 RPS, burst 2
	if err != nil {
		t.Skipf("无法创建测试客户端: %v", err)
		return
	}

	start := time.Now()

	// 连续发送多个请求，验证限速生效
	for i := 0; i < 5; i++ {
		_, err := client.QueryKline("600519", string(KLineTypeDay), 5, 0, 0)
		if err != nil {
			t.Logf("请求 %d 失败: %v", i, err)
		} else {
			t.Logf("请求 %d 成功", i)
		}
	}

	elapsed := time.Since(start)
	t.Logf("5个请求耗时: %v (预期约600ms，限速5RPS，burst=2)", elapsed)
}

// createTestClient 创建测试客户端
func createTestClient(t *testing.T) (*OpenAPIClient, error) {
	return createTestClientWithRateLimit(t, 0, 0) // 不限速
}

// createTestClientWithRateLimit 创建带限速的测试客户端
func createTestClientWithRateLimit(t *testing.T, rateLimit, burst int) (*OpenAPIClient, error) {
	// 优先从环境变量读取配置
	baseURL := os.Getenv("FOSUN_BASE_URL")
	apiKey := os.Getenv("FOSUN_API_KEY")
	clientPrivateKey := os.Getenv("FSOPENAPI_CLIENT_PRIVATE_KEY")
	serverPublicKey := os.Getenv("FSOPENAPI_SERVER_PUBLIC_KEY")

	if baseURL == "" || apiKey == "" {
		return nil, fmt.Errorf("缺少必要的环境变量 FOSUN_BASE_URL 或 FOSUN_API_KEY")
	}

	// 处理 PEM 格式密钥，确保有正确的换行符
	clientPrivateKey = normalizePEM(clientPrivateKey)
	serverPublicKey = normalizePEM(serverPublicKey)

	cfg := Config{
		BaseURL:           baseURL,
		APIKey:            apiKey,
		ClientPrivateKey:  clientPrivateKey,
		ServerPublicKey:   serverPublicKey,
		RequestTimeout:    30,
		MaxRetries:        3,
		RateLimitRequests: rateLimit,
		RateLimitBurst:    burst,
	}

	return NewFromConfig(cfg)
}

// normalizePEM 处理 PEM 格式字符串，确保有正确的换行符
func normalizePEM(pem string) string {
	if pem == "" {
		return ""
	}
	// 移除所有空白字符
	pem = strings.ReplaceAll(pem, "\r", "")
	pem = strings.ReplaceAll(pem, "\n", "")

	// 使用正则表达式处理 PEM 格式
	// 处理私钥
	if strings.Contains(pem, "-----BEGIN PRIVATE KEY-----") {
		pem = strings.Replace(pem, "-----BEGIN PRIVATE KEY-----", "-----BEGIN PRIVATE KEY-----\n", 1)
		pem = strings.Replace(pem, "-----END PRIVATE KEY-----", "\n-----END PRIVATE KEY-----", 1)
	}
	// 处理公钥
	if strings.Contains(pem, "-----BEGIN PUBLIC KEY-----") {
		pem = strings.Replace(pem, "-----BEGIN PUBLIC KEY-----", "-----BEGIN PUBLIC KEY-----\n", 1)
		pem = strings.Replace(pem, "-----END PUBLIC KEY-----", "\n-----END PUBLIC KEY-----", 1)
	}

	return pem
}

// TestParseKLineTime 测试时间解析
func TestParseKLineTime(t *testing.T) {
	tests := []struct {
		name     string
		timeStr  string
		ktype    KLineType
		expected string
		hasError bool
	}{
		{
			name:     "日K线时间",
			timeStr:  "20260401",
			ktype:    KLineTypeDay,
			expected: "2026-04-01",
			hasError: false,
		},
		{
			name:     "分钟K线时间",
			timeStr:  "202604010930",
			ktype:    KLineType1Min,
			expected: "2026-04-01 09:30",
			hasError: false,
		},
		{
			name:     "周K线时间",
			timeStr:  "20260325",
			ktype:    KLineTypeWeek,
			expected: "2026-03-25",
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseKLineTime(tt.timeStr, tt.ktype)
			if tt.hasError {
				if err == nil {
					t.Errorf("期望返回错误，但没有")
				}
				return
			}
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			formatted := result.Format("2006-01-02 15:04")
			if tt.ktype == KLineTypeDay || tt.ktype == KLineTypeWeek || tt.ktype == KLineTypeMonth {
				formatted = result.Format("2006-01-02")
			}
			if formatted != tt.expected {
				t.Errorf("期望 %s, 得到 %s", tt.expected, formatted)
			}
		})
	}
}

// TestConvertPrice 测试价格转换
func TestConvertPrice(t *testing.T) {
	tests := []struct {
		name     string
		price    float64
		market   string
		expected float64
	}{
		{"A股价格", 185000, "sh", 1850.00},
		{"深圳价格", 123456, "sz", 1234.56},
		{"港股价格", 350000, "hk", 350.00},
		{"美股价格", 1500000, "us", 150.00},
		{"未知市场", 1000, "unknown", 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertPrice(tt.price, tt.market)
			if result != tt.expected {
				t.Errorf("ConvertPrice(%v, %s) = %v, 期望 %v",
					tt.price, tt.market, result, tt.expected)
			}
		})
	}
}