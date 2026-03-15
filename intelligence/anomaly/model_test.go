package anomaly

import (
	"math"
	"testing"
)

func TestTimeSeriesWindow_AddAndLen(t *testing.T) {
	w := NewTimeSeriesWindow(5)
	if w.Len() != 0 {
		t.Fatalf("expected len 0, got %d", w.Len())
	}

	w.Add(1.0)
	w.Add(2.0)
	if w.Len() != 2 {
		t.Fatalf("expected len 2, got %d", w.Len())
	}
}

func TestTimeSeriesWindow_Values(t *testing.T) {
	w := NewTimeSeriesWindow(5)
	for i := 1.0; i <= 3; i++ {
		w.Add(i)
	}
	vals := w.Values()
	if len(vals) != 3 {
		t.Fatalf("expected 3 values, got %d", len(vals))
	}
	for i, want := range []float64{1, 2, 3} {
		if vals[i] != want {
			t.Errorf("vals[%d] = %f, want %f", i, vals[i], want)
		}
	}
}

func TestTimeSeriesWindow_Wraparound(t *testing.T) {
	w := NewTimeSeriesWindow(3)
	for i := 1.0; i <= 5; i++ {
		w.Add(i)
	}
	if w.Len() != 3 {
		t.Fatalf("expected len 3 after wraparound, got %d", w.Len())
	}
	vals := w.Values()
	// After wrapping: should contain 3, 4, 5 in insertion order
	for i, want := range []float64{3, 4, 5} {
		if vals[i] != want {
			t.Errorf("vals[%d] = %f, want %f", i, vals[i], want)
		}
	}
}

func TestMean(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		want   float64
	}{
		{"empty", nil, 0},
		{"single", []float64{5}, 5},
		{"normal", []float64{1, 2, 3, 4, 5}, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Mean(tt.values)
			if got != tt.want {
				t.Errorf("Mean(%v) = %f, want %f", tt.values, got, tt.want)
			}
		})
	}
}

func TestStdDev(t *testing.T) {
	// All same values → 0 deviation.
	sd := StdDev([]float64{5, 5, 5, 5})
	if sd != 0 {
		t.Errorf("StdDev of constant values = %f, want 0", sd)
	}

	// Known values.
	sd = StdDev([]float64{2, 4, 4, 4, 5, 5, 7, 9})
	if math.Abs(sd-2.138) > 0.01 {
		t.Errorf("StdDev = %f, want ~2.138", sd)
	}

	// Less than 2 → 0.
	if StdDev([]float64{1}) != 0 {
		t.Error("StdDev of single value should be 0")
	}
}

func TestZScore(t *testing.T) {
	values := []float64{10, 10, 10, 10, 10}
	// Zero stddev → return 0.
	z := ZScore(100, values)
	if z != 0 {
		t.Errorf("ZScore with zero stddev = %f, want 0", z)
	}

	values = []float64{1, 2, 3, 4, 5}
	z = ZScore(3, values)
	if math.Abs(z) > 0.01 {
		t.Errorf("ZScore of mean value = %f, want ~0", z)
	}

	z = ZScore(100, values)
	if z <= 0 {
		t.Errorf("ZScore of extreme value should be positive, got %f", z)
	}
}

func TestPercentile(t *testing.T) {
	if Percentile(nil, 50) != 0 {
		t.Error("Percentile of empty should be 0")
	}

	values := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	p50 := Percentile(values, 50)
	if math.Abs(p50-5.5) > 0.1 {
		t.Errorf("P50 = %f, want ~5.5", p50)
	}

	// Single element.
	p := Percentile([]float64{42}, 95)
	if p != 42 {
		t.Errorf("Percentile of single element = %f, want 42", p)
	}
}

func TestEWMA(t *testing.T) {
	e := NewEWMA(0.5)

	// First value initializes.
	e.Add(10)
	if e.Value() != 10 {
		t.Errorf("EWMA init = %f, want 10", e.Value())
	}

	// Second value: 0.5*20 + 0.5*10 = 15.
	e.Add(20)
	if e.Value() != 15 {
		t.Errorf("EWMA after second = %f, want 15", e.Value())
	}
}

func TestEWMA_Alpha1(t *testing.T) {
	// Alpha=1 means EWMA always equals latest value.
	e := NewEWMA(1.0)
	e.Add(10)
	e.Add(20)
	if e.Value() != 20 {
		t.Errorf("EWMA(alpha=1) = %f, want 20", e.Value())
	}
}

func TestIsolationScore(t *testing.T) {
	// Less than 10 values → 0.5.
	score := IsolationScore(5, []float64{1, 2, 3})
	if score != 0.5 {
		t.Errorf("IsolationScore with few values = %f, want 0.5", score)
	}

	// Normal value in the middle should have lower score than extreme.
	values := make([]float64, 100)
	for i := range values {
		values[i] = float64(i)
	}
	scoreMid := IsolationScore(50, values)
	scoreExtreme := IsolationScore(99, values)
	if scoreExtreme <= scoreMid {
		t.Errorf("extreme score (%f) should be > mid score (%f)", scoreExtreme, scoreMid)
	}
}

func TestLinearTrend(t *testing.T) {
	// Single value → 0.
	if LinearTrend([]float64{5}) != 0 {
		t.Error("LinearTrend of single value should be 0")
	}

	// Upward trend: 1, 2, 3, 4, 5 → slope should be ~1.
	slope := LinearTrend([]float64{1, 2, 3, 4, 5})
	if math.Abs(slope-1.0) > 0.01 {
		t.Errorf("LinearTrend upward = %f, want ~1.0", slope)
	}

	// Flat: all same → slope 0.
	slope = LinearTrend([]float64{5, 5, 5, 5})
	if math.Abs(slope) > 0.01 {
		t.Errorf("LinearTrend flat = %f, want ~0", slope)
	}
}
