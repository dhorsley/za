// +build !linux !freebsd
// +build windows netgo

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
    buildOsLib()
    buildSumLib()
    buildDateLib()
    buildMathLib()
    buildListLib()
    buildFileLib()
    buildNotifyLib()
    buildConversionLib()
    buildNetLib()
    buildDbLib()
    buildHtmlLib()
    buildImageLib()
    buildTuiLib()

}
