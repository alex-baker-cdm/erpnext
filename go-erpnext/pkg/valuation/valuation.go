// Package valuation provides FIFO and LIFO stock valuation methods.
//
// This is a Go port of erpnext/stock/valuation.py.
package valuation

import (
	"math"

	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/frappe"
)

// StockBin represents a [qty, rate] pair in the valuation queue/stack.
type StockBin struct {
	Qty  float64
	Rate float64
}

// BinWiseValuation is the interface for bin-based stock valuation methods.
type BinWiseValuation interface {
	AddStock(qty, rate float64)
	RemoveStock(qty, outgoingRate float64, rateGenerator func() float64) []StockBin
	State() []StockBin
	GetTotalStockAndValue() (float64, float64)
}

// RoundOffIfNearZero rounds the number to zero if it is within 1/10^precision
// of zero. Precision defaults to 7.
func RoundOffIfNearZero(number float64, precision int) float64 {
	if precision <= 0 {
		precision = 7
	}
	if math.Abs(0.0-frappe.Flt(number, -1)) < (1.0 / math.Pow(10, float64(precision))) {
		return 0.0
	}
	return frappe.Flt(number, -1)
}

// --- FIFO Valuation ---

// FIFOValuation implements BinWiseValuation using a FIFO queue.
// New stock is added at the end of the queue.
// Qty consumption happens on a First In First Out basis.
type FIFOValuation struct {
	queue []StockBin
}

// NewFIFOValuation creates a new FIFOValuation with the given initial state.
// If state is nil, an empty queue is used.
func NewFIFOValuation(state []StockBin) *FIFOValuation {
	if state == nil {
		state = []StockBin{}
	}
	// Copy to avoid aliasing
	q := make([]StockBin, len(state))
	copy(q, state)
	return &FIFOValuation{queue: q}
}

// State returns the current state of the FIFO queue.
func (f *FIFOValuation) State() []StockBin {
	return f.queue
}

// AddStock adds new stock to the FIFO queue.
func (f *FIFOValuation) AddStock(qty, rate float64) {
	if len(f.queue) == 0 {
		f.queue = append(f.queue, StockBin{0, 0})
	}

	last := &f.queue[len(f.queue)-1]

	// last row has the same rate, merge new bin
	if last.Rate == rate {
		last.Qty += qty
	} else {
		// Item has a positive balance qty, add new entry
		if last.Qty > 0 {
			f.queue = append(f.queue, StockBin{qty, rate})
		} else {
			// negative balance qty
			newQty := last.Qty + qty
			if newQty > 0 {
				// new balance qty is positive
				last.Qty = newQty
				last.Rate = rate
			} else {
				// new balance qty is still negative, maintain same rate
				last.Qty = newQty
			}
		}
	}
}

// RemoveStock removes stock from the FIFO queue and returns consumed bins.
// If rateGenerator is nil, a default function returning 0.0 is used.
func (f *FIFOValuation) RemoveStock(qty, outgoingRate float64, rateGenerator func() float64) []StockBin {
	if rateGenerator == nil {
		rateGenerator = func() float64 { return 0.0 }
	}

	var consumedBins []StockBin

	for qty != 0 {
		if len(f.queue) == 0 {
			// rely on rate generator
			f.queue = append(f.queue, StockBin{0, rateGenerator()})
		}

		index := 0
		if outgoingRate > 0 {
			// Find the entry where rate matches outgoing rate
			found := false
			for idx, fifoBin := range f.queue {
				if fifoBin.Rate == outgoingRate {
					index = idx
					found = true
					break
				}
			}
			// If no entry found with outgoing rate, consume as per FIFO
			if !found {
				index = 0
			}
		}

		// select first bin or the bin with same rate
		fifoBin := &f.queue[index]
		if qty >= fifoBin.Qty {
			// consume current bin
			qty = RoundOffIfNearZero(qty-fifoBin.Qty, 7)
			toConsume := *fifoBin
			// pop the bin at index
			f.queue = append(f.queue[:index], f.queue[index+1:]...)
			consumedBins = append(consumedBins, toConsume)

			if len(f.queue) == 0 && qty != 0 {
				// stock finished, qty still remains to be withdrawn
				// negative stock, keep as a negative bin
				rate := outgoingRate
				if rate == 0 {
					rate = toConsume.Rate
				}
				f.queue = append(f.queue, StockBin{-qty, rate})
				consumedBins = append(consumedBins, StockBin{qty, rate})
				break
			}
		} else {
			// qty found in current bin, consume it and exit
			fifoBin.Qty = RoundOffIfNearZero(fifoBin.Qty-qty, 7)
			consumedBins = append(consumedBins, StockBin{qty, fifoBin.Rate})
			qty = 0
		}
	}

	return consumedBins
}

