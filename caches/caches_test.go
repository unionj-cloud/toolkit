package caches

import (
	"context"
	"database/sql"
	"sync"
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	_ "github.com/proullon/ramsql/driver"
	"github.com/stretchr/testify/assert"
	"github.com/wubin1989/gorm"
	"github.com/wubin1989/postgres"
)

func setupTestDB(t *testing.T) *gorm.DB {
	// 使用RamSQL创建测试数据库连接，使用测试名称作为数据库名
	sqlDB, err := sql.Open("ramsql", t.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// 创建测试表
	batch := []string{
		`CREATE TABLE users (
			id BIGSERIAL PRIMARY KEY,
			name TEXT,
			age INT
		)`,
		`CREATE TABLE posts (
			id BIGSERIAL PRIMARY KEY,
			title TEXT,
			user_id INT
		)`,
	}

	for _, b := range batch {
		_, err = sqlDB.Exec(b)
		if err != nil {
			t.Fatalf("Failed to create test tables: %v", err)
		}
	}

	// 初始化GORM
	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to initialize GORM: %v", err)
	}

	return db
}

func TestCaches_Initialize(t *testing.T) {
	db := setupTestDB(t)
	defer func() {
		if db, _ := db.DB(); db != nil {
			_ = db.Close()
		}
	}()

	caches := &Caches{
		Conf: &Config{
			Easer:  true,
			Cacher: nil,
		},
	}

	err := caches.Initialize(db)
	assert.NoError(t, err)
}

func TestCaches_Query(t *testing.T) {
	db := setupTestDB(t)
	defer func() {
		if db, _ := db.DB(); db != nil {
			_ = db.Close()
		}
	}()

	caches := &Caches{
		Conf: &Config{
			Easer:  true,
			Cacher: nil,
		},
	}

	err := caches.Initialize(db)
	assert.NoError(t, err)

	// 插入测试数据
	err = db.Exec(`INSERT INTO users (name, age) VALUES ('John', 25)`).Error
	assert.NoError(t, err)

	// 测试查询
	var result struct {
		Name string
		Age  int
	}
	err = db.Raw("SELECT name, age FROM users WHERE name = ?", "John").Scan(&result).Error
	assert.NoError(t, err)
	assert.Equal(t, "John", result.Name)
	assert.Equal(t, 25, result.Age)
}

func TestTablesContext(t *testing.T) {
	// 测试 NewTablesContext
	tables := mapset.NewSet[string]()
	tables.Add("users")
	ctx := context.Background()
	newCtx := NewTablesContext(ctx, tables)

	// 测试 TablesFromContext
	retrievedTables, ok := TablesFromContext(newCtx)
	assert.True(t, ok)
	assert.Equal(t, tables, retrievedTables)
}

func TestCaches_AfterWrite(t *testing.T) {
	db := setupTestDB(t)
	defer func() {
		if db, _ := db.DB(); db != nil {
			_ = db.Close()
		}
	}()

	caches := &Caches{
		Conf: &Config{
			Easer:  true,
			Cacher: nil,
		},
	}

	err := caches.Initialize(db)
	assert.NoError(t, err)

	// 测试插入操作
	err = db.Exec(`INSERT INTO users (name, age) VALUES ('Jane', 30)`).Error
	assert.NoError(t, err)

	// 测试更新操作
	err = db.Exec(`UPDATE users SET age = 31 WHERE name = 'Jane'`).Error
	assert.NoError(t, err)

	// 测试删除操作
	err = db.Exec(`DELETE FROM users WHERE name = 'Jane'`).Error
	assert.NoError(t, err)
}

type MockCacher struct {
	data map[string]*Query
}

func NewMockCacher() *MockCacher {
	return &MockCacher{
		data: make(map[string]*Query),
	}
}

func (m *MockCacher) Get(key string) *Query {
	if query, exists := m.data[key]; exists {
		return query
	}
	return nil
}

func (m *MockCacher) Store(key string, query *Query) error {
	m.data[key] = query
	return nil
}

func (m *MockCacher) Delete(tag string, tags ...string) error {
	delete(m.data, tag)
	for _, t := range tags {
		delete(m.data, t)
	}
	return nil
}

func TestCaches_WithCacher(t *testing.T) {
	db := setupTestDB(t)
	defer func() {
		if db, _ := db.DB(); db != nil {
			_ = db.Close()
		}
	}()

	mockCacher := NewMockCacher()
	caches := &Caches{
		Conf: &Config{
			Easer:  true,
			Cacher: mockCacher,
		},
	}

	err := caches.Initialize(db)
	assert.NoError(t, err)

	// 测试缓存存储
	err = db.Exec(`INSERT INTO users (name, age) VALUES ('Alice', 28)`).Error
	assert.NoError(t, err)

	var result struct {
		Name string
		Age  int
	}

	// 首次查询，应该存入缓存
	err = db.Raw("SELECT name, age FROM users WHERE name = ?", "Alice").Scan(&result).Error
	assert.NoError(t, err)
	assert.Equal(t, "Alice", result.Name)
	assert.Equal(t, 28, result.Age)

	// 再次查询，应该从缓存中获取
	err = db.Raw("SELECT name, age FROM users WHERE name = ?", "Alice").Scan(&result).Error
	assert.NoError(t, err)
	assert.Equal(t, "Alice", result.Name)
	assert.Equal(t, 28, result.Age)
}

