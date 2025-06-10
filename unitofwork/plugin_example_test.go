package unitofwork

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/wubin1989/gorm"
	"github.com/wubin1989/gorm/logger"
	"github.com/wubin1989/sqlite"
)

// ExamplePlugin_BasicUsage 基本使用示例
func ExamplePlugin_BasicUsage() {
	// 设置数据库
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatal("连接数据库失败:", err)
	}

	// 自动迁移
	db.AutoMigrate(&User{}, &Post{}, &Tag{})

	// 注册工作单元插件
	plugin := NewPlugin()
	if err := db.Use(plugin); err != nil {
		log.Fatal("注册插件失败:", err)
	}

	// 使用工作单元
	err = WithUnitOfWork(context.Background(), db, func(db *gorm.DB, uow *UnitOfWork) error {
		// 创建用户
		user := &User{
			Name:  "示例用户",
			Email: "example@test.com",
			Age:   30,
		}
		if err := db.Create(user).Error; err != nil {
			return err
		}

		// 创建文章
		post := &Post{
			Title:   "我的第一篇文章",
			Content: "这是文章内容...",
			UserID:  1, // 假设用户ID为1
		}
		return db.Create(post).Error
	})

	if err != nil {
		fmt.Printf("执行失败: %v\n", err)
		return
	}

	fmt.Println("工作单元执行成功")
	// Output: 工作单元执行成功
}

// ExamplePlugin_CustomConfiguration 自定义配置示例
func ExamplePlugin_CustomConfiguration() {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatal(err)
	}

	db.AutoMigrate(&User{}, &Post{}, &Tag{})

	// 自定义插件配置
	plugin := NewPlugin(
		WithPluginAutoManage(true),
		WithPluginContextKey("my_uow"),
		WithPluginUnitOfWorkConfig(&Config{
			EnableDirtyCheck:     true,
			BatchSize:            500,
			EnableOperationMerge: true,
			EnableDetailLog:      false,
		}),
	)

	if err := db.Use(plugin); err != nil {
		log.Fatal(err)
	}

	// 使用自定义配置的工作单元
	err = WithUnitOfWork(context.Background(), db, func(db *gorm.DB, uow *UnitOfWork) error {
		user := &User{
			Name:  "自定义配置用户",
			Email: "custom@test.com",
			Age:   25,
		}
		return db.Create(user).Error
	})

	if err != nil {
		fmt.Printf("执行失败: %v\n", err)
		return
	}

	fmt.Println("自定义配置执行成功")
	// Output: 自定义配置执行成功
}

// ExamplePlugin_DependencyManagement 依赖关系管理示例
func ExamplePlugin_DependencyManagement() {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatal(err)
	}

	db.AutoMigrate(&User{}, &Post{}, &Tag{})

	// 配置实体依赖关系
	dependencyMapping := map[reflect.Type][]reflect.Type{
		reflect.TypeOf(&Post{}): {reflect.TypeOf(&User{})}, // Post 依赖于 User
		reflect.TypeOf(&Tag{}):  {reflect.TypeOf(&User{})}, // Tag 依赖于 User
	}

	plugin := NewPlugin(
		WithPluginDependencyMapping(dependencyMapping),
	)

	if err := db.Use(plugin); err != nil {
		log.Fatal(err)
	}

	err = WithUnitOfWork(context.Background(), db, func(db *gorm.DB, uow *UnitOfWork) error {
		// 创建顺序：Post -> Tag -> User
		// 实际执行顺序会被自动调整为：User -> Post -> Tag

		post := &Post{
			Title:   "依赖管理文章",
			Content: "测试依赖关系...",
			UserID:  1,
		}
		if err := db.Create(post).Error; err != nil {
			return err
		}

		tag := &Tag{
			Name:  "技术",
			Color: "#FF5722",
		}
		if err := db.Create(tag).Error; err != nil {
			return err
		}

		user := &User{
			Name:  "依赖管理用户",
			Email: "dependency@test.com",
			Age:   28,
		}
		user.ID = 1
		return db.Create(user).Error
	})

	if err != nil {
		fmt.Printf("执行失败: %v\n", err)
		return
	}

	fmt.Println("依赖关系管理成功")
	// Output: 依赖关系管理成功
}

