# Go-doudou项目中工作单元模式的实践应用指南

## 引言

在现代微服务架构中，数据一致性和事务管理一直是开发者面临的核心挑战。特别是在使用 go-doudou 框架构建分布式应用时，如何优雅地处理复杂的业务逻辑、保证数据一致性、提升性能，成为了关键问题。

本文将深入介绍工作单元模式（Unit of Work Pattern）在 go-doudou 项目中的实现和应用，通过丰富的代码示例和最佳实践，帮助开发者掌握这一强大的数据访问模式。

## 工作单元模式概述

### 什么是工作单元模式

工作单元模式是一种企业应用架构模式，它维护一个对象列表，这些对象被某个业务事务影响，并协调写入这些更改和解决并发问题。简单来说，工作单元负责：

1. **跟踪对象状态变化**：新建、修改、删除
2. **延迟数据库操作**：在事务结束时统一提交
3. **解决操作依赖**：按正确顺序执行数据库操作
4. **优化性能**：批量操作、减少数据库交互

### 为什么选择工作单元模式

在传统的数据访问方式中，我们通常会遇到以下问题：

```go
// 传统方式的问题
func CreateUserWithPosts(user *User, posts []*Post) error {
    // 问题1：多次数据库交互
    if err := userRepo.Create(user); err != nil {
        return err
    }
    
    // 问题2：部分失败难以回滚
    for _, post := range posts {
        post.UserID = user.ID
        if err := postRepo.Create(post); err != nil {
            // 如何回滚之前的操作？
            return err
        }
    }
    
    // 问题3：没有优化批量操作
    // 问题4：无法处理复杂的依赖关系
    return nil
}
```

而使用工作单元模式：

```go
// 工作单元模式的优势
func CreateUserWithPostsUoW(user *User, posts []*Post) error {
    return uowManager.ExecuteInUnitOfWork(context.Background(), func(uow *unitofwork.UnitOfWork) error {
        // 注册新用户
        if err := uow.RegisterNew(user); err != nil {
            return err
        }
        
        // 注册所有文章
        for _, post := range posts {
            post.UserID = 1 // 临时ID，实际会在提交时处理
            if err := uow.RegisterNew(post); err != nil {
                return err
            }
        }
        
        // 所有操作在一个事务中自动处理
        return nil
    })
}
```

## toolkit/unitofwork 包架构分析

### 核心组件概览

```go
// 核心接口和结构
type Entity interface {
    GetID() interface{}
    SetID(id interface{})
    GetTableName() string
    IsNew() bool
}

type UnitOfWork struct {
    db                *gorm.DB
    newEntities       map[reflect.Type][]Entity
    dirtyEntities     map[reflect.Type][]Entity
    removedEntities   map[reflect.Type][]Entity
    snapshotManager   *SnapshotManager
    dependencyManager *DependencyManager
    // ...
}
```

### 实体基类设计

```go
// BaseEntity 提供了标准实体的基础功能
type BaseEntity struct {
    ID        interface{} `gorm:"primaryKey" json:"id"`
    CreatedAt time.Time   `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt time.Time   `gorm:"autoUpdateTime" json:"updated_at"`
    DeletedAt *time.Time  `gorm:"index" json:"deleted_at,omitempty"`
    Revision  int64       `gorm:"default:1" json:"revision"`
}

// 支持多种特性接口
type HasRevision interface {
    Entity
    GetRevision() int64
    SetRevision(revision int64)
    GetRevisionNext() int64
}

type SoftDelete interface {
    Entity
    GetDeletedAt() *time.Time
    SetDeletedAt(deletedAt *time.Time)
    IsDeleted() bool
}
```

### 操作类型系统

包定义了多种操作类型，支持单个和批量操作：

```go
type OperationType int

const (
    OperationTypeInsert OperationType = iota
    OperationTypeUpdate
    OperationTypeDelete
    OperationTypeBulkInsert
    OperationTypeBulkUpdate
    OperationTypeBulkDelete
)
```

## 在 go-doudou 项目中集成 unitofwork

### 项目结构设置

首先，让我们在 go-doudou 项目中设置合适的项目结构：

```
go-doudou-project/
├── internal/
│   ├── entity/          # 实体定义
│   ├── repository/      # 数据访问层
│   ├── service/         # 业务逻辑层
│   └── handler/         # HTTP处理层
├── pkg/
│   └── config/          # 配置
└── go.mod
```

### 1. 定义业务实体

```go
// internal/entity/user.go
package entity

import (
    "fmt"
    "github.com/unionj-cloud/toolkit/unitofwork"
)

