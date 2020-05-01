
define factorial(n)
    if n>1
        subsol=factorial(n-1)
        return n*subsol
    else
        return 1
    endif
enddef

