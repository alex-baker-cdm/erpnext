package valuation

import (
	"math"
	"testing"

	"pgregory.net/rapid"
)

// tHelper is satisfied by both *testing.T and *rapid.T
type tHelper interface {
	Helper()
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
}

// --- helpers ---

func assertTotalQty(t tHelper, bins []StockBin, expected float64) {
	t.Helper()
	total := 0.0
	for _, b := range bins {
		total += b.Qty
	}
	if math.Abs(total-expected) > 1e-4 {
		t.Fatalf("expected total qty %v, got %v (bins: %v)", expected, total, bins)
	}
}

func assertTotalValue(t tHelper, bins []StockBin, expected float64) {
	t.Helper()
	total := 0.0
	for _, b := range bins {
		total += b.Qty * b.Rate
	}
	if math.Abs(total-expected) > 1e-2 {
		t.Fatalf("expected total value %v, got %v (bins: %v)", expected, total, bins)
	}
}

func assertState(t tHelper, bins []StockBin, expected []StockBin) {
	t.Helper()
	if len(bins) != len(expected) {
		t.Fatalf("expected state %v, got %v", expected, bins)
	}
	for i := range bins {
		if bins[i].Qty != expected[i].Qty || bins[i].Rate != expected[i].Rate {
			t.Fatalf("expected state %v, got %v", expected, bins)
		}
	}
}

func assertConsumed(t tHelper, consumed []StockBin, expected []StockBin) {
	t.Helper()
	if len(consumed) != len(expected) {
		t.Fatalf("expected consumed %v, got %v", expected, consumed)
	}
	for i := range consumed {
		if consumed[i].Qty != expected[i].Qty || consumed[i].Rate != expected[i].Rate {
			t.Fatalf("expected consumed %v, got %v", expected, consumed)
		}
	}
}

// =====================
// FIFO Tests
// =====================

func TestFIFO_SimpleAddition(t *testing.T) {
	q := NewFIFOValuation(nil)
	q.AddStock(1, 10)
	assertTotalQty(t, q.State(), 1)
}

func TestFIFO_SimpleRemoval(t *testing.T) {
	q := NewFIFOValuation(nil)
	q.AddStock(1, 10)
	q.RemoveStock(1, 0, nil)
	assertTotalQty(t, q.State(), 0)
}

func TestFIFO_MergeNewStock(t *testing.T) {
	q := NewFIFOValuation(nil)
	q.AddStock(1, 10)
	q.AddStock(1, 10)
	assertState(t, q.State(), []StockBin{{2, 10}})
}

func TestFIFO_AddingNegativeStockKeepsRate(t *testing.T) {
	q := NewFIFOValuation([]StockBin{{-5.0, 100}})
	q.AddStock(1, 10)
	assertState(t, q.State(), []StockBin{{-4, 100}})
}

func TestFIFO_AddingNegativeStockUpdatesRate(t *testing.T) {
	q := NewFIFOValuation([]StockBin{{-5.0, 100}})
	q.AddStock(6, 10)
	assertState(t, q.State(), []StockBin{{1, 10}})
}

func TestFIFO_NegativeStock(t *testing.T) {
	q := NewFIFOValuation(nil)
	q.RemoveStock(1, 5, nil)
	assertState(t, q.State(), []StockBin{{-1, 5}})

	q.RemoveStock(1, 0, nil)
	assertTotalQty(t, q.State(), -2)
	assertState(t, q.State(), []StockBin{{-2, 5}})

	q.AddStock(2, 10)
	assertTotalQty(t, q.State(), 0)
	assertTotalValue(t, q.State(), 0)
}

func TestFIFO_RemovingSpecifiedRate(t *testing.T) {
	q := NewFIFOValuation(nil)
	q.AddStock(1, 10)
	q.AddStock(1, 20)

	q.RemoveStock(1, 20, nil)
	assertState(t, q.State(), []StockBin{{1, 10}})
}

func TestFIFO_RemoveMultipleBins(t *testing.T) {
	q := NewFIFOValuation(nil)
	q.AddStock(1, 10)
	q.AddStock(2, 20)
	q.AddStock(1, 20)
	q.AddStock(5, 20)

	q.RemoveStock(4, 0, nil)
	assertState(t, q.State(), []StockBin{{5, 20}})
}

func TestFIFO_RemoveMultipleBinsWithRate(t *testing.T) {
	q := NewFIFOValuation(nil)
	q.AddStock(1, 10)
	q.AddStock(2, 20)
	q.AddStock(1, 20)
	q.AddStock(5, 20)

	q.RemoveStock(3, 20, nil)
	assertState(t, q.State(), []StockBin{{1, 10}, {5, 20}})
}

