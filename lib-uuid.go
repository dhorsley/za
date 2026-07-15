//go:build !test

package main

import (
	"crypto/rand"
	"fmt"
	"strings"
)

func buildUuidLib() {

	features["uuid"] = Feature{version: 1, category: "conversion"}
	categories["uuid"] = []string{"uuid_generate", "uuid_parse", "uuid_validate"}

	slhelp["uuid_generate"] = LibHelp{in: "", out: "string", action: "Generates a random UUID (version 4) and returns it as a lowercase string."}
	stdlib["uuid_generate"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("uuid_generate", args, 0); !ok {
			return nil, err
		}

		b := make([]byte, 16)
		_, err = rand.Read(b)
		if err != nil {
			return "", fmt.Errorf("uuid_generate: could not read random data: %v", err)
		}

		// Version 4: set version bits (0100) and variant bits (10)
		b[6] = (b[6] & 0x0f) | 0x40
		b[8] = (b[8] & 0x3f) | 0x80

		return fmt.Sprintf("%x-%x-%x-%x-%x",
			b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
	}

	slhelp["uuid_parse"] = LibHelp{in: "string", out: "string", action: "Normalises a UUID string to lowercase with dashes. Returns an empty string if the input is not a valid UUID."}
	stdlib["uuid_parse"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("uuid_parse", args, 1, "1", "string"); !ok {
			return nil, err
		}

		s := args[0].(string)
		s = strings.TrimSpace(s)
		s = strings.ToLower(s)

		// Strip existing dashes
		s = strings.ReplaceAll(s, "-", "")

		if len(s) != 32 {
			return "", nil
		}

		// Check hex only
		for _, r := range s {
			if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
				return "", nil
			}
		}

		return fmt.Sprintf("%s-%s-%s-%s-%s",
			s[0:8], s[8:12], s[12:16], s[16:20], s[20:32]), nil
	}

	slhelp["uuid_validate"] = LibHelp{in: "string", out: "bool", action: "Returns true if the input is a valid UUID string (with dashes, lowercase)."}
	stdlib["uuid_validate"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("uuid_validate", args, 1, "1", "string"); !ok {
			return nil, err
		}

		s := args[0].(string)
		s = strings.TrimSpace(s)

		if len(s) != 36 {
			return false, nil
		}

		// Check positions of dashes
		if s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
			return false, nil
		}

		// Check all other chars are hex
		for i, r := range s {
			if i == 8 || i == 13 || i == 18 || i == 23 {
				continue
			}
			if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
				return false, nil
			}
		}

		return true, nil
	}
}
