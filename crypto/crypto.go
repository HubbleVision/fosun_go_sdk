package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/hkdf"
)

// CryptoManager 提供了加密、解密、签名和验证的功能
type CryptoManager struct{}

// LoadIdentityPrivateKey 从 PEM 字符串加载 ECDSA P-384 长期私钥
func LoadIdentityPrivateKey(pemString string) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemString))
	if block == nil {
		return nil, errors.New("failed to decode PEM block containing private key")
	}

	priv, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// 尝试解析为 EC 私钥
		priv, err = x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
	}

	ecdsaPriv, ok := priv.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("not an ECDSA private key")
	}

	return ecdsaPriv, nil
}

// LoadIdentityPublicKey 从 PEM 字符串加载 ECDSA P-384 长期公钥
func LoadIdentityPublicKey(pemString string) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemString))
	if block == nil {
		return nil, errors.New("failed to decode PEM block containing public key")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	ecdsaPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("not an ECDSA public key")
	}

	return ecdsaPub, nil
}

// GenerateECDHKeyPair 生成 secp384r1 (P-384) 曲线的 ECDH 密钥对
func GenerateECDHKeyPair() (*ecdsa.PrivateKey, string, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		return nil, "", err
	}

	// 获取未压缩格式的公钥
	publicBytes := elliptic.Marshal(elliptic.P384(), privateKey.PublicKey.X, privateKey.PublicKey.Y)

	// Python SDK 中的特殊处理
	if len(publicBytes) > 97 {
		publicBytes = publicBytes[len(publicBytes)-97:]
	} else if len(publicBytes) == 96 {
		publicBytes = append([]byte{0x04}, publicBytes...)
	}

	return privateKey, base64.StdEncoding.EncodeToString(publicBytes), nil
}

// SignHandshake 使用客户端长期私钥对 SHA256(eph_pub_bytes + nonce_bytes) 进行 ECDSA 签名
func SignHandshake(privateKey *ecdsa.PrivateKey, ephPubB64, nonceB64 string) (string, error) {
	ephPubBytes, err := base64.StdEncoding.DecodeString(ephPubB64)
	if err != nil {
		return "", err
	}

	nonceBytes, err := base64.StdEncoding.DecodeString(nonceB64)
	if err != nil {
		return "", err
	}

	dataToSign := append(ephPubBytes, nonceBytes...)
	hash := sha256.Sum256(dataToSign)

	// 在 Go 1.15+ 中可以使用 ecdsa.SignASN1
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, hash[:])
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(signature), nil
}

// VerifyHandshake 使用服务端长期公钥验证服务端的握手签名
func VerifyHandshake(publicKey *ecdsa.PublicKey, ephPubB64, nonceB64, signatureB64 string) bool {
	ephPubBytes, err := base64.StdEncoding.DecodeString(ephPubB64)
	if err != nil {
		return false
	}

	nonceBytes, err := base64.StdEncoding.DecodeString(nonceB64)
	if err != nil {
		return false
	}

	signature, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return false
	}

	dataToVerify := append(ephPubBytes, nonceBytes...)
	hash := sha256.Sum256(dataToVerify)

	return ecdsa.VerifyASN1(publicKey, hash[:], signature)
}

// ComputeSharedSecret 计算共享密钥并通过 HKDF 一次派生 enc_key(32) + mac_key(32)
func ComputeSharedSecret(privateKey *ecdsa.PrivateKey, serverPublicKeyB64, clientNonceB64, serverNonceB64 string) ([]byte, []byte, error) {
	serverPublicBytes, err := base64.StdEncoding.DecodeString(serverPublicKeyB64)
	if err != nil {
		return nil, nil, err
	}

	// 解析服务端的公钥
	x, y := elliptic.Unmarshal(elliptic.P384(), serverPublicBytes)
	if x == nil {
		return nil, nil, errors.New("failed to unmarshal server public key")
	}

	// 计算共享密钥 (ECDH)
	sharedKey, _ := elliptic.P384().ScalarMult(x, y, privateKey.D.Bytes())

	clientNonce, err := base64.StdEncoding.DecodeString(clientNonceB64)
	if err != nil {
		return nil, nil, err
	}

	serverNonce, err := base64.StdEncoding.DecodeString(serverNonceB64)
	if err != nil {
		return nil, nil, err
	}

	salt := append(clientNonce, serverNonce...)
	info := []byte("session-derivation")

	// 使用 HKDF 派生密钥
	hkdfReader := hkdf.New(sha256.New, sharedKey.Bytes(), salt, info)
	derivedBytes := make([]byte, 64)
	if _, err := hkdfReader.Read(derivedBytes); err != nil {
		return nil, nil, err
	}

	encKey := derivedBytes[:32]
	macKey := derivedBytes[32:]

	// 返回顺序：signing_key, encryption_key
	return macKey, encKey, nil
}

// BuildResponseAAD 构造响应解密使用的 AAD
func BuildResponseAAD(sessionID, timestamp, nonce string) string {
	return fmt.Sprintf("X-session:%s|X-timestamp:%s|X-nonce:%s", sessionID, timestamp, nonce)
}

// EncryptBody 使用 AES-256-GCM 加密 Body
func EncryptBody(encryptionKey, plaintextBytes, aadBytes []byte) (string, string, string, error) {
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", "", "", err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", "", err
	}

	iv := make([]byte, 12)
	if _, err := rand.Read(iv); err != nil {
		return "", "", "", err
	}

	ciphertextWithTag := aesgcm.Seal(nil, iv, plaintextBytes, aadBytes)
	
	// 分离 ciphertext 和 tag
	tagLen := 16
	ciphertext := ciphertextWithTag[:len(ciphertextWithTag)-tagLen]
	tag := ciphertextWithTag[len(ciphertextWithTag)-tagLen:]

	return base64.StdEncoding.EncodeToString(iv),
		base64.StdEncoding.EncodeToString(ciphertext),
		base64.StdEncoding.EncodeToString(tag), nil
}

// DecryptBody 使用 AES-256-GCM 解密 Body
func DecryptBody(encryptionKey []byte, ivB64, ciphertextB64, tagB64 string, aadBytes []byte) ([]byte, error) {
	iv, err := base64.StdEncoding.DecodeString(ivB64)
	if err != nil {
		return nil, err
	}

	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return nil, err
	}

	tag, err := base64.StdEncoding.DecodeString(tagB64)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// 在 Go 中，GCM 的 Open 方法期望 ciphertext 和 tag 拼接在一起
	ciphertextWithTag := append(ciphertext, tag...)
	return aesgcm.Open(nil, iv, ciphertextWithTag, aadBytes)
}

// Sign 计算请求 HMAC-SHA256 签名
func Sign(sessionKey []byte, method, path, query, timestamp, nonce string, bodyBytes []byte) string {
	var bodyShaHex string
	if len(bodyBytes) > 0 {
		hash := sha256.Sum256(bodyBytes)
		bodyShaHex = hex.EncodeToString(hash[:])
	} else {
		hash := sha256.Sum256([]byte(""))
		bodyShaHex = hex.EncodeToString(hash[:])
	}

	canonicalString := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		strings.ToUpper(method), path, query, timestamp, nonce, bodyShaHex)

	mac := hmac.New(sha256.New, sessionKey)
	mac.Write([]byte(canonicalString))
	signatureBytes := mac.Sum(nil)

	return base64.StdEncoding.EncodeToString(signatureBytes)
}
