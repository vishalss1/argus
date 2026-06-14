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

// RollingStats tracks statistics for a single metric of a device dynamically.
type RollingStats struct {
	window  []float64
	size    int
	count   int64
	min     float64
	max     float64
	varying bool
}

func NewRollingStats(size int) *RollingStats {
	return &RollingStats{
		size: size,
	}
}

func (s *RollingStats) Push(val float64) (mean, variance, stdDev, zScore float64, outlier, stuck bool) {
	nBefore := len(s.window)
	if nBefore > 0 && val != s.window[nBefore-1] {
		s.varying = true
	}

	s.window = append(s.window, val)
	if len(s.window) > s.size {
		s.window = s.window[1:]
	}

	if s.count == 0 {
		s.min = val
		s.max = val
	} else {
		if val < s.min {
			s.min = val
		}
		if val > s.max {
			s.max = val
		}
	}
	s.count++

	n := len(s.window)
	if n == 0 {
		return 0, 0, 0, 0, false, false
	}

	// Calculate Mean
	sum := 0.0
	for _, v := range s.window {
		sum += v
	}
	mean = sum / float64(n)

	// Calculate Variance and StdDev
	varianceSum := 0.0
	for _, v := range s.window {
		varianceSum += (v - mean) * (v - mean)
	}
	variance = varianceSum / float64(n)
	stdDev = math.Sqrt(variance)

	// Calculate Z-Score
	if stdDev > 0 {
		zScore = (val - mean) / stdDev
	}

	// Detect Outlier (only if we have enough samples to avoid false positives on startup)
	if n >= 5 && stdDev > 0 {
		outlier = math.Abs(zScore) > 3.0
	}

	// Detect Stuck Sensor: check if last 10 samples are identical AND metric has previously varied
	if s.varying && n >= 10 {
		stuck = true
		lastVal := s.window[n-1]
		for i := n - 10; i < n; i++ {
			if s.window[i] != lastVal {
				stuck = false
				break
			}
		}
	}

	return mean, variance, stdDev, zScore, outlier, stuck
}

func (s *RollingStats) Min() float64 {
	return s.min
}

func (s *RollingStats) Max() float64 {
	return s.max
}

func (s *RollingStats) Count() int64 {
	return s.count
}

