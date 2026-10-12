package extraction

import (
	"errors"
	"math"
)

func millis(v float64) (int64, error) {
	x := v * 1000
	if math.IsNaN(x) || math.IsInf(x, 0) || x < 0 || x > float64(math.MaxInt64/2) || math.Abs(x-math.Round(x)) > 1e-7 {
		return 0, errors.New("time must use millisecond precision")
	}
	return int64(math.Round(x)), nil
}
func Plan(interval, start, end, duration float64, max int) ([]int64, error) {
	step, e := millis(interval)
	if e != nil || step < 1 {
		return nil, errors.New("invalid interval")
	}
	a, e := millis(start)
	if e != nil {
		return nil, e
	}
	b, e := millis(end)
	if e != nil {
		return nil, e
	}
	if math.IsNaN(duration) || math.IsInf(duration, 0) || duration <= 0 || duration > float64(math.MaxInt64/2000) {
		return nil, errors.New("invalid duration")
	}
	if a >= b || b > int64(math.Ceil(duration*1000)) {
		return nil, errors.New("invalid interval bounds")
	}
	count := (b - a + step - 1) / step
	if count > int64(max) || max <= 0 {
		return nil, errors.New("frame budget exceeded")
	}
	out := make([]int64, count)
	for n := range out {
		out[n] = a + int64(n)*step
	}
	return out, nil
}
