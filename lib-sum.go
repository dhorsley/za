//go:build !test

package main

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"strconv"
)

type s3sum_reply struct {
	sum string
	err int
}

const (
	S3_ERR_NONE int = iota
	S3_WARN_SINGLE
	S3_ERR_FILE
	S3_ERR_SUM
)

func s3sum(filename string, blocksize int64) (s3sum_reply, error) {
	f, err := os.Open(filename)
	if err != nil {
		return s3sum_reply{"", S3_ERR_FILE}, err
	}
	defer f.Close()
	return s3calc(f, blocksize)
}

func s3calc(f io.ReadSeeker, blocksize int64) (s3sum_reply, error) {

	partType := S3_PT_NONE

	if blocksize == -1 {
		partType = S3_PT_SINGLE
		blocksize = 0
	}

	if blocksize == 0 {
		blocksize = 8192 * 1024
	}

	sz, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return s3sum_reply{"", S3_ERR_FILE}, err
	}

	if sz > blocksize {
		switch partType {
		case S3_PT_NONE:
			partType = S3_PT_MULTI
		}
	} else {
		if partType == S3_PT_NONE {
			partType = S3_PT_SINGLE
		}
	}

	if partType == S3_PT_SINGLE {
		sum, err := md5sum(f, 0, sz)
		if err != nil {
			return s3sum_reply{"", S3_ERR_SUM}, err
		}
		blockwarn := S3_ERR_NONE
		if sz > blocksize {
			blockwarn = S3_WARN_SINGLE
		}
		return s3sum_reply{hex.EncodeToString(sum), blockwarn}, nil
	}

	var runsum []byte
	var parts int

	for i := int64(0); i < sz; i += blocksize {
		length := blocksize
		if i+blocksize > sz {
			length = sz - i
		}
		sum, err := md5sum(f, i, length)
		if err != nil {
			return s3sum_reply{"", S3_ERR_SUM}, err
		}
		runsum = append(runsum, sum...)
		parts += 1
	}

	var totsum []byte

	if parts == 1 {
		totsum = runsum
	} else {
		h := md5.New()
		_, err := h.Write(runsum)
		if err != nil {
			return s3sum_reply{"", S3_ERR_SUM}, err
		}
		totsum = h.Sum(nil)
	}

	hsum := hex.EncodeToString(totsum)
	if parts > 1 {
		hsum += "-" + strconv.Itoa(parts)
	}

	return s3sum_reply{hsum, S3_ERR_NONE}, nil
}

func md5sum(r io.ReadSeeker, start, length int64) ([]byte, error) {
	r.Seek(start, io.SeekStart)
	h := md5.New()
	if _, err := io.CopyN(h, r, length); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

func buildSumLib() {

	features["sum"] = Feature{version: 1, category: "integrity"}
	categories["sum"] = []string{
		"md5sum", "sha1sum", "sha224sum", "sha256sum", "s3sum",
	}

	slhelp["s3sum"] = LibHelp{in: "filename[,blocksize]", out: "struct",
		action: "Returns a struct bearing a checksum (.sum) and an error code (.err) for comparison to an S3 ETag field.\n" +
			"Blocksize specifies the size in bytes of multi-part upload chunks.\n" +
			"When blocksize is 0 then auto-select blocksize and upload checksum type.\n" +
			"When blocksize is -1 then treat as a single-part upload.\n" +
			"Error codes: 0 okay, 1 single-part warning, 2 file error, 3 checksum error"}
	stdlib["s3sum"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("s3sum", args, 2,
			"2", "string", "number",
			"1", "string"); !ok {
			return s3sum_reply{"", S3_ERR_NONE}, err
		}

		var blksize int64
		if len(args) == 2 {
			blksize, _ = GetAsInt64(args[1])
		}

		return s3sum(args[0].(string), blksize)
	}

	slhelp["md5sum"] = LibHelp{in: "string", out: "string", action: "Returns the MD5 checksum of the input string."}
	stdlib["md5sum"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("md5sum", args, 1, "1", "string"); !ok {
			return nil, err
		}
		return sf("%x", md5.Sum([]byte(args[0].(string)))), nil
	}

	slhelp["sha1sum"] = LibHelp{in: "string", out: "string", action: "Returns the SHA1 checksum of the input string."}
	stdlib["sha1sum"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("sha1sum", args, 1, "1", "string"); !ok {
			return nil, err
		}
		return sf("%x", sha1.Sum([]byte(args[0].(string)))), nil
	}

	slhelp["sha224sum"] = LibHelp{in: "string", out: "string", action: "Returns the SHA224 checksum of the input string."}
	stdlib["sha224sum"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("sha224sum", args, 1, "1", "string"); !ok {
			return nil, err
		}
		return sf("%x", sha256.Sum224([]byte(args[0].(string)))), nil
	}

	slhelp["sha256sum"] = LibHelp{in: "string", out: "string", action: "Returns the SHA256 checksum of the input string."}
	stdlib["sha256sum"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("sha256sum", args, 1, "1", "string"); !ok {
			return nil, err
		}
		return sf("%x", sha256.Sum256([]byte(args[0].(string)))), nil
	}

}
