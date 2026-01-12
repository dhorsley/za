//go:build (linux || freebsd) && (ignore || windows || !netgo)

package main

type LibHelp struct {
    in     string
    out    string
    action string
}

var slhelp = make(map[string]LibHelp)
var categories = make(map[string][]string)

func buildStandardLib() {

    buildInternalLib()
    buildPackageLib()
    buildStringLib()
    buildRegexLib()
    buildOsLib()
    buildSumLib()
    buildDateLib()
    buildMathLib()
    buildListLib()
    buildMapLib()
    buildArrayLib()
    buildINILib()
    buildFileLib()
    buildNotifyLib()
    buildConversionLib()
    buildWebLib()
    buildNetworkLib()
    buildSystemLib()
    buildDbLib()
    buildHtmlLib()
    buildImageLib()
    buildTuiLib()
    buildErrorLib()
    buildYamlLib()
    buildZipLib()
    buildSmtpLib()
    buildCronLib()
    buildFfiLib()

}
