package unitofwork

import (
	"fmt"
	"reflect"
	"sort"
)

// DependencyManager 实体依赖关系管理器
type DependencyManager struct {
	// 依赖图：dependent -> dependencies
	dependencyGraph map[reflect.Type][]reflect.Type
	// 实体权重：用于同级实体的排序
	entityWeights map[reflect.Type]int
}

// NewDependencyManager 创建依赖管理器
func NewDependencyManager() *DependencyManager {
	return &DependencyManager{
		dependencyGraph: make(map[reflect.Type][]reflect.Type),
		entityWeights:   make(map[reflect.Type]int),
	}
}

// RegisterDependency 注册实体依赖关系
// dependent 依赖于 dependency
func (dm *DependencyManager) RegisterDependency(dependent, dependency reflect.Type) {
	if dependent == dependency {
		return // 自依赖，忽略
	}

	dependencies := dm.dependencyGraph[dependent]

	// 检查是否已存在
	for _, dep := range dependencies {
		if dep == dependency {
			return
		}
	}

	dm.dependencyGraph[dependent] = append(dependencies, dependency)
}

// RegisterEntityWeight 注册实体权重（用于同级排序）
func (dm *DependencyManager) RegisterEntityWeight(entityType reflect.Type, weight int) {
	dm.entityWeights[entityType] = weight
}

// GetInsertionOrder 获取插入顺序（依赖的实体先插入）
func (dm *DependencyManager) GetInsertionOrder(entities []Entity) ([]Entity, error) {
	return dm.topologicalSort(entities, false)
}

// GetDeletionOrder 获取删除顺序（被依赖的实体后删除）
func (dm *DependencyManager) GetDeletionOrder(entities []Entity) ([]Entity, error) {
	result, err := dm.topologicalSort(entities, true)
	if err != nil {
		return nil, err
	}

	// 删除顺序是插入顺序的反向
	for i := 0; i < len(result)/2; i++ {
		j := len(result) - 1 - i
		result[i], result[j] = result[j], result[i]
	}

	return result, nil
}

// topologicalSort 拓扑排序算法
func (dm *DependencyManager) topologicalSort(entities []Entity, reverse bool) ([]Entity, error) {
	if len(entities) == 0 {
		return entities, nil
	}

	// 构建类型到实体的映射
	typeToEntities := make(map[reflect.Type][]Entity)
	for _, entity := range entities {
		entityType := reflect.TypeOf(entity)
		typeToEntities[entityType] = append(typeToEntities[entityType], entity)
	}

	// 获取所有涉及的实体类型
	entityTypes := make([]reflect.Type, 0, len(typeToEntities))
	for entityType := range typeToEntities {
		entityTypes = append(entityTypes, entityType)
	}

	// 计算入度
	inDegree := dm.calculateInDegree(entityTypes)

	// 拓扑排序
	queue := make([]reflect.Type, 0)
	result := make([]reflect.Type, 0)

	// 找到所有入度为0的节点
	for _, entityType := range entityTypes {
		if inDegree[entityType] == 0 {
			queue = append(queue, entityType)
		}
	}

	// 对队列进行权重排序
	dm.sortByWeight(queue)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// 处理当前节点的邻接节点
		neighbors := dm.getNeighbors(current, reverse)
		nextLevel := make([]reflect.Type, 0)

		for _, neighbor := range neighbors {
			if _, exists := inDegree[neighbor]; !exists {
				continue // 不在当前处理的实体集合中
			}

			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				nextLevel = append(nextLevel, neighbor)
			}
		}

		// 对同级节点按权重排序
		dm.sortByWeight(nextLevel)
		queue = append(queue, nextLevel...)
	}

	// 检查是否存在环
	if len(result) != len(entityTypes) {
		return nil, fmt.Errorf("circular dependency detected in entity relationships")
	}

	// 根据排序后的类型构建最终的实体列表
	finalResult := make([]Entity, 0, len(entities))
	for _, entityType := range result {
		entitiesOfType := typeToEntities[entityType]
		// 对同类型的实体按ID排序（确保排序的确定性）
		dm.sortEntitiesByID(entitiesOfType)
		finalResult = append(finalResult, entitiesOfType...)
	}

	return finalResult, nil
}

