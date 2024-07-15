/* This program is free software. It comes without any warranty, to
 * the extent permitted by applicable law. You can redistribute it
 * and/or modify it under the terms of the Do What The Fuck You Want
 * To Public License, Version 2, as published by Sam Hocevar. See
 * http://sam.zoy.org/wtfpl/COPYING for more details. */

// Package tlsf implements a Two-Level Segregated Fit memory allocator.
//
// IMPORTANT: This package is NOT goroutine-safe.
// Concurrent access from multiple goroutines is not supported and may lead to race conditions.
// It is the responsibility of the caller to implement proper synchronization mechanisms
// when using this allocator in a concurrent environment.
package tlsf

import (
	"arena"
	"errors"
	"unsafe"
)

// Arena defines the interface for a memory allocation arena.
// Implementations of this interface are expected to manage memory allocation and deallocation.
type Arena interface {
	// Allocate allocates a block of memory with the specified size.
	// It returns an unsafe.Pointer to the allocated memory and an error if the allocation fails.
	//
	// Parameters:
	//   - size: The size of the memory block to allocate, in bytes.
	//
	// Returns:
	//   - unsafe.Pointer: A pointer to the allocated memory block.
	//   - error: An error if the allocation fails, or nil if successful.
	Allocate(size int64) (unsafe.Pointer, error)

	// Free deallocates the memory block pointed to by ptr.
	// The behavior is undefined if ptr is not a pointer returned by Allocate,
	// or if it has already been freed.
	//
	// Parameters:
	//   - ptr: A pointer to the memory block to be freed.
	Free(ptr unsafe.Pointer)

	// Dispose releases all resources associated with the Arena.
	// After calling Dispose, the Arena should not be used anymore.
	Dispose()

	// UsedSize returns the total amount of block size (NOT allocation size) currently allocated by the Arena.
	//
	// Returns:
	//   - int64: The total size of allocated memory in bytes.
	UsedSize() int64
}

// TLSFArena represents a TLSF memory allocator.
//
// WARNING: This type is NOT goroutine-safe.
type TLSFArena struct {
	arena *arena.Arena

	// First-level bitmap
	flBitmap uint32

	// Second-level bitmap
	slBitmap [RealFLI]uint32

	// Managing head pointers of block headers
	// matrix is a two-dimensional array managing head pointers of block headers.
	// It organizes free blocks by their size classes.
	// Each element is a pointer to a FreeBlockHeader, which is the head of a free block list.
	matrix [RealFLI][MaxSLI]*FreeBlockHeader

	// Base pointer of memory allocation source (&bytes[0])
	head uintptr

	usedSize int64
}

// BlockHeader .
type BlockHeader struct {
	prevHeader *BlockHeader
	blockSize  blockStatus
}

// UsedBlockHeader .
type UsedBlockHeader struct {
	BlockHeader
	ptr uintptr // actual data
}

// FreeBlockHeader .
type FreeBlockHeader struct {
	BlockHeader
	prev *FreeBlockHeader // previous unused block header
	next *FreeBlockHeader // next unused block header
}

// ErrBlockNotFound is returned when the allocator fails to find or allocate a suitable memory block.
// This can occur when there's not enough free memory or when a block of the requested size is unavailable.
var ErrBlockNotFound = errors.New("failed to allocate block")

// getPtr returns a pointer to the usable memory area of the free block.
// It calculates the address by adding the size of the block header to the start of the FreeBlockHeader.
// This method uses unsafe operations for direct memory manipulation.
//
// Returns:
//   - unsafe.Pointer: Pointer to the beginning of the usable memory area in the block.
//
//go:inline
func (bf *FreeBlockHeader) getPtr() unsafe.Pointer {
	return unsafe.Add(unsafe.Pointer(bf), BlockHeaderSize)
}

//go:inline
func (bf *BlockHeader) setBlockStatus(bs blockStatus) {
	bf.blockSize = bf.blockSize | bs
}

//go:inline
func (bf *BlockHeader) getBlockSize() int64 {
	return int64(bf.blockSize & BlockSize)
}

//go:inline
func (bf *BlockHeader) isFree() bool {
	return (bf.blockSize & FreeBlock) == FreeBlock
}

//go:inline
func (bf *BlockHeader) isPreviousBlockFree() bool {
	return (bf.blockSize & PreviousBlockFree) == PreviousBlockFree
}

// NewArena creates a new TLSF memory allocator with the specified allocation size.
func NewArena(allocationBytes uint32) Arena {

	// 1. Initialize the source Arena and allocate memory
	a := arena.NewArena()
	tlsf := arena.New[TLSFArena](a)
	bytes := arena.MakeSlice[byte](a, int(allocationBytes), int(allocationBytes))
	tlsf.arena = a
	tlsf.head = uintptr(unsafe.Pointer(&bytes[0]))

	// 2. Create the initial block
	b := (*FreeBlockHeader)(unsafe.Pointer(tlsf.head))

	// Initial block size = Total size - 32 bytes (initial block header + last block header)
	b.blockSize = roundDown(int64(allocationBytes) - (2 * BlockHeaderSize))
	b.setBlockStatus(PreviousBlockUsed | FreeBlock)

	// 3. initialize the last block
	lb := (*BlockHeader)(unsafe.Add(b.getPtr(), b.getBlockSize()))
	lb.setBlockStatus(PreviousBlockFree | UsedBlock)
	lb.prevHeader = (*BlockHeader)(unsafe.Pointer(b))

	// 4. initialize the first block
	tlsf.Free(b.getPtr())

	tlsf.usedSize = int64(allocationBytes) - b.getBlockSize()

	return tlsf
}

