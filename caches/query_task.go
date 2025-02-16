package caches

import "github.com/wubin1989/gorm"

type queryTask struct {
	id           string
	db           *gorm.DB
	dest         interface{}
	rowsAffected int64
	queryCb      func(db *gorm.DB)
}

func (q *queryTask) GetId() string {
	return q.id
}

func (q *queryTask) Run() {
	q.queryCb(q.db)
}
