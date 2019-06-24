package offheap

import (
	"fmt"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStringbank(t *testing.T) {
	sb := Stringbank{}
	defer sb.Close()

	s1 := sb.Save("hello")
	s2 := sb.Save("goodbye")
	s3 := sb.Save("cheese")

	assert.Equal(t, "hello", sb.Get(s1))
	assert.Equal(t, "goodbye", sb.Get(s2))
	assert.Equal(t, "cheese", sb.Get(s3))
}

func TestCloseAbuse(t *testing.T) {
	sb := Stringbank{}
	defer sb.Close()

	s1 := sb.Save("hello")

	assert.Equal(t, "hello", sb.Get(s1))

	sb.Close()

	s1 = sb.Save("hello")
	assert.Equal(t, "hello", sb.Get(s1))
}

func TestStringbankSize(t *testing.T) {
	sb := Stringbank{}
	defer sb.Close()
	assert.Zero(t, sb.Size())
	sb.Save("hello")
	assert.Equal(t, stringbankSize, sb.Size())
}

func TestLengths(t *testing.T) {
	tests := []struct {
		len int
	}{
		{1},
		{127},
		{128},
		{254},
		{255},
		{256},
		{0xFFFFFFFFFF},
	}

	for _, test := range tests {
		t.Run(strconv.Itoa(test.len), func(t *testing.T) {
			buf := make([]byte, 10)

			l := writeLength(test.len, buf)
			assert.Equal(t, l, spaceForLength(test.len))
			len, lenlen := readLength(buf)
			assert.Equal(t, l, lenlen)
			assert.Equal(t, test.len, len)
		})
	}
}

func TestGC(t *testing.T) {
	sb := Stringbank{}
	defer sb.Close()
	for i := 0; i < 10000000; i++ {
		sb.Save(strconv.Itoa(i))
	}
	runtime.GC()

	start := time.Now()
	runtime.GC()
	assert.True(t, time.Since(start) < 1000*time.Microsecond)
	runtime.KeepAlive(sb)
}

func BenchmarkStringbank(b *testing.B) {
	s := make([]string, b.N)
	for i := range s {
		s[i] = strconv.Itoa(i)
	}

	index := make([]int, b.N)

	b.ReportAllocs()
	b.ResetTimer()
	sb := Stringbank{}
	defer sb.Close()
	for i, v := range s {
		index[i] = sb.Save(v)
	}

	var out string
	for _, i := range index {
		out = sb.Get(i)
	}
	if out != s[b.N-1] {
		b.Fatalf("final string should be %s, is %s", s[b.N-1], out)
	}
}

func ExampleStringbank() {
	sb := Stringbank{}
	defer sb.Close()
	i := sb.Save("goodbye")
	fmt.Println(sb.Get(i))
	// Output: goodbye
}
