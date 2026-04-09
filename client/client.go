package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/HubbleVision/fosun_go_sdk/auth"
	"github.com/HubbleVision/fosun_go_sdk/crypto"

	"github.com/google/uuid"
)

// OpenAPIClient 提供与 FosunXCZ OpenAPI 交互的客户端
type OpenAPIClient struct {
	BaseURL     string
	APIKey      string
	AuthManager *auth.SessionManager
	HTTPClient  *http.Client
	Host        string
	BasePath    string
	APIPrefix   string
	MaxRetries  int
}

// Config 客户端配置
type Config struct {
	BaseURL          string
	APIKey           string
	ClientPrivateKey string
	ServerPublicKey  string
	SDKType          string
	RequestTimeout   int
	MaxRetries       int
}

// NewFromConfig 使用配置创建客户端
func NewFromConfig(cfg Config) (*OpenAPIClient, error) {
	if cfg.BaseURL == "" {
		return nil, errors.New("baseURL is required")
	}
	if cfg.APIKey == "" {
		return nil, errors.New("apiKey is required")
	}

	// 设置默认值
	if cfg.RequestTimeout == 0 {
		cfg.RequestTimeout = 15
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}

	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid baseURL: %v", err)
	}

	authManager, err := auth.NewSessionManagerWithConfig(baseURL, cfg.APIKey, cfg.ClientPrivateKey, cfg.ServerPublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth manager: %v", err)
	}

	// 设置SDK类型
	if cfg.SDKType != "" {
		os.Setenv("SDK_TYPE", cfg.SDKType)
	}

	return &OpenAPIClient{
		BaseURL:     baseURL,
		APIKey:      cfg.APIKey,
		AuthManager: authManager,
		HTTPClient:  &http.Client{Timeout: time.Duration(cfg.RequestTimeout) * time.Second},
		Host:        parsedURL.Host,
		BasePath:    strings.TrimRight(parsedURL.Path, "/"),
		APIPrefix:   authManager.APIPrefix,
		MaxRetries:  cfg.MaxRetries,
	}, nil
}

// NewOpenAPIClient 创建一个新的 OpenAPI 客户端
func NewOpenAPIClient(baseURL, apiKey string) (*OpenAPIClient, error) {
	return NewFromConfig(Config{
		BaseURL:        baseURL,
		APIKey:         apiKey,
		RequestTimeout: 15,
		MaxRetries:     3,
	})
}

