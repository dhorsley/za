//+build !test

package main

import (
    "errors"
    "strconv"
    "path/filepath"
    "io"
    "io/ioutil"
    "mime"
    "net/http"
    "time"
    "os"
    str "strings"
)


const RefreshRate = time.Millisecond * 100

func humansize(i float64,prec int,unit string) (string) {
    if i>=1e9 { unit="Gi"+unit; i=float64(i/1e9)  }
    if i>=1e6 { unit="Mi"+unit; i=float64(i/1e6)  }
    if i>=1e3 { unit="Ki"+unit; i=float64(i/1e3) }
    return sf("%." + strconv.Itoa(prec) + "f %s",i,unit)
}

func is_file(s string) bool {
    f, err := os.Stat(s)
    if err == nil {
        return f.Mode().IsRegular()
    }
    return false
}

type WriteCounter struct {
    tot uint64
}

func (wc *WriteCounter) Write(p []byte) (int, error) {
    n:=len(p)
    wc.tot += uint64(n)
    wc.Show()
    return n, nil
}

func (wc WriteCounter) Show() {
    cursorX(0) ; clearToEOL() ; cursorX(0)
    pf("[download] [#1]%s[#-] complete.",humansize(float64(wc.tot),3,"B"))
}


func FileDownload(fp string) (fname string, ecode int) {

        // setup temp receiver
        tmpfile, err := ioutil.TempFile("", "za-dl-*")
        defer os.Remove(tmpfile.Name())

        // download
        r, err := http.Get(fp)
        if err != nil || (r.StatusCode <200 || r.StatusCode >299) {
            pf("[download] could not get file from %v\n",fp)
            return "",-2
        }
        defer r.Body.Close()

        fname=filepath.Base(r.Request.URL.String())
        contdisp := r.Header.Get("Content-Disposition")
        _, params, err := mime.ParseMediaType(contdisp)
        if err == nil {
            fname = params["filename"]
        }

        pf("\n")
        counter := &WriteCounter{}
        if _, err = io.Copy(tmpfile, io.TeeReader(r.Body, counter)); err!=nil {
            pf("\n[download] error reading from stream for %v\n",fp)
            return "",-3
        }
        pf("\n")

        tmpfile.Close()
        err = os.Rename(tmpfile.Name(), fname)
        if err != nil {
            pf("[download] could not rename temporary file.\n")
            return "",-4
        }

        return fname,0

}


func buildPackageLib() {

    // packages

    features["package"] = Feature{version: 1, category: "os"}
    categories["package"] = []string{"install", "uninstall", "service", "vcmp","is_installed"}

    slhelp["install"] = LibHelp{in: "packages_string", out: "int", action: "Installs the packages in [#i1]packages_string[#i0]. Returns 0 on success or non-zero error code. (-1 is an unknown OS)"}
    stdlib["install"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("install",args,1,"1","string"); !ok { return nil,err }
        done := install(args[0].(string))
        return done, err
    }

    slhelp["is_installed"] = LibHelp{in: "package_name", out: "bool", action: "Is package [#i1]package_name[#i0] installed?"}
    stdlib["is_installed"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("is_installed",args,1,"1","string"); !ok { return nil,err }
        if args[0].(string)=="" { return false,errors.New("Invalid package name") }
        return isinstalled(args[0].(string)),nil
    }

    slhelp["uninstall"] = LibHelp{in: "packages_string", out: "int", action: "Removes the packages in [#i1]packages_string[#i0]. Returns 0 on success or non-zero error code. (-1 is an unknown OS)"}
    stdlib["uninstall"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("uninstall",args,1,"1","string"); !ok { return nil,err }
        done := uninstall(args[0].(string))
        return done, err
    }

    slhelp["vcmp"] = LibHelp{
        in: "string_v1,string_v2",
        out: "int",
        action: "Returns -1, 0, or +1 depending on semantic version string [#i1]string_v1[#i0] being less than,\n"+
        "equal to, or greater than version string [#i1]string_v2[#i0]."}
    stdlib["vcmp"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("vcmp",args,1,"2","string","string"); !ok { return nil,err }
        return vcmp(args[0].(string), args[1].(string))
    }

    slhelp["service"] = LibHelp{in: "service_name,action", out: "success_flag", action: "Attempts to take the required [#i1]action[#i0] on service [#i1]service_name[#i0]. Returns true if successful."}
    stdlib["service"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("service",args,1,"2","string","string"); !ok { return nil,err }
        done, err := service(args[0].(string), args[1].(string))
        return done, err
    }
}