func TestBuildIdentifier(t *testing.T) {
	db := setupTestDB(t)
	defer func() {
		if db, _ := db.DB(); db != nil {
			_ = db.Close()
		}
	}()

	// 测试简单查询
	stmt := db.Session(&gorm.Session{})
	stmt.Statement.SQL.Reset() // 重置SQL buffer
	stmt.Statement.SQL.WriteString("SELECT * FROM users WHERE name = ?")
	stmt.Statement.Vars = []interface{}{"John"}
	identifier := buildIdentifier(stmt)
	assert.Equal(t, "SELECT * FROM users WHERE name = ?-[John]", identifier)

	// 测试多参数查询
	stmt = db.Session(&gorm.Session{})
	stmt.Statement.SQL.Reset() // 重置SQL buffer
	stmt.Statement.SQL.WriteString("SELECT * FROM users WHERE age > ? AND age < ?")
	stmt.Statement.Vars = []interface{}{20, 30}
	identifier = buildIdentifier(stmt)
	assert.Equal(t, "SELECT * FROM users WHERE age > ? AND age < ?-[20 30]", identifier)
}

func TestCacheOperations(t *testing.T) {
	db := setupTestDB(t)
	defer func() {
		if db, _ := db.DB(); db != nil {
			_ = db.Close()
		}
	}()

	mockCacher := NewMockCacher()
	caches := &Caches{
		Conf: &Config{
			Easer:  true,
			Cacher: mockCacher,
		},
	}

	// 测试存储缓存
	query := &Query{
		Tags:         []string{"users"},
		Dest:         &struct{ Name string }{Name: "John"},
		RowsAffected: 1,
	}
	err := caches.Conf.Cacher.Store("test_key", query)
	assert.NoError(t, err)

	// 测试检查缓存
	res, ok := caches.checkCache("test_key")
	assert.True(t, ok)
	assert.Equal(t, query, res)

	// 测试删除缓存
	caches.deleteCache(db, "test_key") // 修改为直接使用key
	res, ok = caches.checkCache("test_key")
	assert.False(t, ok)
	assert.Nil(t, res)
}

func TestGetTablesForDifferentDialects(t *testing.T) {
	db := setupTestDB(t)
	defer func() {
		if db, _ := db.DB(); db != nil {
			_ = db.Close()
		}
	}()

	// 设置为Postgres方言
	db.Statement.Dialector = &postgres.Dialector{}

	tests := []struct {
		name     string
		sql      string
		expected []string
	}{
		{
			name:     "PostgreSQL Simple Select",
			sql:      "SELECT * FROM users",
			expected: []string{"users"},
		},
		{
			name:     "PostgreSQL Join",
			sql:      "SELECT u.name, p.title FROM users u JOIN posts p ON u.id = p.user_id",
			expected: []string{"users", "posts"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt := db.Session(&gorm.Session{})
			stmt.Statement.SQL.Reset() // 重置SQL buffer
			stmt.Statement.SQL.WriteString(tt.sql)
			tables := getTables(stmt)
			for _, expected := range tt.expected {
				assert.Contains(t, tables, expected)
			}
		})
	}
}

func TestAfterBeginAndCommit(t *testing.T) {
	db := setupTestDB(t)
	defer func() {
		if db, _ := db.DB(); db != nil {
			_ = db.Close()
		}
	}()

	caches := &Caches{
		Conf: &Config{
			Easer:  true,
			Cacher: NewMockCacher(),
		},
	}

	// 测试事务开始
	tx := db.Begin()
	caches.AfterBegin(tx)
	tables, ok := TablesFromContext(tx.Statement.Context)
	assert.True(t, ok)
	assert.Equal(t, 0, tables.Cardinality())

	// 在事务中执行操作
	err := tx.Exec(`INSERT INTO users (name, age) VALUES ('Tom', 20)`).Error
	assert.NoError(t, err)

	// 测试事务提交
	caches.AfterCommit(tx)
	err = tx.Commit().Error
	assert.NoError(t, err)
}

