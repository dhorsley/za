local M = {}

-- Za language keywords and patterns
local za_keywords = {
    -- Core statements
    "var",
    "setglob",
    "input",
    "define",
    "def",
    "enddef",
    "end",
    "if",
    "endif",
    "else",
    "while",
    "endwhile",
    "for",
    "endfor",
    "foreach",
    "in",
    "do",
    "to",
    "as",
    "is",
    "and",
    "or",
    "not",
    "return",
    "break",
    "continue",
    "step",
    "pause",
    "debug",
    "async",
    "module",
    "require",
    "import",
    "export",
    "struct",
    "endstruct",
    "enum",
    "case",
    "endcase",
    "with",
    "endwith",
    "try",
    "catch",
    "endtry",
    "throw",

    -- Single character/short keywords
    "ef",
    "ec",
    "ei",
    "ew",
    "es",
    "has",
    "nop",
    "log",
    "cls",
    "web",
    "pane",
    "help",
    "hist",
    "exit",
    "prompt",
    "showdef",
    "version",
    "println",
    "logging",
    "subject",
    "disable",
    "enable",
    "contains",
    "accessfile",
    "showstruct",
    "on",
    "at",
    "print",

    -- Additional keywords from original syntax
    "quiet",
    "setglob",
}

-- Test-related statements (highlighted differently)
local za_test_keywords = {
    "doc",
    "test",
    "endtest",
    "assert",
    "et",
}

local za_types = {
    "int",
    "uint",
    "bool",
    "float",
    "string",
    "map",
    "array",
    "any",
}

