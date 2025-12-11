local filetype = require("za.filetype")
local syntax = require("za.syntax")

local M = {}

-- Setup za syntax highlighting and filetype detection
function M.setup()
  -- Setup filetype detection first
  filetype.setup()
  
  -- Setup syntax highlighting
  syntax.setup()
end

return M