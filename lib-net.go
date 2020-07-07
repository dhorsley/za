//+build !test

package main

import (
    "mime"
    "context"
    "crypto/tls"
    "errors"
    // "fmt"
    "time"
    "net"
    "regexp"
    "io/ioutil"
    "net/http"
    "net/url"
    "math/rand"
    "log"
    "html"
    "os"
    "sync"
    "path/filepath"
    str "strings"
)

/* · web_serve_*:
     · these are not intended for anything complex.
     · quick and dirty page serving and proxying
     · no intention to add header or method inspection or anything other than what is below.
*/

// TEST : Metrics
// The code below was for testing graphite stuff.
//  it can probably be ripped out now - no intention to incorporate it fully.
//  So long as WebSendMetrics remains false then it will have no side effect leaving it in
//  in case minds are changed, but it is most likely dead code.
//  if we wanted to use it, we would need library/env stuff for setting the host+port and 
//  a way of turning metrics generation off and on.
//  we would also want to be able to push metrics on demand from other areas of Za code. 
//  not saying it would be difficult to add, just incredibly low priority.

/*
    var WLOG_METRICS_HOST="172.16.10.29"
    var WLOG_METRICS_PORT=2003
*/
var WebSendMetrics=false

var web_tr *http.Transport
var web_client *http.Client

var web_log_handle *os.File
var web_logger *log.Logger


// TYPES ////////////////////////////////////////////////////////////////////////////////////////

type web_table_entry struct {
    docroot string
    addr string             // addr is the host:port pair to serve traffic from
    host string             // for traffic routing rules
    port int                // which tcp port to create a multiplexor on.
    srv *http.Server
    mux *http.ServeMux
}

var web_handles = make( map[string]web_table_entry )

type web_rule struct {
    code string
    in   string
    mutation interface{}
}

var web_rules = make(map[string][]web_rule)

// HELPER FUNCTIONS /////////////////////////////////////////////////////////////////////////////

var lastWlogMsg string
var lastWlogCount int
var lastWlogStart int=5
var lastWlogEvery int=10000
var throttle bool
var WlogDisplay=true

// Counting semaphore using a buffered channel
func limitNumClients(f http.HandlerFunc, maxClients int, evalfs uint64) http.HandlerFunc {
    sema := make(chan struct{}, maxClients)
    return func(w http.ResponseWriter, req *http.Request) {
        ctx:=req.Context()
        ctx=context.WithValue(ctx,"evalfs",evalfs)
        req=req.WithContext(ctx)

        sema <- struct{}{}
        defer func() { <-sema }()
            f(w, req)
    }
}

// web access logging
func wlog(s string,va... interface{}) {
    lastlock.Lock()
    if log_web {
        new_s:=sf(s,va...)
        throttleEnded:=false
        if lastWlogMsg==new_s { // repeat message
            lastWlogCount++
            if lastWlogCount==lastWlogStart { // begin throttling
                throttle=true
            }
            if throttle {
                WlogDisplay=false
                if (lastWlogCount-lastWlogStart) % lastWlogEvery == 0 { // let one through
                    WlogDisplay=true
                }
            }
        } else {
            lastWlogCount=0
            lastWlogMsg=new_s
            if throttle { throttleEnded=true }
            throttle=false
            WlogDisplay=true
        }

        if WlogDisplay {
            oldFlags:=web_logger.Flags()
            web_logger.SetFlags(log.Lmicroseconds+log.LUTC)
            if throttle      { web_logger.Printf("// skipped %d repeat messages.\n",lastWlogEvery) }
            if throttleEnded { web_logger.Printf("// stopped throttling messages.\n") }
            web_logger.Printf(new_s)
            web_logger.SetFlags(oldFlags)
        }
    }
    lastlock.Unlock()
}

func webLookup(host string) string {
    weblock.RLock()
    for k,v:=range web_handles {
        if v.addr == host {
            weblock.RUnlock()
            return k
        }
    }
    weblock.RUnlock()
    return ""
}

func webClose(h string) {
    pf("* Closing server (%s).\n",h)
    web_handles[h].srv.Shutdown(context.Background())
    delete(web_handles,h)
}

func webCloseAll() {
    for h,s:=range(web_handles) {
        pf("* Closing server (%s) : %+v\n",h,s)
        s.srv.Shutdown(context.Background())
        delete(web_handles,h)
    }
}

