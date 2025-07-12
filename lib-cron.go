//go:build !test
// +build !test

package main

import (
    "fmt"
    "strconv"
    "strings"
    "time"
)

func buildCronLib() {

    features["cron"] = Feature{version: 1, category: "date"}
    categories["cron"] = []string{"cron_parse", "quartz_to_cron", "cron_next", "cron_validate"}

    slhelp["cron_parse"] = LibHelp{in: "cron_schedule", out: "any", action: "Parse cron schedule and return structured description of what it means."}
    stdlib["cron_parse"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("cron_parse", args, 1, "1", "string"); !ok {
            return nil, err
        }

        cronSchedule := args[0].(string)
        result, err := parseCronDescription(cronSchedule)
        if err != nil {
            return nil, fmt.Errorf("cron_parse error: %v", err)
        }

        return result, nil
    }

    slhelp["quartz_to_cron"] = LibHelp{in: "quartz_schedule", out: "string", action: "Convert Quartz format schedule to cron format."}
    stdlib["quartz_to_cron"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("quartz_to_cron", args, 1, "1", "string"); !ok {
            return nil, err
        }

        quartzSchedule := args[0].(string)

        cronSchedule, err := convertQuartzToCron(quartzSchedule)
        if err != nil {
            return nil, fmt.Errorf("quartz_to_cron error: %v", err)
        }

        return cronSchedule, nil
    }

    slhelp["cron_next"] = LibHelp{in: "cron_schedule, [from_epoch]", out: "int", action: "Get seconds until next cron execution."}
    stdlib["cron_next"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("cron_next", args, 2,
            "2", "string", "int",
            "1", "string"); !ok {
            return nil, err
        }

        cronSchedule := args[0].(string)
        var fromTime time.Time

        if len(args) == 2 {
            when := args[1].(int)
            whensecs := int(when / 1000000000)
            whennano := when - (whensecs * 1000000000)
            fromTime = time.Unix(int64(whensecs), int64(whennano))
        } else {
            fromTime = time.Now()
        }

        // Simple cron parser - this is a basic implementation
        // For production use, you might want a more robust cron parser
        nextTime, err := parseCronSchedule(cronSchedule, fromTime)
        if err != nil {
            return 0, err
        }

        return int(nextTime.Sub(fromTime).Seconds()), nil
    }

    slhelp["cron_validate"] = LibHelp{in: "cron_schedule", out: "bool", action: "Validate cron schedule syntax."}
    stdlib["cron_validate"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("cron_validate", args, 1, "1", "string"); !ok {
            return nil, err
        }

        cronSchedule := args[0].(string)
        isValid, err := validateCronSchedule(cronSchedule)
        if err != nil {
            return false, err
        }

        return isValid, nil
    }

}

// Convert Quartz format to cron format
// Quartz format: seconds minutes hours day-of-month month day-of-week [year]
// Cron format: minutes hours day-of-month month day-of-week
func convertQuartzToCron(quartz string) (string, error) {
    fields := strings.Fields(quartz)
    if len(fields) != 6 && len(fields) != 7 {
        return "", fmt.Errorf("invalid Quartz schedule: expected 6 or 7 fields, got %d", len(fields))
    }

    // Quartz fields: seconds minutes hours day-of-month month day-of-week [year]
    // Cron fields: minutes hours day-of-month month day-of-week
    minutes := fields[1]
    hours := fields[2]
    dayOfMonth := fields[3]
    month := fields[4]
    dayOfWeek := fields[5]

    // Convert day-of-week from Quartz (1-7, Sunday=1) to cron (0-6, Sunday=0)
    if dayOfWeek != "*" && dayOfWeek != "?" {
        cronDayOfWeek, err := convertQuartzDayOfWeek(dayOfWeek)
        if err != nil {
            return "", err
        }
        dayOfWeek = cronDayOfWeek
    }

    // Convert month from Quartz (1-12) to cron (1-12) - same format
    if month != "*" && month != "?" {
        cronMonth, err := convertQuartzMonth(month)
        if err != nil {
            return "", err
        }
        month = cronMonth
    }

    // Convert day-of-month - handle "?" (no specific value) to "*"
    if dayOfMonth == "?" {
        dayOfMonth = "*"
    }

    // Convert month - handle "?" (no specific value) to "*"
    if month == "?" {
        month = "*"
    }

    // Convert day-of-week - handle "?" (no specific value) to "*"
    if dayOfWeek == "?" {
        dayOfWeek = "*"
    }

    // Build cron schedule: minutes hours day-of-month month day-of-week
    cronSchedule := fmt.Sprintf("%s %s %s %s %s", minutes, hours, dayOfMonth, month, dayOfWeek)

    return cronSchedule, nil
}

