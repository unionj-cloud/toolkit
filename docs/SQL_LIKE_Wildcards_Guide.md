# SQL LIKE 通配符完整指南

## 概述

SQL LIKE 操作符用于在 WHERE 子句中搜索列中的指定模式。它通过通配符来实现模糊匹配功能。本文档详细介绍了各种数据库系统中的 LIKE 通配符、转义方法以及最佳实践。

## 标准 SQL 通配符

### 1. 百分号 (%)
- **功能**: 匹配任意数量的字符（包括零个字符）
- **标准**: SQL-92 标准
- **支持**: 几乎所有数据库系统

```sql
-- 匹配以 "John" 开头的名字
SELECT * FROM users WHERE name LIKE 'John%';

-- 匹配包含 "admin" 的任何字符串
SELECT * FROM users WHERE role LIKE '%admin%';

-- 匹配以 ".com" 结尾的邮箱
SELECT * FROM users WHERE email LIKE '%.com';
```

### 2. 下划线 (_)
- **功能**: 匹配任意单个字符
- **标准**: SQL-92 标准
- **支持**: 几乎所有数据库系统

```sql
-- 匹配三个字符的代码，第一个字符是 'A'
SELECT * FROM products WHERE code LIKE 'A__';

-- 匹配格式为 "J_hn" 的名字
SELECT * FROM users WHERE name LIKE 'J_hn';
-- 匹配: John, Jahn, J3hn 等
-- 不匹配: Jhn, Johann
```

### 3. ESCAPE 子句
- **功能**: 定义转义字符，用于搜索包含通配符的字面值
- **标准**: SQL-92 标准
- **支持**: MySQL, PostgreSQL, SQL Server, Oracle, SQLite 等

```sql
-- 搜索包含下划线的文件名
SELECT * FROM files WHERE filename LIKE '%test\_%' ESCAPE '\';

-- 搜索包含百分号的产品名
SELECT * FROM products WHERE name LIKE '%50\%' ESCAPE '\';

-- 使用自定义转义字符
SELECT * FROM files WHERE filename LIKE '%test!_%' ESCAPE '!';
```

## 转义字符处理

### 为什么需要转义？
当搜索的字符串本身包含通配符时，需要对这些字符进行转义：

```sql
-- 问题：搜索包含 "_" 的文件名
SELECT * FROM files WHERE filename LIKE '%test_%.txt%';
-- 这会匹配: test1.txt, testA.txt, test_.txt

-- 解决：使用转义字符
SELECT * FROM files WHERE filename LIKE '%test\_%' ESCAPE '\';
-- 这只匹配: test_.txt
```

### 各数据库的转义方法

#### MySQL
```sql
-- ✅ 默认支持反斜杠转义（不需要 ESCAPE 子句）
SELECT * FROM products WHERE name LIKE '%50\%';

-- ✅ 也可以显式指定 ESCAPE 子句
SELECT * FROM products WHERE name LIKE '%50\%' ESCAPE '\';

-- ✅ 使用自定义转义字符
SELECT * FROM products WHERE name LIKE '%50|%' ESCAPE '|';
```

#### PostgreSQL
```sql
-- ✅ 默认支持反斜杠转义（不需要 ESCAPE 子句）
SELECT * FROM files WHERE filename LIKE '%test\_%';

-- ✅ 也可以显式指定 ESCAPE 子句
SELECT * FROM files WHERE filename LIKE '%test\_%' ESCAPE '\';

-- ✅ 使用自定义转义字符
SELECT * FROM files WHERE filename LIKE '%test!_%' ESCAPE '!';
```

#### SQL Server
```sql
-- ✅ 使用方括号转义（SQL Server 特有，不需要 ESCAPE）
SELECT * FROM files WHERE filename LIKE '%[_]%';
SELECT * FROM files WHERE filename LIKE '%[%]%';

-- ✅ 使用 ESCAPE 子句进行反斜杠转义
SELECT * FROM files WHERE filename LIKE '%test\_%' ESCAPE '\';

-- ❌ 直接使用反斜杠通常不工作（需要 ESCAPE 子句）
-- SELECT * FROM files WHERE filename LIKE '%test\_%'; -- 可能不按预期工作
```

#### Oracle
```sql
-- ❌ 必须使用 ESCAPE 子句
SELECT * FROM files WHERE filename LIKE '%test\_%' ESCAPE '\';

-- ✅ 使用自定义转义字符
SELECT * FROM files WHERE filename LIKE '%test!_%' ESCAPE '!';
```

#### SQLite
```sql
-- ✅ 默认支持反斜杠转义（不需要 ESCAPE 子句）
SELECT * FROM files WHERE filename LIKE '%test\_%';

-- ✅ 也可以显式指定 ESCAPE 子句
SELECT * FROM files WHERE filename LIKE '%test\_%' ESCAPE '\';
```

## 默认转义行为总结

