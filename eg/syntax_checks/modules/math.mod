
define factorial(n)
    return factorial_tr(1f,n.as_bigi)
end

define factorial_tr(acc,n)
    on n<2f do return acc.as_bigi
    return factorial_tr(n*acc,n-1)
end

