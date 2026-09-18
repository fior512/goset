package benchmark

import (
	"goset/benchmark/plugin"
	"goset/benchmark/stream"
)

var Registry = map[string]plugin.Benchmark{
	"stream": &stream.Benchmark{},
}
