#!/usr/bin/env python

import sys


def f(NUMBER):
    d = {}
    for i in range(0, NUMBER):
        d[i % 1000] = i
    print(i)


f(int(sys.argv[1]))