local za_functions = {
    -- Hash functions
    "md5sum",
    "sha1sum",
    "sha224sum",
    "sha256sum",
    "s3sum",

    -- Math functions
    "seed",
    "rand",
    "randf",
    "pow",
    "abs",
    "sin",
    "cos",
    "tan",
    "asin",
    "acos",
    "atan",
    "sinh",
    "cosh",
    "tanh",
    "asinh",
    "acosh",
    "atanh",
    "floor",
    "ln",
    "logn",
    "log2",
    "log10",
    "round",
    "rad2deg",
    "deg2rad",
    "e",
    "pi",
    "phi",
    "ln2",
    "ln10",
    "ibase",
    "ubin8",
    "uhex32",
    "numcomma",
    "prec",
    "reshape",
    "zeros",
    "ones",
    "flatten",
    "mean",
    "median",
    "std",
    "variance",
    "identity",
    "trace",
    "argmax",
    "argmin",
    "find",
    "where",
    "stack",
    "concatenate",
    "squeeze",
    "det",
    "det_big",
    "inverse",
    "inverse_big",
    "rank",

    -- System monitoring functions
    "top_cpu",
    "top_mem",
    "top_nio",
    "top_dio",
    "sys_resources",
    "sys_load",
    "mem_info",
    "ps_info",
    "ps_tree",
    "ps_map",
    "cpu_info",
    "nio",
    "dio",
    "resource_usage",
    "iodiff",
    "disk_usage",
    "mount_info",
    "net_devices",

    -- Web functions
    "wpage",
    "wbody",
    "wdiv",
    "wa",
    "wimg",
    "whead",
    "wlink",
    "wp",
    "wtable",
    "wthead",
    "wtbody",
    "wtr",
    "wth",
    "wtd",
    "wh1",
    "wh2",
    "wh3",
    "wh4",
    "wh5",
    "wol",
    "wul",
    "wli",
    "download",
    "web_download",
    "web_head",
    "web_get",
    "web_custom",
    "web_post",
    "web_raw_send",
    "web_serve_start",
    "web_serve_stop",
    "web_serve_up",
    "web_serve_path",
    "web_serve_log",
    "web_serve_log_throttle",
    "web_display",
    "web_serve_decode",
    "web_max_clients",
    "web_cache_enable",
    "web_cache_max_size",
    "web_cache_max_age",
    "web_cache_cleanup_interval",
    "web_cache_max_memory",
    "web_cache_purge",
    "web_cache_stats",
    "web_gzip_enable",
    "net_interfaces",
    "html_escape",
    "html_unescape",
    "tcp_client",
    "tcp_server",
    "tcp_close",
    "tcp_send",
    "tcp_receive",
    "tcp_available",
    "tcp_server_accept",
    "tcp_server_stop",
    "icmp_ping",
    "tcp_ping",
    "traceroute",
    "tcp_traceroute",
    "icmp_traceroute",
    "dns_resolve",
    "port_scan",
    "net_interfaces_detailed",
    "ssl_cert_validate",
    "ssl_cert_install_help",
    "http_headers",
    "http_benchmark",
    "network_stats",
    "netstat",
    "netstat_protocols",
    "netstat_protocol_info",
    "netstat_protocol",
    "netstat_listen",
    "netstat_established",
    "netstat_process",
    "netstat_interface",
    "open_files",

    -- Error handling functions
    "error_message",
    "error_source_location",
    "error_source_context",
    "error_call_chain",
    "error_call_stack",
    "error_local_variables",
    "error_global_variables",
    "error_default_handler",
    "error_extend",
    "error_emergency_exit",
    "error_filename",

    -- Zip functions
    "zip_create",
    "zip_create_from_dir",
    "zip_extract",
    "zip_extract_file",
    "zip_list",
    "zip_add",
    "zip_remove",

    -- Cron functions
    "cron_parse",
    "quartz_to_cron",
    "cron_next",
    "cron_validate",

    -- Package functions
    "install",
    "uninstall",
    "service",
    "vcmp",
    "is_installed",

    -- OS functions
    "env",
    "get_env",
    "set_env",
    "cwd",
    "can_read",
    "can_write",
    "cd",
    "dir",
    "umask",
    "chroot",
    "delete",
    "rename",
    "copy",
    "glob",
    "parent",
    "is_symlink",
    "is_device",
    "is_pipe",
    "is_socket",
    "is_sticky",
    "is_setuid",
    "is_setgid",
    "username",
    "groupname",
    "user_list",
    "group_list",
    "user_add",
    "user_del",
    "group_add",
    "group_del",
    "group_membership",
    "user_info",
    "group_info",

    -- File functions
    "file_mode",
    "file_size",
    "read_file",
    "write_file",
    "is_file",
    "is_dir",
    "perms",
    "stat",
    "fopen",
    "fclose",
    "ftell",
    "fseek",
    "fread",
    "fwrite",
    "feof",
    "fflush",
    "flock",

    -- Event functions
    "ev_watch",
    "ev_watch_close",
    "ev_watch_add",
    "ev_watch_remove",
    "ev_exists",
    "ev_event",
    "ev_mask",

    -- Database functions
    "db_init",
    "db_query",
    "db_close",

    -- String functions
    "pad",
    "field",
    "fields",
    "get_value",
    "has_start",
    "has_end",
    "match",
    "filter",
    "substr",
    "gsub",
    "replace",
    "trim",
    "lines",
    "count",
    "inset",
    "wrap",
    "next_match",
    "line_add",
    "line_delete",
    "line_replace",
    "line_add_before",
    "line_add_after",
    "line_match",
    "line_filter",
    "grep",
    "line_head",
    "line_tail",
    "is_utf8",
    "reverse",
    "tr",
    "lower",
    "upper",
    "format",
    "ccformat",
    "literal",
    "pos",
    "bg256",
    "fg256",
    "bgrgb",
    "fgrgb",
    "split",
    "join",
    "collapse",
    "strpos",
    "stripansi",
    "addansi",
    "stripquotes",
    "stripcc",
    "clean",
    "rvalid",
    "levdist",
    "keys",

    -- Time functions
    "date",
    "epoch_time",
    "epoch_nano_time",
    "time_diff",
    "date_human",
    "time_hours",
    "time_minutes",
    "time_seconds",
    "time_nanos",
    "time_dow",
    "time_dom",
    "time_month",
    "time_year",
    "time_zone",
    "time_zone_offset",
    "format_date",
    "format_time",

    -- Array/List functions
    "col",
    "head",
    "tail",
    "sum",
    "fieldsort",
    "ssort",
    "sort",
    "uniq",
    "append",
    "append_to",
    "insert",
    "remove",
    "push_front",
    "pop",
    "peek",
    "any",
    "all",
    "esplit",
    "min",
    "max",
    "avg",
    "eqlen",
    "empty",
    "list_string",
    "list_float",
    "list_int",
    "list_int64",
    "list_bool",
    "list_bigi",
    "list_bigf",
    "scan_left",
    "zip",
    "list_fill",
    "concat",

    -- Conversion functions
    "byte",
    "as_int",
    "as_int64",
    "as_bigi",
    "as_bigf",
    "as_float",
    "as_bool",
    "as_string",
    "maxuint",
    "char",
    "asc",
    "as_uint",
    "is_number",
    "base64e",
    "base64d",
    "json_decode",
    "json_format",
    "json_query",
    "pp",
    "write_struct",
    "read_struct",
    "btoi",
    "itob",
    "dtoo",
    "otod",
    "s2m",
    "m2s",
    "f2n",
    "to_typed",
    "table",
    "md2ansi",

    -- Internal functions
    "last",
    "last_err",
    "zsh_version",
    "bash_version",
    "bash_versinfo",
    "user",
    "os",
    "home",
    "lang",
    "release_name",
    "release_version",
    "release_id",
    "winterm",
    "hostname",
    "argc",
    "argv",
    "funcs",
    "keypress",
    "tokens",
    "key",
    "clear_line",
    "pid",
    "ppid",
    "system",
    "func_inputs",
    "func_outputs",
    "func_descriptions",
    "func_categories",
    "local",
    "clktck",
    "funcref",
    "thisfunc",
    "thisref",
    "cursoron",
    "cursoroff",
    "cursorx",
    "eval",
    "exec",
    "term_w",
    "term_h",
    "pane_h",
    "pane_w",
    "pane_r",
    "pane_c",
    "utf8supported",
    "execpath",
    "trap",
    "coproc",
    "capture_shell",
    "ansi",
    "interpol",
    "shell_pid",
    "has_shell",
    "has_term",
    "term",
    "has_colour",
    "len",
    "rlen",
    "echo",
    "get_row",
    "get_col",
    "unmap",
    "await",
    "get_mem",
    "zainfo",
    "get_cores",
    "permit",
    "enum_names",
    "enum_all",
    "dump",
    "mdump",
    "sysvar",
    "expect",
    "ast",
    "varbind",
    "sizeof",
    "dup",
    "log_queue_status",
    "logging_stats",
    "exreg",
    "format_stack_trace",
    "panic",
    "array_format",
    "array_colours",

    -- TUI functions
    "tui_new",
    "tui_new_style",
    "tui",
    "tui_box",
    "tui_screen",
    "tui_text",
    "tui_pager",
    "tui_menu",
    "selector",
    "tui_progress",
    "tui_progress_reset",
    "tui_input",
    "tui_clear",
    "tui_template",
    "tui_table",
    "tui_radio",
    "editor",

    -- SVG functions
    "svg_start",
    "svg_end",
    "svg_title",
    "svg_desc",
    "svg_plot",
    "svg_circle",
    "svg_ellipse",
    "svg_rect",
    "svg_roundrect",
    "svg_square",
    "svg_line",
    "svg_polyline",
    "svg_polygon",
    "svg_text",
    "svg_image",
    "svg_grid",
    "svg_def",
    "svg_def_end",
    "svg_group",
    "svg_group_end",
    "svg_link",
    "svg_link_end",

    -- YAML functions
    "yaml_parse",
    "yaml_marshal",
    "yaml_get",
    "yaml_set",
    "yaml_delete",

    -- Email functions
    "smtp_send",
    "smtp_send_with_auth",
    "smtp_send_with_attachments",
    "email_parse_headers",
    "email_get_body",
    "email_get_attachments",
    "email_validate",
    "email_extract_addresses",
    "email_process_template",
    "email_add_header",
    "email_remove_header",
    "email_base64_encode",
    "email_base64_decode",
}

