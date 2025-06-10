# 工作单元模式 GORM 插件

这是一个为 GORM 设计的工作单元模式插件，提供了完整的工作单元模式实现，包括自动事务管理、实体跟踪、依赖关系管理、脏检查等功能。

## 功能特性

- ✅ **自动事务管理**: 自动管理数据库事务的开始、提交和回滚
- ✅ **实体跟踪**: 自动跟踪实体的创建、更新和删除操作
- ✅ **依赖关系管理**: 基于实体依赖关系自动排序操作执行顺序
- ✅ **脏检查**: 自动检测实体变更，只更新发生变化的字段
- ✅ **批量优化**: 自动合并和优化数据库操作
- ✅ **乐观锁**: 支持乐观锁并发控制
- ✅ **软删除**: 支持软删除机制
- ✅ **并发安全**: 线程安全的实现
- ✅ **上下文管理**: 完善的上下文传递和管理
- ✅ **详细日志**: 可配置的详细操作日志

## 快速开始

### 1. 安装插件

```go
import "github.com/unionj-cloud/toolkit/unitofwork"
```

### 2. 定义实体

实体需要实现 `Entity` 接口，推荐继承 `BaseEntity`：

```go
type User struct {
    unitofwork.BaseEntity
    Name  string `gorm:"size:100;not null" json:"name"`
    Email string `gorm:"size:255;uniqueIndex" json:"email"`
    Age   int    `json:"age"`
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
    return nil
}
```

### 3. 注册插件

```go
// 创建数据库连接
db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
if err != nil {
    panic("数据库连接失败")
}

// 注册工作单元插件
plugin := unitofwork.NewPlugin()
if err := db.Use(plugin); err != nil {
    panic("插件注册失败")
}
```

### 4. 使用工作单元

```go
err := unitofwork.WithUnitOfWork(context.Background(), db, func(db *gorm.DB, uow *unitofwork.UnitOfWork) error {
    // 创建用户
    user := &User{
        Name:  "张三",
        Email: "zhangsan@example.com",
        Age:   25,
    }
    if err := db.Create(user).Error; err != nil {
        return err
    }

    // 创建文章
    post := &Post{
        Title:   "我的第一篇文章",
        Content: "文章内容...",
        UserID:  user.GetID(),
    }
    return db.Create(post).Error
})
```

## 配置选项

### 插件配置

```go
plugin := unitofwork.NewPlugin(
    // 启用/禁用自动管理
    unitofwork.WithPluginAutoManage(true),
    
    // 自定义上下文键名
    unitofwork.WithPluginContextKey("my_uow"),
    
    // 配置工作单元选项
    unitofwork.WithPluginUnitOfWorkConfig(&unitofwork.Config{
        EnableDirtyCheck:     true,  // 启用脏检查
        BatchSize:            1000,  // 批量操作大小
        EnableOperationMerge: true,  // 启用操作合并
        MaxEntityCount:       5000,  // 最大实体数量
        EnableDetailLog:      false, // 启用详细日志
    }),
    
    // 配置实体依赖关系
    unitofwork.WithPluginDependencyMapping(map[reflect.Type][]reflect.Type{
        reflect.TypeOf(&Post{}): {reflect.TypeOf(&User{})}, // Post 依赖于 User
        reflect.TypeOf(&Tag{}):  {reflect.TypeOf(&User{})}, // Tag 依赖于 User
    }),
)
```

### 工作单元配置

```go
uow := unitofwork.NewUnitOfWork(db,
    unitofwork.WithDirtyCheck(true),      // 启用脏检查
    unitofwork.WithBatchSize(500),        // 批量大小
    unitofwork.WithOperationMerge(true),  // 操作合并
    unitofwork.WithMaxEntityCount(1000),  // 实体数量限制
    unitofwork.WithDetailLog(true),       // 详细日志
)
```

## 高级用法

### 依赖关系管理