type User struct {
    unitofwork.BaseEntity
    Name     string `gorm:"size:100;not null" json:"name"`
    Email    string `gorm:"size:255;uniqueIndex" json:"email"`
    Age      int    `json:"age"`
    Posts    []Post `gorm:"foreignKey:UserID" json:"posts,omitempty"`
    Profile  *UserProfile `gorm:"foreignKey:UserID" json:"profile,omitempty"`
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

// internal/entity/post.go
type Post struct {
    unitofwork.BaseEntity
    Title    string `gorm:"size:255;not null" json:"title"`
    Content  string `gorm:"type:text" json:"content"`
    UserID   uint   `gorm:"not null" json:"user_id"`
    User     *User  `gorm:"foreignKey:UserID" json:"user,omitempty"`
    Tags     []Tag  `gorm:"many2many:post_tags;" json:"tags,omitempty"`
    Status   PostStatus `gorm:"default:0" json:"status"`
}

func (p *Post) GetTableName() string {
    return "posts"
}

type PostStatus int

const (
    PostStatusDraft PostStatus = iota
    PostStatusPublished
    PostStatusArchived
)

// internal/entity/user_profile.go
type UserProfile struct {
    unitofwork.BaseEntity
    UserID   uint   `gorm:"not null;uniqueIndex" json:"user_id"`
    Avatar   string `gorm:"size:500" json:"avatar"`
    Bio      string `gorm:"type:text" json:"bio"`
    Website  string `gorm:"size:255" json:"website"`
    Location string `gorm:"size:100" json:"location"`
}

func (up *UserProfile) GetTableName() string {
    return "user_profiles"
}
```

### 2. 配置数据库和工作单元管理器

```go
// pkg/config/database.go
package config

import (
    "github.com/unionj-cloud/toolkit/unitofwork"
    "github.com/wubin1989/gorm"
    "github.com/wubin1989/sqlite"
)

type DatabaseConfig struct {
    UoWManager *unitofwork.Manager
    DB         *gorm.DB
}

func NewDatabaseConfig() (*DatabaseConfig, error) {
    // 初始化数据库连接
    db, err := gorm.Open(sqlite.Open("app.db"), &gorm.Config{})
    if err != nil {
        return nil, err
    }

    // 自动迁移
    err = db.AutoMigrate(
        &entity.User{},
        &entity.Post{},
        &entity.UserProfile{},
        &entity.Tag{},
    )
    if err != nil {
        return nil, err
    }

    // 创建工作单元管理器
    uowManager := unitofwork.NewManager(db,
        unitofwork.WithBatchSize(1000),
        unitofwork.WithDirtyCheck(true),
        unitofwork.WithOperationMerge(true),
        unitofwork.WithDetailLog(true),
    )

    // 配置实体依赖关系
    depManager := uowManager.GetCurrentUoW().GetDependencyManager()
    setupEntityDependencies(depManager)

    return &DatabaseConfig{
        UoWManager: uowManager,
        DB:         db,
    }, nil
}

func setupEntityDependencies(dm *unitofwork.DependencyManager) {
    // Post 依赖于 User（外键关系）
    dm.RegisterDependency(reflect.TypeOf(&entity.Post{}), reflect.TypeOf(&entity.User{}))
    
    // UserProfile 依赖于 User
    dm.RegisterDependency(reflect.TypeOf(&entity.UserProfile{}), reflect.TypeOf(&entity.User{}))
    
    // 设置实体权重（影响同级实体的创建顺序）
    dm.RegisterEntityWeight(reflect.TypeOf(&entity.User{}), 1)
    dm.RegisterEntityWeight(reflect.TypeOf(&entity.Post{}), 2)
    dm.RegisterEntityWeight(reflect.TypeOf(&entity.UserProfile{}), 3)
}
```

### 3. 实现服务层

```go
// internal/service/user_service.go
package service

import (
    "context"
    "fmt"
    
    "github.com/unionj-cloud/toolkit/unitofwork"
    "your-project/internal/entity"
    "your-project/pkg/config"
)

type UserService struct {
    uowManager *unitofwork.Manager
    db         *gorm.DB
}

func NewUserService(dbConfig *config.DatabaseConfig) *UserService {
    return &UserService{
        uowManager: dbConfig.UoWManager,
        db:         dbConfig.DB,
    }
}

// CreateUserWithProfile 创建用户和档案
func (s *UserService) CreateUserWithProfile(ctx context.Context, user *entity.User, profile *entity.UserProfile) error {
    return s.uowManager.ExecuteInUnitOfWork(ctx, func(uow *unitofwork.UnitOfWork) error {
        // 注册新用户
        if err := uow.RegisterNew(user); err != nil {
            return fmt.Errorf("注册用户失败: %w", err)
        }

        // 设置档案的用户关联
        profile.UserID = 1 // 临时值，提交时会更新为实际ID
        if err := uow.RegisterNew(profile); err != nil {
            return fmt.Errorf("注册用户档案失败: %w", err)
        }

        return nil
    })
}

// UpdateUserProfile 更新用户档案
func (s *UserService) UpdateUserProfile(ctx context.Context, userID uint, updates map[string]interface{}) error {
    return s.uowManager.ExecuteInUnitOfWork(ctx, func(uow *unitofwork.UnitOfWork) error {
        // 加载现有档案
        var profile entity.UserProfile
        if err := s.db.Where("user_id = ?", userID).First(&profile).Error; err != nil {
            return fmt.Errorf("用户档案不存在: %w", err)
        }

        // 注册为干净实体（用于脏检查）
        uow.RegisterClean(&profile)

        // 应用更新
        if bio, ok := updates["bio"]; ok {
            profile.Bio = bio.(string)
        }
        if website, ok := updates["website"]; ok {
            profile.Website = website.(string)
        }
        if location, ok := updates["location"]; ok {
            profile.Location = location.(string)
        }

        // 工作单元会自动检测变更并注册为脏实体
        return nil
    })
}

// BatchCreateUsers 批量创建用户
func (s *UserService) BatchCreateUsers(ctx context.Context, users []*entity.User) error {
    return s.uowManager.ExecuteInUnitOfWork(ctx, func(uow *unitofwork.UnitOfWork) error {
        for _, user := range users {
            if err := uow.RegisterNew(user); err != nil {
                return fmt.Errorf("注册用户 %s 失败: %w", user.Name, err)
            }
        }
        return nil
    })
}

// TransferUserPosts 转移用户的所有文章到另一个用户
func (s *UserService) TransferUserPosts(ctx context.Context, fromUserID, toUserID uint) error {
    return s.uowManager.ExecuteInUnitOfWork(ctx, func(uow *unitofwork.UnitOfWork) error {
        // 查询要转移的文章
        var posts []entity.Post
        if err := s.db.Where("user_id = ?", fromUserID).Find(&posts).Error; err != nil {
            return fmt.Errorf("查询用户文章失败: %w", err)
        }

        // 批量更新文章的所有者
        for i := range posts {
            // 注册为干净实体
            uow.RegisterClean(&posts[i])
            
            // 修改所有者
            posts[i].UserID = toUserID
            
            // 工作单元会自动检测变更
        }

        return nil
    })
}
```

### 4. 博客服务示例

```go
// internal/service/blog_service.go
package service

type BlogService struct {
    uowManager *unitofwork.Manager
    db         *gorm.DB
}

func NewBlogService(dbConfig *config.DatabaseConfig) *BlogService {
    return &BlogService{
        uowManager: dbConfig.UoWManager,
        db:         dbConfig.DB,
    }
}

// PublishPost 发布文章（复杂业务逻辑示例）
func (s *BlogService) PublishPost(ctx context.Context, authorID uint, postData *entity.Post, tags []string) error {
    return s.uowManager.ExecuteInUnitOfWork(ctx, func(uow *unitofwork.UnitOfWork) error {
        // 1. 验证作者存在
        var author entity.User
        if err := s.db.First(&author, authorID).Error; err != nil {
            return fmt.Errorf("作者不存在: %w", err)
        }

        // 2. 创建文章
        postData.UserID = authorID
        postData.Status = entity.PostStatusPublished
        if err := uow.RegisterNew(postData); err != nil {
            return fmt.Errorf("注册文章失败: %w", err)
        }

        // 3. 处理标签
        for _, tagName := range tags {
            var tag entity.Tag
            
            // 尝试查找现有标签
            err := s.db.Where("name = ?", tagName).First(&tag).Error
            if err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                    // 创建新标签
                    tag = entity.Tag{
                        Name:  tagName,
                        Color: generateRandomColor(),
                    }
                    if err := uow.RegisterNew(&tag); err != nil {
                        return fmt.Errorf("注册标签失败: %w", err)
                    }
                } else {
                    return fmt.Errorf("查询标签失败: %w", err)
                }
            }
        }

        // 4. 更新作者统计（示例）
        uow.RegisterClean(&author)
        // author.PostCount++ // 假设有这个字段

        return nil
    })
}

