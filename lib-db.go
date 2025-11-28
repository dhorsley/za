//go:build !test

package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"gopkg.in/yaml.v3"
)

func buildDbLib() {

	features["db"] = Feature{version: 1, category: "db"}
	categories["db"] = []string{"db_init", "db_query", "db_close"} // ,"db_prepared_query"}

	// open a db connection
	slhelp["db_init"] = LibHelp{in: "string", out: "handle",
		action: "Returns a database connection [#i1]handle[#i0], with a default schema of [#i1]string[#i0] based on\n[#SOL]" +
			"inbound environmental variables. (ZA_DB_HOST, ZA_DB_ENGINE, ZA_DB_PORT, ZA_DB_USER, ZA_DB_PASS.)\n[#SOL]" +
			"Only 'mysql' is currently supported as an engine type."}
	stdlib["db_init"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("db_init", args, 1, "1", "string"); !ok {
			return nil, err
		}

		schema := args[0].(string)

		dbhost, ex_host := os.LookupEnv("ZA_DB_HOST")
		dbeng, ex_eng := os.LookupEnv("ZA_DB_ENGINE")
		dbport, ex_port := os.LookupEnv("ZA_DB_PORT")
		dbuser, ex_user := os.LookupEnv("ZA_DB_USER")
		dbpass, ex_pass := os.LookupEnv("ZA_DB_PASS")

		if !ex_eng {
			return nil, errors.New("Error: No DB engine specified.")
		}

		// instantiate the db connection:
		var dbh *sql.DB

		switch dbeng {
		case "mysql":
			if !(ex_host || ex_port || ex_user || ex_pass) {
				return nil, errors.New("Error: Missing DB details at startup.")
			}
			dbh, err = sql.Open(dbeng, dbuser+":"+dbpass+"@tcp("+dbhost+":"+dbport+")/"+schema)
		case "sqlite3":
			dbh, err = sql.Open(dbeng, schema) // schema will be path or uri
		}
		if err != nil {
			return nil, err
		}

		return dbh, err

	}

	// close a db connection
	slhelp["db_close"] = LibHelp{in: "handle", out: "", action: "Closes the database connection."}
	stdlib["db_close"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("db_close", args, 1, "1", "*sql.DB"); !ok {
			return nil, err
		}
		args[0].(*sql.DB).Close()
		return nil, nil
	}

	slhelp["db_query"] = LibHelp{in: "handle,query[,options]", out: "string", action: `Database query with optional map configuration. Options: map(.params [values], .separator "|", .timeout 30, .fetch_size 1000, .limit 100, .format "string|json|csv|tsv|table|map|array|yaml|xml|jsonl")`}
	stdlib["db_query"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("db_query", args, 3,
			"3", "*sql.DB", "string", "map",
			"3", "*sql.DB", "string", "string",
			"2", "*sql.DB", "string"); !ok {
			return nil, err
		}

		var q string
		var dbh *sql.DB
		var options map[string]interface{}

		dbh = args[0].(*sql.DB)
		q = args[1].(string)

		// Parse options if provided
		if len(args) == 3 {
			switch v := args[2].(type) {
			case map[string]interface{}:
				options = v
			case map[any]any:
				options = make(map[string]interface{})
				for key, val := range v {
					if ks, ok := key.(string); ok {
						options[ks] = val
					}
				}
			case string:
				// Backward compatibility: treat string as separator
				options = map[string]interface{}{"separator": v}
			default:
				return nil, errors.New("db_query third argument must be map (options) or string (separator)")
			}
		}

		// Set defaults
		fsep := "|"
		limit := -1
		format := "string" // Default format
		var params []interface{}

		// Extract options
		if options != nil {
			if sep, exists := options["separator"]; exists {
				if sepStr, ok := sep.(string); ok {
					fsep = sepStr
				}
			}
			if l, exists := options["limit"]; exists {
				if lInt, ok := l.(int); ok {
					limit = lInt
				} else if lFloat, ok := l.(float64); ok {
					limit = int(lFloat)
				}
			}
			if f, exists := options["format"]; exists {
				if fStr, ok := f.(string); ok {
					// Validate format option
					validFormats := []string{"string", "json", "csv", "tsv", "table", "map", "array", "yaml", "xml", "jsonl"}
					valid := false
					for _, vf := range validFormats {
						if fStr == vf {
							valid = true
							break
						}
					}
					if !valid {
						return nil, fmt.Errorf("invalid format '%s'. Valid formats: %v", fStr, validFormats)
					}
					format = fStr
				}
			}
			if p, exists := options["params"]; exists {
				if pSlice, ok := p.([]interface{}); ok {
					params = pSlice
				} else if pArray, ok := p.([]any); ok {
					params = make([]interface{}, len(pArray))
					for i, v := range pArray {
						params[i] = v
					}
				}
			}
		}

		// Test connection
		if err := dbh.Ping(); err != nil {
			return nil, fmt.Errorf("database connection failed: %v", err)
		}

		// Execute query with or without parameters
		var rows *sql.Rows
		if len(params) > 0 {
			// Use prepared statement
			stmt, err := dbh.Prepare(q)
			if err != nil {
				return nil, fmt.Errorf("failed to prepare statement: %v", err)
			}
			defer stmt.Close()

			rows, err = stmt.Query(params...)
			if err != nil {
				return nil, fmt.Errorf("failed to execute prepared statement: %v", err)
			}
		} else {
			// Use direct query
			rows, err = dbh.Query(q)
			if err != nil {
				return nil, fmt.Errorf("failed to execute query: %v", err)
			}
		}
		defer rows.Close()

		// Get column information
		_, err = rows.ColumnTypes()
		if err != nil {
			return nil, fmt.Errorf("failed to get column types: %v", err)
		}

		// Get column names
		columnNames, err := rows.Columns()
		if err != nil {
			return nil, fmt.Errorf("failed to get column names: %v", err)
		}

		// Process results based on format
		switch format {
		case "json":
			return formatJSON(rows, columnNames, limit)
		case "csv":
			return formatCSV(rows, columnNames, limit)
		case "tsv":
			return formatTSV(rows, columnNames, limit)
		case "table":
			return formatTable(rows, columnNames, limit)
		case "map":
			return formatMap(rows, columnNames, limit)
		case "array":
			return formatArray(rows, columnNames, limit)
		case "yaml":
			return formatYAML(rows, columnNames, limit)
		case "xml":
			return formatXML(rows, columnNames, limit)
		case "jsonl":
			return formatJSONL(rows, columnNames, limit)
		case "string":
			fallthrough
		default:
			return formatString(rows, fsep, limit)
		}
	}
}

