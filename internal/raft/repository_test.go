package raft

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.etcd.io/bbolt"
)

// User 结构体定义
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

var userCounter int64

// 创建临时数据库
func createTempDB(t *testing.T) (*bbolt.DB, func()) {
	dir := t.TempDir()
	dbPath := fmt.Sprintf("%s/test.db", dir)
	db, err := bbolt.Open(dbPath, 0600, nil)
	if err != nil {
		t.Fatalf("无法创建临时数据库: %v", err)
	}
	return db, func() { db.Close() }
}

// 创建 Repository 实例
func createRepo[T any](t *testing.T, db *bbolt.DB, bucket string) *Repository[T] {
	repo := NewRepository[T](db, bucket)
	return repo
}

// 随机生成用户数据
func generateRandomUser() User {
	id := int(atomic.AddInt64(&userCounter, 1))
	return User{
		ID:    id,
		Name:  fmt.Sprintf("user-%d", id),
		Email: fmt.Sprintf("user-%d@example.com", id),
	}
}

// 测试 Set 和 Get 方法
func TestSetAndGet(t *testing.T) {
	db, cleanup := createTempDB(t)
	defer cleanup()

	repo := createRepo[User](t, db, "users")

	user := generateRandomUser()
	err := repo.Set(fmt.Sprintf("%d", user.ID), user)
	assert.NoError(t, err)

	retrievedUser, err := repo.Get(fmt.Sprintf("%d", user.ID))
	assert.NoError(t, err)
	assert.Equal(t, user, retrievedUser)
}

// 测试 Delete 方法
func TestDelete(t *testing.T) {
	db, cleanup := createTempDB(t)
	defer cleanup()

	repo := createRepo[User](t, db, "users")

	user := generateRandomUser()
	err := repo.Set(fmt.Sprintf("%d", user.ID), user)
	assert.NoError(t, err)

	err = repo.Delete(fmt.Sprintf("%d", user.ID))
	assert.NoError(t, err)

	_, err = repo.Get(fmt.Sprintf("%d", user.ID))
	assert.Error(t, err)
}

// 测试 ListKeys 方法
func TestListKeys(t *testing.T) {
	db, cleanup := createTempDB(t)
	defer cleanup()

	repo := createRepo[User](t, db, "users")

	users := []User{generateRandomUser(), generateRandomUser(), generateRandomUser()}
	for _, user := range users {
		err := repo.Set(fmt.Sprintf("%d", user.ID), user)
		assert.NoError(t, err)
	}

	keys, err := repo.ListKeys()
	assert.NoError(t, err)
	assert.Len(t, keys, len(users))
}

// 测试 Exists 方法
func TestExists(t *testing.T) {
	db, cleanup := createTempDB(t)
	defer cleanup()

	repo := createRepo[User](t, db, "users")

	user := generateRandomUser()
	err := repo.Set(fmt.Sprintf("%d", user.ID), user)
	assert.NoError(t, err)

	exists, err := repo.Exists(fmt.Sprintf("%d", user.ID))
	assert.NoError(t, err)
	assert.True(t, exists)

	exists, err = repo.Exists(fmt.Sprintf("%d", user.ID+1))
	assert.NoError(t, err)
	assert.False(t, exists)
}

// 测试 WithTx 方法
func TestWithTx(t *testing.T) {
	db, cleanup := createTempDB(t)
	defer cleanup()

	repo := createRepo[User](t, db, "users")
	err := repo.WithTx(func(tx *bbolt.Tx) error {
		bucketName := []byte("users")
		_, err := tx.CreateBucketIfNotExists(bucketName)
		if err != nil {
			return err
		}
		b := tx.Bucket(bucketName)
		if b == nil {
			return fmt.Errorf("bucket not found")
		}
		return nil
	})
	assert.NoError(t, err)
}

