package unitofwork

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wubin1989/gorm"
	"github.com/wubin1989/gorm/logger"
	"github.com/wubin1989/sqlite"
)

// setupPluginTestDB 设置插件测试数据库
func setupPluginTestDB() *gorm.DB {
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

// TestPlugin_Initialize 测试插件初始化
func TestPlugin_Initialize(t *testing.T) {
	db := setupPluginTestDB()

	t.Run("默认配置初始化", func(t *testing.T) {
		plugin := NewPlugin()
		
		err := plugin.Initialize(db)
		require.NoError(t, err)
		
		assert.Equal(t, "unitofwork", plugin.Name())
		assert.NotNil(t, plugin.config)
		assert.True(t, plugin.config.AutoManage)
		assert.Equal(t, "unitofwork", plugin.config.ContextKey)
	})

	t.Run("自定义配置初始化", func(t *testing.T) {
		plugin := NewPlugin(
			WithPluginAutoManage(false),
			WithPluginContextKey("custom_uow"),
			WithPluginUnitOfWorkConfig(&Config{
				EnableDirtyCheck: false,
				BatchSize:        500,
			}),
		)
		
		err := plugin.Initialize(db)
		require.NoError(t, err)
		
		assert.False(t, plugin.config.AutoManage)
		assert.Equal(t, "custom_uow", plugin.config.ContextKey)
		assert.False(t, plugin.config.UnitOfWorkConfig.EnableDirtyCheck)
		assert.Equal(t, 500, plugin.config.UnitOfWorkConfig.BatchSize)
	})
}

// TestPlugin_AutoManage 测试自动管理功能
func TestPlugin_AutoManage(t *testing.T) {
	db := setupPluginTestDB()
	
	// 注册插件
	plugin := NewPlugin(WithPluginAutoManage(true))
	err := db.Use(plugin)
	require.NoError(t, err)

	t.Run("自动创建实体", func(t *testing.T) {
		ctx := context.Background()
		
		err := WithUnitOfWork(ctx, db, func(db *gorm.DB, uow *UnitOfWork) error {
			user := &User{
				BaseEntity: BaseEntity{ID: 1}, // 预先设置ID
				Name:       "插件测试用户",
				Email:      "plugin@example.com",
				Age:        25,
			}
			
			// 使用GORM的Create方法，应该自动被插件拦截
			result := db.Create(user)
			return result.Error
		})
		
		require.NoError(t, err)
		
		// 验证用户已创建
		var count int64
		err = db.Model(&User{}).Where("email = ?", "plugin@example.com").Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})

	t.Run("自动更新实体", func(t *testing.T) {
		// 先创建用户 - 使用原始GORM操作，跳过插件
		user := &User{
			BaseEntity: BaseEntity{ID: 2}, // 预先设置ID
			Name:       "原始用户", 
			Email:      "original@example.com", 
			Age:        20,
		}
		// 使用Session跳过插件回调
		err := db.Session(&gorm.Session{SkipHooks: true}).Create(user).Error
		require.NoError(t, err)

		ctx := context.Background()
		
		err = WithUnitOfWork(ctx, db, func(db *gorm.DB, uow *UnitOfWork) error {
			// 加载用户
			var loadedUser User
			if err := db.First(&loadedUser, user.ID).Error; err != nil {
				return err
			}
			
			// 修改用户
			loadedUser.Age = 21
			
			// 使用GORM的Save方法，应该自动被插件拦截
			result := db.Save(&loadedUser)
			return result.Error
		})
		
		require.NoError(t, err)
		
		// 验证用户已更新
		var updatedUser User
		err = db.First(&updatedUser, user.ID).Error
		require.NoError(t, err)
		assert.Equal(t, 21, updatedUser.Age)
	})

	t.Run("自动删除实体", func(t *testing.T) {
		// 先创建用户 - 使用原始GORM操作，跳过插件
		user := &User{
			BaseEntity: BaseEntity{ID: 3}, // 预先设置ID
			Name:       "待删除用户", 
			Email:      "delete@example.com", 
			Age:        30,
		}
		// 使用Session跳过插件回调
		err := db.Session(&gorm.Session{SkipHooks: true}).Create(user).Error
		require.NoError(t, err)

		ctx := context.Background()
		
		err = WithUnitOfWork(ctx, db, func(db *gorm.DB, uow *UnitOfWork) error {
			// 使用GORM的Delete方法，应该自动被插件拦截
			result := db.Delete(user)
			return result.Error
		})
		
		require.NoError(t, err)
		
		// 验证用户已删除（软删除）
		var deletedUser User
		result := db.First(&deletedUser, user.ID)
		assert.Error(t, result.Error) // 应该找不到记录
	})
}

// TestPlugin_DependencyManagement 测试依赖关系管理
func TestPlugin_DependencyManagement(t *testing.T) {
	db := setupPluginTestDB()
	
	// 配置依赖关系映射
	dependencyMapping := map[reflect.Type][]reflect.Type{
		reflect.TypeOf(&Post{}): {reflect.TypeOf(&User{})},
		reflect.TypeOf(&Tag{}):  {reflect.TypeOf(&User{})},
	}
	
	plugin := NewPlugin(
		WithPluginAutoManage(true),
		WithPluginDependencyMapping(dependencyMapping),
	)
	
	err := db.Use(plugin)
	require.NoError(t, err)

	t.Run("按依赖顺序创建实体", func(t *testing.T) {
		ctx := context.Background()
		
		err := WithUnitOfWork(ctx, db, func(db *gorm.DB, uow *UnitOfWork) error {
			// 先创建依赖的实体（Post），再创建被依赖的实体（User）
			post := &Post{
				BaseEntity: BaseEntity{ID: 1}, // 预先设置ID
				Title:      "依赖测试文章",
				Content:    "测试内容",
				UserID:     1,
			}
			
			if err := db.Create(post).Error; err != nil {
				return err
			}
			
			user := &User{
				BaseEntity: BaseEntity{ID: 1}, // 预先设置ID
				Name:       "依赖测试用户",
				Email:      "dependency@example.com",
				Age:        25,
			}
			
			return db.Create(user).Error
		})
		
		require.NoError(t, err)
		
		// 验证两个实体都已创建
		var userCount, postCount int64
		db.Model(&User{}).Where("email = ?", "dependency@example.com").Count(&userCount)
		db.Model(&Post{}).Where("title = ?", "依赖测试文章").Count(&postCount)
		
		assert.Equal(t, int64(1), userCount)
		assert.Equal(t, int64(1), postCount)
	})
}

// TestPlugin_Context 测试上下文管理
func TestPlugin_Context(t *testing.T) {
	db := setupPluginTestDB()
	
	plugin := NewPlugin()
	err := db.Use(plugin)
	require.NoError(t, err)

	t.Run("上下文中的工作单元", func(t *testing.T) {
		ctx := context.Background()
		
		err := WithUnitOfWork(ctx, db, func(db *gorm.DB, uow *UnitOfWork) error {
			// 验证可以从上下文获取工作单元
			ctxUow := GetUnitOfWorkFromContext(db.Statement.Context, "unitofwork")
			assert.NotNil(t, ctxUow)
			assert.Equal(t, uow, ctxUow)
			
			return nil
		})
		
		require.NoError(t, err)
	})

	t.Run("自定义上下文键", func(t *testing.T) {
		_ = NewPlugin(WithPluginContextKey("custom_key"))
		
		ctx := context.Background()
		uow := NewUnitOfWork(db)
		ctx = SetUnitOfWorkToContext(ctx, uow, "custom_key")
		
		retrievedUow := GetUnitOfWorkFromContext(ctx, "custom_key")
		assert.NotNil(t, retrievedUow)
		assert.Equal(t, uow, retrievedUow)
	})
}

// TestPlugin_NoAutoManage 测试禁用自动管理
func TestPlugin_NoAutoManage(t *testing.T) {
	db := setupPluginTestDB()
	
	plugin := NewPlugin(WithPluginAutoManage(false))
	err := db.Use(plugin)
	require.NoError(t, err)

	t.Run("禁用自动管理时不拦截操作", func(t *testing.T) {
		user := &User{
			BaseEntity: BaseEntity{ID: 6}, // 预先设置ID
			Name:       "非自动管理用户",
			Email:      "nonauto@example.com",
			Age:        25,
		}
		
		// 直接使用GORM，不应该被插件拦截
		err := db.Create(user).Error
		require.NoError(t, err)
		
		assert.NotEqual(t, uint(0), user.ID) // 应该已设置ID
		
		// 验证用户已创建
		var count int64
		err = db.Model(&User{}).Where("email = ?", "nonauto@example.com").Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})
}

// BenchmarkPlugin_WithUnitOfWork 性能测试
func BenchmarkPlugin_WithUnitOfWork(b *testing.B) {
	db := setupPluginTestDB()
	
	plugin := NewPlugin()
	err := db.Use(plugin)
	if err != nil {
		b.Fatalf("插件注册失败: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := WithUnitOfWork(context.Background(), db, func(db *gorm.DB, uow *UnitOfWork) error {
			user := &User{
				BaseEntity: BaseEntity{ID: uint(7 + i)}, // 预先设置ID，避免冲突
				Name:       "性能测试用户",
				Email:      "perf@example.com",
				Age:        25,
			}
			
			return db.Create(user).Error
		})
		
		if err != nil {
			b.Fatalf("性能测试失败: %v", err)
		}
	}
} 