package auth

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/HubbleVision/fosun_go_sdk/crypto"

	"github.com/google/uuid"
)

// SessionManager 管理 API 会话
type SessionManager struct {
	BaseURL            string
	APIKey             string
	APIPrefix          string
	SessionID          string
	SigningKey         []byte
	EncryptionKey      []byte
	ExpiresAt          int64
	PrivateKey         *ecdsa.PrivateKey
	ClientNonce        string
	ServerNonce        string
	ServerPubKey       string
	IdentityPrivateKey *ecdsa.PrivateKey
	IdentityPublicKey  *ecdsa.PublicKey
	HTTPClient         *http.Client
}

// NewSessionManager 创建一个新的 SessionManager
func NewSessionManager(baseURL, apiKey string) (*SessionManager, error) {
	return NewSessionManagerWithConfig(baseURL, apiKey, "", "")
}

// NewSessionManagerWithConfig 使用指定的密钥创建 SessionManager
func NewSessionManagerWithConfig(baseURL, apiKey, clientPrivPEM, serverPubPEM string) (*SessionManager, error) {
	baseURL = strings.TrimRight(baseURL, "/")

	sm := &SessionManager{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		APIPrefix:  resolveAPIPrefix(),
		HTTPClient: &http.Client{Timeout: 15 * time.Second},
	}

	// 如果没有传入密钥，从环境变量读取
	if clientPrivPEM == "" {
		clientPrivPEM = os.Getenv("FSOPENAPI_CLIENT_PRIVATE_KEY")
	}
	if serverPubPEM == "" {
		serverPubPEM = os.Getenv("FSOPENAPI_SERVER_PUBLIC_KEY")
	}

	if clientPrivPEM != "" && serverPubPEM != "" {
		privKey, err := crypto.LoadIdentityPrivateKey(clientPrivPEM)
		if err != nil {
			return nil, fmt.Errorf("failed to load identity private key: %v", err)
		}
		sm.IdentityPrivateKey = privKey

		pubKey, err := crypto.LoadIdentityPublicKey(serverPubPEM)
		if err != nil {
			return nil, fmt.Errorf("failed to load identity public key: %v", err)
		}
		sm.IdentityPublicKey = pubKey
	}

	return sm, nil
}

func resolveAPIPrefix() string {
	sdkType := strings.TrimSpace(strings.ToLower(os.Getenv("SDK_TYPE")))
	if sdkType == "ops" {
		return "/api/ops"
	}
	return "/api"
}

// CreateSession 执行 ECDH+ECDSA 握手创建会话
func (sm *SessionManager) CreateSession() error {
	if sm.IdentityPrivateKey == nil || sm.IdentityPublicKey == nil {
		return errors.New("Missing FSOPENAPI_CLIENT_PRIVATE_KEY or FSOPENAPI_SERVER_PUBLIC_KEY environment variables")
	}

	var err error
	var clientPubKey string
	sm.PrivateKey, clientPubKey, err = crypto.GenerateECDHKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate ECDH key pair: %v", err)
	}

	nonceBytes := make([]byte, 32)
	if _, err := rand.Read(nonceBytes); err != nil {
		return fmt.Errorf("failed to generate client nonce: %v", err)
	}
	sm.ClientNonce = base64.StdEncoding.EncodeToString(nonceBytes)

	signature, err := crypto.SignHandshake(sm.IdentityPrivateKey, clientPubKey, sm.ClientNonce)
	if err != nil {
		return fmt.Errorf("failed to sign handshake: %v", err)
	}

	url := fmt.Sprintf("%s%s/v1/auth/SessionCreate", sm.BaseURL, sm.APIPrefix)
	requestID := uuid.New().String()

	payload := map[string]string{
		"apiKey":              sm.APIKey,
		"clientTempPublicKey": clientPubKey,
	}
	payloadBytes, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return err
	}

	timestamp := fmt.Sprintf("%d", time.Now().UnixNano()/int64(time.Millisecond))

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", sm.APIKey)
	req.Header.Set("X-Request-Id", requestID)
	req.Header.Set("X-timestamp", timestamp)
	req.Header.Set("X-source", "go-sdk")
	req.Header.Set("X-product", "sdk")
	req.Header.Set("X-lang", "zh-CN")
	req.Header.Set("X-Nonce", sm.ClientNonce)
	req.Header.Set("X-Signature", signature)

	// 重试机制
	var resp *http.Response
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		resp, err = sm.HTTPClient.Do(req)
		if err == nil {
			break
		}
		if i < maxRetries-1 {
			time.Sleep(2 * time.Second)
		}
	}

	if err != nil {
		return fmt.Errorf("session create request failed after retries: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("session create failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var respData map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &respData); err != nil {
		return fmt.Errorf("failed to unmarshal response: %v", err)
	}

	var data map[string]interface{}
	if content, ok := respData["content"]; ok && content != nil {
		data = content.(map[string]interface{})
	} else {
		data = respData
	}

	if code, ok := data["code"].(float64); ok && code != 0 {
		msg, _ := data["message"].(string)
		return fmt.Errorf("handshake failed with code %v: %s", code, msg)
	}

	sessionData, ok := data["data"].(map[string]interface{})
	if !ok {
		return errors.New("missing data in response")
	}

	sm.SessionID, _ = sessionData["sessionId"].(string)
	serverPubKey, _ := sessionData["serverTempPublicKey"].(string)
	sm.ServerPubKey = serverPubKey

	if expiresAt, ok := sessionData["expiresAt"].(float64); ok {
		sm.ExpiresAt = int64(expiresAt)
	}

	serverSignature := resp.Header.Get("X-Signature")
	sm.ServerNonce = resp.Header.Get("X-Nonce")

	if serverSignature == "" || sm.ServerNonce == "" {
		return errors.New("missing server signature or nonce in response headers")
	}

	if !crypto.VerifyHandshake(sm.IdentityPublicKey, serverPubKey, sm.ServerNonce, serverSignature) {
		return errors.New("invalid server signature in handshake")
	}

	sm.SigningKey, sm.EncryptionKey, err = crypto.ComputeSharedSecret(
		sm.PrivateKey, serverPubKey, sm.ClientNonce, sm.ServerNonce,
	)
	if err != nil {
		return fmt.Errorf("failed to compute shared secret: %v", err)
	}

	return nil
}

// GetValidSession 获取有效会话，如果过期则重新创建
func (sm *SessionManager) GetValidSession() (string, []byte, []byte, error) {
	now := time.Now().Unix()
	// 如果没有 session 或者快过期了（提前 60 秒）
	if sm.SessionID == "" || (sm.ExpiresAt > 0 && now >= (sm.ExpiresAt-60)) {
		if err := sm.CreateSession(); err != nil {
			return "", nil, nil, err
		}
	}
	return sm.SessionID, sm.SigningKey, sm.EncryptionKey, nil
}