// 测试 Count 方法
func TestCount(t *testing.T) {
	db, cleanup := createTempDB(t)
	defer cleanup()

	repo := createRepo[User](t, db, "users")

	users := []User{generateRandomUser(), generateRandomUser(), generateRandomUser()}
	for _, user := range users {
		err := repo.Set(fmt.Sprintf("%d", user.ID), user)
		assert.NoError(t, err)
	}

	count, err := repo.Count()
	assert.NoError(t, err)
	assert.Equal(t, len(users), count)
}

// 测试 Update 方法
func TestUpdate(t *testing.T) {
	db, cleanup := createTempDB(t)
	defer cleanup()

	repo := createRepo[User](t, db, "users")

	user := generateRandomUser()
	err := repo.Set(fmt.Sprintf("%d", user.ID), user)
	assert.NoError(t, err)

	user.Name = "UpdatedName"
	err = repo.Update(fmt.Sprintf("%d", user.ID), user)
	assert.NoError(t, err)

	retrievedUser, err := repo.Get(fmt.Sprintf("%d", user.ID))
	assert.NoError(t, err)
	assert.Equal(t, user, retrievedUser)
}

// 测试 BatchSet 方法
func TestBatchSet(t *testing.T) {
	db, cleanup := createTempDB(t)
	defer cleanup()

	repo := createRepo[User](t, db, "users")

	users := map[string]User{
		"1": generateRandomUser(),
		"2": generateRandomUser(),
		"3": generateRandomUser(),
	}
	err := repo.BatchSet(users)
	assert.NoError(t, err)

	for key, user := range users {
		retrievedUser, err := repo.Get(key)
		assert.NoError(t, err)
		assert.Equal(t, user, retrievedUser)
	}
}

// 测试 BatchDelete 方法
func TestBatchDelete(t *testing.T) {
	db, cleanup := createTempDB(t)
	defer cleanup()

	repo := createRepo[User](t, db, "users")

	users := []User{generateRandomUser(), generateRandomUser(), generateRandomUser()}
	for _, user := range users {
		err := repo.Set(fmt.Sprintf("%d", user.ID), user)
		assert.NoError(t, err)
	}

	keys := []string{fmt.Sprintf("%d", users[0].ID), fmt.Sprintf("%d", users[1].ID)}
	err := repo.BatchDelete(keys)
	assert.NoError(t, err)

	for _, key := range keys {
		_, err := repo.Get(key)
		assert.Error(t, err)
	}
}

// 测试 ForEach 方法
func TestForEach(t *testing.T) {
	db, cleanup := createTempDB(t)
	defer cleanup()

	repo := createRepo[User](t, db, "users")

	users := []User{generateRandomUser(), generateRandomUser(), generateRandomUser()}
	for _, user := range users {
		err := repo.Set(fmt.Sprintf("%d", user.ID), user)
		assert.NoError(t, err)
	}

	var retrievedUsers []User
	err := repo.ForEach(func(key string, value User) error {
		retrievedUsers = append(retrievedUsers, value)
		return nil
	})
	assert.NoError(t, err)
	assert.Len(t, retrievedUsers, len(users))
}

func TestListByPrefixAndForEachByPrefix(t *testing.T) {
	db, cleanup := createTempDB(t)
	defer cleanup()

	repo := createRepo[User](t, db, "users")

	// 生成几个具有不同前缀的用户键
	users := []User{
		{ID: 1001, Name: "Alice"},
		{ID: 1002, Name: "Bob"},
		{ID: 2001, Name: "Carol"},
	}
	for _, u := range users {
		repo.Set(fmt.Sprintf("%d", u.ID), u)
	}

	keys1, err := repo.ListKeysByPrefix("10")
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"1001", "1002"}, keys1)

	vals1, err := repo.ListByPrefix("10")
	assert.NoError(t, err)
	for _, v := range vals1 {
		assert.True(t, v.ID == 1001 || v.ID == 1002)
	}

	var seen []string
	err = repo.ForEachByPrefix("20", func(key string, u User) error {
		seen = append(seen, key)
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, []string{"2001"}, seen)
}
