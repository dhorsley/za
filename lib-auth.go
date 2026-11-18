//go:build !test

package main

import (
    "crypto/pbkdf2"
    "crypto/rand"
    "crypto/sha256"
    "crypto/subtle"
    "encoding/base64"
    "errors"
    "fmt"
    "strconv"
    "strings"
)

const (
    authDefaultIterations = 600000
    authSaltBytes         = 16
    authKeyBytes          = 32
    authTokenMaxBytes     = 4096
)

func authRandomToken(size int) (string, error) {
    if size <= 0 || size > authTokenMaxBytes {
        return "", fmt.Errorf("random_token: size must be between 1 and %d", authTokenMaxBytes)
    }
    b := make([]byte, size)
    if _, err := rand.Read(b); err != nil {
        return "", err
    }
    return base64.RawURLEncoding.EncodeToString(b), nil
}

func authPasswordHash(password string, iterations int) (string, error) {
    if password == "" {
        return "", errors.New("password_hash: password must not be empty")
    }
    if iterations == 0 {
        iterations = authDefaultIterations
    }
    if iterations < 1 || iterations > 10000000 {
        return "", errors.New("password_hash: iterations must be between 1 and 10000000")
    }

    salt := make([]byte, authSaltBytes)
    if _, err := rand.Read(salt); err != nil {
        return "", err
    }

    key, err := pbkdf2.Key(sha256.New, password, salt, iterations, authKeyBytes)
    if err != nil {
        return "", err
    }

    return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s",
        iterations,
        base64.RawURLEncoding.EncodeToString(salt),
        base64.RawURLEncoding.EncodeToString(key),
    ), nil
}

func authPasswordVerify(password, encoded string) (bool, error) {
    parts := strings.Split(encoded, "$")
    if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
        return false, errors.New("password_verify: unsupported hash format")
    }

    iterations, err := strconv.Atoi(parts[1])
    if err != nil || iterations < 1 || iterations > 10000000 {
        return false, errors.New("password_verify: invalid iteration count")
    }

    salt, err := base64.RawURLEncoding.DecodeString(parts[2])
    if err != nil {
        return false, errors.New("password_verify: invalid salt encoding")
    }

    want, err := base64.RawURLEncoding.DecodeString(parts[3])
    if err != nil {
        return false, errors.New("password_verify: invalid hash encoding")
    }

    got, err := pbkdf2.Key(sha256.New, password, salt, iterations, len(want))
    if err != nil {
        return false, err
    }

    return subtle.ConstantTimeCompare(got, want) == 1, nil
}

func authSecureCompare(a, b string) bool {
    ah := sha256.Sum256([]byte(a))
    bh := sha256.Sum256([]byte(b))
    return subtle.ConstantTimeCompare(ah[:], bh[:]) == 1
}

func buildAuthLib() {
    features["auth"] = Feature{version: 1, category: "security"}
    categories["auth"] = []string{"random_token", "password_hash", "password_verify", "secure_compare"}

    slhelp["random_token"] = LibHelp{in: "size", out: "string", action: "Returns a cryptographically secure URL-safe random token."}
    stdlib["random_token"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("random_token", args, 1, "1", "int"); !ok {
            return nil, err
        }
        return authRandomToken(args[0].(int))
    }

    slhelp["password_hash"] = LibHelp{in: "password[,iterations]", out: "string", action: "Returns a PBKDF2-HMAC-SHA256 password hash."}
    stdlib["password_hash"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("password_hash", args, 2,
            "2", "string", "int",
            "1", "string"); !ok {
            return nil, err
        }
        iterations := 0
        if len(args) == 2 {
            iterations = args[1].(int)
        }
        return authPasswordHash(args[0].(string), iterations)
    }

    slhelp["password_verify"] = LibHelp{in: "password,hash", out: "bool", action: "Verifies a password against a PBKDF2-HMAC-SHA256 hash."}
    stdlib["password_verify"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("password_verify", args, 1, "2", "string", "string"); !ok {
            return nil, err
        }
        return authPasswordVerify(args[0].(string), args[1].(string))
    }

    slhelp["secure_compare"] = LibHelp{in: "a,b", out: "bool", action: "Compares two strings in constant time."}
    stdlib["secure_compare"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("secure_compare", args, 1, "2", "string", "string"); !ok {
            return nil, err
        }
        return authSecureCompare(args[0].(string), args[1].(string)), nil
    }
}