// calculateInDegree 计算每个节点的入度
func (dm *DependencyManager) calculateInDegree(entityTypes []reflect.Type) map[reflect.Type]int {
	inDegree := make(map[reflect.Type]int)

	// 初始化所有节点的入度为0
	for _, entityType := range entityTypes {
		inDegree[entityType] = 0
	}

	// 计算每个节点的入度
	for dependent, dependencies := range dm.dependencyGraph {
		if _, exists := inDegree[dependent]; !exists {
			continue // 不在当前处理的实体集合中
		}

		for _, dependency := range dependencies {
			if _, exists := inDegree[dependency]; exists {
				inDegree[dependency]++
			}
		}
	}

	return inDegree
}

// getNeighbors 获取邻接节点
func (dm *DependencyManager) getNeighbors(entityType reflect.Type, reverse bool) []reflect.Type {
	if !reverse {
		// 正向：返回当前类型的依赖项
		return dm.dependencyGraph[entityType]
	} else {
		// 反向：返回依赖当前类型的项
		neighbors := make([]reflect.Type, 0)
		for dependent, dependencies := range dm.dependencyGraph {
			for _, dependency := range dependencies {
				if dependency == entityType {
					neighbors = append(neighbors, dependent)
					break
				}
			}
		}
		return neighbors
	}
}

// sortByWeight 按权重排序
func (dm *DependencyManager) sortByWeight(entityTypes []reflect.Type) {
	sort.Slice(entityTypes, func(i, j int) bool {
		weightI := dm.getEntityWeight(entityTypes[i])
		weightJ := dm.getEntityWeight(entityTypes[j])

		if weightI != weightJ {
			return weightI < weightJ
		}

		// 权重相同时，按类型名称排序（确保确定性）
		return entityTypes[i].String() < entityTypes[j].String()
	})
}

// getEntityWeight 获取实体权重
func (dm *DependencyManager) getEntityWeight(entityType reflect.Type) int {
	if weight, exists := dm.entityWeights[entityType]; exists {
		return weight
	}
	return 0 // 默认权重
}

// sortEntitiesByID 按ID排序实体
func (dm *DependencyManager) sortEntitiesByID(entities []Entity) {
	sort.Slice(entities, func(i, j int) bool {
		idI := toString(entities[i].GetID())
		idJ := toString(entities[j].GetID())
		return idI < idJ
	})
}

// DefaultDependencyManager 默认依赖管理器，包含常见的实体依赖关系
func DefaultDependencyManager() *DependencyManager {
	dm := NewDependencyManager()

	// 可以在这里注册常见的依赖关系
	// 例如：用户 -> 角色, 订单 -> 用户 等

	return dm
}

// GetAllEntityTypes 获取依赖图中的所有实体类型
func (dm *DependencyManager) GetAllEntityTypes() []reflect.Type {
	typeSet := make(map[reflect.Type]bool)

	for dependent, dependencies := range dm.dependencyGraph {
		typeSet[dependent] = true
		for _, dependency := range dependencies {
			typeSet[dependency] = true
		}
	}

	types := make([]reflect.Type, 0, len(typeSet))
	for entityType := range typeSet {
		types = append(types, entityType)
	}

	return types
}

// HasDependency 检查是否存在依赖关系
func (dm *DependencyManager) HasDependency(dependent, dependency reflect.Type) bool {
	dependencies := dm.dependencyGraph[dependent]
	for _, dep := range dependencies {
		if dep == dependency {
			return true
		}
	}
	return false
}

// RemoveDependency 移除依赖关系
func (dm *DependencyManager) RemoveDependency(dependent, dependency reflect.Type) {
	dependencies := dm.dependencyGraph[dependent]
	newDependencies := make([]reflect.Type, 0, len(dependencies))

	for _, dep := range dependencies {
		if dep != dependency {
			newDependencies = append(newDependencies, dep)
		}
	}

	dm.dependencyGraph[dependent] = newDependencies
}

// Clear 清空所有依赖关系
func (dm *DependencyManager) Clear() {
	dm.dependencyGraph = make(map[reflect.Type][]reflect.Type)
	dm.entityWeights = make(map[reflect.Type]int)
}
