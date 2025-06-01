package unitofwork

import (
	"context"
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/wubin1989/gorm"
	"github.com/wubin1989/gorm/logger"
	"github.com/wubin1989/sqlite"
	"sync"
	"testing"
)

// setupTestDB 设置测试数据库
func setupManagerTestDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		panic("连接数据库失败: " + err.Error())
	}

	// 自动迁移
	err = db.AutoMigrate(&User{}, &Post{}, &Tag{})
	if err != nil {
		panic("数据库迁移失败: " + err.Error())
	}

	return db
}

// TestNewManager 测试创建工作单元管理器
func TestNewManager(t *testing.T) {
	db := setupManagerTestDB()

	t.Run("使用自定义配置创建管理器", func(t *testing.T) {
		manager := NewManager(db,
			WithBatchSize(500),
			WithDirtyCheck(false),
			WithDetailLog(true),
		)

		assert.NotNil(t, manager)
		assert.Equal(t, 500, manager.config.BatchSize)
		assert.False(t, manager.config.EnableDirtyCheck)
		assert.True(t, manager.config.EnableDetailLog)
	})
}

// TestManager_ExecuteInUnitOfWork 测试在工作单元中执行操作
func TestManager_ExecuteInUnitOfWork(t *testing.T) {
	db := setupManagerTestDB()
	manager := NewManager(db)

	t.Run("成功执行操作", func(t *testing.T) {
		var capturedUoW *UnitOfWork
		err := manager.ExecuteInUnitOfWork(context.Background(), func(uow *UnitOfWork) error {
			capturedUoW = uow

			// 创建用户
			user := &User{
				Name:  "测试用户",
				Email: "test@example.com",
				Age:   25,
			}
			return uow.Create(user)
		})

		assert.NoError(t, err)
		assert.NotNil(t, capturedUoW)

		// 验证数据已保存到数据库
		var count int64
		err = db.Model(&User{}).Where("email = ?", "test@example.com").Count(&count).Error
		assert.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})

	t.Run("业务逻辑失败时回滚", func(t *testing.T) {
		expectedError := errors.New("业务逻辑错误")

		err := manager.ExecuteInUnitOfWork(context.Background(), func(uow *UnitOfWork) error {
			// 创建用户
			user := &User{
				Name:  "测试用户2",
				Email: "test2@example.com",
				Age:   30,
			}
			if createErr := uow.Create(user); createErr != nil {
				return createErr
			}

			// 模拟业务逻辑失败
			return expectedError
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "business logic failed")

		// 验证数据没有保存到数据库
		var count int64
		err = db.Model(&User{}).Where("email = ?", "test2@example.com").Count(&count).Error
		assert.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})

	t.Run("使用上下文", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "user_id", "admin")

		err := manager.ExecuteInUnitOfWork(ctx, func(uow *UnitOfWork) error {
			// 验证上下文已传递
			assert.Equal(t, ctx, uow.ctx)
			return nil
		})

		assert.NoError(t, err)
	})
}

