# GORM 自动化工作单元插件集成方案

## 概述

本方案基于 `wubin1989/gorm` 的插件与回调机制，实现了一个完全透明的自动化工作单元模式。开发者无需手动管理事务和实体变更跟踪，所有数据库操作将自动纳入工作单元管理，确保数据一致性和操作优化。

## 设计目标

- **完全透明**：对开发者零侵入，无需修改现有业务代码
- **自动事务管理**：自动开启、提交或回滚事务
- **智能实体跟踪**：自动检测并管理新增、修改、删除的实体
- **操作优化**：自动合并无效操作，批量执行，依赖排序
- **灵活配置**：支持多种配置选项，适应不同场景需求
- **插件兼容**：与其他 GORM 插件（如缓存插件）协同工作

## 核心特性

### 1. 自动工作单元管理

- **事务级生命周期**：每个事务自动创建独立的工作单元
- **实体自动注册**：Create/Update/Delete 操作自动注册到工作单元
- **依赖关系管理**：自动按外键依赖关系排序操作
- **批量优化**：相同类型操作自动合并为批量操作

### 2. 智能实体检测

- **接口自动检测**：自动识别实现 `Entity` 接口的对象
- **方法反射检测**：通过反射检测必要方法，自动包装为实体
- **自定义检测器**：支持自定义实体检测逻辑
- **类型安全**：确保只有有效实体参与工作单元管理

### 3. 脏检查与快照

- **自动快照**：查询后自动为实体创建快照
- **脏检查**：自动检测实体变更，只更新真正修改的字段
- **变更跟踪**：详细记录字段级别的变更信息

### 4. 错误处理与回滚

- **自动回滚**：发生错误时自动回滚所有变更
- **验证集成**：自动执行实体验证，验证失败时回滚
- **错误传播**：保持原有的错误处理语义

## 技术实现

### 核心组件

```go
// AutoUnitOfWorkPlugin 自动化工作单元插件
type AutoUnitOfWorkPlugin struct {
    config       *AutoUowConfig
    globalConfig *Config
    mu           sync.RWMutex
}

// AutoUowConfig 插件配置
type AutoUowConfig struct {
    Enabled          bool                        // 是否启用插件
    AutoDetectEntity bool                        // 是否自动检测实体接口
    AutoSnapshot     bool                        // 是否在查询时自动创建快照
    SkipReadOnly     bool                        // 是否跳过只读操作
    EntityDetector   func(interface{}) Entity    // 自定义实体检测函数
    VerboseLog       bool                        // 是否启用详细日志
    ExcludedTables   []string                   // 排除的表名
    ContextKey       string                      // 自定义上下文键名
}
```

### 回调注册机制

插件通过 GORM 的回调机制在以下时点注入逻辑：

1. **Begin**: 事务开始时创建工作单元
2. **Query**: 查询后创建实体快照（可选）
3. **Create/Update/Delete**: 拦截写操作，注册到工作单元
4. **Commit**: 事务提交时执行工作单元提交
5. **Rollback**: 事务回滚时执行工作单元回滚

### 实体自动检测

```go
// 直接接口检测
if entity, ok := obj.(Entity); ok {
    return entity
}

// 反射方法检测
requiredMethods := []string{"GetID", "SetID", "GetTableName", "IsNew"}
for _, methodName := range requiredMethods {
    if _, found := objType.MethodByName(methodName); !found {
        return nil // 不是有效实体
    }
}

// 创建包装器
wrapper := &entityWrapper{obj: obj, value: objValue, typ: objType}
return wrapper
```

## 使用指南

### 基本使用

```go
package main

import (
    "github.com/wubin1989/gorm"
    "github.com/wubin1989/sqlite"
    "github.com/unionj-cloud/toolkit/unitofwork"
)

func main() {
    // 1. 初始化数据库
    db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
    if err != nil {
        panic(err)
    }

    // 2. 注册自动化工作单元插件
    plugin := unitofwork.NewAutoUnitOfWorkPlugin(nil, nil)
    err = db.Use(plugin)
    if err != nil {
        panic(err)
    }

    // 3. 正常使用，插件会自动管理工作单元
    err = db.Transaction(func(tx *gorm.DB) error {
        user := &User{Name: "张三", Email: "zhangsan@example.com"}
        if err := tx.Create(user).Error; err != nil {
            return err
        }

        post := &Post{Title: "我的文章", UserID: user.ID}
        if err := tx.Create(post).Error; err != nil {
            return err
        }

        // 插件会自动按依赖关系排序，确保先创建用户再创建文章
        return nil
    })
}
```

### 高级配置

