package anomaly

import (
	"math"
)

// EWMA (Exponential Weighted Moving Average)
type EWMA struct {
	alpha float64
	value float64
	init  bool
}

func NewEWMA(alpha float64) *EWMA {
	return &EWMA{alpha: alpha}
}

func (e *EWMA) Update(val float64) float64 {
	if !e.init {
		e.value = val
		e.init = true
		return val
	}
	e.value = e.alpha*val + (1-e.alpha)*e.value
	return e.value
}

func (e *EWMA) Value() float64 {
	return e.value
}

// ZScore detector
type ZScore struct {
	window []float64
	size   int
}

func NewZScore(size int) *ZScore {
	return &ZScore{size: size}
}

func (z *ZScore) Push(val float64) float64 {
	z.window = append(z.window, val)
	if len(z.window) > z.size {
		z.window = z.window[1:]
	}

	if len(z.window) < z.size {
		return 0
	}

	mean := 0.0
	for _, v := range z.window {
		mean += v
	}
	mean /= float64(len(z.window))

	stdDev := 0.0
	for _, v := range z.window {
		stdDev += math.Pow(v-mean, 2)
	}
	stdDev = math.Sqrt(stdDev / float64(len(z.window)))

	if stdDev == 0 {
		return 0
	}

	return (val - mean) / stdDev
}
