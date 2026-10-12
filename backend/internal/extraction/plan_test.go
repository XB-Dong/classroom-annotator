package extraction

import (
	"reflect"
	"testing"
)

func TestPlanUsesIntegerMillisecondsAndExclusiveEnd(t *testing.T) {
	p, e := Plan(0.003, 0.001, 0.01, 0.01, 10)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(p, []int64{1, 4, 7}) {
		t.Fatalf("targets=%v", p)
	}
	for _, v := range []float64{0, 0.0001, 0.0011} {
		if _, e := Plan(v, 0, 0.01, 0.01, 10); e == nil {
			t.Fatalf("accepted interval %v", v)
		}
	}
	if _, e := Plan(.001, 0, 10, 10, 5); e == nil {
		t.Fatal("frame budget ignored")
	}
	if _, e := Plan(1, 0, 1.001, 1, 10); e == nil {
		t.Fatal("duration exceeded")
	}
}