// ExamplePlugin_ManualUnitOfWork 手动工作单元管理示例
func ExamplePlugin_ManualUnitOfWork() {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatal(err)
	}

	db.AutoMigrate(&User{}, &Post{}, &Tag{})

	// 禁用自动管理，手动控制工作单元
	plugin := NewPlugin(WithPluginAutoManage(false))
	if err := db.Use(plugin); err != nil {
		log.Fatal(err)
	}

	// 手动创建和管理工作单元
	uow := NewUnitOfWork(db)

	// 手动注册实体
	user := &User{
		Name:  "手动管理用户",
		Email: "manual@test.com",
		Age:   35,
	}

	if err := uow.Create(user); err != nil {
		log.Fatal(err)
	}

	// 手动提交
	if err := uow.Commit(); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("手动管理成功，用户ID: %v\n", user.GetID())
	// Output: 手动管理成功，用户ID: 1
}

// ExamplePlugin_ErrorHandling 错误处理示例
func ExamplePlugin_ErrorHandling() {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatal(err)
	}

	db.AutoMigrate(&User{}, &Post{}, &Tag{})

	plugin := NewPlugin()
	if err := db.Use(plugin); err != nil {
		log.Fatal(err)
	}

	// 模拟业务逻辑失败
	err = WithUnitOfWork(context.Background(), db, func(db *gorm.DB, uow *UnitOfWork) error {
		// 创建有效用户
		user := &User{
			Name:  "有效用户",
			Email: "valid@test.com",
			Age:   25,
		}
		if err := db.Create(user).Error; err != nil {
			return err
		}

		// 创建无效用户（邮箱为空）
		invalidUser := &User{
			Name:  "",
			Email: "invalid@test.com",
			Age:   30,
		}
		return db.Create(invalidUser).Error
	})

	if err != nil {
		fmt.Printf("预期的错误发生: %v\n", err)

		// 验证回滚生效 - 没有用户被创建
		var count int64
		db.Model(&User{}).Count(&count)
		fmt.Printf("用户数量: %d\n", count)
	}

	// Output: 预期的错误发生: business logic failed: validation failed for entity *unitofwork.User: 用户名不能为空
	// 用户数量: 0
}

// ExamplePlugin_ConcurrentOperations 并发操作示例
func ExamplePlugin_ConcurrentOperations() {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatal(err)
	}

	db.AutoMigrate(&User{}, &Post{}, &Tag{})

	plugin := NewPlugin()
	if err := db.Use(plugin); err != nil {
		log.Fatal(err)
	}

	// 先创建一个测试用户确保数据库连接正常
	testUser := &User{Name: "测试", Email: "test@test.com", Age: 25}
	db.Create(testUser)

	// 使用带缓冲的channel收集结果
	results := make(chan string, 3)
	var wg sync.WaitGroup

	for i := 1; i <= 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// 使用序列化的方式避免并发问题
			time.Sleep(time.Duration(id-1) * 10 * time.Millisecond)

			err := WithUnitOfWork(context.Background(), db, func(db *gorm.DB, uow *UnitOfWork) error {
				user := &User{
					Name:  fmt.Sprintf("并发用户%d", id),
					Email: fmt.Sprintf("concurrent%d@test.com", id),
					Age:   20 + id,
				}
				return db.Create(user).Error
			})

			if err != nil {
				results <- fmt.Sprintf("并发操作%d失败: %v", id, err)
			} else {
				results <- fmt.Sprintf("并发操作%d成功", id)
			}
		}(i)
	}

	wg.Wait()
	close(results)

	// 收集并排序结果
	var resultSlice []string
	for result := range results {
		resultSlice = append(resultSlice, result)
	}

	// 按操作ID排序输出
	for i := 1; i <= 3; i++ {
		for _, result := range resultSlice {
			if strings.Contains(result, fmt.Sprintf("并发操作%d", i)) {
				fmt.Println(result)
				break
			}
		}
	}

	fmt.Println("所有并发操作完成")
	// Output: 并发操作1成功
	// 并发操作2成功
	// 并发操作3成功
	// 所有并发操作完成
}

