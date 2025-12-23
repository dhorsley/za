---
title: "Za — The Za Programming Language Handbook"
author: "Daniel Horsley"
version: "1.2.1"
css: za.css
---

<div style="text-align:center; page-break-after: always;">

<br/>
<br/>
![ ](title-page.png)

#### A Practical Handbook for System Administrators

Daniel Horsley
<br/>

</div>


---

# Contents

## Preface
- About this book
- Who this book is for
- Conventions used
- Version coverage

## Part I - Getting Oriented
1. What Za is (and is not)
2. Running Za
3. The REPL

## Part II - Language Fundamentals
4. Lexical structure
5. Literals and constants
6. Variables and assignment
7. Typed declarations with `var`

## Part III - Data Types and Structures
8. Scalar types
9. Arrays and multi-dimensional data
10. Maps
11. Maps as sets
12. Structs and anonymous structs
13. Struct-associated functions and `self`
14. Enums

## Part IV - Expressions and Control Flow
15. Expressions and operator precedence
16. Operators (arithmetic, boolean, bitwise, set, path, string)
17. Conditionals
18. Loops
19. Case statements

## Part V - Functional Data Processing
20. Expression strings, `#`, and `$idx`
21. Map and filter operators (`->`, `?>`)
22. Searching arrays (`find`, `where`)

## Part VI - Functions, Modules, and Composition
23. Functions (`def … end`)
24. Modules and namespaces

## Part VII - Errors, Debugging, and Safety
25. Error handling philosophy
26. Exceptions (`try … catch … then … endtry`)
27. Enhanced error handling (`trap`, `error_*`)
28. Debugger (how to use it)
29. Profiling (how to interpret output)
30. Security controls (`permit`)

## Part VIII - Concurrency
31. Async execution (`async`, `await`)

## Part IX - Output and Presentation
32. Program output (`print`, `println`)
33. Inspection (`pp`) and tables (`table`)
34. Array display controls (`array_format`, `array_colours`)
35. ANSI colour/style macros

## Part X - Standard Library Overview
36. Library categories and discovery (`help`, `func`)
37. Category tour (representative calls and idioms)
38. Category samples

## Part XI - Sysadmin Cookbook
39. CLI data ingestion
40. Disk and filesystem checks
41. Process and service inspection
42. Network diagnostics
43. Parallel host probing
44. Drift detection and set-based reasoning

## Part XII - Logging
45. Overview
46. Configuration
47. Architecture

## Part XIII - Testing
48. Overview
49. Test blocks
50. Behaviours

## Appendices

A. Operator reference (summary)

B. Keywords (summary)

C. Built-in constants

D. Standard library reference

E. Worked example script


<div style="page-break-after: always;"></div>

# Preface

## About this book

Za is an interpreted language aimed primarily at:

- system administration
- diagnostics and inspection
- testing and harnessing tools
- one-off operational automation

Za is stable enough to run in production, but it is not designed as a framework language or a long-running service runtime. Its strengths are readability and portability.

## Who this book is for

The intended readership is **Linux system administrators**, from novice to expert. Existing programmers should also be able to use this book as a reliable syntax and behaviour reference.

## Version coverage

This text is written for **Za 1.2.1**.

---

# Part I — Getting Oriented

## 1. What Za is (and is not)

Za is a scripting language for people who need to maintain and monitor systems.
It can be used for glue scripts, test rigs, ad-hoc reporting, monitoring and many
small tasks that are in the scope of system administrators, SREs, developers
and others who are regularly expected to probe issues and generate state information.

It prioritises:

- **ease of maintenance**
- **rapid prototyping**
- **portability**

Za does not attempt to replace a general-purpose language for large services and
should be used cautiously in production environments.

## 2. Running Za

Za runs scripts and also provides interactive tooling.

Za is available for Linux-, BSD- and Windows-variants. The same features are available
across all platforms where allowed by the OS.

The interpreter also has a REPL with interactive help features.

## 3. The REPL

The REPL is a workflow tool designed for prototyping, data exploration and system inspection.

You can use it as a shell, if desired, but this is not the intent of this interactive mode.

### 3.1 REPL Overview

The REPL provides several key capabilities that make it particularly effective for system administration tasks:

- **State persistence**: Variables and functions defined in one command remain available in subsequent commands
- **Inspectability**: You can examine any variable or expression at any time using built-in inspection tools
- **Integrated help**: Both general help and targeted library search are available without leaving the REPL
- **Macro support**: Create reusable command shortcuts for common operations
- **Startup configuration**: Customize your environment with automatic script execution
- **Shell integration**: Direct access to system commands while maintaining Za's data structures

### 3.2 Getting Started

Starting the REPL is simple:

```bash
$ za
```

Upon startup, you'll see the default prompt:

```za
>>
```

#### Customizing Your Environment

A useful REPL feature is the startup script. Create a file at `~/.zarc` to automatically configure your session. Here's a simplified one:

```za

# Set a basic prompt
prompt=">> "

# Define helpful macros
macro +ll `ls -la`
macro +df `df -h`

# Enable command fallback - this allows execution of shell commands
_=permit("cmdfallback",true)
```

The full example startup script in the Za repository demonstrates advanced features including:

- Custom prompts with git status and system information
- Environment-specific AWS profile handling
- Coloured system banners
- Complex macros for file listing and system monitoring

#### Display welcome message

```za
println "Za REPL ready. Type 'help' for assistance."
```

Now when you start `za`, your environment is automatically configured with your preferred prompt, modules, and helper macros.

### 3.3 Discovery and Help

Za provides built-in help and discovery mechanisms. Use the command-line help to see available options:

```bash
$ za -h
```

For discovering available functions and modules, examine the standard library source files and examples in the `eg/` directory. The language documentation and examples provide comprehensive coverage of available capabilities.

### 3.4 Interactive Features

#### Command History

The REPL maintains a persistent command history across sessions:

```za
# History is automatically saved to ~/.za_history
# Navigate with up/down arrows
# Search history with ctrl-r (reverse search)
>> ctrl-r
(search): `ls`:
# Type to search, use arrows to navigate results
```

#### Line Editing

Interactive mode supports UTF-8 characters. Navigation keystrokes should be familiar from similar systems:

Standard arrow navigation works as expected

```za
ctrl-a  # Beginning of line
ctrl-e  # End of line
ctrl-u  # Delete to beginning
ctrl-k  # Delete to end
ctrl-c  # interrupt (interrupt session)
ctrl-d  # end-of-input (end session)
ctrl-z  # suspend REPL to background
```

### 3.5 REPL Macros

Macros provide powerful command shortcuts, particularly useful for repetitive system administration tasks.

#### Basic Macro Definition

```za
# Simple text replacement
macro +ps `ps aux | grep -v grep`

# Use in commands
>> #ps
# Expands to: ps aux | grep -v grep
```

#### Parameterized Macros

```za
# Macro with parameters
macro +ls(path) `ls -la {path}`

# Usage
>> #ls("/etc")
>> #ls("~")  # Home directory expansion works
```

#### Varargs and Advanced Features

```za
# Variable arguments with ...
macro +addall(base, ...) `$base + $1 + $2 + $3`

# Nested macro expansion
macro +complex `#addall(10, #ls("."), "extra")`

# Debug macro expansion - Shows expanded code before execution
>> #macro_name!
```

#### Macro Management

```za
# List all macros
macro

# Remove specific macro
macro -ls

# Remove all macros
macro -

# Verbose operations (when logging enabled, shows confirmation messages)
macro ! +test `echo "debug"`
# Output: Macro 'test' defined
```

### 3.6 REPL Workflow Examples

The REPL is ideal for prototyping and interactive data exploration. Test your commands interactively before incorporating them into scripts.

```za
# Filter system data for interesting entries
>> high_usage = disk_usage() ?> `#.usage_percent > 50`

# Examine results
>> println high_usage.pp
[
  {
    "available": 65881,
    "mounted_path": "/sys/firmware/efi/efivars",
    "path": "efivarfs",
    "size": 151464,
    "usage_percent": 56.50385570168489,
    "used": 85583
  }
]

# Create a quick report
>> foreach item in high_usage
>>     println "ALERT: {=item.path} at {=item.mounted_path} is {=item.usage_percent}"
>> endfor
ALERT: /dev/sda1 at /boot is 92%
ALERT: /dev/sda2 at / is 87%
```
```
```

#### System Inspection Workflow

```za
# Quick system overview
>> println sys_resources().pp
{
  "CPUCount": 20,
  "LoadAverage": [
    0.59,
    0.32,
    0.28
  ],
  "MemoryTotal": 32889544704,
  "MemoryUsed": 3189137408,
  "MemoryFree": 25005395968,
  "MemoryCached": 4691128320,
  "SwapTotal": 4294963200,
  "SwapUsed": 0,
  "SwapFree": 4294963200,
  "Uptime": 12977.04
}

# Network interface check
>> println pp(net_devices() ?> `#.name ~ "wlan"`)
[
  {
    "device_type": "1",
    "duplex": "",
    "enabled": true,
    "gateway": "192.168.1.1",
    "ip_addresses": [
      "192.168.1.16",
      "fe80::869e:56ff:fe34:d39d"
    ],
    "link_speed": "",
    "mac_address": "84:9e:56:34:d3:9d",
    "name": "wlan0",
    "operstate": "up"
  }
]
```

These patterns demonstrate how the REPL enables iterative development with immediate feedback, making it ideal for the exploratory nature of system administration work.

---

# Part II — Language Fundamentals

## 4. Lexical structure

- Comments begin with `#`.
- Identifiers are used for variables, functions, and type names.
- Whitespace is mostly insignificant, except in string literals.

## 5. Literals and constants

### 5.1 Strings

Za supports **double-quoted** strings and **backtick** strings. Both may span multiple source lines
and may contain interpolation.

Backticks are especially useful when you want to avoid escaping double quotes inside expression strings:

```za
expr = `#.replace("%","").as_int > 80`
```

You can also escape characters such as quotes/backticks within string literals. Similarly,
control codes such as line feed, tab and others may be expressed using escape sequences, in
the usual manner for C-like sprintf formatting.

Za also supports interpolation forms such as `{...}` and `{=...}` (interpolation can be enabled/disabled via policy controls).

### 5.2 Numeric literals

Numeric literal type is determined by suffix and decimal point:

- `10` → int
- `10.0` or `10f` → float (64-bit)
- `10n` → bigi
- `10.5n` → bigf

Integer base prefixes are supported:

- `x = 0xFF`
- `y = 0o755`
- `z = 0b1010`

### 5.3 Built-in constants

Za provides only:

- `true`, `false`
- `nil`
- `NaN`

## 6. Variables and assignment

### 6.1 Implicit creation

Most variables are created by assignment:

```za
x = 10
name = "db1"
```

With implicit creation, there is no fixed type for the variable, but using it
in invalid combinations with operators will report run-time errors.

In the event that you wish to catch these type errors earlier, you can also
declare a variable using the VAR statement:
e.g.
```za
var x int = 10
```

Using VAR in this way will:

- fix the type and, optionally, set the value.
- raise an error if the value assigned (during declaration or after) is of
   an incompatible type.
- prevent re-typing of the variable (and report if this is attempted).

To re-type that variable locally you would first have to UNSET the variable.

### 6.2 Auto-vivification (assignment-driven)

Auto-vivification is an assignment feature: assigning through an access path
creates intermediate containers as needed. This enables concise construction
of nested structures without pre-allocation boilerplate.

### 6.3 Global mutation is explicit (`@`)

To modify a global variable from inside a function, use `@`. Example pattern:

```za
def q()
    @a = true
