# Flowable引擎核心组件DbSqlSession技术深度剖析

## 引言

在 Flowable 工作流引擎的架构中，数据持久化层扮演着至关重要的角色。其中，`DbSqlSession` 作为数据库访问的核心组件，承担着 ORM 映射、缓存管理、事务控制等关键职责。本文将深度剖析 Flowable 引擎中 `DbSqlSession` 的设计理念、实现原理和演进历程。

## DbSqlSession 架构概览

### 设计理念

`DbSqlSession` 采用了会话模式（Session Pattern）和工作单元模式（Unit of Work Pattern）的设计思想，主要负责：

1. **延迟刷新（Delayed Flushing）**：将插入、更新、删除操作延迟到事务提交时统一执行
2. **可选脏检查（Optional Dirty Checking）**：智能识别对象变更，避免不必要的数据库操作
3. **数据库特定映射（Database-specific Statement Mapping）**：支持多种数据库的 SQL 方言

### 核心组件架构

```java
public class DbSqlSession implements Session {
    // MyBatis 会话，实际执行 SQL
    protected SqlSession sqlSession;
    
    // 会话工厂，负责创建和配置
    protected DbSqlSessionFactory dbSqlSessionFactory;
    
    // 缓存体系
    protected Map<Class<? extends PersistentObject>, List<PersistentObject>> insertedObjects;
    protected Map<Class<?>, Map<String, CachedObject>> cachedObjects;
    
    // 删除操作队列
    protected List<DeleteOperation> deleteOperations;
    
    // 反序列化对象管理
    protected List<DeserializedObject> deserializedObjects;
}
```

## 缓存机制深度分析

### 二级缓存架构

DbSqlSession 实现了一个精巧的二级缓存系统：

#### 1. 第一级缓存：插入对象缓存

```java
protected Map<Class<? extends PersistentObject>, List<PersistentObject>> insertedObjects = new HashMap<>();

public void insert(PersistentObject persistentObject) {
    if (persistentObject.getId() == null) {
        String id = dbSqlSessionFactory.getIdGenerator().getNextId();
        persistentObject.setId(id);
    }
    
    Class<? extends PersistentObject> clazz = persistentObject.getClass();
    if (!insertedObjects.containsKey(clazz)) {
        insertedObjects.put(clazz, new ArrayList<>());
    }
    
    insertedObjects.get(clazz).add(persistentObject);
    cachePut(persistentObject, false);
}
```

**设计亮点**：
- 按类型分组管理插入对象，便于批量操作优化
- 自动 ID 生成，保证对象唯一性
- 立即放入缓存，避免重复查询

#### 2. 第二级缓存：查询结果缓存

```java
protected Map<Class<?>, Map<String, CachedObject>> cachedObjects = new HashMap<>();

public static class CachedObject {
    protected PersistentObject persistentObject;
    protected Object persistentObjectState;
    
    public CachedObject(PersistentObject persistentObject, boolean storeState) {
        this.persistentObject = persistentObject;
        if (storeState) {
            this.persistentObjectState = persistentObject.getPersistentState();
        }
    }
}
```

**核心特性**：
- 存储对象状态快照，实现脏检查
- 双重 Map 结构（类型 -> ID -> 对象），O(1) 查找效率
- 智能状态管理，区分新建和查询对象

### 缓存过滤机制

```java
protected PersistentObject cacheFilter(PersistentObject persistentObject) {
    PersistentObject cachedPersistentObject = cacheGet(persistentObject.getClass(), persistentObject.getId());
    if (cachedPersistentObject != null) {
        return cachedPersistentObject; // 返回缓存对象
    }
    cachePut(persistentObject, true); // 缓存新对象
    return persistentObject;
}
```

## 删除操作的策略模式实现

### 删除操作接口设计

```java
public interface DeleteOperation {
    Class<? extends PersistentObject> getPersistentObjectClass();
    boolean sameIdentity(PersistentObject other);
    void clearCache();
    void execute();
}
```

### 三种删除策略

#### 1. 批量删除操作（BulkDeleteOperation）

```java
public class BulkDeleteOperation implements DeleteOperation {
    private String statement;
    private Object parameter;
    
    @Override
    public void execute() {
        sqlSession.delete(statement, parameter);
    }
}
```

**适用场景**：删除某个执行实例的所有变量等批量操作

#### 2. 检查式删除操作（CheckedDeleteOperation）

```java
public class CheckedDeleteOperation implements DeleteOperation {
    protected final PersistentObject persistentObject;
    
    @Override
    public void execute() {
        if (persistentObject instanceof HasRevision) {
            int nrOfRowsDeleted = sqlSession.delete(deleteStatement, persistentObject);
            if (nrOfRowsDeleted == 0) {
                throw new ActivitiOptimisticLockingException(...);
            }
        }
    }
}
```

**核心功能**：
- 乐观锁检查，防止并发修改
- 精确控制删除结果

