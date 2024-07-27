#!/usr/bin/za

# notes:
#  adopted from a C example
#  amended to use single dimension arrays
#  still has a few bugs in it, for example abnormally
#   short final paths cause problems.

enum ptype    ( empty=0, pc, block, target, path )
enum waytypes ( NoPath=0, Path, InvalidInput )

struct XY
    x int
    y int
endstruct

struct XYScores
    G int
    H int
    F int
    x int
    y int
endstruct

def find_path(MapArray, DimensionX, DimensionY)

    ListLength=DimensionX*DimensionY
    var OriginalSquare, DestinationSquare, CurrentSquare XY
    var SolvedMapArray [] any
    var FinalPath [ListLength] any
    var ClosedList [ListLength] any
    var OpenList [ListLength] any
    var AdjacentSquares [4] any

    var IsInClosedList, IsInOpenList int
    var i, j, k, m, N, T, temp int
    var TempG int

    var LowestF, PlaceOfLowestF, PlaceOfCurrentSquare int
    var IsWay int

    OriginalSquare.x = -1
    DestinationSquare.x = -1

    # initialise lists
    for e=0 to ListLength-1
        ClosedList[e]=XYScores()
        OpenList[e]  =XYScores()
        FinalPath[e] =XY()
    endfor

    # initialise adjacents
    AdjacentSquares[0]=XY()
    AdjacentSquares[1]=XY()
    AdjacentSquares[2]=XY()
    AdjacentSquares[3]=XY()


    # find start and end positions
    for i = 0 to DimensionY-1
        for j = 0 to DimensionX-1
            case MapArray[i*DimensionX+j]
            is ptype.pc
                OriginalSquare.x = j
                OriginalSquare.y = i
            is ptype.target
                DestinationSquare.x = j
                DestinationSquare.y = i
            endcase
        endfor
    endfor

    # Find the positions of original and destination squares
    if (OriginalSquare.x == -1) or (DestinationSquare.x == -1)
        IsWay = waytypes.InvalidInput
    else
        TempG = 0
        T = 0               # Counts items of Open-List
        N = 0               # Counts items of Closed-List
        OpenList[0].F = -1  # A sign that shows Open-List is empty

        while
            if N == 0
                # Get the square with the lowest F score
                CurrentSquare.x = OriginalSquare.x
                CurrentSquare.y = OriginalSquare.y
                # Add the current square to the Closed-List
                ClosedList[0].x = CurrentSquare.x
                ClosedList[0].y = CurrentSquare.y
                ClosedList[0].H = abs(DestinationSquare.x - OriginalSquare.x) + abs(DestinationSquare.y - OriginalSquare.y)
                ClosedList[0].G = 0
                ClosedList[0].F = ClosedList[0].G + ClosedList[0].H
                N++
            else
                # Get the square with the lowest F score
                LowestF = OpenList[T-1].F
                PlaceOfLowestF = T-1
                for m = T-2 to 0 step -1
                    if OpenList[m].F < LowestF
                        LowestF = OpenList[m].F
                        PlaceOfLowestF = m
                    endif
                endfor

                CurrentSquare.x = OpenList[PlaceOfLowestF].x
                CurrentSquare.y = OpenList[PlaceOfLowestF].y

                # Add the current square to the Closed-List
                ClosedList[N].x = CurrentSquare.x
                ClosedList[N].y = CurrentSquare.y
                ClosedList[N].F = OpenList[PlaceOfLowestF].F
                ClosedList[N].G = OpenList[PlaceOfLowestF].G
                ClosedList[N].H = OpenList[PlaceOfLowestF].H
                PlaceOfCurrentSquare = N
                N++

                # Remove current square from the Open-List
                TempG = OpenList[PlaceOfLowestF].G

                if PlaceOfLowestF == (T - 1)
                    OpenList[T - 1].G = -1
                    OpenList[T - 1].H = -1
                    OpenList[T - 1].F = -1
                    OpenList[T - 1].x = -1
                    OpenList[T - 1].y = -1
                    T--
                else
                    for m = PlaceOfLowestF to T-2
                        OpenList[m].G = OpenList[m + 1].G
                        OpenList[m].H = OpenList[m + 1].H
                        OpenList[m].F = OpenList[m + 1].F
                        OpenList[m].x = OpenList[m + 1].x
                        OpenList[m].y = OpenList[m + 1].y
                    endfor

                    OpenList[T - 1].G = -1
                    OpenList[T - 1].H = -1
                    OpenList[T - 1].F = -1
                    OpenList[T - 1].x = -1
                    OpenList[T - 1].y = -1
                    T--
                endif

                # If we added the destination to the Closed-List, we've found a path
                IsInClosedList = 0
                for m = 0 to N-1
                    if (DestinationSquare.x == ClosedList[m].x) && (DestinationSquare.y == ClosedList[m].y)
                        IsInClosedList = 1
                        break
                    endif
                endfor
                if IsInClosedList == 1
                    IsWay = waytypes.Path
                    break
                endif
            endif

            # Retrieve all its walkable adjacent squares
            AdjacentSquares[0].x = CurrentSquare.x
            AdjacentSquares[0].y = CurrentSquare.y - 1
            AdjacentSquares[1].x = CurrentSquare.x - 1
            AdjacentSquares[1].y = CurrentSquare.y
            AdjacentSquares[2].x = CurrentSquare.x
            AdjacentSquares[2].y = CurrentSquare.y + 1
            AdjacentSquares[3].x = CurrentSquare.x + 1
            AdjacentSquares[3].y = CurrentSquare.y

            for k = 0 to 3
                # If this adjacent square is already in the Closed-List or if it is not an open square, ignore it
                IsInClosedList = 0
                for m = 0 to N-1
                    if (AdjacentSquares[k].x == ClosedList[m].x) && (AdjacentSquares[k].y == ClosedList[m].y)
                        IsInClosedList = 1
                        break
                    endif
                endfor

                on (
                    (AdjacentSquares[k].x < 0)           || (AdjacentSquares[k].y < 0)           ||
                    (AdjacentSquares[k].x >= DimensionX) || (AdjacentSquares[k].y >= DimensionY) ||
                    (MapArray[AdjacentSquares[k].x+AdjacentSquares[k].y*DimensionX] == ptype.block ) ||
                    (IsInClosedList == 1) 
                   ) do continue
                
                IsInOpenList = 0
                for m = 0 to T-1
                    if (AdjacentSquares[k].x == OpenList[m].x) && (AdjacentSquares[k].y == OpenList[m].y)
                        IsInOpenList = 1
                        temp = m
                        break
                    endif
                endfor

                if IsInOpenList != 1
                    # Compute its score and add it to the Open-List
                    OpenList[T].H = abs(DestinationSquare.x - AdjacentSquares[k].x) + abs(DestinationSquare.y - AdjacentSquares[k].y)
                    OpenList[T].G = TempG + 1
                    OpenList[T].F = OpenList[T].H + OpenList[T].G
                    OpenList[T].x = AdjacentSquares[k].x
                    OpenList[T].y = AdjacentSquares[k].y
                    T++
                else
                    # if its already in the Open-List
                    if (ClosedList[PlaceOfCurrentSquare].G + 1) < OpenList[temp].G
                        # Update score of adjacent square that is in Open-List
                        OpenList[temp].G = ClosedList[PlaceOfCurrentSquare].G + 1
                        OpenList[temp].F = OpenList[temp].G + OpenList[temp].H
                    endif
                endif
            endfor
            on OpenList[0].F==-1 do break
        endwhile

        if IsWay == waytypes.Path
            # If there is at least one way to the destination square
            # Now all the algorithm has to do is go backwards to figure out the final path
            m = 0
            CurrentSquare.x = ClosedList[N - 1].x
            CurrentSquare.y = ClosedList[N - 1].y
            TempG = ClosedList[N - 1].G

            while
                if m > 0
                    FinalPath[m - 1].x = CurrentSquare.x
                    FinalPath[m - 1].y = CurrentSquare.y
                endif

                # Retrieve all its walkable adjacent squares
                AdjacentSquares[0].x = CurrentSquare.x
                AdjacentSquares[0].y = CurrentSquare.y - 1
                AdjacentSquares[1].x = CurrentSquare.x - 1
                AdjacentSquares[1].y = CurrentSquare.y
                AdjacentSquares[2].x = CurrentSquare.x
                AdjacentSquares[2].y = CurrentSquare.y + 1
                AdjacentSquares[3].x = CurrentSquare.x + 1
                AdjacentSquares[3].y = CurrentSquare.y

                for k = 0 to 3
                    # If this adjacent square is not an open square, ignore it
                    on (
                        (AdjacentSquares[k].x < 0) || (AdjacentSquares[k].y < 0) ||
                        (AdjacentSquares[k].x >= DimensionX) || (AdjacentSquares[k].y >= DimensionY) ||
                        (MapArray[AdjacentSquares[k].x+AdjacentSquares[k].y*DimensionX] == ptype.block )
                       ) do continue

                    IsInClosedList = 0
                    for j = 0 to N-1
                        if (AdjacentSquares[k].x == ClosedList[j].x) && (AdjacentSquares[k].y == ClosedList[j].y)
                            IsInClosedList = 1
                            temp = j
                            break
                        endif
                    endfor

                    if IsInClosedList == 1
                        # If this adjacent square is in the Closed-List
                        if ClosedList[temp].G == TempG - 1
                            m++
                            CurrentSquare.x = ClosedList[temp].x
                            CurrentSquare.y = ClosedList[temp].y
                            TempG = ClosedList[temp].G
                            break
                        endif
                    endif
                endfor
                on TempG==0 do break
            endwhile

            # Copy MapArray to SolvedMapArray
            for i = 0 to DimensionY-1
                for j = 0 to DimensionX-1
                    SolvedMapArray[i*DimensionX+j] = MapArray[i*DimensionX+j]
                endfor
            endfor

            # Write FinalPath on the SolvedMapArray
            for i = 0 to m-2
                SolvedMapArray[FinalPath[i].x+FinalPath[i].y*DimensionX] = ptype.path
            endfor
        endif
    endif
    return SolvedMapArray, IsWay, m
end

