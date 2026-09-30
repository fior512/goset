package generic

import (
	"fmt"
	"iter"
	"math/bits"
	"strconv"
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

func (set *CPUSet) apply(lo, hi int, op maskOp) {
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
	for word := loW; word <= hiW; word++ {
		if word == hiW {
			mask &= ^uint64(0) >> uint(63-(hi&63))
		}
		switch op {
		case opSet:
			set[word] |= mask
		case opClear:
			set[word] &^= mask // AND NOT
		case opInv:
			set[word] ^= mask
		}
		mask = ^uint64(0)
	}
}


func (set *CPUSet) SetBit(cpu int)   { set.apply(cpu, cpu, opSet) }
func (set *CPUSet) ClearBit(cpu int) { set.apply(cpu, cpu, opClear) }
func (set *CPUSet) InvBit(cpu int)   { set.apply(cpu, cpu, opInv) }

func (set *CPUSet) SetRange(lo, hi int)   { set.apply(lo, hi, opSet) }
func (set *CPUSet) ClearRange(lo, hi int) { set.apply(lo, hi, opClear) }
func (set *CPUSet) InvRange(lo, hi int)   { set.apply(lo, hi, opInv) }

func (set *CPUSet) GetBit(cpu int) bool {
	if cpu < 0 || cpu >= CPUSetBits { // GetBit is not inside apply
		return false
	}
	return set[cpu>>6]&(1<<uint(cpu&63)) != 0
}


func (set *CPUSet) Any() bool {
	for _, word := range set {
		if word != 0 {
			return true
		}
	}
	return false
}


func (set *CPUSet) Count() int {
	total := 0
	for _, word := range set {
		total += bits.OnesCount64(word)
	}
	return total
}


func (set *CPUSet) Max() int {
	for i := CPUSetWords - 1; i >= 0; i-- { // backward
		if set[i] != 0 {
			return (i<<6) + 63 - bits.LeadingZeros64(set[i])
		}
	}
	return -1 // empty
}


func (set *CPUSet) And(other CPUSet) {
	for i := range set {
		set[i] &= other[i]
	}
}


// Or unions two CPU sets
func (set *CPUSet) Or(other CPUSet) {
	for word := range set {
		set[word] |= other[word]
	}
}

func (set *CPUSet) AndNot(other CPUSet) {
	for i := range set {
		set[i] &^= other[i]
	}
}


func (set *CPUSet) IsSubset(super CPUSet) bool {
	for i := range set {
		if set[i]&^super[i] != 0 {
			return false
		}
	}
	return true
}


func (set *CPUSet) NextSet(from int) int {
	if from < 0 {
		from = 0
	}
	word := from >> 6
	if word >= CPUSetWords {
		return -1
	}
	chunk := set[word] &^ (1<<uint(from&63) - 1)
	for {
		if chunk != 0 {
			return word<<6 + bits.TrailingZeros64(chunk)
		}
		word++
		if word >= CPUSetWords {
			return -1
		}
		chunk = set[word]
	}
}


// NextSet but for `range`
func (set *CPUSet) All() iter.Seq[int] {
	return func(yield func(int) bool) {
		for cpu := set.NextSet(0); cpu >= 0; cpu = set.NextSet(cpu + 1) {
			if !yield(cpu) {
				return
			}
		}
	}
}


// String auto translate
func (set *CPUSet) String() string {
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
	for cpu := range set.All() {
		if lo >= 0 && cpu == hi+1 {
			hi = cpu
			continue
		}
		flush()
		lo, hi = cpu, cpu
	}
	flush()
	return strings.Join(parts, ",")
}


func (set *CPUSet) Set(str string) error {
	parsed, err := ParseCPUList(str)
	if err != nil {
		return err
	}
	*set = parsed
	return nil
}


// ParseCPUList translate strings to CPUSet
func ParseCPUList(list string) (CPUSet, error) {
	str := strings.TrimSpace(list)
	if str == "" {
		return CPUSet{}, nil
	}

	var out CPUSet
	for _, element := range strings.Split(str, ",") {
		if element = strings.TrimSpace(element); element == "" {
			continue
		}

		if lo, hi, ok := strings.Cut(element, "-"); ok {
			l, err := strconv.Atoi(strings.TrimSpace(lo))
			if err != nil {
				return CPUSet{}, err
			}
			h, err := strconv.Atoi(strings.TrimSpace(hi))
			if err != nil {
				return CPUSet{}, err
			}

			// ranges and gremlins
			for id := min(l, h); id <= max(l, h); id++ {
				out.SetBit(id)
			}
		} else {
			id, err := strconv.Atoi(element)
			if err != nil {
				return CPUSet{}, err
			}
			out.SetBit(id)
		}
	}
	return out, nil
}


func CPUListEqual(a, b string) bool {
	if a == b {
		return true
	}
	setA, errA := ParseCPUList(a)
	setB, errB := ParseCPUList(b)
	if errA != nil || errB != nil {
		return false
	}
	return setA.String() == setB.String()
}