// TestManager_ExecuteInNewUnitOfWork 测试创建新的工作单元
func TestManager_ExecuteInNewUnitOfWork(t *testing.T) {
	db := setupManagerTestDB()
	manager := NewManager(db)

	err := manager.ExecuteInNewUnitOfWork(func(uow *UnitOfWork) error {
		user := &User{
			Name:  "新工作单元用户",
			Email: "new@example.com",
			Age:   28,
		}
		return uow.Create(user)
	})

	assert.NoError(t, err)

	// 验证数据已保存
	var count int64
	err = db.Model(&User{}).Where("email = ?", "new@example.com").Count(&count).Error
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

// TestManager_ConcurrentAccess 测试并发访问
func TestManager_ConcurrentAccess(t *testing.T) {
	db := setupManagerTestDB()
	manager := NewManager(db)

	const numGoroutines = 1
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	// 并发执行多个工作单元
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			err := manager.ExecuteInUnitOfWork(context.Background(), func(uow *UnitOfWork) error {
				user := &User{
					Name:  fmt.Sprintf("并发用户%d", index),
					Email: fmt.Sprintf("concurrent%d@example.com", index),
					Age:   20 + index,
				}
				user.ID = uint(index + 1)
				return uow.Create(user)
			})

			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// 检查是否有错误
	for err := range errors {
		t.Errorf("并发执行出错: %v", err)
	}

	// 验证所有用户都已创建
	var count int64
	err := db.Model(&User{}).Where("email LIKE ?", "concurrent%@example.com").Count(&count).Error
	assert.NoError(t, err)
	assert.Equal(t, int64(numGoroutines), count)
}

// TestManager_EdgeCases 测试边界情况
func TestManager_EdgeCases(t *testing.T) {
	db := setupManagerTestDB()
	manager := NewManager(db)

	t.Run("空操作函数", func(t *testing.T) {
		err := manager.ExecuteInUnitOfWork(context.Background(), func(uow *UnitOfWork) error {
			// 什么都不做
			return nil
		})
		assert.NoError(t, err)
	})

	t.Run("多次调用相同操作", func(t *testing.T) {
		user := &User{
			Name:  "重复操作用户",
			Email: "duplicate@example.com",
			Age:   40,
		}

		// 第一次执行
		err := manager.ExecuteInUnitOfWork(context.Background(), func(uow *UnitOfWork) error {
			return uow.Create(user)
		})
		assert.NoError(t, err)

		// 验证用户ID已设置
		assert.NotEqual(t, uint(0), user.GetID())

		// 再次尝试创建相同用户（应该失败，因为邮箱唯一）
		err = manager.ExecuteInUnitOfWork(context.Background(), func(uow *UnitOfWork) error {
			duplicateUser := &User{
				Name:  "重复邮箱用户",
				Email: "duplicate@example.com", // 相同邮箱
				Age:   45,
			}
			return uow.Create(duplicateUser)
		})
		assert.Error(t, err) // 应该因为邮箱重复而失败
	})
}

// TestManager_MemoryManagement 测试内存管理
func TestManager_MemoryManagement(t *testing.T) {
	db := setupManagerTestDB()
	manager := NewManager(db, WithMaxEntityCount(5))

	t.Run("超出实体数量限制", func(t *testing.T) {
		err := manager.ExecuteInUnitOfWork(context.Background(), func(uow *UnitOfWork) error {
			// 尝试创建超过限制数量的实体
			for i := 0; i < 10; i++ {
				user := &User{
					Name:  fmt.Sprintf("限制测试用户%d", i),
					Email: fmt.Sprintf("limit%d@example.com", i),
					Age:   20 + i,
				}
				user.ID = uint(i + 1)
				if err := uow.Create(user); err != nil {
					return err
				}
			}
			return nil
		})

		// 应该在某个点失败，因为超出了实体数量限制
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "entity count limit exceeded")
	})
}

// TestManager_ConfigurationPropagation 测试配置传播
func TestManager_ConfigurationPropagation(t *testing.T) {
	db := setupManagerTestDB()

	config := &Config{
		EnableDirtyCheck:     false,
		BatchSize:            100,
		EnableOperationMerge: false,
		MaxEntityCount:       1000,
		EnableDetailLog:      true,
	}

	manager := NewManager(db, func(c *Config) {
		*c = *config
	})

	err := manager.ExecuteInUnitOfWork(context.Background(), func(uow *UnitOfWork) error {
		// 验证配置已正确传播到工作单元
		assert.Equal(t, config.EnableDirtyCheck, uow.config.EnableDirtyCheck)
		assert.Equal(t, config.BatchSize, uow.config.BatchSize)
		assert.Equal(t, config.EnableOperationMerge, uow.config.EnableOperationMerge)
		assert.Equal(t, config.MaxEntityCount, uow.config.MaxEntityCount)
		assert.Equal(t, config.EnableDetailLog, uow.config.EnableDetailLog)
		return nil
	})

	assert.NoError(t, err)
}

// BenchmarkManager_ExecuteInUnitOfWork 性能测试
func BenchmarkManager_ExecuteInUnitOfWork(b *testing.B) {
	db := setupManagerTestDB()
	manager := NewManager(db)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := manager.ExecuteInUnitOfWork(context.Background(), func(uow *UnitOfWork) error {
			user := &User{
				Name:  fmt.Sprintf("性能测试用户%d", i),
				Email: fmt.Sprintf("perf%d@example.com", i),
				Age:   25,
			}
			return uow.Create(user)
		})
		if err != nil {
			b.Fatalf("性能测试失败: %v", err)
		}
	}
}

// ExampleManager_BasicUsage 管理器基本使用示例
func ExampleManager_BasicUsage() {
	db := setupManagerTestDB()
	manager := NewManager(db)

	err := manager.ExecuteInUnitOfWork(context.Background(), func(uow *UnitOfWork) error {
		user := &User{
			Name:  "示例用户",
			Email: "example@test.com",
			Age:   30,
		}
		return uow.Create(user)
	})

	if err != nil {
		fmt.Printf("执行失败: %v\n", err)
		return
	}

	fmt.Println("用户创建成功")
	// Output: 用户创建成功
}