// ArchiveOldPosts 归档旧文章
func (s *BlogService) ArchiveOldPosts(ctx context.Context, beforeDate time.Time) error {
    return s.uowManager.ExecuteInUnitOfWork(ctx, func(uow *unitofwork.UnitOfWork) error {
        // 分页处理大量数据
        pageSize := 100
        offset := 0

        for {
            var posts []entity.Post
            
            result := s.db.Where("created_at < ? AND status = ?", beforeDate, entity.PostStatusPublished).
                Offset(offset).
                Limit(pageSize).
                Find(&posts)
                
            if result.Error != nil {
                return fmt.Errorf("查询文章失败: %w", result.Error)
            }

            if len(posts) == 0 {
                break // 没有更多数据
            }

            // 批量标记为归档
            for i := range posts {
                uow.RegisterClean(&posts[i])
                posts[i].Status = entity.PostStatusArchived
            }

            offset += pageSize

            // 避免单个事务过大，可以考虑分批提交
            if offset%1000 == 0 {
                // 可以在这里记录进度
                fmt.Printf("已处理 %d 篇文章\n", offset)
            }
        }

        return nil
    })
}
```

### 5. HTTP 处理层集成

```go
// internal/handler/user_handler.go
package handler

import (
    "net/http"
    "strconv"
    
    "github.com/gin-gonic/gin"
    "your-project/internal/entity"
    "your-project/internal/service"
)