```go
// 自定义插件配置
autoConfig := &unitofwork.AutoUowConfig{
    Enabled:          true,
    AutoDetectEntity: true,
    AutoSnapshot:     true,
    VerboseLog:       true,                    // 启用详细日志
    ExcludedTables:   []string{"audit_logs"}, // 排除审计日志表
    EntityDetector: func(obj interface{}) unitofwork.Entity {
        // 自定义实体检测逻辑
        if entity, ok := obj.(unitofwork.Entity); ok {
            return entity
        }
        return nil
    },
}

// 自定义工作单元配置
globalConfig := &unitofwork.Config{
    EnableDirtyCheck:     true,
    BatchSize:            1000,
    EnableOperationMerge: true,
    MaxEntityCount:       10000,
    EnableDetailLog:      true,
}

plugin := unitofwork.NewAutoUnitOfWorkPlugin(autoConfig, globalConfig)
db.Use(plugin)
```

### 手动控制

在某些情况下，你可能需要手动访问工作单元：

```go
err = db.Transaction(func(tx *gorm.DB) error {
    // 获取当前工作单元
    uow := unitofwork.GetCurrentUnitOfWork(tx.Statement.Context)
    if uow != nil {
        // 获取统计信息
        stats := uow.GetStats()
        log.Printf("工作单元统计: %+v", stats)

        // 手动设置依赖关系
        depManager := uow.GetDependencyManager()
        depManager.RegisterDependency(
            reflect.TypeOf(&Post{}),
            reflect.TypeOf(&User{}),
        )
    }

    // 正常的数据库操作...
    return nil
})
```

## 实体定义要求

为了与插件正常工作，实体需要满足以下要求之一：

### 方式一：实现 Entity 接口

```go
type User struct {
    ID        uint      `gorm:"primaryKey"`
    Name      string    `gorm:"size:100;not null"`
    Email     string    `gorm:"size:255;uniqueIndex"`
    CreatedAt time.Time `gorm:"autoCreateTime"`
    UpdatedAt time.Time `gorm:"autoUpdateTime"`
    DeletedAt *time.Time `gorm:"index"`
    Revision  int64     `gorm:"default:1"`
}

func (u *User) GetID() interface{}     { return u.ID }
func (u *User) SetID(id interface{})   { u.ID = id.(uint) }
func (u *User) GetTableName() string   { return "users" }
func (u *User) IsNew() bool            { return u.ID == 0 }
```

### 方式二：嵌入 BaseEntity

```go
type User struct {
    unitofwork.BaseEntity
    Name  string `gorm:"size:100;not null"`
    Email string `gorm:"size:255;uniqueIndex"`
}

func (u *User) GetTableName() string { return "users" }
```

### 方式三：自动检测（需要有对应方法）

```go
type User struct {
    ID    uint   `gorm:"primaryKey"`
    Name  string `gorm:"size:100;not null"`
    Email string `gorm:"size:255;uniqueIndex"`
}

// 插件会通过反射检测这些方法
func (u *User) GetID() interface{}     { return u.ID }
func (u *User) SetID(id interface{})   { u.ID = id.(uint) }
func (u *User) GetTableName() string   { return "users" }
func (u *User) IsNew() bool            { return u.ID == 0 }
```

## 高级特性

### 依赖关系管理

插件支持自动或手动设置实体间的依赖关系：

```go
// 在事务中手动设置依赖关系
uow := unitofwork.GetCurrentUnitOfWork(tx.Statement.Context)
depManager := uow.GetDependencyManager()

// Post 依赖于 User
depManager.RegisterDependency(
    reflect.TypeOf(&Post{}),
    reflect.TypeOf(&User{}),
)
```

### 批量操作优化

插件会自动将相同类型的操作合并为批量操作：

```go
// 创建100个用户，插件会自动优化为批量插入
for i := 0; i < 100; i++ {
    user := &User{Name: fmt.Sprintf("用户%d", i)}
    tx.Create(user) // 每个操作都会被注册到工作单元
}
// 提交时自动执行批量插入
```

### 脏检查

启用脏检查后，插件会自动检测实体变更：

```go
// 查询用户（自动创建快照）
var user User
tx.First(&user, 1)

// 修改用户（自动检测变更）
user.Name = "新名字"
tx.Save(&user) // 只更新变更的字段
```

### 操作合并

插件会自动移除无效操作：

```go
user := &User{Name: "测试用户"}
tx.Create(user)  // 创建操作
tx.Delete(user)  // 删除操作

// 提交时，插件会自动移除这一对相互抵消的操作
```

## 性能优化

### 批量大小配置

```go
config := &unitofwork.Config{
    BatchSize: 1000, // 批量操作大小
}
```

### 内存限制

```go
config := &unitofwork.Config{
    MaxEntityCount: 10000, // 最大实体数量限制
}
```

### 操作合并

```go
config := &unitofwork.Config{
    EnableOperationMerge: true, // 启用操作合并优化
}
```

## 错误处理

### 自动回滚