func webRoutesAll() {
    weblock.RLock()
    for uid,entry:=range web_handles {
        host:=entry.addr
        port:=entry.port
        pf("Service Host : %s / %d\n",host,port)
        webRoutes(uid)
    }
    weblock.RUnlock()
}

func webRoutes(uid string) {
    for _,action:=range web_rules[uid] {
        pf("* %s : %s %s -> %+v\n",uid,action.code,action.in,action.mutation)
    }

}

func fireMetric(host string,typ string,value int) {

    // what to send:
    // increments to query count
    // increments to permanent redirects
    // increments to local served
    // increments to proxied requests
    // failed to serve
/*     var strType string
    switch typ {
    case "e":
        strType="error"
    case "r":
        strType="redirect"
    case "f":
        strType="call"
    case "s":
        strType="served"
    case "p":
        strType="proxied"
    default:
        return
    }
*/

/*
    reqHostSplitAt:=str.IndexByte(host,':')
    reqHost:=host[:reqHostSplitAt]
    reqPort:=host[reqHostSplitAt+1:]
*/

    // metconn, _ := net.Dial("tcp", WLOG_METRICS_HOST+":"+sf("%v",WLOG_METRICS_PORT))
    // wlog("metric: local.%s.%s.%s %v %v\n",reqHost,reqPort,strType,value,time.Now().Unix())
    // fmt.Fprintf(metconn,"local.%s.%s.%s %v %v\n",reqHost,reqPort,strType,value,time.Now().Unix())
    // we should check error here, but what would we do with it?
    // metconn.Close()

}


type webstruct struct {
    host    string
    path    string
    method  string
    remote  string
    query   string
    fragment string
    data    []interface{}
}


/// webRouter()
//
// this function should pick apart the request to get the ip/dns and the subpath.
// we should also pick the tcp port out.
// with the host+port details we should be able to look up the uid in web_handles[]
// once we have the uid, we should iterate over the web_rules[uid] for a match.
// if no match is found, the default will be to try and serve the request statically from the docroot
// the docroot for the vhost should be available in web_handles[uid].docroot

