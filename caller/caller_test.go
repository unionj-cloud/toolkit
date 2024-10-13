package caller_test

import (
	"fmt"
	"testing"

	"github.com/unionj-cloud/toolkit/caller"
)

func TestCaller_String(t *testing.T) {
	c := caller.NewCaller()
	fmt.Println(c.String())
}
