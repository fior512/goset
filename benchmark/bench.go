package benchmark

import (
	"goset/benchmark/chase"
	"goset/benchmark/jitter"
	"goset/benchmark/matmul"
	"goset/benchmark/pingpong"
	"goset/benchmark/plugin"
	"goset/benchmark/stream"
)

var Registry = map[string]plugin.Benchmark{
	"stream":   &stream.Benchmark{},
	"jitter":   &jitter.Benchmark{},
	"matmul":   &matmul.Benchmark{},
	"chase":    &chase.Benchmark{},
	"pingpong": &pingpong.Benchmark{},
}