// Convert Quartz day-of-week (1-7, Sunday=1) to cron day-of-week (0-6, Sunday=0)
func convertQuartzDayOfWeek(quartzDay string) (string, error) {
    // Handle day name abbreviations
    quartzDay = strings.ToUpper(quartzDay)
    dayNameMap := map[string]int{
        "SUN": 1, "SUNDAY": 1,
        "MON": 2, "MONDAY": 2,
        "TUE": 3, "TUESDAY": 3,
        "WED": 4, "WEDNESDAY": 4,
        "THU": 5, "THURSDAY": 5,
        "FRI": 6, "FRIDAY": 6,
        "SAT": 7, "SATURDAY": 7,
    }

    // Check if it's a day name
    if dayNum, exists := dayNameMap[quartzDay]; exists {
        quartzDay = strconv.Itoa(dayNum)
    }

    // Handle ranges like "1-5" or "MON-FRI"
    if strings.Contains(quartzDay, "-") {
        parts := strings.Split(quartzDay, "-")
        if len(parts) == 2 {
            start, err1 := parseDayOfWeek(parts[0])
            end, err2 := parseDayOfWeek(parts[1])
            if err1 != nil || err2 != nil {
                return "", fmt.Errorf("invalid day-of-week range: %s-%s", parts[0], parts[1])
            }

            // Convert Quartz (1-7) to cron (0-6)
            cronStart := (start - 1) % 7
            cronEnd := (end - 1) % 7

            return fmt.Sprintf("%d-%d", cronStart, cronEnd), nil
        }
    }

    // Handle comma-separated values like "1,2,3" or "MON,WED,FRI"
    if strings.Contains(quartzDay, ",") {
        parts := strings.Split(quartzDay, ",")
        var cronDays []string
        for _, part := range parts {
            day, err := parseDayOfWeek(strings.TrimSpace(part))
            if err != nil {
                return "", fmt.Errorf("invalid day-of-week value: %s", part)
            }
            // Convert Quartz (1-7) to cron (0-6)
            cronDay := (day - 1) % 7
            cronDays = append(cronDays, strconv.Itoa(cronDay))
        }
        return strings.Join(cronDays, ","), nil
    }

    // Handle step values like "*/2"
    if strings.Contains(quartzDay, "/") {
        parts := strings.Split(quartzDay, "/")
        if len(parts) == 2 {
            step, err := strconv.Atoi(parts[1])
            if err != nil {
                return "", fmt.Errorf("invalid day-of-week step: %s", parts[1])
            }
            return fmt.Sprintf("*/%d", step), nil
        }
    }

    // Single value
    day, err := parseDayOfWeek(quartzDay)
    if err != nil {
        return "", err
    }

    // Convert Quartz (1-7) to cron (0-6)
    cronDay := (day - 1) % 7
    return strconv.Itoa(cronDay), nil
}

// parseDayOfWeek parses a day-of-week value, handling both numbers and names
func parseDayOfWeek(dayStr string) (int, error) {
    // Handle day name abbreviations
    dayStr = strings.ToUpper(dayStr)
    dayNameMap := map[string]int{
        "SUN": 1, "SUNDAY": 1,
        "MON": 2, "MONDAY": 2,
        "TUE": 3, "TUESDAY": 3,
        "WED": 4, "WEDNESDAY": 4,
        "THU": 5, "THURSDAY": 5,
        "FRI": 6, "FRIDAY": 6,
        "SAT": 7, "SATURDAY": 7,
    }

    // Check if it's a day name
    if dayNum, exists := dayNameMap[dayStr]; exists {
        return dayNum, nil
    }

    // Try parsing as number
    day, err := strconv.Atoi(dayStr)
    if err != nil {
        return 0, fmt.Errorf("invalid day-of-week value: %s", dayStr)
    }

    if day < 1 || day > 7 {
        return 0, fmt.Errorf("day-of-week value out of range: %d (valid: 1-7)", day)
    }

    return day, nil
}

