package main

import (
	str "strings"
)

const alpha = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
const alphaplus = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz_@{}"
const alphanumeric = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
const numeric = "0123456789."

// parse():
//
//   process an input string into separate lines of commands (Phrases). Each phrase is
//   built from successive calls to nextToken(). Input ends at end-of-string or earlier
//   if an invalid token is found.
//
//   Each phrase is appended to the 'function space' (i.e. function body) of the function
//   referenced by fs. A phrase is a list of processed tokens.
//
//   functionspaces[] is a global.
//

func parse(fs string, input string, start int) (badword bool, eof bool) {

	pos := start
	lstart := start

	var eol bool

	var tempToken Token
	var phrase = Phrase{}

	previousToken := Error
	curLine := 1

    var strPhrase str.Builder
    strPhrase.Grow(32)

	for ; pos < len(input); pos++ {

        // debug(15,"nt : (pos:%d) calling nextToken()\n",pos)
		tempToken, eol, eof = nextToken(input, &curLine, pos, previousToken)
		previousToken = tempToken.tokType

        // debug(15,"nt-t: (tokpos:%d) %v\n",tempToken.tokPos,tokNames[tempToken.tokType])

        if previousToken==SingleComment {
            // at this point we have returned the full comment, pos was backtracked to just before the EOL.
            tempToken.tokType=EOL
        }

        if previousToken==C_Semicolon {
            tempToken.tokType=EOL
            tempToken.tokText=""
            eol=true
        }

		phrase.Tokens = append(phrase.Tokens, tempToken)
		phrase.TokenCount++
        strPhrase.WriteString(tempToken.tokText+" ")

		if tempToken.tokPos != -1 {
			pos = tempToken.tokPos
		}

		if tempToken.tokType == Error {
			pf("Error found on line %d in %s\n", curLine, tempToken.tokText)
			break
		}

		if eof || eol {

            // -- strip the eol
            if eol { phrase.TokenCount-- }

			// -- cleanup phrase text
			phrase.Text = str.TrimRight(strPhrase.String(), " ")
            // debug(15,"current phrase text = '%v'\n",phrase.Text)

			// -- add original version
			// if pos>0 { phrase.Original = input[lstart:pos+1] }
			if pos>0 { phrase.Original = input[lstart:pos] }
			// lstart = pos + 1
			lstart = pos
            // debug(15,"current phrase orig = '%v'\n",phrase.Original)

            // -- discard empty lines
            if phrase.TokenCount!=0 {
                // -- add phrase to function
                lmv,_:=fnlookup.lmget(fs)
                if lockSafety { fspacelock.Lock() }
                functionspaces[lmv] = append(functionspaces[lmv], phrase)
                if lockSafety { fspacelock.Unlock() }
            }

			// reset phrase
			phrase = Phrase{}
            strPhrase.Reset()
		}

		if eof {
			break
		}

	}

	return badword, eof

}