func webRouter(w http.ResponseWriter, r *http.Request) {

    evalfs,_:=r.Context().Value("evalfs").(uint64)

    // we do not log by default
    // if globalvar log_web is true, then log to web_log_file (global)

    // we also throw out some debug info too when enabled. this should not be the default!
    // it really slows down request processing.

    // get debug level once

    debuglock.RLock()
    dlevel:=debug_level
    debuglock.RUnlock()

    method:=r.Method        // get, put, etc
    purl  :=r.URL           // provided url
    header:=r.Header        // map[string][]string
    host  :=r.Host          // string from either Header or URL : this appears to include the port number on it
    remote:=r.RemoteAddr    // requester ip:port string
    path  :=purl.Path       // .Path (may or may not including leading path when relative)
    scheme:=purl.Scheme     // http/https/etc
    if scheme=="" { scheme="http" }
    _=method ; _=header

    srvAddr := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
    srvStr  := srvAddr.String()

    localSplitAt:=str.IndexByte(srvStr,':')
    // localIp:=srvStr[:localSplitAt]
    localPort:=srvStr[localSplitAt+1:]

    remoteSplitAt:=str.IndexByte(remote,':')
    remoteIp:=remote[:remoteSplitAt]

    if dlevel>5 { wlog("^ NEW REQUEST : host [%v] path [%v]\n",host,purl) }

    serviced:=false         // was a rule acted upon?
    handle := webLookup(host)

    if handle=="" {
        port:=""
        if dlevel>5 { wlog("  entered empty handle processing\n") }
        // extract port
        reqHostSplitAt:=str.IndexByte(host,':')
        if reqHostSplitAt>-1 {
            reqPort:=host[reqHostSplitAt+1:]
            port=reqPort
        } else {
            // .. or use port from request context
            port=localPort
        }
        handle=webLookup("0.0.0.0:"+port)
        if dlevel>5 { wlog("  new handle: %v [with 0.0.0.0:%v]\n",handle,port) }
    } else {
        if dlevel>5 { wlog("  using handle: %v [with %v]\n",handle,host) }
    }

    // deal with forced redirects and reverse proxying first

    webrulelock.RLock()
    wr_copy:=web_rules[handle]
    webrulelock.RUnlock()

    for _,rule := range wr_copy {

        switch rule.code {

        case "r": // redirect

            // should reply with a redirect code
            // pointing the requesting client to go 
            // to mutation instead of host+path

            if str.HasPrefix(path,rule.in) {
                http.Redirect(w,r,rule.mutation.(string),http.StatusMovedPermanently)
                wlog("Redirected from %s to %s.\n",path,rule.mutation.(string))
                if WebSendMetrics { fireMetric(host,"r",http.StatusMovedPermanently) }
                serviced=true
            }

        case "p": // rewrite and reverse proxy

            if dlevel>5 { wlog("Hitting reverse proxy rule [ %s / %s ]\n",rule.in,rule.mutation) }
            // build new_path based on path in rule.in + rule.mutation
            // make a client request from here to host+new_path
            // pass result back including status codes

            var re = regexp.MustCompile(rule.in)
            if re.MatchString(path) {

                // @todo: we should figure the best way (and a quick way to start with)
                //  to fail gracefully when the rewrite goes into a loop on the same host+path

                var new_path string
                if rule.in!="" && rule.mutation.(string)!="" {
                    // provided with a regex rewrite
                    new_path = re.ReplaceAllString(path, rule.mutation.(string))
                } else {
                    // just proxy
                    new_path=rule.in
                }

                // make a client request
                var content []byte
                var down_code int = -1
                // var header http.Header

                switch method {

                case "GET":
                    nq:=new_path
                    if purl.RawQuery!="" {
                        nq+="?"+purl.RawQuery
                    }
                    if purl.Fragment!="" {
                        nq+="#"+purl.Fragment
                    }
                    // log_nq,_:=url.QueryUnescape(nq)
                    // wlog("* GET Proxying this URL: %s\n",log_nq)
                    content, down_code, header = download(nq)

                    w.Header().Add("proxied-by-za","true")
                    for k,v:=range header {
                        w.Header().Set(k,str.Join(v,","))
                    }

                case "HEAD":
                    nq:=new_path
                    if purl.RawQuery!="" {
                        nq+="?"+purl.RawQuery
                    }
                    if purl.Fragment!="" {
                        nq+="#"+purl.Fragment
                    }
                    // log_nq,_:=url.QueryUnescape(nq)
                    // wlog("* HEAD Proxying this URL: %s\n",log_nq)
                    content, down_code = head(nq)

                case "POST", "PUT":

                    // get the form data from the request
                    r.ParseForm()
                    fvals:=url.Values{}
                    if r.PostForm!=nil {
                        fvals=r.PostForm
                    } else {
                        if r.Form!=nil {
                            fvals=r.Form
                        }
                    }

                    // .. and feed it into the backend request
                    tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
                    client := &http.Client{Transport: tr}
                    resp, err := client.PostForm(new_path,fvals)
                    defer resp.Body.Close()
                    down_code=resp.StatusCode
                    if err==nil {
                        content,err=ioutil.ReadAll(resp.Body)
                        // resp.Body.Close()
                    }
                    // wlog("POST request: Form value provided: %#v\n",r.URL.RawQuery)

                }

                if down_code>299 {
                    // now look for matching rules for failure
                    // for _,fail_rule := range web_rules[handle] {
                    for _,fail_rule := range wr_copy {
                        switch fail_rule.code[0] {
                        case 'e':
                        // fail_rule.in is which path prefix to match on
                        // fail_rule.mutation is where to redirect to on error
                            if dlevel>5 { wlog("Checking error rules for path [%s]\n",path) }
                            expectedStatusCode:=fail_rule.code[1:]
                            if str.HasPrefix(path,fail_rule.in) {
                                if expectedStatusCode=="" || expectedStatusCode==sf("%d",down_code) {
                                    http.Redirect(w,r,fail_rule.mutation.(string),http.StatusTemporaryRedirect)
                                    wlog("Redirected temporarily from %s to %s due to bad response.\n",path,fail_rule.mutation.(string))
                                    if WebSendMetrics { fireMetric(host,"e",http.StatusTemporaryRedirect) }
                                    serviced=true
                                    break
                                } else {
                                    if dlevel>5 { wlog("Error check reached inconclusive state with rule code '%s' and inbound code '%+v'\n",expectedStatusCode,down_code) }
                                }
                            }
                        }
                    }
                } else {
                    // on success:
                    // case "w": // @todo: rewrite response body
                    // wlog("Proxy read the page successfully. Writing back to remote client.\n")
                    wlog("%s served %s to %s.\n",host,new_path,remoteIp)
                    if WebSendMetrics { fireMetric(host,"p",1) }
                    w.Write([]byte(content))
                }
                serviced=true
            }

        case "f":

            // uid, "f", "regex_path", "func"
            // on regex match, build argument list from POST params.
            //   The list should have it's arguments cleansed for the usual bad things.
            //   Make a call to za function "func" with those as the arguments.
            // if return == not_empty then w.Write(ret_string)
            // if return == empty then check "e" rules,
            //   e rule may redirect or send a specific error
            // if no error rule available, return a notfound or nothing at all?

            var re = regexp.MustCompile(rule.in)
            if re.MatchString(path) {

                fn:=rule.mutation.(string)
                if dlevel>5 { wlog("Called %s from %s.\n",fn,path) }

                // check that function exists
                ifn,found:=fnlookup.lmget(fn)

                if !found {

                    report(evalfs,-1,sf("forwarder function '%v' not found.",fn))

                    finish(false,ERR_SYNTAX)
                    break
                }

                // add parameters
                iargs := []interface{}{}
                var query string
                var fragment string

                switch method {
                case "GET","PUT","POST":
                    r.ParseForm()

                    // query kv pairs
                    query=purl.RawQuery
                    fragment=purl.Fragment

                    // post data
                    fvals:=url.Values{}
                    if r.PostForm!=nil {
                        fvals=r.PostForm
                    } else {
                        if r.Form!=nil {
                            fvals=r.Form
                        }
                    }
                    iargs=append(iargs,fvals)
                }

                var webcallstruct webstruct = webstruct{
                    host,
                    path,
                    method,
                    remote,
                    query,
                    fragment,
                    iargs,
                }

                // make call

                local_lastfs:=evalfs

                loc,id := GetNextFnSpace(fn+"@")
                calllock.Lock()
                ifn,_=fnlookup.lmget(fn)
                calltable[loc] = call_s{fs: id, base: ifn, caller: local_lastfs, retvar: "@temp"}
                calllock.Unlock()

                Call(MODE_NEW, loc, webcallstruct)

                _,ok := VarLookup(local_lastfs, "@temp")
                if ok {
                    tmp,_:=vget(local_lastfs,"@temp")
                    w.Write([]byte(sf("%v",tmp)))
                }

                serviced=true
            }

        }

    }

    if !serviced {
        // only try to serve locally once other rules are processed.
        for _,rule := range wr_copy {

            switch rule.code {
            case "s": // directly serve request

                // deal with rewrites first
                var new_path string
                var re = regexp.MustCompile(rule.in)

                if re.MatchString(path) {
                    // wlog("Attempting to serve %s\n",path)
                    if rule.in!="" && rule.mutation.(string)!="" {
                        // provided with a regex rewrite
                        new_path = re.ReplaceAllString(path, rule.mutation.(string))
                        if dlevel>5 { wlog("Rewrote %s to %s\n",path,new_path) }
                    } else {
                        // just proxy
                        new_path=path
                    }

                    // decode the reformed url
                    new_url,url_err:=url.Parse(new_path)
                    if url_err==nil {
                        if dlevel>5 { wlog("New URL Struct : %#v\n",new_url) }
                        new_path=new_url.Path

                    }

                    // form new file system path
                    weblock.RLock()
                    docroot:=web_handles[handle].docroot
                    weblock.RUnlock()
                    fp := filepath.Join(docroot,filepath.Clean(new_path))

                    // Return a 404 if the file doesn't exist
                    _, err := os.Stat(fp)
                    if err != nil {
                        if os.IsNotExist(err) {
                            wlog("Could not serve from %v to %v.\n",fp,remoteIp)
                            for _,fail_rule := range wr_copy {
                                switch fail_rule.code[0] {
                                case 'e':
                                    expectedStatusCode:=fail_rule.code[1:]
                                    if str.HasPrefix(path,fail_rule.in) {
                                        if expectedStatusCode=="" || expectedStatusCode==sf("%v","404") {
                                            http.Redirect(w,r,fail_rule.mutation.(string),http.StatusTemporaryRedirect)
                                            wlog("Redirected temporarily from %s to %s due to bad response.\n",path,fail_rule.mutation.(string))
                                            if WebSendMetrics { fireMetric(host,"e",http.StatusTemporaryRedirect) }
                                            serviced=true
                                            break
                                        }
                                    }
                                }
                            }
                            if !serviced { http.NotFound(w, r) }
                            return
                        }
                    }

                    // fetch file from store
                    s, err := ioutil.ReadFile(fp)

                    // read content type
                    contentType := mime.TypeByExtension(filepath.Ext(fp))
                    if contentType=="application/octet-stream" {
                        contentType = http.DetectContentType(s)
                    }
                    // pf("For %v found type : %v\n",fp,contentType)

                    // set type outbound
                    w.Header().Set("Content-Type", contentType)

                    // serve
                    if err==nil {
                        wlog("%s served %s to %s.\n",host,new_path,remoteIp)
                        w.Write([]byte(s))
                    } else {
                        wlog("Could not read file %v to serve to %v.\n",new_path,remoteIp)
                    }

                    if WebSendMetrics { fireMetric(host,"s",1) }
                    serviced=true
                    break
                } // endif regex match path
            } // endswitch rule.code
            if serviced { break }
        } // endfor rules
    }

    if serviced { return }
    http.NotFound(w, r)

}