type UserHandler struct {
    userService *service.UserService
    blogService *service.BlogService
}

func NewUserHandler(userService *service.UserService, blogService *service.BlogService) *UserHandler {
    return &UserHandler{
        userService: userService,
        blogService: blogService,
    }
}

// CreateUser 创建用户API
func (h *UserHandler) CreateUser(c *gin.Context) {
    var req struct {
        Name     string `json:"name" binding:"required"`
        Email    string `json:"email" binding:"required,email"`
        Age      int    `json:"age" binding:"min=1"`
        Profile  *struct {
            Bio      string `json:"bio"`
            Website  string `json:"website"`
            Location string `json:"location"`
        } `json:"profile,omitempty"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    user := &entity.User{
        Name:  req.Name,
        Email: req.Email,
        Age:   req.Age,
    }

    var profile *entity.UserProfile
    if req.Profile != nil {
        profile = &entity.UserProfile{
            Bio:      req.Profile.Bio,
            Website:  req.Profile.Website,
            Location: req.Profile.Location,
        }
    }

    ctx := c.Request.Context()
    
    if profile != nil {
        err := h.userService.CreateUserWithProfile(ctx, user, profile)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
    } else {
        err := h.userService.ExecuteInUnitOfWork(ctx, func(uow *unitofwork.UnitOfWork) error {
            return uow.RegisterNew(user)
        })
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
    }

    c.JSON(http.StatusCreated, gin.H{
        "message": "用户创建成功",
        "user_id": user.GetID(),
    })
}

// UpdateUserProfile 更新用户档案API
func (h *UserHandler) UpdateUserProfile(c *gin.Context) {
    userIDStr := c.Param("user_id")
    userID, err := strconv.ParseUint(userIDStr, 10, 32)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "无效的用户ID"})
        return
    }

    var updates map[string]interface{}
    if err := c.ShouldBindJSON(&updates); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    ctx := c.Request.Context()
    err = h.userService.UpdateUserProfile(ctx, uint(userID), updates)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "档案更新成功"})
}

// PublishPost 发布文章API
func (h *UserHandler) PublishPost(c *gin.Context) {
    var req struct {
        Title   string   `json:"title" binding:"required"`
        Content string   `json:"content" binding:"required"`
        AuthorID uint    `json:"author_id" binding:"required"`
        Tags    []string `json:"tags"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    post := &entity.Post{
        Title:   req.Title,
        Content: req.Content,
    }

    ctx := c.Request.Context()
    err := h.blogService.PublishPost(ctx, req.AuthorID, post, req.Tags)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusCreated, gin.H{
        "message": "文章发布成功",
        "post_id": post.GetID(),
    })
}
```

## 高级特性应用

### 1. 乐观锁处理

```go
// 处理并发更新的安全示例
func (s *UserService) SafeUpdateUser(ctx context.Context, userID uint, updates map[string]interface{}) error {
    maxRetries := 3
    
    for i := 0; i < maxRetries; i++ {
        err := s.uowManager.ExecuteInUnitOfWork(ctx, func(uow *unitofwork.UnitOfWork) error {
            var user entity.User
            if err := s.db.First(&user, userID).Error; err != nil {
                return err
            }

            // 注册为干净实体，启用乐观锁检查
            uow.RegisterClean(&user)

            // 应用更新
            if name, ok := updates["name"]; ok {
                user.Name = name.(string)
            }
            if age, ok := updates["age"]; ok {
                user.Age = age.(int)
            }

            return nil
        })

        if err == nil {
            return nil // 成功
        }

        // 检查是否是乐观锁冲突
        if isOptimisticLockError(err) && i < maxRetries-1 {
            // 短暂延迟后重试
            time.Sleep(time.Millisecond * time.Duration(100*(i+1)))
            continue
        }

        return err
    }

    return fmt.Errorf("更新失败，超过最大重试次数")
}

