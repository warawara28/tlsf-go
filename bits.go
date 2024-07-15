package tlsf

const (
	MaxFLI     int64 = 30
	MaxLog2SLI int64 = 5
	MaxSLI     int64 = 1 << MaxLog2SLI // 32

	FLIOffset int64 = 6
	RealFLI   int64 = MaxFLI - FLIOffset // 30 - 6 = 24 bits
)

// Bit flags representing the block usage state
// If the 0th bit of the block size is 1, the block is unused; if 0, the block is in use
// If the 1st bit of the block size is 1, the previous block is unused; if 0, the previous block is in use
type blockStatus = int64

const (
	// FreeBlock is a block that is not in use.
	FreeBlock int64 = 0x1
	// UsedBlock is able to be used.
	UsedBlock int64 = 0x0

	// Bit flags representing the state of the previous block
	PreviousBlockFree int64 = 0x1 << 1
	PreviousBlockUsed int64 = 0x0 << 1

	// Used to align block size to multiples of 8
	PtrMask   = 7
	BlockSize = 0xFFFFFFFF - PtrMask

	// Needed to store two pointers (one pointer is 8 bytes) in a 64-bit environment
	BlockAlign int64 = 16
	MemAlign   int64 = BlockAlign - 1

	// Overhead of BlockHeader
	// unsafe.Sizeof(BlockHeader)
	BlockHeaderSize = 16

	// Minimum block size
	// unsafe.Sizeof(FreeBlockHeader) - unsafe.sizeof(BlockHeader)
	MinBlockSize = 16

	// Threshold for small block size
	SmallBlockSize int64 = 128
)

var table = [256]int64{
	-1, 0, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 3, 3, 3, 3, 4, 4, 4, 4, 4, 4, 4, 4, 4,
	4, 4, 4, 4, 4, 4, 4,
	5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5,
	5, 5, 5, 5, 5, 5, 5,
	6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6,
	6, 6, 6, 6, 6, 6, 6,
	6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6,
	6, 6, 6, 6, 6, 6, 6,
	7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7,
}

// roundUp rounds up the given size to the nearest multiple of 16
func roundUp(size int64) int64 {
	return (size + MemAlign) & ^MemAlign
}

// roundDown rounds down the given size to the nearest multiple of 16
func roundDown(size int64) int64 {
	return size & ^MemAlign
}

// msb returns the most significant bit.
// go:inline
func msb(n int64) int64 {
	x := uint32(n)

	var a uint32
	if x <= 0xffff {
		if x <= 0xff {
			a = 0
		} else {
			a = 8
		}
	} else {
		if x <= 0xffffff {
			a = 16
		} else {
			a = 24
		}
	}

	return table[x>>a] + int64(a)
}

// lsb returns the least significant bit.
// go:inline
func lsb(n int64) int64 {
	x := uint32(n) & -uint32(n)

	var a uint32
	if x <= 0xffff {
		if x <= 0xff {
			a = 0
		} else {
			a = 8
		}
	} else {
		if x <= 0xffffff {
			a = 16
		} else {
			a = 24
		}
	}

	return table[x>>a] + int64(a)
}

// setBit sets a specific bit in a uint32 value to 1.
// go:inline
func setBit(nr int64, addr *uint32) {
	*addr |= 1 << uint(nr&0x1f)
}

// clearBit clears (sets to 0) a specific bit in a uint32 value.
// go:inline
func clearBit(nr int64, addr *uint32) {
	*addr &^= 1 << (nr & 0x1f)
}

// determineLevels determines the levels of the block with given size.
// go:inline
func determineLevels(size int64, fl, sl *int64) {
	if size < SmallBlockSize {
		*fl = 0
		*sl = size / (SmallBlockSize / MaxSLI)
	} else {
		*fl = msb(size)
		*sl = (size >> (*fl - MaxLog2SLI)) - MaxSLI
		*fl -= FLIOffset
	}
}

// selectLevelsAndSize selects levels and size of the block.
// The size parameter must be a multiple of 16.
// go:inline
func selectLevelsAndSize(size *int64, fl, sl *int64) {
	if *size < SmallBlockSize {
		*fl = 0
		*sl = *size / (SmallBlockSize / MaxSLI)
	} else {
		var t int64 = (1 << (msb(*size) - MaxLog2SLI)) - 1
		*size = *size + t
		*fl = msb(*size)
		*sl = (*size >> (*fl - MaxLog2SLI)) - MaxSLI
		*fl -= FLIOffset
		*size &= ^t
	}
}

// findSuitableBlock finds a suitable block for the given fl and sl.
// go:inline
func (t *TLSFArena) findSuitableBlock(fl, sl *int64) *FreeBlockHeader {
	var b *FreeBlockHeader

	// Extract all bits from slBitmap[fl] that are at or above the sl position
	var tmp int64 = int64(t.slBitmap[*fl]) & (^int64(0) << *sl)
	if tmp != 0 {
		// found a block, so we can use it
		*sl = lsb(tmp)
		b = t.matrix[*fl][*sl]
	} else {
		// If block is not found,
		// extract bits from flBitmap that are above fl,
		// and set the new fl to the position of the lowest set bit
		*fl = lsb(int64(t.flBitmap) & (^int64(0) << (*fl + 1)))
		if *fl > 0 {
			*sl = lsb(int64(t.slBitmap[*fl]))
			b = t.matrix[*fl][*sl]
		}
	}
	return b
}
