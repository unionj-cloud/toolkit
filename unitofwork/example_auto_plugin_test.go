package unitofwork

import (
	"context"
	"fmt"
	"log"
	"reflect"

	"github.com/wubin1989/gorm"
	"github.com/wubin1989/sqlite"
)

// ExampleUser 示例用户实体
type ExampleUser struct {
	BaseEntity
	Name  string `gorm:"size:100;not null" json:"name"`
	Email string `gorm:"size:255;uniqueIndex" json:"email"`
	Age   int    `json:"age"`
}

func (u *ExampleUser) GetTableName() string {
	return "users"
}

func (u *ExampleUser) Validate() error {
	if u.Name == "" {
		return fmt.Errorf("用户名不能为空")
	}
	if u.Email == "" {
		return fmt.Errorf("邮箱不能为空")
	}
	return nil
}

// ExamplePost 示例文章实体
type ExamplePost struct {
	BaseEntity
	Title   string       `gorm:"size:255;not null" json:"title"`
	Content string       `gorm:"type:text" json:"content"`
	UserID  uint         `gorm:"not null" json:"user_id"`
	User    *ExampleUser `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (p *ExamplePost) GetTableName() string {
	return "posts"
}

func (p *ExamplePost) Validate() error {
	if p.Title == "" {
		return fmt.Errorf("标题不能为空")
	}
	if p.UserID == 0 {
		return fmt.Errorf("用户ID不能为空")
	}
	return nil
}

//
//// ExampleBasicUsage 基本使用示例
//func ExampleBasicUsage() {
//	// 1. 初始化数据库
//	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
//	if err != nil {
//		log.Fatal("连接数据库失败:", err)
//	}
//
//	// 2. 注册自动化工作单元插件（使用默认配置）
//	plugin := NewAutoUnitOfWorkPlugin(nil, nil)
//	err = db.Use(plugin)
//	if err != nil {
//		log.Fatal("注册插件失败:", err)
//	}
//
//	// 3. 自动迁移
//	err = db.AutoMigrate(&ExampleUser{}, &ExamplePost{})
//	if err != nil {
//		log.Fatal("数据库迁移失败:", err)
//	}
//
//	// 4. 使用事务 - 插件会自动管理工作单元
//	err = db.Transaction(func(tx *gorm.DB) error {
//		// 创建用户 - 自动注册到工作单元
//		user := &ExampleUser{
//			Name:  "张三",
//			Email: "zhangsan@example.com",
//			Age:   25,
//		}
//		if err := tx.Create(user).Error; err != nil {
//			return err
//		}
//
//		// 创建文章 - 自动注册到工作单元
//		post := &ExamplePost{
//			Title:   "我的第一篇文章",
//			Content: "这是文章内容...",
//			UserID:  uint(user.GetID().(int)),
//		}
//		if err := tx.Create(post).Error; err != nil {
//			return err
//		}
//
//		// 更新用户 - 自动注册到工作单元
//		user.Age = 26
//		if err := tx.Save(user).Error; err != nil {
//			return err
//		}
//
//		return nil
//		// 事务提交时，工作单元会自动按依赖顺序执行所有操作
//	})
//
//	if err != nil {
//		log.Fatal("事务执行失败:", err)
//	}
//
//	fmt.Println("基本使用示例完成")
//}

// ExampleAdvancedConfiguration 高级配置示例
func ExampleAdvancedConfiguration() {
	// 1. 初始化数据库
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		log.Fatal("连接数据库失败:", err)
	}

	// 2. 自定义配置
	autoConfig := &AutoUowConfig{
		Enabled:          true,
		AutoDetectEntity: true,
		AutoSnapshot:     true,
		VerboseLog:       true,                    // 启用详细日志
		ExcludedTables:   []string{"system_logs"}, // 排除系统日志表
		EntityDetector: func(obj interface{}) Entity {
			// 自定义实体检测逻辑
			if entity, ok := obj.(Entity); ok {
				return entity
			}
			return nil
		},
	}

	globalConfig := &Config{
		EnableDirtyCheck:     true,
		BatchSize:            500,
		EnableOperationMerge: true,
		MaxEntityCount:       5000,
		EnableDetailLog:      true,
	}

	// 3. 注册插件
	plugin := NewAutoUnitOfWorkPlugin(autoConfig, globalConfig)
	err = db.Use(plugin)
	if err != nil {
		log.Fatal("注册插件失败:", err)
	}

	// 4. 自动迁移
	err = db.AutoMigrate(&ExampleUser{}, &ExamplePost{})
	if err != nil {
		log.Fatal("数据库迁移失败:", err)
	}

	fmt.Println("高级配置示例完成")
}

// ExampleManualControl 手动控制示例
func ExampleManualControl() {
	// 1. 初始化数据库和插件
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		log.Fatal("连接数据库失败:", err)
	}

	plugin := NewAutoUnitOfWorkPlugin(nil, nil)
	err = db.Use(plugin)
	if err != nil {
		log.Fatal("注册插件失败:", err)
	}

	err = db.AutoMigrate(&ExampleUser{}, &ExamplePost{})
	if err != nil {
		log.Fatal("数据库迁移失败:", err)
	}

	// 2. 手动获取当前工作单元
	ctx := context.Background()
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 获取当前工作单元
		uow := GetCurrentUnitOfWork(tx.Statement.Context)
		if uow != nil {
			// 获取统计信息
			stats := uow.GetStats()
			fmt.Printf("工作单元统计: %+v\n", stats)

			// 获取依赖管理器
			depManager := uow.GetDependencyManager()
			// 可以手动设置依赖关系
			_ = depManager
		}

		// 正常的数据库操作
		user := &ExampleUser{Name: "李四", Email: "lisi@example.com", Age: 30}
		return tx.Create(user).Error
	})

	if err != nil {
		log.Fatal("手动控制示例失败:", err)
	}

	fmt.Println("手动控制示例完成")
}

// ExampleWithoutTransaction 非事务环境示例
func ExampleWithoutTransaction() {
	// 1. 初始化数据库和插件
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		log.Fatal("连接数据库失败:", err)
	}

	plugin := NewAutoUnitOfWorkPlugin(nil, nil)
	err = db.Use(plugin)
	if err != nil {
		log.Fatal("注册插件失败:", err)
	}

	err = db.AutoMigrate(&ExampleUser{}, &ExamplePost{})
	if err != nil {
		log.Fatal("数据库迁移失败:", err)
	}

	// 2. 非事务环境下的操作（插件会自动跳过工作单元管理）
	user := &ExampleUser{
		Name:  "王五",
		Email: "wangwu@example.com",
		Age:   35,
	}

	// 直接执行，不会进入工作单元
	err = db.Create(user).Error
	if err != nil {
		log.Fatal("创建用户失败:", err)
	}

	// 查询操作（如果启用了AutoSnapshot，会自动创建快照）
	var loadedUser ExampleUser
	err = db.First(&loadedUser, user.GetID()).Error
	if err != nil {
		log.Fatal("查询用户失败:", err)
	}

	fmt.Printf("非事务环境示例完成，用户: %+v\n", loadedUser)
}

// ExampleErrorHandling 错误处理示例
func ExampleErrorHandling() {
	// 1. 初始化数据库和插件
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		log.Fatal("连接数据库失败:", err)
	}

	config := &AutoUowConfig{
		Enabled:    true,
		VerboseLog: true, // 启用详细日志以观察错误处理
	}

	plugin := NewAutoUnitOfWorkPlugin(config, nil)
	err = db.Use(plugin)
	if err != nil {
		log.Fatal("注册插件失败:", err)
	}

	err = db.AutoMigrate(&ExampleUser{}, &ExamplePost{})
	if err != nil {
		log.Fatal("数据库迁移失败:", err)
	}

	// 2. 故意制造错误的事务
	err = db.Transaction(func(tx *gorm.DB) error {
		// 创建有效用户
		user := &ExampleUser{
			Name:  "赵六",
			Email: "zhaoliu@example.com",
			Age:   40,
		}
		if err := tx.Create(user).Error; err != nil {
			return err
		}

		// 创建无效用户（邮箱为空，验证失败）
		invalidUser := &ExampleUser{
			Name:  "无效用户",
			Email: "", // 空邮箱会导致验证失败
			Age:   25,
		}
		if err := tx.Create(invalidUser).Error; err != nil {
			return err
		}

		return nil
	})

	// 3. 错误会被工作单元捕获并自动回滚
	if err != nil {
		fmt.Printf("事务失败（预期行为）: %v\n", err)
		fmt.Println("工作单元已自动回滚所有变更")
	}

	fmt.Println("错误处理示例完成")
}

// ExampleDependencyManagement 依赖关系管理示例
func ExampleDependencyManagement() {
	// 1. 初始化数据库和插件
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		log.Fatal("连接数据库失败:", err)
	}

	plugin := NewAutoUnitOfWorkPlugin(nil, nil)
	err = db.Use(plugin)
	if err != nil {
		log.Fatal("注册插件失败:", err)
	}

	err = db.AutoMigrate(&ExampleUser{}, &ExamplePost{})
	if err != nil {
		log.Fatal("数据库迁移失败:", err)
	}

	// 2. 在事务中设置依赖关系
	err = db.Transaction(func(tx *gorm.DB) error {
		// 获取工作单元和依赖管理器
		uow := GetCurrentUnitOfWork(tx.Statement.Context)
		if uow != nil {
			depManager := uow.GetDependencyManager()

			// 手动设置依赖关系：Post 依赖于 User
			// 这样可以确保 User 先被创建，然后才创建 Post
			userType := reflect.TypeOf(&ExampleUser{})
			postType := reflect.TypeOf(&ExamplePost{})
			depManager.RegisterDependency(postType, userType)
		}

		// 注意：我们故意先创建 Post，再创建 User
		// 工作单元会自动按依赖关系重新排序
		post := &ExamplePost{
			Title:   "依赖管理测试文章",
			Content: "这篇文章用于测试依赖关系管理",
			UserID:  1, // 假设用户ID为1
		}
		if err := tx.Create(post).Error; err != nil {
			return err
		}

		user := &ExampleUser{
			Name:  "依赖测试用户",
			Email: "dependency@example.com",
			Age:   28,
		}
		if err := tx.Create(user).Error; err != nil {
			return err
		}

		return nil
		// 提交时，工作单元会按依赖关系自动排序：先创建 User，再创建 Post
	})

	if err != nil {
		log.Fatal("依赖关系管理示例失败:", err)
	}

	fmt.Println("依赖关系管理示例完成")
}

// ExampleBatchOperations 批量操作示例
func ExampleBatchOperations() {
	// 1. 初始化数据库和插件
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		log.Fatal("连接数据库失败:", err)
	}

	// 配置较小的批量大小以演示批量操作
	globalConfig := &Config{
		BatchSize: 10,
	}

	plugin := NewAutoUnitOfWorkPlugin(nil, globalConfig)
	err = db.Use(plugin)
	if err != nil {
		log.Fatal("注册插件失败:", err)
	}

	err = db.AutoMigrate(&ExampleUser{}, &ExamplePost{})
	if err != nil {
		log.Fatal("数据库迁移失败:", err)
	}

	// 2. 批量创建数据
	err = db.Transaction(func(tx *gorm.DB) error {
		// 创建100个用户
		for i := 0; i < 100; i++ {
			user := &ExampleUser{
				Name:  fmt.Sprintf("批量用户%d", i+1),
				Email: fmt.Sprintf("batch%d@example.com", i+1),
				Age:   20 + i%50,
			}
			if err := tx.Create(user).Error; err != nil {
				return err
			}
		}

		// 创建200篇文章
		for i := 0; i < 200; i++ {
			post := &ExamplePost{
				Title:   fmt.Sprintf("批量文章%d", i+1),
				Content: fmt.Sprintf("这是第%d篇批量创建的文章", i+1),
				UserID:  uint(1 + i%100), // 分配给前100个用户
			}
			if err := tx.Create(post).Error; err != nil {
				return err
			}
		}

		return nil
		// 工作单元会自动将这些操作分批执行
	})

	if err != nil {
		log.Fatal("批量操作示例失败:", err)
	}

	fmt.Println("批量操作示例完成")
}
