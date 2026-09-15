package cpu

import (
	"fmt"
	"iter"
	"math/bits"
	"strings"
)

const (
	CPUSetWords = 16
	CPUSetBits  = CPUSetWords * 64
)

// mirrors the kernel cpu_set_t layout
type (
	CPUSet [CPUSetWords]uint64
	maskOp uint8
)

const (
	opSet maskOp = iota
	opClear
	opInv
)

func (M *CPUSet) apply(lo, hi int, op maskOp) {
	if lo > hi {
		lo, hi = hi, lo
	}
	if hi < 0 || lo >= CPUSetBits {
		return // fully out of range: no-op
	}
	lo = max(lo, 0)
	hi = min(hi, CPUSetBits-1)
	loW, hiW := lo>>6, hi>>6
	mask := ^uint64(0) << uint(lo&63)
	for w := loW; w <= hiW; w++ {
		if w == hiW {
			mask &= ^uint64(0) >> uint(63-(hi&63))
		}
		switch op {
		case opSet:
			M[w] |= mask
		case opClear:
			M[w] &^= mask // AND NOT
		case opInv:
			M[w] ^= mask
		}
		mask = ^uint64(0)
	}
}


func (M *CPUSet) SetBit(cpu int)   { M.apply(cpu, cpu, opSet) }
func (M *CPUSet) ClearBit(cpu int) { M.apply(cpu, cpu, opClear) }
func (M *CPUSet) InvBit(cpu int)   { M.apply(cpu, cpu, opInv) }

func (M *CPUSet) SetRange(lo, hi int)   { M.apply(lo, hi, opSet) }
func (M *CPUSet) ClearRange(lo, hi int) { M.apply(lo, hi, opClear) }
func (M *CPUSet) InvRange(lo, hi int)   { M.apply(lo, hi, opInv) }

func (M *CPUSet) GetBit(cpu int) bool {
	if cpu < 0 || cpu >= CPUSetBits { //GetBit is not inside apply
		return false
	}
	return M[cpu>>6]&(1<<uint(cpu&63)) != 0
}


func (M *CPUSet) Any() bool {
	for _, w := range M {
		if w != 0 {
			return true
		}
	}
	return false
}


func (M *CPUSet) Count() int {
	n := 0
	for _, w := range M {
		n += bits.OnesCount64(w)
	}
	return n
}


func (M *CPUSet) And(other CPUSet) {
	for i := range M {
		M[i] &= other[i]
	}
}


func (M *CPUSet) AndNot(other CPUSet) {
	for i := range M {
		M[i] &^= other[i]
	}
}


func (M *CPUSet) IsSubset(super CPUSet) bool {
	for i := range M {
		if M[i]&^super[i] != 0 {
			return false
		}
	}
	return true
}


func (M *CPUSet) NextSet(from int) int {
	if from < 0 {
		from = 0
	}
	w := from >> 6
	if w >= CPUSetWords {
		return -1
	}
	word := M[w] &^ (1<<uint(from&63) - 1)
	for {
		if word != 0 {
			return w<<6 + bits.TrailingZeros64(word)
		}
		w++
		if w >= CPUSetWords {
			return -1
		}
		word = M[w]
	}
}


// NextSet but for `range`
func (M *CPUSet) All() iter.Seq[int] {
	return func(yield func(int) bool) {
		for c := M.NextSet(0); c >= 0; c = M.NextSet(c + 1) {
			if !yield(c) {
				return
			}
		}
	}
}


// String auto translate
func (M *CPUSet) String() string {
	var parts []string
	lo, hi := -1, -1
	flush := func() {
		if lo < 0 {
			return
		}
		if lo == hi {
			parts = append(parts, fmt.Sprintf("%d", lo))
		} else {
			parts = append(parts, fmt.Sprintf("%d-%d", lo, hi))
		}
	}
	for c := range M.All() {
		if lo >= 0 && c == hi+1 {
			hi = c
			continue
		}
		flush()
		lo, hi = c, c
	}
	flush()
	return strings.Join(parts, ",")
}


func (M *CPUSet) Set(s string) error {
	parsed, err := ParseCPUList(s)
	if err != nil {
		return err
	}
	*M = parsed
	return nil
}