| 数据库 | 默认反斜杠转义 | 需要 ESCAPE 子句 | 特殊转义方法 |
|--------|----------------|------------------|--------------|
| MySQL | ✅ 支持 | ❌ 不需要 | - |
| PostgreSQL | ✅ 支持 | ❌ 不需要 | - |
| SQL Server | ❌ 不支持 | ✅ 需要 | `[_]` 方括号转义 |
| Oracle | ❌ 不支持 | ✅ 需要 | - |
| SQLite | ✅ 支持 | ❌ 不需要 | - |

## 数据库特定通配符

### MySQL
除了标准通配符，MySQL 还支持：

#### 字符集合 [...]
```sql
-- 匹配 a, b, c 中的任意一个字符
SELECT * FROM users WHERE name LIKE '[abc]%';

-- 匹配数字 0-9
SELECT * FROM products WHERE code LIKE '[0-9]%';

-- 匹配非数字字符
SELECT * FROM products WHERE code LIKE '[^0-9]%';
```

#### 范围 [a-z]
```sql
-- 匹配 a 到 z 的任意字符
SELECT * FROM users WHERE name LIKE '[a-z]%';

-- 匹配 A 到 Z 的任意字符
SELECT * FROM users WHERE name LIKE '[A-Z]%';
```

### PostgreSQL
PostgreSQL 除了标准通配符，还提供：

#### ILIKE 操作符
```sql
-- 大小写不敏感的匹配
SELECT * FROM users WHERE name ILIKE 'john%';
-- 匹配: John, john, JOHN, Johnny 等
```

#### POSIX 正则表达式
```sql
-- 使用 ~ 操作符进行正则匹配
SELECT * FROM users WHERE name ~ '^J.*n$';

-- 大小写不敏感的正则匹配
SELECT * FROM users WHERE name ~* '^j.*n$';
```

### SQL Server
SQL Server 支持额外的通配符：

#### 字符集合 [...]
```sql
-- 匹配特定字符集合
SELECT * FROM users WHERE name LIKE '[JM]%';
-- 匹配以 J 或 M 开头的名字

-- 匹配字符范围
SELECT * FROM users WHERE name LIKE '[A-F]%';
```

#### 排除字符集合 [^...]
```sql
-- 匹配不在指定字符集合中的字符
SELECT * FROM users WHERE name LIKE '[^A-M]%';
-- 匹配不以 A-M 开头的名字
```

### Oracle
Oracle 的 LIKE 操作符主要遵循 SQL 标准：

#### 基本通配符
```sql
-- 标准用法
SELECT * FROM users WHERE name LIKE 'J%';
SELECT * FROM users WHERE name LIKE 'J_n';
```

## 需要转义的字符总结

| 字符 | 含义 | 需要转义的场景 | 支持的数据库 |
|------|------|----------------|--------------|
| `%` | 匹配任意数量字符 | 搜索包含百分号的文字 | 所有数据库 |
| `_` | 匹配单个字符 | 搜索包含下划线的文字 | 所有数据库 |
| `\` | 转义字符 | 搜索包含反斜杠的文字 | 所有数据库 |
| `[` | 字符集合开始 | 搜索包含方括号的文字 | SQL Server/MySQL |
| `]` | 字符集合结束 | 搜索包含方括号的文字 | SQL Server/MySQL |
| `^` | 排除字符集合 | 搜索包含插入符号的文字 | SQL Server/MySQL |

## 转义字符的层次解析

### Go 语言中的字符串转义
```go
// 转义层次分析
"ESCAPE '\\'"      // Go字符串: ESCAPE '\'     (正确)
"ESCAPE '\\\\'"    // Go字符串: ESCAPE '\\'    (过度转义)

// 实际测试
fmt.Println("ESCAPE '\\'")     // 输出: ESCAPE '\'
fmt.Println("ESCAPE '\\\\'")   // 输出: ESCAPE '\\'
```

### 各数据库的正确 ESCAPE 语法
```sql
-- 所有数据库的正确语法
SELECT * FROM table WHERE column LIKE '%pattern\_%' ESCAPE '\';

-- 错误的语法（过度转义）
SELECT * FROM table WHERE column LIKE '%pattern\_%' ESCAPE '\\';
```

## 性能考虑

### 效率对比
```sql
-- 高效：前缀匹配可以使用索引
SELECT * FROM users WHERE name LIKE 'John%';

-- 低效：前导通配符无法使用索引
SELECT * FROM users WHERE name LIKE '%John%';

-- 低效：后缀匹配通常无法使用索引
SELECT * FROM users WHERE name LIKE '%John';
```

### 优化建议
1. **避免前导通配符**：尽量不要以 `%` 开头
2. **使用全文索引**：对于复杂的文本搜索
3. **限制结果集**：使用 `LIMIT` 或 `TOP` 限制返回行数
4. **考虑专用搜索引擎**：对于复杂的搜索需求

## 安全考虑

### SQL 注入风险
```sql
-- 危险：直接拼接用户输入
SELECT * FROM users WHERE name LIKE '%" + userInput + "%';