end
```

Without `@`, assignment targets local scope.

## 7. Typed declarations with `var`

Za is dynamically typed by default. `var` is used when you want explicit intent:

### 7.1 Scalars and structs

```za
var z int
var user struct_user
var cow,pig,sheep animal
```

Namespaced struct types are supported:

```za
var x ea::type_struct
```

### 7.2 Fixed-size arrays (usual pattern)

Fixed-size arrays use:

```za
var arr [1000] int
```

Multi-dimensional fixed arrays are also supported:

```za
var grid [2][3]int
```

### 7.3 Multi-dimensional dynamic arrays

```za
var matrix [][]int
var cube [][][]string
```

Dynamic arrays will be resized on demand on out-of-bounds assignment.

---

# Part III — Data Types and Structures

## 8. Scalar types

Za provides the scalar types used most often in operational scripting:

- `int`, `uint`, `byte`
- `float`
- `bigi`, `bigf`
- `string`, `bool`
- `any`

## 9. Arrays and multi-dimensional data

Arrays may be dynamic or fixed-size. Nested arrays represent multi-dimensional data. Matrix-style helpers exist in the array library (e.g., identity/trace/determinant/inverse), operating on nested arrays with consistent dimensions.

## 10. Maps

Map literals use a dotted-key form:

```za
m = map(.host "localhost", .port 5432)
```

Maps are used for:

- configuration/data storage
- options to library calls
- semi-structured records
- sets (keys only)

## 11. Maps as sets

A map can represent a set using its keys. Set algebra operators apply to maps, which may be hierarchical or flat:

- union: `|`
- intersection: `&`
- difference: `-`
- symmetric difference: `^`

Predicate functions for relationships:

- `is_subset(a, b)`
- `is_superset(a, b)`
- `is_disjoint(a, b)`

Use operators when you want a resulting set, and predicate functions when you want a relationship test.

## 12. Structs and anonymous structs

### 12.1 Defining structs

```za
struct person
    name string [ = "default_value" ]
    age  int    [ = default_value ]
endstruct
```

Field names are normalised (you do not need to capitalise struct field names).

You may declare a variable of a struct type using

`var variable_name struct_name`

The variable will receive any default values you set during struct definition (or
zero-like defaults if no defaults were provided).

You can manipulate fields of the struct through dotted access, e.g.:

```za
var p person
p.name = "Billy"
p.age  = 42
```

### 12.2 Struct literals

Struct instances may also be created using constructor-style syntax and field initialisers (as shown in examples):

```za
p1 = person("Alice",30)
p2 = person(.name "Bob", .age 21)
```

### 12.3 Anonymous structs

Anonymous structs are created with `anon(...)`:

```za
x = anon(.device device, .usage usage)
```

Use them when you want record-like data without declaring a named struct type.

## 13. Struct-associated functions and `self`

Structs may contain function definitions, supporting a lightweight method-like scheme.

Inside a struct-associated function, `self` refers to the current instance, and fields may be read/updated via `self.field`.

## 14. Enums

Za has an `enum` statement for defining enums at global or module scope. There is one
predefined enum: `ex`, containing default exception categories.

An enum is defined like this:

```za
enum enum_name ( enum_name_1 [ = value1 ] [ , ... , enum_name_N [ = valueN ] ] )
```

Setting a value with = sets the current auto-incrementing value. This means that
you should always set a value for non-integer enum values.

---

# Part IV — Expressions and Control Flow

## 15. Expressions and precedence

Za defines operator precedence in the interpreter. Notable points:

- arithmetic binds tighter than comparisons
- comparisons bind tighter than boolean `and/or`
- mapping/filtering (`->`, `?>`) bind relatively loosely (near assignment),
   which is intentional for readability of pipelines
- you may occasionally need to avoid UFCS syntax due to this

## 16. Operators (overview)

**Comprehensive operator coverage for system administration tasks.**

Za provides a rich set of operators that cover everything from basic arithmetic to advanced string manipulation and file operations. These operators are designed to make common system administration tasks concise and readable.

### 16.1 Arithmetic Operators

The standard arithmetic operators work with both integers and floating-point numbers:

```za
# Basic arithmetic
result = 10 + 5        # 15
difference = 20 - 8    # 12
product = 6 * 7        # 42
quotient = 15 / 3      # 5.0
remainder = 17 % 5     # 2
power = 2 ** 8         # 256

# Floating-point operations
pi_approx = 22 / 7     # 3.142857...
area = 3.14159 * radius ** 2

# System administration examples
cpu_cores = 4
total_threads = cpu_cores * 2  # Hyperthreading
memory_gb = 16
memory_mb = memory_gb * 1024    # Convert to MB
disk_usage_percent = as_float(used_space / total_space) * 100
```

### 16.2 Comparison Operators

Comparison operators return boolean values and are essential for conditional logic:

```za
# Numeric comparisons
if cpu_usage > 80.0
    alert("High CPU usage")
endif

if memory_available < 1024  # Less than 1GB
    alert("Low memory")
endif

# String comparisons
if hostname == "web-server-01"
    role = "web"
endif

if version != "latest"
    update_available = true
endif

# System administration examples
if disk_usage_percent >= 90
    cleanup_old_logs()
endif

if response_time <= 100  # milliseconds
    service_status = "good"
endif

if load_average >= cpu_cores
    scale_horizontal()
endif
```

### 16.3 Boolean Operators

Za provides both word-based and symbol-based boolean operators:

```za
# Word-based operators (more readable)
if user_exists and password_valid
    grant_access()
endif

on backup_failed or disk_full do send_alert()

if not service_running
    start_service()
endif

# Symbol-based operators (concise)
user = get_env("USER")
pass = get_env("PASSWORD")
debug = get_env("DEBUG")
verbose = get_env("VERBOSE")

if user != "" && pass != ""
    authenticate()
endif

if debug == "true" || verbose == "true"
    enable_logging()
endif

service_active = get_env("SERVICE_ACTIVE")
if service_active != "true"
    restart_service()
endif

# System administration examples
if file_exists(config) and permissions_ok(config)
    load_config(config)
endif

if morning_hours or weekend
    backup_mode = "full"
else
    backup_mode = "incremental"
endif
```

### 16.4 Bitwise Operators

Bitwise operators are useful for working with file permissions, network masks, and flags:

```za
# File permission manipulation
read_write = 0o666
executable = read_write | 0o111  # Add execute permission

# Permission checking
if file_mode & 0o111  # Check if executable
    file_type = "executable"
endif

# Network operations
network_mask = 0xFFFFFF00
network_part = ip_address & network_mask

# Flag operations
backup_flags = 0
backup_flags = backup_flags | 0x01  # Enable compression
backup_flags = backup_flags | 0x02  # Enable encryption

if backup_flags & 0x01
    use_compression = true
endif
```

### 16.5 Set Operators on Maps

Set operators work on maps to combine, intersect, and subtract key-value pairs:

```za
# Configuration merging
default_config = map(.port 8080, .timeout 30, .debug false)
user_config = map(.timeout 60, .debug true)
final_config = default_config | user_config
# Result: {"port": 8080, "timeout": 60, "debug": true}

# Finding common configuration
required_keys = map(.host "localhost", .port 8080)
provided_keys = map(.host "localhost", .port 8080, .ssl true)
common = required_keys & provided_keys
# Result: {"host": "localhost", "port": 8080}

# Removing unwanted settings
base_config = map(.user "admin", .pass "secret", .host "db")
sanitized = base_config - map(.pass "secret")
# Result: {"user": "admin", "host": "db"}

# System administration examples
server_defaults = map(.cpu_limit "2", .memory "4G", .disk "20G")
override_config = map(.memory "8G", .disk "50G")
final_config = server_defaults | override_config
```

### 16.6 Range Operator

The range operator `..` creates numeric ranges useful for loops and indexing:

```za
# Basic ranges
numbers = 1..10        # [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]

# System administration examples
#  - many of these can be done in alternate ways

# Port checking using shell commands (you would normally use a lib call for this)
open_ports=[]
foreach port in 8000..8005
    port_check = ${nc -z localhost {port} 2>&1}
    # Empty output means port is open
    on port_check.len == 0 do open_ports=concat(open_ports,port)
endfor

# Process multiple log files
foreach day in 1..31
    log_file = "/var/log/app-{day}.log"
    on is_file(log_file) do process_log(log_file)
endfor

# Generate server names
foreach i in 1..5
    server_name = "web-server-{i}"
    create_server(server_name)
endfor
```

### 16.7 Regex Match Operators

Za provides several regex operators for pattern matching:

```za
# Case-sensitive match (for this you would normally just do file_type = $pe filename)
if filename ~ "\\.log$"
    file_type = "log"
endif

# Case-insensitive match
if hostname ~i "^web"
    server_role = "web"
endif

# System administration examples
if log_line ~ "ERROR"
    error_count += 1
endif

if user_agent ~i "bot|crawler|spider"
    traffic_type = "automated"
endif
```

### 16.8 Path Unary Operators

Path operators provide convenient file system path manipulation:

```za
filepath = "/home/user/documents/report.txt"

# $pa - absolute path
path = $pa filepath        # "/home/user/documents"

# $pp - parent path
parent = $pp filepath          # "/home/user"

# $pb - Base name (filename)
basename = $pb filepath       # "report.txt"

# $pn - Name without extension
name = $pn filepath           # "report"

# $pe - Extension
extension = $pe filepath      # "txt"
```

### 16.9 String Transform Operators

String operators provide common text transformations:

```za
text = "Hello World"

# $uc - Uppercase
upper = $uc text              # "HELLO WORLD"

# $lc - Lowercase
lower = $lc text              # "hello world"

# $st - String trim (both ends)
trimmed = $st "   spaced   "      # "spaced"
# $lt and $rt also exist for left trim and right trim
```

### 16.10 File I/O Operators

File operators make reading and writing files concise:

```za
# $in - Read file contents
config_content = $in "/etc/app/config.json"
log_data = $in "/var/log/system.log"

# $out - Write to file
"New config content" $out "/tmp/new_config.json"
log_entry $out "/var/log/app.log"

# System administration examples
# Read configuration
db_config = json_decode($in "/etc/database/config.json")

# Write backup
current_config $out "/backup/config-{=date()}.json"

# Process configuration files - you could also use glob() for this
foreach config_file in dir("/etc/app",".*.conf")
    content = $in config_file.name
    processed = process_config(content)
    processed $out "/tmp/processed_{=$pb config_file}"
endfor
```

### 16.11 Shell Execution Operators

Shell operators enable system command execution:

```za
# | - Pipe to shell (discard output)
| mkdir -p /tmp/backup

# Capture output
files =| ls -la /var/log
disk_usage =| df -h /

# Process monitoring - there are other ways of doing this
if ${pgrep mysqld} == ""
    | systemctl start mysql
endif

# Log analysis
error_count = ${grep -c ERROR /var/log/app.log} . as_int
if error_count > 100
    send_alert("Too many errors in logs")
endif
```

These operators provide a comprehensive toolkit for system administration tasks, making Za scripts concise, readable, and powerful for managing complex system operations.

### 16.1 Numeric Clamping with `[start:end]`

Za provides a clamping operator for constraining numeric values within specified ranges. This is particularly useful for data validation, normalization, and ensuring values stay within acceptable bounds.

#### Clamping Syntax

```za
result = value[start:end]
```

The expression returns:

- `start` if `value < start`
- `end` if `value > end`
- `value` if `start ≤ value ≤ end`

#### Type Inference

- If clamping occurs, **result has the same type as the clamping bound** (`start` or `end`)
- If `value` lies within range, result has the same type as `value`

#### Partial Clamping

Either bound may be omitted:

```za
# Only upper bound
percentage = score[:100]

# Only lower bound
temperature = [-10:]

# No clamping (full range)
normal_range = [0:100]
```

#### Practical Examples

```za
# Basic clamping
sensor_reading = 127
normalized = sensor_reading[0:255]  # Clamps to 0-255 range
percentage = 85[0:100]              # Stays at 85
overflow = 300[0:255]               # Clamped to 255

# Type handling
float_value = 4.7[3.0:5.0]          # Result: 4.7 (within range)
int_clamped = 5.7[3:5]              # Result: 5 (int, clamped to upper bound)

# Configuration bounds
cpu_usage = current_cpu[0f:100f]  # Normalize CPU percentage
memory_usage = current_mem[0f:1f] # Normalize to 0-100% range
```

#### Use Cases for System Administration

```za
# Network latency normalization
latency_ms = ping_time[0:5000]      # Clamp to reasonable network range

