package gorm

import (
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/wubin1989/gorm"
	"github.com/wubin1989/postgres"
)

func TestFilterBuilder_Basic(t *testing.T) {
	// 测试基本的过滤器构建
	filter := NewFilter().
		Equal("name", "john").
		And().
		GreaterThan("age", 18).
		Build()

	expected := []interface{}{
		[]interface{}{"name", "=", "john"},
		[]interface{}{"and"},
		[]interface{}{"age", ">", 18},
	}

	fmt.Printf("Basic filter: %+v\n", filter)
	fmt.Printf("Expected: %+v\n", expected)
}

func TestFilterBuilder_ComplexConditions(t *testing.T) {
	// 测试复杂的过滤器条件
	filter := NewFilter().
		Like("name", "john").
		Or().
		Between("age", 20, 30).
		And().
		In("status", "active", "pending", "approved").
		Build()

	fmt.Printf("Complex filter: %+v\n", filter)
}

func TestFilterBuilder_GroupConditions(t *testing.T) {
	// 测试分组条件
	filter := NewFilter().
		Group(func(g *FilterBuilder) {
			g.Equal("name", "john").
				And().
				GreaterThan("age", 18)
		}).
		Or().
		Group(func(g *FilterBuilder) {
			g.Equal("status", "admin").
				And().
				IsNotNull("email")
		}).
		Build()

	fmt.Printf("Grouped filter: %+v\n", filter)
}

func TestFilterBuilder_NullConditions(t *testing.T) {
	// 测试NULL条件
	filter := NewFilter().
		IsNull("deleted_at").
		And().
		IsNotNull("email").
		Build()

	fmt.Printf("Null conditions filter: %+v\n", filter)
}

func TestFilterBuilder_JSON(t *testing.T) {
	// 测试JSON格式输出
	filter := NewFilter().
		Equal("name", "john").
		And().
		Between("age", 20, 30)

	jsonStr, err := filter.BuildJSON()
	if err != nil {
		t.Errorf("BuildJSON failed: %v", err)
	}

	fmt.Printf("JSON filter: %s\n", jsonStr)
}

func TestFilterBuilder_QuickFunctions(t *testing.T) {
	// 测试快速创建函数
	equalFilter := QuickEqual("name", "john")
	likeFilter := QuickLike("name", "john")
	betweenFilter := QuickBetween("age", 20, 30)
	inFilter := QuickIn("status", "active", "pending")
	nullFilter := QuickIsNull("deleted_at")

	fmt.Printf("Quick equal: %+v\n", equalFilter)
	fmt.Printf("Quick like: %+v\n", likeFilter)
	fmt.Printf("Quick between: %+v\n", betweenFilter)
	fmt.Printf("Quick in: %+v\n", inFilter)
	fmt.Printf("Quick null: %+v\n", nullFilter)
}

func TestFilterBuilder_WithPagination(t *testing.T) {
	// 测试与分页功能的集成
	mockDB, _, _ := sqlmock.New()
	dialector := postgres.New(postgres.Config{
		Conn:       mockDB,
		DriverName: "postgres",
	})
	db, _ := gorm.Open(dialector, &gorm.Config{})

	// 创建复杂的过滤器
	filter := NewFilter().
		Like("name", "john").
		And().
		Between("age", 20, 30).
		And().
		IsNotNull("email")

	// 构建JSON格式的过滤器字符串
	filterJSON, err := filter.BuildJSON()
	if err != nil {
		t.Errorf("BuildJSON failed: %v", err)
		return
	}

	// 创建参数对象
	param := Parameter{
		Page:    0,
		Size:    10,
		Sort:    "name",
		Order:   "asc",
		Fields:  "",
		Filters: filterJSON,
	}

	// 使用分页功能
	pg := New(&Config{
		FieldSelectorEnabled: true,
	})

	resCxt := pg.With(db).Request(param)
	statement, args := resCxt.BuildWhereClause()

	fmt.Printf("Generated SQL WHERE clause: %s\n", statement)
	fmt.Printf("SQL parameters: %+v\n", args)
}