// Convert Quartz month (1-12) to cron month (1-12)
func convertQuartzMonth(quartzMonth string) (string, error) {
    // Handle ranges like "1-5"
    if strings.Contains(quartzMonth, "-") {
        parts := strings.Split(quartzMonth, "-")
        if len(parts) == 2 {
            start, err := strconv.Atoi(parts[0])
            if err != nil {
                return "", fmt.Errorf("invalid month range start: %s", parts[0])
            }
            end, err := strconv.Atoi(parts[1])
            if err != nil {
                return "", fmt.Errorf("invalid month range end: %s", parts[1])
            }

            // Months are the same in both formats (1-12)
            return fmt.Sprintf("%d-%d", start, end), nil
        }
    }

    // Handle comma-separated values like "1,2,3"
    if strings.Contains(quartzMonth, ",") {
        parts := strings.Split(quartzMonth, ",")
        var cronMonths []string
        for _, part := range parts {
            month, err := strconv.Atoi(strings.TrimSpace(part))
            if err != nil {
                return "", fmt.Errorf("invalid month value: %s", part)
            }
            // Months are the same in both formats (1-12)
            cronMonths = append(cronMonths, strconv.Itoa(month))
        }
        return strings.Join(cronMonths, ","), nil
    }

    // Handle step values like "*/2"
    if strings.Contains(quartzMonth, "/") {
        parts := strings.Split(quartzMonth, "/")
        if len(parts) == 2 {
            step, err := strconv.Atoi(parts[1])
            if err != nil {
                return "", fmt.Errorf("invalid month step: %s", parts[1])
            }
            return fmt.Sprintf("*/%d", step), nil
        }
    }

    // Single value
    month, err := strconv.Atoi(quartzMonth)
    if err != nil {
        return "", fmt.Errorf("invalid month value: %s", quartzMonth)
    }

    // Months are the same in both formats (1-12)
    return strconv.Itoa(month), nil
}

// Helper function to parse cron schedule
func parseCronSchedule(schedule string, fromTime time.Time) (time.Time, error) {
    fields := strings.Fields(schedule)
    if len(fields) != 5 {
        return time.Time{}, fmt.Errorf("invalid cron schedule: expected 5 fields, got %d", len(fields))
    }

    // Parse each field: minute hour day month weekday
    minute := parseCronField(fields[0], 0, 59)
    hour := parseCronField(fields[1], 0, 23)
    day := parseCronField(fields[2], 1, 31)
    month := parseCronField(fields[3], 1, 12)
    weekday := parseCronField(fields[4], 0, 6)

    // Find next occurrence
    nextTime := fromTime
    // Round down to the nearest minute to avoid missing the current minute
    nextTime = time.Date(nextTime.Year(), nextTime.Month(), nextTime.Day(), nextTime.Hour(), nextTime.Minute(), 0, 0, nextTime.Location())

    // Look ahead up to 2 years to find the next occurrence
    maxAttempts := 2 * 365 * 24 * 60 // 2 years worth of minutes
    for attempt := 0; attempt < maxAttempts; attempt++ {
        // Check if current time matches all fields
        if matchesCronTime(nextTime, minute, hour, day, month, weekday) {
            // If the matched time is in the past or current, move to next minute
            if !nextTime.After(fromTime) {
                nextTime = nextTime.Add(time.Minute)
                continue
            }
            return nextTime, nil
        }
        nextTime = nextTime.Add(time.Minute)
    }

    return time.Time{}, fmt.Errorf("could not find next cron execution time within 2 years")
}

// Parse a single cron field (e.g., "*/5", "1,2,3", "1-5")
func parseCronField(field string, min, max int) []int {
    var values []int

    if field == "*" {
        // All values
        for i := min; i <= max; i++ {
            values = append(values, i)
        }
        return values
    }

    // Handle ranges like "1-5"
    if strings.Contains(field, "-") {
        parts := strings.Split(field, "-")
        if len(parts) == 2 {
            start, _ := strconv.Atoi(parts[0])
            end, _ := strconv.Atoi(parts[1])
            for i := start; i <= end; i++ {
                if i >= min && i <= max {
                    values = append(values, i)
                }
            }
        }
        return values
    }

    // Handle step values like "*/5"
    if strings.Contains(field, "/") {
        parts := strings.Split(field, "/")
        if len(parts) == 2 {
            step, _ := strconv.Atoi(parts[1])
            for i := min; i <= max; i += step {
                values = append(values, i)
            }
        }
        return values
    }

    // Handle comma-separated values like "1,2,3"
    if strings.Contains(field, ",") {
        parts := strings.Split(field, ",")
        for _, part := range parts {
            if val, err := strconv.Atoi(part); err == nil {
                if val >= min && val <= max {
                    values = append(values, val)
                }
            }
        }
        return values
    }

    // Single value
    if val, err := strconv.Atoi(field); err == nil {
        if val >= min && val <= max {
            values = append(values, val)
        }
    }

    return values
}