// ExamplePlugin_ComplexTransaction 复杂事务示例
func ExamplePlugin_ComplexTransaction() {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatal(err)
	}

	db.AutoMigrate(&User{}, &Post{}, &Tag{})

	plugin := NewPlugin()
	if err := db.Use(plugin); err != nil {
		log.Fatal(err)
	}

	err = WithUnitOfWork(context.Background(), db, func(db *gorm.DB, uow *UnitOfWork) error {
		// 创建作者
		author := &User{
			BaseEntity: BaseEntity{ID: 1}, // 预先设置ID
			Name:       "技术博主",
			Email:      "author@test.com",
			Age:        30,
		}
		if err := db.Create(author).Error; err != nil {
			return err
		}

		// 创建多篇文章
		for i := 1; i <= 3; i++ {
			post := &Post{
				BaseEntity: BaseEntity{ID: uint(i)}, // 预先设置ID
				Title:      fmt.Sprintf("技术文章%d", i),
				Content:    fmt.Sprintf("这是第%d篇技术文章的内容...", i),
				UserID:     1, // 假设作者ID为1
			}
			if err := db.Create(post).Error; err != nil {
				return err
			}
		}

		// 创建标签
		tags := []string{"Go语言", "数据库", "架构设计"}
		for i, tagName := range tags {
			tag := &Tag{
				BaseEntity: BaseEntity{ID: uint(i + 1)}, // 预先设置ID
				Name:       tagName,
				Color:      "#2196F3",
			}
			if err := db.Create(tag).Error; err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		fmt.Printf("复杂事务失败: %v\n", err)
		return
	}

	// 验证结果
	var userCount, postCount, tagCount int64
	db.Model(&User{}).Count(&userCount)
	db.Model(&Post{}).Count(&postCount)
	db.Model(&Tag{}).Count(&tagCount)

	fmt.Printf("复杂事务成功: 用户%d个，文章%d篇，标签%d个\n", userCount, postCount, tagCount)
	// Output: 复杂事务成功: 用户1个，文章3篇，标签3个
}

// ExamplePlugin_QueryAndUpdate 查询后更新示例，验证快照创建时机
func ExamplePlugin_QueryAndUpdate() {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatal(err)
	}

	db.AutoMigrate(&User{}, &Post{}, &Tag{})

	// 启用详细日志以看到快照创建
	plugin := NewPlugin(WithPluginUnitOfWorkConfig(&Config{
		EnableDetailLog: true,
	}))
	if err := db.Use(plugin); err != nil {
		log.Fatal(err)
	}

	// 先创建一个用户
	initialUser := &User{
		BaseEntity: BaseEntity{ID: 1},
		Name:       "初始用户",
		Email:      "initial@test.com", 
		Age:        25,
	}
	db.Create(initialUser)

	// 在工作单元中查询并更新用户
	err = WithUnitOfWork(context.Background(), db, func(db *gorm.DB, uow *UnitOfWork) error {
		// 查询用户 - 此时应该创建快照
		var user User
		if err := db.First(&user, 1).Error; err != nil {
			return err
		}

		// 修改用户信息
		user.Name = "更新后用户"
		user.Age = 30

		// 保存修改 - 此时应该检测到变更并注册到工作单元
		return db.Save(&user).Error
	})

	if err != nil {
		fmt.Printf("查询更新失败: %v\n", err)
		return
	}

	// 验证用户已被更新
	var updatedUser User
	db.First(&updatedUser, 1)
	fmt.Printf("用户更新成功: %s, 年龄: %d\n", updatedUser.Name, updatedUser.Age)
	// Output: 用户更新成功: 更新后用户, 年龄: 30
} 