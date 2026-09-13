package cpu

import (
	"fmt"
	"sort"
)

// CPUScore score per Threads
type CPUScore struct {
	CPU          int
	Steerable    uint64
	NonSteerable uint64
	Total        uint64
	SiblingLoad  uint64
	Node         int
	KernalIsol   bool
	NohzFull     bool
}

// SelectionResult Benchmark Results
type SelectionResult struct {
	Benchmark   CPUSet
	HouseKeeper int
	Scores      []CPUScore
	SamplingMS  int
	Node        int
}

func SelectCPUs(t *Topology, n int, include CPUSet, exclude CPUSet, samplingMS int, preferNode bool) (*SelectionResult, error) {
	if n < 1 {
		return nil, fmt.Errorf("need at least 1 benchmark cpu")
	}

	candidates := t.Online
	candidates.AndNot(exclude) // exclude

	if !include.IsSubset(candidates) {
		bad := include
		bad.AndNot(candidates)
		return nil, fmt.Errorf("cpu %d is not a valid candidate", bad.NextSet(0))
	}

	bench := include
	if bench.Count() > n {
		return nil, fmt.Errorf("include set has %d cpus, only %d requested", bench.Count(), n)
	}

	remain := candidates
	remain.AndNot(bench)
	scores := scoreCandidates(t, remain)

	need := n - bench.Count()
	if need > 0 && preferNode {
		seedNode := -1
		if bench.Any() {
			seedNode = t.NumaNode[bench.NextSet(0)]
		}
		if picked := pickSameNode(scores, need, seedNode); picked != nil {
			for _, c := range picked {
				bench.SetBit(c)
			}
			need = 0
		}
	}
	if need > 0 {
		if len(scores) < need {
			return nil, fmt.Errorf("need %d more cpus, %d available", need, len(scores))
		}
		for i := 0; i < need; i++ {
			bench.SetBit(scores[i].CPU)
		}
	}

	node := -1
	if bench.Any() {
		node = t.NumaNode[bench.NextSet(0)]
		for c := range bench.All() {
			if t.NumaNode[c] != node {
				node = -1
				break
			}
		}
	}

	candidates.AndNot(bench)
	hk, err := pickHousekeeping(t, candidates, bench)
	if err != nil {
		return nil, err
	}

	return &SelectionResult{
		Benchmark:   bench,
		HouseKeeper: hk,
		Scores:      scores,
		SamplingMS:  samplingMS,
		Node:        node,
	}, nil
}

func scoreCandidates(t *Topology, cand CPUSet) []CPUScore {
	var scores []CPUScore
	for c := range cand.All() {
		scores = append(scores, CPUScore{
			CPU:        c,
			Node:       t.NumaNode[c],
			KernalIsol: t.KernalIsol[c],
			NohzFull:   t.NohzFull[c],
		})
	}
	sort.Slice(scores, func(i, j int) bool { return rank(scores[i], scores[j]) })
	return scores
}

func rank(a, b CPUScore) bool {
	if a.KernalIsol != b.KernalIsol {
		return a.KernalIsol
	}
	pa, pb := a.Total+a.SiblingLoad, b.Total+b.SiblingLoad
	if pa != pb {
		return pa < pb
	}
	return a.CPU < b.CPU
}

func pickSameNode(scores []CPUScore, need int, seedNode int) []int {
	byNode := map[int][]int{}
	nodeCost := map[int]uint64{}
	for _, s := range scores {
		if s.Node < 0 {
			continue
		}
		if seedNode >= 0 && s.Node != seedNode {
			continue
		}
		if len(byNode[s.Node]) < need {
			byNode[s.Node] = append(byNode[s.Node], s.CPU)
			nodeCost[s.Node] += s.Total + s.SiblingLoad
		}
	}
	var best []int
	bestNode := -1
	for node, cpus := range byNode {
		if len(cpus) < need {
			continue
		}
		if bestNode < 0 || nodeCost[node] < nodeCost[bestNode] ||
			(nodeCost[node] == nodeCost[bestNode] && node < bestNode) {
			best, bestNode = cpus, node
		}
	}
	return best
}

func pickHousekeeping(t *Topology, cand, bench CPUSet) (int, error) {
	for pass := 0; pass < 2; pass++ {
		for c := range cand.All() {
			if pass == 0 {
				shared := false
				siblings := t.Siblings[c]
				for s := range siblings.All() {
					if bench.GetBit(s) {
						shared = true
						break
					}
				}
				if shared {
					continue
				}
			}
			return c, nil
		}
	}
	return -1, fmt.Errorf("no cpu available for housekeeping")
}