```go
// 配置实体依赖关系
dependencyMapping := map[reflect.Type][]reflect.Type{
    reflect.TypeOf(&Post{}): {reflect.TypeOf(&User{})},
    reflect.TypeOf(&Tag{}):  {reflect.TypeOf(&User{})},
}

plugin := unitofwork.NewPlugin(
    unitofwork.WithPluginDependencyMapping(dependencyMapping),
)

// 使用时，操作会自动按依赖关系排序
err := unitofwork.WithUnitOfWork(ctx, db, func(db *gorm.DB, uow *unitofwork.UnitOfWork) error {
    // 即使先创建 Post，实际执行时会先创建 User
    post := &Post{Title: "文章", Content: "内容", UserID: 1}
    db.Create(post)
    
    user := &User{Name: "作者", Email: "author@example.com", Age: 30}
    user.ID = 1
    db.Create(user)
    
    return nil
})
```

### 脏检查

```go
// 启用脏检查后，只有实际发生变化的字段才会被更新
err := unitofwork.WithUnitOfWork(ctx, db, func(db *gorm.DB, uow *unitofwork.UnitOfWork) error {
    // 加载实体并创建快照
    var user User
    db.First(&user, 1)
    uow.TakeSnapshot(&user)
    
    // 修改实体
    user.Age = 26  // 只有这个字段会被更新
    
    return db.Save(&user).Error
})
```

### 手动工作单元管理

```go
// 禁用自动管理，手动控制工作单元
plugin := unitofwork.NewPlugin(unitofwork.WithPluginAutoManage(false))
db.Use(plugin)

// 手动创建工作单元
uow := unitofwork.NewUnitOfWork(db)

// 手动注册实体
user := &User{Name: "手动用户", Email: "manual@example.com", Age: 25}
if err := uow.Create(user); err != nil {
    return err
}

// 手动提交
if err := uow.Commit(); err != nil {
    return err
}
```

### 错误处理和回滚

```go
err := unitofwork.WithUnitOfWork(ctx, db, func(db *gorm.DB, uow *unitofwork.UnitOfWork) error {
    // 创建用户
    user := &User{Name: "用户", Email: "user@example.com", Age: 25}
    if err := db.Create(user).Error; err != nil {
        return err
    }
    
    // 模拟业务逻辑错误
    if someCondition {
        return errors.New("业务逻辑错误")
    }
    
    return nil
})

if err != nil {
    // 发生错误时，所有操作都会自动回滚
    fmt.Printf("操作失败: %v", err)
}
```

## 实体接口

### 基础接口

```go
type Entity interface {
    GetID() uint
    SetID(id uint)
    GetTableName() string
    IsNew() bool
}
```

### 扩展接口

```go
// 乐观锁支持
type HasRevision interface {
    Entity
    GetRevision() int64
    SetRevision(revision int64)
    GetRevisionNext() int64
}

// 时间戳支持
type HasTimestamps interface {
    Entity
    GetCreatedAt() time.Time
    SetCreatedAt(createdAt time.Time)
    GetUpdatedAt() time.Time
    SetUpdatedAt(updatedAt time.Time)
}

// 软删除支持
type SoftDelete interface {
    Entity
    GetDeletedAt() gorm.DeletedAt
    SetDeletedAt(deletedAt gorm.DeletedAt)
    IsDeleted() bool
}

// 验证支持
type Validatable interface {
    Entity
    Validate() error
}
```

## 性能优化

1. **批量操作**: 自动将相同类型的操作合并为批量操作
2. **操作优化**: 移除冗余操作（如创建后立即删除）
3. **依赖排序**: 按依赖关系优化执行顺序
4. **内存保护**: 可配置的实体数量限制
5. **连接池**: 复用数据库连接

## 最佳实践

1. **实体设计**: 继承 `BaseEntity` 并实现必要的接口
2. **依赖配置**: 提前配置好实体间的依赖关系
3. **错误处理**: 适当处理验证错误和业务逻辑错误
4. **批量大小**: 根据数据量调整合适的批量大小
5. **日志配置**: 在开发环境启用详细日志，生产环境关闭
6. **并发控制**: 利用乐观锁处理并发更新

## 注意事项

1. 实体必须实现 `Entity` 接口才能被工作单元管理
2. 启用自动管理时，GORM 的默认 CRUD 操作会被拦截
3. 依赖关系配置应该在插件初始化时完成
4. 脏检查会增加内存使用，大量数据时需要注意
5. 工作单元是线程安全的，但建议每个请求使用独立的工作单元

## 许可证

MIT License 