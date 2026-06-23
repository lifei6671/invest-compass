package datasourcecredential

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const keyByteLength = 32

// KeyProvider 为凭据加密提供 32 字节本地密钥。
type KeyProvider interface {
	Key() ([]byte, error)
}

// FileKeyProvider 从工作区文件读取或创建本地凭据加密密钥。
type FileKeyProvider struct {
	Path string
}

// Key 返回本地凭据加密密钥，首次使用时创建 owner-only 权限文件。
func (provider FileKeyProvider) Key() ([]byte, error) {
	path := strings.TrimSpace(provider.Path)
	if path == "" {
		return nil, fmt.Errorf("data source credential key path is required")
	}
	content, err := os.ReadFile(path)
	if err == nil {
		return decodeStoredKey(content)
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read data source credential key: %w", err)
	}
	key := make([]byte, keyByteLength)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate data source credential key: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create data source credential key dir: %w", err)
	}
	encoded := []byte(hex.EncodeToString(key))
	if err := os.WriteFile(path, encoded, keyFileMode()); err != nil {
		return nil, fmt.Errorf("write data source credential key: %w", err)
	}
	return key, nil
}

// StaticKeyProvider 在单元测试中提供固定密钥。
type StaticKeyProvider []byte

// Key 返回固定测试密钥。
func (provider StaticKeyProvider) Key() ([]byte, error) {
	if len(provider) != keyByteLength {
		return nil, fmt.Errorf("data source credential key must be 32 bytes")
	}
	key := make([]byte, keyByteLength)
	copy(key, provider)
	return key, nil
}

// encryptCredential 使用 AES-GCM 加密凭据明文，返回 base64 密文和 nonce。
func encryptCredential(key []byte, plaintext string) (string, string, error) {
	if len(key) != keyByteLength {
		return "", "", fmt.Errorf("data source credential key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", fmt.Errorf("create data source credential cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", fmt.Errorf("create data source credential gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", "", fmt.Errorf("generate data source credential nonce: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), base64.StdEncoding.EncodeToString(nonce), nil
}

// decryptCredential 解密 SQLite 中保存的数据源凭据密文。
func decryptCredential(key []byte, ciphertext string, nonceText string) (string, error) {
	if len(key) != keyByteLength {
		return "", fmt.Errorf("data source credential key must be 32 bytes")
	}
	cipherBytes, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode data source credential ciphertext: %w", err)
	}
	nonce, err := base64.StdEncoding.DecodeString(nonceText)
	if err != nil {
		return "", fmt.Errorf("decode data source credential nonce: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create data source credential cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create data source credential gcm: %w", err)
	}
	plaintext, err := gcm.Open(nil, nonce, cipherBytes, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt data source credential: %w", err)
	}
	return string(plaintext), nil
}

// decodeStoredKey 解析本地密钥文件内容，兼容 hex 文本和原始 32 字节。
func decodeStoredKey(content []byte) ([]byte, error) {
	trimmed := strings.TrimSpace(string(content))
	if len(trimmed) == keyByteLength*2 {
		key, err := hex.DecodeString(trimmed)
		if err != nil {
			return nil, fmt.Errorf("decode data source credential key: %w", err)
		}
		if len(key) == keyByteLength {
			return key, nil
		}
	}
	if len(content) == keyByteLength {
		key := make([]byte, keyByteLength)
		copy(key, content)
		return key, nil
	}
	return nil, fmt.Errorf("data source credential key must be 32 bytes")
}

// keyFileMode 返回平台可接受的本地密钥文件权限。
func keyFileMode() os.FileMode {
	if runtime.GOOS == "windows" {
		return 0o600
	}
	return 0o600
}