// semantic version comparison
func vcmp(vs1, vs2 string) (int, error) {
    v1, e := vconvert(vs1)
    if e != nil {
        return 0, errors.New(sf("'%s' has an invalid format for version comparison.", vs1))
    }
    v2, e := vconvert(vs2)
    if e != nil {
        return 0, errors.New(sf("'%s' has an invalid format for version comparison.", vs2))
    }
    if v1 < v2 {
        return -1, nil
    }
    if v1 > v2 {
        return 1, nil
    }
    return 0, nil
}


// convert a semantic version number into a floating point or comparisons.
func vconvert(v string) (float64, error) {
    var p1, p2 int
    if !str.ContainsAny(v, ".") {
        v = v + ".0"
    }
    a := str.Split(v, ".")
    f := a[0] + "."
    p1, _ = strconv.Atoi(a[1])
    if len(a) == 3 {
        p2, _ = strconv.Atoi(a[2])
    }
    f += sf("%06d", p1)
    if len(a) == 3 {
        f += sf("%06d", p2)
    }
    return strconv.ParseFloat(f, 64)
}

func isinstalled(pkg string) (bool) {

    v, _ := gvget("@release_id")

    err:=-1
    switch v.(string) {
    case "ubuntu", "debian","pop":
        err = Copper("dpkg -V "+pkg+" 2>/dev/null", true).code
    case "opensuse":
        err = Copper("rpm -q "+pkg+" 2>/dev/null", true).code
    case "alpine":
        err = Copper("apk info -e "+pkg+" 2>/dev/null", true).code
    case "fedora", "amzn", "centos", "rhel":
        err = Copper("rpm -q "+pkg+" 2>/dev/null", true).code
    default:
        return false
    }

    if err==0 { return true }
    return false

}

func uninstall(pkgs string) (state int) {

    var pm, upopts, inopts, checkcmd1, checkcmd2 string

    v, _ := gvget("@release_id")

    switch v.(string) {
    case "ubuntu", "debian", "pop":
        pm = "apt"
        upopts = "-y update"
        inopts = "-y remove"
        checkcmd1 = "dpkg 2>/dev/null -V "
        checkcmd2 = ``
    case "opensuse":
        pm = "zypper"
        upopts = "update -y"
        inopts = "remove -y"
        checkcmd1 = "rpm -q "
        checkcmd2 = ""
    case "alpine":
        pm = "apk"
        upopts = "update -q"
        inopts = "remove -q"
        checkcmd1 = "apk info -e "
        checkcmd2 = ""
    case "fedora", "amzn", "centos", "rhel":
        pm = "yum"
        upopts = "-y update"
        inopts = "-y remove"
        checkcmd1 = "rpm -q "
        checkcmd2 = ""
    default:
        return -1
    }

    updateCommand := sf("%s %s", pm, upopts)
    // pf("[upd] Executing: %s\n",updateCommand)

    if firstInstallRun {
        pf("Updating repository.\n")
        err := Copper(updateCommand, true).code
        if err != 0 {
            pf("Problem performing system update!\n")
            finish(true, ERR_PACKAGE)
            return err
        }
        firstInstallRun = false
    }

    err := Copper(checkcmd1+pkgs+checkcmd2, true).code
    if err == 0 { // installed
        // remove
        removeCommand := sf("%s %s %s", pm, inopts, pkgs)
        // pf("[rem] Executing: %s\n",removeCommand)
        pf("Removing: %v\n", pkgs)
        cop := Copper(removeCommand, true)
        if !cop.okay {
            pf("\nPotential problem removing packages [%s]\n",pkgs)
            pf(cop.out)
            finish(false,ERR_PACKAGE)
            return cop.code
        }
    } else {
        return -1
    }

    return 0
}