func isOptimisticLockError(err error) bool {
    return strings.Contains(err.Error(), "optimistic lock")
}
```

### 2. 软删除处理

```go
// 软删除用户及其相关数据
func (s *UserService) SoftDeleteUser(ctx context.Context, userID uint) error {
    return s.uowManager.ExecuteInUnitOfWork(ctx, func(uow *unitofwork.UnitOfWork) error {
        // 1. 软删除用户
        var user entity.User
        if err := s.db.First(&user, userID).Error; err != nil {
            return err
        }
        
        uow.RegisterClean(&user)
        now := time.Now()
        user.SetDeletedAt(&now)

        // 2. 软删除用户的所有文章
        var posts []entity.Post
        if err := s.db.Where("user_id = ?", userID).Find(&posts).Error; err != nil {
            return err
        }

        for i := range posts {
            uow.RegisterClean(&posts[i])
            posts[i].SetDeletedAt(&now)
        }

        // 3. 软删除用户档案
        var profile entity.UserProfile
        if err := s.db.Where("user_id = ?", userID).First(&profile).Error; err == nil {
            uow.RegisterClean(&profile)
            profile.SetDeletedAt(&now)
        }

        return nil
    })
}
```

### 3. 复杂查询集成

```go
// 复杂查询与工作单元结合
func (s *BlogService) OptimizePopularPosts(ctx context.Context) error {
    return s.uowManager.ExecuteInUnitOfWork(ctx, func(uow *unitofwork.UnitOfWork) error {
        // 查询热门文章（假设有浏览量字段）
        var posts []entity.Post
        err := s.db.Where("view_count > ? AND status = ?", 1000, entity.PostStatusPublished).
            Order("view_count DESC").
            Limit(100).
            Find(&posts).Error
            
        if err != nil {
            return err
        }

        // 批量优化处理
        for i := range posts {
            uow.RegisterClean(&posts[i])
            
            // 添加优化标记（假设字段）
            // posts[i].IsOptimized = true
            
            // 更新SEO信息
            if posts[i].Title != "" {
                // posts[i].SEOTitle = generateSEOTitle(posts[i].Title)
            }
        }

        return nil
    })
}
```

### 4. 分页处理大数据集

```go
// 分页处理大量数据的迁移任务
func (s *UserService) MigrateUserData(ctx context.Context, migrationFunc func(*entity.User) error) error {
    pageSize := 1000
    offset := 0
    
    for {
        err := s.uowManager.ExecuteInUnitOfWork(ctx, func(uow *unitofwork.UnitOfWork) error {
            var users []entity.User
            
            result := s.db.Offset(offset).Limit(pageSize).Find(&users)
            if result.Error != nil {
                return result.Error
            }
            
            if len(users) == 0 {
                return nil // 没有更多数据
            }
            
            // 处理当前批次
            for i := range users {
                uow.RegisterClean(&users[i])
                
                if err := migrationFunc(&users[i]); err != nil {
                    return fmt.Errorf("迁移用户 %d 失败: %w", users[i].GetID(), err)
                }
            }
            
            return nil
        })
        
        if err != nil {
            return err
        }
        
        if offset == 0 { // 第一批为空，说明没有数据
            break
        }
        
        offset += pageSize
        
        // 添加进度日志
        if offset%10000 == 0 {
            fmt.Printf("已处理 %d 个用户\n", offset)
        }
    }
    
    return nil
}
```

## 性能优化最佳实践

### 1. 批量操作优化

```go
// 优化的批量插入
func (s *UserService) BulkImportUsers(ctx context.Context, users []*entity.User) error {
    batchSize := 1000
    
    for i := 0; i < len(users); i += batchSize {
        end := i + batchSize
        if end > len(users) {
            end = len(users)
        }
        
        batch := users[i:end]
        
        err := s.uowManager.ExecuteInUnitOfWork(ctx, func(uow *unitofwork.UnitOfWork) error {
            for _, user := range batch {
                if err := uow.RegisterNew(user); err != nil {
                    return err
                }
            }
            return nil
        })
        
        if err != nil {
            return fmt.Errorf("批次 %d-%d 处理失败: %w", i, end-1, err)
        }
    }
    
    return nil
}
```

### 2. 内存使用优化

```go
// 内存友好的大数据处理
func (s *BlogService) ProcessLargeDataset(ctx context.Context) error {
    return s.uowManager.ExecuteInUnitOfWork(ctx, func(uow *unitofwork.UnitOfWork) error {
        // 使用流式处理避免内存溢出
        rows, err := s.db.Model(&entity.Post{}).Where("status = ?", entity.PostStatusDraft).Rows()
        if err != nil {
            return err
        }
        defer rows.Close()
        
        count := 0
        for rows.Next() {
            var post entity.Post
            if err := s.db.ScanRows(rows, &post); err != nil {
                return err
            }
            
            uow.RegisterClean(&post)
            post.Status = entity.PostStatusPublished
            
            count++
            
            // 每处理1000个实体，检查内存使用
            if count%1000 == 0 {
                // 可以在这里添加内存检查逻辑
                if uow.GetStats()["total_entities"].(int) > 5000 {
                    // 如果实体太多，可以考虑分批提交
                    break
                }
            }
        }
        
        return nil
    })
}
```

### 3. 缓存集成

```go
// 结合缓存的查询优化
type CachedUserService struct {
    *UserService
    cache map[uint]*entity.User // 简化的缓存示例
    mu    sync.RWMutex
}

