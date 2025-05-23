package caches

import (
	"reflect"
	"sync"

	"github.com/unionj-cloud/toolkit/copier"
)

func ease(t *queryTask, queue *sync.Map) *queryTask {
	eq := &eased{
		task: t,
		wg:   &sync.WaitGroup{},
	}
	eq.wg.Add(1)

	runner, ok := queue.LoadOrStore(t.GetId(), eq)
	et := runner.(*eased)

	// If this request is the first of its kind, we execute the Run
	if !ok {
		et.task.Run()

		resultValue := reflect.ValueOf(et.task.db.Statement.Dest)

		if resultValue.IsValid() && !resultValue.IsZero() && resultValue.Kind() == reflect.Ptr {
			elementValue := resultValue.Elem()
			elementType := elementValue.Type()
			newValue := reflect.New(elementType)
			et.task.dest = newValue.Interface()

			copier.DeepCopy(et.task.db.Statement.Dest, et.task.dest)
			et.task.rowsAffected = et.task.db.Statement.RowsAffected
		} else {
			et.task.dest = et.task.db.Statement.Dest
			et.task.rowsAffected = et.task.db.Statement.RowsAffected
		}

		queue.Delete(et.task.GetId())
		et.wg.Done()
	}

	et.wg.Wait()
	return et.task
}

type eased struct {
	task *queryTask
	wg   *sync.WaitGroup
}
