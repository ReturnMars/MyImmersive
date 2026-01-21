package cache

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"

	"github.com/dgraph-io/badger/v4"
)

// Cache 缓存接口
type Cache interface {
	Get(key string) (string, bool)
	Set(key string, value string) error
	Close() error
}

// BadgerCache BadgerDB 实现的缓存
type BadgerCache struct {
	db *badger.DB
}

// NewBadgerCache 创建 BadgerDB 缓存实例
func NewBadgerCache(dataDir string) (*BadgerCache, error) {
	opts := badger.DefaultOptions(dataDir)
	opts.Logger = nil // 禁用 Badger 内部日志

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open BadgerDB: %w", err)
	}

	fmt.Printf("[Cache] Initialized at %s\n", dataDir)
	return &BadgerCache{db: db}, nil
}

// Get 从缓存获取值
func (c *BadgerCache) Get(key string) (string, bool) {
	var value string
	err := c.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			value = string(val)
			return nil
		})
	})

	if err != nil {
		return "", false
	}
	return value, true
}

// Set 写入缓存
func (c *BadgerCache) Set(key string, value string) error {
	return c.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), []byte(value))
	})
}

// Close 关闭数据库连接
func (c *BadgerCache) Close() error {
	fmt.Println("[Cache] Closing...")
	return c.db.Close()
}

// HashKey 生成缓存 Key (MD5)
func HashKey(text string) string {
	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])
}
