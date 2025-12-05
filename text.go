package main

import (
    "regexp"
    str "strings"
    "unicode/utf8"
)

// line grep from string
func lgrep(s string, reg string) string {
    list := str.Split(s, "\n")
    var ns str.Builder
    ns.Grow(len(s) / 4)
    for _, v := range list {
        if m, _ := regexp.MatchString(reg, v); m {
            ns.WriteString(v + "\n")
        }
    }
    repl := ns.String()
    if len(repl) > 0 {
        if repl[len(repl)-1] == '\n' {
            repl = repl[:len(repl)-1]
        }
    }
    return repl
}

func lcut(s string, pos int, sep string) string {
    fstr := str.TrimSuffix(s, "\n")
    ta := str.FieldsFunc(fstr, func(c rune) bool { return str.ContainsRune(sep, c) })
    if pos > 0 && pos <= len(ta) {
        return ta[pos-1]
    }
    return ""
}

func lastCharSize(s string) int {
    _, size := utf8.DecodeLastRuneInString(s)
    return size
}

func pad(s string, just int, w int, fill string) string {

    if s == "" {
        return ""
    }

    ls := utf8.RuneCountInString(StripCC(s))
    if ls == 0 {
        return ""
    }

    switch just {

    case -1:
        // left
        return s + rep(fill, w-ls)

    case 1:
        // right
        if ls > w {
            s = string([]rune(s)[:w])
        }
        return rep(fill, int(w-utf8.RuneCountInString(s))) + s

    case 0:
        // center
        p := int(w/2) - int(ls/2)
        extra := 1
        if (w % 2) == 0 {
            extra = 0
        }
        r_remove := ls % 2
        if extra == 1 && r_remove == 1 {
            extra = 0
            r_remove = 0
        }
        return rep(fill, p+extra) + s + rep(fill, p-r_remove)

    }
    return ""
}

func sanitise(s string) string {
    var ns str.Builder
    ns.Grow(len(s))
    pass := true
    nest := 0
    for p := 0; p < len(s); p += 1 {
        if s[p] == '{' {
            nest += 1
            pass = false
        }
        if s[p] == '}' {
            if nest != 0 {
                nest -= 1
            }
            if nest == 0 && pass == false {
                pass = true
                continue
            }
        }
        if pass {
            ns.WriteByte(s[p])
        }
    }
    return ns.String()
}

func stripOuter(s string, c byte) string {
    if len(s) > 0 && s[0] == c {
        s = s[1:]
    }
    if len(s) > 0 && s[len(s)-1] == c {
        s = s[:len(s)-1]
    }
    return s
}

func stripSingleQuotes(s string) string {
    return stripOuter(s, '\'')
}

func stripBacktickQuotes(s string) string {
    return stripOuter(s, '`')
}

func stripDoubleQuotes(s string) string {
    return stripOuter(s, '"')
}

func stripOuterQuotes(s string, maxdepth int) string {

    for ; maxdepth > 0; maxdepth -= 1 {
        s = stripSingleQuotes(s)
        s = stripDoubleQuotes(s)
        if !(hasOuterSingleQuotes(s) || hasOuterDoubleQuotes(s)) {
            break
        }
    }
    return s
}

func hasOuterBraces(s string) bool {
    if len(s) > 0 && s[0] == '(' && s[len(s)-1] == ')' {
        return true
    }
    return false
}

func hasOuter(s string, c byte) bool {
    if len(s) > 0 && s[0] == c && s[len(s)-1] == c {
        return true
    }
    return false
}

func hasOuterBacktickQuotes(s string) bool {
    return hasOuter(s, '`')
}

func hasOuterSingleQuotes(s string) bool {
    return hasOuter(s, '\'')
}

func hasOuterDoubleQuotes(s string) bool {
    return hasOuter(s, '"')
}

