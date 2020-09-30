#/usr/bin/za

define humansize(i,prec,unit)
    if i>=1e9; unit="Billion"+unit; i=float(i/1e9); endif
    if i>=1e6; unit="Million"+unit; i=float(i/1e6); endif
    if i>=1e3; unit="Thousand"+unit; i=float(i/1e3); endif
    return format( "%." + prec + "f %s" ,i,unit)
enddef

define report(ts,te,max_count)
    dur=time_diff(te,ts)/1000000
    println format("\nduration       [#5]%0.3f s[#-]",dur)
    println format("iterations     %v",humansize(float(max_count),0,""))
    rate=float(dur/max_count)
    println format("avg iteration  %0.3f ns",rate*1e9)
    hs=humansize(1/rate,3,"")
    println format("its/sec        [#5]%s[#-]\n",hs)
enddef