func (s *CachedUserService) GetUserWithCache(ctx context.Context, userID uint) (*entity.User, error) {
    // 先检查缓存
    s.mu.RLock()
    if cachedUser, exists := s.cache[userID]; exists {
        s.mu.RUnlock()
        return cachedUser, nil
    }
    s.mu.RUnlock()
    
    // 缓存未命中，查询数据库
    var user entity.User
    if err := s.db.First(&user, userID).Error; err != nil {
        return nil, err
    }
    
    // 更新缓存
    s.mu.Lock()
    s.cache[userID] = &user
    s.mu.Unlock()
    
    return &user, nil
}

func (s *CachedUserService) UpdateUserWithCache(ctx context.Context, userID uint, updates map[string]interface{}) error {
    err := s.UserService.UpdateUserProfile(ctx, userID, updates)
    if err != nil {
        return err
    }
    
    // 清除缓存
    s.mu.Lock()
    delete(s.cache, userID)
    s.mu.Unlock()
    
    return nil
}
```

## 测试策略

### 1. 单元测试

```go
// internal/service/user_service_test.go
package service

import (
    "context"
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "your-project/internal/entity"
    "your-project/pkg/config"
)

func TestUserService_CreateUserWithProfile(t *testing.T) {
    // 设置测试数据库
    dbConfig, err := config.NewTestDatabaseConfig()
    require.NoError(t, err)
    
    service := NewUserService(dbConfig)
    
    t.Run("成功创建用户和档案", func(t *testing.T) {
        user := &entity.User{
            Name:  "测试用户",
            Email: "test@example.com",
            Age:   25,
        }
        
        profile := &entity.UserProfile{
            Bio:      "测试简介",
            Website:  "https://test.com",
            Location: "测试城市",
        }
        
        ctx := context.Background()
        err := service.CreateUserWithProfile(ctx, user, profile)
        
        require.NoError(t, err)
        assert.NotNil(t, user.GetID())
        assert.NotNil(t, profile.GetID())
        
        // 验证数据库中的数据
        var dbUser entity.User
        err = dbConfig.DB.First(&dbUser, user.GetID()).Error
        require.NoError(t, err)
        assert.Equal(t, "测试用户", dbUser.Name)
        
        var dbProfile entity.UserProfile
        err = dbConfig.DB.Where("user_id = ?", user.GetID()).First(&dbProfile).Error
        require.NoError(t, err)
        assert.Equal(t, "测试简介", dbProfile.Bio)
    })
    
    t.Run("验证失败回滚", func(t *testing.T) {
        user := &entity.User{
            Name:  "", // 空名称会导致验证失败
            Email: "invalid@example.com",
            Age:   25,
        }
        
        profile := &entity.UserProfile{
            Bio: "测试简介",
        }
        
        ctx := context.Background()
        err := service.CreateUserWithProfile(ctx, user, profile)
        
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "用户名不能为空")
        
        // 验证数据库中没有数据
        var count int64
        dbConfig.DB.Model(&entity.User{}).Where("email = ?", "invalid@example.com").Count(&count)
        assert.Equal(t, int64(0), count)
    })
}
```

### 2. 集成测试

```go
func TestUserService_Integration(t *testing.T) {
    dbConfig, err := config.NewTestDatabaseConfig()
    require.NoError(t, err)
    
    service := NewUserService(dbConfig)
    
    t.Run("完整业务流程测试", func(t *testing.T) {
        ctx := context.Background()
        
        // 1. 创建用户
        user := &entity.User{
            Name:  "集成测试用户",
            Email: "integration@test.com",
            Age:   30,
        }
        
        profile := &entity.UserProfile{
            Bio:      "集成测试简介",
            Website:  "https://integration.test",
            Location: "测试城市",
        }
        
        err := service.CreateUserWithProfile(ctx, user, profile)
        require.NoError(t, err)
        
        // 2. 更新档案
        updates := map[string]interface{}{
            "bio":      "更新后的简介",
            "location": "新城市",
        }
        
        err = service.UpdateUserProfile(ctx, uint(user.GetID().(int)), updates)
        require.NoError(t, err)
        
        // 3. 验证更新结果
        var updatedProfile entity.UserProfile
        err = dbConfig.DB.Where("user_id = ?", user.GetID()).First(&updatedProfile).Error
        require.NoError(t, err)
        assert.Equal(t, "更新后的简介", updatedProfile.Bio)
        assert.Equal(t, "新城市", updatedProfile.Location)
        assert.Equal(t, "https://integration.test", updatedProfile.Website) // 未更新的字段保持不变
        
        // 4. 软删除
        err = service.SoftDeleteUser(ctx, uint(user.GetID().(int)))
        require.NoError(t, err)
        
        // 5. 验证软删除结果
        var deletedUser entity.User
        err = dbConfig.DB.Unscoped().First(&deletedUser, user.GetID()).Error
        require.NoError(t, err)
        assert.True(t, deletedUser.IsDeleted())
    })
}
```

### 3. 性能测试

```go
func BenchmarkUserService_BatchCreate(b *testing.B) {
    dbConfig, err := config.NewTestDatabaseConfig()
    require.NoError(b, err)
    
    service := NewUserService(dbConfig)
    
    b.Run("批量创建1000用户", func(b *testing.B) {
        for i := 0; i < b.N; i++ {
            users := make([]*entity.User, 1000)
            for j := 0; j < 1000; j++ {
                users[j] = &entity.User{
                    Name:  fmt.Sprintf("性能测试用户%d_%d", i, j),
                    Email: fmt.Sprintf("perf%d_%d@test.com", i, j),
                    Age:   20 + j%50,
                }
            }
            
            ctx := context.Background()
            err := service.BatchCreateUsers(ctx, users)
            if err != nil {
                b.Fatal(err)
            }
        }
    })
}
```

## 监控和日志

### 1. 性能监控

```go
// pkg/middleware/unitofwork_monitor.go
package middleware

