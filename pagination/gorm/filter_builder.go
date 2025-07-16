package gorm

import (
	"github.com/bytedance/sonic"
)

// FilterBuilder 提供链式调用的方式构建过滤器参数
type FilterBuilder struct {
	filters []interface{}
}

// NewFilter 创建一个新的过滤器构建器
func NewFilter() *FilterBuilder {
	return &FilterBuilder{
		filters: make([]interface{}, 0),
	}
}

// Equal 添加等于条件 (column = value)
func (f *FilterBuilder) Equal(column string, value interface{}) *FilterBuilder {
	f.filters = append(f.filters, []interface{}{column, "=", value})
	return f
}

// NotEqual 添加不等于条件 (column != value)
func (f *FilterBuilder) NotEqual(column string, value interface{}) *FilterBuilder {
	f.filters = append(f.filters, []interface{}{column, "!=", value})
	return f
}

// Like 添加LIKE条件 (column LIKE %value%)
func (f *FilterBuilder) Like(column string, value string) *FilterBuilder {
	f.filters = append(f.filters, []interface{}{column, "like", value})
	return f
}

// NotLike 添加NOT LIKE条件 (column NOT LIKE %value%)
func (f *FilterBuilder) NotLike(column string, value string) *FilterBuilder {
	f.filters = append(f.filters, []interface{}{column, "not like", value})
	return f
}

// ILike 添加ILIKE条件 (column ILIKE %value%, 大小写不敏感)
func (f *FilterBuilder) ILike(column string, value string) *FilterBuilder {
	f.filters = append(f.filters, []interface{}{column, "ilike", value})
	return f
}

// NotILike 添加NOT ILIKE条件 (column NOT ILIKE %value%, 大小写不敏感)
func (f *FilterBuilder) NotILike(column string, value string) *FilterBuilder {
	f.filters = append(f.filters, []interface{}{column, "not ilike", value})
	return f
}

// GreaterThan 添加大于条件 (column > value)
func (f *FilterBuilder) GreaterThan(column string, value interface{}) *FilterBuilder {
	f.filters = append(f.filters, []interface{}{column, ">", value})
	return f
}

// GreaterThanOrEqual 添加大于等于条件 (column >= value)
func (f *FilterBuilder) GreaterThanOrEqual(column string, value interface{}) *FilterBuilder {
	f.filters = append(f.filters, []interface{}{column, ">=", value})
	return f
}

// LessThan 添加小于条件 (column < value)
func (f *FilterBuilder) LessThan(column string, value interface{}) *FilterBuilder {
	f.filters = append(f.filters, []interface{}{column, "<", value})
	return f
}

// LessThanOrEqual 添加小于等于条件 (column <= value)
func (f *FilterBuilder) LessThanOrEqual(column string, value interface{}) *FilterBuilder {
	f.filters = append(f.filters, []interface{}{column, "<=", value})
	return f
}

// Between 添加BETWEEN条件 (column BETWEEN min AND max)
func (f *FilterBuilder) Between(column string, min, max interface{}) *FilterBuilder {
	f.filters = append(f.filters, []interface{}{column, "between", []interface{}{min, max}})
	return f
}

// NotBetween 添加NOT BETWEEN条件 (column NOT BETWEEN min AND max)
func (f *FilterBuilder) NotBetween(column string, min, max interface{}) *FilterBuilder {
	f.filters = append(f.filters, []interface{}{column, "not between", []interface{}{min, max}})
	return f
}

// In 添加IN条件 (column IN (values...))
func (f *FilterBuilder) In(column string, values ...interface{}) *FilterBuilder {
	f.filters = append(f.filters, []interface{}{column, "in", values})
	return f
}

// NotIn 添加NOT IN条件 (column NOT IN (values...))
func (f *FilterBuilder) NotIn(column string, values ...interface{}) *FilterBuilder {
	f.filters = append(f.filters, []interface{}{column, "not in", values})
	return f
}

// IsNull 添加IS NULL条件 (column IS NULL)
func (f *FilterBuilder) IsNull(column string) *FilterBuilder {
	f.filters = append(f.filters, []interface{}{column, "is", nil})
	return f
}

// IsNotNull 添加IS NOT NULL条件 (column IS NOT NULL)
func (f *FilterBuilder) IsNotNull(column string) *FilterBuilder {
	f.filters = append(f.filters, []interface{}{column, "is not", nil})
	return f
}

// And 添加AND逻辑操作符
func (f *FilterBuilder) And() *FilterBuilder {
	f.filters = append(f.filters, []interface{}{"and"})
	return f
}

// Or 添加OR逻辑操作符
func (f *FilterBuilder) Or() *FilterBuilder {
	f.filters = append(f.filters, []interface{}{"or"})
	return f
}

// Group 添加分组条件 (将条件包装在括号中)
func (f *FilterBuilder) Group(groupBuilder func(*FilterBuilder)) *FilterBuilder {
	subBuilder := NewFilter()
	groupBuilder(subBuilder)
	f.filters = append(f.filters, subBuilder.filters)
	return f
}

// Custom 添加自定义条件
func (f *FilterBuilder) Custom(column, operator string, value interface{}) *FilterBuilder {
	f.filters = append(f.filters, []interface{}{column, operator, value})
	return f
}

// SimpleEqual 添加简单等于条件的简写形式 (column, value)
func (f *FilterBuilder) SimpleEqual(column string, value interface{}) *FilterBuilder {
	f.filters = append(f.filters, []interface{}{column, value})
	return f
}

// Build 构建最终的过滤器参数，返回可以直接用于参数的值
func (f *FilterBuilder) Build() interface{} {
	if len(f.filters) == 0 {
		return nil
	}
	if len(f.filters) == 1 {
		return f.filters[0]
	}
	return f.filters
}

// BuildJSON 构建JSON字符串格式的过滤器参数
func (f *FilterBuilder) BuildJSON() (string, error) {
	result := f.Build()
	if result == nil {
		return "", nil
	}
	
	data, err := sonic.Marshal(result)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Reset 重置过滤器构建器，清空所有条件
func (f *FilterBuilder) Reset() *FilterBuilder {
	f.filters = make([]interface{}, 0)
	return f
}

// Len 返回当前过滤器条件的数量
func (f *FilterBuilder) Len() int {
	return len(f.filters)
}

// IsEmpty 检查是否为空
func (f *FilterBuilder) IsEmpty() bool {
	return len(f.filters) == 0
}

// 便捷函数，用于快速创建常用的过滤器

// QuickFilter 快速创建单个过滤器条件
func QuickFilter(column, operator string, value interface{}) interface{} {
	return []interface{}{column, operator, value}
}

// QuickEqual 快速创建等于条件
func QuickEqual(column string, value interface{}) interface{} {
	return []interface{}{column, value}
}

// QuickLike 快速创建LIKE条件
func QuickLike(column, value string) interface{} {
	return []interface{}{column, "like", value}
}

// QuickBetween 快速创建BETWEEN条件
func QuickBetween(column string, min, max interface{}) interface{} {
	return []interface{}{column, "between", []interface{}{min, max}}
}

// QuickIn 快速创建IN条件
func QuickIn(column string, values ...interface{}) interface{} {
	return []interface{}{column, "in", values}
}

// QuickIsNull 快速创建IS NULL条件
func QuickIsNull(column string) interface{} {
	return []interface{}{column, "is", nil}
} 