// Helper functions for different output formats

func formatJSON(rows *sql.Rows, columnNames []string, limit int) (any, error) {
	var result []map[string]interface{}
	rc := 0

	vals := make([]any, len(columnNames))
	for i := range columnNames {
		vals[i] = new(sql.RawBytes)
	}

	for rows.Next() {
		err := rows.Scan(vals...)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}

		row := make(map[string]interface{})
		for i, val := range vals {
			// Convert sql.RawBytes to string
			if rawBytes, ok := val.(*sql.RawBytes); ok {
				row[columnNames[i]] = string(*rawBytes)
			} else {
				row[columnNames[i]] = val
			}
		}
		result = append(result, row)
		rc++

		if limit > 0 && rc >= limit {
			break
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %v", err)
	}

	// Convert to JSON string
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %v", err)
	}

	return string(jsonBytes), nil
}

func formatCSV(rows *sql.Rows, columnNames []string, limit int) (any, error) {
	var result strings.Builder
	rc := 0

	// Add header row
	for i, col := range columnNames {
		if i > 0 {
			result.WriteString(",")
		}
		result.WriteString(fmt.Sprintf("%q", col))
	}
	result.WriteString("\n")

	vals := make([]any, len(columnNames))
	for i := range columnNames {
		vals[i] = new(sql.RawBytes)
	}

	for rows.Next() {
		err := rows.Scan(vals...)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}

		for i, val := range vals {
			if i > 0 {
				result.WriteString(",")
			}
			// Convert sql.RawBytes to string and escape for CSV
			if rawBytes, ok := val.(*sql.RawBytes); ok {
				cellValue := string(*rawBytes)
				// Escape quotes and wrap in quotes if needed
				if strings.Contains(cellValue, `"`) || strings.Contains(cellValue, ",") || strings.Contains(cellValue, "\n") {
					cellValue = strings.ReplaceAll(cellValue, `"`, `""`)
					cellValue = fmt.Sprintf("%q", cellValue)
				}
				result.WriteString(cellValue)
			} else {
				result.WriteString(fmt.Sprintf("%v", val))
			}
		}
		result.WriteString("\n")
		rc++

		if limit > 0 && rc >= limit {
			break
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %v", err)
	}

	return result.String(), nil
}