// install software with the default package manager.
//   if a path is provided, then treat as a local package instead.
func install(pkgs string) (state int) {

    // return state
    // 0  : all successfully installed
    // -1 : unknown os
    // >0 : error code

    potpack:=str.ToLower(pkgs)

    // remote file request
    if str.HasPrefix(potpack,"http:") || str.HasPrefix(potpack,"https:") {
        localname,errcode:=FileDownload(pkgs)
        defer os.Remove(localname)
        if errcode!=0 {
            pf("[#2]Error when downloading %s[#-]\n",pkgs)
            return errcode
        }
        pkgs=localname
    // } else {
        // pf("no internet prefix\n")
    }

    // local file install
    if is_file(pkgs) {

        ext:=filepath.Ext(pkgs)
        pbname:=filepath.Base(pkgs)
        pkgparts:=str.Split(pbname,"_")
        if len(pkgparts)>1 { pbname=pkgparts[0] }

        switch ext {
        case ".deb": // dpkg
            cmd := "dpkg -s "+pbname
            cop := Copper(cmd, true)
            if cop.code>0 || !str.Contains(cop.out,"Status: install ok installed") { // not installed
                pf("[#3]%s not currently installed.[#-]\n",pbname)
            } else {
                pf("[#3]%s already installed. Overwriting.[#-]\n",pbname)
            }
            cmd = "dpkg -i "+pkgs
            cop = Copper(cmd, true)
            if cop.code>0 {
                pf("[#2]Error during package install! Do you have privileges?[#-]\n")
                return -1
            }
            pf("[#4]%s installed.[#-]\n",pkgs)
            return 0
        case ".rpm": // rpm
            cmd := "rpm -qi "+pbname
            cop := Copper(cmd, true)
            if cop.code==0 { // installed
                pf("[#3]%s already installed. Overwriting.[#-]\n",pbname)
            } else {
                pf("[#3]%s install not detected.[#-]\n",pbname)
            }
            cmd = "rpm -U "+pkgs
            cop = Copper(cmd, true)
            if cop.code>0 {
                pf("[#2]Error during package install! Do you have privileges?[#-]\n")
                return -1
            }
            pf("[#4]%s installed.[#-]\n",pkgs)
            return 0
        case ".apk": // apk
            cmd:="apk add --allow-untrusted "+pbname
            cop:=Copper(cmd,true)
            if cop.code>0 {
                pf("[#2]Error during package install! Do you have privileges?[#-]\n")
                return -1
            }
            pf("[#4]%s installed.[#-]\n",pkgs)
            return 0
        case ".sh" : // execute script
            pf("[#2]Script execution not supported![#-]")
            return -1
            // not adding this yet, as there's a good chance an install script could require
            // interactivity, which may or may not work correctly. not checked yet.
            // this is probably just a matter of adding a Copper(pkgs, true) command.
            // have to check pkgs only contains one item too i suppose.
        default:
        }
    } else {
        pf("[#3]local file %s not found. trying repositories instead.[#-]\n",pkgs)
    }


    // if not a local or downloaded file, then use package manager

    // get manager name
    var pm, upopts, inopts, checkcmd1, checkcmd2 string

    inopts = ""

    v, _ := gvget("@release_id")
    switch v.(string) {
    case "ubuntu", "debian", "pop":
        pm = "apt"
        upopts = "-y update"
        inopts = "-y install"
        checkcmd1 = "dpkg 2>/dev/null -V "
        checkcmd2 = ``
    case "opensuse":
        pm = "zypper"
        upopts = "update -y"
        inopts = "install -y -l"
        checkcmd1 = "rpm -q "
        checkcmd2 = ""
    case "alpine":
        pm = "apk"
        upopts = "update -q"
        inopts = "add -q"
        checkcmd1 = "apk info -e "
        checkcmd2 = ""
    case "fedora", "amzn", "centos", "rhel":
        pm = "yum"
        upopts = "-y update"
        inopts = "-y install"
        checkcmd1 = "rpm -q "
        checkcmd2 = ""
    default:
        return -1
    }

    updateCommand := sf("%s %s", pm, upopts)

    if firstInstallRun {
        // do update
        pf("Updating repository.\n")
        err := Copper(updateCommand, true).code
        if err != 0 {
            pf("Problem performing system update!\n")
            finish(true, ERR_PACKAGE)
            return err
        }
        firstInstallRun = false
    }

    // @note: doing it this way is bad if there are co-dependencies that
    //   must be resolved at the same time as each other. maybe should change
    //   to just process the whole list in one go?

    plist := str.Split(pkgs, ",")
    for _, p := range plist {

        err := Copper(checkcmd1+p+checkcmd2, true).code
        if err == 1 { // not installed
            // install
            installCommand := sf("%s %s %s", pm, inopts, p)
            pf("Installing: %v\n", p)
            cop := Copper(installCommand, true)
            if !cop.okay {
                // pf("\nPotential problem installing packages [%s]\n",p)
                pf(cop.out)
                finish(false,ERR_PACKAGE)
                return cop.code
            }
        } else {
            // already there or invalid names. either way, do nothing...
            // pf("Packages in '%s' are already installed.\n", p)
        }

    }

    return 0
}


