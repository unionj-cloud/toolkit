package copier

import (
	"reflect"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/decoder"
	"github.com/pkg/errors"
	"golang.org/x/exp/maps"
)

var json = sonic.ConfigDefault

// DeepCopy src to target with json marshal and unmarshal
func DeepCopy(src, target interface{}) error {
	if src == nil || target == nil {
		return nil
	}
	if reflect.ValueOf(target).Kind() != reflect.Ptr {
		return errors.New("Target should be a pointer")
	}

	srcVal := src
	srcValue := reflect.ValueOf(src)

	// 获取底层值（如果是指针则获取指针指向的值）
	for srcValue.Kind() == reflect.Ptr {
		srcValue = srcValue.Elem()
	}

	// 如果底层类型是 map，进行深拷贝
	if srcValue.Kind() == reflect.Map {
		// 尝试转换为 map[string]interface{}
		if val, ok := srcValue.Interface().(map[string]interface{}); ok {
			srcVal = maps.Clone(val)
		} else {
			// 对于其他类型的 map，创建一个新的 map 并复制所有键值对
			newMap := reflect.MakeMap(srcValue.Type())
			iter := srcValue.MapRange()
			for iter.Next() {
				newMap.SetMapIndex(iter.Key(), iter.Value())
			}
			srcVal = newMap.Interface()
		}
	}

	b, err := json.MarshalToString(srcVal)
	if err != nil {
		return errors.WithStack(err)
	}
	dec := decoder.NewDecoder(b)
	dec.UseInt64()
	if err = dec.Decode(target); err != nil {
		return errors.WithStack(err)
	}
	return nil
}