-- Define highlight groups using nvim_set_hl
local function define_highlights()
    -- Basic syntax groups
    vim.api.nvim_set_hl(0, "zaKeyword", {
        fg = "#4169e1", -- Slightly brighter blue
        bold = true,
    })

    vim.api.nvim_set_hl(0, "zaType", {
        fg = "#ff00ff",
    })

    vim.api.nvim_set_hl(0, "zaFunction", {
        fg = "#00ff00",
    })

    vim.api.nvim_set_hl(0, "zaUserFunction", {
        fg = "#40e0d0", -- Turquoise
    })

    vim.api.nvim_set_hl(0, "zaNamespace", {
        fg = "#008080", -- Teal
    })

    vim.api.nvim_set_hl(0, "zaComment", {
        fg = "#6495ed", -- Cornflower blue
        italic = true,
    })

    vim.api.nvim_set_hl(0, "zaTestStatement", {
        fg = "#8b0000", -- Dark red
        italic = true,
    })

    vim.api.nvim_set_hl(0, "zaString", {
        fg = "#daa520", -- Dull gold/mid yellow
    })

    vim.api.nvim_set_hl(0, "zaNumber", {
        fg = "#add8e6",
    })

    vim.api.nvim_set_hl(0, "zaOperator", {
        fg = "#ffff00",
    })

    -- Special za color codes
    vim.api.nvim_set_hl(0, "zaColourB0", { bg = "#000000", fg = "#ffffff" })
    vim.api.nvim_set_hl(0, "zaColourB1", { bg = "#0000ff", fg = "#ffffff" })
    vim.api.nvim_set_hl(0, "zaColourB2", { bg = "#ff0000", fg = "#ffffff" })
    vim.api.nvim_set_hl(0, "zaColourB3", { bg = "#ff00ff", fg = "#ffffff" })
    vim.api.nvim_set_hl(0, "zaColourB4", { bg = "#00ff00", fg = "#ffffff" })
    vim.api.nvim_set_hl(0, "zaColourB5", { bg = "#00ffff", fg = "#000000" })
    vim.api.nvim_set_hl(0, "zaColourB6", { bg = "#ffff00", fg = "#000000" })
    vim.api.nvim_set_hl(0, "zaColourB7", { bg = "#808080", fg = "#000000" })

    vim.api.nvim_set_hl(0, "zaColourF0", { fg = "#a9a9a9" })
    vim.api.nvim_set_hl(0, "zaColourF1", { fg = "#0000ff" })
    vim.api.nvim_set_hl(0, "zaColourF2", { fg = "#ff0000" })
    vim.api.nvim_set_hl(0, "zaColourF3", { fg = "#ff00ff" })
    vim.api.nvim_set_hl(0, "zaColourF4", { fg = "#00ff00" })
    vim.api.nvim_set_hl(0, "zaColourF5", { fg = "#00ffff" })
    vim.api.nvim_set_hl(0, "zaColourF6", { fg = "#ffff00" })
    vim.api.nvim_set_hl(0, "zaColourF7", { fg = "#ffffff" })
