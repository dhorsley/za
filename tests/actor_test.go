// actor_test.go
package main

import (
	"math/big"
	"testing"
)

func TestStrcmp(t *testing.T) {
	if !strcmp("", "") {
		t.Fatalf("strcmp(\"\",\"\") expected true")
	}
	if strcmp("foo", "bar") {
		t.Fatalf("strcmp(\"foo\",\"bar\") expected false")
	}
	if !strcmp("abcdef", "abcdef") {
		t.Fatalf("strcmp identical strings expected true")
	}
	if strcmp("a", "ab") {
		t.Fatalf("strcmp(\"a\",\"ab\") expected false")
	}
}

func TestGetAsStringAndBigInt(t *testing.T) {
	bi := big.NewInt(12345)
	s := GetAsString(bi)
	if s != "12345" {
		t.Fatalf("GetAsString(*big.Int) expected \"12345\", got %q", s)
	}

	bf := big.NewFloat(3.14)
	sf := GetAsString(bf)
	if sf != bf.String() {
		t.Fatalf("GetAsString(*big.Float) got %q, expected %q", sf, bf.String())
	}

	s2 := GetAsString(99)
	if s2 != "99" {
		t.Fatalf("GetAsString(int) expected \"99\", got %q", s2)
	}
}

func TestGetAsUintAndInSlice(t *testing.T) {
	// float → uint
	if u, err := GetAsUint(2.0); err || u != 2 {
		t.Fatalf("GetAsUint(2.0) expected (2,false), got (%v,%v)", u, err)
	}
	// string → uint
	if u, err := GetAsUint("10"); err || u != 10 {
		t.Fatalf("GetAsUint(\"10\") expected (10,false), got (%v,%v)", u, err)
	}
	// invalid → error
	if _, err := GetAsUint([]int{1, 2, 3}); !err {
		t.Fatalf("GetAsUint([]int) expected error, got no error")
	}

	list := []int64{5, 10, 15}
	if !InSlice(10, list) {
		t.Fatalf("InSlice(10,[5,10,15]) expected true")
	}
	if InSlice(7, list) {
		t.Fatalf("InSlice(7,[5,10,15]) expected false")
	}
}