#### 3. 批量检查式删除操作（BulkCheckedDeleteOperation）

```java
public class BulkCheckedDeleteOperation implements DeleteOperation {
    protected List<PersistentObject> persistentObjects = new ArrayList<>();
    
    @Override
    public void execute() {
        if (persistentObjects.get(0) instanceof HasRevision) {
            int nrOfRowsDeleted = sqlSession.delete(bulkDeleteStatement, persistentObjects);
            if (nrOfRowsDeleted < persistentObjects.size()) {
                throw new ActivitiOptimisticLockingException(...);
            }
        }
    }
}
```

## 工作单元模式的完美实现

### flush() 方法：事务的核心

```java
@Override
public void flush() {
    List<DeleteOperation> removedOperations = removeUnnecessaryOperations();
    
    flushDeserializedObjects();
    List<PersistentObject> updatedObjects = getUpdatedObjects();
    
    // 按依赖顺序执行操作
    flushInserts();
    flushUpdates(updatedObjects);
    flushDeletes(removedOperations);
}
```

### 优化算法：removeUnnecessaryOperations()

```java
protected List<DeleteOperation> removeUnnecessaryOperations() {
    for (Iterator<DeleteOperation> deleteIterator = deleteOperations.iterator(); deleteIterator.hasNext();) {
        DeleteOperation deleteOperation = deleteIterator.next();
        List<PersistentObject> insertedObjectsOfSameClass = insertedObjects.get(deletedPersistentObjectClass);
        
        if (insertedObjectsOfSameClass != null) {
            for (Iterator<PersistentObject> insertIterator = insertedObjectsOfSameClass.iterator(); insertIterator.hasNext();) {
                PersistentObject insertedObject = insertIterator.next();
                
                if (deleteOperation.sameIdentity(insertedObject)) {
                    // 插入和删除抵消
                    insertIterator.remove();
                    deleteIterator.remove();
                }
            }
        }
        
        deleteOperation.clearCache();
    }
}
```

**优化效果**：
- 避免无效的插入-删除操作
- 减少数据库 I/O
- 提升事务执行效率

## 实体依赖顺序管理

### EntityDependencyOrder 设计

```java
protected void flushInserts() {
    // 按实体依赖顺序处理
    for (Class<? extends PersistentObject> persistentObjectClass : EntityDependencyOrder.INSERT_ORDER) {
        if (insertedObjects.containsKey(persistentObjectClass)) {
            flushPersistentObjects(persistentObjectClass, insertedObjects.get(persistentObjectClass));
        }
    }
}
```

**设计价值**：
- 保证外键约束不被违反
- 维护数据一致性
- 支持复杂的实体关系

## 脏检查机制实现

### 智能变更检测

```java
public List<PersistentObject> getUpdatedObjects() {
    List<PersistentObject> updatedObjects = new ArrayList<>();
    
    for (Class<?> clazz : cachedObjects.keySet()) {
        Map<String, CachedObject> classCache = cachedObjects.get(clazz);
        
        for (CachedObject cachedObject : classCache.values()) {
            PersistentObject persistentObject = cachedObject.getPersistentObject();
            Object originalState = cachedObject.getPersistentObjectState();
            
            if (persistentObject.getPersistentState() != null &&
                !persistentObject.getPersistentState().equals(originalState)) {
                updatedObjects.add(persistentObject);
            }
        }
    }
    return updatedObjects;
}
```

## 数据库兼容性设计

### 多数据库支持

```java
public boolean isTablePresent(String tableName) {
    String databaseType = dbSqlSessionFactory.getDatabaseType();
    
    if ("postgres".equals(databaseType)) {
        tableName = tableName.toLowerCase();
    }
    
    try {
        tables = databaseMetaData.getTables(catalog, schema, tableName, JDBC_METADATA_TABLE_TYPES);
        return tables.next();
    } catch (Exception e) {
        throw new ActivitiException("couldn't check if tables are already present using metadata: " + e.getMessage(), e);
    }
}
```

### SQL 方言映射

```java
protected String getCleanVersion(String versionString) {
    Matcher matcher = CLEAN_VERSION_REGEX.matcher(versionString);
    if (!matcher.find()) {
        throw new ActivitiException("Illegal format for version: " + versionString);
    }
    return matcher.group();
}
```

## 性能优化策略

### 批量操作优化

```java
protected void flushBulkInsert(List<PersistentObject> persistentObjectList, Class<? extends PersistentObject> clazz) {
    if (persistentObjectList.size() <= dbSqlSessionFactory.getMaxNrOfStatementsInBulkInsert()) {
        sqlSession.insert(insertStatement, persistentObjectList);
    } else {
        // 分批处理大量数据
        for (int start = 0; start < persistentObjectList.size(); start += maxBatchSize) {
            List<PersistentObject> subList = persistentObjectList.subList(start, Math.min(start + maxBatchSize, persistentObjectList.size()));
            sqlSession.insert(insertStatement, subList);
        }
    }
}
```

