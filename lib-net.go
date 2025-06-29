//go:build !test
// +build !test

package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"html"
	"io/ioutil"
	"log"
	"math/rand"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	str "strings"
	"sync"
	"sync/atomic"
	"time"
)

/* · web_serve_*:
   · these are not intended for anything complex.
   · quick and dirty page serving and proxying
   · no intention to add header or method inspection or anything other than what is below.
*/

var web_tr *http.Transport
var web_client *http.Client

var web_log_handle *os.File

// TYPES ////////////////////////////////////////////////////////////////////////////////////////

type web_table_entry struct {
	docroot string
	addr    string // addr is the host:port pair to serve traffic from
	host    string // for traffic routing rules
	port    int    // which tcp port to create a multiplexor on.
	srv     *http.Server
	mux     *http.ServeMux
}

var web_handles = make(map[string]web_table_entry)

type web_rule struct {
	code     string
	in       string
	mutation any
}

var web_rules = make(map[string][]web_rule)

// HELPER FUNCTIONS /////////////////////////////////////////////////////////////////////////////

var lastWlogMsg string
var lastWlogCount int
var lastWlogStart int = 5
var lastWlogEvery int = 10000
var throttle bool
var WlogDisplay = true

// Counting semaphore using a buffered channel
func limitNumClients(f http.HandlerFunc, maxClients int, evalfs uint32, ident *[]Variable) http.HandlerFunc {
	sema := make(chan struct{}, maxClients)
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		ctx = context.WithValue(ctx, "evalfs", evalfs)
		req = req.WithContext(ctx)

		sema <- struct{}{}
		defer func() { <-sema }()
		f(w, req)
	}
}

// web access logging
func wlog(s string, va ...any) {
	lastlock.Lock()
	if log_web {
		if len(va) > 0 {
			s = sf(s, va...)
		}
		throttleEnded := false
		if lastWlogMsg == s { // repeat message
			lastWlogCount++
			if lastWlogCount == lastWlogStart { // begin throttling
				throttle = true
			}
			if throttle {
				WlogDisplay = false
				if (lastWlogCount-lastWlogStart)%lastWlogEvery == 0 { // let one through
					WlogDisplay = true
				}
			}
		} else {
			lastWlogCount = 0
			lastWlogMsg = s
			if throttle {
				throttleEnded = true
			}
			throttle = false
			WlogDisplay = true
		}

		if WlogDisplay {
			// Use main logging infrastructure with background queue
			if web_log_file != "" {
				// Create log fields for web access entry
				var webFields map[string]any
				if jsonLoggingEnabled {
					webFields = make(map[string]any)
					webFields["type"] = "web_access"
					// Copy main log fields if any
					for k, v := range logFields {
						webFields[k] = v
					}
				}

				// Handle throttling messages through queue
				if throttle {
					message := sf("// skipped %d repeat messages.", lastWlogEvery)
					request := LogRequest{
						Message:     message,
						Fields:      webFields,
						IsJSON:      jsonLoggingEnabled,
						IsError:     false,
						IsWebAccess: true,
						DestFile:    web_log_file,
						HTTPStatus:  0,
						Level:       LOG_INFO,
						Timestamp:   time.Now(),
					}
					queueLogRequest(request)
				}
				if throttleEnded {
					message := sf("// stopped throttling messages.")
					request := LogRequest{
						Message:     message,
						Fields:      webFields,
						IsJSON:      jsonLoggingEnabled,
						IsError:     false,
						IsWebAccess: true,
						DestFile:    web_log_file,
						HTTPStatus:  0,
						Level:       LOG_INFO,
						Timestamp:   time.Now(),
					}
					queueLogRequest(request)
				}

				// Queue the main message
				request := LogRequest{
					Message:     s,
					Fields:      webFields,
					IsJSON:      jsonLoggingEnabled,
					IsError:     false,
					IsWebAccess: true,
					DestFile:    web_log_file,
					HTTPStatus:  0,        // Will be set by caller if available
					Level:       LOG_INFO, // Web access logs default to INFO level
					Timestamp:   time.Now(),
				}
				queueLogRequest(request)
			} else {
				// Fallback: print to console when no access file configured
				if throttle {
					pf("// skipped %d repeat messages.\n", lastWlogEvery)
				}
				if throttleEnded {
					pf("// stopped throttling messages.\n")
				}
				pf(s)
			}
		}
	}
	lastlock.Unlock()
}

