// Package offheap is an off-heap implementation of stringbank. Memory to back the strings is allocated
// in chunks directly from the OS
package offheap

import (
	"encoding/binary"
	"fmt"
	"io"
	"iter"
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
	allocations []*[stringbankSize]byte
}

// Close releases resources associated with the StringBank
func (s *Stringbank) Close() error {
	for _, allocation := range s.allocations {
		if err := mmap.Free(allocation[:]); err != nil {
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
		return unsafe.String(&data[offset+1], l)
	}
	l, llen := readLength(data[offset:])
	return unsafe.String(&data[offset+llen], l)
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
		s.allocations = append(s.allocations, (*[stringbankSize]byte)(slice))
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

// All returns an iterator over all strings in the Stringbank
func (s *Stringbank) All() iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, allocation := range s.allocations {
			offset := 0
			for offset < len(allocation) {
				if allocation[offset] == 0 {
					break
				}
				var slen int
				var llen int
				if allocation[offset]&0x80 == 0 {
					slen = int(allocation[offset])
					llen = 1
				} else {
					slen, llen = readLength(allocation[offset:])
				}
				str := unsafe.String(&allocation[offset+llen], slen)
				if !yield(str) {
					return
				}
				offset += llen + slen
			}
		}
	}
}

const persistTag = "STRINGBANK_V1   "

// Persist writes the contents of the Stringbank to an io.Writer. The format is just a dump of the underlying
// byte slices, so it can be read back in with LoadStringbank
func (s *Stringbank) Persist(w io.Writer) error {
	if _, err := w.Write([]byte(persistTag)); err != nil {
		return err
	}

	count := int64(len(s.allocations))
	if err := binary.Write(w, binary.NativeEndian, &count); err != nil {
		return err
	}

	for _, allocation := range s.allocations {
		if _, err := w.Write(allocation[:]); err != nil {
			return fmt.Errorf("writing stringbank data: %w", err)
		}
	}
	return nil
}

// LoadStringbank reads a Stringbank from an io.Reader, and returns a new
// Stringbank with the same contents. The format is just a dump of the
// underlying byte slices, so it can be written with Persist.
func (s *Stringbank) Load(r io.Reader) error {
	tag := make([]byte, len(persistTag))
	if _, err := io.ReadFull(r, tag); err != nil {
		return err
	}
	if string(tag) != persistTag {
		return fmt.Errorf("invalid persist tag: %s", string(tag))
	}

	var count int64
	if err := binary.Read(r, binary.NativeEndian, &count); err != nil {
		return fmt.Errorf("reading allocation count: %w", err)
	}

	allocations := make([]*[stringbankSize]byte, 0, count)
	for range count {
		slice, err := mmap.Alloc[byte](stringbankSize)
		if err != nil {
			return fmt.Errorf("allocating memory: %w", err)
		}

		if _, err := io.ReadFull(r, slice[:]); err != nil {
			mmap.Free(slice[:])
			return fmt.Errorf("reading stringbank data: %w", err)
		}
		allocations = append(allocations, (*[stringbankSize]byte)(slice))
	}

	if len(allocations) == 0 {
		return nil
	}

	current := allocations[len(allocations)-1][:]
	// find the end of the last string in the current allocation
	offset := 0
	for offset < len(current) {
		if current[offset] == 0 {
			break
		}
		var slen int
		var llen int
		if current[offset]&0x80 == 0 {
			slen = int(current[offset])
			llen = 1
		} else {
			slen, llen = readLength(current[offset:])
		}
		offset += llen + slen
	}

	s.allocations = allocations
	s.current = current[:offset]

	return nil
}