### 乐观锁实现

```java
protected void flushUpdates(List<PersistentObject> updatedObjects) {
    for (PersistentObject updatedObject : updatedObjects) {
        int updatedRecords = sqlSession.update(updateStatement, updatedObject);
        if (updatedRecords != 1) {
            throw new ActivitiOptimisticLockingException(updatedObject + " was updated by another transaction concurrently");
        }
        
        if (updatedObject instanceof HasRevision) {
            ((HasRevision) updatedObject).setRevision(((HasRevision) updatedObject).getRevisionNext());
        }
    }
}
```

## Flowable 版本演进分析

### Flowable5 vs 新版本 Flowable 对比

通过对比源码可以发现显著的架构演进：

#### 1. 缓存架构升级

**Flowable5**:
```java
protected Map<Class<?>, Map<String, CachedObject>> cachedObjects = new HashMap<>();
```

**新版本 Flowable**:
```java
protected EntityCache entityCache;
protected Map<Class<? extends Entity>, Map<String, Entity>> insertedObjects = new HashMap<>();
protected Map<Class<? extends Entity>, Map<String, Entity>> deletedObjects = new HashMap<>();
```

**改进点**：
- 引入独立的 EntityCache 组件
- 分离插入和删除对象管理
- 使用 LinkedHashMap 保证插入顺序

#### 2. 删除操作重构

**新版本特性**：
```java
protected Map<Class<? extends Entity>, List<BulkDeleteOperation>> bulkDeleteOperations = new HashMap<>();

public void delete(String statement, Object parameter, Class<? extends Entity> entityClass) {
    if (!bulkDeleteOperations.containsKey(entityClass)) {
        bulkDeleteOperations.put(entityClass, new ArrayList<>(1));
    }
    bulkDeleteOperations.get(entityClass).add(new BulkDeleteOperation(statement, parameter));
}
```

**改进价值**：
- 更清晰的批量删除操作管理
- 按实体类型分组，提升执行效率
- 简化了删除策略的复杂度

#### 3. 查询缓存优化

```java
@SuppressWarnings("rawtypes")
public List selectList(String statement, ListQueryParameterObject parameter, Class entityClass) {
    parameter.setDatabaseType(dbSqlSessionFactory.getDatabaseType());
    if (parameter instanceof CacheAwareQuery) {
        return queryWithRawParameter(statement, (CacheAwareQuery) parameter, entityClass, true);
    } else {
        return selectListWithRawParameter(statement, parameter);
    }
}
```

**新增功能**：
- 缓存感知查询（CacheAwareQuery）
- 灵活的缓存加载策略
- 更精细的查询控制

## 最佳实践与应用建议

### 1. 合理使用缓存

```java
// 避免在大批量操作中缓存过多对象
@Service
public class ProcessInstanceService {
    
    public void batchCreateProcessInstances(List<ProcessDefinition> definitions) {
        for (int i = 0; i < definitions.size(); i++) {
            processEngineService.createProcessInstance(definitions.get(i));
            
            // 定期清理缓存，避免内存溢出
            if (i % 100 == 0) {
                dbSqlSession.flush();
            }
        }
    }
}
```

### 2. 优化删除操作

```java
// 优先使用批量删除减少数据库交互
public void cleanupProcessData(String processInstanceId) {
    // 使用 BulkDeleteOperation
    dbSqlSession.delete("deleteVariablesByProcessInstanceId", processInstanceId);
    dbSqlSession.delete("deleteTasksByProcessInstanceId", processInstanceId);
    
    // 避免逐个删除
    // for (Variable var : variables) {
    //     dbSqlSession.delete(var); // 低效方式
    // }
}
```

### 3. 事务边界管理

```java
@Transactional
public void complexBusinessOperation() {
    try {
        // 业务逻辑执行
        executeBusinessLogic();
        
        // 手动触发 flush，确保数据一致性
        dbSqlSession.flush();
        
    } catch (Exception e) {
        // 异常时回滚
        dbSqlSession.rollback();
        throw e;
    }
}
```

## 总结

DbSqlSession 作为 Flowable 引擎的数据访问核心，展现了优秀的架构设计和工程实践：

1. **模式应用**：工作单元模式和策略模式的完美结合
2. **性能优化**：多级缓存、批量操作、脏检查等机制
3. **扩展性**：支持多数据库、自定义映射、插件化删除策略
4. **可靠性**：乐观锁、事务管理、异常处理等保障

通过深入理解 DbSqlSession 的设计原理，我们可以：
- 更好地使用 Flowable 引擎
- 在自己的项目中借鉴其设计思想
- 针对特定场景进行性能优化
- 理解企业级软件的数据访问层设计

Flowable 引擎在版本演进中不断优化 DbSqlSession 的实现，体现了对性能、可维护性和扩展性的持续追求，为我们提供了宝贵的工程实践参考。