import (
    "context"
    "time"
    
    "github.com/gin-gonic/gin"
    "github.com/unionj-cloud/toolkit/unitofwork"
    "github.com/unionj-cloud/toolkit/zlogger"
)

// UnitOfWorkMonitor 工作单元监控中间件
func UnitOfWorkMonitor() gin.HandlerFunc {
    return func(c *gin.Context) {
        startTime := time.Now()
        
        // 在请求上下文中添加监控信息
        ctx := context.WithValue(c.Request.Context(), "request_start_time", startTime)
        ctx = context.WithValue(ctx, "request_id", generateRequestID())
        
        c.Request = c.Request.WithContext(ctx)
        
        // 处理请求
        c.Next()
        
        duration := time.Since(startTime)
        
        // 记录性能指标
        zlogger.Info().
            Str("method", c.Request.Method).
            Str("path", c.Request.URL.Path).
            Int("status", c.Writer.Status()).
            Dur("duration", duration).
            Msg("Request completed")
    }
}

// UnitOfWorkStatsLogger 工作单元统计日志
func LogUnitOfWorkStats(uow *unitofwork.UnitOfWork, operation string) {
    stats := uow.GetStats()
    
    zlogger.Info().
        Str("operation", operation).
        Int("new_entities", stats["new_entities"].(int)).
        Int("dirty_entities", stats["dirty_entities"].(int)).
        Int("removed_entities", stats["removed_entities"].(int)).
        Int("total_operations", stats["total_operations"].(int)).
        Bool("is_committed", stats["is_committed"].(bool)).
        Msg("UnitOfWork statistics")
}
```

### 2. 错误处理和重试

```go
// pkg/utils/retry.go
package utils

import (
    "context"
    "time"
    
    "github.com/unionj-cloud/toolkit/unitofwork"
)

type RetryConfig struct {
    MaxRetries int
    BaseDelay  time.Duration
    MaxDelay   time.Duration
}

// ExecuteWithRetry 带重试的工作单元执行
func ExecuteWithRetry(
    ctx context.Context,
    uowManager *unitofwork.Manager,
    config RetryConfig,
    operation func(*unitofwork.UnitOfWork) error,
) error {
    var lastErr error
    
    for attempt := 0; attempt <= config.MaxRetries; attempt++ {
        err := uowManager.ExecuteInUnitOfWork(ctx, operation)
        if err == nil {
            return nil
        }
        
        lastErr = err
        
        // 检查是否应该重试
        if !shouldRetry(err) {
            break
        }
        
        if attempt < config.MaxRetries {
            delay := calculateDelay(attempt, config)
            
            zlogger.Warn().
                Err(err).
                Int("attempt", attempt+1).
                Int("max_retries", config.MaxRetries).
                Dur("delay", delay).
                Msg("UnitOfWork operation failed, retrying")
                
            time.Sleep(delay)
        }
    }
    
    return fmt.Errorf("operation failed after %d attempts: %w", config.MaxRetries+1, lastErr)
}

func shouldRetry(err error) bool {
    // 定义哪些错误应该重试
    errorMessages := []string{
        "database is locked",
        "connection reset",
        "timeout",
        "optimistic lock",
    }
    
    errStr := err.Error()
    for _, msg := range errorMessages {
        if strings.Contains(errStr, msg) {
            return true
        }
    }
    
    return false
}

func calculateDelay(attempt int, config RetryConfig) time.Duration {
    delay := config.BaseDelay * time.Duration(1<<uint(attempt)) // 指数退避
    if delay > config.MaxDelay {
        delay = config.MaxDelay
    }
    return delay
}
```

## 部署和配置

### 1. 生产环境配置

```go
// pkg/config/production.go
package config

import (
    "os"
    "strconv"
    "time"
    
    "github.com/unionj-cloud/toolkit/unitofwork"
)

