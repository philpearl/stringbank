/*
Package stringbank allows you to hold large numbers of strings without bothering the
garbage collector. For small strings storage is reduced as the lengths are encoded compactly.
*/
package stringbank

import (
	"math/bits"
	"unsafe"
)

const stringbankSize = 1 << 18 // about 250k as a power of 2

var packageBank Stringbank

// Index is returned by Save and can be converted into the saved string by calling it's String() method
type Index int

func (i Index) String() string {
	return packageBank.Get(int(i))
}

// Save stores the string in the default package-level Stringbank, and returns an Index which can be converted back to the original string by calling its String() method
func Save(val string) Index {
	return Index(packageBank.Save(val))
}

// Stringbank is a place to put strings that never need to be deleted. Saving a string into the Stringbank
// returns an integer offset for the string, so the string can be stored and referenced without bothering the
// garbage collector. The offset can be exchanged for the original string via a call to Get
type Stringbank struct {
	current     []byte
	allocations [][]byte
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
	l, llen := readLength(data[offset:])

	b := data[offset+llen : offset+llen+l]
	return *(*string)(unsafe.Pointer(&b))
}

// Save copies a string into the Stringbank, and returns the index of the string in the bank
func (s *Stringbank) Save(tocopy string) int {
	l := len(tocopy)
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
		s.current = make([]byte, 0, stringbankSize)
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
	for i := uint(0); ; i++ {
		val := buf[i]
		total += int(val&0x7F) << (7 * i)
		if val&0x80 == 0 {
			return total, int(i + 1)
		}
	}
}
