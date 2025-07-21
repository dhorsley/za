# Installing the ZA Language Extension

## Method 1: Manual Installation (Development)

1. **Copy the extension folder**:
   ```bash
   # On Windows
   copy "docs\vscode-za" "%USERPROFILE%\.vscode\extensions\za-language"
   
   # On macOS/Linux
   cp -r docs/vscode-za ~/.vscode/extensions/za-language
   ```

2. **Restart VS Code**

3. **Test the extension**:
   - Open a `.za` file
   - You should see syntax highlighting

## Method 2: VSIX Package (Recommended)

1. **Install vsce** (VS Code Extension Manager):
   ```bash
   npm install -g vsce
   ```

2. **Package the extension**:
   ```bash
   cd docs/vscode-za
   vsce package
   ```

3. **Install the VSIX**:
   - In VS Code: `Ctrl+Shift+P` → "Extensions: Install from VSIX..."
   - Select the generated `.vsix` file

## Method 3: Development Mode

1. **Clone the repository** (if not already done)

2. **Open the extension folder**:
   ```bash
   code docs/vscode-za
   ```

3. **Press F5** to launch a new VS Code window with the extension

4. **Test with sample.za**:
   - Open `sample.za` in the new window
   - Verify syntax highlighting works

## Verification

After installation, you should see:

- **Syntax highlighting** for ZA keywords, functions, strings, comments
- **Auto-completion** for brackets and quotes
- **Smart indentation** for code blocks
- **File association** - `.za` files open with ZA language mode

## Troubleshooting

1. **No syntax highlighting**:
   - Check that the extension is enabled in VS Code
   - Restart VS Code
   - Verify the file has `.za` extension

2. **Extension not loading**:
   - Check the VS Code Developer Console for errors
   - Verify all files are in the correct locations

3. **Missing features**:
   - Ensure you're using VS Code 1.60.0 or later
   - Check the extension's README for feature list

## Uninstalling

1. **Manual installation**: Delete the extension folder
2. **VSIX installation**: Use VS Code's extension manager to uninstall
3. **Development mode**: Close the development window 