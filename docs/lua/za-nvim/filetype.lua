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

-- Setup filetype detection for za files
function M.setup()
    vim.api.nvim_create_autocmd({ "BufRead", "BufNewFile" }, {
        pattern = "*",
        callback = function(args)
            -- Skip if filetype is already set to za
            if vim.bo[args.buf].filetype == "za" then
                return
            end

            -- Get first line of the file
            local lines = vim.api.nvim_buf_get_lines(args.buf, 0, 1, false)
            local first_line = lines[1]

            -- Check if it's a za file
            if is_za_file(first_line) then
                vim.bo[args.buf].filetype = "za"
            end
        end,
        desc = "Detect za files by shebang line",
    })
end

return M
