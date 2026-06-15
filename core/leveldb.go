package core

import (
	"sync"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
)

var dbPath = "/var/lib/trojan-manager"
var dbMu sync.Mutex

// openDBWithRetry 带有退避重试的数据库打开方法，解决多协程/多进程并发锁冲突问题
func openDBWithRetry() (*leveldb.DB, error) {
	var db *leveldb.DB
	var err error
	for i := 0; i < 10; i++ {
		db, err = leveldb.OpenFile(dbPath, nil)
		if err == nil {
			return db, nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil, err
}

// GetValue 获取leveldb值
func GetValue(key string) (string, error) {
	dbMu.Lock()
	defer dbMu.Unlock()

	db, err := openDBWithRetry()
	if err != nil {
		return "", err
	}
	defer db.Close()
	result, err := db.Get([]byte(key), nil)
	if err != nil {
		return "", err
	}
	return string(result), nil
}

// SetValue 设置leveldb值
func SetValue(key string, value string) error {
	dbMu.Lock()
	defer dbMu.Unlock()

	db, err := openDBWithRetry()
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Put([]byte(key), []byte(value), nil)
}

// DelValue 删除值
func DelValue(key string) error {
	dbMu.Lock()
	defer dbMu.Unlock()

	db, err := openDBWithRetry()
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Delete([]byte(key), nil)
}
