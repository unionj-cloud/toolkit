package caches

import (
	"github.com/unionj-cloud/toolkit/copier"
)

func SetPointedValue(dest interface{}, src interface{}) {
	copier.DeepCopy(src, dest)
}