func TestCaches_ease(t *testing.T) {
	db := setupTestDB(t)
	defer func() {
		if db, _ := db.DB(); db != nil {
			_ = db.Close()
		}
	}()

	caches := &Caches{
		Conf: &Config{
			Easer:  true,
			Cacher: nil,
		},
		queue: &sync.Map{},
	}

	// 准备测试数据
	err := db.Exec(`INSERT INTO users (name, age) VALUES ('John', 25)`).Error
	assert.NoError(t, err)

	// 测试并发查询
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var concurrentResult struct {
				Name string
				Age  int
			}
			stmt := db.Session(&gorm.Session{}).Model(&concurrentResult)
			stmt.Statement.SQL.WriteString("SELECT name, age FROM users WHERE name = ?")
			stmt.Statement.Vars = []interface{}{"John"}
			stmt.Statement.Dest = &concurrentResult

			caches.ease(stmt, "test_query", func(db *gorm.DB) {
				err := db.Statement.ConnPool.QueryRowContext(
					db.Statement.Context,
					db.Statement.SQL.String(),
					db.Statement.Vars...,
				).Scan(&concurrentResult.Name, &concurrentResult.Age)
				assert.NoError(t, err)
			})

			result := stmt.Statement.Dest.(*struct {
				Name string
				Age  int
			})

			assert.Equal(t, "John", result.Name)
			assert.Equal(t, 25, result.Age)

			result.Age = 100
		}()
	}
	wg.Wait()
}

type testStruct struct {
	Name string
	Age  int
}

func TestCaches_ease_WithStructSlice(t *testing.T) {
	db := setupTestDB(t)
	defer func() {
		if db, _ := db.DB(); db != nil {
			_ = db.Close()
		}
	}()

	caches := &Caches{
		Conf: &Config{
			Easer:  true,
			Cacher: nil,
		},
		queue: &sync.Map{},
	}

	// 准备测试数据
	err := db.Exec(`INSERT INTO users (name, age) VALUES ('John', 25)`).Error
	assert.NoError(t, err)

	// 测试并发查询
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var concurrentResult testStruct
			stmt := db.Session(&gorm.Session{}).Model(&concurrentResult)
			stmt.Statement.SQL.WriteString("SELECT name, age FROM users WHERE name = ?")
			stmt.Statement.Vars = []interface{}{"John"}

			concurrentResultSlice := make([]testStruct, 1)
			stmt.Statement.Dest = &concurrentResultSlice

			caches.ease(stmt, "test_query", func(db *gorm.DB) {
				err := db.Statement.ConnPool.QueryRowContext(
					db.Statement.Context,
					db.Statement.SQL.String(),
					db.Statement.Vars...,
				).Scan(&concurrentResult.Name, &concurrentResult.Age)
				assert.NoError(t, err)

				concurrentResultSlice[0] = concurrentResult
			})

			result := *stmt.Statement.Dest.(*[]testStruct)

			assert.Equal(t, "John", result[0].Name)
			assert.Equal(t, 25, result[0].Age)

			result[0].Age = 100
		}()
	}
	wg.Wait()
}

func TestCaches_ease_WithPtrStructSlice(t *testing.T) {
	db := setupTestDB(t)
	defer func() {
		if db, _ := db.DB(); db != nil {
			_ = db.Close()
		}
	}()

	caches := &Caches{
		Conf: &Config{
			Easer:  true,
			Cacher: nil,
		},
		queue: &sync.Map{},
	}

	// 准备测试数据
	err := db.Exec(`INSERT INTO users (name, age) VALUES ('John', 25)`).Error
	assert.NoError(t, err)

	// 测试并发查询
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var concurrentResult testStruct
			stmt := db.Session(&gorm.Session{}).Model(&concurrentResult)
			stmt.Statement.SQL.WriteString("SELECT name, age FROM users WHERE name = ?")
			stmt.Statement.Vars = []interface{}{"John"}

			concurrentResultSlice := make([]*testStruct, 1)
			stmt.Statement.Dest = &concurrentResultSlice

			caches.ease(stmt, "test_query", func(db *gorm.DB) {
				err := db.Statement.ConnPool.QueryRowContext(
					db.Statement.Context,
					db.Statement.SQL.String(),
					db.Statement.Vars...,
				).Scan(&concurrentResult.Name, &concurrentResult.Age)
				assert.NoError(t, err)

				concurrentResultSlice[0] = &concurrentResult
			})

			result := *stmt.Statement.Dest.(*[]*testStruct)

			assert.Equal(t, "John", result[0].Name)
			assert.Equal(t, 25, result[0].Age)

			result[0].Age = 100
		}()
	}
	wg.Wait()
}

