package main

import (
	"reflect"
	"sync"
	"testing"
)

func Test_nlmcreate(t *testing.T) {
	type args struct {
		sz int
	}
	tests := []struct {
		name string
		args args
		want *Nmap
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nlmcreate(tt.args.sz); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("nlmcreate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNmap_lmset(t *testing.T) {
	type fields struct {
		RWMutex sync.RWMutex
		nmap    map[uint64]string
	}
	type args struct {
		k uint64
		v string
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
			u := &Nmap{
				RWMutex: tt.fields.RWMutex,
				nmap:    tt.fields.nmap,
			}
			u.lmset(tt.args.k, tt.args.v)
		})
	}
}

func TestNmap_lmget(t *testing.T) {
	type fields struct {
		RWMutex sync.RWMutex
		nmap    map[uint64]string
	}
	type args struct {
		k uint64
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
		want1  bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &Nmap{
				RWMutex: tt.fields.RWMutex,
				nmap:    tt.fields.nmap,
			}
			got, got1 := u.lmget(tt.args.k)
			if got != tt.want {
				t.Errorf("Nmap.lmget() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("Nmap.lmget() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestNmap_lmdelete(t *testing.T) {
	type fields struct {
		RWMutex sync.RWMutex
		nmap    map[uint64]string
	}
	type args struct {
		k uint64
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
			u := &Nmap{
				RWMutex: tt.fields.RWMutex,
				nmap:    tt.fields.nmap,
			}
			if got := u.lmdelete(tt.args.k); got != tt.want {
				t.Errorf("Nmap.lmdelete() = %v, want %v", got, tt.want)
			}
		})
	}
}
