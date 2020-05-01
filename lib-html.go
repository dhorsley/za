//+build !test

package main

import (
    "errors"
)

// This package includes functions for generating HTML


// convert string 'o' into a set of html element arguments.
func getHtmlOptionsInterface(o map[string]interface{}) string {
    var s string
    for k,v:=range o {
        s=sf(`%s%v='%v' `,s,k,v)
    }
    return s
}

func getHtmlOptionsString(o map[string]string) string {
    var s string
    for k,v:=range o {
        s=sf(`%s%v='%v' `,s,k,v)
    }
    return s
}

func getHtmlOptionsInt(o map[string]int) string {
    var s string
    for k,v:=range o {
        s=sf(`%s%v='%v' `,s,k,v)
    }
    return s
}

func parseHtmlArgs(arg interface{}) (o string,e error) {
    switch arg.(type) {
    case string:
        o=arg.(string)
    case map[string]string:
        o=getHtmlOptionsString(arg.(map[string]string))
    case map[string]interface{}:
        o=getHtmlOptionsInterface(arg.(map[string]interface{}))
    case map[string]int:
        o=getHtmlOptionsInt(arg.(map[string]int))
    default:
        return "",errors.New(sf("Bad arguments provided to argument parser (%T/%v)",arg,arg))
    }
    return o,nil
}

