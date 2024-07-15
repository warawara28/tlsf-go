package tlsf

import (
	"math/bits"
	"testing"
)

func TestRoundUp(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		size int64
		want int64
	}{
		{"roundUp(0)", 0, 0},
		{"roundUp(1)", 1, 16},
		{"roundUp(15)", 15, 16},
		{"roundUp(16)", 16, 16},
		{"roundUp(17)", 17, 32},
		{"roundUp(31)", 31, 32},
		{"roundUp(32)", 32, 32},
		{"roundUp(33)", 33, 48},
		{"roundUp(1024)", 1024, 1024},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := roundUp(tt.size); got != tt.want {
				t.Errorf("roundUp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRoundDown(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		size int64
		want int64
	}{
		{"roundDown(0)", 0, 0},
		{"roundDown(1)", 1, 0},
		{"roundDown(15)", 15, 0},
		{"roundDown(16)", 16, 16},
		{"roundDown(17)", 17, 16},
		{"roundDown(31)", 31, 16},
		{"roundDown(32)", 32, 32},
		{"roundDown(33)", 33, 32},
		{"roundDown(1024)", 1024, 1024},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := roundDown(tt.size); got != tt.want {
				t.Errorf("roundDown() = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestLSB(t *testing.T) {
	tests := []struct {
		input    int64
		expected int64
	}{
		{0, -1}, // special case
		{1, 0},
		{2, 1},
		{3, 0},
		{4, 2},
		{7, 0},
		{8, 3},
		{15, 0},
		{16, 4},
		{0xFF, 0},
		{0x100, 8},
		{0xFFFF, 0},
		{0x10000, 16},
		{0xFFFFFF, 0},
		{0x1000000, 24},
		{0xFFFFFFFF, 0},
	}

	for _, test := range tests {
		result := lsb(test.input)
		if result != test.expected {
			t.Errorf("lsb(%d) = %d; want %d", test.input, result, test.expected)
		}

		// Compare with the standard library implementation
		stdResult := int64(bits.TrailingZeros32(uint32(test.input)))
		if test.input != 0 && result != stdResult {
			t.Errorf("LsBit(%d) = %d; standard library returns %d", test.input, result, stdResult)
		}
	}
}

func TestMSB(t *testing.T) {
	tests := []struct {
		input    int64
		expected int64
	}{
		{0, -1}, // special case
		{1, 0},
		{2, 1},
		{3, 1},
		{4, 2},
		{7, 2},
		{8, 3},
		{15, 3},
		{16, 4},
		{0xFF, 7},
		{0x100, 8},
		{0xFFFF, 15},
		{0x10000, 16},
		{0xFFFFFF, 23},
		{0x1000000, 24},
		{0xFFFFFFFF, 31},
	}

	for _, test := range tests {
		result := msb(test.input)
		if result != test.expected {
			t.Errorf("msb(%d) = %d; want %d", test.input, result, test.expected)
		}

		// Compare with the standard library implementation
		stdResult := int64(bits.Len32(uint32(test.input)) - 1)
		if test.input != 0 && result != stdResult {
			t.Errorf("msb(%d) = %d; standard library returns %d", test.input, result, stdResult)
		}
	}
}

func TestDetermineLevels(t *testing.T) {
	tests := []struct {
		name   string
		size   int64
		wantFL int64
		wantSL int64
	}{
		{"Small size 64", 64, 0, 16},
		{"Exact SmallBlockSize", SmallBlockSize, 1, 0},
		{"Large size 256", 256, 2, 0},
		{"Large size 420", 420, 2, 20},
		{"Large size 460", 460, 2, 25},
		{"Large size 464", 464, 2, 26},
		{"Large size 500", 500, 2, 30},
		{"Large size 512", 512, 3, 0},
		{"Large size 1024", 1024, 4, 0},
		{"Large size 2048", 2048, 5, 0},
		{"Large size 32736", 32736, 8, 31},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotFL, gotSL int64
			determineLevels(tt.size, &gotFL, &gotSL)
			if gotFL != tt.wantFL {
				t.Errorf("determineLevels() gotFL = %v, want %v", gotFL, tt.wantFL)
			}
			if gotSL != tt.wantSL {
				t.Errorf("determineLevels() gotSL = %v, want %v", gotSL, tt.wantSL)
			}
		})
	}
}

func TestSelectLevelsAndSize(t *testing.T) {
	tests := []struct {
		name     string
		size     int64
		wantSize int64
		wantFL   int64
		wantSL   int64
	}{
		{"Small size 64", 64, 64, 0, 16},
		{"Exact SmallBlockSize", SmallBlockSize, SmallBlockSize, 1, 0},
		{"Large size 256", 256, 256, 2, 0},
		{"Large size 464", 464, 464, 2, 26},
		{"Large size 512", 512, 512, 3, 0},
		{"Large size 1024", 1024, 1024, 4, 0},
		{"Large size 2048", 2048, 2048, 5, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := tt.size
			var fl, sl int64
			selectLevelsAndSize(&size, &fl, &sl)
			if size != tt.wantSize {
				t.Errorf("selectLevelsAndSize() size = %v, want %v", size, tt.wantSize)
			}
			if fl != tt.wantFL {
				t.Errorf("selectLevelsAndSize() fl = %v, want %v", fl, tt.wantFL)
			}
			if sl != tt.wantSL {
				t.Errorf("selectLevelsAndSize() sl = %v, want %v", sl, tt.wantSL)
			}
		})
	}
}
