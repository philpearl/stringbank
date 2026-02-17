package stringbank

import (
	"fmt"
	"runtime"
	"strconv"
	"testing"
	"time"
)

func TestStringbank(t *testing.T) {
	sb := Stringbank{}

	s1 := sb.Save("hello")
	s2 := sb.Save("goodbye")
	s3 := sb.Save("cheese")

	if got := sb.Get(s1); got != "hello" {
		t.Errorf("expected %q, got %q", "hello", got)
	}
	if got := sb.Get(s2); got != "goodbye" {
		t.Errorf("expected %q, got %q", "goodbye", got)
	}
	if got := sb.Get(s3); got != "cheese" {
		t.Errorf("expected %q, got %q", "cheese", got)
	}
}

func TestStringbankSize(t *testing.T) {
	sb := Stringbank{}
	if sb.Size() != 0 {
		t.Errorf("expected 0, got %d", sb.Size())
	}
	sb.Save("hello")
	if sb.Size() != stringbankSize {
		t.Errorf("expected %d, got %d", stringbankSize, sb.Size())
	}
}

func TestPackageBank(t *testing.T) {
	s1 := Save("hello")
	s2 := Save("goodbye")
	s3 := Save("cheese")

	if s1.String() != "hello" {
		t.Errorf("expected %q, got %q", "hello", s1.String())
	}
	if s2.String() != "goodbye" {
		t.Errorf("expected %q, got %q", "goodbye", s2.String())
	}
	if s3.String() != "cheese" {
		t.Errorf("expected %q, got %q", "cheese", s3.String())
	}
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
			if l != spaceForLength(test.len) {
				t.Errorf("expected %d, got %d", spaceForLength(test.len), l)
			}
			length, lenlen := readLength(buf)
			if lenlen != l {
				t.Errorf("expected %d, got %d", l, lenlen)
			}
			if length != test.len {
				t.Errorf("expected %d, got %d", test.len, length)
			}
		})
	}
}

func TestGC(t *testing.T) {
	sb := Stringbank{}
	for i := range 10000000 {
		sb.Save(strconv.Itoa(i))
	}
	runtime.GC()

	start := time.Now()
	runtime.GC()
	if elapsed := time.Since(start); elapsed >= 1000*time.Microsecond {
		t.Errorf("GC took too long: %v", elapsed)
	}
	runtime.KeepAlive(sb)

	i := 0
	for s := range sb.All() {
		if s != strconv.Itoa(i) {
			t.Fatalf("expected %s, got %s", strconv.Itoa(i), s)
		}
		i++
	}
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

func ExampleSave() {
	i := Save("hello")
	fmt.Println(i)
	// Output: hello
}

func ExampleStringbank() {
	sb := Stringbank{}
	i := sb.Save("goodbye")
	fmt.Println(sb.Get(i))
	// Output: goodbye
}
