
explimit = 100
step = 0.000001


def myexp(val):
    sum = 0.0
    fact = 1.0
    x = 1.0

    for i in range(1, explimit):
        fact = fact * i
        x = x * val
        sum = sum + x/fact

    return sum + 1.0


def integrate(min, max):
    sum = 0.0

    while min < max:
        sum = sum + myexp(min)*step
        min = min + step

    return sum


print("exponent( 1.0)=%s" % (myexp(1.0)))
print("integral(0..1)=%s" % (integrate(0.0,1.0)))
