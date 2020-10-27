sumOfNumbers = 0
numberToAdd = 1
40000000.times() do
    sumOfNumbers = sumOfNumbers.+(numberToAdd)
    numberToAdd+=1
end
puts sumOfNumbers