func formatTable(rows *sql.Rows, columnNames []string, limit int) (any, error) {
	var result []string
	rc := 0

	// Calculate column widths
	colWidths := make([]int, len(columnNames))
	for i, col := range columnNames {
		colWidths[i] = len(col)
	}

	// First pass: determine column widths
	vals := make([]any, len(columnNames))
	for i := range columnNames {
		vals[i] = new(sql.RawBytes)
	}

	// Store all rows to calculate widths
	var allRows [][]string
	for rows.Next() {
		err := rows.Scan(vals...)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}

		row := make([]string, len(columnNames))
		for i, val := range vals {
			if rawBytes, ok := val.(*sql.RawBytes); ok {
				row[i] = string(*rawBytes)
			} else {
				row[i] = fmt.Sprintf("%v", val)
			}
			if len(row[i]) > colWidths[i] {
				colWidths[i] = len(row[i])
			}
		}
		allRows = append(allRows, row)
		rc++

		if limit > 0 && rc >= limit {
			break
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %v", err)
	}

	// Build table
	// Header
	header := "|"
	for i, col := range columnNames {
		header += fmt.Sprintf(" %-*s |", colWidths[i], col)
	}
	result = append(result, header)

	// Separator
	separator := "|"
	for _, width := range colWidths {
		separator += strings.Repeat("-", width+2) + "|"
	}
	result = append(result, separator)

	// Data rows
	for _, row := range allRows {
		rowStr := "|"
		for i, cell := range row {
			rowStr += fmt.Sprintf(" %-*s |", colWidths[i], cell)
		}
		result = append(result, rowStr)
	}

	return result, nil
}

func formatMap(rows *sql.Rows, columnNames []string, limit int) (any, error) {
	var result []map[string]interface{}
	rc := 0

	vals := make([]any, len(columnNames))
	for i := range columnNames {
		vals[i] = new(sql.RawBytes)
	}

	for rows.Next() {
		err := rows.Scan(vals...)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}

		row := make(map[string]interface{})
		for i, val := range vals {
			// Convert sql.RawBytes to appropriate type
			if rawBytes, ok := val.(*sql.RawBytes); ok {
				// Try to convert to number if possible
				strVal := string(*rawBytes)
				if intVal, err := strconv.Atoi(strVal); err == nil {
					row[columnNames[i]] = intVal
				} else if floatVal, err := strconv.ParseFloat(strVal, 64); err == nil {
					row[columnNames[i]] = floatVal
				} else if strVal == "true" || strVal == "false" {
					row[columnNames[i]] = strVal == "true"
				} else {
					row[columnNames[i]] = strVal
				}
			} else {
				row[columnNames[i]] = val
			}
		}
		result = append(result, row)
		rc++

		if limit > 0 && rc >= limit {
			break
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %v", err)
	}

	return result, nil
}

func formatArray(rows *sql.Rows, columnNames []string, limit int) (any, error) {
	var result [][]interface{}
	rc := 0

	vals := make([]any, len(columnNames))
	for i := range columnNames {
		vals[i] = new(sql.RawBytes)
	}

	for rows.Next() {
		err := rows.Scan(vals...)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}

		row := make([]interface{}, len(columnNames))
		for i, val := range vals {
			// Convert sql.RawBytes to appropriate type
			if rawBytes, ok := val.(*sql.RawBytes); ok {
				strVal := string(*rawBytes)
				if intVal, err := strconv.Atoi(strVal); err == nil {
					row[i] = intVal
				} else if floatVal, err := strconv.ParseFloat(strVal, 64); err == nil {
					row[i] = floatVal
				} else if strVal == "true" || strVal == "false" {
					row[i] = strVal == "true"
				} else {
					row[i] = strVal
				}
			} else {
				row[i] = val
			}
		}
		result = append(result, row)
		rc++

		if limit > 0 && rc >= limit {
			break
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %v", err)
	}

	return result, nil
}

func formatTSV(rows *sql.Rows, columnNames []string, limit int) (any, error) {
	var result strings.Builder
	rc := 0

	// Add header row
	for i, col := range columnNames {
		if i > 0 {
			result.WriteString("\t")
		}
		result.WriteString(col)
	}
	result.WriteString("\n")

	vals := make([]any, len(columnNames))
	for i := range columnNames {
		vals[i] = new(sql.RawBytes)
	}

	for rows.Next() {
		err := rows.Scan(vals...)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}

		for i, val := range vals {
			if i > 0 {
				result.WriteString("\t")
			}
			if rawBytes, ok := val.(*sql.RawBytes); ok {
				result.WriteString(string(*rawBytes))
			} else {
				result.WriteString(fmt.Sprintf("%v", val))
			}
		}
		result.WriteString("\n")
		rc++

		if limit > 0 && rc >= limit {
			break
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %v", err)
	}

	return result.String(), nil
}

