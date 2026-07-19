#!/usr/bin/env bash
set -euo pipefail

# ZA Language Server + VS Code: Extension Installer
# Run from the za repository root or from docs/lsp/

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

VSCODE_ZA_DIR="${REPO_ROOT}/docs/vscode-za"
LSP_DIR="${REPO_ROOT}/docs/lsp"

echo "=== ZA Language Server Installer ==="
echo ""

# --- Check prerequisites ---

echo "[1/5] Checking prerequisites..."

missing=()

if ! command -v go &>/dev/null; then
    missing+=("go (https://go.dev/dl/)")
fi

if ! command -v node &>/dev/null; then
    missing+=("node (https://nodejs.org/)")
fi

if ! command -v npm &>/dev/null; then
    missing+=("npm (usually bundled with node)")
fi

if ! command -v code &>/dev/null; then
    missing+=("VS Code: CLI (https://code.visualstudio.com/)")
fi

if [ ${#missing[@]} -ne 0 ]; then
    echo "ERROR: The following tools are missing:"
    for tool in "${missing[@]}"; do
        echo "  - $tool"
    done
    echo ""
    echo "Please install them and re-run this script."
    exit 1
fi

# --- Check za binary ---

echo "[2/5] Checking za binary..."

ZA_PATH=""
if command -v za &>/dev/null; then
    ZA_PATH="$(command -v za)"
    echo "  Found za at: ${ZA_PATH}"
else
    # Try common locations
    candidates=(
        "${REPO_ROOT}/za"
        "/usr/local/bin/za"
        "/usr/bin/za"
        "${HOME}/go/bin/za"
    )
    for candidate in "${candidates[@]}"; do
        if [ -x "${candidate}" ]; then
            ZA_PATH="${candidate}"
            echo "  Found za at: ${ZA_PATH}"
            break
        fi
    done
fi

if [ -z "${ZA_PATH}" ]; then
    echo "ERROR: za binary not found in PATH or common locations."
    echo ""
    echo "The LSP server requires the za interpreter to load stdlib metadata."
    echo "Please build and install za first:"
    echo "  cd ${REPO_ROOT}"
    echo "  go build -o za ."
    echo "  sudo cp za /usr/local/bin/"
    echo ""
    exit 1
fi

# --- Build LSP server ---

echo "[3/5] Building za-lsp server..."
cd "${REPO_ROOT}"
go build -o "${LSP_DIR}/za-lsp" ./docs/lsp
echo "  Built: ${LSP_DIR}/za-lsp"

# --- Build VS Code: extension ---

echo "[4/5] Building VS Code: extension..."
cd "${VSCODE_ZA_DIR}"

# Ensure za-lsp binary is in the extension's bin directory
mkdir -p bin
cp "${LSP_DIR}/za-lsp" bin/za-lsp

# Install deps and build
if [ ! -d "node_modules" ]; then
    echo "  Installing npm dependencies..."
    npm install
fi

npm run compile

# Package into .vsix
echo "  Packaging extension..."
npx vsce package 2>&1 | tail -1

# Find the generated .vsix
VSIX=$(ls -t za-language-*.vsix 2>/dev/null | head -1)
if [ -z "${VSIX}" ]; then
    echo "ERROR: Failed to find generated .vsix file"
    exit 1
fi
echo "  Packaged: ${VSIX}"

# --- Install extension ---

echo "[5/5] Installing extension to VS Code:..."
code --install-extension "${VSIX}" --force 2>&1 | head -3
echo ""

# --- Summary ---

echo "=== Installation Complete ==="
echo ""
echo "The ZA Language extension is now installed in VS Code:."
echo ""
echo "To use it:"
echo "  1. Open a .za file (or a file with #!/usr/bin/env za shebang)"
echo "  2. VS Code: will automatically activate the extension"
echo "  3. Features available: syntax highlighting, completion, hover,"
echo "     go-to-definition, document outline, signature help, diagnostics"
echo ""

# --- Neovim instructions ---

echo "=== Neovim Setup ==="
echo ""
echo "The LSP server is editor-agnostic. To use it with Neovim:"
echo ""
echo "  1. Install nvim-lspconfig (or your preferred LSP client)"
echo "  2. Add this to your init.lua or init.vim:"
echo ""
cat << 'NVIM_CONFIG'
    local lspconfig = require('lspconfig')
    local configs = require('lspconfig.configs')

    -- Define the ZA LSP server
    if not configs.za then
      configs.za = {
        default_config = {
          cmd = {'za-lsp', '/usr/local/bin/za'},
          filetypes = {'za'},
          root_dir = lspconfig.util.find_git_ancestor,
          single_file_support = true,
          settings = {},
        },
      }
    end

    lspconfig.za.setup{}
NVIM_CONFIG

echo ""
echo "  3. Ensure 'za-lsp' is in your PATH, or use the full path:"
echo "       cmd = {'${LSP_DIR}/za-lsp', '${ZA_PATH}'},"
echo ""
echo "  4. Optional: add za filetype detection to Neovim:"
cat << 'NVIM_FT'
    vim.filetype.add({
      extension = {
        za = 'za',
      },
      pattern = {
        ['.*'] = {
          function(path, bufnr)
            local content = vim.api.nvim_buf_get_lines(bufnr, 0, 1, false)[1] or ''
            if content:match('^#!.*\\bza\\b') then
              return 'za'
            end
          end,
        },
      },
    })
NVIM_FT

echo ""
echo "For other editors (Helix, Zed, Emacs, etc.), configure the LSP client"
echo "to run: za-lsp <path-to-za-binary>"
echo ""
echo "The server communicates over stdio and supports the standard LSP protocol."
echo ""