// GetTotalStockAndValue returns the total quantity and total value across all bins.
func (f *FIFOValuation) GetTotalStockAndValue() (float64, float64) {
	return getTotalStockAndValue(f.queue)
}

// --- LIFO Valuation ---

// LIFOValuation implements BinWiseValuation using a LIFO stack.
// New stock is added at the top of the stack.
// Qty consumption happens on a Last In First Out basis.
type LIFOValuation struct {
	stack []StockBin
}

// NewLIFOValuation creates a new LIFOValuation with the given initial state.
// If state is nil, an empty stack is used.
func NewLIFOValuation(state []StockBin) *LIFOValuation {
	if state == nil {
		state = []StockBin{}
	}
	// Copy to avoid aliasing
	s := make([]StockBin, len(state))
	copy(s, state)
	return &LIFOValuation{stack: s}
}

// State returns the current state of the LIFO stack.
func (l *LIFOValuation) State() []StockBin {
	return l.stack
}

// AddStock adds new stock to the LIFO stack.
// Behaviour is the same as FIFO valuation.
func (l *LIFOValuation) AddStock(qty, rate float64) {
	if len(l.stack) == 0 {
		l.stack = append(l.stack, StockBin{0, 0})
	}

	last := &l.stack[len(l.stack)-1]

	// last row has the same rate, merge new bin
	if last.Rate == rate {
		last.Qty += qty
	} else {
		// Item has a positive balance qty, add new entry
		if last.Qty > 0 {
			l.stack = append(l.stack, StockBin{qty, rate})
		} else {
			// negative balance qty
			newQty := last.Qty + qty
			if newQty > 0 {
				// new balance qty is positive
				last.Qty = newQty
				last.Rate = rate
			} else {
				// new balance qty is still negative, maintain same rate
				last.Qty = newQty
			}
		}
	}
}

// RemoveStock removes stock from the LIFO stack and returns consumed bins.
// outgoingRate is ignored for LIFO (kept for interface compatibility).
// If rateGenerator is nil, a default function returning 0.0 is used.
func (l *LIFOValuation) RemoveStock(qty, outgoingRate float64, rateGenerator func() float64) []StockBin {
	if rateGenerator == nil {
		rateGenerator = func() float64 { return 0.0 }
	}

	var consumedBins []StockBin

	for qty != 0 {
		if len(l.stack) == 0 {
			// rely on rate generator
			l.stack = append(l.stack, StockBin{0, rateGenerator()})
		}

		// start at the end (LIFO)
		index := len(l.stack) - 1

		stockBin := &l.stack[index]
		if qty >= stockBin.Qty {
			// consume current bin
			qty = RoundOffIfNearZero(qty-stockBin.Qty, 7)
			toConsume := *stockBin
			// pop from end
			l.stack = l.stack[:index]
			consumedBins = append(consumedBins, toConsume)

			if len(l.stack) == 0 && qty != 0 {
				// stock finished, qty still remains to be withdrawn
				// negative stock, keep as a negative bin
				rate := outgoingRate
				if rate == 0 {
					rate = toConsume.Rate
				}
				l.stack = append(l.stack, StockBin{-qty, rate})
				consumedBins = append(consumedBins, StockBin{qty, rate})
				break
			}
		} else {
			// qty found in current bin, consume it and exit
			stockBin.Qty = RoundOffIfNearZero(stockBin.Qty-qty, 7)
			consumedBins = append(consumedBins, StockBin{qty, stockBin.Rate})
			qty = 0
		}
	}

	return consumedBins
}

// GetTotalStockAndValue returns the total quantity and total value across all bins.
func (l *LIFOValuation) GetTotalStockAndValue() (float64, float64) {
	return getTotalStockAndValue(l.stack)
}

// --- shared helpers ---

func getTotalStockAndValue(bins []StockBin) (float64, float64) {
	totalQty := 0.0
	totalValue := 0.0
	for _, bin := range bins {
		totalQty += frappe.Flt(bin.Qty, -1)
		totalValue += frappe.Flt(bin.Qty, -1) * frappe.Flt(bin.Rate, -1)
	}
	return RoundOffIfNearZero(totalQty, 7), RoundOffIfNearZero(totalValue, 7)
}