func formatYAML(rows *sql.Rows, columnNames []string, limit int) (any, error) {
	var result []map[string]interface{}
	rc := 0

	vals := make([]any, len(columnNames))
	for i := range columnNames {
		vals[i] = new(sql.RawBytes)
	}

	for rows.Next() {
		err := rows.Scan(vals...)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}

		row := make(map[string]interface{})
		for i, val := range vals {
			if rawBytes, ok := val.(*sql.RawBytes); ok {
				strVal := string(*rawBytes)
				// Try type conversion for YAML
				if intVal, err := strconv.Atoi(strVal); err == nil {
					row[columnNames[i]] = intVal
				} else if floatVal, err := strconv.ParseFloat(strVal, 64); err == nil {
					row[columnNames[i]] = floatVal
				} else if strVal == "true" || strVal == "false" {
					row[columnNames[i]] = strVal == "true"
				} else {
					row[columnNames[i]] = strVal
				}
			} else {
				row[columnNames[i]] = val
			}
		}
		result = append(result, row)
		rc++

		if limit > 0 && rc >= limit {
			break
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %v", err)
	}

	// Convert to YAML string
	yamlBytes, err := yaml.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal YAML: %v", err)
	}

	return string(yamlBytes), nil
}

func formatXML(rows *sql.Rows, columnNames []string, limit int) (any, error) {
	var result []string
	rc := 0

	// Start XML
	result = append(result, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>")
	result = append(result, "<results>")

	vals := make([]any, len(columnNames))
	for i := range columnNames {
		vals[i] = new(sql.RawBytes)
	}

	for rows.Next() {
		err := rows.Scan(vals...)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}

		row := "  <row>"
		for i, val := range vals {
			if rawBytes, ok := val.(*sql.RawBytes); ok {
				cellValue := string(*rawBytes)
				// Escape XML characters
				cellValue = strings.ReplaceAll(cellValue, "&", "&amp;")
				cellValue = strings.ReplaceAll(cellValue, "<", "&lt;")
				cellValue = strings.ReplaceAll(cellValue, ">", "&gt;")
				cellValue = strings.ReplaceAll(cellValue, "\"", "&quot;")
				cellValue = strings.ReplaceAll(cellValue, "'", "&apos;")
				row += fmt.Sprintf("    <%s>%s</%s>", columnNames[i], cellValue, columnNames[i])
			} else {
				row += fmt.Sprintf("    <%s>%v</%s>", columnNames[i], val, columnNames[i])
			}
		}
		row += "  </row>"
		result = append(result, row)
		rc++

		if limit > 0 && rc >= limit {
			break
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %v", err)
	}

	result = append(result, "</results>")
	return result, nil
}

func formatJSONL(rows *sql.Rows, columnNames []string, limit int) (any, error) {
	var result []string
	rc := 0

	vals := make([]any, len(columnNames))
	for i := range columnNames {
		vals[i] = new(sql.RawBytes)
	}

	for rows.Next() {
		err := rows.Scan(vals...)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}

		row := make(map[string]interface{})
		for i, val := range vals {
			if rawBytes, ok := val.(*sql.RawBytes); ok {
				row[columnNames[i]] = string(*rawBytes)
			} else {
				row[columnNames[i]] = val
			}
		}

		// Convert to JSON string (one line per row)
		jsonBytes, err := json.Marshal(row)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal JSON: %v", err)
		}
		result = append(result, string(jsonBytes))
		rc++

		if limit > 0 && rc >= limit {
			break
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %v", err)
	}

	return result, nil
}

func formatString(rows *sql.Rows, separator string, limit int) (any, error) {
	l := make([]string, 0, 50)
	rc := 0

	// Use columnNames length instead of rows.ColumnTypes()
	columnNames, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get column names: %v", err)
	}
	vals := make([]any, len(columnNames))
	for i := range vals {
		vals[i] = new(sql.RawBytes)
	}

	for rows.Next() {
		err := rows.Scan(vals...)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}

		// Build row string
		rowStr := ""
		for v := range vals {
			rowStr += sf("%s"+separator, vals[v])[1:]
		}
		if len(rowStr) > 0 {
			rowStr = rowStr[0 : len(rowStr)-1]
		}

		l = append(l, rowStr)
		rc++

		// Check limit
		if limit > 0 && rc >= limit {
			break
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %v", err)
	}

	return l[:rc], nil
}