// wlog_with_status logs web access with HTTP status code for enhanced error logging
func wlog_with_status(httpStatus int, s string, va ...any) {

	// early exit
	if !log_web {
		return
	}

	lastlock.Lock()

	s = sf(s, va...)

	lastWlogMsg = s
	WlogDisplay = true

	if WlogDisplay {
		// Use main logging infrastructure with background queue
		if web_log_file != "" {
			// Create log fields for web access entry
			var webFields map[string]any
			if jsonLoggingEnabled {
				webFields = make(map[string]any)
				webFields["type"] = "web_access"
				// Copy main log fields if any
				for k, v := range logFields {
					webFields[k] = v
				}
			}

			// Determine log level based on HTTP status code
			var logLevel int
			if httpStatus >= 500 {
				logLevel = LOG_ERR // Server errors
			} else if httpStatus >= 400 {
				logLevel = LOG_WARNING // Client errors
			} else if httpStatus >= 300 {
				logLevel = LOG_NOTICE // Redirects
			} else {
				logLevel = LOG_INFO // Success responses
			}

			// Queue the message with HTTP status
			request := LogRequest{
				Message:     s,
				Fields:      webFields,
				IsJSON:      jsonLoggingEnabled,
				IsError:     false, // Will be set by queue processor based on HTTPStatus
				IsWebAccess: true,
				DestFile:    web_log_file,
				HTTPStatus:  httpStatus,
				Level:       logLevel,
				Timestamp:   time.Now(),
			}
			queueLogRequest(request)
		} else {
			// Fallback: print to console when no access file configured
			pf(s)
		}
	}

	lastlock.Unlock()
}

func webLookup(host string) string {
	weblock.RLock()
	for k, v := range web_handles {
		if v.addr == host {
			weblock.RUnlock()
			return k
		}
	}
	weblock.RUnlock()
	return ""
}

func webClose(h string) {
	pf("* Closing server (%s).\n", h)
	weblock.Lock()
	web_handles[h].srv.Shutdown(context.Background())
	delete(web_handles, h)
	weblock.Unlock()
}

func webCloseAll() {
	weblock.Lock()
	for h, s := range web_handles {
		pf("* Closing server (%s) : %+v\n", h, s)
		s.srv.Shutdown(context.Background())
		delete(web_handles, h)
	}
	weblock.Unlock()
}

func webRoutesAll() {
	weblock.RLock()
	for uid, entry := range web_handles {
		host := entry.addr
		port := entry.port
		pf("Service Host : %s / %d\n", host, port)
		webRoutes(uid)
	}
	weblock.RUnlock()
}

func webRoutes(uid string) {
	for _, action := range web_rules[uid] {
		pf("* %s : %s %s -> %+v\n", uid, action.code, action.in, action.mutation)
	}

}