func buildHtmlLib() {

	// conversion

	features["html"] = Feature{version: 1, category: "net"}
	categories["html"] = []string{"wpage","wbody","wdiv","wa","wimg","whead","wlink","wp",
                                "wtable","wthead","wtbody","wtr","wth","wtd",
                                "wh1","wh2","wh3","wh4","wh5","wol","wul","wli",
    }

	slhelp["wpage"] = LibHelp{in: "content[,options]", out: "", action: ""}
	stdlib["wpage"] = func(args ...interface{}) (ret interface{}, err error) {
        var content,o string
        if len(args)>0 { content=sf("%v",args[0]) }
        if len(args)==2 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<!DOCTYPE html>\n<HTML "+o+">\n"+content+"</HTML>\n",nil
	}

	slhelp["wbody"] = LibHelp{in: "content[,options]", out: "", action: ""}
	stdlib["wbody"] = func(args ...interface{}) (ret interface{}, err error) {
        var content,o string
        if len(args)>0 { content=sf("%v",args[0]) }
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<BODY "+o+">\n"+content+"\n</BODY>\n",nil
	}

	slhelp["whead"] = LibHelp{in: "content[,options]", out: "", action: ""}
	stdlib["whead"] = func(args ...interface{}) (ret interface{}, err error) {
        var content,o string
        if len(args)>0 { content=sf("%v",args[0]) }
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<HEAD "+o+">\n"+content+"</HEAD>\n",nil
	}

	slhelp["wdiv"] = LibHelp{in: "content[,options]", out: "", action: ""}
	stdlib["wdiv"] = func(args ...interface{}) (ret interface{}, err error) {
        var content,o string
        if len(args)>0 { content=sf("%v",args[0]) }
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<div "+o+">"+content+"</div>\n",nil
	}

	slhelp["wp"] = LibHelp{in: "content[,options]", out: "", action: ""}
	stdlib["wp"] = func(args ...interface{}) (ret interface{}, err error) {
        var content,o string
        if len(args)>0 { content=sf("%v",args[0]) }
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<p "+o+">"+content+"</p>\n",nil
	}

	slhelp["wimg"] = LibHelp{in: "content[,options]", out: "", action: ""}
	stdlib["wimg"] = func(args ...interface{}) (ret interface{}, err error) {
        var o string
        if len(args)>0 { o,err=parseHtmlArgs(args[0]) }
        if err!=nil { return "",err }
        return "<img "+o+">",nil
	}

	slhelp["wlink"] = LibHelp{in: "content[,options]", out: "", action: ""}
	stdlib["wlink"] = func(args ...interface{}) (ret interface{}, err error) {
        var o string
        if len(args)>0 { o,err=parseHtmlArgs(args[0]) }
        if err!=nil { return "",err }
        return "<link "+o+">\n",nil
	}

	slhelp["wa"] = LibHelp{in: "content[,options]", out: "", action: ""}
	stdlib["wa"] = func(args ...interface{}) (ret interface{}, err error) {
        var content,o string
        if len(args)>0 { content=sf("%v",args[0]) }
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<a "+o+">"+content+"</a>",nil
	}

	slhelp["wtable"] = LibHelp{in: "content[,options]", out: "", action: ""}
	stdlib["wtable"] = func(args ...interface{}) (ret interface{}, err error) {
        var content,o string
        if len(args)>0 { content=sf("%v",args[0]) }
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<table "+o+">\n"+content+"</table>\n",nil
	}

    slhelp["wthead"] = LibHelp{in: "content[,options]", out: "", action: ""}
	stdlib["wthead"] = func(args ...interface{}) (ret interface{}, err error) {
        var content,o string
        if len(args)>0 { content=sf("%v",args[0]) }
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<thead "+o+">\n"+content+"</thead>\n",nil
	}

    slhelp["wtbody"] = LibHelp{in: "content[,options]", out: "", action: ""}
	stdlib["wtbody"] = func(args ...interface{}) (ret interface{}, err error) {
        var content,o string
        if len(args)>0 { content=sf("%v",args[0]) }
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<tbody "+o+">\n"+content+"</tbody>\n",nil
	}

    slhelp["wtr"] = LibHelp{in: "content[,options]", out: "", action: ""}
	stdlib["wtr"] = func(args ...interface{}) (ret interface{}, err error) {
        var content,o string
        if len(args)>0 { content=sf("%v",args[0]) }
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<tr "+o+">"+content+"</tr>\n",nil
	}

    slhelp["wth"] = LibHelp{in: "content[,options]", out: "", action: ""}
	stdlib["wth"] = func(args ...interface{}) (ret interface{}, err error) {
        var content,o string
        if len(args)>0 { content=sf("%v",args[0]) }
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<th "+o+">"+content+"</th>",nil
	}

    slhelp["wtd"] = LibHelp{in: "content[,options]", out: "", action: ""}
	stdlib["wtd"] = func(args ...interface{}) (ret interface{}, err error) {
        var content,o string
        if len(args)>0 { content=sf("%v",args[0]) }
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<td "+o+">"+content+"</td>",nil
	}

    slhelp["wh1"] = LibHelp{in: "content[,options]", out: "", action: ""}
	stdlib["wh1"] = func(args ...interface{}) (ret interface{}, err error) {
        var content,o string
        if len(args)>0 { content=sf("%v",args[0]) }
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<h1 "+o+">"+content+"</h1>\n",nil
	}

    slhelp["wh2"] = LibHelp{in: "content[,options]", out: "", action: ""}
	stdlib["wh2"] = func(args ...interface{}) (ret interface{}, err error) {
        var content,o string
        if len(args)>0 { content=sf("%v",args[0]) }
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<h2 "+o+">"+content+"</h2>\n",nil
	}

    slhelp["wh3"] = LibHelp{in: "content[,options]", out: "", action: ""}
	stdlib["wh3"] = func(args ...interface{}) (ret interface{}, err error) {
        var content,o string
        if len(args)>0 { content=sf("%v",args[0]) }
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<h3 "+o+">"+content+"</h3>\n",nil
	}

    slhelp["wh4"] = LibHelp{in: "content[,options]", out: "", action: ""}
	stdlib["wh4"] = func(args ...interface{}) (ret interface{}, err error) {
        var content,o string
        if len(args)>0 { content=sf("%v",args[0]) }
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<h4 "+o+">"+content+"</h4>\n",nil
	}

    slhelp["wh5"] = LibHelp{in: "content[,options]", out: "", action: ""}
	stdlib["wh5"] = func(args ...interface{}) (ret interface{}, err error) {
        var content,o string
        if len(args)>0 { content=sf("%v",args[0]) }
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<h5 "+o+">"+content+"</h5>\n",nil
	}

    slhelp["wol"] = LibHelp{in: "content[,options]", out: "", action: ""}
	stdlib["wol"] = func(args ...interface{}) (ret interface{}, err error) {
        var content,o string
        if len(args)>0 { content=sf("%v",args[0]) }
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<ol "+o+">"+content+"</ol>\n",nil
	}

    slhelp["wul"] = LibHelp{in: "content[,options]", out: "", action: ""}
	stdlib["wul"] = func(args ...interface{}) (ret interface{}, err error) {
        var content,o string
        if len(args)>0 { content=sf("%v",args[0]) }
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<ul "+o+">"+content+"</ul>\n",nil
	}

    slhelp["wli"] = LibHelp{in: "content[,options]", out: "", action: ""}
	stdlib["wli"] = func(args ...interface{}) (ret interface{}, err error) {
        var content,o string
        if len(args)>0 { content=sf("%v",args[0]) }
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<li "+o+">"+content+"</li>\n",nil
	}

}
