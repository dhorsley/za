//+build !test

package main

import (
	"errors"
//	"reflect"
	"time"
)

func buildDateLib() {

	features["date"] = Feature{version: 1, category: "date"}
	categories["date"] = []string{"date", "epoch_time", "epoch_nano_time", "time_diff"}

	slhelp["date"] = LibHelp{in: "epoch_time", out: "string", action: "Returns a human-readable date/time. The parsed timestamp is either the current date/time or the optional [#i1]epoch_time[#i0] integer."}
	stdlib["date"] = func(args ...interface{}) (ret interface{}, err error) {

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

	slhelp["epoch_time"] = LibHelp{in: "", out: "integer", action: "Returns the current epoch (Unix) time in seconds."}
	stdlib["epoch_time"] = func(args ...interface{}) (ret interface{}, err error) {
		return int(time.Now().Unix()), err
	}

	slhelp["epoch_nano_time"] = LibHelp{in: "", out: "integer", action: "Returns the current epoch (Unix) time in nano-seconds."}
	stdlib["epoch_nano_time"] = func(args ...interface{}) (ret interface{}, err error) {
		return int(time.Now().UnixNano()), err
	}

	slhelp["time_diff"] = LibHelp{in: "te,ts", out: "integer", action: "Returns difference in milli-seconds between two nano-epoch dates."}
	stdlib["time_diff"] = func(args ...interface{}) (ret interface{}, err error) {

		if len(args) != 2 {
			return 0, errors.New("Bad args (count) for time_diff()")
		}
        a,ea:=GetAsInt64(args[0])
        b,eb:=GetAsInt64(args[1])

        if ea || eb {
			return 0, errors.New("Bad args (type) for time_diff()")
        }

/*		if reflect.TypeOf(args[0]).Name() != "int64" || reflect.TypeOf(args[1]).Name() != "int64" {
			return 0, errors.New("Bad args (type) for time_diff()")
		}
		return float64(args[0].(int64)-args[1].(int64)) / 1000000, nil
*/
		return float64(a-b) / 1000000, nil

	}

}

