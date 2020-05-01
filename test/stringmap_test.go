package main

import (
	"reflect"
	"sync"
	"testing"
)

func Test_lmcreate(t *testing.T) {
	type args struct {
		sz int
	}
	tests := []struct {
		name string
		args args
		want *Lmap
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lmcreate(tt.args.sz); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("lmcreate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLmap_lmset(t *testing.T) {
	type fields struct {
		RWMutex sync.RWMutex
		smap    map[string]uint64
	}
	type args struct {
		k string
		v uint64
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &Lmap{
				RWMutex: tt.fields.RWMutex,
				smap:    tt.fields.smap,
			}
			u.lmset(tt.args.k, tt.args.v)
		})
	}
}

func TestLmap_lmget(t *testing.T) {
	type fields struct {
		RWMutex sync.RWMutex
		smap    map[string]uint64
	}
	type args struct {
		k string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   uint64
		want1  bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &Lmap{
				RWMutex: tt.fields.RWMutex,
				smap:    tt.fields.smap,
			}
			got, got1 := u.lmget(tt.args.k)
			if got != tt.want {
				t.Errorf("Lmap.lmget() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("Lmap.lmget() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestLmap_lmdelete(t *testing.T) {
	type fields struct {
		RWMutex sync.RWMutex
		smap    map[string]uint64
	}
	type args struct {
		k string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &Lmap{
				RWMutex: tt.fields.RWMutex,
				smap:    tt.fields.smap,
			}
			if got := u.lmdelete(tt.args.k); got != tt.want {
				t.Errorf("Lmap.lmdelete() = %v, want %v", got, tt.want)
			}
		})
	}
}