type webstruct struct {
	host     string
	path     string
	method   string
	remote   string
	query    string
	fragment string
	data     []any
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

	evalfs, _ := r.Context().Value("evalfs").(uint32)

	// we do not log by default
	// if globalvar log_web is true, then log to web_log_file (global)

	// we also throw out some debug info too when enabled. this should not be the default!
	// it really slows down request processing.

	method := r.Method     // get, put, etc
	purl := r.URL          // provided url
	header := r.Header     // map[string][]string
	host := r.Host         // string from either Header or URL : this appears to include the port number on it
	remote := r.RemoteAddr // requester ip:port string
	path := purl.Path      // .Path (may or may not including leading path when relative)
	scheme := purl.Scheme  // http/https/etc
	if scheme == "" {
		scheme = "http"
	}
	_ = method
	_ = header

	srvAddr := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	srvStr := srvAddr.String()

	localSplitAt := str.IndexByte(srvStr, ':')
	localPort := srvStr[localSplitAt+1:]

	remoteSplitAt := str.IndexByte(remote, ':')
	remoteIp := remote[:remoteSplitAt]

	serviced := false // was a rule acted upon?
	handle := webLookup(host)

	if handle == "" {
		port := ""
		// extract port
		reqHostSplitAt := str.IndexByte(host, ':')
		if reqHostSplitAt > -1 {
			reqPort := host[reqHostSplitAt+1:]
			port = reqPort
		} else {
			// .. or use port from request context
			port = localPort
		}
		handle = webLookup("0.0.0.0:" + port)
	}

	// deal with forced redirects and reverse proxying first

	webrulelock.RLock()
	wr_copy := web_rules[handle]
	webrulelock.RUnlock()

	for _, rule := range wr_copy {

		switch rule.code {

		case "r": // redirect

			// should reply with a redirect code
			// pointing the requesting client to go
			// to mutation instead of host+path

			if str.HasPrefix(path, rule.in) {
				http.Redirect(w, r, rule.mutation.(string), http.StatusMovedPermanently)
				wlog("Redirected from %s to %s.\n", path, rule.mutation.(string))
				serviced = true
			}

		case "p": // rewrite and reverse proxy

			// build new_path based on path in rule.in + rule.mutation
			// make a client request from here to host+new_path
			// pass result back including status codes

			var re = regexp.MustCompile(rule.in)
			if re.MatchString(path) {

				var new_path string
				if rule.in != "" && rule.mutation.(string) != "" {
					// provided with a regex rewrite
					new_path = re.ReplaceAllString(path, rule.mutation.(string))
				} else {
					// just proxy
					new_path = rule.in
				}

				// make a client request
				var content []byte
				var down_code int = -1

				switch method {

				case "GET":
					nq := new_path
					if purl.RawQuery != "" {
						nq += "?" + purl.RawQuery
					}
					if purl.Fragment != "" {
						nq += "#" + purl.Fragment
					}

					// log_nq,_:=url.QueryUnescape(nq)
					// wlog("* GET Proxying this URL: %s\n",log_nq)

					content, down_code, header = download(nq)

					w.Header().Add("proxied-by-za", "true")
					for k, v := range header {
						w.Header().Set(k, str.Join(v, ","))
					}

				case "HEAD":
					nq := new_path
					if purl.RawQuery != "" {
						nq += "?" + purl.RawQuery
					}
					if purl.Fragment != "" {
						nq += "#" + purl.Fragment
					}

					// log_nq,_:=url.QueryUnescape(nq)
					// wlog("* HEAD Proxying this URL: %s\n",log_nq)

					content, down_code = head(nq)

				case "POST", "PUT":

					// get the form data from the request
					r.ParseForm()
					fvals := url.Values{}
					if r.PostForm != nil {
						fvals = r.PostForm
					} else {
						if r.Form != nil {
							fvals = r.Form
						}
					}

					// .. and feed it into the backend request
					tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: false}}
					client := &http.Client{Transport: tr}
					resp, err := client.PostForm(new_path, fvals)
					defer resp.Body.Close()
					down_code = resp.StatusCode
					if err == nil {
						content, err = ioutil.ReadAll(resp.Body)
					}
					// wlog("POST request: Form value provided: %#v\n",r.URL.RawQuery)

				}

				if down_code > 299 {
					// now look for matching rules for failure
					for _, fail_rule := range wr_copy {
						switch fail_rule.code[0] {
						case 'e':
							// fail_rule.in is which path prefix to match on
							// fail_rule.mutation is where to redirect to on error
							expectedStatusCode := fail_rule.code[1:]
							if str.HasPrefix(path, fail_rule.in) {
								if expectedStatusCode == "" || expectedStatusCode == sf("%d", down_code) {
									http.Redirect(w, r, fail_rule.mutation.(string), http.StatusTemporaryRedirect)
									wlog("Redirected temporarily from %s to %s due to bad response.\n", path, fail_rule.mutation.(string))
									serviced = true
									break
								}
							}
						}
					}
				} else {
					// on success:
					// case "w": // @todo: rewrite response body
					// wlog("Proxy read the page successfully. Writing back to remote client.\n")
					wlog("%s served %s to %s.\n", host, new_path, remoteIp)
					w.Write([]byte(content))
				}
				serviced = true
			}

		case "f":

			// uid, "f", "regex_path", "func"
			// on regex match, build argument list from POST params.
			//   The list should have it's arguments cleansed for the usual bad things.
			//   Make a call to za function "func" with those as the arguments.
			// if return == not_empty then w.Write(ret_string)
			// if return == empty then check "e" rules,
			//   e rule may redirect or send a specific error

			var re = regexp.MustCompile(rule.in)
			if re.MatchString(path) {

				fn := rule.mutation.(string)

				// check that function exists
				ifn, found := fnlookup.lmget(fn)

				if !found {

					pf("forwarder function '%v' not found.", fn)
					finish(false, ERR_SYNTAX)
					break
				}

				// add parameters
				iargs := []any{}
				var query string
				var fragment string

				switch method {
				case "GET", "PUT", "POST":
					r.ParseForm()

					// query kv pairs
					query = purl.RawQuery
					fragment = purl.Fragment

					// post data
					fvals := url.Values{}
					if r.PostForm != nil {
						fvals = r.PostForm
					} else {
						if r.Form != nil {
							fvals = r.Form
						}
					}
					iargs = append(iargs, fvals)
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

				ifn, _ = fnlookup.lmget(fn)
				loc, _ := GetNextFnSpace(true, fn+"@", call_s{prepared: true, base: ifn, caller: evalfs})

				ctx := withProfilerContext(context.Background())
				var ident = make([]Variable, identInitialSize)
				atomic.AddInt32(&concurrent_funcs, 1)
				rcount, _, _, errVal := Call(ctx, MODE_NEW, &ident, loc, ciLnet, false, nil, "", []string{}, webcallstruct)
				atomic.AddInt32(&concurrent_funcs, -1)

				if errVal != nil {
					panic(sf("error in web router called function (%s)", fn))
				}

				calllock.Lock()
				tmp := calltable[loc].retvals
				switch rcount {
				case 0:
					w.Write([]byte(""))
				case 1:
					switch tmp.(type) {
					case nil:
						// translate bad func call to error page
						http.NotFound(w, r)
					default:
						w.Write([]byte(sf("%v", tmp.([]any)[0])))
					}
				default:
					w.Write([]byte(sf("%v", tmp.([]any)[0])))
				}

				calltable[loc].gcShyness = 40
				calltable[loc].gc = true

				calllock.Unlock()
				serviced = true
			}

		}

	}

	if !serviced {
		// only try to serve locally once other rules are processed.
		for _, rule := range wr_copy {

			switch rule.code {
			case "s": // directly serve request

				// deal with rewrites first
				var new_path string
				var re = regexp.MustCompile(rule.in)

				if re.MatchString(path) {
					// wlog("Attempting to serve %s\n",path)
					if rule.in != "" && rule.mutation.(string) != "" {
						// provided with a regex rewrite
						new_path = re.ReplaceAllString(path, rule.mutation.(string))
					} else {
						// just proxy
						new_path = path
					}

					// decode the reformed url
					new_url, url_err := url.Parse(new_path)
					if url_err == nil {
						new_path = new_url.Path

					}

					// form new file system path
					weblock.RLock()
					docroot := web_handles[handle].docroot
					weblock.RUnlock()
					fp := filepath.Join(docroot, filepath.Clean(new_path))

					// Return a 404 if the file doesn't exist
					_, err := os.Stat(fp)
					if err != nil {
						if os.IsNotExist(err) {
							wlog("Could not serve from %v to %v.\n", fp, remoteIp)
							for _, fail_rule := range wr_copy {
								switch fail_rule.code[0] {
								case 'e':
									expectedStatusCode := fail_rule.code[1:]
									if str.HasPrefix(path, fail_rule.in) {
										if expectedStatusCode == "" || expectedStatusCode == sf("%v", "404") {
											http.Redirect(w, r, fail_rule.mutation.(string), http.StatusTemporaryRedirect)
											wlog("Redirected temporarily from %s to %s due to bad response.\n", path, fail_rule.mutation.(string))
											serviced = true
											break
										}
									}
								}
							}
							if !serviced {
								http.NotFound(w, r)
							}
							return
						}
					}

					// fetch file from store
					s, err := ioutil.ReadFile(fp)

					// read content type
					contentType := mime.TypeByExtension(filepath.Ext(fp))
					if contentType == "application/octet-stream" {
						contentType = http.DetectContentType(s)
					}

					// set type outbound
					w.Header().Set("Content-Type", contentType)

					// serve
					if err == nil {
						wlog("%s served %s to %s.\n", host, new_path, remoteIp)
						w.Write([]byte(s))
					} else {
						wlog("Could not read file %v to serve to %v.\n", new_path, remoteIp)
					}

					serviced = true
					break
				} // endif regex match path
			} // endswitch rule.code
			if serviced {
				break
			}
		} // endfor rules
	}

	if serviced {
		return
	}
	http.NotFound(w, r)

}