func TestCaches_ease_WithMapSlice(t *testing.T) {
	db := setupTestDB(t)
	defer func() {
		if db, _ := db.DB(); db != nil {
			_ = db.Close()
		}
	}()

	caches := &Caches{
		Conf: &Config{
			Easer:  true,
			Cacher: nil,
		},
		queue: &sync.Map{},
	}

	// 准备测试数据
	err := db.Exec(`INSERT INTO users (name, age) VALUES ('John', 25)`).Error
	assert.NoError(t, err)

	// 测试并发查询
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var (
				name string
				age  int
			)
			stmt := db.Session(&gorm.Session{}).Table("users")
			stmt.Statement.SQL.WriteString("SELECT name, age FROM users")
			stmt.Statement.Vars = []interface{}{}

			concurrentResult := make([]map[string]interface{}, 1)
			stmt.Statement.Dest = &concurrentResult

			caches.ease(stmt, "test_query", func(db *gorm.DB) {
				err := db.Statement.ConnPool.QueryRowContext(
					db.Statement.Context,
					db.Statement.SQL.String(),
					db.Statement.Vars...,
				).Scan(&name, &age)
				assert.NoError(t, err)

				row := make(map[string]interface{})
				row["name"] = name
				row["age"] = age

				concurrentResult[0] = row
			})

			// v1 := fmt.Sprintf("%p", stmt.Statement.Dest)
			// v2 := fmt.Sprintf("%p", &concurrentResult)

			// assert.False(t, v1 != v2)

			result := *stmt.Statement.Dest.(*[]map[string]interface{})

			assert.Equal(t, "John", result[0]["name"])
			assert.Equal(t, int64(25), result[0]["age"])

			result[0]["age"] = 100
		}()
	}
	wg.Wait()
}

func TestCaches_ease_WithPtrMapSlice(t *testing.T) {
	db := setupTestDB(t)
	defer func() {
		if db, _ := db.DB(); db != nil {
			_ = db.Close()
		}
	}()

	caches := &Caches{
		Conf: &Config{
			Easer:  true,
			Cacher: nil,
		},
		queue: &sync.Map{},
	}

	// 准备测试数据
	err := db.Exec(`INSERT INTO users (name, age) VALUES ('John', 25)`).Error
	assert.NoError(t, err)

	// 测试并发查询
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var (
				name string
				age  int
			)
			stmt := db.Session(&gorm.Session{}).Table("users")
			stmt.Statement.SQL.WriteString("SELECT name, age FROM users")
			stmt.Statement.Vars = []interface{}{}

			concurrentResult := make([]*map[string]interface{}, 1)
			stmt.Statement.Dest = &concurrentResult

			caches.ease(stmt, "test_query", func(db *gorm.DB) {
				err := db.Statement.ConnPool.QueryRowContext(
					db.Statement.Context,
					db.Statement.SQL.String(),
					db.Statement.Vars...,
				).Scan(&name, &age)
				assert.NoError(t, err)

				row := make(map[string]interface{})
				row["name"] = name
				row["age"] = age

				concurrentResult[0] = &row
			})

			// v1 := fmt.Sprintf("%p", stmt.Statement.Dest)
			// v2 := fmt.Sprintf("%p", &concurrentResult)

			// assert.False(t, v1 != v2)

			result := *stmt.Statement.Dest.(*[]*map[string]interface{})

			assert.Equal(t, "John", (*result[0])["name"])
			assert.Equal(t, int64(25), (*result[0])["age"])

			(*result[0])["age"] = 100
		}()
	}
	wg.Wait()
}

func TestCaches_ease_WithMap(t *testing.T) {
	db := setupTestDB(t)
	defer func() {
		if db, _ := db.DB(); db != nil {
			_ = db.Close()
		}
	}()

	caches := &Caches{
		Conf: &Config{
			Easer:  true,
			Cacher: nil,
		},
		queue: &sync.Map{},
	}

	// 准备测试数据
	err := db.Exec(`INSERT INTO users (name, age) VALUES ('John', 25)`).Error
	assert.NoError(t, err)

	// 测试并发查询
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var (
				name string
				age  int
			)
			stmt := db.Session(&gorm.Session{}).Table("users")
			stmt.Statement.SQL.WriteString("SELECT name, age FROM users")
			stmt.Statement.Vars = []interface{}{}

			concurrentResult := make(map[string]interface{})
			stmt.Statement.Dest = &concurrentResult

			caches.ease(stmt, "test_query", func(db *gorm.DB) {
				err := db.Statement.ConnPool.QueryRowContext(
					db.Statement.Context,
					db.Statement.SQL.String(),
					db.Statement.Vars...,
				).Scan(&name, &age)
				assert.NoError(t, err)

				concurrentResult["name"] = name
				concurrentResult["age"] = age
			})

			// v1 := fmt.Sprintf("%p", stmt.Statement.Dest)
			// v2 := fmt.Sprintf("%p", &concurrentResult)

			// assert.False(t, v1 != v2)

			result := *stmt.Statement.Dest.(*map[string]interface{})

			assert.Equal(t, "John", result["name"])
			assert.Equal(t, int64(25), result["age"])

			result["age"] = 100
		}()
	}
	wg.Wait()
}
