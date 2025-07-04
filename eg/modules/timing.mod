
define humansize(i,prec,unit)
    if i>=1e9; unit="Billion"+unit; i=as_float(i/1e9); endif
    if i>=1e6; unit="Million"+unit; i=as_float(i/1e6); endif
    if i>=1e3; unit="Thousand"+unit; i=as_float(i/1e3); endif
    return format( "%." + prec + "f %s" ,i,unit)
end

define report(ts,te,max_count,op_count=nil)
    dur=time_diff(te,ts)/1000000
    println format("\nduration       [#5]%0.3f s[#-]",dur)
    println format("iterations     %v",humansize(max_count.as_float,3,""))
    rate=as_float(dur/max_count)
    println format("avg iteration  %0.3f ns",rate*1e9)
    hs=humansize(1/rate,3,"")
    println format("its/sec        [#5]%s[#-]",hs)
    if op_count!=nil
        rate=as_float(dur/(max_count*op_count))
        hs=humansize(1/rate,3,"")
        println format("ops/sec        [#5]%s[#-]",hs)
    endif
    println
end