// ZA LIBRARY FUNCTIONS //////////////////////////////////////////////////////////////////////

var    weblock = &sync.RWMutex{}
var    webrulelock = &sync.RWMutex{}

func buildNetLib() {

    // persistent http client
    web_tr = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
    web_client = &http.Client{Transport: web_tr}


    features["net"] = Feature{version: 1, category: "net"}
    categories["net"] = []string{"web_download", "web_head", "web_get", "web_custom", "web_post", "web_serve_start", "web_serve_stop", "web_serve_up", "web_serve_path", "web_serve_log_throttle", "web_display", "web_serve_decode", "web_serve_log", "web_max_clients", "net_interfaces", "html_escape", "html_unescape", "download", }


    // listenandserve always fires off a server we don't fully control. The Serve() part returns a non-nil
    // error under all circumstances. We'll have track handles against ip/port here.

    slhelp["net_interfaces"] = LibHelp{in: "", out: "device_string", action: "newline separated list of device names."}
    stdlib["net_interfaces"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        i,_:=net.Interfaces()
        a:=""
        for _,v:=range i {
            a=a+v.Name+"\n"
        }
        return a[:len(a)-1],nil
    }

    slhelp["web_display"] = LibHelp{in: "", out: "", action: "(debug)"}
    stdlib["web_display"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        webRoutesAll()
        return nil,nil
    }

    slhelp["web_max_clients"] = LibHelp{in: "", out: "int", action: "(read-only) returns the maximum permitted client count for a web server."}
    stdlib["web_max_clients"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        return int(MAX_CLIENTS),nil
    }

    slhelp["web_serve_log"] = LibHelp{in: "args", out: "", action: "Write arguments to the web log file, if available."}
    stdlib["web_serve_log"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        switch len(args) {
        case 0:
            return nil,nil
        case 1:
            wlog(args[0].(string))
        case 2:
            wlog(args[0].(string),args[1:])
        }
        return nil,nil
    }

    slhelp["web_serve_decode"] = LibHelp{in: "webcallstruct", out: "call_details",
            action: "Returns a struct representing details of an inbound web request.\n"+
                    "web_serve_decode() returns the following member fields:\n"+
                    "  .host (host:port), .method, .path, .remote_ip (remote_ip:port) and .data (POST data).",
}
    stdlib["web_serve_decode"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)==1 {
            switch args[0].(type) {
            case webstruct:
                return args[0].(webstruct),nil
            }
        }
        return nil,err
    }

    slhelp["web_serve_start"] = LibHelp{in: "docroot,port,vhost", out: "handle", action: "Returns an identifier for a new http server."}
    stdlib["web_serve_start"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

        // validate args (docroot,port,addr)

        if len(args)<2 {
            return "",errors.New("bad arguments to web_serve_start()")
        }

        var docroot string

        var host string
        var port int

        switch args[0].(type) {
        case string:
            docroot=args[0].(string)
        default:
            // bad
            return "",errors.New("bad arguments to web_serve_start()")
        }

        switch args[1].(type) {
        case int:
            port=args[1].(int)
        default:
            return "",errors.New("bad arguments to web_serve_start()")
        }

        if len(args)==3 {
            // host string
            switch args[2].(type) {
            case string:
                host=args[2].(string)
            default:
                return "",errors.New("bad arguments to web_serve_start()")
            }
        }

        // verify

        if port<=0 || port>65535 {
            return "",errors.New("port must be between 1 and 65535 in web_serve_start()")
        }

        // setup server

        var e error
        var srv http.Server
        var addr string

        /* Removed as inexplicit setting may have tcp6 issues.
        if host=="0.0.0.0" {
            host=""
        }
        */

        addr=host+":"+sf("%v",port)
        srv.Addr=addr

        // setup basic serving and the generic handler
        mux:=http.NewServeMux()
        srv.Handler = mux
        mux.HandleFunc("/", limitNumClients(webRouter, MAX_CLIENTS, evalfs))

        go func() {
            // e=srv.ListenAndServe()
            // @note: testing: manually enforce tcp4 to make docker happier.
            l, e := net.Listen("tcp4", addr)
            if e != nil {
                log.Fatal(err)
	        } else {
	            e=srv.Serve(l)
            }
        }()

        // have to give listenandserve a chance to fail and write 'e'
        // @note: there's probably a graceful way to do this.

        time.Sleep(100*time.Millisecond)


        if e==nil {
            // create a handle
            var uid string
            for ;; {
                b := make([]byte, 16)
                _, err := rand.Read(b)
                if err != nil { log.Fatal(err) }
                uid = sf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
                weblock.RLock()
                if _,exists:=web_handles[uid]; !exists {
                    weblock.RUnlock()
                    break
                }
                weblock.RUnlock()
            }
            // store in lookup table
            weblock.Lock()
            web_handles[uid]=web_table_entry{srv:&srv,mux:mux,docroot:docroot,addr:addr,host:host,port:port}
            weblock.Unlock()
            wlog("Started web service "+uid+"\n")
            return uid,nil
        } else {
            return "",e
        }
    }

    slhelp["web_serve_stop"] = LibHelp{in: "handle", out: "success_flag", action: "Stops and discards a running http server."}
    stdlib["web_serve_stop"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

        var uid string
        if len(args)==1 {
            switch args[0].(type) {
            case string:
                uid=args[0].(string)
            default:
                return false,nil
            }
        } else {
            return false,nil
        }

        webClose(uid)
        wlog("Stopped web service "+uid+"\n")
        weblock.Lock()
        delete(web_handles,uid)
        weblock.Unlock()
        return true,nil
    }

    slhelp["html_escape"] = LibHelp{in: "string", out: "string", action: "Converts HTML special characters to ampersand values."}
    stdlib["html_escape"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        var s string
        if len(args)==1 {
            switch args[0].(type) {
            case string:
                s=args[0].(string)
            default:
                return "",errors.New("html_escape() not provided a string value.")
            }
            return html.EscapeString(s),nil
        }
        return "",errors.New("html_escape() requires a string value.")
    }

    slhelp["html_unescape"] = LibHelp{in: "string", out: "string", action: "Converts a string containing ampersand values to include HTML special characters."}
    stdlib["html_unescape"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        var s string
        if len(args)==1 {
            switch args[0].(type) {
            case string:
                s=args[0].(string)
            default:
                return "",errors.New("html_unescape() not provided a string value.")
            }
            return html.UnescapeString(s),nil
        }
        return "",errors.New("html_unescape() requires a string value.")
    }


    slhelp["web_serve_up"] = LibHelp{in: "handle", out: "bool", action: "Checks if a web server is still running."}
    stdlib["web_serve_up"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

        var uid string
        if len(args)==1 {
            switch args[0].(type) {
            case string:
                uid=args[0].(string)
            default:
                return false,nil
            }
        } else {
            return false,nil
        }

        // @note: web should maybe change this into a test instead of an assumption
        //          or ensure that this value is updated periodically.
        _,exists:=web_handles[uid]
        return exists,nil

    }

    slhelp["web_serve_path"] = LibHelp{in: "handle,action_type,request_regex,new_path", out: "string", action: "Provides a traffic routing instruction to a web server."}
    stdlib["web_serve_path"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)!=4 {
            return false,errors.New("bad call args in web_serve_path()")
        }
        var uid string
        switch args[0].(type) {
        case string:
            uid=args[0].(string)
        default:
            return false,errors.New("argument 1 must be a string in web_serve_path()")
        }
        switch args[1].(type) {
        case string:
            switch str.ToLower(args[1].(string))[0] {
            case 's': // directly serve request
            case 'r': // redirect
            case 'p': // rewrite and reverse proxy
            case 'w': // rewrite response body @todo
            case 'f': // build func forwarder
            case 'e': // build rule for handling return failure
            default:
                return false,errors.New("argument 2 of web_serve_path() must be one of S, R, P, F, W or E")
            }
        default:
            return false,errors.New("argument 2 must be a string in web_serve_path()")
        }

        var rule web_rule
        rule.code=args[1].(string)

        switch args[2].(type) {
        case string:
            rule.in=args[2].(string)
        default:
            return false,errors.New("argument 3 must be a string in web_serve_path()")
        }

        switch args[3].(type) {
        case string:
            rule.mutation=args[3].(string) // name of za function to call
        default:
            return false,errors.New("argument 4 must be a string in web_serve_path()")
        }

        webrulelock.Lock()
        web_rules[uid]=append(web_rules[uid],rule)
        webrulelock.Unlock()

        return true,nil
    }

    // @todo: add a call for removing web_rules

    slhelp["web_serve_log_throttle"] = LibHelp{in: "start,freq", out: "", action: "Set the throttle controls for web server logging."}
    stdlib["web_serve_log_throttle"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)!=2 {
            return nil,errors.New("invalid args to web_serve_log_throttle()")
        }
        if sf("%T",args[0])!="int" || sf("%T",args[1])!="int" {
            return nil,errors.New("invalid args to web_serve_log_throttle()")
        }
        lastWlogStart=args[0].(int)
        lastWlogEvery=args[1].(int)
        // wlog("// throttle changed to start at %d and show every %d messages.\n",lastWlogStart,lastWlogEvery)
        return nil,nil
    }

