// Package offheap is an off-heap implementation of stringbank. Memory to back the strings is allocated
// in chunks directly from the OS
package offheap

import (
	"math/bits"
	"unsafe"

	"github.com/philpearl/mmap"
)

const stringbankSize = 1 << 18 // about 250k as a power of 2

// Stringbank is a place to put strings that never need to be deleted. Saving a string into the Stringbank
// returns an integer offset for the string, so the string can be stored and referenced without bothering the
// garbage collector. The offset can be exchanged for the original string via a call to Get
type Stringbank struct {
	current     []byte
	allocations [][]byte
}

// Close releases resources associated with the StringBank
func (s *Stringbank) Close() error {
	for _, allocation := range s.allocations {
		if err := mmap.Free(allocation); err != nil {
			return err
		}
	}
	s.allocations = nil
	s.current = nil
	return nil
}

// Size returns the approximate number of bytes in the string bank. The estimate includes currently unused and
// wasted space
func (s *Stringbank) Size() int {
	return len(s.allocations) * stringbankSize
}

// Get converts an index to the original string
func (s *Stringbank) Get(index int) string {
	// read the length and string from the data
	data := s.allocations[index/stringbankSize]
	offset := index % stringbankSize
	if l := data[offset]; l&0x80 == 0 {
		b := data[offset+1 : offset+1+int(l)]
		return *(*string)(unsafe.Pointer(&b))

	}
	l, llen := readLength(data[offset:])
	b := data[offset+llen : offset+llen+l]
	return *(*string)(unsafe.Pointer(&b))
}

// Save copies a string into the Stringbank, and returns the index of the string in the bank
func (s *Stringbank) Save(tocopy string) int {
	l := len(tocopy)
	if l <= 0x7F {
		// fast-track easy case
		offset, buf := s.reserve(l + 1)
		// write length
		buf[0] = byte(l)
		// write data
		copy(buf[1:], tocopy)
		return offset
	}
	offset, buf := s.reserve(l + spaceForLength(l))
	// Write the length
	start := writeLength(l, buf)

	// Write the data
	copy(buf[start:], tocopy)
	return offset
}

// reserve finds a contiguous space of length l that can be used for writing data
func (s *Stringbank) reserve(l int) (index int, data []byte) {
	if len(s.current)+l > cap(s.current) {
		slice, _ := mmap.Alloc[byte](stringbankSize)
		s.current = slice[:0]
		s.allocations = append(s.allocations, s.current[0:stringbankSize])
	}
	offset := len(s.current)
	s.current = s.current[:offset+l]
	return (len(s.allocations)-1)*stringbankSize + offset, s.current[offset:]
}

func spaceForLength(len int) int {
	// 7 bits => 1 byte
	// 8 bits => 2 byte
	// 1
	bits := bits.Len(uint(len))
	return (bits + 6) / 7
}

func writeLength(len int, buf []byte) int {
	// Want to write the length in a compact manner, with the assumption that short lengths
	// are much more common
	remainder := len
	var i int
	for i = 0; remainder != 0; i++ {
		val := byte(remainder & 0x7F)
		remainder = remainder >> 7
		if remainder != 0 {
			val |= 0x80
		}
		buf[i] = val
	}
	return i
}

func readLength(buf []byte) (int, int) {
	total := 0
	for i, val := range buf {
		total += int(val&0x7F) << (7 * uint(i))
		if val&0x80 == 0 {
			return total, int(i + 1)
		}
	}
	// Shouldn't get here as the buffer should always be big enough
	panic("read length overrun")
}
