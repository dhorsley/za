
define factorial(n)
    return factorial_tr(1,n)
end

define factorial_tr(acc,n)
    on n<2 do return acc
    return factorial_tr(n*acc,n-1)
end