// Check if a time matches the cron specification
func matchesCronTime(t time.Time, minute, hour, day, month, weekday []int) bool {
    // Check minute
    if !contains(minute, t.Minute()) {
        return false
    }

    // Check hour
    if !contains(hour, t.Hour()) {
        return false
    }

    // Check day of month
    if !contains(day, t.Day()) {
        return false
    }

    // Check month
    if !contains(month, int(t.Month())) {
        return false
    }

    // Check day of week (0=Sunday, 6=Saturday)
    if !contains(weekday, int(t.Weekday())) {
        return false
    }

    return true
}

// Helper function to check if a slice contains a value
func contains(slice []int, value int) bool {
    for _, v := range slice {
        if v == value {
            return true
        }
    }
    return false
}

// parseCronDescription parses a cron expression and returns a structured description
func parseCronDescription(schedule string) (map[string]any, error) {
    fields := strings.Fields(schedule)
    if len(fields) != 5 {
        return nil, fmt.Errorf("invalid cron schedule: expected 5 fields, got %d", len(fields))
    }

    result := map[string]any{
        "minute":      parseCronFieldDescription(fields[0], "minute", 0, 59),
        "hour":        parseCronFieldDescription(fields[1], "hour", 0, 23),
        "day":         parseCronFieldDescription(fields[2], "day", 1, 31),
        "month":       parseCronFieldDescription(fields[3], "month", 1, 12),
        "weekday":     parseCronFieldDescription(fields[4], "weekday", 0, 6),
        "expression":  schedule,
        "description": generateHumanDescription(fields),
    }

    return result, nil
}

// parseCronFieldDescription parses a single cron field and returns a description
func parseCronFieldDescription(field, fieldName string, min, max int) map[string]any {
    result := map[string]any{
        "field": field,
        "type":  "unknown",
    }

    if field == "*" {
        result["type"] = "all"
        result["description"] = fmt.Sprintf("every %s", fieldName)
        return result
    }

    // Handle ranges like "1-5"
    if strings.Contains(field, "-") {
        parts := strings.Split(field, "-")
        if len(parts) == 2 {
            start, err1 := strconv.Atoi(parts[0])
            end, err2 := strconv.Atoi(parts[1])
            if err1 == nil && err2 == nil && start >= min && end <= max && start <= end {
                result["type"] = "range"
                result["start"] = start
                result["end"] = end
                result["description"] = fmt.Sprintf("%s %d to %d", fieldName, start, end)
                return result
            }
        }
    }

    // Handle step values like "*/5"
    if strings.Contains(field, "/") {
        parts := strings.Split(field, "/")
        if len(parts) == 2 {
            step, err := strconv.Atoi(parts[1])
            if err == nil && step > 0 {
                result["type"] = "step"
                result["step"] = step
                if parts[0] == "*" {
                    result["description"] = fmt.Sprintf("every %d %s", step, fieldName)
                } else {
                    result["description"] = fmt.Sprintf("every %d %s starting from %s", step, fieldName, parts[0])
                }
                return result
            }
        }
    }

    // Handle comma-separated values like "1,2,3"
    if strings.Contains(field, ",") {
        parts := strings.Split(field, ",")
        var values []int
        for _, part := range parts {
            if val, err := strconv.Atoi(strings.TrimSpace(part)); err == nil && val >= min && val <= max {
                values = append(values, val)
            }
        }
        if len(values) > 0 {
            result["type"] = "list"
            result["values"] = values
            result["description"] = fmt.Sprintf("%s %v", fieldName, values)
            return result
        }
    }

    // Single value
    if val, err := strconv.Atoi(field); err == nil && val >= min && val <= max {
        result["type"] = "specific"
        result["value"] = val
        result["description"] = fmt.Sprintf("%s %d", fieldName, val)
        return result
    }

    result["type"] = "invalid"
    result["description"] = fmt.Sprintf("invalid %s: %s", fieldName, field)
    return result
}

// generateHumanDescription creates a human-readable description of the cron expression
func generateHumanDescription(fields []string) string {
    var parts []string

    // Minute
    if fields[0] == "0" {
        parts = append(parts, "at minute 0")
    } else if fields[0] == "*" {
        parts = append(parts, "every minute")
    } else {
        parts = append(parts, fmt.Sprintf("at minute %s", fields[0]))
    }

    // Hour
    if fields[1] == "0" {
        parts = append(parts, "at hour 0")
    } else if fields[1] == "*" {
        parts = append(parts, "every hour")
    } else {
        parts = append(parts, fmt.Sprintf("at hour %s", fields[1]))
    }

    // Day of month
    if fields[2] == "*" {
        parts = append(parts, "every day")
    } else {
        parts = append(parts, fmt.Sprintf("on day %s", fields[2]))
    }

    // Month
    if fields[3] == "*" {
        parts = append(parts, "every month")
    } else {
        monthNames := []string{"", "January", "February", "March", "April", "May", "June",
            "July", "August", "September", "October", "November", "December"}
        if month, err := strconv.Atoi(fields[3]); err == nil && month >= 1 && month <= 12 {
            parts = append(parts, fmt.Sprintf("in %s", monthNames[month]))
        } else {
            parts = append(parts, fmt.Sprintf("in month %s", fields[3]))
        }
    }

    // Day of week
    if fields[4] == "*" {
        parts = append(parts, "every day of week")
    } else {
        dayNames := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
        if day, err := strconv.Atoi(fields[4]); err == nil && day >= 0 && day <= 6 {
            parts = append(parts, fmt.Sprintf("on %s", dayNames[day]))
        } else {
            parts = append(parts, fmt.Sprintf("on day of week %s", fields[4]))
        }
    }

    return strings.Join(parts, ", ")
}

