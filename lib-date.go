//+build !test

package main

import (
	"errors"
	"time"
)

func buildDateLib() {

	features["date"] = Feature{version: 1, category: "date"}
	categories["date"] = []string{"date", "epoch_time", "epoch_nano_time", "time_diff", "date_human",
                                "time_hours","time_minutes","time_seconds","time_nanos",
                                "time_dow","time_dom","time_month","time_year" }

	slhelp["date"] = LibHelp{in: "integer", out: "string", action: "Returns a date/time string. The parsed timestamp is either the current date/time or the optional [#i1]integer[#i0]. RFC3339 format."}
	stdlib["date"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
		var t time.Time
		if len(args) == 1 {
			when, invalid := GetAsInt64(args[0])
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

	slhelp["time_hours"] = LibHelp{in: "epochnano", out: "integer", action: "Returns the current hour of the day (no arguments) or the hour of the day specified by the epoch nano time in [#i1]epochnano[#i0]."}
	stdlib["time_hours"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
		var t time.Time
		if len(args) == 1 {
			when, invalid := GetAsInt(args[0])
			if invalid { return nil, errors.New("Bad args (type) in time_hours().") }
            whensecs:=int(when/1000000000)
            whennano:=when-(whensecs*1000000000)
			t = time.Unix(int64(whensecs),int64(whennano))
		} else {
			t = time.Now()
		}
		return t.Hour(), nil
	}

	slhelp["time_minutes"] = LibHelp{in: "epochnano", out: "integer", action: "Returns the current minute of the day (no arguments) or the minute of the day specified by the epoch nano time in [#i1]epochnano[#i0]."}
	stdlib["time_minutes"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
		var t time.Time
		if len(args) == 1 {
			when, invalid := GetAsInt(args[0])
			if invalid { return nil, errors.New("Bad args (type) in time_minutes().") }
            whensecs:=int(when/1000000000)
            whennano:=when-(whensecs*1000000000)
			t = time.Unix(int64(whensecs),int64(whennano))
		} else {
			t = time.Now()
		}
		return t.Minute(), nil
	}

	slhelp["time_seconds"] = LibHelp{in: "epochnano", out: "integer", action: "Returns the current second of the day (no arguments) or the second of the day specified by the epoch nano time in [#i1]epochnano[#i0]."}
	stdlib["time_seconds"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
		var t time.Time
		if len(args) == 1 {
			when, invalid := GetAsInt(args[0])
			if invalid { return nil, errors.New("Bad args (type) in time_seconds().") }
            whensecs:=int(when/1000000000)
            whennano:=when-(whensecs*1000000000)
			t = time.Unix(int64(whensecs),int64(whennano))
		} else {
			t = time.Now()
		}
		return t.Second(), nil
	}

	slhelp["time_nanos"] = LibHelp{in: "epochnano", out: "integer", action: "Returns the offset within the current second, in nanoseconds, (no arguments) or the offset within the epoch nano time specified in [#i1]epochnano[#i0]."}
	stdlib["time_nanos"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
		var t time.Time
		if len(args) == 1 {
			when, invalid := GetAsInt(args[0])
			if invalid { return nil, errors.New("Bad args (type) in time_nanos().") }
            whensecs:=int(when/1000000000)
            whennano:=when-(whensecs*1000000000)
			t = time.Unix(int64(whensecs),int64(whennano))
		} else {
			t = time.Now()
		}
		return t.Nanosecond(), nil
	}

	slhelp["time_dow"] = LibHelp{in: "epochnano", out: "string", action: "Returns the current day of the week (no arguments) or the day of the epoch nano time specified in [#i1]epochnano[#i0]."}
	stdlib["time_dow"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
		var t time.Time
		if len(args) == 1 {
			when, invalid := GetAsInt(args[0])
			if invalid { return nil, errors.New("Bad args (type) in time_dow().") }
            whensecs:=int(when/1000000000)
            whennano:=when-(whensecs*1000000000)
			t = time.Unix(int64(whensecs),int64(whennano))
		} else {
			t = time.Now()
		}
		return t.Weekday(), nil
	}

	slhelp["time_dom"] = LibHelp{in: "epochnano", out: "integer", action: "Returns the current day of the month (no arguments) or the day of the epoch nano time specified in [#i1]epochnano[#i0]."}
	stdlib["time_dom"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
		var t time.Time
		if len(args) == 1 {
			when, invalid := GetAsInt(args[0])
			if invalid { return nil, errors.New("Bad args (type) in time_dom().") }
            whensecs:=int(when/1000000000)
            whennano:=when-(whensecs*1000000000)
			t = time.Unix(int64(whensecs),int64(whennano))
		} else {
			t = time.Now()
		}
		return t.Day(), nil
	}

	slhelp["time_month"] = LibHelp{in: "epochnano", out: "integer", action: "Returns the current month of the year (no arguments) or the month of the epoch nano time specified in [#i1]epochnano[#i0]."}
	stdlib["time_month"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
		var t time.Time
		if len(args) == 1 {
			when, invalid := GetAsInt(args[0])
			if invalid { return nil, errors.New("Bad args (type) in time_month().") }
            whensecs:=int(when/1000000000)
            whennano:=when-(whensecs*1000000000)
			t = time.Unix(int64(whensecs),int64(whennano))
		} else {
			t = time.Now()
		}
		return t.Month(), nil
	}

	slhelp["time_year"] = LibHelp{in: "epochnano", out: "integer", action: "Returns the current year (no arguments) or the year of the epoch nano time specified in [#i1]epochnano[#i0]."}
	stdlib["time_year"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
		var t time.Time
		if len(args) == 1 {
			when, invalid := GetAsInt(args[0])
			if invalid { return nil, errors.New("Bad args (type) in time_year().") }
            whensecs:=int(when/1000000000)
            whennano:=when-(whensecs*1000000000)
			t = time.Unix(int64(whensecs),int64(whennano))
		} else {
			t = time.Now()
		}
		return t.Year(), nil
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

