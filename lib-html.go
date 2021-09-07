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
    case map[string]int:
        o=getHtmlOptionsInt(arg.(map[string]int))
    case map[string]interface{}:
        o=getHtmlOptionsInterface(arg.(map[string]interface{}))
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

	slhelp["wpage"] = LibHelp{in: "content[,options]", out: "string", action: "Create a HTML page tag wrapping."}
	stdlib["wpage"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("wpage",args,3,"2","string","any","1","string","0"); !ok { return nil,err }
        var content,o string
        content=sf("%v",args[0])
        if len(args)==2 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<!DOCTYPE html>\n<HTML "+o+">\n"+content+"</HTML>\n",nil
	}

	slhelp["wbody"] = LibHelp{in: "content[,options]", out: "string", action: "Create a HTML body tag wrapping."}
	stdlib["wbody"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("wbody",args,3,"2","string","any","1","string","0"); !ok { return nil,err }
        var content,o string
        content=sf("%v",args[0])
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<BODY "+o+">\n"+content+"\n</BODY>\n",nil
	}

	slhelp["whead"] = LibHelp{in: "content[,options]", out: "string", action: "Create a HTML head tag wrapping."}
	stdlib["whead"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("whead",args,3,"2","string","any","1","string","0"); !ok { return nil,err }
        var content,o string
        content=sf("%v",args[0])
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<HEAD "+o+">\n"+content+"</HEAD>\n",nil
	}

	slhelp["wdiv"] = LibHelp{in: "content[,options]", out: "string", action: "Create a HTML division tag wrapping."}
	stdlib["wdiv"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("wdiv",args,3,"2","string","any","1","string","0"); !ok { return nil,err }
        var content,o string
        content=sf("%v",args[0])
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<div "+o+">"+content+"</div>\n",nil
	}

	slhelp["wp"] = LibHelp{in: "content[,options]", out: "string", action: "Create a HTML paragraph tag wrapping."}
	stdlib["wp"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("wp",args,3,"2","string","any","1","string","0"); !ok { return nil,err }
        var content,o string
        content=sf("%v",args[0])
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<p "+o+">"+content+"</p>\n",nil
	}

	slhelp["wimg"] = LibHelp{in: "content[,options]", out: "string", action: "Create a HTML image tag."}
	stdlib["wimg"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("wimg",args,1,"1","any"); !ok { return nil,err }
        var o string
        o,err=parseHtmlArgs(args[0])
        if err!=nil { return "",err }
        return "<img "+o+">",nil
	}

	slhelp["wlink"] = LibHelp{in: "content[,options]", out: "string", action: "Create a HTML hyper-link tag."}
	stdlib["wlink"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("wlink",args,1,"1","any","0"); !ok { return nil,err }
        var o string
        o,err=parseHtmlArgs(args[0])
        if err!=nil { return "",err }
        return "<link "+o+">\n",nil
	}

	slhelp["wa"] = LibHelp{in: "content[,options]", out: "string", action: "Create a HTML anchor tag."}
	stdlib["wa"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("wa",args,2,"2","string","any","1","string"); !ok { return nil,err }
        var content,o string
        content=sf("%v",args[0])
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return nil,err }
        return "<a "+o+">"+content+"</a>",nil
	}

	slhelp["wtable"] = LibHelp{in: "content[,options]", out: "string", action: "Create a HTML table tag wrapping."}
	stdlib["wtable"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("wtable",args,3,"2","string","any","1","string","0"); !ok { return nil,err }
        var content,o string
        content=sf("%v",args[0])
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<table "+o+">\n"+content+"</table>\n",nil
	}

    slhelp["wthead"] = LibHelp{in: "content[,options]", out: "string", action: "Create a HTML table head tag wrapping."}
	stdlib["wthead"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        var content,o string
        if ok,err:=expect_args("wthead",args,3,"2","string","any","1","string","0"); !ok { return nil,err }
        content=sf("%v",args[0])
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<thead "+o+">\n"+content+"</thead>\n",nil
	}

    slhelp["wtbody"] = LibHelp{in: "content[,options]", out: "string", action: "Create a HTML table body tag wrapping."}
	stdlib["wtbody"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("wtbody",args,3,"2","string","any","1","string","0"); !ok { return nil,err }
        var content,o string
        content=sf("%v",args[0])
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<tbody "+o+">\n"+content+"</tbody>\n",nil
	}

    slhelp["wtr"] = LibHelp{in: "content[,options]", out: "string", action: "Create a HTML TR table row tag wrapping."}
	stdlib["wtr"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("wtr",args,3,"2","string","any","1","string","0"); !ok { return nil,err }
        var content,o string
        content=sf("%v",args[0])
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<tr "+o+">"+content+"</tr>\n",nil
	}

    slhelp["wth"] = LibHelp{in: "content[,options]", out: "string", action: "Create a HTML TH table header tag wrapping."}
	stdlib["wth"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("wth",args,3,"2","string","any","1","string","0"); !ok { return nil,err }
        var content,o string
        content=sf("%v",args[0])
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<th "+o+">"+content+"</th>",nil
	}

    slhelp["wtd"] = LibHelp{in: "content[,options]", out: "string", action: "Create a HTML TD table data tag wrapping."}
	stdlib["wtd"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("wtd",args,3,"2","string","any","1","string","0"); !ok { return nil,err }
        var content,o string
        content=sf("%v",args[0])
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<td "+o+">"+content+"</td>",nil
	}

    slhelp["wh1"] = LibHelp{in: "content[,options]", out: "string", action: "Create a HTML h1 header tag wrapping."}
	stdlib["wh1"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("wh1",args,3,"2","string","any","1","string","0"); !ok { return nil,err }
        var content,o string
        content=sf("%v",args[0])
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<h1 "+o+">"+content+"</h1>\n",nil
	}

    slhelp["wh2"] = LibHelp{in: "content[,options]", out: "string", action: "Create a HTML h2 header tag wrapping."}
	stdlib["wh2"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("wh2",args,3,"2","string","any","1","string","0"); !ok { return nil,err }
        var content,o string
        content=sf("%v",args[0])
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<h2 "+o+">"+content+"</h2>\n",nil
	}

    slhelp["wh3"] = LibHelp{in: "content[,options]", out: "string", action: "Create a HTML h3 header tag wrapping."}
	stdlib["wh3"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("wh3",args,3,"2","string","any","1","string","0"); !ok { return nil,err }
        var content,o string
        content=sf("%v",args[0])
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<h3 "+o+">"+content+"</h3>\n",nil
	}

    slhelp["wh4"] = LibHelp{in: "content[,options]", out: "string", action: "Create a HTML h4 header tag wrapping."}
	stdlib["wh4"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("wh4",args,3,"2","string","any","1","string","0"); !ok { return nil,err }
        var content,o string
        content=sf("%v",args[0])
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<h4 "+o+">"+content+"</h4>\n",nil
	}

    slhelp["wh5"] = LibHelp{in: "content[,options]", out: "string", action: "Create a HTML h5 header tag wrapping."}
	stdlib["wh5"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("wh5",args,3,"2","string","any","1","string","0"); !ok { return nil,err }
        var content,o string
        content=sf("%v",args[0])
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<h5 "+o+">"+content+"</h5>\n",nil
	}

    slhelp["wol"] = LibHelp{in: "content[,options]", out: "string", action: "Create a HTML ordered list tag wrapping."}
	stdlib["wol"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("wol",args,3,"2","string","any","1","string","0"); !ok { return nil,err }
        var content,o string
        content=sf("%v",args[0])
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<ol "+o+">"+content+"</ol>\n",nil
	}

    slhelp["wul"] = LibHelp{in: "content[,options]", out: "string", action: "Create a HTML unordered list tag wrapping."}
	stdlib["wul"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("wul",args,3,"2","string","any","1","string","0"); !ok { return nil,err }
        var content,o string
        content=sf("%v",args[0])
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<ul "+o+">"+content+"</ul>\n",nil
	}

    slhelp["wli"] = LibHelp{in: "content[,options]", out: "string", action: "Create a HTML list tag wrapping."}
	stdlib["wli"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("wli",args,3,"2","string","any","1","string","0"); !ok { return nil,err }
        var content,o string
        content=sf("%v",args[0])
        if len(args)>1 { o,err=parseHtmlArgs(args[1]) }
        if err!=nil { return "",err }
        return "<li "+o+">"+content+"</li>\n",nil
	}

}
