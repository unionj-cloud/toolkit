package caches

import (
	"reflect"
	"sync"
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

		if resultValue.IsValid(){
			elementValue := reflect.ValueOf(et.task.db.Statement.Dest).Elem()
			et.task.dest = elementValue.Interface()
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
