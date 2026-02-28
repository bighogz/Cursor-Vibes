package trend

// QuarterTrend holds quarterly return and slope.
type QuarterTrend struct {
	QuarterPct float64
	QReturn    float64
	Slope      float64
	Last       float64
}

// FromCloses computes quarterly return and linear regression slope.
// Uses ~63-point lookback when available. Returns nil if < 30 valid points.
func FromCloses(closes []float64) *QuarterTrend {
	valid := make([]float64, 0, len(closes))
	for _, c := range closes {
		if c > 0 {
			valid = append(valid, c)
		}
	}
	if len(valid) < 30 {
		return nil
	}
	last := valid[len(valid)-1]
	lookback := 63
	if len(valid) <= 63 {
		lookback = len(valid) / 2
	}
	if lookback < 1 {
		lookback = 1
	}
	prev := valid[len(valid)-lookback]
	if prev <= 0 {
		return nil
	}
	qReturn := (last/prev - 1)
	quarterPct := qReturn * 100
	slope := linearSlope(valid[len(valid)-lookback:])
	return &QuarterTrend{quarterPct, qReturn, slope, last}
}

func linearSlope(y []float64) float64 {
	n := float64(len(y))
	if n < 2 {
		return 0
	}
	xMean := (n - 1) / 2
	var ySum float64
	for _, v := range y {
		ySum += v
	}
	yMean := ySum / n
	var num, den float64
	for i, yi := range y {
		xi := float64(i)
		num += (xi - xMean) * (yi - yMean)
		den += (xi - xMean) * (xi - xMean)
	}
	if den == 0 {
		return 0
	}
	return num / den
}
