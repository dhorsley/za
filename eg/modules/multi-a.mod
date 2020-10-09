
# show globals

println "Inside module a - ref id ",thisfunc()

println "LA - a->{=a}"
println "LA - s->{=s}"

test "in_mod_a-t1" group "modules" assert fail
    doc "Inside test block {_test_group}/{_test_name}"
    assert getglob("a")==42
    assert getglob("s")=="the answer"
endtest

