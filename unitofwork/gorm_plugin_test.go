package unitofwork

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wubin1989/gorm"
	"github.com/wubin1989/sqlite"
)

// TestUser 测试用户实体
type TestUser struct {
	BaseEntity
	Name  string `gorm:"size:100;not null" json:"name"`
	Email string `gorm:"size:255;uniqueIndex" json:"email"`
	Age   int    `json:"age"`
}

func (u *TestUser) GetTableName() string {
	return "test_users"
}

func (u *TestUser) Validate() error {
	if u.Name == "" {
		return fmt.Errorf("用户名不能为空")
	}
	if u.Email == "" {
		return fmt.Errorf("邮箱不能为空")
	}
	return nil
}

// TestPost 测试文章实体
type TestPost struct {
	BaseEntity
	Title   string    `gorm:"size:255;not null" json:"title"`
	Content string    `gorm:"type:text" json:"content"`
	UserID  uint      `gorm:"not null" json:"user_id"`
	User    *TestUser `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (p *TestPost) GetTableName() string {
	return "test_posts"
}

func (p *TestPost) Validate() error {
	if p.Title == "" {
		return fmt.Errorf("标题不能为空")
	}
	if p.UserID == 0 {
		return fmt.Errorf("用户ID不能为空")
	}
	return nil
}

// setupPluginTestDB 设置测试数据库
func setupPluginTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// 自动迁移
	err = db.AutoMigrate(&TestUser{}, &TestPost{})
	require.NoError(t, err)

	return db
}

//
//func TestAutoUnitOfWorkPlugin_BasicUsage(t *testing.T) {
//	db := setupPluginTestDB(t)
//
//	// 注册插件
//	plugin := NewAutoUnitOfWorkPlugin(nil, nil)
//	err := db.Use(plugin)
//	require.NoError(t, err)
//
//	// 测试事务中的自动工作单元管理
//	err = db.Transaction(func(tx *gorm.DB) error {
//		// 创建用户
//		user := &TestUser{
//			Name:  "测试用户",
//			Email: "test@example.com",
//			Age:   25,
//		}
//		if err := tx.Create(user).Error; err != nil {
//			return err
//		}
//
//		// 创建文章
//		post := &TestPost{
//			Title:   "测试文章",
//			Content: "这是一篇测试文章",
//			UserID:  uint(user.GetID().(int)),
//		}
//		if err := tx.Create(post).Error; err != nil {
//			return err
//		}
//
//		// 更新用户
//		user.Age = 26
//		if err := tx.Save(user).Error; err != nil {
//			return err
//		}
//
//		return nil
//	})
//
//	assert.NoError(t, err)
//
//	// 验证数据是否正确保存
//	var userCount int64
//	db.Model(&TestUser{}).Count(&userCount)
//	assert.Equal(t, int64(1), userCount)
//
//	var postCount int64
//	db.Model(&TestPost{}).Count(&postCount)
//	assert.Equal(t, int64(1), postCount)
//}

func TestAutoUnitOfWorkPlugin_ManualAccess(t *testing.T) {
	db := setupPluginTestDB(t)

	plugin := NewAutoUnitOfWorkPlugin(nil, nil)
	err := db.Use(plugin)
	require.NoError(t, err)

	// 测试手动访问工作单元
	err = db.Transaction(func(tx *gorm.DB) error {
		// 获取当前工作单元
		uow := GetCurrentUnitOfWork(tx.Statement.Context)
		assert.NotNil(t, uow)

		if uow != nil {
			// 获取统计信息
			stats := uow.GetStats()
			assert.NotNil(t, stats)

			// 获取依赖管理器
			depManager := uow.GetDependencyManager()
			assert.NotNil(t, depManager)

			// 手动设置依赖关系
			depManager.RegisterDependency(
				reflect.TypeOf(&TestPost{}),
				reflect.TypeOf(&TestUser{}),
			)
		}

		// 创建实体
		user := &TestUser{Name: "手动测试用户", Email: "manual@example.com", Age: 28}
		return tx.Create(user).Error
	})

	assert.NoError(t, err)
}

func TestAutoUnitOfWorkPlugin_ErrorHandling(t *testing.T) {
	db := setupPluginTestDB(t)

	config := &AutoUowConfig{
		Enabled:    true,
		VerboseLog: false, // 关闭详细日志以避免测试输出混乱
	}

	plugin := NewAutoUnitOfWorkPlugin(config, nil)
	err := db.Use(plugin)
	require.NoError(t, err)

	// 测试验证失败的情况
	err = db.Transaction(func(tx *gorm.DB) error {
		// 创建有效用户
		user := &TestUser{
			Name:  "有效用户",
			Email: "valid@example.com",
			Age:   25,
		}
		if err := tx.Create(user).Error; err != nil {
			return err
		}

		// 创建无效用户（邮箱为空，验证失败）
		invalidUser := &TestUser{
			Name:  "无效用户",
			Email: "", // 空邮箱会导致验证失败
			Age:   30,
		}
		if err := tx.Create(invalidUser).Error; err != nil {
			return err
		}

		return nil
	})

	// 应该失败，因为验证不通过
	assert.Error(t, err)

	// 验证数据没有被保存（回滚成功）
	var userCount int64
	db.Model(&TestUser{}).Count(&userCount)
	assert.Equal(t, int64(0), userCount)
}

func TestAutoUnitOfWorkPlugin_WithoutTransaction(t *testing.T) {
	db := setupPluginTestDB(t)

	plugin := NewAutoUnitOfWorkPlugin(nil, nil)
	err := db.Use(plugin)
	require.NoError(t, err)

	// 非事务环境下的操作
	user := &TestUser{
		Name:  "非事务用户",
		Email: "nontx@example.com",
		Age:   35,
	}

	// 直接执行，不会进入工作单元
	err = db.Create(user).Error
	assert.NoError(t, err)

	// 验证数据被保存
	var savedUser TestUser
	err = db.First(&savedUser, user.GetID()).Error
	assert.NoError(t, err)
	assert.Equal(t, "非事务用户", savedUser.Name)
}

func TestWithManualUnitOfWork(t *testing.T) {
	// 测试手动设置工作单元到上下文
	db := setupPluginTestDB(t)

	// 创建一个工作单元
	uow := NewUnitOfWork(db)

	// 手动绑定到上下文
	ctx := context.Background()
	ctx = WithManualUnitOfWork(ctx, uow)

	// 验证可以获取到工作单元
	retrievedUow := GetCurrentUnitOfWork(ctx)
	assert.Equal(t, uow, retrievedUow)
}

// 性能测试
func BenchmarkAutoUnitOfWorkPlugin(b *testing.B) {
	db := setupPluginTestDB(&testing.T{})

	plugin := NewAutoUnitOfWorkPlugin(nil, nil)
	err := db.Use(plugin)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err = db.Transaction(func(tx *gorm.DB) error {
			user := &TestUser{
				Name:  fmt.Sprintf("性能测试用户%d", i),
				Email: fmt.Sprintf("perf%d@example.com", i),
				Age:   25,
			}
			return tx.Create(user).Error
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// 并发测试
func TestAutoUnitOfWorkPlugin_Concurrency(t *testing.T) {
	db := setupPluginTestDB(t)

	plugin := NewAutoUnitOfWorkPlugin(nil, nil)
	err := db.Use(plugin)
	require.NoError(t, err)

	// 并发执行多个事务
	const goroutineCount = 10
	errChan := make(chan error, goroutineCount)

	for i := 0; i < goroutineCount; i++ {
		go func(id int) {
			err := db.Transaction(func(tx *gorm.DB) error {
				user := &TestUser{
					Name:  fmt.Sprintf("并发用户%d", id),
					Email: fmt.Sprintf("concurrent%d@example.com", id),
					Age:   25 + id,
				}
				return tx.Create(user).Error
			})
			errChan <- err
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < goroutineCount; i++ {
		err := <-errChan
		assert.NoError(t, err)
	}

	// 验证所有用户都被创建
	var userCount int64
	db.Model(&TestUser{}).Count(&userCount)
	assert.Equal(t, int64(goroutineCount), userCount)
}