// Request 发送带加密和签名的请求
func (c *OpenAPIClient) requestInternal(method, path string, data interface{}, params map[string]string, allowRetry bool) (map[string]interface{}, error) {
	if path == "" {
		return nil, errors.New("path is required")
	}

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	requestPath := fmt.Sprintf("%s%s", c.APIPrefix, path)
	reqURL := fmt.Sprintf("%s%s", c.BaseURL, requestPath)

	sessionID, signingKey, encryptionKey, err := c.AuthManager.GetValidSession()
	if err != nil {
		return nil, fmt.Errorf("failed to get valid session: %v", err)
	}

	timestamp := fmt.Sprintf("%d", time.Now().UnixNano()/int64(time.Millisecond))
	requestID := uuid.New().String()
	nonce := strings.ReplaceAll(uuid.New().String(), "-", "")

	queryStr := ""
	if len(params) > 0 {
		keys := make([]string, 0, len(params))
		for k := range params {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		queryParts := make([]string, 0, len(keys))
		for _, k := range keys {
			queryParts = append(queryParts, fmt.Sprintf("%s=%s", k, params[k]))
		}
		queryStr = strings.Join(queryParts, "&")
	}

	fullSignPath := fmt.Sprintf("%s%s", c.BasePath, requestPath)

	isMarketPath := strings.Contains(requestPath, "/market/") || strings.Contains(requestPath, "/optmarket/")
	isEncrypted := data != nil && !isMarketPath

	reqPayload := map[string]interface{}{
		"encrypted": false,
		"content":   data,
	}
	if data == nil {
		reqPayload["content"] = map[string]interface{}{}
	}

	if isEncrypted {
		plaintextBytes, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal data: %v", err)
		}

		aadStr := crypto.BuildResponseAAD(sessionID, timestamp, nonce)

		ivB64, ciphertextB64, tagB64, err := crypto.EncryptBody(
			encryptionKey, plaintextBytes, []byte(aadStr),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt body: %v", err)
		}

		reqPayload = map[string]interface{}{
			"encrypted": true,
			"iv":        ivB64,
			"tag":       tagB64,
			"content":   ciphertextB64,
		}
	}

	finalBodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %v", err)
	}

	signature := crypto.Sign(
		signingKey, method, fullSignPath, queryStr, timestamp, nonce, finalBodyBytes,
	)

	if queryStr != "" {
		reqURL = fmt.Sprintf("%s?%s", reqURL, queryStr)
	}

	req, err := http.NewRequest(strings.ToUpper(method), reqURL, bytes.NewBuffer(finalBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.APIKey)
	req.Header.Set("X-session", sessionID)
	req.Header.Set("X-Request-Id", requestID)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Signature", signature)

	// 添加重试机制
	var resp *http.Response
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		resp, err = c.HTTPClient.Do(req)
		if err == nil {
			break
		}
		if i < maxRetries-1 {
			time.Sleep(2 * time.Second)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("request failed after retries: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Session 过期时自动续期重试一次
		if allowRetry && resp.StatusCode == 401 && bytes.Contains(bodyBytes, []byte("Session expired")) {
			resp.Body.Close()
			c.AuthManager.InvalidateSession()
			return c.requestInternal(method, path, data, params, false)
		}
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var respPayload map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &respPayload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	var respData interface{}
	if _, ok := respPayload["encrypted"]; !ok {
		respData = respPayload
	} else {
		isRespEncrypted, _ := respPayload["encrypted"].(bool)
		contentObj := respPayload["content"]

		if isRespEncrypted {
			ivB64, _ := respPayload["iv"].(string)
			tagB64, _ := respPayload["tag"].(string)
			ciphertextB64, _ := contentObj.(string)

			aadStr := crypto.BuildResponseAAD(sessionID, timestamp, nonce)

			decryptedBytes, err := crypto.DecryptBody(
				encryptionKey, ivB64, ciphertextB64, tagB64, []byte(aadStr),
			)
			if err != nil {
				return nil, fmt.Errorf("response decryption failed: %v", err)
			}

			if err := json.Unmarshal(decryptedBytes, &respData); err != nil {
				return nil, fmt.Errorf("failed to parse decrypted JSON: %v", err)
			}
		} else {
			respData = contentObj
		}
	}

	dataMap, ok := respData.(map[string]interface{})
	if !ok {
		return map[string]interface{}{"data": respData}, nil
	}

	if code, ok := dataMap["code"].(float64); ok && code != 0 {
		msg, _ := dataMap["message"].(string)
		return nil, fmt.Errorf("business error (code %v): %s", code, msg)
	}

	return dataMap, nil
}

// Request 发送带加密和签名的请求，Session 过期时自动续期重试
func (c *OpenAPIClient) Request(method, path string, data interface{}, params map[string]string) (map[string]interface{}, error) {
	return c.requestInternal(method, path, data, params, true)
}

// Post 发送 POST 请求
func (c *OpenAPIClient) Post(path string, data interface{}) (map[string]interface{}, error) {
	return c.Request("POST", path, data, nil)
}

// QueryKline 查询 K线数据（默认上海市场）
// market: "sh"(上海)/"sz"(深圳)/"hk"(港股)/"us"(美股)
// ktype: "min1"/"min5"/"min15"/"min30"/"min60"/"day"/"week"/"month"
// right: "NOR"(不复权)/"FQ"(前复权)/"DR"(后复权)
func (c *OpenAPIClient) QueryKline(code, ktype string, num int, startTime, endTime int64) ([]KLine, error) {
	return c.QueryKlineByMarket("sh", code, ktype, num, startTime, endTime)
}

// QueryKlineByMarket 按市场查询K线，返回解析后的K线数据
func (c *OpenAPIClient) QueryKlineByMarket(market, code, ktype string, num int, startTime, endTime int64) ([]KLine, error) {
	resp, err := c.queryKlineWithMarket(market, code, ktype, num, startTime, endTime)
	if err != nil {
		return nil, err
	}
	return NewKLineWithMarket(resp, market, ktype)
}

func (c *OpenAPIClient) queryKlineWithMarket(market, code, ktype string, num int, startTime, endTime int64) (map[string]interface{}, error) {
	fullCode := code
	if market == "sh" {
		fullCode = "sh" + code
	} else if market == "sz" {
		fullCode = "sz" + code
	} else if market == "hk" {
		fullCode = "hk" + code
	} else if market == "us" {
		fullCode = "us" + code
	}

	data := map[string]interface{}{
		"code":       fullCode,
		"ktype":      ktype,
		"num":        num,
		"startTime":  startTime,
		"endTime":    endTime,
		"delay":      true,
		"right":      "DR",
		"suspension": 0,
		"time":       0,
	}
	return c.Request("POST", "/v1/market/kline", data, nil)
}

// Get 发送 GET 请求
func (c *OpenAPIClient) Get(path string, params map[string]string) (map[string]interface{}, error) {
	return c.Request("GET", path, nil, params)
}