// validateCronSchedule performs comprehensive validation of a cron expression
func validateCronSchedule(schedule string) (bool, error) {
    fields := strings.Fields(schedule)
    if len(fields) != 5 {
        return false, fmt.Errorf("invalid cron schedule: expected 5 fields, got %d", len(fields))
    }

    // Validate each field
    if err := validateCronField(fields[0], "minute", 0, 59); err != nil {
        return false, err
    }
    if err := validateCronField(fields[1], "hour", 0, 23); err != nil {
        return false, err
    }
    if err := validateCronField(fields[2], "day", 1, 31); err != nil {
        return false, err
    }
    if err := validateCronField(fields[3], "month", 1, 12); err != nil {
        return false, err
    }
    if err := validateCronField(fields[4], "weekday", 0, 6); err != nil {
        return false, err
    }

    // Additional logical validation
    if err := validateCronLogic(fields); err != nil {
        return false, err
    }

    return true, nil
}

// validateCronField validates a single cron field
func validateCronField(field, fieldName string, min, max int) error {
    if field == "*" {
        return nil // Wildcard is always valid
    }

    // Handle ranges like "1-5"
    if strings.Contains(field, "-") {
        parts := strings.Split(field, "-")
        if len(parts) != 2 {
            return fmt.Errorf("invalid %s range format: %s", fieldName, field)
        }

        start, err1 := strconv.Atoi(parts[0])
        end, err2 := strconv.Atoi(parts[1])
        if err1 != nil || err2 != nil {
            return fmt.Errorf("invalid %s range values: %s", fieldName, field)
        }

        if start < min || end > max || start > end {
            return fmt.Errorf("invalid %s range: %d-%d (valid range: %d-%d)", fieldName, start, end, min, max)
        }
        return nil
    }

    // Handle step values like "*/5" or "1-10/2"
    if strings.Contains(field, "/") {
        parts := strings.Split(field, "/")
        if len(parts) != 2 {
            return fmt.Errorf("invalid %s step format: %s", fieldName, field)
        }

        step, err := strconv.Atoi(parts[1])
        if err != nil || step <= 0 {
            return fmt.Errorf("invalid %s step value: %s", fieldName, field)
        }

        // Validate the base part of the step
        if parts[0] != "*" {
            if err := validateCronField(parts[0], fieldName, min, max); err != nil {
                return err
            }
        }
        return nil
    }

    // Handle comma-separated values like "1,2,3"
    if strings.Contains(field, ",") {
        parts := strings.Split(field, ",")
        for _, part := range parts {
            part = strings.TrimSpace(part)
            if part == "" {
                return fmt.Errorf("empty value in %s list: %s", fieldName, field)
            }
            if err := validateCronField(part, fieldName, min, max); err != nil {
                return err
            }
        }
        return nil
    }

    // Single value
    val, err := strconv.Atoi(field)
    if err != nil {
        return fmt.Errorf("invalid %s value: %s", fieldName, field)
    }
    if val < min || val > max {
        return fmt.Errorf("invalid %s value: %d (valid range: %d-%d)", fieldName, val, min, max)
    }

    return nil
}

// validateCronLogic performs logical validation of cron expressions
func validateCronLogic(fields []string) error {
    // Check for day of month vs day of week conflicts
    // If both day and weekday are specified (not *), it might be ambiguous
    dayField := fields[2]
    weekdayField := fields[4]

    if dayField != "*" && weekdayField != "*" {
        // This is a warning case - some cron implementations handle this differently
        // For now, we'll allow it but could add a warning
    }

    // Additional logical checks could be added here:
    // - Leap year considerations for February 29th
    // - Month-specific day validations (e.g., April 31st doesn't exist)
    // - Business day considerations

    return nil
}