func TestFIFO_QueueWithUnknownRate(t *testing.T) {
	q := NewFIFOValuation(nil)
	q.AddStock(1, 1)
	q.AddStock(1, 2)
	q.AddStock(1, 3)
	q.AddStock(1, 4)

	assertTotalValue(t, q.State(), 10)

	q.RemoveStock(3, 1, nil)
	assertState(t, q.State(), []StockBin{{1, 4}})
}

func TestFIFO_RoundingOff(t *testing.T) {
	q := NewFIFOValuation(nil)
	q.AddStock(1.0, 1.0)
	q.RemoveStock(1.0-1e-9, 0, nil)
	assertTotalQty(t, q.State(), 0)
}

func TestFIFO_RoundingOffNearZero(t *testing.T) {
	if RoundOffIfNearZero(0, 7) != 0 {
		t.Error("round_off_if_near_zero(0) should be 0")
	}
	if RoundOffIfNearZero(1, 7) != 1 {
		t.Error("round_off_if_near_zero(1) should be 1")
	}
	if RoundOffIfNearZero(-1, 7) != -1 {
		t.Error("round_off_if_near_zero(-1) should be -1")
	}
	if RoundOffIfNearZero(-1e-8, 7) != 0 {
		t.Error("round_off_if_near_zero(-1e-8) should be 0")
	}
	if RoundOffIfNearZero(1e-8, 7) != 0 {
		t.Error("round_off_if_near_zero(1e-8) should be 0")
	}
}

func TestFIFO_Totals(t *testing.T) {
	q := NewFIFOValuation(nil)
	q.AddStock(1, 10)
	q.AddStock(2, 13)
	q.AddStock(1, 17)
	q.RemoveStock(1, 0, nil)
	q.RemoveStock(1, 0, nil)
	q.RemoveStock(1, 0, nil)
	q.AddStock(5, 17)
	q.AddStock(8, 11)
	// Just ensure no panic; the Python test doesn't assert values either.
	totalQty, totalValue := q.GetTotalStockAndValue()
	if totalQty == 0 && totalValue == 0 {
		// This would be suspicious; the queue should have stock.
		// But mirroring Python test which just runs without assertion.
	}
}

// =====================
// LIFO Tests
// =====================

func TestLIFO_SimpleAddition(t *testing.T) {
	s := NewLIFOValuation(nil)
	s.AddStock(1, 10)
	assertTotalQty(t, s.State(), 1)
}

func TestLIFO_MergeNewStock(t *testing.T) {
	s := NewLIFOValuation(nil)
	s.AddStock(1, 10)
	s.AddStock(1, 10)
	assertState(t, s.State(), []StockBin{{2, 10}})
}

func TestLIFO_SimpleRemoval(t *testing.T) {
	s := NewLIFOValuation(nil)
	s.AddStock(1, 10)
	s.RemoveStock(1, 0, nil)
	assertTotalQty(t, s.State(), 0)
}

func TestLIFO_AddingNegativeStockKeepsRate(t *testing.T) {
	s := NewLIFOValuation([]StockBin{{-5.0, 100}})
	s.AddStock(1, 10)
	assertState(t, s.State(), []StockBin{{-4, 100}})
}

func TestLIFO_AddingNegativeStockUpdatesRate(t *testing.T) {
	s := NewLIFOValuation([]StockBin{{-5.0, 100}})
	s.AddStock(6, 10)
	assertState(t, s.State(), []StockBin{{1, 10}})
}

func TestLIFO_RoundingOff(t *testing.T) {
	s := NewLIFOValuation(nil)
	s.AddStock(1.0, 1.0)
	s.RemoveStock(1.0-1e-9, 0, nil)
	assertTotalQty(t, s.State(), 0)
}

func TestLIFO_Consumption(t *testing.T) {
	s := NewLIFOValuation(nil)
	s.AddStock(10, 10)
	s.AddStock(10, 20)
	consumed := s.RemoveStock(15, 0, nil)
	assertConsumed(t, consumed, []StockBin{{10, 20}, {5, 10}})
	assertTotalQty(t, s.State(), 5)
}

func TestLIFO_ConsumptionGoingNegative(t *testing.T) {
	s := NewLIFOValuation(nil)
	s.AddStock(10, 10)
	s.AddStock(10, 20)
	consumed := s.RemoveStock(25, 0, nil)
	assertConsumed(t, consumed, []StockBin{{10, 20}, {10, 10}, {5, 10}})
	assertTotalQty(t, s.State(), -5)
}

func TestLIFO_ConsumptionMultiple(t *testing.T) {
	s := NewLIFOValuation(nil)
	s.AddStock(1, 1)
	s.AddStock(2, 2)
	consumed := s.RemoveStock(1, 0, nil)
	assertConsumed(t, consumed, []StockBin{{1, 2}})

	s.AddStock(3, 3)
	consumed = s.RemoveStock(4, 0, nil)
	assertConsumed(t, consumed, []StockBin{{3, 3}, {1, 2}})

	s.AddStock(4, 4)
	consumed = s.RemoveStock(5, 0, nil)
	assertConsumed(t, consumed, []StockBin{{4, 4}, {1, 1}})

	s.AddStock(5, 5)
	consumed = s.RemoveStock(5, 0, nil)
	assertConsumed(t, consumed, []StockBin{{5, 5}})
}

