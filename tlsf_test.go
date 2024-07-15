package tlsf

import (
	"arena"
	"fmt"
	"testing"
)

var testCases = []int{
	1_024,   /*1KB*/
	10_240,  /*10KB*/
	102_400, /*100KB*/
	512_000, /*500KB*/
}

const ArenaSize = 1_024 * 1000 * 1000 /*1000MB*/

func BenchmarkSlice_NoArena(b *testing.B) {
	for _, testBytes := range testCases {
		b.Run(fmt.Sprintf("testBytes=%d", testBytes), func(b *testing.B) {
			for range b.N {
				_ = make([]byte, testBytes, testBytes)
			}
		})
	}
}

func BenchmarkSlice_Arena(b *testing.B) {
	for _, testBytes := range testCases {
		b.Run(fmt.Sprintf("testBytes=%d", testBytes), func(b *testing.B) {
			a := arena.NewArena()
			for range b.N {
				_ = arena.MakeSlice[byte](a, testBytes, testBytes)
			}
			defer a.Free()
		})
	}
}

func BenchmarkSlice_TLSFArena(b *testing.B) {
	for _, testBytes := range testCases {
		b.Run(fmt.Sprintf("testBytes=%d", testBytes), func(b *testing.B) {
			a := NewArena(ArenaSize)
			for range b.N {
				ptr, _ := a.Allocate(int64(testBytes))
				a.Free(ptr)
			}
			defer a.Dispose()
		})
	}
}

func Example() {
	const bytes32KB uint32 = 32 * 1024
	arena := NewArena(bytes32KB)

	ptr, err := arena.Allocate(460)
	if err != nil {
		panic(err)
	}

	fmt.Printf("used_size: %d byte", arena.UsedSize())

	arena.Free(ptr)

	arena.Dispose()
	// Output: used_size: 512 byte
}
