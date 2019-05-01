package offheap

import (
	"reflect"
	"syscall"
)

func alloc(eltsize uintptr, size int) (reflect.SliceHeader, error) {
	var errno syscall.Errno
	var slice reflect.SliceHeader
	fd := -1
	slice.Data, _, errno = syscall.Syscall6(
		syscall.SYS_MMAP,
		0, // address
		eltsize*uintptr(size),
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_ANON|syscall.MAP_PRIVATE,
		uintptr(fd), // No file descriptor
		0,           // offset
	)
	slice.Cap = size

	var err error
	if errno != 0 {
		err = errno
	}
	return slice, err
}

func free(slice reflect.SliceHeader, eltsize uintptr) error {
	_, _, errno := syscall.Syscall(syscall.SYS_MUNMAP, slice.Data, eltsize*uintptr(slice.Cap), 0)
	if errno != 0 {
		return errno
	}
	return nil
}
