package main

import "testing"

func Test_getReportFunctionName(t *testing.T) {
	type args struct {
		ifs uint64
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getReportFunctionName(tt.args.ifs); got != tt.want {
				t.Errorf("getReportFunctionName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_report(t *testing.T) {
	type args struct {
		ifs uint64
		pos int
		s   string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report(tt.args.ifs, tt.args.pos, tt.args.s)
		})
	}
}

func Test_appendToTestReport(t *testing.T) {
	type args struct {
		test_output_file string
		ifs              uint64
		pos              int
		s                string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appendToTestReport(tt.args.test_output_file, tt.args.ifs, tt.args.pos, tt.args.s)
		})
	}
}

func Test_version(t *testing.T) {
	tests := []struct {
		name string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version()
		})
	}
}

func Test_help(t *testing.T) {
	type args struct {
		hargs string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			help(tt.args.hargs)
		})
	}
}

func Test_commands(t *testing.T) {
	tests := []struct {
		name string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commands()
		})
	}
}