end

--[[

-- Check if position is in a string
local function in_string(line, pos)
  local before = line:sub(1, pos)
  local quotes = 0
  for i = 1, #before do
    local char = before:sub(i, i)
    if char == '"' then
      quotes = quotes + 1
    elseif char == '\\' and i > 1 and before:sub(i-1, i-1) ~= '\\' then
      quotes = quotes - 1
    end
  end
  return quotes % 2 == 1
end

-- Check if position is in a comment
local function in_comment(line, pos)
  local before = line:sub(1, pos)
  local comment_pos = before:find("#", 1, true)
  local comment_pos2 = before:find("//", 1, true)
  local actual_comment_pos = nil
  if comment_pos and comment_pos2 then
    actual_comment_pos = math.min(comment_pos, comment_pos2)
  elseif comment_pos then
    actual_comment_pos = comment_pos
  elseif comment_pos2 then
    actual_comment_pos = comment_pos2
  end

  if actual_comment_pos then
    -- Check if comment is inside a string
    local before_comment = line:sub(1, actual_comment_pos - 1)
    local quotes = 0
    for i = 1, #before_comment do
      local char = before_comment:sub(i, i)
      if char == '"' then
        quotes = quotes + 1
      end
    end
    return quotes % 2 == 0  -- Comment is not in string if even number of quotes before it
  end

  return false
end

--]]