/*

//  @note: really should write something for this, but i'm lazy and it needs thinking about substantially more
//  than the few seconds I've considered it for.

    slhelp["web_template"] = LibHelp{in: "handle,template_path", out: "processed_string", action: "Reads from either an absolute path or a docroot relative path (if handle not nil), with template instructions interpolated."}
    stdlib["web_template"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
    return nil,errors.New("Not implemented.")
    }
*/

    slhelp["web_head"] = LibHelp{in: "loc_string", out: "bool", action: "Makes a HEAD request of the given [#i1]loc_string[#i0]. Returns true if retrieved successfully."}
    stdlib["web_head"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1 { return false,errors.New("Bad args (count) to web_head()") }
        _, down_code := head(args[0].(string))
        if down_code>299 {
            return false,nil
        }
        return true,nil
    }

    slhelp["web_get"] = LibHelp{in: "loc_string", out: "list", action: "Returns a [#i1]list[#i0] with content downloaded from [#i1]loc_string[#i0]. list[0] is the content string. list[1] is the header."}
    stdlib["web_get"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1 { return false,errors.New("Bad args (count) to web_get()") }
        s, down_code, header := download(args[0].(string))
        if down_code>299 {
            return "", nil
        }
        return []interface{}{string(s),header}, nil
    }

    slhelp["web_custom"] = LibHelp{in: "method_string,loc_string,headers_array", out: "string", action: "Returns a [#i1]string[#i0] with content downloaded from [#i1]loc_string[#i0]."}
    stdlib["web_custom"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        var method_string, loc_string string
        var headers =make(map[string]string)

        switch len(args) {
        case 2:
            if sf("%T",args[0])!="string" || sf("%T",args[1])!="string" {
                return []interface{}{"Invalid arguments (type) in web_custom()",nil,400},nil
            }
            method_string   = args[0].(string)
            loc_string      = args[1].(string)
        case 3:
            if sf("%T",args[0])!="string" || sf("%T",args[1])!="string" || sf("%T",args[2])!="map[string]interface {}" {
                return []interface{}{"Invalid arguments (type) in web_custom()",nil,400},nil
            }
            method_string   = args[0].(string)
            loc_string      = args[1].(string)
            for k,v:=range args[2].(map[string]interface{}) { headers[k]=v.(string) }
        default:
            return []interface{}{"Invalid arguments (count) in web_custom()",nil,400},nil
        }
        // headers
        request, err := http.NewRequest(method_string, loc_string, nil)
        if err != nil { return []interface{}{"Could not create a new HTTP request in web_custom()",nil,400},nil }
        for k,v:=range headers {
            request.Header.Add(k,v)
        }
        // request
        resp, err := web_client.Do(request)
        if err == nil {
            defer resp.Body.Close()
            if resp.StatusCode>299 {
                return []interface{}{"",nil,resp.StatusCode},nil
            }
            s, err := ioutil.ReadAll(resp.Body)
            if err == nil {
                return []interface{}{string(s), resp.Header, resp.StatusCode},nil
            }
        }
        return []interface{}{"404 - Not found in web_custom()",nil,404},nil
    }

    slhelp["web_post"] = LibHelp{in: "loc_string,key_value_list", out: "result_string", action: "Perform a HTTP POST."}
    stdlib["web_post"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)!=2 {
            return "",errors.New("invalid args to web_post()")
        }
        if sf("%T",args[0]) != "string" {
            return "",errors.New("invalid args to web_post()")
        }
        s, up_ok := post(args[0].(string),args[1])
        if !up_ok {
            return "",errors.New(sf("Could not post to %v",args[0].(string)))
        }
        return string(s),nil
    }

    slhelp["download"] = LibHelp{in: "url_string", out: "local_name", action: "Downloads from URL [#i1]url_string[#i0] and stores the returned data in the file [#i1]local_name[#i0]. Includes console feedback."}
    stdlib["download"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if sf("%T",args[0])!="string" {
            return "",errors.New("Bad args (type) in download()")
        }
        if len(args)!=1 { return "",errors.New("Bad args (count) in download()") }
        fname, down_code := FileDownload(args[0].(string))
        if down_code<300 { return fname, nil }
        return "", nil
    }

    slhelp["web_download"] = LibHelp{in: "url_string,local_file", out: "bool_okay", action: "Downloads from URL [#i1]url_string[#i0] and stores the returned data in the file [#i1]local_file[#i0]."}
    stdlib["web_download"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)!=2 { return false,errors.New("Bad args (count) to web_download()") }
        cont, down_code, header := download(args[0].(string))
        _=header
        if down_code<300 {
            ioutil.WriteFile(args[1].(string), cont, default_WriteMode)
            return true, nil
        }
        return false, nil
    }

}

