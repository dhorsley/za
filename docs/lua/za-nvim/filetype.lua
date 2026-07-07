local M = {}

-- Function to detect if a file contains a za shebang
local function is_za_file(first_line)
    if not first_line then
        return false
    end

    return first_line:match("^#!%s*/usr/bin/za")
        or first_line:match("^#!%s*/usr/bin/env%s+za")
        or first_line:match("^#!%s*/usr/local/bin/za")
end

-- Function to detect if a filename has a .za extension
local function is_za_extension(filename)
    if not filename then
        return false
    end
    return filename:match("%.za$") ~= nil
end

-- Setup filetype detection for za files
function M.setup()
    vim.api.nvim_create_autocmd({ "BufRead", "BufNewFile" }, {
        pattern = "*",
        callback = function(args)
            -- Skip if filetype is already set to za
            if vim.bo[args.buf].filetype == "za" then
                return
            end

            -- Check filename extension
            local filename = vim.api.nvim_buf_get_name(args.buf)
            if is_za_extension(filename) then
                vim.bo[args.buf].filetype = "za"
                return
            end

            -- Get first line of the file
            local lines = vim.api.nvim_buf_get_lines(args.buf, 0, 1, false)
            local first_line = lines[1]

            -- Check if it's a za file by shebang
            if is_za_file(first_line) then
                vim.bo[args.buf].filetype = "za"
            end
        end,
        desc = "Detect za files by extension or shebang line",
    })
end

return M