-- 安全：使用参数化查询
SELECT * FROM users WHERE name LIKE ? -- 参数：%userInput%
```

### 输入验证
```go
// Go 语言示例：转义用户输入
func escapeForLike(input string) string {
    // 转义特殊字符
    input = strings.ReplaceAll(input, `\`, `\\`)
    input = strings.ReplaceAll(input, `%`, `\%`)
    input = strings.ReplaceAll(input, `_`, `\_`)
    return input
}
```

## 实际应用示例

### 1. 用户搜索功能
```sql
-- 搜索用户名包含关键词的用户
SELECT id, username, email 
FROM users 
WHERE username LIKE '%search_term%'
   OR email LIKE '%search_term%';
```

### 2. 文件名模式匹配
```sql
-- 查找所有图片文件
SELECT * FROM files 
WHERE filename LIKE '%.jpg' 
   OR filename LIKE '%.jpeg' 
   OR filename LIKE '%.png' 
   OR filename LIKE '%.gif';
```

### 3. 电话号码格式验证
```sql
-- 匹配特定格式的电话号码：(xxx) xxx-xxxx
SELECT * FROM contacts 
WHERE phone LIKE '([0-9][0-9][0-9]) [0-9][0-9][0-9]-[0-9][0-9][0-9][0-9]';
```

### 4. 产品代码匹配
```sql
-- 匹配产品代码格式：ABC-123
SELECT * FROM products 
WHERE product_code LIKE '[A-Z][A-Z][A-Z]-[0-9][0-9][0-9]';
```

## 最佳实践

### 1. 转义处理
```go
// 完整的转义函数
func escapeLikeValue(value string) string {
    // 按顺序转义，避免重复转义
    value = strings.ReplaceAll(value, `\`, `\\`)  // 先转义反斜杠
    value = strings.ReplaceAll(value, `%`, `\%`)  // 再转义百分号
    value = strings.ReplaceAll(value, `_`, `\_`)  // 最后转义下划线
    return value
}
```

### 2. 智能搜索实现
```go
// 智能搜索：将空格替换为通配符
func smartSearch(query string) string {
    // 先转义特殊字符
    query = escapeLikeValue(query)
    
    // 将空格替换为 %，实现分词搜索
    re := regexp.MustCompile(`\s+`)
    query = re.ReplaceAllString(query, "%")
    
    // 前后加上通配符
    return "%" + query + "%"
}
```

### 3. 数据库兼容性处理
```go
// 根据数据库类型选择转义方法
func getEscapeClause(driverName string) string {
    switch driverName {
    case "mysql", "postgres", "sqlite":
        return "" // 这些数据库默认支持反斜杠转义
    case "sqlserver":
        return "ESCAPE '\\'" // SQL Server 需要显式 ESCAPE
    case "oracle":
        return "ESCAPE '\\'" // Oracle 需要显式 ESCAPE
    default:
        return "ESCAPE '\\'" // 保险起见，默认使用 ESCAPE
    }
}

// 总是使用 ESCAPE 子句的安全方法
func getSafeEscapeClause(driverName string) string {
    switch driverName {
    case "postgres", "sqlite", "oracle", "sqlserver", "mysql":
        return "ESCAPE '\\'"
    default:
        return "ESCAPE '\\'"
    }
}
```

## 测试用例

### 基本通配符测试
```sql
-- 测试数据
INSERT INTO test_table (name) VALUES 
('John'), ('Jane'), ('J_hn'), ('J%hn'), ('J\\hn'), ('admin_user'), ('user%data');

-- 测试 % 通配符
SELECT * FROM test_table WHERE name LIKE 'J%';
-- 期望: John, Jane, J_hn, J%hn, J\\hn

-- 测试 _ 通配符
SELECT * FROM test_table WHERE name LIKE 'J_hn';
-- 期望: J_hn, J%hn, J\\hn (如果没有转义)

-- 测试转义
SELECT * FROM test_table WHERE name LIKE 'J\_hn' ESCAPE '\';
-- 期望: J_hn (只匹配包含下划线的)
```

### 转义功能测试
```sql
-- 测试百分号转义
SELECT * FROM test_table WHERE name LIKE '%\%%' ESCAPE '\';
-- 期望: J%hn, user%data

-- 测试下划线转义
SELECT * FROM test_table WHERE name LIKE '%\_%' ESCAPE '\';
-- 期望: J_hn, admin_user
```

## 总结

SQL LIKE 通配符是数据库查询中的强大工具，但需要注意：

1. **标准通配符**：`%`（任意字符）和 `_`（单个字符）
2. **默认转义行为**：MySQL、PostgreSQL、SQLite 默认支持反斜杠转义
3. **ESCAPE 子句**：所有数据库都支持，语法为 `ESCAPE '\'`
4. **转义处理**：对于包含特殊字符的搜索，必须正确转义
5. **避免过度转义**：ESCAPE 子句中只需要一个反斜杠字符
6. **性能影响**：前导通配符会影响索引使用
7. **安全考虑**：防止 SQL 注入，验证用户输入
8. **数据库差异**：不同数据库有不同的扩展语法

正确使用 LIKE 通配符能够实现灵活的搜索功能，同时保证系统的安全性和性能。 