func NewProductionDatabaseConfig() (*DatabaseConfig, error) {
    // 从环境变量获取配置
    batchSize, _ := strconv.Atoi(getEnvOrDefault("UOW_BATCH_SIZE", "1000"))
    maxEntityCount, _ := strconv.Atoi(getEnvOrDefault("UOW_MAX_ENTITY_COUNT", "10000"))
    enableDirtyCheck, _ := strconv.ParseBool(getEnvOrDefault("UOW_ENABLE_DIRTY_CHECK", "true"))
    enableOperationMerge, _ := strconv.ParseBool(getEnvOrDefault("UOW_ENABLE_OPERATION_MERGE", "true"))
    enableDetailLog, _ := strconv.ParseBool(getEnvOrDefault("UOW_ENABLE_DETAIL_LOG", "false"))
    
    // 数据库连接配置
    dsn := os.Getenv("DATABASE_URL")
    if dsn == "" {
        return nil, fmt.Errorf("DATABASE_URL environment variable is required")
    }
    
    db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
        // 生产环境优化配置
        PrepareStmt:              true,
        DisableForeignKeyConstraintWhenMigrating: false,
        Logger: logger.New(
            log.New(os.Stdout, "\r\n", log.LstdFlags),
            logger.Config{
                SlowThreshold:             time.Second,
                LogLevel:                  logger.Warn,
                IgnoreRecordNotFoundError: true,
                Colorful:                  false,
            },
        ),
    })
    
    if err != nil {
        return nil, fmt.Errorf("failed to connect to database: %w", err)
    }
    
    // 连接池配置
    sqlDB, err := db.DB()
    if err != nil {
        return nil, err
    }
    
    sqlDB.SetMaxIdleConns(10)
    sqlDB.SetMaxOpenConns(100)
    sqlDB.SetConnMaxLifetime(time.Hour)
    
    // 创建工作单元管理器
    uowManager := unitofwork.NewManager(db,
        unitofwork.WithBatchSize(batchSize),
        unitofwork.WithDirtyCheck(enableDirtyCheck),
        unitofwork.WithOperationMerge(enableOperationMerge),
        unitofwork.WithMaxEntityCount(maxEntityCount),
        unitofwork.WithDetailLog(enableDetailLog),
    )
    
    return &DatabaseConfig{
        UoWManager: uowManager,
        DB:         db,
    }, nil
}

func getEnvOrDefault(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}
```

### 2. Docker 配置

```dockerfile
# Dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/server

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/main .

# 环境变量
ENV UOW_BATCH_SIZE=1000
ENV UOW_MAX_ENTITY_COUNT=10000
ENV UOW_ENABLE_DIRTY_CHECK=true
ENV UOW_ENABLE_OPERATION_MERGE=true
ENV UOW_ENABLE_DETAIL_LOG=false

CMD ["./main"]
```

### 3. Kubernetes 配置

```yaml
# k8s/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: go-doudou-app
spec:
  replicas: 3
  selector:
    matchLabels:
      app: go-doudou-app
  template:
    metadata:
      labels:
        app: go-doudou-app
    spec:
      containers:
      - name: app
        image: your-registry/go-doudou-app:latest
        ports:
        - containerPort: 8080
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: db-secret
              key: url
        - name: UOW_BATCH_SIZE
          value: "500"
        - name: UOW_MAX_ENTITY_COUNT
          value: "5000"
        - name: UOW_ENABLE_DETAIL_LOG
          value: "true"
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
```

## 总结

工作单元模式在 go-doudou 项目中的应用为我们提供了一个强大的数据访问层解决方案。通过本文的深入探讨，我们了解了：

### 核心优势

1. **事务一致性**：确保复杂业务操作的原子性
2. **性能优化**：批量操作、减少数据库交互
3. **并发安全**：乐观锁支持、版本控制
4. **开发效率**：简化复杂的数据操作逻辑
5. **可维护性**：清晰的关注点分离

### 应用场景

1. **复杂业务逻辑**：涉及多个实体的操作
2. **批量数据处理**：大量数据的导入导出
3. **数据迁移**：安全的数据转换和迁移
4. **并发控制**：需要严格一致性的场景
5. **性能敏感**：需要优化数据库访问的应用

### 最佳实践总结

1. **合理设计实体**：充分利用接口特性
2. **控制事务粒度**：避免过大或过小的事务
3. **监控性能指标**：及时发现和解决性能问题
4. **错误处理**：完善的重试和回滚机制
5. **测试覆盖**：全面的单元和集成测试

### 未来展望

随着 go-doudou 生态的不断发展，工作单元模式的实现也会持续演进：

1. **分布式事务支持**：跨服务的事务协调
2. **更多数据库支持**：NoSQL 数据库的集成
3. **智能优化**：基于机器学习的性能优化
4. **云原生特性**：更好的容器化和微服务支持

通过掌握工作单元模式，开发者可以构建更加健壮、高效的 go-doudou 应用，为用户提供更好的服务体验。这个模式不仅仅是一个技术工具，更是一种优秀的软件设计思想，值得我们深入学习和实践。 