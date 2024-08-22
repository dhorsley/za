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
                                "time_dow","time_dom","time_month","time_year", "time_zone", "time_zone_offset", }

	slhelp["date"] = LibHelp{in: "[integer]", out: "string", action: "Returns a date/time string (RFC3339 format). Optional [#i1]integer[#i0] defaults to the current date/time, otherwise must be an epoch timestamp."}
	stdlib["date"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("date",args,2,
            "1","int",
            "0"); !ok { return nil,err }
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

	slhelp["date_human"] = LibHelp{in: "[integer]", out: "string",
        action: "Returns the current date and time in a readable format (RFC822Z).\n"+
            "If optional [#i1]integer[#i0] present, uses that as the epoch timestamp to convert."}
	stdlib["date_human"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("date_human",args,2,
            "1","int",
            "0"); !ok { return nil,err }
        if len(args)==1 {
            return time.Unix(int64(args[0].(int)),0).Format(time.RFC822Z),nil
        }
        return time.Now().Format(time.RFC822Z),nil
    }

	slhelp["epoch_time"] = LibHelp{in: "[string]", out: "integer", action: "Returns the current epoch (Unix) time in seconds. If the optional [#i1]string[#i0] is present, then this is converted to epoch int format from RFC3339 format."}
	stdlib["epoch_time"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("epoch_time",args,2,
            "1","string",
            "0"); !ok { return nil,err }
        if len(args)==1 {
            dt,e:=time.Parse(time.RFC3339,args[0].(string))
            if e==nil { return int(dt.Unix()),nil }
            return nil,e
        }
		return int(time.Now().Unix()), err
	}

	slhelp["time_zone"] = LibHelp{in: "[string]", out: "integer", action: "Returns the time zone."}
	stdlib["time_zone"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("time_zone",args,2,
            "1","int",
            "0"); !ok { return nil,err }
        name:=""
        if len(args)==1 {
            dt,e:=time.Parse(time.RFC3339,args[0].(string))
            if e==nil { 
                name,_=dt.Zone()
                return name,nil
            }
            return nil,e
        }
        name,_=time.Now().Zone()
        return name,nil
	}

	slhelp["time_zone_offset"] = LibHelp{in: "[string]", out: "integer", action: "Returns the time zone offset in seconds."}
	stdlib["time_zone_offset"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("time_zone_offset",args,2,
            "1","int",
            "0"); !ok { return nil,err }
        var offset int
        if len(args)==1 {
            dt,e:=time.Parse(time.RFC3339,args[0].(string))
            if e==nil { 
                _,offset=dt.Zone()
                return offset,nil
            }
            return nil,e
        }
        _,offset=time.Now().Zone()
        return offset,nil
	}

	slhelp["epoch_nano_time"] = LibHelp{in: "none", out: "integer", action: "Returns the current epoch (Unix) time in nano-seconds."}
	stdlib["epoch_nano_time"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("epoch_nano_time",args,2,
            "1","string",
            "0"); !ok { return nil,err }
        if len(args)==1 {
            dt,e:=time.Parse(time.RFC3339,args[0].(string))
            if e==nil { return int(dt.UnixNano()),nil }
            return nil,e
        }
		return int(time.Now().UnixNano()), err
	}

	slhelp["time_hours"] = LibHelp{in: "[epochnano]", out: "integer", action: "Returns the current hour of the day (no arguments) or the hour of the day specified by the epoch nano time in [#i1]epochnano[#i0]."}
	stdlib["time_hours"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("time_hours",args,2,
            "1","int",
            "0"); !ok { return nil,err }
		var t time.Time
		if len(args) == 1 {
			when := args[0].(int)
            whensecs:=int(when/1000000000)
            whennano:=when-(whensecs*1000000000)
			t = time.Unix(int64(whensecs),int64(whennano))
		} else {
			t = time.Now()
		}
		return t.Hour(), nil
	}

	slhelp["time_minutes"] = LibHelp{in: "[epochnano]", out: "integer", action: "Returns the current minute of the day (no arguments) or the minute of the day specified by the epoch nano time in [#i1]epochnano[#i0]."}
	stdlib["time_minutes"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("time_minutes",args,2,
            "1","int",
            "0"); !ok { return nil,err }
		var t time.Time
		if len(args) == 1 {
			when := args[0].(int)
            whensecs:=int(when/1000000000)
            whennano:=when-(whensecs*1000000000)
			t = time.Unix(int64(whensecs),int64(whennano))
		} else {
			t = time.Now()
		}
		return t.Minute(), nil
	}

	slhelp["time_seconds"] = LibHelp{in: "[epochnano]", out: "integer",
        action: "Returns the current second of the day (no arguments) or the second of the day specified by\n"+
            "the epoch nano time in [#i1]epochnano[#i0]."}
	stdlib["time_seconds"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("time_seconds",args,2,
            "1","int",
            "0"); !ok { return nil,err }
		var t time.Time
		if len(args) == 1 {
			when := args[0].(int)
            whensecs:=int(when/1000000000)
            whennano:=when-(whensecs*1000000000)
			t = time.Unix(int64(whensecs),int64(whennano))
		} else {
			t = time.Now()
		}
		return t.Second(), nil
	}

	slhelp["time_nanos"] = LibHelp{in: "[epochnano]", out: "integer",
        action: "Returns the offset within the current second, in nanoseconds, (no arguments) or the\n"+
            "offset within the epoch nano time specified in [#i1]epochnano[#i0]."}
	stdlib["time_nanos"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("time_nanos",args,2,
            "1","int",
            "0"); !ok { return nil,err }
		var t time.Time
		if len(args) == 1 {
			when := args[0].(int)
            whensecs:=int(when/1000000000)
            whennano:=when-(whensecs*1000000000)
			t = time.Unix(int64(whensecs),int64(whennano))
		} else {
			t = time.Now()
		}
		return t.Nanosecond(), nil
	}

	slhelp["time_dow"] = LibHelp{in: "[epochnano]", out: "string", action: "Returns the current day of the week (no arguments) or the day of the epoch nano time specified in [#i1]epochnano[#i0]."}
	stdlib["time_dow"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("time_dow",args,2,
            "1","int",
            "0"); !ok { return nil,err }
		var t time.Time
		if len(args) == 1 {
			when := args[0].(int)
            whensecs:=int(when/1000000000)
            whennano:=when-(whensecs*1000000000)
			t = time.Unix(int64(whensecs),int64(whennano))
		} else {
			t = time.Now()
		}
		return t.Weekday(), nil
	}

	slhelp["time_dom"] = LibHelp{in: "[epochnano]", out: "integer", action: "Returns the current day of the month (no arguments) or the day of the epoch nano time specified in [#i1]epochnano[#i0]."}
	stdlib["time_dom"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("time_dom",args,2,
            "1","int",
            "0"); !ok { return nil,err }
		var t time.Time
		if len(args) == 1 {
			when := args[0].(int)
            whensecs:=int(when/1000000000)
            whennano:=when-(whensecs*1000000000)
			t = time.Unix(int64(whensecs),int64(whennano))
		} else {
			t = time.Now()
		}
		return t.Day(), nil
	}

	slhelp["time_month"] = LibHelp{in: "[epochnano]", out: "integer", action: "Returns the current month of the year (no arguments) or the month of the epoch nano time specified in [#i1]epochnano[#i0]."}
	stdlib["time_month"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("time_month",args,2,
            "1","int",
            "0"); !ok { return nil,err }
		var t time.Time
		if len(args) == 1 {
			when := args[0].(int)
            whensecs:=int(when/1000000000)
            whennano:=when-(whensecs*1000000000)
			t = time.Unix(int64(whensecs),int64(whennano))
		} else {
			t = time.Now()
		}
		return int(t.Month()), nil
	}

	slhelp["time_year"] = LibHelp{in: "[epochnano]", out: "integer", action: "Returns the current year (no arguments) or the year of the epoch nano time specified in [#i1]epochnano[#i0]."}
	stdlib["time_year"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("time_year",args,2,
            "1","int",
            "0"); !ok { return nil,err }
		var t time.Time
		if len(args) == 1 {
			when := args[0].(int)
            whensecs:=int(when/1000000000)
            whennano:=when-(whensecs*1000000000)
			t = time.Unix(int64(whensecs),int64(whennano))
		} else {
			t = time.Now()
		}
		return t.Year(), nil
	}

	slhelp["time_diff"] = LibHelp{in: "te,ts", out: "integer", action: "Returns difference in micro-seconds between two nano-epoch dates."}
	stdlib["time_diff"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("time_diff",args,1,"2","int","int"); !ok { return nil,err }
        return float64(args[0].(int)-args[1].(int))/1000,nil

	}

}