// take a service action. actions are permitted for upstart and systemd tools. 
func service(name string, action string) (bool, error) {

    v, _ := gvget("@release_id")
    rid := v.(string)
    v, _ = gvget("@release_version")
    rv := v.(string)

    sys := Copper("ps -o comm= -q 1", true)
    if !sys.okay {
        pf("Error: could not check process 1.\n")
        return false, errors.New("Could not check process 1.")
    }

    expected := ""

    switch rid {
    case "ubuntu", "debian", "pop":
        switch rid {
        case "ubuntu","pop": // default from v15.04
            if vc, _ := vcmp(rv, "15.4"); vc >= 0 {
                expected = "systemd"
            } else {
                expected = "upstart"
            }
        case "debian": // default from v8
            if vc, _ := vcmp(rv, "8"); vc >= 0 {
                expected = "systemd"
            } else {
                expected = "upstart"
            }
        }
    case "opensuse": // default from 12.2 (12 in suse enterprise)
        if vc, _ := vcmp(rv, "12.2"); vc >= 0 {
            expected = "systemd"
        } else {
            expected = "upstart"
        }
    case "fedora", "amzn", "centos", "rhel":
        switch rid {
        case "fedora":
            if vc, _ := vcmp(rv, "15"); vc == 1 {
                expected = "systemd"
            } else {
                expected = "upstart"
            }
        case "amzn": // amazon linux 1 (pre-sept 2017?) uses upstart
            if vc, _ := vcmp(rv, "2"); vc == 1 {
                expected = "systemd"
            } else {
                expected = "upstart"
            }
        case "centos":
            if vc, _ := vcmp(rv, "7"); vc >= 0 {
                expected = "systemd"
            } else {
                expected = "upstart"
            }
        case "rhel":
            if vc, _ := vcmp(rv, "7"); vc >= 0 {
                expected = "systemd"
            } else {
                expected = "upstart"
            }
        }
    case "alpine":
        pf("Service control is disabled for Alpine!\n")
        finish(false, ERR_UNSUPPORTED)
    default:
        pf("A number of systems are currently unsupported.\nThese include:- Arch, CoreOS, Gentoo, Knoppix, Mageia, Mint, Slackware and Solus.\n")
        finish(false, ERR_UNSUPPORTED)
    }

    if sys.out != expected {
        pf("Warning: your current init system does not match the expected init system for this OS!\nContinuing execution, however, you may encounter issues.\n")
    }

    unknown := false
    var cop struct{out string;err string;code int; okay bool}

    switch expected {
    case "systemd":
        switch action {
        case "stop":
            cop = Copper("systemctl stop "+name, true)
        case "start":
            cop = Copper("systemctl start "+name, true)
        case "restart":
            cop = Copper("systemctl try-restart "+name, true)
        case "reload":
            cop = Copper("systemctl try-reload-or-restart "+name, true)
        case "disable":
            cop = Copper("systemctl disable "+name, true)
        case "enable":
            cop = Copper("systemctl enable "+name, true)
        }
    case "upstart":
        switch action {
        case "stop":
            cop = Copper("service "+name+" stop", true)
        case "start":
            cop = Copper("service "+name+" start", true)
        case "restart":
            cop = Copper("service "+name+" restart", true)
        case "reload":
            cop = Copper("service "+name+" reload", true)
        case "disable":
            cop = Copper("service "+name+" disable", true)
        case "enable":
            cop = Copper("service "+name+" enable", true)
        }
    default: // system V scripts? or something else. Either way, not supporting them!
        unknown = true
    }

    if unknown {
        return false, errors.New("Error: We only support upstart and systemd.\n")
    }

    if !cop.okay {
        pf("Error: the required service action '%s' on '%s' could not be completed successfully. Please investigate.\n", action, name)
        return false, errors.New("Error: could not complete the required action.")
    }

    pf("%s\n", cop.out)
    return true, nil

}

