//go:build !test

package main

import (
    "crypto/md5"
    "crypto/sha1"
    "crypto/sha256"
    "encoding/base64"
    "encoding/hex"
    "hash/crc32"
    "io"
    "os"
    "strconv"
)

type s3sum_reply struct {
    sum     string
    err     int
    headers map[string]string
}

const (
    S3_ERR_NONE int = iota
    S3_WARN_SINGLE
    S3_ERR_FILE
    S3_ERR_SUM
)

func s3sum(filename string, blocksize int64, legacyOnly bool) (s3sum_reply, error) {
    f, err := os.Open(filename)
    if err != nil {
        return s3sum_reply{"", S3_ERR_FILE, nil}, err
    }
    defer f.Close()
    return s3calc(f, blocksize, legacyOnly)
}

func s3calc(f io.ReadSeeker, blocksize int64, legacyOnly bool) (s3sum_reply, error) {

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
        return s3sum_reply{"", S3_ERR_FILE, nil}, err
    }

    var headers map[string]string

    if !legacyOnly {
        headers, err = computeModernChecksums(f, sz)
        if err != nil {
            return s3sum_reply{"", S3_ERR_SUM, nil}, err
        }
    }

    _, err = f.Seek(0, io.SeekStart)
    if err != nil {
        return s3sum_reply{"", S3_ERR_FILE, headers}, err
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
            return s3sum_reply{"", S3_ERR_SUM, headers}, err
        }
        blockwarn := S3_ERR_NONE
        if sz > blocksize {
            blockwarn = S3_WARN_SINGLE
        }
        return s3sum_reply{hex.EncodeToString(sum), blockwarn, headers}, nil
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
            return s3sum_reply{"", S3_ERR_SUM, headers}, err
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
            return s3sum_reply{"", S3_ERR_SUM, headers}, err
        }
        totsum = h.Sum(nil)
    }

    hsum := hex.EncodeToString(totsum)
    if parts > 1 {
        hsum += "-" + strconv.Itoa(parts)
    }

    return s3sum_reply{hsum, S3_ERR_NONE, headers}, nil
}

func computeModernChecksums(f io.ReadSeeker, sz int64) (map[string]string, error) {
    _, err := f.Seek(0, io.SeekStart)
    if err != nil {
        return nil, err
    }

    hmd5 := md5.New()
    hsha256 := sha256.New()
    hsha1 := sha1.New()
    hcrc32 := crc32.NewIEEE()
    hcrc32c := crc32.New(crc32.MakeTable(crc32.Castagnoli))

    mw := io.MultiWriter(hmd5, hsha256, hsha1, hcrc32, hcrc32c)
    _, err = io.CopyN(mw, f, sz)
    if err != nil {
        return nil, err
    }

    headers := make(map[string]string)
    headers["md5"] = hex.EncodeToString(hmd5.Sum(nil))
    headers["md5_base64"] = base64.StdEncoding.EncodeToString(hmd5.Sum(nil))
    headers["sha256"] = hex.EncodeToString(hsha256.Sum(nil))
    headers["sha256_base64"] = base64.StdEncoding.EncodeToString(hsha256.Sum(nil))
    headers["sha1"] = hex.EncodeToString(hsha1.Sum(nil))
    headers["sha1_base64"] = base64.StdEncoding.EncodeToString(hsha1.Sum(nil))
    headers["crc32"] = hex.EncodeToString(hcrc32.Sum(nil))
    headers["crc32_base64"] = base64.StdEncoding.EncodeToString(hcrc32.Sum(nil))
    headers["crc32c"] = hex.EncodeToString(hcrc32c.Sum(nil))
    headers["crc32c_base64"] = base64.StdEncoding.EncodeToString(hcrc32c.Sum(nil))

    return headers, nil
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

    slhelp["s3sum"] = LibHelp{in: "filename[,blocksize[,legacyOnly]]", out: "struct",
        action: "Returns a struct bearing a checksum (.sum), an error code (.err), and a headers map (.headers) for comparison to an S3 ETag field.\n" +
            "[#SOL]Blocksize specifies the size in bytes of multi-part upload chunks.\n" +
            "[#SOL]When blocksize is 0 then auto-select blocksize and upload checksum type.\n" +
            "[#SOL]When blocksize is -1 then treat as a single-part upload.\n" +
            "[#SOL]When legacyOnly is true, skip modern checksums and only compute legacy ETag.\n" +
            "[#SOL]Error codes: 0 okay, 1 single-part warning, 2 file error, 3 checksum error\n" +
            "[#SOL]The .headers map contains: md5, md5_base64, sha256, sha256_base64, sha1, sha1_base64, crc32, crc32_base64, crc32c, crc32c_base64\n" +
            "[#SOL]To get remote checksums: checksums=${aws s3api head-object --bucket {bucket} --key {key} --checksum-mode ENABLED}\n" +
            "[#SOL]To upload with checksums: aws s3api put-object --bucket {bucket} --key {key} --body {file} --checksum-algorithm SHA256"}
    stdlib["s3sum"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("s3sum", args, 3,
            "3", "string", "number", "bool",
            "2", "string", "number",
            "1", "string"); !ok {
            return s3sum_reply{"", S3_ERR_NONE, nil}, err
        }

        var blksize int64
        if len(args) >= 2 {
            blksize, _ = GetAsInt64(args[1])
        }

        var legacyOnly bool
        if len(args) == 3 {
            legacyOnly, _ = args[2].(bool)
        }

        return s3sum(args[0].(string), blksize, legacyOnly)
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
