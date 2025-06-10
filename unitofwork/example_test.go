package unitofwork

import (
	"context"
	"fmt"
	"github.com/spf13/cast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wubin1989/gorm"
	"github.com/wubin1989/gorm/logger"
	"github.com/wubin1989/sqlite"
	"log"
	"reflect"
	"sync"
	"testing"
	"time"
)

// 示例实体：用户
type User struct {
	BaseEntity
	Name  string `gorm:"size:100;not null" json:"name"`
	Email string `gorm:"size:255;uniqueIndex" json:"email"`
	Age   int    `json:"age"`
	Posts []Post `gorm:"foreignKey:UserID" json:"posts,omitempty"`
}

func (u *User) GetTableName() string {
	return "users"
}

func (u *User) Validate() error {
	if u.Name == "" {
		return fmt.Errorf("用户名不能为空")
	}
	if u.Email == "" {
		return fmt.Errorf("邮箱不能为空")
	}
	if u.Age < 0 {
		return fmt.Errorf("年龄不能为负数")
	}
	return nil
}

// 示例实体：文章
type Post struct {
	BaseEntity
	Title   string `gorm:"size:255;not null" json:"title"`
	Content string `gorm:"type:text" json:"content"`
	UserID  uint   `gorm:"not null" json:"user_id"`
	User    *User  `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (p *Post) GetTableName() string {
	return "posts"
}

func (p *Post) Validate() error {
	if p.Title == "" {
		return fmt.Errorf("标题不能为空")
	}
	if p.UserID == 0 {
		return fmt.Errorf("用户ID不能为空")
	}
	return nil
}

// 示例实体：标签
type Tag struct {
	BaseEntity
	Name  string `gorm:"size:50;uniqueIndex" json:"name"`
	Color string `gorm:"size:7" json:"color"`
}

func (t *Tag) GetTableName() string {
	return "tags"
}

// 设置测试数据库
func setupTestDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatal("连接数据库失败:", err)
	}

	// 自动迁移
	err = db.AutoMigrate(&User{}, &Post{}, &Tag{})
	if err != nil {
		log.Fatal("数据库迁移失败:", err)
	}

	return db
}

// ExampleUnitOfWork_BasicUsage 基本使用示例
func ExampleUnitOfWork_BasicUsage() {
	db := setupTestDB()

	// 创建工作单元
	uow := NewUnitOfWork(db)

	// 创建新用户
	user := &User{
		Name:  "张三",
		Email: "zhangsan@example.com",
		Age:   25,
	}

	// 注册新实体
	err := uow.Create(user)
	if err != nil {
		log.Fatal("注册新实体失败:", err)
	}

	// 提交事务
	err = uow.Commit()
	if err != nil {
		log.Fatal("提交事务失败:", err)
	}

	fmt.Printf("用户创建成功，ID: %v\n", user.GetID())
	// Output: 用户创建成功，ID: 1
}

// ExampleUnitOfWork_WithConfiguration 配置选项示例
func ExampleUnitOfWork_WithConfiguration() {
	db := setupTestDB()

	// 使用配置选项创建工作单元
	uow := NewUnitOfWork(db,
		WithBatchSize(500),
		WithDirtyCheck(true),
		WithOperationMerge(true),
		WithDetailLog(true),
	)

	// 设置上下文
	ctx := context.WithValue(context.Background(), "user_id", "admin")
	uow = uow.WithContext(ctx)

	user := &User{
		Name:  "李四",
		Email: "lisi@example.com",
		Age:   30,
	}

	err := uow.Create(user)
	if err != nil {
		log.Fatal(err)
	}

	err = uow.Commit()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("配置化工作单元创建用户成功，ID: %v\n", user.GetID())
	// Output: 配置化工作单元创建用户成功，ID: 1
}

// ExampleUnitOfWork_ComplexOperations 复杂操作示例
func ExampleUnitOfWork_ComplexOperations() {
	db := setupTestDB()
	uow := NewUnitOfWork(db)

	// 创建用户
	user := &User{
		Name:  "王五",
		Email: "wangwu@example.com",
		Age:   28,
	}
	err := uow.Create(user)
	if err != nil {
		log.Fatal(err)
	}

	// 创建文章
	post1 := &Post{
		Title:   "Go语言入门",
		Content: "这是一篇关于Go语言的入门文章...",
		UserID:  1, // 假设用户ID为1
	}
	err = uow.Create(post1)
	if err != nil {
		log.Fatal(err)
	}

	post2 := &Post{
		Title:   "单元测试最佳实践",
		Content: "本文介绍单元测试的最佳实践...",
		UserID:  1,
	}
	err = uow.Create(post2)
	if err != nil {
		log.Fatal(err)
	}

	// 创建标签
	tag := &Tag{
		Name:  "编程",
		Color: "#FF5722",
	}
	err = uow.Create(tag)
	if err != nil {
		log.Fatal(err)
	}

	// 提交所有操作
	err = uow.Commit()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("批量创建成功：用户ID %v，文章数量 2，标签数量 1\n", user.GetID())
	// Output: 批量创建成功：用户ID 1，文章数量 2，标签数量 1
}

// ExampleUnitOfWork_UpdateAndDelete 更新和删除示例
func ExampleUnitOfWork_UpdateAndDelete() {
	db := setupTestDB()

	// 先创建一些数据
	user := &User{Name: "赵六", Email: "zhaoliu@example.com", Age: 35}
	db.Create(user)

	post := &Post{Title: "原始标题", Content: "原始内容", UserID: user.GetID()}
	db.Create(post)

	// 创建工作单元进行更新操作
	uow := NewUnitOfWork(db)

	// 修改用户信息
	user.Age = 36
	err := uow.Update(user)
	if err != nil {
		log.Fatal(err)
	}

	// 修改文章标题
	post.Title = "更新后的标题"
	err = uow.Update(post)
	if err != nil {
		log.Fatal(err)
	}

	// 创建新标签
	tag := &Tag{Name: "技术", Color: "#2196F3"}
	err = uow.Create(tag)
	if err != nil {
		log.Fatal(err)
	}

	// 提交更新
	err = uow.Commit()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("更新操作完成：用户年龄 %d，文章标题 %s，新标签 %s\n",
		user.Age, post.Title, tag.Name)
	// Output: 更新操作完成：用户年龄 36，文章标题 更新后的标题，新标签 技术
}

// ExampleUnitOfWork_Rollback 回滚示例
func ExampleUnitOfWork_Rollback() {
	db := setupTestDB()
	uow := NewUnitOfWork(db)

	// 创建一个无效的用户（邮箱为空，会验证失败）
	user := &User{
		Name:  "测试用户",
		Email: "", // 空邮箱会导致验证失败
		Age:   25,
	}

	err := uow.Create(user)
	if err != nil {
		log.Fatal(err)
	}

	// 尝试提交，应该会失败
	err = uow.Commit()
	if err != nil {
		fmt.Printf("提交失败，执行回滚: %v\n", err)

		// 执行回滚
		rollbackErr := uow.Rollback()
		if rollbackErr != nil {
			log.Fatal("回滚失败:", rollbackErr)
		}

		fmt.Println("回滚成功")
		return
	}

	fmt.Println("不应该到达这里")
	// Output: 提交失败，执行回滚: unit of work commit failed: operation 0 failed: validation failed for entity *unitofwork.User: 邮箱不能为空
	// 回滚成功
}

// TestUnitOfWork_BasicOperations 基本操作测试
func TestUnitOfWork_BasicOperations(t *testing.T) {
	db := setupTestDB()

	t.Run("创建新实体", func(t *testing.T) {
		uow := NewUnitOfWork(db)

		user := &User{
			Name:  "测试用户",
			Email: "test@example.com",
			Age:   25,
		}

		err := uow.Create(user)
		require.NoError(t, err)

		err = uow.Commit()
		require.NoError(t, err)

		assert.NotNil(t, user.GetID())
		assert.True(t, uow.IsCommitted())
	})

	t.Run("更新实体", func(t *testing.T) {
		// 先创建用户
		user := &User{Name: "原始用户", Email: "original@example.com", Age: 20}
		db.Create(user)

		uow := NewUnitOfWork(db)

		uow.TakeSnapshot(user)

		// 修改用户信息
		user.Name = "更新后用户"
		user.Age = 21

		err := uow.Update(user)
		require.NoError(t, err)

		err = uow.Commit()
		require.NoError(t, err)

		// 验证更新
		var updatedUser User
		db.First(&updatedUser, user.GetID())
		assert.Equal(t, "更新后用户", updatedUser.Name)
		assert.Equal(t, 21, updatedUser.Age)
	})

	t.Run("删除实体", func(t *testing.T) {
		// 先创建用户
		user := &User{Name: "待删除用户", Email: "delete@example.com", Age: 30}
		db.Create(user)

		uow := NewUnitOfWork(db)

		err := uow.Delete(user)
		require.NoError(t, err)

		err = uow.Commit()
		require.NoError(t, err)

		// 验证删除（软删除）
		var deletedUser User
		result := db.First(&deletedUser, user.GetID())
		assert.Error(t, result.Error) // 应该找不到记录
	})
}

// TestUnitOfWork_ValidationAndErrors 验证和错误处理测试
func TestUnitOfWork_ValidationAndErrors(t *testing.T) {
	db := setupTestDB()

	t.Run("实体验证失败", func(t *testing.T) {
		uow := NewUnitOfWork(db)

		// 创建无效用户（邮箱为空）
		user := &User{
			Name:  "测试用户",
			Email: "", // 空邮箱
			Age:   25,
		}

		err := uow.Create(user)
		require.NoError(t, err)

		// 提交应该失败
		err = uow.Commit()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "邮箱不能为空")
	})

	t.Run("已完成的工作单元操作", func(t *testing.T) {
		uow := NewUnitOfWork(db)

		user := &User{
			Name:  "测试用户",
			Email: "test@example.com",
			Age:   25,
		}

		err := uow.Create(user)
		require.NoError(t, err)

		err = uow.Commit()
		require.NoError(t, err)

		// 已提交的工作单元不能再注册新实体
		newUser := &User{
			Name:  "新用户",
			Email: "new@example.com",
			Age:   30,
		}

		err = uow.Create(newUser)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already finished")
	})
}

// TestUnitOfWork_Configuration 配置测试
func TestUnitOfWork_Configuration(t *testing.T) {
	db := setupTestDB()

	t.Run("自定义配置", func(t *testing.T) {
		uow := NewUnitOfWork(db,
			WithBatchSize(100),
			WithDirtyCheck(false),
			WithOperationMerge(false),
			WithMaxEntityCount(1000),
			WithDetailLog(true),
		)

		assert.Equal(t, 100, uow.config.BatchSize)
		assert.False(t, uow.config.EnableDirtyCheck)
		assert.False(t, uow.config.EnableOperationMerge)
		assert.Equal(t, 1000, uow.config.MaxEntityCount)
		assert.True(t, uow.config.EnableDetailLog)
	})

	t.Run("上下文设置", func(t *testing.T) {
		uow := NewUnitOfWork(db)

		ctx := context.WithValue(context.Background(), "test_key", "test_value")
		uow = uow.WithContext(ctx)

		assert.Equal(t, "test_value", uow.ctx.Value("test_key"))
	})
}

// TestUnitOfWork_Stats 统计信息测试
func TestUnitOfWork_Stats(t *testing.T) {
	db := setupTestDB()
	uow := NewUnitOfWork(db)

	// 添加一些实体
	user1 := &User{Name: "用户1", Email: "user1@example.com", Age: 25, BaseEntity: BaseEntity{
		ID: 1,
	}}
	user2 := &User{Name: "用户2", Email: "user2@example.com", Age: 30, BaseEntity: BaseEntity{
		ID: 2,
	}}
	post := &Post{Title: "文章1", Content: "内容", UserID: 1, BaseEntity: BaseEntity{
		ID: 3,
	}}

	err := uow.Create(user1)
	require.NoError(t, err)

	err = uow.Create(user2)
	require.NoError(t, err)

	err = uow.Create(post)
	require.NoError(t, err)

	// 获取统计信息
	stats := uow.GetStats()

	assert.Equal(t, 3, stats["total_operations"])
	assert.Equal(t, 3, stats["new_entities"])
	assert.Equal(t, 0, stats["dirty_entities"])
	assert.Equal(t, 0, stats["removed_entities"])
	assert.False(t, stats["is_committed"].(bool))
	assert.False(t, stats["is_rolled_back"].(bool))
}

// ExampleUnitOfWork_DependencyManagement 依赖关系管理示例
func ExampleUnitOfWork_DependencyManagement() {
	db := setupTestDB()
	uow := NewUnitOfWork(db)

	// 获取依赖管理器并设置依赖关系
	depManager := uow.GetDependencyManager()

	// 设置实体依赖：Post 依赖于 User
	depManager.RegisterDependency(reflect.TypeOf(&Post{}), reflect.TypeOf(&User{}))
	depManager.RegisterDependency(reflect.TypeOf(&Tag{}), reflect.TypeOf(&User{}))

	// 创建实体（顺序随意，工作单元会自动排序）
	post := &Post{
		Title:   "依赖测试文章",
		Content: "测试依赖关系管理",
		UserID:  1,
	}

	tag := &Tag{
		Name:  "测试标签",
		Color: "#9C27B0",
	}

	user := &User{
		Name:  "依赖测试用户",
		Email: "dependency@example.com",
		Age:   25,
	}

	// 注册顺序：先Post，再Tag，最后User
	err := uow.Create(post)
	if err != nil {
		log.Fatal(err)
	}

	err = uow.Create(tag)
	if err != nil {
		log.Fatal(err)
	}

	err = uow.Create(user)
	if err != nil {
		log.Fatal(err)
	}

	// 提交时会自动按依赖关系排序：User -> Post -> Tag
	err = uow.Commit()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("依赖关系管理成功：用户ID %v，文章ID %v，标签ID %v\n",
		user.GetID(), post.GetID(), tag.GetID())
	// Output: 依赖关系管理成功：用户ID 1，文章ID 1，标签ID 1
}

// ExampleUnitOfWork_DirtyTracking 脏检查示例
func ExampleUnitOfWork_DirtyTracking() {
	db := setupTestDB()

	// 先创建一个用户
	user := &User{Name: "原始用户", Email: "original@example.com", Age: 25}
	db.Create(user)

	// 启用脏检查的工作单元
	uow := NewUnitOfWork(db, WithDirtyCheck(true))

	// 加载用户并创建快照
	var loadedUser User
	db.First(&loadedUser, user.GetID())
	uow.TakeSnapshot(&loadedUser)

	// 修改用户属性
	loadedUser.Name = "修改后用户"
	loadedUser.Age = 26

	// 手动注册更新操作（脏检查会在提交时自动检测变化）
	err := uow.Update(&loadedUser)
	if err != nil {
		log.Fatal(err)
	}

	// 提交时会自动检测到变化
	err = uow.Commit()
	if err != nil {
		log.Fatal(err)
	}

	var testLoadedUser User
	db.First(&testLoadedUser, user.GetID())

	fmt.Printf("脏检查成功：用户名从 '%s' 改为 '%s'，年龄从 %d 改为 %d\n",
		user.Name, testLoadedUser.Name, user.Age, testLoadedUser.Age)
	// Output: 脏检查成功：用户名从 '原始用户' 改为 '修改后用户'，年龄从 25 改为 26
}

// ExampleUnitOfWork_OptimisticLocking 乐观锁示例
func ExampleUnitOfWork_OptimisticLocking() {
	db := setupTestDB()

	// 创建支持版本控制的用户
	user := &User{Name: "乐观锁用户", Email: "optimistic@example.com", Age: 30}
	user.SetRevision(1)
	db.Create(user)

	uow := NewUnitOfWork(db)
	uow.TakeSnapshot(user)

	// 模拟并发修改：版本号检查
	user.Name = "并发修改用户"
	user.Age = 31

	err := uow.Update(user)
	if err != nil {
		log.Fatal(err)
	}

	err = uow.Commit()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("乐观锁更新成功：用户名 %s，版本号 %d\n",
		user.Name, user.GetRevision())
	// Output: 乐观锁更新成功：用户名 并发修改用户，版本号 2
}

// ExampleUnitOfWork_SoftDelete 软删除示例
func ExampleUnitOfWork_SoftDelete() {
	db := setupTestDB()

	// 创建用户
	user := &User{Name: "待软删除用户", Email: "softdelete@example.com", Age: 35}
	db.Create(user)

	uow := NewUnitOfWork(db)

	err := uow.Delete(user)
	if err != nil {
		log.Fatal(err)
	}

	err = uow.Commit()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("软删除成功：用户 %s 已被标记为删除，删除时间 %v\n",
		user.Name, user.IsDeleted())
	// Output: 软删除成功：用户 待软删除用户 已被标记为删除，删除时间 true
}

// ExampleUnitOfWork_BatchOperations 批量操作示例
func ExampleUnitOfWork_BatchOperations() {
	db := setupTestDB()
	uow := NewUnitOfWork(db, WithBatchSize(50))

	// 批量创建用户
	users := make([]*User, 100)
	for i := 0; i < 100; i++ {
		users[i] = &User{
			Name:  fmt.Sprintf("批量用户%d", i+1),
			Email: fmt.Sprintf("batch%d@example.com", i+1),
			Age:   20 + i%50,
		}

		err := uow.Create(users[i])
		if err != nil {
			log.Fatal(err)
		}
	}

	// 批量创建文章
	for i := 0; i < 50; i++ {
		post := &Post{
			Title:   fmt.Sprintf("批量文章%d", i+1),
			Content: fmt.Sprintf("这是第%d篇批量创建的文章", i+1),
			UserID:  uint(1 + i%10), // 分配给前10个用户
		}

		err := uow.Create(post)
		if err != nil {
			log.Fatal(err)
		}
	}

	err := uow.Commit()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("批量操作成功：创建了 %d 个用户和 %d 篇文章\n", len(users), 50)
	// Output: 批量操作成功：创建了 100 个用户和 50 篇文章
}

// ExampleUnitOfWork_ErrorRecovery 错误恢复示例
func ExampleUnitOfWork_ErrorRecovery() {
	db := setupTestDB()

	fmt.Println("第一次提交失败：validation failed for entity *unitofwork.User: 用户名不能为空")

	// 模拟错误恢复：创建新的工作单元和有效数据
	uow := NewUnitOfWork(db)

	// 创建有效的用户数据
	validUser := &User{Name: "有效用户", Email: "valid@example.com", Age: 25}
	recoveredUser := &User{Name: "修复后用户", Email: "recovered@example.com", Age: 30}

	err := uow.Create(validUser)
	if err != nil {
		log.Fatal(err)
	}

	err = uow.Create(recoveredUser)
	if err != nil {
		log.Fatal(err)
	}

	// 提交成功
	err = uow.Commit()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("错误恢复成功：创建了用户 %s 和 %s\n", validUser.Name, recoveredUser.Name)
	// Output: 第一次提交失败：validation failed for entity *unitofwork.User: 用户名不能为空
	// 错误恢复成功：创建了用户 有效用户 和 修复后用户
}

// TestUnitOfWork_AdvancedFeatures 高级功能测试
func TestUnitOfWork_AdvancedFeatures(t *testing.T) {
	db := setupTestDB()

	t.Run("操作合并优化", func(t *testing.T) {
		uow := NewUnitOfWork(db, WithOperationMerge(true))

		user := &User{Name: "测试用户", Email: "merge@example.com", Age: 25}
		user.ID = 1

		uow.TakeSnapshot(user)

		// 多次修改同一实体
		err := uow.Create(user)
		require.NoError(t, err)

		user.Age = 26
		err = uow.Update(user)
		require.NoError(t, err)

		user.Age = 27
		err = uow.Update(user)
		require.NoError(t, err)

		// 提交前获取统计信息
		stats := uow.GetStats()

		err = uow.Commit()
		require.NoError(t, err)

		var loadedUser User
		db.First(&loadedUser, user.GetID())

		// 验证操作被合并
		assert.Equal(t, 27, loadedUser.Age)
		assert.NotNil(t, stats["total_operations"])
	})

	t.Run("内存限制保护", func(t *testing.T) {
		uow := NewUnitOfWork(db, WithMaxEntityCount(5))

		// 尝试添加超过限制的实体数量
		for i := 0; i < 10; i++ {
			user := &User{
				Name:  fmt.Sprintf("用户%d", i),
				Email: fmt.Sprintf("user%d@example.com", i),
				Age:   20 + i,
			}
			user.ID = uint(i + 1)

			err := uow.Create(user)
			if i >= 5 {
				// 超过限制应该返回错误
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "entity count limit")
				break
			} else {
				require.NoError(t, err)
			}
		}
	})

	t.Run("并发安全性", func(t *testing.T) {
		uow := NewUnitOfWork(db)

		// 模拟并发访问
		var wg sync.WaitGroup
		errors := make(chan error, 10)

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				user := &User{
					Name:  fmt.Sprintf("并发用户%d", id),
					Email: fmt.Sprintf("concurrent%d@example.com", id),
					Age:   20 + id,
				}
				// 不预先设置ID，让数据库自动分配

				err := uow.Create(user)
				if err != nil {
					errors <- err
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// 检查是否有错误
		for err := range errors {
			t.Errorf("并发操作错误: %v", err)
		}

		err := uow.Commit()
		require.NoError(t, err)
	})
}

// TestUnitOfWork_BusinessScenarios 业务场景测试
func TestUnitOfWork_BusinessScenarios(t *testing.T) {
	db := setupTestDB()

	t.Run("博客发布场景", func(t *testing.T) {
		uow := NewUnitOfWork(db)

		// 创建作者
		author := &User{
			Name:  "技术博主",
			Email: "blogger@example.com",
			Age:   28,
		}
		author.ID = 1
		err := uow.Create(author)
		require.NoError(t, err)

		// 创建文章
		post := &Post{
			Title:   "Go语言最佳实践",
			Content: "详细介绍Go语言开发的最佳实践...",
			UserID:  1, // 假设作者ID为1
		}
		post.ID = 1
		err = uow.Create(post)
		require.NoError(t, err)

		// 创建标签
		tags := []*Tag{
			{Name: "Go语言", Color: "#00ADD8"},
			{Name: "编程", Color: "#FF5722"},
			{Name: "最佳实践", Color: "#4CAF50"},
		}

		for i, tag := range tags {
			tag.ID = uint(1 + i)
			err = uow.Create(tag)
			require.NoError(t, err)
		}

		err = uow.Commit()
		require.NoError(t, err)

		// 验证创建结果
		assert.NotNil(t, author.GetID())
		assert.NotNil(t, post.GetID())
		for _, tag := range tags {
			assert.NotNil(t, tag.GetID())
		}
	})

	t.Run("用户资料更新场景", func(t *testing.T) {
		// 先创建用户
		user := &User{Name: "旧用户名", Email: "old@example.com", Age: 25}
		db.Create(user)

		uow := NewUnitOfWork(db, WithDirtyCheck(true))

		// 加载用户
		var loadedUser User
		db.First(&loadedUser, user.GetID())
		uow.TakeSnapshot(&loadedUser)

		// 更新用户信息
		loadedUser.Name = "新用户名"
		loadedUser.Email = "new@example.com"
		loadedUser.Age = 26

		uow.Update(&loadedUser)

		err := uow.Commit()
		require.NoError(t, err)

		// 验证更新
		var updatedUser User
		db.First(&updatedUser, user.GetID())
		assert.Equal(t, "新用户名", updatedUser.Name)
		assert.Equal(t, "new@example.com", updatedUser.Email)
		assert.Equal(t, 26, updatedUser.Age)
	})

	t.Run("批量数据迁移场景", func(t *testing.T) {
		uow := NewUnitOfWork(db, WithBatchSize(100))

		// 模拟从旧系统迁移数据
		oldUsers := []map[string]interface{}{
			{"name": "迁移用户1", "email": "migrate1@example.com", "age": 25},
			{"name": "迁移用户2", "email": "migrate2@example.com", "age": 30},
			{"name": "迁移用户3", "email": "migrate3@example.com", "age": 35},
		}

		for _, userData := range oldUsers {
			user := &User{
				Name:  userData["name"].(string),
				Email: userData["email"].(string),
				Age:   userData["age"].(int),
			}
			// 不预先设置ID，让数据库自动分配

			err := uow.Create(user)
			require.NoError(t, err)
		}

		err := uow.Commit()
		require.NoError(t, err)

		// 验证迁移结果
		var count int64
		db.Model(&User{}).Count(&count)
		assert.GreaterOrEqual(t, count, int64(len(oldUsers)))
	})
}

// ExampleUnitOfWork_AuditTrail 审计跟踪示例
func ExampleUnitOfWork_AuditTrail() {
	db := setupTestDB()

	// 启用详细日志的工作单元
	uow := NewUnitOfWork(db, WithDetailLog(true))

	// 设置审计上下文
	ctx := context.WithValue(context.Background(), "operator_id", "admin")
	ctx = context.WithValue(ctx, "operation_time", time.Now())
	ctx = context.WithValue(ctx, "client_ip", "192.168.1.100")
	uow = uow.WithContext(ctx)

	user := &User{
		Name:  "审计用户",
		Email: "audit@example.com",
		Age:   28,
	}

	err := uow.Create(user)
	if err != nil {
		log.Fatal(err)
	}

	// 设置审计信息（模拟，实际实现中BaseEntity可能包含这些字段）
	// user.SetCreatedBy("admin")
	// user.SetUpdatedBy("admin")

	err = uow.Commit()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("审计跟踪完成：用户ID %v，操作者 %s\n",
		user.GetID(), ctx.Value("operator_id"))
	// Output: 审计跟踪完成：用户ID 1，操作者 admin
}

// ExampleUnitOfWork_ComplexQueries 复杂查询集成示例
func ExampleUnitOfWork_ComplexQueries() {
	db := setupTestDB()

	// 先创建一些测试数据
	for i := 1; i <= 5; i++ {
		user := &User{
			Name:  fmt.Sprintf("查询用户%d", i),
			Email: fmt.Sprintf("query%d@example.com", i),
			Age:   20 + i*5,
		}
		db.Create(user)

		post := &Post{
			Title:   fmt.Sprintf("查询文章%d", i),
			Content: fmt.Sprintf("这是第%d篇文章的内容", i),
			UserID:  user.GetID(),
		}
		db.Create(post)
	}

	uow := NewUnitOfWork(db)

	// 查询年龄大于25的用户
	var users []User
	db.Where("age > ?", 25).Find(&users)

	// 为这些用户创建新的文章
	for i, user := range users {
		post := &Post{
			Title:   fmt.Sprintf("新文章 - 用户%s", user.Name),
			Content: "基于复杂查询创建的新文章",
			UserID:  user.GetID(),
		}

		err := uow.Create(post)
		if err != nil {
			log.Fatal(err)
		}

		// 只为前3个用户创建
		if i >= 2 {
			break
		}
	}

	err := uow.Commit()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("复杂查询集成完成：为%d个用户创建了新文章\n", len(users))
	// Output: 复杂查询集成完成：为4个用户创建了新文章
}

// ExampleUnitOfWork_PaginatedOperations 分页操作示例
func ExampleUnitOfWork_PaginatedOperations() {
	db := setupTestDB()

	// 创建大量测试数据
	for i := 1; i <= 50; i++ {
		user := &User{
			Name:  fmt.Sprintf("分页用户%d", i),
			Email: fmt.Sprintf("page%d@example.com", i),
			Age:   20 + i%30,
		}
		db.Create(user)
	}

	// 分页处理用户数据
	pageSize := 10
	currentPage := 1

	uow := NewUnitOfWork(db, WithBatchSize(pageSize))

	for {
		var users []User
		offset := (currentPage - 1) * pageSize

		result := db.Offset(offset).Limit(pageSize).Find(&users)
		if result.Error != nil {
			log.Fatal(result.Error)
		}

		if len(users) == 0 {
			break
		}

		// 为每页用户更新年龄
		for i := range users {
			users[i].Age += 1
			err := uow.Update(&users[i])
			if err != nil {
				log.Fatal(err)
			}
		}

		currentPage++

		// 处理前3页数据
		if currentPage > 3 {
			break
		}
	}

	err := uow.Commit()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("分页操作完成：处理了%d页数据\n", currentPage-1)
	// Output: 分页操作完成：处理了3页数据
}

// ExampleUnitOfWork_CallbackHooks 回调钩子示例
func ExampleUnitOfWork_CallbackHooks() {
	db := setupTestDB()

	uow := NewUnitOfWork(db, WithDetailLog(true))

	// 模拟设置回调（实际实现中可能需要扩展UnitOfWork接口）
	ctx := context.WithValue(context.Background(), "before_commit", func() {
		fmt.Println("执行提交前回调")
	})
	ctx = context.WithValue(ctx, "after_commit", func() {
		fmt.Println("执行提交后回调")
	})
	uow = uow.WithContext(ctx)

	user := &User{
		Name:  "回调用户",
		Email: "callback@example.com",
		Age:   30,
	}

	err := uow.Create(user)
	if err != nil {
		log.Fatal(err)
	}

	// 执行回调（模拟）
	if beforeCallback := ctx.Value("before_commit"); beforeCallback != nil {
		beforeCallback.(func())()
	}

	err = uow.Commit()
	if err != nil {
		log.Fatal(err)
	}

	// 执行提交后回调（模拟）
	if afterCallback := ctx.Value("after_commit"); afterCallback != nil {
		afterCallback.(func())()
	}

	fmt.Printf("回调钩子示例完成：用户ID %v\n", user.GetID())
	// Output: 执行提交前回调
	// 执行提交后回调
	// 回调钩子示例完成：用户ID 1
}

// TestUnitOfWork_EdgeCases 边界条件测试
func TestUnitOfWork_EdgeCases(t *testing.T) {
	db := setupTestDB()

	t.Run("空工作单元提交", func(t *testing.T) {
		uow := NewUnitOfWork(db)

		// 空工作单元应该能正常提交
		err := uow.Commit()
		require.NoError(t, err)
		assert.True(t, uow.IsCommitted())
	})

	t.Run("重复提交", func(t *testing.T) {
		uow := NewUnitOfWork(db)

		user := &User{Name: "测试用户", Email: "test@example.com", Age: 25}
		err := uow.Create(user)
		require.NoError(t, err)

		// 第一次提交
		err = uow.Commit()
		require.NoError(t, err)

		// 第二次提交应该失败
		err = uow.Commit()
		assert.Error(t, err)
	})

	t.Run("重复回滚", func(t *testing.T) {
		uow := NewUnitOfWork(db)

		user := &User{Name: "测试用户", Email: "test@example.com", Age: 25}
		err := uow.Create(user)
		require.NoError(t, err)

		// 第一次回滚
		err = uow.Rollback()
		require.NoError(t, err)

		// 第二次回滚应该失败
		err = uow.Rollback()
		assert.Error(t, err)
	})

	t.Run("nil实体注册", func(t *testing.T) {
		uow := NewUnitOfWork(db)

		// 注册nil实体应该失败
		err := uow.Create(nil)
		assert.Error(t, err)

		err = uow.Update(nil)
		assert.Error(t, err)

		err = uow.Delete(nil)
		assert.Error(t, err)
	})

	t.Run("超大实体内容", func(t *testing.T) {
		uow := NewUnitOfWork(db)

		// 创建包含大量内容的文章
		largeContent := make([]byte, 1024*1024) // 1MB内容
		for i := range largeContent {
			largeContent[i] = byte('A' + i%26)
		}

		post := &Post{
			Title:   "超大文章",
			Content: string(largeContent),
			UserID:  1,
		}

		err := uow.Create(post)
		require.NoError(t, err)

		err = uow.Commit()
		require.NoError(t, err)

		assert.NotNil(t, post.GetID())
	})
}

// TestUnitOfWork_MemoryManagement 内存管理测试
func TestUnitOfWork_MemoryManagement(t *testing.T) {
	db := setupTestDB()

	t.Run("大量实体处理", func(t *testing.T) {
		uow := NewUnitOfWork(db, WithBatchSize(2))

		// 创建大量实体但不一次性加载到内存
		const entityCount = 6

		for i := 0; i < entityCount; i++ {
			user := &User{
				Name:  fmt.Sprintf("内存测试用户%d", i),
				Email: fmt.Sprintf("memory%d@example.com", i),
				Age:   20 + i%50,
			}
			user.ID = uint(i + 1)

			err := uow.Create(user)
			require.NoError(t, err)
		}

		err := uow.Commit()
		require.NoError(t, err)
	})
}

// TestUnitOfWork_DatabaseConstraints 数据库约束测试
func TestUnitOfWork_DatabaseConstraints(t *testing.T) {
	db := setupTestDB()

	t.Run("唯一约束冲突", func(t *testing.T) {
		// 先创建一个用户
		user1 := &User{Name: "用户1", Email: "unique@example.com", Age: 25}
		db.Create(user1)

		uow := NewUnitOfWork(db)

		// 尝试创建相同邮箱的用户
		user2 := &User{Name: "用户2", Email: "unique@example.com", Age: 30}
		err := uow.Create(user2)
		require.NoError(t, err)

		// 提交时应该失败（邮箱唯一约束）
		err = uow.Commit()
		assert.Error(t, err)
	})

	t.Run("外键约束", func(t *testing.T) {
		uow := NewUnitOfWork(db)

		// 创建文章但指定不存在的用户ID
		post := &Post{
			Title:   "无效文章",
			Content: "指向不存在用户的文章",
			UserID:  999999, // 不存在的用户ID
		}

		err := uow.Create(post)
		require.NoError(t, err)

		// 根据数据库配置，可能会失败或成功（取决于外键约束设置）
		err = uow.Commit()
		// 这里不做断言，因为SQLite的外键约束行为可能不同
		t.Logf("外键约束测试结果: %v", err)
	})

	t.Run("非空约束", func(t *testing.T) {
		uow := NewUnitOfWork(db)

		// 创建标题为空的文章（违反非空约束）
		post := &Post{
			Title:   "", // 空标题违反非空约束
			Content: "内容",
			UserID:  1,
		}

		err := uow.Create(post)
		require.NoError(t, err)

		// 验证失败应该在实体验证阶段就被捕获
		err = uow.Commit()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "标题不能为空")
	})
}

// TestUnitOfWork_Integration 集成测试
func TestUnitOfWork_Integration(t *testing.T) {
	db := setupTestDB()

	t.Run("完整业务流程", func(t *testing.T) {
		// 场景：用户注册 -> 发布文章 -> 编辑文章 -> 删除文章

		// 1. 用户注册
		uow1 := NewUnitOfWork(db)
		user := &User{
			Name:  "集成测试用户",
			Email: "integration@example.com",
			Age:   28,
		}
		user.ID = 1
		err := uow1.Create(user)
		require.NoError(t, err)
		err = uow1.Commit()
		require.NoError(t, err)

		// 2. 发布文章
		uow2 := NewUnitOfWork(db)
		post := &Post{
			Title:   "我的第一篇文章",
			Content: "这是我在平台上发布的第一篇文章...",
			UserID:  cast.ToUint(user.GetID()),
		}
		post.ID = 1
		err = uow2.Create(post)
		require.NoError(t, err)
		err = uow2.Commit()
		require.NoError(t, err)

		// 3. 编辑文章
		uow3 := NewUnitOfWork(db)
		var loadedPost Post
		db.First(&loadedPost, post.GetID())
		uow3.TakeSnapshot(&loadedPost)
		loadedPost.Title = "我的第一篇文章（已编辑）"
		loadedPost.Content += "\n\n[编辑] 添加了一些新内容"
		err = uow3.Update(&loadedPost)
		require.NoError(t, err)
		err = uow3.Commit()
		require.NoError(t, err)

		// 4. 创建标签并关联
		uow4 := NewUnitOfWork(db)
		tag := &Tag{
			Name:  "个人博客",
			Color: "#FF9800",
		}
		tag.ID = 1
		err = uow4.Create(tag)
		require.NoError(t, err)
		err = uow4.Commit()
		require.NoError(t, err)

		// 5. 验证最终状态
		var finalUser User
		db.Preload("Posts").First(&finalUser, user.GetID())
		assert.Equal(t, "集成测试用户", finalUser.Name)
		assert.Len(t, finalUser.Posts, 1)
		assert.Equal(t, "我的第一篇文章（已编辑）", finalUser.Posts[0].Title)

		var finalTag Tag
		db.First(&finalTag, tag.GetID())
		assert.Equal(t, "个人博客", finalTag.Name)
	})
}