// ZA LIBRARY FUNCTIONS //////////////////////////////////////////////////////////////////////

var weblock = &sync.RWMutex{}
var webrulelock = &sync.RWMutex{}

func buildNetLib() {

	// persistent http client
	web_tr = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: false}}
	web_client = &http.Client{Transport: web_tr}

	features["net"] = Feature{version: 1, category: "net"}
	categories["net"] = []string{"web_download", "web_head", "web_get", "web_custom", "web_post", "web_serve_start", "web_serve_stop", "web_serve_up", "web_serve_path", "web_serve_log_throttle", "web_display", "web_serve_decode", "web_serve_log", "web_max_clients", "net_interfaces", "html_escape", "html_unescape", "download"}

	// listenandserve always fires off a server we don't fully control. The Serve() part returns a non-nil
	// error under all circumstances. We'll have track handles against ip/port here.

	slhelp["net_interfaces"] = LibHelp{in: "", out: "device_string", action: "newline separated list of device names."}
	stdlib["net_interfaces"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("net_interfaces", args, 0); !ok {
			return nil, err
		}
		i, _ := net.Interfaces()
		a := ""
		for _, v := range i {
			a = a + v.Name + "\n"
		}
		return a[:len(a)-1], nil
	}

	slhelp["web_display"] = LibHelp{in: "", out: "", action: "Show configured request routing."}
	stdlib["web_display"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("web_display", args, 0); !ok {
			return nil, err
		}
		webRoutesAll()
		return nil, nil
	}

	slhelp["web_max_clients"] = LibHelp{in: "", out: "int", action: "(read-only) returns the maximum permitted client count for a web server."}
	stdlib["web_max_clients"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("web_max_clients", args, 0); !ok {
			return nil, err
		}
		return int(MAX_CLIENTS), nil
	}

	slhelp["web_serve_log"] = LibHelp{in: "args", out: "", action: "Write arguments to the web log file, if available."}
	stdlib["web_serve_log"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("web_serve_log", args, 0); !ok {
			return nil, err
		}
		switch len(args) {
		case 0:
			return nil, nil
		case 1:
			wlog(args[0].(string))
		case 2:
			wlog(args[0].(string), args[1:])
		}
		return nil, nil
	}

	slhelp["web_serve_decode"] = LibHelp{in: "webcallstruct", out: "call_details",
		action: "Returns a struct representing details of an inbound web request.\n" +
			"web_serve_decode() returns the following member fields:\n" +
			"  .host (host:port), .method, .path, .remote (remote_ip:port) and .data (POST data).",
	}
	stdlib["web_serve_decode"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("web_serve_decode", args, 1, "1", "main.webstruct"); !ok {
			return nil, err
		}
		if len(args) == 1 {
			switch args[0].(type) {
			case webstruct:
				return args[0].(webstruct), nil
			}
		}
		return nil, err
	}

	slhelp["web_serve_start"] = LibHelp{in: "docroot,port[,vhost]", out: "handle", action: "Returns an identifier for a new http server."}
	stdlib["web_serve_start"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("web_serve_start", args, 2,
			"3", "string", "int", "string",
			"2", "string", "int"); !ok {
			return nil, err
		}

		// validate args (docroot,port,addr)

		var host string
		docroot := args[0].(string)
		port := args[1].(int)

		if len(args) == 3 {
			host = args[2].(string)
		}

		// verify
		if port <= 0 || port > 65535 {
			return "", errors.New("port must be between 1 and 65535 in web_serve_start()")
		}

		// setup server

		var e error
		var srv http.Server
		var addr string

		addr = host + ":" + sf("%v", port)
		srv.Addr = addr

		// setup basic serving and the generic handler
		mux := http.NewServeMux()
		srv.Handler = mux
		mux.HandleFunc("/", limitNumClients(webRouter, MAX_CLIENTS, evalfs, ident))

		go func() {
			// e=srv.ListenAndServe()
			// @note: testing: manually enforce tcp4 to make docker happier.
			l, e := net.Listen("tcp4", addr)
			if e != nil {
				log.Fatal(err)
			} else {
				e = srv.Serve(l)
			}
		}()

		// have to give listenandserve a chance to fail and write 'e'
		// @note: there's probably a graceful way to do this.

		time.Sleep(100 * time.Millisecond)

		if e == nil {
			// create a handle
			var uid string
			for {
				b := make([]byte, 16)
				_, err := rand.Read(b)
				if err != nil {
					log.Fatal(err)
				}
				uid = sf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
				weblock.RLock()
				if _, exists := web_handles[uid]; !exists {
					weblock.RUnlock()
					break
				}
				weblock.RUnlock()
			}
			// store in lookup table
			weblock.Lock()
			web_handles[uid] = web_table_entry{srv: &srv, mux: mux, docroot: docroot, addr: addr, host: host, port: port}
			weblock.Unlock()
			wlog("Started web service %s\n", uid)
			return uid, nil
		} else {
			return "", e
		}
	}

	slhelp["web_serve_stop"] = LibHelp{in: "handle", out: "success_flag", action: "Stops and discards a running http server."}
	stdlib["web_serve_stop"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("web_serve_stop", args, 1, "1", "string"); !ok {
			return nil, err
		}
		uid := args[0].(string)
		webClose(uid)
		wlog("Stopped web service %s\n", uid)
		weblock.Lock()
		delete(web_handles, uid)
		weblock.Unlock()
		return true, nil
	}

	slhelp["html_escape"] = LibHelp{in: "string", out: "string", action: "Converts HTML special characters to ampersand values."}
	stdlib["html_escape"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("html_escape", args, 1, "1", "string"); !ok {
			return nil, err
		}
		return html.EscapeString(args[0].(string)), nil
	}

	slhelp["html_unescape"] = LibHelp{in: "string", out: "string", action: "Converts a string containing ampersand values to include HTML special characters."}
	stdlib["html_unescape"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("html_unescape", args, 1, "1", "string"); !ok {
			return nil, err
		}
		return html.UnescapeString(args[0].(string)), nil
	}

	slhelp["web_serve_up"] = LibHelp{in: "handle", out: "bool", action: "Checks if a web server is still running."}
	stdlib["web_serve_up"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("web_serve_up", args, 1, "1", "string"); !ok {
			return nil, err
		}
		uid := args[0].(string)
		// @note: we should maybe change this into a test instead of an assumption
		//          or ensure that this value is updated periodically.
		weblock.RLock()
		_, exists := web_handles[uid]
		weblock.RUnlock()
		return exists, nil

	}

	slhelp["web_serve_path"] = LibHelp{in: "handle,action_type,request_regex,new_path", out: "string", action: "Provides a traffic routing instruction to a web server."}
	stdlib["web_serve_path"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("web_serve_path", args, 1,
			"4", "string", "string", "string", "string"); !ok {
			return nil, err
		}

		uid := args[0].(string)
		switch str.ToLower(args[1].(string))[0] {
		case 's': // directly serve request
		case 'r': // redirect
		case 'p': // rewrite and reverse proxy
		case 'w': // rewrite response body @todo
		case 'f': // build func forwarder
		case 'e': // build rule for handling return failure
		default:
			return false, errors.New("argument 2 of web_serve_path() must be one of S, R, P, F, W or E")
		}

		var rule web_rule
		rule.code = args[1].(string)
		rule.in = args[2].(string)
		rule.mutation = args[3].(string) // name of za function to call

		if !regexWillCompile(rule.in) {
			return false, fmt.Errorf("invalid regex in web_serve_path() : %s", rule.in)
		}

		webrulelock.Lock()
		web_rules[uid] = append(web_rules[uid], rule)
		webrulelock.Unlock()

		return true, nil
	}

	slhelp["web_serve_log_throttle"] = LibHelp{in: "start,freq", out: "", action: "Set the throttle controls for web server logging."}
	stdlib["web_serve_log_throttle"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("web_serve_log_throttle", args, 1, "2", "int", "int"); !ok {
			return nil, err
		}
		lastWlogStart = args[0].(int)
		lastWlogEvery = args[1].(int)
		// wlog("// throttle changed to start at %d and show every %d messages.\n",lastWlogStart,lastWlogEvery)
		return nil, nil
	}

	/*

	   //  @note: really should write something for this, but i'm lazy and it needs thinking about substantially more
	   //  than the few seconds I've considered it for.

	       slhelp["web_template"] = LibHelp{in: "handle,template_path", out: "processed_string", action: "Reads from either an absolute path or a docroot relative path (if handle not nil), with template instructions interpolated."}
	       stdlib["web_template"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
	       return nil,errors.New("Not implemented.")
	       }
	*/

	slhelp["web_head"] = LibHelp{in: "loc_string", out: "bool", action: "Makes a HEAD request of the given [#i1]loc_string[#i0]. Returns true if retrieved successfully."}
	stdlib["web_head"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("web_head", args, 1, "1", "string"); !ok {
			return nil, err
		}
		_, down_code := head(args[0].(string))
		if down_code > 299 {
			return false, nil
		}
		return true, nil
	}

	slhelp["web_get"] = LibHelp{in: "loc_string", out: "structure", action: "Returns a [#i1]structure[#i0] with content downloaded from [#i1]loc_string[#i0]. [#i1].result[#i0] is the content string. [#i1].code[#i0] is the status code."}
	stdlib["web_get"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("web_get", args, 1, "1", "string"); !ok {
			return nil, err
		}
		s, down_code, _ := download(args[0].(string))
		if down_code > 299 {
			return web_info{result: "", code: int(down_code)}, nil
		}
		return web_info{result: string(s), code: int(down_code)}, nil
	}

	slhelp["web_custom"] = LibHelp{in: "method_string,loc_string[,[string]assoc_headers_strings]", out: "string", action: "Returns a [#i1]string[#i0] with content downloaded from [#i1]loc_string[#i0]."}
	stdlib["web_custom"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("web_custom", args, 2,
			"3", "string", "string", "map[string]interface{ }",
			"2", "string", "string"); !ok {
			return nil, err
		}

		method_string := args[0].(string)
		loc_string := args[1].(string)

		var headers = make(map[string]string)
		switch len(args) {
		case 3:
			for k, v := range args[2].(map[string]any) {
				headers[k] = v.(string)
			}
		}

		// headers
		request, err := http.NewRequest(method_string, loc_string, nil)
		if err != nil {
			return []any{"Could not create a new HTTP request in web_custom()", nil, 400}, nil
		}
		for k, v := range headers {
			request.Header.Add(k, v)
		}
		// request
		resp, err := web_client.Do(request)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode > 299 {
				return []any{"", nil, resp.StatusCode}, nil
			}
			s, err := ioutil.ReadAll(resp.Body)
			if err == nil {
				return []any{string(s), resp.Header, resp.StatusCode}, nil
			}
		}
		return []any{"404 - Not found in web_custom()", nil, 404}, nil
	}

	slhelp["web_post"] = LibHelp{in: "loc_string,[]key_value_list", out: "result_string", action: "Perform a HTTP POST."}
	stdlib["web_post"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("web_post", args, 1,
			"2", "string", "[]interface {}"); !ok {
			return nil, err
		}

		s, up_ok := post(args[0].(string), args[1])
		if !up_ok {
			return "", errors.New(sf("Could not post to %v", args[0].(string)))
		}
		return string(s), nil
	}

	slhelp["download"] = LibHelp{in: "url_string", out: "local_name", action: "Downloads from URL [#i1]url_string[#i0] and stores the returned data in the file [#i1]local_name[#i0]. Includes console feedback."}
	stdlib["download"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("download", args, 1, "1", "string"); !ok {
			return nil, err
		}

		fname, down_code := FileDownload(args[0].(string))
		if down_code < 300 {
			return fname, nil
		}
		return "", nil
	}

	slhelp["web_download"] = LibHelp{in: "url_string,local_file", out: "bool_okay", action: "Downloads from URL [#i1]url_string[#i0] and stores the returned data in the file [#i1]local_file[#i0]."}
	stdlib["web_download"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("web_download", args, 1, "2", "string", "string"); !ok {
			return nil, err
		}
		cont, down_code, _ := download(args[0].(string))
		if down_code < 300 {
			ioutil.WriteFile(args[1].(string), cont, default_WriteMode)
			return true, nil
		}
		return false, nil
	}

}

