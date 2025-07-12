//go:build !test
// +build !test

package main

import (
    "bytes"
    "encoding/base64"
    "errors"
    "fmt"
    "mime/multipart"
    "net/smtp"
    "net/textproto"
    "os"
    "strings"
)

func buildSmtpLib() {

    features["smtp"] = Feature{version: 1, category: "network"}
    categories["smtp"] = []string{"smtp_send", "smtp_send_with_auth", "smtp_send_with_attachments", "email_parse_headers", "email_get_body", "email_get_attachments"}

    slhelp["smtp_send"] = LibHelp{in: "server, from, to, subject, body", out: "bool", action: "Send email via SMTP without authentication."}
    stdlib["smtp_send"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("smtp_send", args, 1, "5", "string", "string", "[]string", "string", "string"); !ok {
            return nil, err
        }

        server := args[0].(string)
        from := args[1].(string)
        to := args[2].([]string)
        subject := args[3].(string)
        body := args[4].(string)

        // Build email message
        message := buildEmailMessage(from, to, subject, body)

        // Send email
        err = smtp.SendMail(server, nil, from, to, message)
        if err != nil {
            return false, fmt.Errorf("smtp_send error: %v", err)
        }

        return true, nil
    }

    slhelp["smtp_send_with_auth"] = LibHelp{in: "server, username, password, from, to, subject, body", out: "bool", action: "Send email via SMTP with authentication."}
    stdlib["smtp_send_with_auth"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("smtp_send_with_auth", args, 1, "7", "string", "string", "string", "string", "[]string", "string", "string"); !ok {
            return nil, err
        }

        server := args[0].(string)
        username := args[1].(string)
        password := args[2].(string)
        from := args[3].(string)
        to := args[4].([]string)
        subject := args[5].(string)
        body := args[6].(string)

        // Build email message
        message := buildEmailMessage(from, to, subject, body)

        // Create auth
        auth := smtp.PlainAuth("", username, password, strings.Split(server, ":")[0])

        // Send email
        err = smtp.SendMail(server, auth, from, to, message)
        if err != nil {
            return false, fmt.Errorf("smtp_send_with_auth error: %v", err)
        }

        return true, nil
    }

    slhelp["smtp_send_with_attachments"] = LibHelp{in: "server, from, to, subject, body, attachments", out: "bool", action: "Send email via SMTP with file attachments."}
    stdlib["smtp_send_with_attachments"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("smtp_send_with_attachments", args, 1, "6", "string", "string", "[]string", "string", "string", "[]string"); !ok {
            return nil, err
        }

        server := args[0].(string)
        from := args[1].(string)
        to := args[2].([]string)
        subject := args[3].(string)
        body := args[4].(string)
        attachments := args[5].([]string)

        // Build multipart email message
        message, err := buildMultipartEmailMessage(from, to, subject, body, attachments)
        if err != nil {
            return false, fmt.Errorf("smtp_send_with_attachments error building message: %v", err)
        }

        // Send email
        err = smtp.SendMail(server, nil, from, to, message)
        if err != nil {
            return false, fmt.Errorf("smtp_send_with_attachments error: %v", err)
        }

        return true, nil
    }

    slhelp["email_parse_headers"] = LibHelp{in: "email_content", out: "map", action: "Parse email headers from email content string (RFC compliant)."}
    stdlib["email_parse_headers"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("email_parse_headers", args, 1, "1", "string"); !ok {
            return nil, err
        }

        emailContent := args[0].(string)

        headers := make(map[string]string)
        lines := strings.Split(emailContent, "\r\n")
        if len(lines) == 1 {
            lines = strings.Split(emailContent, "\n")
        }

        var currentHeader string
        var currentValue strings.Builder

        for _, line := range lines {
            line = strings.TrimSpace(line)

            // Empty line marks end of headers
            if line == "" {
                if currentHeader != "" {
                    headers[currentHeader] = strings.TrimSpace(currentValue.String())
                }
                break
            }

            // Check if this is a continuation line (starts with space or tab)
            if (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) && currentHeader != "" {
                currentValue.WriteString(" " + strings.TrimSpace(line))
            } else if strings.Contains(line, ":") {
                // Save previous header if exists
                if currentHeader != "" {
                    headers[currentHeader] = strings.TrimSpace(currentValue.String())
                }

                // Start new header
                parts := strings.SplitN(line, ":", 2)
                if len(parts) == 2 {
                    currentHeader = strings.TrimSpace(parts[0])
                    currentValue.Reset()
                    currentValue.WriteString(strings.TrimSpace(parts[1]))
                }
            }
        }

        // Save last header
        if currentHeader != "" {
            headers[currentHeader] = strings.TrimSpace(currentValue.String())
        }

        return headers, nil
    }

    slhelp["email_get_body"] = LibHelp{in: "email_content", out: "string", action: "Extract email body from email content string (RFC compliant)."}
    stdlib["email_get_body"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("email_get_body", args, 1, "1", "string"); !ok {
            return nil, err
        }

        emailContent := args[0].(string)

        // Split by \r\n first, then \n if needed
        lines := strings.Split(emailContent, "\r\n")
        if len(lines) == 1 {
            lines = strings.Split(emailContent, "\n")
        }

        bodyStart := -1

        // Find the empty line that separates headers from body
        for i, line := range lines {
            if strings.TrimSpace(line) == "" {
                bodyStart = i + 1
                break
            }
        }

        if bodyStart == -1 {
            return "", errors.New("email_get_body: could not find body separator")
        }

        // Reconstruct body with original line endings
        if strings.Contains(emailContent, "\r\n") {
            body := strings.Join(lines[bodyStart:], "\r\n")
            return body, nil
        } else {
            body := strings.Join(lines[bodyStart:], "\n")
            return body, nil
        }
    }

    slhelp["email_get_attachments"] = LibHelp{in: "email_content", out: "[]map", action: "Extract attachment information from email content string."}
    stdlib["email_get_attachments"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("email_get_attachments", args, 1, "1", "string"); !ok {
            return nil, err
        }

        emailContent := args[0].(string)

        // Parse MIME boundaries to find attachments
        attachments := []map[string]any{}

        // Split by \r\n first, then \n if needed
        lines := strings.Split(emailContent, "\r\n")
        if len(lines) == 1 {
            lines = strings.Split(emailContent, "\n")
        }

        var boundary string
        var inAttachment bool
        var attachmentData strings.Builder
        var currentAttachment map[string]string

        for _, line := range lines {
            line = strings.TrimRight(line, "\r")

            // Find boundary
            if strings.Contains(line, "boundary=") {
                parts := strings.Split(line, "boundary=")
                if len(parts) > 1 {
                    boundary = strings.Trim(parts[1], `"`)
                }
            }

            // Check for boundary start
            if boundary != "" && strings.Contains(line, "--"+boundary) {
                if inAttachment && currentAttachment != nil {
                    // Save previous attachment
                    attachment := map[string]any{
                        "filename":     currentAttachment["filename"],
                        "content_type": currentAttachment["content_type"],
                        "content":      attachmentData.String(),
                    }
                    attachments = append(attachments, attachment)
                }

                inAttachment = true
                attachmentData.Reset()
                currentAttachment = make(map[string]string)
                continue
            }

            // Check for boundary end
            if boundary != "" && strings.Contains(line, "--"+boundary+"--") {
                if inAttachment && currentAttachment != nil {
                    // Save last attachment
                    attachment := map[string]any{
                        "filename":     currentAttachment["filename"],
                        "content_type": currentAttachment["content_type"],
                        "content":      attachmentData.String(),
                    }
                    attachments = append(attachments, attachment)
                }
                break
            }

            // Parse attachment headers
            if inAttachment && strings.Contains(line, ":") && !strings.HasPrefix(line, "--") {
                parts := strings.SplitN(line, ":", 2)
                if len(parts) == 2 {
                    header := strings.ToLower(strings.TrimSpace(parts[0]))
                    value := strings.TrimSpace(parts[1])

                    if header == "content-disposition" {
                        // Extract filename
                        if strings.Contains(value, "filename=") {
                            filenameStart := strings.Index(value, "filename=") + 9
                            filenameEnd := strings.Index(value[filenameStart:], ";")
                            if filenameEnd == -1 {
                                filenameEnd = len(value)
                            } else {
                                filenameEnd += filenameStart
                            }
                            filename := strings.Trim(value[filenameStart:filenameEnd], `"`)
                            currentAttachment["filename"] = filename
                        }
                    } else if header == "content-type" {
                        currentAttachment["content_type"] = value
                    }
                }
            } else if inAttachment && line == "" {
                // Empty line marks end of headers, start of content
                continue
            } else if inAttachment {
                // Attachment content
                attachmentData.WriteString(line + "\n")
            }
        }

        return attachments, nil
    }

}

