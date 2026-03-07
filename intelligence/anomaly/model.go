package anomaly

import (
	"math"
	"sort"
)

// TimeSeriesWindow is a ring buffer for maintaining a sliding window of metric values.
type TimeSeriesWindow struct {
	values   []float64
	capacity int
	pos      int
	full     bool
}

// NewTimeSeriesWindow creates a new sliding window with the given capacity.
func NewTimeSeriesWindow(capacity int) *TimeSeriesWindow {
	return &TimeSeriesWindow{
		values:   make([]float64, capacity),
		capacity: capacity,
	}
}

// Add inserts a value into the window.
func (w *TimeSeriesWindow) Add(v float64) {
	w.values[w.pos] = v
	w.pos = (w.pos + 1) % w.capacity
	if w.pos == 0 {
		w.full = true
	}
}

// Len returns the number of values in the window.
func (w *TimeSeriesWindow) Len() int {
	if w.full {
		return w.capacity
	}
	return w.pos
}

// Values returns all values in insertion order.
func (w *TimeSeriesWindow) Values() []float64 {
	n := w.Len()
	result := make([]float64, n)
	if w.full {
		copy(result, w.values[w.pos:])
		copy(result[w.capacity-w.pos:], w.values[:w.pos])
	} else {
		copy(result, w.values[:w.pos])
	}
	return result
}

// Mean computes the arithmetic mean of the window values.
func Mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// StdDev computes the standard deviation of the window values.
func StdDev(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	mean := Mean(values)
	var sumSqDiff float64
	for _, v := range values {
		d := v - mean
		sumSqDiff += d * d
	}
	return math.Sqrt(sumSqDiff / float64(len(values)-1))
}

// ZScore computes how many standard deviations a value is from the mean.
func ZScore(value float64, values []float64) float64 {
	sd := StdDev(values)
	if sd == 0 {
		return 0
	}
	return (value - Mean(values)) / sd
}

// Percentile computes the p-th percentile of the values (0-100).
func Percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	rank := p / 100.0 * float64(len(sorted)-1)
	lower := int(math.Floor(rank))
	upper := int(math.Ceil(rank))
	if lower == upper {
		return sorted[lower]
	}
	frac := rank - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}

// EWMA implements Exponentially Weighted Moving Average.
type EWMA struct {
	alpha   float64
	value   float64
	initialized bool
}

// NewEWMA creates a new EWMA with the given smoothing factor (0 < alpha <= 1).
func NewEWMA(alpha float64) *EWMA {
	return &EWMA{alpha: alpha}
}

// Add incorporates a new value into the average.
func (e *EWMA) Add(v float64) {
	if !e.initialized {
		e.value = v
		e.initialized = true
		return
	}
	e.value = e.alpha*v + (1-e.alpha)*e.value
}

// Value returns the current EWMA value.
func (e *EWMA) Value() float64 {
	return e.value
}

// IsolationScore computes a simplified isolation-forest-style outlier score.
// Values near 1.0 indicate anomalies; values near 0.5 are normal.
func IsolationScore(value float64, values []float64) float64 {
	if len(values) < 10 {
		return 0.5
	}

	// Count how many values are on each side.
	var below, above int
	for _, v := range values {
		if v < value {
			below++
		} else if v > value {
			above++
		}
	}

	// The more isolated (fewer nearby values), the higher the score.
	minority := below
	if above < minority {
		minority = above
	}

	ratio := float64(minority) / float64(len(values))

	// Transform: if the point is at the extreme, ratio is near 0 -> score near 1.
	score := 1.0 - (ratio * 2)
	if score < 0 {
		score = 0
	}
	return score
}

// LinearTrend computes the slope of a linear regression over the values.
func LinearTrend(values []float64) float64 {
	n := float64(len(values))
	if n < 2 {
		return 0
	}

	var sumX, sumY, sumXY, sumX2 float64
	for i, y := range values {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return 0
	}

	return (n*sumXY - sumX*sumY) / denom
}
