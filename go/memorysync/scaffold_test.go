package memorysync

import "testing"

func TestScaffold(t *testing.T) {
	// the package builds + tests run
	if 1+1 != 2 {
		t.Fatal("math is broken")
	}
}
