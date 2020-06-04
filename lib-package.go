//+build !test

package main

import (
	"errors"
	"strconv"
	str "strings"
)

func buildPackageLib() {

	// packages

	features["package"] = Feature{version: 1, category: "os"}
	categories["package"] = []string{"install", "service", "vcmp"}

	slhelp["install"] = LibHelp{in: "packages_string", out: "", action: "Installs the packages in [#i1]packages_string[#i0]."}
	stdlib["install"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
		if len(args) != 1 {
			return false, errors.New("invalid argument count for install()")
		}
		switch args[0].(type) {
		case string:
			done := install(args[0].(string))
			return done, err
		}
		return false, errors.New("invalid argument type for install()")
	}

	slhelp["vcmp"] = LibHelp{in: "v1,v2", out: "int", action: "Returns -1, 0, or +1 depending on semantic version string [#i1]v1[#i0] being less than, equal to, or greater than version string [#i1]v2[#i0]."}
	stdlib["vcmp"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
		if len(args) != 2 {
			return 0, errors.New("Bad args (count) in vcmp()")
		}
        if sf("%T",args[0])!="string" || sf("%T",args[1])!="string" {
			return 0, errors.New("Bad args (type) in vcmp()")
        }
		return vcmp(args[0].(string), args[1].(string))
	}

	slhelp["service"] = LibHelp{in: "service_name,action", out: "success_flag", action: "Attempts to take the required [#i1]action[#i0] on service [#i1]service_name[#i0]. Returns true if successful."}
	stdlib["service"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

		if len(args) != 2 {
			return false,errors.New("Bad args (count) in service().\n")
		}

        if sf("%T",args[0])!="string" || sf("%T",args[1])!="string" {
			return 0, errors.New("Bad args (type) in service()")
        }

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


// install software with the default package manager.
func install(pkgs string) (state int) {

	// return state
	// 0  : all successfully installed
	// -1 : unknown os
	// >0 : error code

	// get manager name
	var pm, upopts, inopts, checkcmd1, checkcmd2 string
	var withsudo bool = true
	inopts = ""

	v, _ := vget(0, "@release_id")
	switch v.(string) {

	case "ubuntu", "debian":
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
		withsudo = false
	case "fedora", "amzn", "centos", "rhel":
		pm = "yum"
		upopts = "-y update"
		inopts = "-y install"
		checkcmd1 = "rpm -q "
		checkcmd2 = ""
	default:
		return -1

	}

	updateCommand := sf("sudo %s %s", pm, upopts)
	if !withsudo {
		updateCommand = sf("%s %s", pm, upopts)
	}

	debug(6, "UpdateCommand : [%v]\n", updateCommand)

	// check firstInstallRun

	if firstInstallRun {
		// do update
		pf("Updating repository.\n")
		out, err := Copper(updateCommand, true)
		if err != 0 {
			pf("Problem performing system update!\n")
			pf("->\n")
			pf(out + "\n\n")
			finish(true, ERR_PACKAGE)
		}
		firstInstallRun = false
		pf(out + "\n\n")
	}

	plist := str.Split(pkgs, ",")
	for _, p := range plist {
		// is installed?
		// yes, skip, no install:

		_, err := Copper(checkcmd1+p+checkcmd2, true)
		if err == 1 { // not installed
			// install
			installCommand := sf("sudo %s %s %s", pm, inopts, p)
			if !withsudo {
				installCommand = sf("%s %s %s", pm, inopts, p)
			}
			pf("Installing: %v\n", p)
			out, err := Copper(installCommand, true)
			if err != 0 {
				// pf("\nPotential problem installing packages [%s]\n",p)
				pf(out)
				// finish(false,ERR_PACKAGE)
				return err
			}
		} else {
			// already there
			pf("Packages in '%s' are already installed.\n", p)
		}
	}

	return 0
}


// take a service action. actions are permitted for upstart and systemd tools. 
func service(name string, action string) (bool, error) {

	v, _ := vget(0, "@release_id")
	rid := v.(string)
	v, _ = vget(0, "@release_version")
	rv := v.(string)

	sys, err := Copper("ps -o comm= -q 1", true)
	if err != 0 {
		pf("Error: could not check process 1.\n")
		return false, errors.New("Could not check process 1.")
	}

	expected := ""

	switch rid {
	case "ubuntu", "debian":
		switch rid {
		case "ubuntu": // default from v15.04
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

	if sys != expected {
		pf("Warning: your current init system does not match the expected init system for this OS!\nContinuing execution, however, you may encounter issues.\n")
	}

	unknown := false
	var out string
	switch expected {
	case "systemd":
		switch action {
		case "stop":
			out, err = Copper("sudo systemctl stop "+name, true)
		case "start":
			out, err = Copper("sudo systemctl start "+name, true)
		case "restart":
			out, err = Copper("sudo systemctl try-restart "+name, true)
		case "reload":
			out, err = Copper("sudo systemctl try-reload-or-restart "+name, true)
		case "disable":
			out, err = Copper("sudo systemctl disable "+name, true)
		case "enable":
			out, err = Copper("sudo systemctl enable "+name, true)
		}
	case "upstart":
		switch action {
		case "stop":
			out, err = Copper("sudo service "+name+" stop", true)
		case "start":
			out, err = Copper("sudo service "+name+" start", true)
		case "restart":
			out, err = Copper("sudo service "+name+" restart", true)
		case "reload":
			out, err = Copper("sudo service "+name+" reload", true)
		case "disable":
			out, err = Copper("sudo service "+name+" disable", true)
		case "enable":
			out, err = Copper("sudo service "+name+" enable", true)
		}
	default: // system V scripts? or something else. Either way, not supporting them!
		unknown = true
	}

	if unknown {
		return false, errors.New("Error: We only support upstart and systemd.\n")
	}

	if err != 0 {
		pf("Error: the required service action '%s' on '%s' could not be completed successfully. Please investigate.\n", action, name)
		return false, errors.New("Error: could not complete the required action.")
	}

	pf("%s\n", out)
	return true, nil

}

