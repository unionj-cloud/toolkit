package copier

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type Family struct {
	Father string
	Mather string
	Pets   map[string]string
	Toys   []string
}

type TestStruct struct {
	Name   string
	Age    int
	Family Family
}

type FamilyShadow struct {
	Father string
	Mather string
	Pets   map[string]string
}

type TestStructShadow struct {
	Name   string
	Family FamilyShadow
}

func TestDeepCopy(t *testing.T) {
	t.Run("nil values", func(t *testing.T) {
		var target interface{}
		err := DeepCopy(nil, &target)
		assert.NoError(t, err)
		assert.Nil(t, target)
	})

	t.Run("non-pointer target", func(t *testing.T) {
		var target string
		err := DeepCopy("source", target)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Target should be a pointer")
	})

	t.Run("basic types", func(t *testing.T) {
		src := "hello"
		var target string
		err := DeepCopy(src, &target)
		assert.NoError(t, err)
		assert.Equal(t, src, target)
	})

	t.Run("map[string]interface{}", func(t *testing.T) {
		src := map[string]interface{}{
			"name": "test",
			"age":  30,
			"nested": map[string]interface{}{
				"key": "value",
			},
		}
		var target map[string]interface{}
		err := DeepCopy(src, &target)
		assert.NoError(t, err)

		// 使用 JSONEq 比较内容
		srcJSON, _ := json.Marshal(src)
		targetJSON, _ := json.Marshal(target)
		assert.JSONEq(t, string(srcJSON), string(targetJSON))

		// 验证深拷贝
		src["name"] = "modified"
		src["nested"].(map[string]interface{})["key"] = "modified"
		assert.NotEqual(t, src["name"], target["name"])
		assert.NotEqual(t, src["nested"].(map[string]interface{})["key"],
			target["nested"].(map[string]interface{})["key"])
	})

	t.Run("pointer to map", func(t *testing.T) {
		src := &map[string]interface{}{
			"name": "test",
			"age":  30,
		}
		var target map[string]interface{}
		err := DeepCopy(src, &target)
		assert.NoError(t, err)

		// 使用 JSONEq 比较内容
		srcJSON, _ := json.Marshal(*src)
		targetJSON, _ := json.Marshal(target)
		assert.JSONEq(t, string(srcJSON), string(targetJSON))
	})

	t.Run("custom map type", func(t *testing.T) {
		src := map[int]string{
			1: "one",
			2: "two",
		}
		var target map[int]string
		err := DeepCopy(src, &target)
		assert.NoError(t, err)
		assert.Equal(t, src, target)

		// 验证深拷贝
		src[1] = "modified"
		assert.NotEqual(t, src[1], target[1])
	})

	t.Run("complex nested structure", func(t *testing.T) {
		type nested struct {
			Field string
			Map   map[string]int
		}
		src := nested{
			Field: "test",
			Map: map[string]int{
				"one": 1,
				"two": 2,
			},
		}
		var target nested
		err := DeepCopy(src, &target)
		assert.NoError(t, err)

		// 使用 JSONEq 比较内容
		srcJSON, _ := json.Marshal(src)
		targetJSON, _ := json.Marshal(target)
		assert.JSONEq(t, string(srcJSON), string(targetJSON))

		// 验证深拷贝
		src.Map["one"] = 100
		assert.NotEqual(t, src.Map["one"], target.Map["one"])
	})

	t.Run("concurrent map access", func(t *testing.T) {
		src := map[string]interface{}{
			"data": "value",
		}

		done := make(chan bool)
		for i := 0; i < 10000; i++ {
			go func() {
				var target map[string]interface{}
				err := DeepCopy(src, &target)
				assert.NoError(t, err)
				assert.Equal(t, "value", target["data"])
				done <- true
			}()
		}

		// 等待所有goroutine完成
		for i := 0; i < 10000; i++ {
			<-done
		}
	})
}

func TestDeepCopy_ShouldHasError(t *testing.T) {
	pets := make(map[string]string)
	pets["a"] = "dog"
	pets["b"] = "cat"

	family := Family{
		Father: "Jack",
		Mather: "Lily",
		Pets:   pets,
		Toys: []string{
			"car",
			"lego",
		},
	}
	src := TestStruct{
		Name:   "Rose",
		Age:    18,
		Family: family,
	}

	var target TestStructShadow

	type args struct {
		src    interface{}
		target interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "TestDeepCopy",
			args: args{
				src:    src,
				target: target,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := DeepCopy(tt.args.src, tt.args.target); (err != nil) != tt.wantErr {
				t.Errorf("DeepCopy() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