-- Manual syntax highlighting using nvim_buf_add_highlight
local function highlight_line(buf, line_num, line)
    local highlights = {}
    local original_line = line

    -- Find all string ranges first (both double-quoted and backtick)
    local string_ranges = {}
    local is_in_string = false
    local string_start = 0
    local string_delim = ""
    local i = 1
    while i <= #line do
        local char = line:sub(i, i)
        if (char == '"' or char == "`") and (i == 1 or line:sub(i - 1, i - 1) ~= "\\") then
            if not is_in_string then
                is_in_string = true
                string_start = i
                string_delim = char
            elseif char == string_delim then
                is_in_string = false
                table.insert(string_ranges, { string_start, i })
                string_delim = ""
            end
        end
        i = i + 1
    end

    -- Handle unclosed string at end of line
    if is_in_string then
        table.insert(string_ranges, { string_start, #line })
    end

    -- Highlight strings
    for _, range in ipairs(string_ranges) do
        vim.api.nvim_buf_add_highlight(buf, -1, "zaString", line_num - 1, range[1] - 1, range[2])
    end

    -- Find comment ranges
    local comment_start = line:find("#", 1, true)
    local comment_start2 = line:find("//", 1, true)
    local comment_pos = nil
    if comment_start and (not comment_start2 or comment_start < comment_start2) then
        comment_pos = comment_start
    elseif comment_start2 then
        comment_pos = comment_start2
    end

    if comment_pos then
        vim.api.nvim_buf_add_highlight(buf, -1, "zaComment", line_num - 1, comment_pos - 1, #line)
    end

    -- Helper function to check if position is in string
    local function is_pos_in_string(pos)
        for _, range in ipairs(string_ranges) do
            if pos >= range[1] and pos <= range[2] then
                return true
            end
        end
        return false
    end

    -- Helper function to check if position is in comment
    local function is_pos_in_comment(pos)
        return comment_pos and pos >= comment_pos
    end

    -- Highlight keywords
    for _, keyword in ipairs(za_keywords) do
        local start_pos = line:find(keyword, 1, true)
        while start_pos do
            local end_pos = start_pos + #keyword
            local prev_char = start_pos > 1 and line:sub(start_pos - 1, start_pos - 1) or " "
            local next_char = line:sub(end_pos, end_pos)
            if
                (not prev_char:match("[%w_]") or prev_char == "") and (not next_char:match("[%w_]") or next_char == "")
            then
                if not is_pos_in_string(start_pos) and not is_pos_in_comment(start_pos) then
                    vim.api.nvim_buf_add_highlight(buf, -1, "zaKeyword", line_num - 1, start_pos - 1, end_pos - 1)
                end
            end
            start_pos = line:find(keyword, end_pos, true)
        end
    end

    -- Highlight test keywords
    for _, test_keyword in ipairs(za_test_keywords) do
        local start_pos = line:find(test_keyword, 1, true)
        while start_pos do
            local end_pos = start_pos + #test_keyword
            local prev_char = start_pos > 1 and line:sub(start_pos - 1, start_pos - 1) or " "
            local next_char = line:sub(end_pos, end_pos)
            if
                (not prev_char:match("[%w_]") or prev_char == "") and (not next_char:match("[%w_]") or next_char == "")
            then
                if not is_pos_in_string(start_pos) and not is_pos_in_comment(start_pos) then
                    vim.api.nvim_buf_add_highlight(buf, -1, "zaTestStatement", line_num - 1, start_pos - 1, end_pos - 1)
                end
            end
            start_pos = line:find(test_keyword, end_pos, true)
        end
    end

    -- Highlight types
    for _, type_name in ipairs(za_types) do
        local start_pos = line:find(type_name, 1, true)
        while start_pos do
            local end_pos = start_pos + #type_name
            local prev_char = start_pos > 1 and line:sub(start_pos - 1, start_pos - 1) or " "
            local next_char = line:sub(end_pos, end_pos)
            if
                (not prev_char:match("[%w_]") or prev_char == "") and (not next_char:match("[%w_]") or next_char == "")
            then
                if not is_pos_in_string(start_pos) and not is_pos_in_comment(start_pos) then
                    vim.api.nvim_buf_add_highlight(buf, -1, "zaType", line_num - 1, start_pos - 1, end_pos - 1)
                end
            end
            start_pos = line:find(type_name, end_pos, true)
        end
    end

    -- Highlight built-in functions
    for _, func in ipairs(za_functions) do
        local func_start = line:find(func, 1, true)
        while func_start do
            local func_end = func_start + #func
            local prev_char = func_start > 1 and line:sub(func_start - 1, func_start - 1) or " "
            local next_char = line:sub(func_end, func_end)
            -- Ensure word boundaries: not preceded/followed by word characters
            if
                next_char == "("
                and (not prev_char:match("[%w_]") or prev_char == "")
                and not is_pos_in_string(func_start)
                and not is_pos_in_comment(func_start)
            then
                vim.api.nvim_buf_add_highlight(buf, -1, "zaFunction", line_num - 1, func_start - 1, func_end - 1)
            end
            func_start = line:find(func, func_end, true)
        end
    end

    -- Highlight user-defined function calls (any word followed by parentheses that's not a built-in)
    for word in line:gmatch("[%a_][%w_]*%s*%(") do
        local func_name = word:match("[%a_][%w_]*")
        local func_start = line:find(func_name, 1, true)
        while func_start do
            local func_end = func_start + #func_name
            local next_char = line:sub(func_end, func_end)
            if next_char == "(" then
                -- Check if it's not a built-in function
                local is_builtin = false
                for _, builtin in ipairs(za_functions) do
                    if func_name == builtin then
                        is_builtin = true
                        break
                    end
                end
                -- Check if it's not a keyword
                local is_keyword = false
                for _, keyword in ipairs(za_keywords) do
                    if func_name == keyword then
                        is_keyword = true
                        break
                    end
                end
                if
                    not is_builtin
                    and not is_keyword
                    and not is_pos_in_string(func_start)
                    and not is_pos_in_comment(func_start)
                then
                    vim.api.nvim_buf_add_highlight(
                        buf,
                        -1,
                        "zaUserFunction",
                        line_num - 1,
                        func_start - 1,
                        func_end - 1
                    )
                end
            end
            func_start = line:find(func_name, func_end, true)
        end
    end

    -- Highlight function definitions (after 'def' or 'function' keywords)
    for _, def_keyword in ipairs({ "def", "function" }) do
        local def_start = line:find(def_keyword, 1, true)
        while def_start do
            local def_end = def_start + #def_keyword
            local prev_char = def_start > 1 and line:sub(def_start - 1, def_start - 1) or " "
            local next_char = line:sub(def_end, def_end)
            if (not prev_char:match("[%w_]") or prev_char == "") and (next_char:match("%s") or next_char == "") then
                -- Find the function name after the keyword
                local after_keyword = line:sub(def_end + 1)
                local func_name, func_name_end = after_keyword:match("^%s*([%a_][%w_]*)()")
                if func_name then
                    local actual_start = def_end + func_name_end - #func_name - 1
                    local actual_end = def_end + func_name_end - 1
                    if not is_pos_in_string(actual_start) and not is_pos_in_comment(actual_start) then
                        vim.api.nvim_buf_add_highlight(
                            buf,
                            -1,
                            "zaUserFunction",
                            line_num - 1,
                            actual_start - 1,
                            actual_end - 1
                        )
                    end
                end
            end
            def_start = line:find(def_keyword, def_end, true)
        end
    end

    -- Highlight namespaces (identifiers to the left of ::)
    for namespace in line:gmatch("[%a_][%w_]*%s*::") do
        local namespace_name = namespace:match("[%a_][%w_]*")
        local ns_start = line:find(namespace, 1, true)
        while ns_start do
            local ns_end = ns_start + #namespace_name - 1
            if not is_pos_in_string(ns_start) and not is_pos_in_comment(ns_start) then
                vim.api.nvim_buf_add_highlight(buf, -1, "zaNamespace", line_num - 1, ns_start - 1, ns_end)
            end
            ns_start = line:find(namespace, ns_end + 1, true)
        end
    end

    -- Highlight numbers
    for word in line:gmatch("[%d]+%.?[%d]*") do
        local start_pos = line:find(word, 1, true)
        while start_pos do
            local end_pos = start_pos + #word
            if not is_pos_in_string(start_pos) and not is_pos_in_comment(start_pos) then
                vim.api.nvim_buf_add_highlight(buf, -1, "zaNumber", line_num - 1, start_pos - 1, end_pos - 1)
            end
            start_pos = line:find(word, end_pos, true)
        end
    end

    -- Highlight operators
    local operators = { "<<", ">>", "!", "&", ";", "|", "[", "]" }
    for _, op in ipairs(operators) do
        local start_pos = line:find(op, 1, true)
        while start_pos do
            if not is_pos_in_string(start_pos) and not is_pos_in_comment(start_pos) then
                vim.api.nvim_buf_add_highlight(buf, -1, "zaOperator", line_num - 1, start_pos - 1, start_pos + #op - 1)
            end
            start_pos = line:find(op, start_pos + 1, true)
        end
    end

    -- Highlight UFCS (Unified Function Call Syntax) dotted calls
    for dot_pos in line:gmatch("%.()") do
        if dot_pos > 1 and dot_pos < #line then
            local char_before = line:sub(dot_pos - 1, dot_pos - 1)
            local char_after = line:sub(dot_pos + 1, dot_pos + 1)
            if char_before:match("[%w_]") and char_after:match("[%w_]") then
                if not is_pos_in_string(dot_pos) and not is_pos_in_comment(dot_pos) then
                    vim.api.nvim_buf_add_highlight(buf, -1, "zaOperator", line_num - 1, dot_pos - 1, dot_pos)
                end
            end
        end
    end

    return highlights
end

-- Debounced highlighting function
local highlight_timers = {}
local function debounced_highlight(buf)
    local bufnr = vim.api.nvim_buf_get_number(buf)
    if highlight_timers[bufnr] then
        vim.fn.timer_stop(highlight_timers[bufnr])
    end

    highlight_timers[bufnr] = vim.fn.timer_start(50, function()
        highlight_visible_lines(buf)
    end)
end

-- Setup syntax highlighting for za files
function M.setup()
    -- Define highlight groups
    define_highlights()

    -- Create autocmd for syntax highlighting
    vim.api.nvim_create_autocmd("FileType", {
        pattern = "za",
        callback = function()
            local buf = vim.api.nvim_get_current_buf()

            -- Highlight only visible lines for performance
            local function highlight_visible_lines()
                -- Clear existing highlights
                vim.api.nvim_buf_clear_namespace(buf, -1, 0, -1)

                -- Get only visible lines
                local start_line = vim.fn.line("w0") - 1 -- Convert to 0-based
                local end_line = vim.fn.line("w$") - 1
                local visible_lines = vim.api.nvim_buf_get_lines(buf, start_line, end_line, false)

                -- Highlight visible lines
                for i, line in ipairs(visible_lines) do
                    local line_num = start_line + i
                    highlight_line(buf, line_num, line)
                end
            end

            -- Debounced highlighting function
            local highlight_timers = {}
            local function debounced_highlight()
                local bufnr = vim.api.nvim_buf_get_number(buf)
                if highlight_timers[bufnr] then
                    vim.fn.timer_stop(highlight_timers[bufnr])
                end

                highlight_timers[bufnr] = vim.fn.timer_start(50, highlight_visible_lines)
            end

            -- Initial highlight
            highlight_visible_lines()

            -- Set up debounced updates for various events
            local events = {
                "TextChanged",
                "TextChangedI",
                "WinScrolled",
                "CursorMoved",
            }

            vim.api.nvim_create_autocmd(events, {
                buffer = buf,
                callback = debounced_highlight,
                desc = "Update za syntax highlighting on changes",
            })
        end,
        desc = "Setup za syntax highlighting",
    })
end

return M