// Helper function to build a simple email message
func buildEmailMessage(from string, to []string, subject string, body string) []byte {
    var message bytes.Buffer

    // Headers
    message.WriteString(fmt.Sprintf("From: %s\r\n", from))
    message.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(to, ", ")))
    message.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
    message.WriteString("MIME-Version: 1.0\r\n")
    message.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
    message.WriteString("\r\n")

    // Body
    message.WriteString(body)

    return message.Bytes()
}

// Helper function to build a multipart email message with attachments
func buildMultipartEmailMessage(from string, to []string, subject string, body string, attachments []string) ([]byte, error) {
    var message bytes.Buffer

    // Headers
    message.WriteString(fmt.Sprintf("From: %s\r\n", from))
    message.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(to, ", ")))
    message.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
    message.WriteString("MIME-Version: 1.0\r\n")

    // Create multipart writer
    multipartWriter := multipart.NewWriter(&message)
    message.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=%s\r\n", multipartWriter.Boundary()))
    message.WriteString("\r\n")

    // Add text part
    textPart, err := multipartWriter.CreatePart(textproto.MIMEHeader{
        "Content-Type": []string{"text/plain; charset=UTF-8"},
    })
    if err != nil {
        return nil, err
    }
    textPart.Write([]byte(body))

    // Add attachments
    for _, attachment := range attachments {
        // Read file content
        fileContent, err := readFileContent(attachment)
        if err != nil {
            return nil, fmt.Errorf("error reading attachment %s: %v", attachment, err)
        }

        // Create attachment part
        attachmentPart, err := multipartWriter.CreatePart(textproto.MIMEHeader{
            "Content-Type":              []string{"application/octet-stream"},
            "Content-Disposition":       []string{fmt.Sprintf("attachment; filename=%s", attachment)},
            "Content-Transfer-Encoding": []string{"base64"},
        })
        if err != nil {
            return nil, err
        }

        // Encode and write attachment
        encoded := base64.StdEncoding.EncodeToString(fileContent)
        attachmentPart.Write([]byte(encoded))
    }

    multipartWriter.Close()
    return message.Bytes(), nil
}

// Helper function to read file content
func readFileContent(filename string) ([]byte, error) {
    // Read file content using os.ReadFile
    content, err := os.ReadFile(filename)
    if err != nil {
        return nil, fmt.Errorf("error reading file %s: %v", filename, err)
    }
    return content, nil
}