// =====================
// Property-based tests (rapid)
// =====================

func TestFIFO_QtyHypothesis(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		q := NewFIFOValuation(nil)
		totalQty := 0.0

		for i := 0; i < 10; i++ {
			qty := rapid.Float64Range(-1e6, 1e6).Draw(t, "qty")
			rate := rapid.Float64Range(1, 1e6).Draw(t, "rate")

			if RoundOffIfNearZero(qty, 7) == 0 {
				continue
			}
			if qty > 0 {
				q.AddStock(qty, rate)
				totalQty += qty
			} else {
				absQty := math.Abs(qty)
				consumed := q.RemoveStock(absQty, 0, nil)
				consumedQty := 0.0
				for _, c := range consumed {
					consumedQty += c.Qty
				}
				if math.Abs(consumedQty-absQty) > 1e-4 {
					t.Fatalf("incorrect consumption: expected %v, got %v from %v", absQty, consumedQty, consumed)
				}
				totalQty -= absQty
			}
			assertTotalQty(t, q.State(), totalQty)
		}
	})
}

func TestFIFO_QtyValueNonNegHypothesis(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		q := NewFIFOValuation(nil)
		totalQty := 0.0
		totalValue := 0.0

		for i := 0; i < 10; i++ {
			qty := rapid.Float64Range(-1e6, 1e6).Draw(t, "qty")
			rate := rapid.Float64Range(1, 1e6).Draw(t, "rate")

			if RoundOffIfNearZero(qty, 7) == 0 || totalQty+qty < 0 || math.Abs(qty) < 0.1 {
				continue
			}
			if qty > 0 {
				q.AddStock(qty, rate)
				totalQty += qty
				totalValue += qty * rate
			} else {
				absQty := math.Abs(qty)
				consumed := q.RemoveStock(absQty, 0, nil)
				consumedQty := 0.0
				for _, c := range consumed {
					consumedQty += c.Qty
				}
				if math.Abs(consumedQty-absQty) > 1e-4 {
					t.Fatalf("incorrect consumption: expected %v, got %v", absQty, consumedQty)
				}
				totalQty -= absQty
				for _, c := range consumed {
					totalValue -= c.Qty * c.Rate
				}
			}
			assertTotalQty(t, q.State(), totalQty)
			assertTotalValue(t, q.State(), totalValue)
		}
	})
}

func TestLIFO_QtyHypothesis(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := NewLIFOValuation(nil)
		totalQty := 0.0

		for i := 0; i < 10; i++ {
			qty := rapid.Float64Range(-1e6, 1e6).Draw(t, "qty")
			rate := rapid.Float64Range(1, 1e6).Draw(t, "rate")

			if RoundOffIfNearZero(qty, 7) == 0 {
				continue
			}
			if qty > 0 {
				s.AddStock(qty, rate)
				totalQty += qty
			} else {
				absQty := math.Abs(qty)
				consumed := s.RemoveStock(absQty, 0, nil)
				consumedQty := 0.0
				for _, c := range consumed {
					consumedQty += c.Qty
				}
				if math.Abs(consumedQty-absQty) > 1e-4 {
					t.Fatalf("incorrect consumption: expected %v, got %v from %v", absQty, consumedQty, consumed)
				}
				totalQty -= absQty
			}
			assertTotalQty(t, s.State(), totalQty)
		}
	})
}

func TestLIFO_QtyValueNonNegHypothesis(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := NewLIFOValuation(nil)
		totalQty := 0.0
		totalValue := 0.0

		for i := 0; i < 10; i++ {
			qty := rapid.Float64Range(-1e6, 1e6).Draw(t, "qty")
			rate := rapid.Float64Range(1, 1e6).Draw(t, "rate")

			if RoundOffIfNearZero(qty, 7) == 0 || totalQty+qty < 0 || math.Abs(qty) < 0.1 {
				continue
			}
			if qty > 0 {
				s.AddStock(qty, rate)
				totalQty += qty
				totalValue += qty * rate
			} else {
				absQty := math.Abs(qty)
				consumed := s.RemoveStock(absQty, 0, nil)
				consumedQty := 0.0
				for _, c := range consumed {
					consumedQty += c.Qty
				}
				if math.Abs(consumedQty-absQty) > 1e-4 {
					t.Fatalf("incorrect consumption: expected %v, got %v", absQty, consumedQty)
				}
				totalQty -= absQty
				for _, c := range consumed {
					totalValue -= c.Qty * c.Rate
				}
			}
			assertTotalQty(t, s.State(), totalQty)
			assertTotalValue(t, s.State(), totalValue)
		}
	})
}