# File size validation
file_size_mb = file_size[0:10240]   # Max 10GB in MB

# Process priority adjustment
priority = nice_level[-20:19]       # Valid nice range

# Temperature monitoring - not sure why you would enforce this, but...
temp_celsius = sensor_temp[-40:125] # Operating range for server room
```

This clamping syntax reuses Za's range-like brackets `[:]` but provides distinct behaviour for numeric types compared to array slicing.

## 17. Conditionals

Block conditional:

```za
if condition
    # action
[ else
    # action ]
endif
```

Single-statement guard:

```za
on condition1 do break
on condition2 do println "ok"
# etc
```

`on … do` executes exactly one statement when condition is true.

## 18. Loops

Counted loop:

```za
for i=0 to 10
    println i
endfor
```

-or-

```za
# c-like for construct - each term is optional
for i=0, i<=10, i++
	println i
endfor
```

Container iteration:

```za
foreach item in items
    println item
endfor
```

Loop control:

```za
break [ construct_type ]
break if condition
continue
continue if condition
```

## 19. Case statements

The CASE construct is written as a switch-like variant.
It also allows for pattern matching:

```za
case [expression]
[is expression_value
	# action
	]
[has condition_expression
    # action
	]
[contains "regex_match"
	# action
	]
	.
	.
[or
    # default action
	]
endcase
```

---

# Part V — Functional Data Processing

## 20. Expression strings, `#`, and `$idx`

Za uses expression strings in several places:

- filter operator `?>`
- map operator `->`
- array search functions `find()` and `where()`

Substitution phrases:

- `#` → current element value
- `$idx` → index/key (for maps, `$idx` is always a string; map keys are always strings)

Use backticks for clarity when the expression contains quotes.

## 21. Map and filter operators

Filter:

```za
bad = rows ?> `#.UsePercent.replace("%","").as_int > 80`
```

Map:

```za
names = users -> `#.name`
```

## 22. Searching arrays (`find`, `where`)

Because the same expression engine is re-used, you can write consistent predicates:

```za
idx = rows.find(`#.MountedOn=="/"`)
sel = rows.where(`#.Filesystem~"^/dev"`)
```

---

# Part VI — Functions, Modules, and Composition

## 23. Functions (`def … end`)

```za
def f(x)
    return x*2
end
```

`return` may be used without a value.
`return` may also return multiple values (comma separated expressions).

The return values may also be unpacked on return:

```go
def f(a,b,c)
	return b,c,a
end

def g(a,b,c)
	return [b,c,a]
end

b,c,a=f(1,2,3)
b,c,a=g(4,5,6)
# or
vals=g(4,5,6) # vals=[5,6,4]
```

## 24. Modules and namespaces

Import a module:

```za
module "cron"
module "util" as u
```

Namespaced types and values are referenced with `::`:

```za
var x u::struct_example
```

The USE statement

This statement is used to indicate the order in which namespaces are processed by the interpreter.

Syntax:
```za
    USE -           # empties the use_chain internal array
    USE + name      # adds name to the use_chain namespace list (if unique)
    USE - name      # removes name from use_chain namespace list
    USE ^ name      # places namespace name at the top of the use_chain namespace list (pushes rest down)
                    # new name: inserts, existing name: moves
    USE PUSH        # push current chain on chain stack
    USE POP         # pop chain from top of chain stack
                    # push and pop would be used to completely isolate namespacing in a module.
```

The current namespace is always either main:: or the module name/alias.
If you want to use a different namespace then you need to create a new file and import it with MODULE.

The use_chain array will be consulted, when not empty, on function calls, enum references and struct references for
matches ahead of the default behaviour, if no explicit name is supplied:

	1. explicit namespace (name::)
	2. use_chain match
	3. current namespace (no :: ref), then
	4. main:: (global namespace)

Example:

```za
	# global ns / main program
	MODULE "modpath/time" AS tm

	tm::string_date()       # call function string_date in module time with explicit alias 'tm'
	string_date()           # tries to call (non-existant) function string_date in main:: namespace (current namespace)
							# which should error as undefined.

	USE +tm

	string_date()           # check if string_date() exists in tm namespace and call it if found.

							# if not found (even though this one would be) then try to call it in current namespace (main::)
							#  which should error as undefined.

							# whenever there are conflicting names then the first match takes precedence.
							#  i.e.
							# explicit name > use_chain > current namespace > main

```

---

# Part VII — Errors, Debugging, and Safety

## 25. Error handling philosophy

Exceptions exist, but are not intended as a primary, idiomatic error-handling scheme. You should prefer explicit return values and
structured results where available, using exceptions when there is no better alternative or when you choose that style deliberately.

**Important Note:** Avoid using `try...catch` blocks for routine error handling. Reserve exceptions for truly exceptional circumstances that cannot be handled through normal return value patterns. Overuse of exceptions makes code harder to read and maintain.

## 26. Exceptions (`try … catch … then … endtry`)

A try block may capture outer variables explicitly with `uses`:

```za
try uses captured_var [,...,captured_var] [throws string_category|ex_enum_category]
    # action
catch [err] [is category_expr | in list_expr | contains regex]
    # action
endtry
```

Catches may use predicates (example pattern):

```za
catch err is "invalid"
    println "invalid:", err
```

`then` is the cleanup/finally section and runs regardless of whether an exception occurred.

## 27. Enhanced error handling (`trap`, `error_*`)

Za supports registering an error trap and introspecting error context inside the handler using `error_*` functions. Use this to produce better diagnostics (message, source location, source context, call stack, locals/globals) than the default handler when needed.

## 28. Debugger

Za includes an interactive debugger that supports step execution, expression inspection, tracing, and runtime introspection. The debugger is designed for both development debugging and operational troubleshooting.

### 28.1 Enabling the Debugger

Debugging can be enabled in two ways:

- **Via command line**:
  ```bash
  za -D script.za
  ```

- **From within a script**:
  ```za
  debug on
  ```

Disable it using:
  ```za
  debug off
  ```

### 28.2 Debugger Activation

When enabled, the debugger can pause execution at:

- **Manual breakpoints** using `debug break`
- **Interpreter startup** (`za -D`)
- **External signals**:
  - **UNIX**: `SIGUSR1`
  - **Windows**: `CTRL+BREAK` or `SIGBREAK`

> ⚠️ The debugger does **not** automatically trigger on unhandled errors or exceptions.

### 28.3 Setting Breakpoints

Use breakpoints to pause execution at specific points:

```za
def complex_calculation(x, y)
    debug break  # Pause execution here
    result = x * y + sqrt(x + y)
    debug break  # Or here to inspect intermediate result
    return result
end
```

### 28.4 Debugger Prompt and Commands

When paused, Za shows an interactive prompt:

```
[scope main : line 0012 : idx 0008] debug> _
```

#### Essential Debugger Commands

| Command | Description |
|---------|-------------|
| `c`, `continue` | Resume execution |
| `s`, `step` | Step into next statement or function |
| `n`, `next` | Step to next statement in current function |
| `l`, `list` | Show current statement tokens |
| `v`, `vars` | Dump local variables |
| `p <var>`, `print <var>` | Print value of a variable |
| `bt`, `where` | Show call chain backtrace |
| `w <var>`, `watch <var>` | Add variable to watch list |
| `e <expr>`, `eval <expr>` | Evaluate expression in current scope |

#### Advanced Debugger Commands

| Command | Description |
|---------|-------------|
| `ctx` | Set line context size for `list` |
| `mvars` | Dump module/global variables |
| `gvars` | Dump system/global variables |
| `sf`, `showf` | Show all defined functions |
| `ss`, `shows` | Show all defined structs |
| `b`, `breakpoints` | List all breakpoints |
| `b+`, `ba` | Add a breakpoint interactively |
| `b-`, `br` | Remove a breakpoint |
| `d`, `dis` | Disassemble current statement tokens |
| `uw <var>`, `unwatch <var>` | Remove variable from watch list |
| `wl`, `watchlist` | Show all watched variables |
| `fn`, `file` | Show current file name |
| `ton`, `traceon` | Enable line-by-line execution trace |
| `toff`, `traceoff` | Disable execution trace |
| `fs`, `functionspace` | Show current debug entrypoint and functionspace |
| `cls` | Clear debugger screen |
| `q`, `quit`, `exit` | Exit interpreter completely |
| `h`, `help` | Show this help message |

### 28.5 Debugger Scope and Behaviour

- The debugger only affects **the main script thread**. Any `async` tasks or shell commands continue running independently.
- Variable commands (`vars`, `mvars`, `gvars`) rely on standard library debug functions.
- Line tracing (`traceon`) shows execution flow at a fine-grained level.
- The debugger maintains separate watch lists for different variable scopes.

### 28.6 Practical Debugging Examples

#### Debugging Function Calls

```za
def calculate_discount(price, category)
    debug break  # Inspect inputs
    if category == "premium"
        discount = 0.20
    else
        discount = 0.10
    endif
    debug break  # Check calculation logic
    final_price = price * (1 - discount)
    debug break  # Verify result
    return final_price
end

# Usage
price = calculate_discount(100.0, "premium")
# Debugger pauses at each debug break for inspection
```

#### Debugging Data Processing

```za
# Complex data transformation
data = table(${ps aux}, map(.parse_only true))
cpu_intensive = data ?> `#[2].as_float > 0.5`

debug break  # Inspect filtering results

# Process high-CPU processes
foreach proc in cpu_intensive
    debug break  # Examine each process
    println "Killing process, name : ", proc[10]
    | kill -9 {=proc[1]}
endfor
```

#### Debugging System Integration

```za
# Network service debugging
def check_service(host, port)
    debug break  # Check connection parameters
    result = tcp_ping(host, port, 5000)  # 5 second timeout
    debug break  # Examine connection result

    if not result.okay
        log error: "Cannot reach", host, ":", port
        return false
    endif

    debug break  # Verify success path
    return true
end

# Test with debugger
reachable = check_service("db.example.com", 5432)
```

## 29. Profiler

Za includes a built-in function-level profiler that records execution times for each function and optionally provides detailed call-chain breakdowns.

### 29.1 Enabling the Profiler

Enable profiling using either:

- **Command line**:
  ```bash
  za -P script.za
  ```

### 29.2 Profiler Behaviour

When enabled:

- All function calls are instrumented to record their **inclusive durations**
- The profiler records:
  - Total time spent in each function
  - Recursive call depth (when detected)
  - Per-caller breakdowns for detailed path analysis

Profiling incurs minimal overhead but is best used for performance debugging and analysis.

### 29.3 Viewing and Interpreting Results

After script execution completes, a summary is printed to standard output:

```za
Profile Summary

main:
  parse: 360.182µs
  enum_names: 15.696µs
  execution time: 13.345043ms

  main > x11:
    execution time: 57.649µs

x11::fg:
  enum_names: 287.404µs
  eval: 717.135µs
  execution time: 3.801211ms
```

#### Interpreting Output

- **Entries show** fully qualified function name and **total inclusive time**
- **Recursive paths** are grouped under caller chains:
  - Example: `main > main::compute`
- **Recursive timings** are flagged as `[unreliable]` since inclusive durations may be inflated by self-calls
- **Function path tracking** helps identify **expensive call chains** and **inefficient recursion**

### 29.4 Performance Analysis Patterns

#### Finding Bottlenecks

```za
# Profile this script
za -P data_processing.za

# Sample output shows:
data_processing:
  load_data: 2.3s
  parse_records: 8.7s
  validate_data: 15.2s
  transform_data: 45.6s  # <-- Bottleneck
  save_results: 1.1s
```

```

#### Analyzing Recursive Performance

**Note:** The following example is illustrative only - actual profiling output may differ.

```za
# Recursive function with profiling
def factorial(n)
    on n <= 1 do return 1
    return n * factorial(n - 1)
end

# Profile to analyze recursion depth
za -P recursive_test.za
# Look for performance patterns in actual output
```

### 29.5 Profiler Best Practices

- **Use for specific performance investigation**, not continuous monitoring
- **Focus on total time** rather than individual call counts for optimization
- **Pay attention to recursive paths** - they may indicate algorithmic improvements
- **Combine with debugger** for detailed analysis of slow functions
- **Profile realistic data** - artificial test data may not reveal real bottlenecks

## 30. Security controls (`permit`)

`permit()` controls runtime capabilities such as allowing shell execution, eval, interpolation, macros, and strictness for uninitialised variables.

**Comprehensive security best practices for Za scripts.**

If you are going to use Za in situations it was not designed for, then work through the checklist below:

### 30.1 Security Checklist

Use this checklist for security review:

**Input Validation:**

- [ ] All external inputs are validated
- [ ] File paths are sanitized against directory traversal
- [ ] Numeric inputs are range-checked
- [ ] String inputs are checked for injection patterns

**File Operations:**

- [ ] File permissions are checked before access
- [ ] Sensitive files have restricted permissions
- [ ] Temporary files are properly cleaned up
- [ ] File paths are validated and canonicalized

**Command Execution:**

- [ ] Commands are validated against allowlist
- [ ] Shell injection vulnerabilities are prevented
- [ ] Command output is properly handled

**Network Security:**

- [ ] URLs are validated for protocol and format
- [ ] Server-side request forgery is prevented
- [ ] API keys and secrets are properly handled
- [ ] HTTPS is used for sensitive communications

**Logging and Monitoring:**

- [ ] Sensitive data is not logged
- [ ] Security events are properly logged
- [ ] Error messages are sanitized for external display
- [ ] Log files have appropriate permissions

**Code Integrity:**

- [ ] Script files have appropriate permissions
- [ ] Configuration files are validated
- [ ] Code signing/integrity checks are implemented
- [ ] Dependencies are from trusted sources

By following these security practices and using `permit()` to control runtime capabilities, you can write Za scripts that are robust, maintainable, and secure against common vulnerabilities. It does not mean that you should though.

Handle errors without exposing sensitive information:


### 30.2 Runtime Security with `permit()`

The `permit()` function controls runtime capabilities such as allowing shell execution, eval, interpolation, macros, and strictness for uninitialised variables. For hardened scripts, disable what you don't need and re-enable only deliberately.

```za
# Disable all potentially dangerous features
permit("shell", false)
permit("eval", false)
permit("interpol", false)
permit("macro", false)

# Enable strict variable checking
permit("uninit", false)

# Later, selectively re-enable what's needed
permit("shell", true)
permit("sanitisation", false)
```

### 30.3 Input Validation and Sanitization

Always validate and sanitize external inputs to prevent injection attacks and data corruption.


## 31. Common async patterns

**Parallel execution patterns for system administration.**

Za's async capabilities enable efficient parallel processing of multiple hosts, services, or data sources.

### 31.1 Fan-out/Fan-in Host Probing

```za
# Define async check function
def check_host(host_id)
    pause rand(500)  # Simulate network delay
    return host_id % 3 == 0 ? "up" : "down"
end

# Fan-out: check multiple hosts in parallel
hosts = [1, 2, 3, 4, 5, 6]
var handles map

for x = 0 to len(hosts)-1
    async handles check_host(hosts[x]) x  # Use index as key
endfor

# Fan-in: collect all results
results = await(ref handles, true)
for e = 0 to len(hosts)-1
    println "Host {=hosts[e]} -> {=results[e]}"
endfor

# Filter up hosts
up_hosts = hosts ?> `results[#] == "up"`
println "Up hosts:", up_hosts
```

### 31.2 Parallel Service Checks

```za
# Service status checking
def check_service(service_name)
    In real usage: status = ${systemctl is-active {service_name}}
    return service_name ~ "nginx|mysql" ? "running" : "stopped"
end

# Check multiple services in parallel
services = ["nginx", "mysql", "redis", "postgresql"]
var service_handles map

for x = 0 to len(services)-1
    async service_handles check_service(services[x]) x  # Use index as key
endfor

service_results = await(ref service_handles, true)
for e = 0 to len(services)-1
    println "Service {=services[e]} -> {=service_results[e]}"
endfor

# Filter running services
running_services = services ?> `service_results[#] == "running"`
println "Running services:", running_services
```

### 31.3 Parallel Data Processing

```za
# Process multiple data items in parallel
def process_data(item)
    pause rand(500)  # Simulate processing time
    return item * 2  # Simple transformation
end

# Process array in parallel
data = [10, 20, 30, 40, 50]
var data_handles map

for x = 0 to len(data)-1
    async data_handles process_data(data[x]) x  # Use index as key
endfor

processed_results = await(ref data_handles, true)
for e = 0 to len(data)-1
    println "Input: {=data[e]} -> Output: {=processed_results[e]}"
endfor

# Filter results
high_values = processed_results ?> `# > 50`
println "High values:", high_values
```

### 31.4 Isolated Failure Handling

```za
# Network checks with isolated failures
def check_network(host)
    pause rand(500)  # Simulate network timeout
    # Simulate different failure modes
    case host
    is "8.8.8.8"
        return "up"
    is "1.1.1.1"
        return "timeout"
    or
        return "down"
    endcase
end

# Check multiple network hosts
network_hosts = ["8.8.8.8", "1.1.1.1", "gateway.company.com"]
var net_handles map

for x = 0 to len(network_hosts)-1
    async net_handles check_network(network_hosts[x]) x  # Use index as key
endfor

network_results = await(ref net_handles, true)
for e = 0 to len(network_hosts)-1
    println "Host {=network_hosts[e]} -> {=network_results[e]}"
endfor

# Process results even if some failed
up_hosts = network_hosts ?> `network_results[#] == "up"`
problem_hosts = network_hosts ?> `network_results[#] != "up"`

println "Up hosts:", up_hosts
println "Problem hosts:", problem_hosts
```

### 31.5 Partial Completion Collection

```za
# Database connectivity with partial success handling
def check_database(db_name)
    pause rand(800)  # Simulate connection attempt
    # Simulate different connection outcomes
    case db_name
    is "primary"
        return "connected"
    is "replica"
        return "connected"
    is "cache"
        return "timeout"
    or
        return "error"
    endcase
end

# Database configurations
databases = ["primary", "replica", "cache", "backup"]

# Check all databases in parallel
var db_handles map
for x = 0 to len(databases)-1
    async db_handles check_database(databases[x]) x  # Use index as key
endfor

db_results = await(ref db_handles, true)
for e = 0 to len(databases)-1
    println "Database {=databases[e]} -> {=db_results[e]}"
endfor

# Count successful connections
connected_dbs = databases ?> `db_results[#] == "connected"`
total_dbs = len(databases)

println "Connected: {=len(connected_dbs)}/{total_dbs} databases"

if len(connected_dbs) < total_dbs
    failed_dbs = databases ?> `db_results[#] != "connected"`
    println "Failed databases:", failed_dbs
endif
```

These async patterns enable efficient parallel processing while maintaining robust error handling and partial result collection.

---

# Part IX — Output and Presentation

## 32. Program output (`print`, `println`)

Use `print`/`println` for ordinary output and examples not related to logging.

## 33. Inspection (`pp`) and tables (`table`)

UFCS calling forms are equivalent mechanisms for the same call:

- `pp(x)` and `x.pp`
- `table(x, opts)` and `x.table(opts)`

`table()` is used heavily for formatting record-like data and for importing CLI output when parsing is enabled via options.

## 34. Array display controls (`array_format`, `array_colours`)

These are **configuration functions**, not formatters:

- `array_format(true|false)` toggles array pretty-print mode (interpreter-wide)
- `array_colours([...])` sets the nesting depth colour scheme used by pretty array printing and returns the previous scheme

## 35. ANSI colour/style macros

Za strings supports inline style macros like:

```za
println "[#bold][#1]ERROR[#-] message"
```

The ANSI macro handling can be enabled/disabled via `ansi(true|false)` and startup flags.

A full list of supported style macros can be found with:

```za
help colour
```



---

# Part X — Standard Library Overview

## 36. Library categories and discovery

Za groups standard library calls into categories. For further information about library
calls use:

```za
HELP [statement_name|function_name]
funcs("partial-function-name|category-name")
```


## 37. Category tour (representative idioms)

**Comprehensive examples from Za's standard library categories.**

Za's standard library is organized into functional categories that provide ready-to-use tools for common system administration tasks. This section showcases representative idioms from each category to demonstrate practical usage patterns.

## 38. Category samples

### 38.1 String Operations

String manipulation is fundamental for processing logs, configuration files, and user input:

```za
# Basic string operations
text = "  System Log Entry  "
trimmed = text.trim                      # "System Log Entry"
upper = text.upper                       # "  SYSTEM LOG ENTRY  "
lower = text.lower                       # "  system log entry  "

# String splitting and joining
log_line = "2023-12-01 10:30:15 ERROR: Database connection failed"
parts = log_line.split(" ")              # ["2023-12-01", "10:30:15", "ERROR:", "Database", "connection", "failed"]
timestamp = parts[0] + " " + parts[1]    # "2023-12-01 10:30:15"
message = parts[3:].join(" ")            # "Database connection failed"

# Pattern matching and replacement
config_line = "port=8080"
if config_line ~ "port"
    port_value = config_line.split("=")[1].trim
    port_num = int(port_value)
endif

# Regular expressions for log parsing
log_entry = "192.168.1.100 - - [01/Dec/2023:10:30:15] \"GET /api/users HTTP/1.1\" 200 1234"
ip_match = log_entry.match("^(\\d+\\.\\d+\\.\\d+\\.\\d+)")
if ip_match
    client_ip = ip_match[0]
endif

# String formatting for reports
report = "Server: {0}, CPU: {1}%, Memory: {2}%".format(hostname, cpu_usage, memory_usage)
```

### 38.2 List and Array Operations

Lists and arrays are essential for managing collections of servers, files, or data points:

```za
# Creating and manipulating lists
servers = ["web-01", "web-02", "db-01", "cache-01"]
web_servers = servers ?> `has_start("web-")`     # ["web-01", "web-02"]

# List transformations
port_numbers = [80, 443, 8080, 3000]
secure_ports = port_numbers -> `as_int(#+1)`              # [81, 444, 8081, 3001]

# List aggregation
response_times = [120, 85, 200, 95, 150]
avg_response = response_times.sum / response_times.len    # Average response time
max_response = response_times.max                         # 200
min_response = response_times.min                         # 85

# Array operations for numeric data
cpu_readings = [45.2, 67.8, 89.1, 34.5, 78.9]
high_cpu_periods = cpu_readings ?> `#>80`                 # [89.1]

# Multi-dimensional arrays for metrics
hourly_metrics = [
    [10, 15, 12, 8],
    [20, 25, 18, 22],
    [30, 35, 28, 32]
] # Hour 0-3, 4-7, 8-11
morning_avg = hourly_metrics[1].sum / 4                # Average for hours 4-7
```

### 38.3 Map Operations

Maps are perfect for configuration management, key-value stores, and structured data:

```za
# Configuration management
server_config = map(
    .host "localhost",
    .port 8080,
    .ssl true,
    .timeout 30
)

# Accessing and modifying
if server_config.ssl is bool and server_config.ssl
    protocol = "https"
else
    protocol = "http"
endif

# Merging configurations
default_config = map(.timeout 60, .retries 3, .debug false)
user_config = map(.timeout 120, .debug true)
final_config = default_config.merge(user_config)
# Result: {"timeout": 120, "retries": 3, "debug": true}

# Map operations for system inventory
server_info = map(
    .web-01 map(.cpu 4, .memory 8, .disk 100),
    .web-02 map(.cpu 4, .memory 8, .disk 100),
    .db-01  map(.cpu 8, .memory 32,.disk 500)
)

# Extract specific information
total_memory = (server_info . values -> `#.memory`) . sum  # 48 GB
high_cpu_servers = server_info ?> `#.cpu>4`                # {"db-01": {...}}

# Dynamic map building - not real functions!
for server in server_list
    cpu = get_cpu_usage(server)
    memory = get_memory_usage(server)
    metrics[server] = map(.cpu cpu, .memory memory)
endfor
```

### 38.4 Type Conversion

Conversion between data types is crucial for data processing and validation:

```za
# String to number conversion
port_str = "8080"
port_num = port_str.as_int                     # 8080
memory_str = "4.5 GB"
memory_gb = as_float(memory_str.split(" ")[0]) # 4.5

# Number to string conversion
cpu_percent = 75.5
cpu_str = as_string(cpu_percent)               # "75.5"
status_code = 200
status_str = as_string(status_code)            # "200"

# JSON conversion
config_map = map(.host "localhost, .port 8080)
config_json = config_map.pp                 # '{"host":"localhost","port":8080}'
parsed_config = json_decode(config_json)    # Back to map

# Base64 encoding/decoding - don't do this
secret_data = "user:password"
encoded = secret_data.base64e        # "dXNlcjpwYXNzd29yZA=="
decoded = encoded.base64d            # "user:password"
```

### 38.5 File Operations

File operations are essential for configuration management, log processing, and data persistence:

```za
# Reading files
config_content = read_file("/etc/app/config.json")

# Writing files
backup_content = "Backup created at " + date()
write_file("/backup/config-{=date()}.bak", backup_content)

# File existence and properties
if is_file("/etc/app/config")
    stat = stat("/etc/app/config")
    size_mb = stat.size / (1024f * 1024)
    modified = stat.modtime
endif

# Directory operations
config_files = dir("/etc/app",".*.conf")  # List all .conf files
foreach config_file in config_files
    if config_file.size > 0
        process_config(config_file.name)
    endif
endfor
```

### 38.6 OS and System Operations

System operations enable interaction with the operating system for monitoring and control:

```za
# Environment variables
user = get_env("USER") or "unknown"
home_dir = get_env("HOME") or "/tmp"
path_list = get_env("PATH").split(":")

# System information
hostname = hostname()
os_type = os()
pid = pid()

# Directory operations
if not is_dir(file)
    | mkdir "{file}"
    | chmod 755 {file}
endif

cd("/var/log")
current_dir = cwd()

# Process management
current_pid = pid()
parent_pid = ppid()
process_info = ps_info(current_pid)

# User and group information
current_user = user()
current_uid = user_info(current_user).UID
current_gid = user_info(current_user).GID
```

### 38.7 Network Operations

Network operations provide tools for connectivity testing, HTTP requests, and network monitoring:

```za
# HTTP requests
response = web_get("https://api.example.com/status")
if response.code == 200
    status_data = json_decode(response.result)
endif

# POST request with data
data = map(.message "Server backup completed")
headers = map(.Content-Type "application/json")
response = web_raw_send("POST", https://hooks.slack.com/webhook", headers, data)

# Network connectivity testing
if icmp_ping("8.8.8.8")
    internet_available = true
endif

# Port checking
if port_scan("localhost", [80], 2) . 80
    web_server_running = true
endif

# DNS resolution
ip_address = dns_resolve("example.com")
if ip_address.records.len>0
    println "example.com resolves to: " + ip_address.records[0]
endif

# Network interface information
for interface in net_interfaces_detailed()
    if interface.up and not interface.name == "lo"
        println "Interface: ", interface.name, " IP: ", interface.ips
    endif
endfor
```

### 38.8 Database Operations

Database operations enable interaction with various database systems for data storage and retrieval:

```za

set_env("ZA_DB_ENGINE","sqlite3")

try
    h=db_init(execpath()+"/files/test.db")
    res=h.db_query("select * from users",map(.format "map"))
    h.db_close
    # first 30
    println res[:30].table(
        map(
            .border_style "unicode",
            .colours map(
                .header     fgrgb(200,100,0),
                .data       fgrgb(10,100,200)
            ),
            .column_order ["id","name","email"]
        )
    )
endtry

```

### 38.9 YAML and Configuration Operations

YAML operations are essential for managing configuration files in modern applications.

#### Basic Usage

Parse YAML:

```za
yaml_str = "name: John\nage: 30\ncity: New York"
data = yaml_parse(yaml_str)
```

Marshal to YAML:

```za
data["name"] = "Alice"
data["age"] = 28
yaml_output = yaml_marshal(data)
```

Parse nested structures:

```za
yaml3 = "person:\n  name: Jane\n  age: 25\n  hobbies:\n    - reading\n    - swimming"
result3 = yaml_parse(yaml3)
```

Parse lists:

```za
yaml2 = "- apple\n- banana\n- orange"
result2 = yaml_parse(yaml2)
```

# Get nested values

```za
host = yaml_get(data, "server.host")           # returns "localhost"
debug = yaml_get(data, "server.config.debug")  # returns true
port1 = yaml_get(data, "server.ports[0]")      # returns 8080
port2 = yaml_get(data, "server.ports[1]")      # returns 8081
```

# Update existing values

```za
data = yaml_set(data, "server.host", "example.com")
data = yaml_set(data, "server.config.debug", false)
data = yaml_set(data, "server.ports[0]", 9090)
```

# Add new values

```za
data = yaml_set(data, "server.timeout", 30)
```

# Remove specific values

```za
data = yaml_delete(data, "server.config.debug")
data = yaml_delete(data, "server.ports[1]")  # removes second port
```

Please see za_tests/test_yaml.za for a larger example set.

### 38.10 Archive Operations (ZIP)

Archive operations are useful for backup, deployment, and file distribution.

Create ZIP:

```za
files = ["test1.txt", "test2.txt"]
result = zip_create("test_archive.zip", files)
```

List contents:

```za
contents = zip_list("test_archive.zip")
```

Extract all files:

```za
result = zip_extract("test_archive.zip", extract_dir)
```

Extract specific files:

```za
result = zip_extract_file("test_archive.zip", ["files","to","extract"], "single_extract_dir")
```

Add files:

```za
result = zip_add("test_archive.zip", ["files","to","add])
```

Remove files:

```za
result = zip_remove("test_archive.zip", ["test2.txt"])
```

Please see za_tests/test_zip.za for a larger example set.

### 38.11 Regular Expressions (PCRE)

Regular expressions provide powerful pattern matching for text processing. The reg_* library calls use a PCRE library implementation instead of the builtin regular expression engine. Due to this, these calls are only available on static linux builds of Za.

Searching:

```za
# Tests if string contains regex match:
reg_match(string, regex)
```

Filtering:

```za
# Returns array of [start_pos, end_pos] match positions:
reg_filter(string, regex[, count])
```

Replacement:

```za
# Replaces regex matches with replacement string:
reg_replace(var, regex, replacement[, int_flags])
```


### 38.12 Checksum Operations

Checksum operations are essential for file integrity verification and security.

```za
# Returns MD5 checksum of input string:
md5sum(string)

 Returns SHA1 checksum of input string
sha1sum(string)

# Returns SHA224 checksum of input string
sha224sum(string)

# Returns SHA256 checksum of input string:
sha256sum(string)

# Returns struct with .sum and .err for S3 ETag comparison:
s3sum(filename[, blocksize])
```

- s3sum supports multipart upload calculations with configurable block sizes.
- Error codes: 0=ok, 1=single-part warning, 2=file error, 3=checksum error
- Auto-selects 8MB blocksize when blocksize=0

S3 ETag Functionality

The s3sum function specifically calculates checksums compatible with Amazon S3 ETags, including multipart upload format (hash-parts) for files larger than the blocksize.

### 38.13 TUI (Terminal User Interface)

Create TUI objects and style:

```za
# Create TUI options map
tui_obj = tui_new()
# Create style with custom borders and colours
style = tui_new_style()
```

Text display with box:

```za
tui_obj["Action"] = "text"
tui_obj["Content"] = "Hello, World!"
tui_obj["Row"] = 5
tui_obj["Col"] = 10
tui_obj["Width"] = 30
tui_obj["Height"] = 5
tui_obj["Border"] = true
tui(tui_obj, style)
```

Interactive menu:

```za
tui_obj["Action"] = "menu"
tui_obj["Title"] = "Choose an option:"
tui_obj["Options"] = ["Option 1", "Option 2", "Exit"]
tui_obj["Row"] = 10
tui_obj["Col"] = 20
result = tui(tui_obj, style)
```

Progress bar:

```za
tui_obj["Action"] = "progress"
tui_obj["Title"] = "Processing..."
tui_obj["Value"] = 0.75  # 75% complete
tui_obj["Row"] = 15
tui_obj["Col"] = 5
tui_obj["Width"] = 40
tui_progress(tui_obj, style)
```

Text editor:

```za
edited_text = editor("Initial content", 80, 24, "Edit Document")
```

Table display:

```za
tui_obj["Action"] = "table"
tui_obj["Data"] = "Name,Age,City\nJohn,30,NYC\nJane,25,LA"
tui_obj["Format"] = "csv"
tui_obj["Headers"] = true
tui_table(tui_obj, style)
```

Screen buffer switching:

```za
tui_screen(0)  # Switch to primary screen
tui_screen(1)  # Switch to secondary screen
```

The TUI system uses maps to configure display properties like position (Row, Col), size (Width, Height), content (Content, Data), and styling (Border, colours)


### 38.14 Notification Operations

Za provides 7 builtin file system notification library functions.

Watcher Management Functions

```za
# Create new watcher, returns [watcher, error_code]
# - Error codes: 0=success, 1=create_watcher_failed, 2=file_path_failure
ev_watch(filepath_string)

# Dispose of watcher object
ev_watch_close(watcher)

# Check if watcher is still available
ev_exists(watcher)
```

Path Management Functions

```za
# Add a path to existing watcher
ev_watch_add(watcher, filepath_string)

# Remove a path from watcher
ev_watch_remove(watcher, filepath_string)
```

Event Handling Functions

```za
# Sample events from watcher, returns notify_event or nil
ev_event(watcher)

# Test event type, returns filename or nil
ev_mask(notify_event, str_event_type)
```

Event Types

Supported event types for ev_mask:

- "create" - File/directory creation
- "write"  - File write operations
- "remove" - File/directory deletion
- "rename" - File/directory renaming
- "chmod"  - Permission changes


### 38.15 Error Handling and Logging

Robust error handling and logging are essential for reliable system administration:

```za
# Structured error handling
try
    config = load_configuration("/etc/app/config.json")
    validate_config(config)
    apply_config(config)
catch config_error
    log error: "Configuration error: " + config_error.message
catch validation_error
    log error: "Validation failed: " + validation_error.message
    rollback_config()
catch system_error
    log critical: "System error: " + system_error.message
    emergency_shutdown()
finally
    cleanup_temp_files()
endtry

# Custom exception types
exreg("ConfigError", "error")
exreg("NetworkError", "warning")
exreg("DatabaseError", "critical")

# Logging with different levels
log debug: "Starting configuration process"
log info: "Loading configuration from " + config_file
log warning: "Using default values for missing settings"
log error: "Failed to connect to database"
log critical: "System out of memory"

```

These representative idioms demonstrate the flexibility of Za's standard library categories for system administration tasks. Each category provides specialized tools that can be combined to create comprehensive automation solutions.

---

# Part XI — Sysadmin Cookbook

## 39. CLI data ingestion with `table()`

Use `table()` to turn columnar CLI output into structured data, avoiding fragile string slicing.

```za
t = table(| "df -h", map(.parse_only true))
println t.pp
```

## 40. Disk and filesystem checks

```za
t = disk_usage()
bad = t ?> `#.usage_percent > 90`
foreach r in bad
    println r.path, r.mounted_path, r.usage
endfor
```

## 41. Process and service inspection

Use system/process library calls where available; otherwise, ingest CLI output via `table()` where possible and operate structurally.

### 41.1 Process Monitoring

```za
# Get process list from /proc filesystem
proc_dirs = dir("/proc") ?> `#.name ~ "^[0-9]+$"` -> `#.name`
println "Found processes:", len(proc_dirs)

# Read process information
if len(proc_dirs) > 0
    first_pid = proc_dirs[0]
    stat_file = "/proc/" + first_pid + "/stat"
    if is_file(stat_file)
        stat_content = $in stat_file
        parts = split(stat_content, " ")
        if len(parts) > 1
            println "PID:", first_pid, "Process:", parts[1]
        endif
    endif
endif

# Filter processes by criteria
test_pids = proc_dirs[0:10]  # First 10 processes
filtered_pids = test_pids ?> `int(#) > 100`
println "High PIDs:", filtered_pids
```

### 41.2 Service Status via CLI

```za
# Parse service status using table()
service_output = ${systemctl list-units --type=service --state=running}
services = table(service_output, map(.parse_only true))

# Filter services by name
web_services = services ?> `#.0 ~ "nginx|apache|httpd"`
println "Web services:", web_services

# Check specific service status
nginx_status = ${systemctl is-active nginx}
if $st nginx_status == "active"
    println "Nginx is running"
else
    println "Nginx is not running"
endif
```

## 42. Network diagnostics

Za provides network helpers for common tasks (reachability, DNS, port checks). Prefer structured results over parsing external tool output.

### 42.1 Basic Network Testing

```za
# Test connectivity using ping
ping_result = ${ping -c 1 8.8.8.8}
if ping_result ~ "1 received"
    println "Internet connectivity OK"
else
    println "Internet connectivity failed"
endif

# DNS resolution test
dns_result = ${nslookup google.com}
if dns_result ~ "Address:"
    println "DNS resolution working"
else
    println "DNS resolution failed"
endif
```

### 42.2 Port Checking

```za
# Check if ports are open using netcat
def check_port(host, port)
    result = ${nc -z {host} {port} 2>&1}
    return result.len() == 0  # Empty output means port is open
end

# Test multiple ports
ports_to_check = [80, 443, 22, 3306]
for port in ports_to_check
    if check_port("localhost", port)
        println "Port", port, "is open"
    else
        println "Port", port, "is closed"
    endif
endfor
```

## 43. Parallel host probing

Use async fan-out and deterministic collection:

```za
def check(h)
    return icmp_ping(h, 2)
end

var handles map
foreach h in hosts
    async handles check(h) h
endfor
res = await(ref handles, true)
println res.pp
```

## 44. Drift detection and set-based reasoning

Represent key sets as maps and use set operators/predicates:

```za
changed = before ^ after
on changed.len > 0 do println changed.pp
```

---

# Part XII - Logging

## 45. Logging Overview

Za provides a unified logging system designed for operational monitoring and debugging. The system supports both application logging and web access logging through a single, coherent infrastructure that handles background processing, rotation, and multiple output formats.

### 45.1 Logging Philosophy

The logging system is built around several key principles:

- **Non-blocking operations**: All logging happens in the background to avoid impacting script performance
- **Unified architecture**: Both application logs and web access logs share the same processing pipeline
- **Graceful degradation**: Under memory pressure, the system prioritizes critical logs and drops less important ones
- **Flexible output**: Support for both plain text and structured JSON formatting
- **Automatic rotation**: Size-based log rotation with configurable retention policies

The basic logging configuration provides:

- **File output**: Logs are written to the specified file with automatic rotation
- **Console echo**: By default, log entries also appear on console (can be suppressed)
- **Timestamp handling**: Automatic timestamps added to all entries
- **Subject prefixes**: Optional prefixes to identify log sources

## 46. Logging Configuration

### 46.1 Format Control

Za supports both plain text and JSON logging formats:

```za
# Enable JSON formatting for structured logs
logging json on
logging subject "WEBMON"

# Add custom fields to all JSON entries
logging json fields +service "web-monitor"
logging json fields +version "1.2.1"

# Use plain text for human-readable logs
logging json off
```

JSON logging provides structured data that's easier to parse and analyze:

```json
// Plain text output
2023-10-15 14:23:11 [WEBMON] Service started on 15 Oct 23 14:23:11 +0000

// JSON output
{"timestamp":"2023-10-15T14:23:11Z","level":"INFO","subject":"WEBMON","message":"Service started on 15 Oct 23 14:23:11 +0000","service":"web-monitor","version":"1.2.1"}
```

### 46.2 Web Access Logging

For scripts that use Za's built-in web server, you can enable separate access logging:

```za
# Enable web access logging
logging web enable

# Set custom access log location
logging accessfile "/var/log/za_access.log"

# Configure web-specific settings
logging web enable
log "Web server started on port: ", port
```

Web access logs capture HTTP requests, response codes, and client information with automatic status code categorization (3xx=WARNING, 4xx/5xx=ERROR).

### 46.3 Rotation and Resource Management

Configure log rotation to manage disk space:

```za
# Rotate when files reach 10MB
logging rotate size 10485760

# Keep 5 rotated files
logging rotate count 5

# Set memory reserve for critical logs (1MB)
logging reserve 1048576
```

The rotation system automatically:

- Rotates files when they exceed the size threshold
- Maintains a configurable number of historical files
- Cleans up old files beyond the retention limit
- Uses atomic writes to prevent log corruption

### 46.4 Performance and Monitoring

Monitor logging system performance:

```za
# Check logging system status
logging status

# View detailed statistics
stats = logging_stats()
println "Queue usage: ", stats.queue_usage, "%"
println "Processed: ", stats.total_processed, " entries"
```

The background queue system provides:

- **Configurable queue size**: Default 60 entries, adjustable for high-volume scenarios
- **Overflow handling**: Automatic warnings when queue is full
- **Priority processing**: Errors get priority over normal logs
- **Memory awareness**: Graceful degradation under memory pressure


## 47. Logging Architecture

Za's logging system provides a comprehensive infrastructure for both application events and web access logging. The system is designed around non-blocking operations, unified processing, and graceful resource management to ensure reliable logging without impacting script performance.

## 47.1 Logging Statement Types

The logging system supports several categories of statements for different logging needs. Application Logging uses the primary log statement which writes to both log file and console by default, with support for level-specific logging.

Basic Control statements enable and disable logging, configure output paths, and control console echo behaviour.

Format Control allows switching between plain text and JSON formats, managing custom fields for structured logs.

Web Access Logging provides separate controls for HTTP request logging with configurable file locations and automatic status code categorization.

Advanced Features include subject prefixes, automatic error logging, queue management, rotation control, and memory reservation.

```za
```
Application Logging:

```za
log "message",x,y,z             # Primary logging statement
log level: "message",x,y,z      # Level-specific logging
```

Basic Control:

```za
logging on [filepath]           # Enable main logging, optionally set log file path
logging off                     # Disable main logging
logging status                  # Display comprehensive logging configuration and statistics
logging quiet                   # Suppress console output from log statements
logging loud                    # Enable console output from log statements (default)
```

Format Control:

```za
logging json on                     # Enable JSON format for all logs
logging json off                    # Use plain text format (default)
logging json fields +field value    # Add custom field to JSON logs
logging json fields -field          # Remove specific field from JSON logs
logging json fields -               # Clear all custom fields
logging json fields push            # Save current fields to stack
logging json fields pop             # Restore fields from stack
```

Web Access Logging:

```za
logging web enable                  # Enable web access logging
logging web disable                 # Disable web access logging
logging accessfile <path>           # Set web access log file location (default: ./za_access.log)
```

Advanced Features:

```za
logging subject <text>              # Set prefix for all log entries
logging error on/off                # Enable/disable automatic error logging
logging queue size <number>         # Set background processing queue size (default: 60)
logging rotate size <bytes>         # Set log rotation file size threshold
logging rotate count <number>       # Set number of rotated files to keep
logging reserve <bytes>             # Set emergency memory reserve for logging under pressure
```

## 47.2 Infrastructure Design

The logging architecture uses a unified architecture where both application code and web server code feed into a common background queue that processes entries through shared formatting and rotation pipelines.

Key Components include background queue processing with configurable size and overflow handling, dual destinations for main logs and web access logs, and format management supporting both plain text and JSON with custom field capabilities.

Format Management handles automatic timestamps and subject prefix handling while allowing custom field manipulation for structured logs. Log Rotation provides size-based rotation for both main and web access logs with configurable file count retention and automatic cleanup of old rotated files.

Memory Management includes an emergency memory reserve system and priority-based queue management that favours errors over normal logs under pressure. Error integration automatically logs Za interpreter errors with enhanced context and HTTP status code tracking for web access logs.

## 47.3 Performance Characteristics

The logging system is optimized for minimal performance impact through non-blocking I/O for all logging operations, ensuring script execution never waits on log writes.

The system implements memory-aware request dropping for web access logs under memory pressure, with automatic warnings when queue capacity is exceeded.

Statistics tracking provides comprehensive monitoring of logging system performance and health. Cross-platform path validation and security ensures safe file operations with appropriate permission checks and path sanitization across different operating systems.

This design ensures that logging operations provide comprehensive coverage of application events while maintaining high performance and reliability.


---

# Part XIII - Testing

## 48. Testing Overview

Za provides a built-in testing framework designed for both development verification and operational validation. The testing system supports assertions, documentation integration, and flexible execution modes that make it suitable for everything from unit tests to operational checks.

### 48.1 Testing Philosophy

The testing framework follows these principles:

- **Integrated testing**: Tests are part of the source code, not separate files
- **Documentation coupling**: Tests can include embedded documentation that appears in test output
- **Flexible assertions**: Support for both value assertions and error handling validation
- **Group organization**: Tests can be grouped for selective execution and reporting
- **Production safety**: Test blocks are ignored during normal script execution

### 48.2 Test Execution Modes

```bash
# Run all tests
za -t script

# Run specific test groups
za -t -G "database" script

# Run with custom output file
za -t -o "test_results.txt" script

# Override group assertion failure action]
za -t -O "fail|continue" script
```

### 48.3 Test Structure

Tests use a simple structure:

```za
test "test_name" GROUP "group_name" [ASSERT FAIL|CONTINUE]
    # Test setup code
    assert condition [, custom_message ]
    # Additional assertions
    doc "This test verifies that..."
    # Test execution code
endtest
```

The optional assertion mode controls test behaviour:
- **ASSERT FAIL** (default): Test execution stops on first assertion failure
- **ASSERT CONTINUE**: Test continues past assertion failures, reporting all failures

## 49. Test Blocks

### 49.1 Basic Test Structure

```za
test "integer_addition" GROUP "math_basics"
    # Test basic arithmetic
    result = 2 + 3
    assert result == 5, "2 + 3 should equal 5"

    # Test with different values
    assert (10 + 15) == 25, "10 + 15 should equal 25"
    doc "Verifies basic integer addition operations"
endtest
```

### 49.2 Error Handling Tests

```za
test "file_error_handling" GROUP "io_operations" ASSERT CONTINUE
    # Test file not found error
    try
        content = read_file("/nonexistent/file.txt")
    catch err
        println "error type : ",err.pp
    endtry

    # Test permission error handling
    try
        write_file("/root/protected.txt", "test")
    catch err
        println "error type : ",err.pp
    endtry

    doc "Tests file operation error detection and categorization"
endtest
```

### 49.3 Function Return Value Tests

```za
test "function_returns" GROUP "function_validation"
    # Test multiple return values
    def compute_stats(a, b)
        sum = a + b
        diff = a - b
        return sum, diff
    end

    result_sum, result_diff = compute_stats(10, 3)
    assert result_sum == 13
    assert result_diff == 7

    # Test single return value unpacking
    values = compute_stats(5, 2)
    assert values == [7, 3]

    doc "Validates function return value handling"
endtest
```

### 49.4 Data Structure Tests

```za
test "map_operations" GROUP "data_structures"
    # Test map creation and access
    config = map(.host "localhost", .port 5432, .ssl true)
    assert config.host == "localhost"
    assert config.port == 5432
    assert config.ssl == true

    # Test map as set operations
    set_a = map(.a 1, .b 2, .c 3)
    set_b = map(.b 2, .c 3, .d 4)

    intersection = set_a & set_b
    assert intersection.len == 2
    assert intersection.c == 2
    assert intersection.b == 2

    doc "Tests map literal syntax and set operations"
endtest
```

### 49.5 Integration and System Tests

```za
test "system_integration" GROUP "integration"
    # Test system call integration
    result =| "echo 'test output'"
    assert result.okay
    assert result.out ~ "test output"

    # Test table parsing
    df_data = table(${echo 'Filesystem 1K-blocks Used Available Use% Mounted on'},
                   map(.parse_only true)
    )
    assert df_data.len == 1
    assert df_data[0].Filesystem == "Filesystem"

    doc "Validates integration with system commands and data parsing"
endtest
```

## 50. Test Behaviours and Best Practices

### 50.1 Test Organization

Group tests logically by functionality:

```za
test "user_auth_valid" GROUP "authentication"
test "user_auth_invalid" GROUP "authentication"
test "user_permission_check" GROUP "authorization"
test "database_connection" GROUP "database"
test "database_query" GROUP "database"
```

This organization allows:

- **Selective execution**: Run specific groups with `-G` flag
- **Clear reporting**: Test output organized by functional area
- **Maintenance**: Easy to locate and update related tests

### 50.2 Error Handling in Tests

The `ASSERT ERROR` syntax handles function call failures gracefully:

```za
test "robust_function_calls" GROUP "error_handling"
    # This continues execution even if connect() fails
    assert error connect("invalid_host")
    doc "Tests error handling with ASSERT ERROR syntax"
endtest
```

### 50.3 Documentation Integration

Use `doc` statements to provide test context:

```za
test "complex_business_logic" GROUP "business_rules"
    # Setup complex scenario
    customer = create_test_customer()
    order = process_order(customer, test_items)

    # Document the test purpose
    doc "Verifies that order processing correctly applies business rules:
         1. Customer discount applied correctly
         2. Tax calculations accurate
         3. Inventory updated appropriately"

    # Assertions for each rule
    assert order.discount_applied
    assert order.tax_amount > 0
    assert inventory_updated(order.items)
endtest
```

### Expanded DOC statement usage

The DOC statement is also used to generate HEREDOC content in both normal execution and test modes. Some example use cases below:

#### Basic DOC with VAR clause

```za
doc var myvar "Hello World"
println myvar
```

#### DOC with GEN clause (default delimiter)

```za
doc gen
These lines should be
captured in test mode.
Yes?
```

#### DOC with GEN and custom DELIM clause

```za
doc gen delim TERMINAL
These [#2]delimited[#-] lines should
be captured in test mode.
TERMINAL
```

#### DOC with custom DELIM and VAR clauses

```za
doc delim END var multiline
This is a multi-line
string with "quotes"
END
print multiline
```

#### DOC with GEN clause and variable interpolation

```za
doc gen
These lines should be
captured in test mode.
abc value is {=abc}
Yes?
```

### Key Features Demonstrated

- VAR clause: Stores content in variable (myvar, multiline)
- GEN clause: Enables test mode documentation capture
- DELIM clause: Custom terminators (TERMINAL, END)
- Variable interpolation: {=abc} substitution in GEN mode
- Multi-line content: Using custom delimiters for block text
- Default delimiter: "\n\n" when no DELIM specified


---

# Appendix A — Operator reference (summary)

- Arithmetic: `+ - * / % **`
- Assignment: `= += -= *= /= %=`
- Boolean: `and or not`, `&& || !`
- Bitwise: `& | ^ << >>`
- Set (maps): `| & - ^`
- Range: `..`
- Regex: `~ ~i ~f`
- Map/filter: `-> ?>`
- Path unary: `$pa $pp $pb $pn $pe`
- String unary: `$uc $lc $lt $rt $st`
- File read unary: `$in`
- File write: `$out`
- Shell: `|` and `=|`

# Appendix B — Keywords (summary)

`if else endif`
`for foreach endfor`
`case is has contains or endcase`
`while endwhile`
`def end return`
`try catch then endtry`
`struct endstruct`
`enum`
`module use namespace`
`test endtest assert doc`
`async var pause |`
`debug continue break exit`
`print println log logging`
`cls at pane input prompt `
`on do`

# Appendix C — Built-in constants

`true`, `false`, `nil`, `NaN`

# Appendix D — Standard Library Categories

Za’s standard library is implemented in the interpreter source as a set of built-in calls. These library calls do not require any module imports.

This appendix lists the calls **by category**. For each category, all function names are listed, followed by a short “commonly used” section.

> This appendix intentionally does not repeat full per-function documentation, because Za can generate function reference pages automatically and the REPL supports `help` and `func(...)` lookups.


## array

**Functions (23):**


argmax, argmin, concatenate, det, det_big, find, flatten, identity, inverse, inverse_big, mean, median, ones, prod, rank, reshape, squeeze, stack, std, trace, variance, where, zeros


**Commonly used (from examples/tests):**


- find
- where
- inverse_big
- det_big
- argmax
- mean
- identity
- argmin
- stack
- inverse
- concatenate
- prod


## conversion

**Functions (35):**


as_bigf, as_bigi, as_bool, as_float, as_int, as_int64, as_string, as_uint, asc, base64d, base64e, btoi, byte, char, dtoo, explain, f2n, is_number, itob, json_decode, json_format, json_query, kind, m2s, maxfloat, maxint, maxuint, md2ansi, otod, pp, read_struct, s2m, table, to_typed, write_struct


**Commonly used (from examples/tests):**


- as_float
- pp
- as_int
- as_string
- kind
- json_query
- char
- to_typed
- table
- btoi
- as_bool
- is_number


## cron

**Functions (4):**


cron_next, cron_parse, cron_validate, quartz_to_cron


**Commonly used (from examples/tests):**


- cron_parse
- quartz_to_cron
- cron_next
- cron_validate


## date

**Functions (17):**


date, date_human, epoch_nano_time, epoch_time, format_date, format_time, time_diff, time_dom, time_dow, time_hours, time_minutes, time_month, time_nanos, time_seconds, time_year, time_zone, time_zone_offset


**Commonly used (from examples/tests):**


- epoch_nano_time
- epoch_time
- time_diff
- date
- date_human
- time_seconds
- time_minutes
- time_hours
- time_year
- time_nanos
- time_month
- time_dow


## db

**Functions (3):**


db_close, db_init, db_query


**Commonly used (from examples/tests):**


- db_query
- db_init
- db_close


## error

**Functions (15):**


error_call_chain, error_call_stack, error_default_handler, error_emergency_exit, error_extend, error_filename, error_global_variables, error_local_variables, error_message, error_source_context, error_source_line_numbers, error_source_location, error_style, log_exception, log_exception_with_stack


**Commonly used (from examples/tests):**


- error_extend
- error_emergency_exit
- error_source_location
- error_source_context
- error_message
- error_local_variables
- error_global_variables
- error_call_stack
- error_call_chain
- error_style
- error_source_line_numbers


## file (unix)

**Functions (17):**


fclose, feof, fflush, file_mode, file_size, flock, fopen, fread, fseek, ftell, fwrite, is_dir, is_file, perms, read_file, stat, write_file


**Commonly used (from examples/tests):**


- is_file
- read_file
- write_file
- is_dir
- fopen
- fread
- fclose
- feof
- fwrite
- file_size
- stat
- fseek


## file (windows)

**Functions (17):**


fclose, feof, fflush, file_mode, file_size, flock, fopen, fread, fseek, ftell, fwrite, is_dir, is_file, perms, read_file, stat, write_file


**Commonly used (from examples/tests):**


- is_file
- read_file
- write_file
- is_dir
- fopen
- fread
- fclose
- feof
- fwrite
- file_size
- stat
- fseek


## html

**Functions (22):**


wa, wbody, wdiv, wh1, wh2, wh3, wh4, wh5, whead, wimg, wli, wlink, wol, wp, wpage, wtable, wtbody, wtd, wth, wthead, wtr, wul


**Commonly used (from examples/tests):**


- wdiv
- wtr
- wthead
- wth
- wtd
- wtable
- wpage
- wlink
- wimg
- whead
- wbody
- wa


## image

**Functions (22):**


svg_circle, svg_def, svg_def_end, svg_desc, svg_ellipse, svg_end, svg_grid, svg_group, svg_group_end, svg_image, svg_line, svg_link, svg_link_end, svg_plot, svg_polygon, svg_polyline, svg_rect, svg_roundrect, svg_square, svg_start, svg_text, svg_title


**Commonly used (from examples/tests):**


- svg_line
- svg_title
- svg_text
- svg_start
- svg_square
- svg_roundrect
- svg_rect
- svg_polyline
- svg_polygon
- svg_plot
- svg_link_end
- svg_link


## internal

**Functions (105):**


ansi, argc, argv, array_colours, array_format, ast, await, bash_versinfo, bash_version, capture_shell, clear_line, clktck, cmd_version, conclear, conread, conset, conwrite, coproc, cursoroff, cursoron, cursorx, difference, dinfo, dump, dup, echo, enum_all, enum_names, eval, exception_strictness, exec, execpath, expect, exreg, feed, format_stack_trace, func_categories, func_descriptions, func_inputs, func_outputs, funcref, funcs, gdump, get_col, get_cores, get_mem, get_row, has_colour, has_shell, has_term, home, hostname, interpol, interpolate, intersect, is_disjoint, is_subset, is_superset, key, keypress, lang, last, last_err, len, local, log_queue_status, logging_stats, mdump, merge, os, pane_c, pane_h, pane_r, pane_w, panic, permit, pid, powershell_version, ppid, release_id, release_name, release_version, rlen, set_depth, shell_pid, sizeof, suppress_prompt, symmetric_difference, system, sysvar, term, term_h, term_w, thisfunc, thisref, tokens, trap, unmap, user, utf8supported, varbind, wininfo, winterm, zainfo, zsh_version


**Commonly used (from examples/tests):**


- len
- term_w
- term_h
- exreg
- cursoroff
- keypress
- cursoron
- key
- interpol
- permit
- pid
- execpath


## list

**Functions (35):**


alltrue, anytrue, append, append_to, avg, col, concat, empty, eqlen, esplit, fieldsort, head, insert, list_bigf, list_bigi, list_bool, list_fill, list_float, list_int, list_int64, list_string, max, min, msplit, peek, pop, push_front, remove, scan_left, sort, ssort, sum, tail, uniq, zip


**Commonly used (from examples/tests):**


- append
- append_to
- sum
- min
- sort
- max
- avg
- list_int
- fieldsort
- col
- tail
- remove


## math

**Functions (38):**


abs, acos, acosh, asin, asinh, atan, atanh, cos, cosh, deg2rad, dot, e, floor, ibase, ln, ln10, ln2, log10, log2, logn, matmul, numcomma, phi, pi, pow, prec, rad2deg, rand, randf, round, seed, sin, sinh, tan, tanh, transpose, ubin8, uhex32


**Commonly used (from examples/tests):**


- rand
- pi
- seed
- abs
- sin
- randf
- cos
- numcomma
- logn
- e
- deg2rad
- transpose


## network

**Functions (31):**


dns_resolve, has_privileges, http_benchmark, http_headers, icmp_ping, icmp_traceroute, net_interfaces_detailed, netstat, netstat_established, netstat_interface, netstat_listen, netstat_process, netstat_protocol, netstat_protocol_info, netstat_protocols, network_stats, open_files, port_scan, ssl_cert_install_help, ssl_cert_validate, tcp_available, tcp_client, tcp_close, tcp_ping, tcp_receive, tcp_send, tcp_server, tcp_server_accept, tcp_server_stop, tcp_traceroute, traceroute


**Commonly used (from examples/tests):**


- dns_resolve
- http_headers
- tcp_ping
- port_scan
- traceroute
- tcp_server_stop
- tcp_server
- tcp_send
- tcp_close
- tcp_client
- tcp_available
- ssl_cert_validate


## notify

**Functions (7):**


ev_event, ev_exists, ev_mask, ev_watch, ev_watch_add, ev_watch_close, ev_watch_remove


**Commonly used (from examples/tests):**


- ev_mask
- ev_watch_close
- ev_watch
- ev_exists
- ev_event


## os

**Functions (37):**


can_read, can_write, cd, chroot, copy, cwd, delete, dir, env, fileabs, filebase, get_env, glob, group_add, group_del, group_info, group_list, group_membership, group_mod, groupname, is_device, is_pipe, is_setgid, is_setuid, is_socket, is_sticky, is_symlink, parent, rename, set_env, umask, user_add, user_del, user_info, user_list, user_mod, username


**Commonly used (from examples/tests):**


- dir
- set_env
- get_env
- delete
- env
- username
- parent
- groupname
- cwd
- can_read
- is_symlink
- is_socket


## package

**Functions (5):**


install, is_installed, service, uninstall, vcmp


**Commonly used (from examples/tests):**


- is_installed
- install


## pcre

**Functions (3):**


reg_filter, reg_match, reg_replace


**Commonly used (from examples/tests):**


- reg_replace
- reg_filter
- reg_match


## smtp

**Functions (13):**


email_add_header, email_base64_decode, email_base64_encode, email_extract_addresses, email_get_attachments, email_get_body, email_parse_headers, email_process_template, email_remove_header, email_validate, smtp_send, smtp_send_with_attachments, smtp_send_with_auth


**Commonly used (from examples/tests):**


- email_parse_headers
- email_get_body
- email_get_attachments
- smtp_send
- email_validate
- email_remove_header
- email_process_template
- email_extract_addresses
- email_base64_encode
- email_base64_decode
- email_add_header


## string

**Functions (56):**


addansi, bg256, bgrgb, ccformat, clean, collapse, count, fg256, fgrgb, field, fields, filter, format, get_value, grep, gsub, has_end, has_start, inset, is_utf8, join, keys, levdist, line_add, line_add_after, line_add_before, line_delete, line_filter, line_head, line_match, line_replace, line_tail, lines, literal, log_sanitise, lower, match, next_match, pad, pos, replace, reverse, rvalid, sanitisation, split, stripansi, stripcc, stripquotes, strpos, substr, tr, trim, upper, values, wrap, wrap_text


**Commonly used (from examples/tests):**


- format
- tr
- pad
- replace
- join
- split
- field
- match
- count
- fields
- substr
- fgrgb


## sum

**Functions (5):**


md5sum, s3sum, sha1sum, sha224sum, sha256sum


**Commonly used:** (no occurrences found in `eg/` or `za_tests/` for this category in the uploaded tree)


## system

**Functions (23):**


cpu_info, debug_cpu_files, dio, disk_usage, gw_address, gw_info, gw_interface, iodiff, mem_info, mount_info, net_devices, nio, ps_info, ps_list, ps_map, ps_tree, resource_usage, sys_load, sys_resources, top_cpu, top_dio, top_mem, top_nio


**Commonly used (from examples/tests):**


- sys_resources
- gw_interface
- gw_info
- gw_address
- cpu_info
- ps_list
- ps_info
- nio
- net_devices
- mem_info
- disk_usage


## tui

**Functions (16):**


editor, tui, tui_box, tui_clear, tui_input, tui_menu, selector, tui_new, tui_new_style, tui_pager, tui_progress, tui_progress_reset, tui_radio, tui_screen, tui_table, tui_template, tui_text


**Commonly used (from examples/tests):**


- tui_clear
- tui_screen
- tui_table
- tui_text
- tui_template
- tui_radio
- tui_progress_reset
- tui_progress
- tui_pager
- tui_menu
- tui_input


## web

**Functions (28):**


download, html_escape, html_unescape, net_interfaces, web_cache_cleanup_interval, web_cache_enable, web_cache_max_age, web_cache_max_memory, web_cache_max_size, web_cache_purge, web_cache_stats, web_custom, web_display, web_download, web_get, web_gzip_enable, web_head, web_max_clients, web_post, web_raw_send, web_serve_decode, web_serve_log, web_serve_log_throttle, web_serve_path, web_serve_start, web_serve_stop, web_serve_up, web_template


**Commonly used (from examples/tests):**


- web_serve_path
- web_serve_start
- web_serve_stop
- web_get
- web_serve_up
- web_serve_decode
- web_serve_log_throttle
- web_gzip_enable
- web_display
- web_custom
- web_cache_stats
- web_cache_max_size


## yaml

**Functions (5):**


yaml_delete, yaml_get, yaml_marshal, yaml_parse, yaml_set


**Commonly used (from examples/tests):**


- yaml_parse
- yaml_marshal


## zip

**Functions (7):**


zip_add, zip_create, zip_create_from_dir, zip_extract, zip_extract_file, zip_list, zip_remove


**Commonly used (from examples/tests):**


- zip_create
- zip_list
- zip_extract
- zip_remove
- zip_extract_file
- zip_add

<div style="page-break-after: always;"></div>

# Appendix E — Worked Example: `eg/mon` (annotated)

This appendix is an annotated walkthrough of the shipped example script `eg/mon`. All claims below are grounded in the script itself, with line references.

## E.1 What `eg/mon` is

The file identifies itself as a test/diagnostic script: it describes itself as a "Test script for za" and says it displays "key system resource information" in a summary view (lines 3–9).

## E.2 Core drawing primitives

Before it gathers any system information, the script defines a small set of terminal-drawing helpers.

`clear(lstart,lend,column)` clears a range of lines by calling `clear_line` in a counted loop (lines 15–19).

`header(t)` draws a coloured underline across the pane width, then prints a title at the top (lines 21–28). You can see it iterating from 0 to `pane_w()-2` and printing a coloured underscore (lines 23–25).

### Vertical and horizontal bars

The script implements its own bar widgets using block characters.

- `vbar(...)` draws a vertical bar for a percentage value, with optional label rendering (lines 30–71). It computes unit size `us = vsize / 100f` and uses that to derive the filled height (lines 45–48).

- `chart(...)` draws a series of vertical bars by calling `vbar` for each value in `series` (lines 74–82). Notice that it derives a per-bar colour by substituting into a template string and evaluating it (line 76).

- `bar(...)` draws a horizontal progress bar using partial block characters for quarter steps (lines 84–102).

## E.3 Small utility helpers

Several short helpers support later panes:

- `interface_ip(ip_in)` scans `net_devices()` and returns the first IP address of the matching interface name (lines 104–109).

- `negreg(inp,matcher)` filters out lines that match a pattern using `continue if match(...)` (lines 111–118).

- `shorten(s,l)` truncates long strings and uses an ellipsis if UTF‑8 is supported (lines 126–129).

## E.4 Built-in unit test embedded in the script

`eg/mon` includes a small unit test section:

- `logging testfile "mon.test.out"` sets a test log file (line 120).

- A `test ... et` block named `"fn_ip"` asserts that the loopback interface IP starts with `127.0.0.` (lines 121–124).

## E.5 Environment pane (`showEnv`)

`showEnv()` selects the `envs` pane, redraws its line colour, prints a header, clears a region, and prints system facts such as hostname, user, OS, locale, and distribution information (lines 131–149).

A small OS-specific clause is shown via `case os()` where it prints the bash version only for Linux (lines 145–148).

## E.6 Files/inodes pane (`showFiles`)

`showFiles()` is guarded to avoid Windows terminal mode: it runs only when `!winterm()` (lines 155–185).

It reads Linux kernel counters directly from `/proc` using `$in` (lines 160–162), then parses them using string helpers such as `field`, `tr`, and `as_float` (lines 171–183).

If either `/proc/sys/fs/file-nr` or `/proc/sys/fs/inode-nr` could not be read, it returns early (line 163).

## E.7 Memory pane (`showMem`)

`showMem()` shows two important things:

First, it gathers memory information from **built-in calls**: it calls `mem_info()` (line 216) and `sys_resources()` (line 252), then pulls named fields such as `MemoryTotal`, `MemoryFree`, `MemoryCached`, `SwapFree`, and `SwapTotal` (lines 253–259).

Second, it optionally computes "slab" usage details when it has privileges (`access`) (lines 210–212, 290–313). It iterates `mem_detailed.Slab` and derives MB sizes from object counts and sizes (lines 224–228), then sorts via `fieldsort` (line 237).

For display, it uses `mdisplay(...)` which draws a bar and prints a compact size using `smallprint` (lines 189–199, 270–281).

It also displays Za’s own memory usage via `get_mem().alloc` and `get_mem().system` (lines 283–287).

## E.8 Process pane (`showProcs`)

`showProcs(ct, uptime)` calls `ps_list(map(.include_cmdline true))` to obtain process info (line 324).

It builds `proc_list` as a map keyed by PID, storing a small array of derived metrics including computed CPU percentages (lines 326–352).

To present the "top" entries, it serialises rows into a string (`shellout`), sorts with `fieldsort(..., "n", true)` and takes lines (`lines(":17")`) before applying `uniq` (lines 358–365).

The display loop uses `fields(p," ")` and the resulting `F[...]` fields for formatted printing. It supports filtering with a regex against `proc_filter` (lines 374–387).

## E.9 CPU pane (`showCpu`)

`showCpu(...)` pulls per-core usage from `cpu_info().Usage["cores"]` (line 424) and uses `prev`/`diff` maps to compute deltas over time (lines 427–451).

If the script is still in its initial warm-up period (`sample_start_in-->0`), it prints a message and returns the updated counter (lines 453–457).

Core names are sorted alphanumerically (`cpuinfo.keys.sort(map(.alphanumeric true))`, line 460).

The pane can show:
- detailed per-activity counters (lines 472–481),
- a coloured bar row built from repeated activity glyphs (lines 483–497),
- and an optional totals column (lines 499–506).

## E.10 Disk pane (`showHdd`)

`showHdd()` uses `disk_usage()` directly (line 521). It then filters out unwanted devices using regex conditions under a `case os()` block and continues early for skipped entries (lines 534–542).

It assembles a sortable list of anonymous structs containing `usage_pct` from the `usage_percent` field and the mounted path (lines 543–552).

It sorts with `ssort(sorted_disks, "usage_pct", false)` (line 555) and prints a limited number of rows (lines 557–582).

Sizes are formatted using the local `hobbitsize(...)` helper (lines 399–404) and strings are truncated with `shorten(...)` (lines 126–129, 576–579).

## E.11 Network pane (`showNet`)

`showNet(...)` selects the network pane, prints the chosen interface and its IP, then scans `nio()` for that interface (lines 586–598).

It keeps previous byte counters (`prev_rbytes`, `prev_tbytes`) and maintains history lists `rblist` and `tblist` by shifting and appending new deltas (lines 610–619).

The charts are plotted by mapping the history list through an expression string using `->` and then converting to integers with `.list_int` (lines 628–629). The expression clamps values with `[0f:100f]` inside the string.

Finally it prints averaged RX/TX rates using `humansize(...)` scaled by the sampling timeout (lines 635–641).

## E.12 Pane layout (`redef_layout`)

`redef_layout(cpu_count,pfilter)` defines the pane grid using repeated `pane define` calls (lines 647–659).

It also initialises the network history lists to a fixed length (`net_sample_count = 40`, lines 660–668) and draws an origin line for the network chart using a UTF‑8 glyph when supported (lines 672–681).

## E.13 Main loop and key handling

The script exits early if there is no output channel (`term_h()==-1`, line 690).

It sets up timing (`key_timeout=1000`, lines 695–697), determines the gateway interface (`gw_interface()`, line 701), and detects privilege level (`access=has_privileges()`, line 716).

It computes CPU tick rate via `clktck()` and aborts if unavailable (lines 706–709).

Panes are created by calling `redef_layout(cpu_count,pfilter)` after discovering core count (`get_cores()`, lines 721–728).

Inside the main `while !quit` loop (line 752), it redraws on window changes, captures uptime (`sys_resources().Uptime`, line 765), and calls the pane renderers (lines 768–774).

The bottom status line prints a human date (`date_human(...)`) and frame time based on `epoch_nano_time()` deltas (lines 783–795).

User input is handled with `keypress(key_timeout)` and a `case char(k)` dispatch (lines 798–834). It supports changing interface (`i`), process filter (`f`), timeout (`t`), toggling CPU sections (`D`, `B`, `T`), help (`h`), quit (`q`), and redraw on Ctrl‑L (`k==12`) (lines 800–834).

On exit it restores terminal state and turns the cursor back on (lines 841–844).