// Allocate allocates a block of memory with the specified size.
// It returns an unsafe.Pointer to the allocated memory and an error if the allocation fails.
//
// Parameters:
//   - size: The size of the memory block to allocate, in bytes.
//
// Returns:
//   - unsafe.Pointer: A pointer to the allocated memory block.
//   - error: An error if the allocation fails, or nil if successful.
func (t *TLSFArena) Allocate(size int64) (unsafe.Pointer, error) {
	// Round up to 16 bytes if less than 16,
	// otherwise round up to the nearest multiple of 16 bytes.
	if size < (MinBlockSize) {
		size = MinBlockSize
	} else {
		size = roundUp(size)
	}

	// search free block
	var fl, sl int64
	selectLevelsAndSize(&size, &fl, &sl)

	b := t.findSuitableBlock(&fl, &sl)
	if b == nil {
		return nil, ErrBlockNotFound
	}

	t.extractBlockHdr(b, fl, sl)

	// use the free block
	nb := (*BlockHeader)(unsafe.Add(b.getPtr(), b.getBlockSize()))
	tmpSize := b.getBlockSize() - size
	const blockHeaderSize = unsafe.Sizeof(BlockHeader{}) // 16byte

	if tmpSize >= int64(blockHeaderSize) {
		// divide the block
		tmpSize -= BlockHeaderSize

		b2 := (*FreeBlockHeader)(unsafe.Add(b.getPtr(), size))
		b2.blockSize = tmpSize
		b2.setBlockStatus(PreviousBlockUsed | FreeBlock)
		nb.prevHeader = (*BlockHeader)(unsafe.Pointer(b2))

		determineLevels(tmpSize, &fl, &sl)
		t.insertBlock(b2, fl, sl)

		// extract the previous block status and set it
		b.blockSize = size | (b.blockSize & 0x2)
	} else {
		// no need to divide the block
		nb.blockSize &= (^PreviousBlockFree)
		b.blockSize &= (^FreeBlock)
	}

	t.addSize(b)

	return b.getPtr(), nil
}

// Free deallocates the memory block pointed to by ptr.
// The behavior is undefined if ptr is not a pointer returned by Allocate,
// or if it has already been freed.
//
// Parameters:
//   - ptr: A pointer to the memory block to be freed.
func (t *TLSFArena) Free(ptr unsafe.Pointer) {
	b := (*FreeBlockHeader)(unsafe.Pointer(uintptr(ptr) - BlockHeaderSize))
	b.setBlockStatus(FreeBlock)

	t.removeSize(b)

	b.prev = nil
	b.next = nil

	var fl, sl int64

	nb := (*BlockHeader)(unsafe.Add(unsafe.Pointer(b), b.getBlockSize()))
	if nb.isFree() {
		determineLevels(nb.getBlockSize(), &fl, &sl)
		nfb := (*FreeBlockHeader)(unsafe.Pointer(nb))
		t.extractBlock(nfb, fl, sl)
		b.blockSize += nb.getBlockSize() + BlockHeaderSize
	}
	if nb.isPreviousBlockFree() {
		pfb := (*FreeBlockHeader)(unsafe.Pointer(nb.prevHeader))
		determineLevels(pfb.getBlockSize(), &fl, &sl)
		t.extractBlock(pfb, fl, sl)
		pfb.blockSize += b.getBlockSize() + BlockHeaderSize
		b = pfb
	}
	determineLevels(b.getBlockSize(), &fl, &sl)
	t.insertBlock(b, fl, sl)
}

// Dispose releases all resources associated with the Arena.
func (t *TLSFArena) Dispose() {
	t.arena.Free()
	t.arena = nil
}

// UsedSize returns the total amount of block size (NOT allocation size) currently allocated by the Arena.
func (t *TLSFArena) UsedSize() int64 {
	return t.usedSize
}

// addSize adds the size of the block to the used size.
// go:inline
func (t *TLSFArena) addSize(b *FreeBlockHeader) {
	t.usedSize += b.getBlockSize() + BlockHeaderSize
}

// removeSize removes the size of the block from the used size.
// go:inline
func (t *TLSFArena) removeSize(b *FreeBlockHeader) {
	t.usedSize -= b.getBlockSize() + BlockHeaderSize
}

// go:inline
func (t *TLSFArena) extractBlockHdr(b *FreeBlockHeader, fl, sl int64) {
	t.matrix[fl][sl] = b.next
	if t.matrix[fl][sl] != nil {
		t.matrix[fl][sl].prev = nil
	} else {
		clearBit(sl, &t.slBitmap[fl])
		if t.slBitmap[fl] == 0 {
			clearBit(fl, &t.flBitmap)
		}
	}
	b.prev = nil
	b.next = nil
}

// go:inline
func (t *TLSFArena) insertBlock(b *FreeBlockHeader, fl, sl int64) {
	b.prev = nil
	b.prev = t.matrix[fl][sl]
	if t.matrix[fl][sl] != nil {
		t.matrix[fl][sl].prev = b
	}
	t.matrix[fl][sl] = b

	setBit(sl, &t.slBitmap[fl])
	setBit(fl, &t.flBitmap)
}

// go:inline
func (t *TLSFArena) extractBlock(b *FreeBlockHeader, fl, sl int64) {
	if b.next != nil {
		b.next.prev = b.prev
	}
	if b.prev != nil {
		b.prev.next = b.next
	}
	if t.matrix[fl][sl] == b {
		t.matrix[fl][sl] = b.next
		if t.matrix[fl][sl] != nil {
			clearBit(sl, &t.slBitmap[fl])
			if t.slBitmap[fl] != 0 {
				clearBit(fl, &t.flBitmap)
			}
		}
	}
	b.prev = nil
	b.next = nil
}
