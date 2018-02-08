package proxy

import (
	"testing"

	"github.com/jookco/pgproxy.v1/pool"
)

func BenchmarkTest(b *testing.B) {
	_map := newWriteNodeMap()
	for i := 0; i < b.N; i++ {
		_map.Set("x", &pool.Pool{})
	}
}
