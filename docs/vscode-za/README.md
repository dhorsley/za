# ZA Language Extension for VS Code

This extension provides syntax highlighting for the ZA programming language in Visual Studio Code.

## Features

- **Syntax Highlighting**: Comprehensive syntax highlighting for ZA language constructs
- **Language Support**: Full support for `.za` files
- **Auto-closing**: Automatic bracket and quote pairing
- **Indentation**: Smart indentation rules for ZA code blocks

## Syntax Highlighting

The extension provides highlighting for:

### Keywords
- Control flow: `if`, `else`, `while`, `for`, `foreach`, `case`, `switch`, etc.
- Function definitions: `def`, `define`, `enddef`
- Data structures: `struct`, `enum`, `map`, `array`
- Modules: `module`, `require`
- Testing: `test`, `assert`, `doc`
- Exception handling: `try`, `catch`, `endtry`, `throw`, `then`, `throws`
- Operators: `??` (safe operator)

### Functions
- **Time functions**: `date()`, `epoch_time()`, `time_diff()`, etc.
- **List functions**: `empty()`, `head()`, `tail()`, `sort()`, `append()`, etc.
- **String functions**: `len()`, `split()`, `join()`, `grep()`, `replace()`, etc.
- **Math functions**: `sin()`, `cos()`, `pow()`, `abs()`, `round()`, etc.
- **File functions**: `read_file()`, `write_file()`, `is_file()`, `dir()`, etc.
- **Web functions**: `download()`, `web_get()`, `web_post()`, etc.
- **OS functions**: `env()`, `cd()`, `cwd()`, `system()`, etc.
- **TUI functions**: `tui()`, `editor()`, `tui_menu()`, etc.
- **Database functions**: `db_init()`, `db_query()`, `db_close()`
- **Conversion functions**: `as_int()`, `as_string()`, `as_float()`, etc.
- **Internal functions**: `ast()`, `eval()`, `exec()`, `dump()`, etc.
- **Exception functions**: `exception_strictness()`, `log_exception()`, `exreg()`, etc.

### Types
- `int`, `uint`, `bool`, `float`, `string`, `map`, `array`, `any`

### Comments
- Line comments with `#`

### Strings
- Double-quoted strings: `"hello world"`
- Backtick strings: `` `command output` ``

### Color Codes
- Background colors: `[#b0]` through `[#b7]` or `[#bblack]`, `[#bblue]`, etc.
- Foreground colors: `[#0]` through `[#7]` or `[#fblack]`, `[#fblue]`, etc.
- Normal color: `[##]` or `[#-]`

### Numbers
- Integers: `123`, `-456`, `+789`
- Floats: `3.14`, `2.718e-10`, `1.0f`

## Installation

1. Copy the `vscode-za` folder to your VS Code extensions directory
2. Restart VS Code
3. Open a `.za` file to see syntax highlighting

## Language Configuration

The extension includes:
- Auto-closing brackets and quotes
- Smart indentation rules
- Comment support
- Bracket matching

## Contributing

To improve the syntax highlighting:
1. Edit `syntaxes/za.tmLanguage.json`
2. Test with sample ZA code
3. Submit improvements

## License

This extension is part of the ZA language project. 