func post(loc string, valueMap any) ([]byte, bool) {
	var s []byte
	vlist := url.Values{}
	switch valueMap := valueMap.(type) {
	case map[string]int:
		for k, val := range valueMap {
			vlist.Set(k, sf("%v", val))
		}
	case map[string]float64:
		for k, val := range valueMap {
			vlist.Set(k, sf("%v", val))
		}
	case map[string]string:
		for k, val := range valueMap {
			vlist.Set(k, sf("%v", val))
		}
	case map[string]any:
		for k, val := range valueMap {
			vlist.Set(k, sf("%v", val))
		}
	default:
		return []byte{}, false
	}
	resp, err := web_client.PostForm(loc, vlist)
	if err != nil {
		return []byte{}, false
	}
	defer resp.Body.Close()
	if err == nil {
		s, err = ioutil.ReadAll(resp.Body)
		if err == nil {
			return s, true
		}
	}
	return []byte{}, false
}

func download(loc string) ([]byte, int, http.Header) {
	var s []byte

	resp, err := web_client.Get(loc)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode > 299 {
			return []byte{}, resp.StatusCode, nil
		}
		s, err = ioutil.ReadAll(resp.Body)
		if err == nil {
			return s, resp.StatusCode, resp.Header
		}
	}
	return []byte{}, 404, nil
}

func head(loc string) ([]byte, int) {
	var s []byte

	resp, err := web_client.Head(loc)
	if err != nil {
		return []byte{}, 404
	}
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode > 299 {
			return []byte{}, resp.StatusCode
		}
		s, err = ioutil.ReadAll(resp.Body)
		if err == nil {
			return s, resp.StatusCode
		}
	}
	return []byte{}, resp.StatusCode
}
