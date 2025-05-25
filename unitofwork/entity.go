package unitofwork

import (
	"github.com/wubin1989/gorm"
	"time"
)

// Entity 实体接口，所有参与工作单元的实体都必须实现此接口
type Entity interface {
	// GetID 获取实体ID
	GetID() uint

	// SetID 设置实体ID
	SetID(id uint)

	// GetTableName 获取表名
	GetTableName() string

	// IsNew 判断是否为新实体
	IsNew() bool
}

// HasRevision 支持乐观锁的实体接口
type HasRevision interface {
	Entity

	// GetRevision 获取版本号
	GetRevision() int64

	// SetRevision 设置版本号
	SetRevision(revision int64)

	// GetRevisionNext 获取下一个版本号
	GetRevisionNext() int64
}

// HasTimestamps 支持时间戳的实体接口
type HasTimestamps interface {
	Entity

	// GetCreatedAt 获取创建时间
	GetCreatedAt() time.Time

	// SetCreatedAt 设置创建时间
	SetCreatedAt(createdAt time.Time)

	// GetUpdatedAt 获取更新时间
	GetUpdatedAt() time.Time

	// SetUpdatedAt 设置更新时间
	SetUpdatedAt(updatedAt time.Time)
}

// SoftDelete 支持软删除的实体接口
type SoftDelete interface {
	Entity

	// GetDeletedAt 获取删除时间
	GetDeletedAt() gorm.DeletedAt

	// SetDeletedAt 设置删除时间
	SetDeletedAt(deletedAt gorm.DeletedAt)

	// IsDeleted 判断是否已删除
	IsDeleted() bool
}

// Validatable 支持验证的实体接口
type Validatable interface {
	Entity

	// Validate 验证实体数据
	Validate() error
}

// BaseEntity 基础实体结构，可嵌入到业务实体中
type BaseEntity struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
	Revision  int64          `gorm:"default:1" json:"revision"`
}

// GetID 实现Entity接口
func (b *BaseEntity) GetID() uint {
	return b.ID
}

// SetID 实现Entity接口
func (b *BaseEntity) SetID(id uint) {
	b.ID = id
}

// GetTableName 默认实现，子类应重写
func (b *BaseEntity) GetTableName() string {
	return ""
}

// IsNew 实现Entity接口
func (b *BaseEntity) IsNew() bool {
	return b.ID == 0
}

// GetRevision 实现HasRevision接口
func (b *BaseEntity) GetRevision() int64 {
	return b.Revision
}

// SetRevision 实现HasRevision接口
func (b *BaseEntity) SetRevision(revision int64) {
	b.Revision = revision
}

// GetRevisionNext 实现HasRevision接口
func (b *BaseEntity) GetRevisionNext() int64 {
	return b.Revision + 1
}

// GetCreatedAt 实现HasTimestamps接口
func (b *BaseEntity) GetCreatedAt() time.Time {
	return b.CreatedAt
}

// SetCreatedAt 实现HasTimestamps接口
func (b *BaseEntity) SetCreatedAt(createdAt time.Time) {
	b.CreatedAt = createdAt
}

// GetUpdatedAt 实现HasTimestamps接口
func (b *BaseEntity) GetUpdatedAt() time.Time {
	return b.UpdatedAt
}

// SetUpdatedAt 实现HasTimestamps接口
func (b *BaseEntity) SetUpdatedAt(updatedAt time.Time) {
	b.UpdatedAt = updatedAt
}

// GetDeletedAt 实现SoftDelete接口
func (b *BaseEntity) GetDeletedAt() gorm.DeletedAt {
	return b.DeletedAt
}

// SetDeletedAt 实现SoftDelete接口
func (b *BaseEntity) SetDeletedAt(deletedAt gorm.DeletedAt) {
	b.DeletedAt = deletedAt
}

// IsDeleted 实现SoftDelete接口
func (b *BaseEntity) IsDeleted() bool {
	return b.DeletedAt.Valid
}

// Validate 默认验证实现
func (b *BaseEntity) Validate() error {
	return nil
}