func post(loc string,valueMap interface{}) ([]byte,bool) {
    var s []byte
    // tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
    // client := &http.Client{Transport: tr}
    vlist := url.Values{}
    switch valueMap.(type) {
    case map[string]interface{}:
        for k,val := range valueMap.(map[string]interface{}) { vlist.Set(k,sf("%v",val)) }
    case map[string]int:
        for k,val := range valueMap.(map[string]int) { vlist.Set(k,sf("%v",val)) }
    case map[string]float64:
        for k,val := range valueMap.(map[string]float64) { vlist.Set(k,sf("%v",val)) }
    case map[string]string:
        for k,val := range valueMap.(map[string]string) { vlist.Set(k,sf("%v",val)) }
    default:
        return []byte{},false
    }
    resp, err := web_client.PostForm(loc,vlist)
    if err!=nil {
        return []byte{},false
    }
    defer resp.Body.Close()
    if err==nil {
        s,err=ioutil.ReadAll(resp.Body)
        if err==nil {
            return s,true
        }
    }
    return []byte{},false
}


func download(loc string) ([]byte, int, http.Header) {
    var s []byte

    resp, err := web_client.Get(loc)
    // req.Header.Set("name", "value")
    if err == nil {
        defer resp.Body.Close()
        if resp.StatusCode>299 {
            return []byte{},resp.StatusCode,nil
        }
        s, err = ioutil.ReadAll(resp.Body)
        if err == nil {
            return s, resp.StatusCode,resp.Header
        }
    }
    return []byte{}, 404, nil
}

// @todo: add more return information than just the body. (status code,etc)
func head(loc string) ([]byte, int) {
    var s []byte

    resp, err := web_client.Head(loc)
    if err!=nil {
        return []byte{},404
    }
    defer resp.Body.Close()
    if err == nil {
        if resp.StatusCode>299 {
            return []byte{}, resp.StatusCode
        }
        s, err = ioutil.ReadAll(resp.Body)
        if err == nil {
            return s, resp.StatusCode
        }
    }
    return []byte{}, resp.StatusCode
}