// log_sanitise redacts sensitive information from strings for logging purposes
func log_sanitise(input string) string {
    result := input

    // CLI-focused patterns for common sensitive data exposure
    patterns := []struct {
        pattern *regexp.Regexp
        replace string
    }{
        // CLI command patterns - full redaction of all arguments
        {regexp.MustCompile(`^(?:[A-Z_][A-Z0-9_]*=\S+\s+)*(curl|wget|scp|sftp|rsync|ssh|telnet|ncftp|git|svn|hg|cvs|npm|pip|maven|gradle|cargo|yarn|composer|docker|docker-compose|podman|kubectl|helm|rkt|istioctl|openshift|aws|gcloud|az|doctl|linode-cli|heroku|turbolift|scaleway|rclone|upctl|ibmcloud|oci|mysql|psql|sqlite3|mongo|redis-cli|sqlplus|isql|cockroach|terraform|ansible|puppet|chef|salt|consul|vault|packer|vagrant|openssl|gpg|ssh-keygen|keytool|certbot|mkcert|pass|age|su|passwd|useradd|usermod|keycloak|okta|auth0|gluu|freeipa|sssd|ansible-vault|chef-vault|nmap|masscan|nikto|burpsuite|sqlmap|openvas|rdesktop|x2go|vncviewer|teamviewer|anydesk|nomachine|rclone|aws-cli|gdrive|dropbox|megasync|s3cmd|sed|awk|grep|zip|7z|rar|strace)\s+.*`), "$1 [REDACTED]"},
        {regexp.MustCompile(`^(?:[A-Z_][A-Z0-9_]*=\S+\s+)*sudo\s+([a-zA-Z0-9_/.-]+).*`), "sudo $1 [REDACTED]"},

        // Basic authentication patterns
        {regexp.MustCompile(`(?i)(password|passwd|pwd|secret|token|key|api_key|apikey|access_key|secret_key|private_key|auth|authorization|bearer)\s*=\s*\S+`), "${1}=[REDACTED]"},
        {regexp.MustCompile(`(?i)(-p|--password)\s*\S+`), "${1} [REDACTED]"},
        {regexp.MustCompile(`(\w+):(\w+)@([\w\.-]+:\d+/[\w/]+)`), "${1}:[REDACTED]@${3}"},

        // Cloud provider keys
        {regexp.MustCompile(`AKIA[0-9A-Z]{16}`), "[AWS_ACCESS_KEY_REDACTED]"},
        {regexp.MustCompile(`sk_[a-zA-Z0-9]{20,}`), "[STRIPE_KEY_REDACTED]"},
        {regexp.MustCompile(`ghp_[a-zA-Z0-9]{36}`), "[GITHUB_TOKEN_REDACTED]"},
        {regexp.MustCompile(`AIza[A-Za-z0-9_-]{35}`), "[GOOGLE_API_KEY_REDACTED]"},
        {regexp.MustCompile(`GOOG-[0-9A-Z]{26}`), "[GOOGLE_CLOUD_KEY_REDACTED]"},
        {regexp.MustCompile(`ya29\.[0-9A-Z]{24}`), "[GCP_SERVICE_ACCOUNT_REDACTED]"},
        {regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`), "[AZURE_SUBSCRIPTION_ID_REDACTED]"},
        {regexp.MustCompile(`dop_v1_[a-zA-Z0-9]{32}`), "[DIGITALOCEAN_TOKEN_REDACTED]"},
        {regexp.MustCompile(`LINODE_[0-9A-Z]{64}`), "[LINODE_API_KEY_REDACTED]"},

        // SSH and cryptographic keys
        {regexp.MustCompile(`-----BEGIN [A-Z ]+PRIVATE KEY-----`), "[SSH_PRIVATE_KEY_REDACTED]"},
        {regexp.MustCompile(`-----BEGIN CERTIFICATE-----`), "[CERTIFICATE_REDACTED]"},
        {regexp.MustCompile(`SHA256:[a-fA-F0-9]{64}`), "[SSH_FINGERPRINT_REDACTED]"},
        {regexp.MustCompile(`MD5:[a-fA-F0-9]{32}`), "[SSH_FINGERPRINT_REDACTED]"},

        // Database connection strings
        {regexp.MustCompile(`mongodb://(\w+):(\w+)@`), "mongodb://${1}:[REDACTED]@"},
        {regexp.MustCompile(`redis://(\w+):(\w+)@`), "redis://${1}:[REDACTED]@"},
        {regexp.MustCompile(`postgresql://(\w+):(\w+)@`), "postgresql://${1}:[REDACTED]@"},

        // API tokens and service keys
        {regexp.MustCompile(`xox[bap]-[0-9a-f-]{40}`), "[SLACK_TOKEN_REDACTED]"},
        {regexp.MustCompile(`(MTE|OTA)[0-9A-Za-z_-]{83}`), "[DISCORD_TOKEN_REDACTED]"},
        {regexp.MustCompile(`bot[_-]?token[:\s=]\s*[A-Za-z0-9_-]{10,}`), "[TELEGRAM_BOT_TOKEN_REDACTED]"},
        {regexp.MustCompile(`AC[a-z0-9]{32}`), "[TWILIO_ACCOUNT_SID_REDACTED]"},
        {regexp.MustCompile(`SK[a-z0-9]{32}`), "[TWILIO_AUTH_TOKEN_REDACTED]"},

        // Hash patterns (various lengths)
        {regexp.MustCompile(`[a-fA-F0-9]{40}`), "[HASH_REDACTED]"},
        {regexp.MustCompile(`[a-fA-F0-9]{64}`), "[HASH_REDACTED]"},
        {regexp.MustCompile(`[a-fA-F0-9]{32}`), "[HASH_REDACTED]"},
        {regexp.MustCompile(`[a-fA-F0-9]{16}`), "[HASH_REDACTED]"},
    }

    // Apply patterns
    for _, p := range patterns {
        result = p.pattern.ReplaceAllString(result, p.replace)
    }

    return result
}
