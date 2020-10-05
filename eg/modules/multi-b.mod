
# show globals

println "Inside module b - ref id ",thisfunc()

define test_b()
    println "Inside module b, test func - ref id ",thisfunc()
    a=getglob("a")
    s=getglob("s")
    println "LB - a->{a}"
    println "LB - s->{s}"
enddef

define testdef(q)
    return q*3
enddef