```go
err = db.Transaction(func(tx *gorm.DB) error {
    user := &User{Name: "", Email: "test@example.com"} // 无效数据
    return tx.Create(user).Error // 验证失败
})
// 错误发生时，工作单元会自动回滚所有变更
```

### 验证集成

实体可以实现 `Validatable` 接口进行自动验证：

```go
func (u *User) Validate() error {
    if u.Name == "" {
        return fmt.Errorf("用户名不能为空")
    }
    return nil
}
```

## 日志和监控

### 详细日志

```go
config := &unitofwork.AutoUowConfig{
    VerboseLog: true, // 启用详细日志
}
```

日志示例：
```
[INFO] Auto UnitOfWork plugin initialized successfully
[DEBUG] Auto UnitOfWork: Created new unit of work for transaction
[DEBUG] Auto UnitOfWork: Registered new entity User
[DEBUG] Auto UnitOfWork: Registered new entity Post
[INFO] Auto UnitOfWork: Successfully committed unit of work
```

### 统计信息

```go
uow := unitofwork.GetCurrentUnitOfWork(ctx)
stats := uow.GetStats()
// 输出: map[new_entities:2 dirty_entities:1 removed_entities:0 ...]
```

## 与其他插件的兼容性

### 与缓存插件协同

```go
// 同时使用工作单元和缓存插件
cachePlugin := &caches.Caches{...}
uowPlugin := unitofwork.NewAutoUnitOfWorkPlugin(nil, nil)

db.Use(cachePlugin)
db.Use(uowPlugin)
```

### 插件执行顺序

插件会根据回调的注册顺序执行，通常建议：
1. 先注册缓存插件
2. 再注册工作单元插件

## 测试支持

### 单元测试

```go
func TestAutoUnitOfWork(t *testing.T) {
    db := setupTestDB(t)
    
    plugin := unitofwork.NewAutoUnitOfWorkPlugin(nil, nil)
    err := db.Use(plugin)
    require.NoError(t, err)
    
    err = db.Transaction(func(tx *gorm.DB) error {
        user := &User{Name: "测试用户"}
        return tx.Create(user).Error
    })
    
    assert.NoError(t, err)
}
```

### 集成测试

```go
func TestComplexWorkflow(t *testing.T) {
    // 测试复杂的业务流程
    err := db.Transaction(func(tx *gorm.DB) error {
        // 创建用户
        user := &User{Name: "测试用户"}
        if err := tx.Create(user).Error; err != nil {
            return err
        }
        
        // 创建文章
        post := &Post{Title: "测试文章", UserID: user.ID}
        if err := tx.Create(post).Error; err != nil {
            return err
        }
        
        // 更新用户
        user.Name = "更新后的用户"
        return tx.Save(user).Error
    })
    
    assert.NoError(t, err)
}
```

## 最佳实践

### 1. 实体设计

- 推荐使用嵌入 `BaseEntity` 的方式
- 实现必要的验证逻辑
- 正确设置外键关系

### 2. 事务使用

- 在事务中执行相关的数据库操作
- 避免在事务中执行长时间的外部调用
- 合理设置事务超时时间

### 3. 配置优化

- 根据业务场景调整批量大小
- 合理设置内存限制
- 在开发环境启用详细日志

### 4. 错误处理

- 实现完整的验证逻辑
- 合理处理事务回滚
- 记录详细的错误信息

## 常见问题

### Q: 插件会影响性能吗？

A: 插件通过批量操作、操作合并等优化手段，通常能提升性能。只有在大量小事务的场景下可能有轻微开销。

### Q: 如何调试工作单元的执行过程？

A: 启用 `VerboseLog` 配置可以看到详细的执行日志。还可以通过 `GetStats()` 方法获取统计信息。

### Q: 插件支持哪些数据库？

A: 插件基于 GORM，支持所有 GORM 支持的数据库（MySQL、PostgreSQL、SQLite、SQL Server 等）。

### Q: 如何与现有代码集成？

A: 插件设计为零侵入，只需注册插件即可。现有代码无需修改，插件会自动接管事务管理。

### Q: 可以禁用某些表的工作单元管理吗？

A: 可以通过 `ExcludedTables` 配置排除特定表，这些表的操作不会进入工作单元管理。

## 总结

GORM 自动化工作单元插件通过 GORM 的插件与回调机制，实现了完全透明的工作单元模式。插件具有以下优势：

- **零侵入**：无需修改现有业务代码
- **自动化**：全自动的事务和实体管理
- **高性能**：批量操作和智能优化
- **可配置**：丰富的配置选项适应不同需求
- **可扩展**：与其他插件良好兼容

这个插件特别适合以下场景：

- 企业级应用的数据一致性保障
- 复杂业务流程的事务管理
- 大量数据操作的性能优化
- 微服务架构中的本地事务管理

通过使用这个插件，开发者可以专注于业务逻辑的实现，而将复杂的事务管理和数据一致性保障交给框架自动处理。