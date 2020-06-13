//+build !test

package main

import (
	"errors"
	"time"
)

func buildDateLib() {

	features["date"] = Feature{version: 1, category: "date"}
	categories["date"] = []string{"date", "epoch_time", "epoch_nano_time", "time_diff", "date_human"}

	slhelp["date"] = LibHelp{in: "integer", out: "string", action: "Returns a date/time string. The parsed timestamp is either the current date/time or the optional [#i1]integer[#i0]. RFC3339 format."}
	stdlib["date"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

		var t time.Time

		if len(args) == 1 {
			when, invalid := GetAsInt(args[0])
			if invalid {
				return nil, errors.New("Bad args (type) in date().")
			}
			t = time.Unix(int64(when), 0)
		} else {
			t = time.Now()
		}

		st := t.Format(time.RFC3339)
		return st, err
	}

	slhelp["date_human"] = LibHelp{in: "", out: "string", action: "Returns the current date and time in a readable format (RFC822Z)"}
	stdlib["date_human"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        return time.Now().Format(time.RFC822Z),nil
    }

	slhelp["epoch_time"] = LibHelp{in: "", out: "integer", action: "Returns the current epoch (Unix) time in seconds."}
	stdlib["epoch_time"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
		return int(time.Now().Unix()), err
	}

	slhelp["epoch_nano_time"] = LibHelp{in: "", out: "integer", action: "Returns the current epoch (Unix) time in nano-seconds."}
	stdlib["epoch_nano_time"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
		return int(time.Now().UnixNano()), err
	}

	slhelp["time_diff"] = LibHelp{in: "te,ts", out: "integer", action: "Returns difference in micro-seconds between two nano-epoch dates."}
	stdlib["time_diff"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

		if len(args) != 2 {
			return 0, errors.New("Bad args (count) for time_diff()")
		}
        a,ea:=GetAsInt64(args[0])
        b,eb:=GetAsInt64(args[1])

        if ea || eb {
			return 0, errors.New("Bad args (type) for time_diff()")
        }

		return float64(a-b) / 1000, nil

	}

}

