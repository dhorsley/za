" za syntax file - Neovim compatible version
" Language:     za (za)
" Maintainer:   Daniel Horsley  <dhorsley@gmail.com>
" Last Change:  Dec 10, 2025
" Version:      11 (Shebang detection)

" Shebang detection function
function! s:DetectZaShebang()
  " Only check if syntax is not already set to za_neovim
  if &syntax == 'za'
    return 1
  endif

  " Check if first line contains za shebang (various forms)
  let first_line = getline(1)
  if first_line =~ '^#!\s*/usr/bin/za\>' ||
   \ first_line =~ '^#!\s*/usr/bin/env\s\+za\>' ||
   \ first_line =~ '^#!\s*/usr/local/bin/za\>'
    set syntax=za
    set filetype=za
    return 1
  endif
  return 0
endfunction

" Version control
if exists("b:current_syntax")
  finish
endif

" Clear existing syntax
syntax clear

" Set up the syntax-highlighting iskeyword for modern Vim/Neovim
" Remove the old patch check - modern versions handle this correctly
setlocal iskeyword+=-

" Background detection (Neovim compatible)
set background&


" Operators: {{{1
" ==========
syn match   zaOperator    "<<\|>>"        contained
syn match   zaOperator    "[!&;|]"        contained
syn match   zaOperator    "\[[[^:]\|\]]"  contained

" Misc: {{{1
"======
syn match   zaWrapLineOperator "\\$"
syn match   zaColon   '^\s*\zs:'

" String And Character Constants: {{{1
"================================
syn match zaInteger   "\<[-+]\=\d\+\([Ee]\=\d*\)\>" contained
syn match zaFloat     "\<[-+]\=\d\+[\.]\=\d*\([Ee][-+]\=\d\+\)\=[f]\=\>" contained

" Comments: {{{1
"==========
syn match   zaComment     "^\s*\zs#.*$"   contains=@Spell,@CommentGroup
syn match   zaComment     "\s\zs#.*$"     contains=@Spell,@CommentGroup
syn match   zaComment     "^\s*\zs//.*$"  contains=@Spell,@CommentGroup
syn match   zaComment     "\s\zs//.*$"    contains=@Spell,@CommentGroup

" Identifiers: {{{1
"=============
syn match   zaIdentifiers           "\<[[:alpha:]_][[:alnum:]_\.]*\>"
syn keyword zaAssignStatements    var setglob input nextgroup=zaVarLHS skipwhite


" Functions: {{{1
" ==========

" Time functions
syntax match zaTimeFunctions "\(^|.\|\s*\)date\s*("he=e-1
syntax match zaTimeFunctions "\(^|.\|\s*\)epoch_time\s*("he=e-1
syntax match zaTimeFunctions "\(^|.\|\s*\)epoch_nano_time\s*("he=e-1
syntax match zaTimeFunctions "\(^|.\|\s*\)time_diff\s*("he=e-1
syntax match zaTimeFunctions "\(^|.\|\s*\)date_human\s*("he=e-1
syntax match zaTimeFunctions "\(^|.\|\s*\)time_year\s*("he=e-1
syntax match zaTimeFunctions "\(^|.\|\s*\)time_month\s*("he=e-1
syntax match zaTimeFunctions "\(^|.\|\s*\)time_dom\s*("he=e-1
syntax match zaTimeFunctions "\(^|.\|\s*\)time_dow\s*("he=e-1
syntax match zaTimeFunctions "\(^|.\|\s*\)time_hours\s*("he=e-1
syntax match zaTimeFunctions "\(^|.\|\s*\)time_minutes\s*("he=e-1
syntax match zaTimeFunctions "\(^|.\|\s*\)time_seconds\s*("he=e-1
syntax match zaTimeFunctions "\(^|.\|\s*\)time_nanos\s*("he=e-1
syntax match zaTimeFunctions "\(^|.\|\s*\)time_zone\s*("he=e-1
syntax match zaTimeFunctions "\(^|.\|\s*\)time_zone_offset\s*("he=e-1

" List functions
syntax match zaListFunctions "\(^|.\|\s*\)empty\s*("he=e-1
syntax match zaListFunctions "\(^|.\|\s*\)col\s*("he=e-1
syntax match zaListFunctions "\(^|.\|\s*\)fieldsort\s*("he=e-1
syntax match zaListFunctions "\(^|.\|\s*\)head\s*("he=e-1
syntax match zaListFunctions "\(^|.\|\s*\)tail\s*("he=e-1
syntax match zaListFunctions "\(^|.\|\s*\)uniq\s*("he=e-1
syntax match zaListFunctions "\(^|.\|\s*\)append\s*("he=e-1
syntax match zaListFunctions "\(^|.\|\s*\)append_to\s*("he=e-1
syntax match zaListFunctions "\(^|.\|\s*\)insert\s*("he=e-1
syntax match zaListFunctions "\(^|.\|\s*\)remove\s*("he=e-1
syntax match zaListFunctions "\(^|.\|\s*\)push_front\s*("he=e-1
syntax match zaListFunctions "\(^|.\|\s*\)pop\s*("he=e-1
syntax match zaListFunctions "\(^|.\|\s*\)sort\s*("he=e-1
syntax match zaListFunctions "\(^|.\|\s*\)ssort\s*("he=e-1
syntax match zaListFunctions "\(^|.\|\s*\)peek\s*("he=e-1
syntax match zaListFunctions "\(^|.\|\s*\)any\s*("he=e-1
syntax match zaListFunctions "\(^|.\|\s*\)all\s*("he=e-1
syntax match zaListFunctions "\(^|.\|\s*\)esplit\s*("he=e-1
syntax match zaListFunctions "\(^|.\|\s*\)sum\s*("he=e-1
syntax match zaListFunctions "\(^|.\|\s*\)min\s*("he=e-1
syntax match zaListFunctions "\(^|.\|\s*\)max\s*("he=e-1
syntax match zaListFunctions "\(^|.\|\s*\)avg\s*("he=e-1
syntax match zaListFunctions "\(^|.\|\s*\)zip\s*("he=e-1
syntax match zaListFunctions "\(^|.\|\s*\)scan_left\s*("he=e-1
syntax match zaListFunctions "\(^|.\|\s*\)eqlen\s*("he=e-1
syntax match zaListFunctions "\(^|.\|\s*\)list_fill\s*("he=e-1


" Conversion functions
syntax match zaConversionFunctions "\(^|.\|\s*\)md2ansi\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)f2n\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)byte\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)as_int64\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)as_bigi\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)as_bigf\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)as_int\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)as_uint\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)as_bool\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)as_float\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)as_string\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)kind\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)is_number\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)char\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)asc\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)list_float\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)list_string\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)list_int\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)list_bool\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)list_bigi\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)list_bigf\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)local\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)base64e\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)base64d\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)json_decode\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)json_format\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)json_query\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)write_struct\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)read_struct\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)btoi\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)itob\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)dtoo\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)otod\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)s2m\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)m2s\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)maxint\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)maxuint\s*("he=e-1
syntax match zaConversionFunctions "\(^|.\|\s*\)maxfloat\s*("he=e-1


" Internal functions (first batch)
syntax match zaInternalFunctions "\(^|.\|\s*\)ast\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)dinfo\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)feed\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)sizeof\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)sysvar\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)varbind\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)set_depth\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)execpath\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)last\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)last_err\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)zsh_version\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)bash_version\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)bash_versinfo\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)user\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)os\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)home\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)lang\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)release_name\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)release_version\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)release_id\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)has_shell\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)has_colour\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)has_term\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)term\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)shell_pid\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)winterm\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)wininfo\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)hostname\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)argv\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)argc\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)dump\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)gdump\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)mdump\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)exec\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)eval\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)keypress\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)tokens\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)clear_line\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)key\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)clktck\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)funcref\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)thisfunc\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)thisref\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)pid\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)ppid\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)cursoron\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)cursoroff\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)cursorx\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)term_h\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)term_w\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)pane_h\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)pane_w\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)pane_r\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)pane_c\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)utf8supported\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)system\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)ansi\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)interpol\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)interpolate\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)echo\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)get_row\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)get_col\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)unmap\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)coproc\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)capture_shell\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)await\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)get_mem\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)get_cores\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)funcs\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)func_inputs\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)func_outputs\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)func_descriptions\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)func_categories\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)zainfo\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)wrap\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)permit\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)enum_names\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)enum_all\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)dup\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)expect\s*("he=e-1
syntax match zaInternalFunctions "\(^|.\|\s*\)trap\s*("he=e-1


" TUI functions
syntax match zaTuiFunctions   "\(^|.\|\s*\)tui\s*("he=e-1
syntax match zaTuiFunctions   "\(^|.\|\s*\)editor\s*("he=e-1
syntax match zaTuiFunctions   "\(^|.\|\s*\)tui_new_style\s*("he=e-1
syntax match zaTuiFunctions   "\(^|.\|\s*\)tui_new\s*("he=e-1
syntax match zaTuiFunctions   "\(^|.\|\s*\)tui_template\s*("he=e-1
syntax match zaTuiFunctions   "\(^|.\|\s*\)tui_clear\s*("he=e-1
syntax match zaTuiFunctions   "\(^|.\|\s*\)tui_box\s*("he=e-1
syntax match zaTuiFunctions   "\(^|.\|\s*\)tui_progress_reset\s*("he=e-1
syntax match zaTuiFunctions   "\(^|.\|\s*\)tui_menu\s*("he=e-1
syntax match zaTuiFunctions   "\(^|.\|\s*\)tui_table\s*("he=e-1
syntax match zaTuiFunctions   "\(^|.\|\s*\)tui_pager\s*("he=e-1
syntax match zaTuiFunctions   "\(^|.\|\s*\)tui_progress\s*("he=e-1
syntax match zaTuiFunctions   "\(^|.\|\s*\)tui_screen\s*("he=e-1
syntax match zaTuiFunctions   "\(^|.\|\s*\)tui_radio\s*("he=e-1
syntax match zaTuiFunctions   "\(^|.\|\s*\)tui_text\s*("he=e-1
syntax match zaTuiFunctions   "\(^|.\|\s*\)tui_input\s*("he=e-1

" Image functions
syntax match zaImageFunctions "\(^|.\|\s*\)svg_start\s*("he=e-1
syntax match zaImageFunctions "\(^|.\|\s*\)svg_end\s*("he=e-1
syntax match zaImageFunctions "\(^|.\|\s*\)svg_title\s*("he=e-1
syntax match zaImageFunctions "\(^|.\|\s*\)svg_desc\s*("he=e-1
syntax match zaImageFunctions "\(^|.\|\s*\)svg_plot\s*("he=e-1
syntax match zaImageFunctions "\(^|.\|\s*\)svg_circle\s*("he=e-1
syntax match zaImageFunctions "\(^|.\|\s*\)svg_ellipse\s*("he=e-1
syntax match zaImageFunctions "\(^|.\|\s*\)svg_rect\s*("he=e-1
syntax match zaImageFunctions "\(^|.\|\s*\)svg_square\s*("he=e-1
syntax match zaImageFunctions "\(^|.\|\s*\)svg_roundrect\s*("he=e-1
syntax match zaImageFunctions "\(^|.\|\s*\)svg_grid\s*("he=e-1
syntax match zaImageFunctions "\(^|.\|\s*\)svg_line\s*("he=e-1
syntax match zaImageFunctions "\(^|.\|\s*\)svg_polyline\s*("he=e-1
syntax match zaImageFunctions "\(^|.\|\s*\)svg_polygon\s*("he=e-1
syntax match zaImageFunctions "\(^|.\|\s*\)svg_text\s*("he=e-1
syntax match zaImageFunctions "\(^|.\|\s*\)svg_image\s*("he=e-1
syntax match zaImageFunctions "\(^|.\|\s*\)svg_def\s*("he=e-1
syntax match zaImageFunctions "\(^|.\|\s*\)svg_def_end\s*("he=e-1
syntax match zaImageFunctions "\(^|.\|\s*\)svg_link\s*("he=e-1
syntax match zaImageFunctions "\(^|.\|\s*\)svg_link_end\s*("he=e-1
syntax match zaImageFunctions "\(^|.\|\s*\)svg_group\s*("he=e-1
syntax match zaImageFunctions "\(^|.\|\s*\)svg_group_end\s*("he=e-1


" Package functions
syntax match zaPackageFunctions "\(^|.\|\s*\)uninstall\s*("he=e-1
syntax match zaPackageFunctions "\(^|.\|\s*\)is_installed\s*("he=e-1
syntax match zaPackageFunctions "\(^|.\|\s*\)install\s*("he=e-1
syntax match zaPackageFunctions "\(^|.\|\s*\)service\s*("he=e-1
syntax match zaPackageFunctions "\(^|.\|\s*\)vcmp\s*("he=e-1

" Math functions
syntax match zaMathFunctions "\(^|.\|\s*\)seed\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)rand\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)randf\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)pow\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)sin\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)cos\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)tan\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)asin\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)acos\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)atan\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)sinh\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)cosh\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)tanh\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)asinh\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)acosh\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)atanh\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)ln\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)logn\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)log2\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)log10\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)round\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)rad2deg\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)deg2rad\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)pi\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)phi\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)e\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)ln2\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)ln10\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)numcomma\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)ubin8\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)uhex32\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)abs\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)floor\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)ibase\s*("he=e-1
syntax match zaMathFunctions "\(^|.\|\s*\)prec\s*("he=e-1

" File functions
syntax match zaFileFunctions "\(^|.\|\s*\)file_mode\s*("he=e-1
syntax match zaFileFunctions "\(^|.\|\s*\)file_size\s*("he=e-1
syntax match zaFileFunctions "\(^|.\|\s*\)read_file\s*("he=e-1
syntax match zaFileFunctions "\(^|.\|\s*\)write_file\s*("he=e-1
syntax match zaFileFunctions "\(^|.\|\s*\)is_file\s*("he=e-1
syntax match zaFileFunctions "\(^|.\|\s*\)is_dir\s*("he=e-1
syntax match zaFileFunctions "\(^|.\|\s*\)perms\s*("he=e-1
syntax match zaFileFunctions "\(^|.\|\s*\)fopen\s*("he=e-1
syntax match zaFileFunctions "\(^|.\|\s*\)fclose\s*("he=e-1
syntax match zaFileFunctions "\(^|.\|\s*\)fseek\s*("he=e-1
syntax match zaFileFunctions "\(^|.\|\s*\)fread\s*("he=e-1
syntax match zaFileFunctions "\(^|.\|\s*\)fwrite\s*("he=e-1
syntax match zaFileFunctions "\(^|.\|\s*\)feof\s*("he=e-1
syntax match zaFileFunctions "\(^|.\|\s*\)ftell\s*("he=e-1
syntax match zaFileFunctions "\(^|.\|\s*\)fflush\s*("he=e-1
syntax match zaFileFunctions "\(^|.\|\s*\)stat\s*("he=e-1
syntax match zaFileFunctions "\(^|.\|\s*\)flock\s*("he=e-1


" Web functions
syntax match zaWebFunctions "\(^|.\|\s*\)download\s*("he=e-1
syntax match zaWebFunctions "\(^|.\|\s*\)web_download\s*("he=e-1
syntax match zaWebFunctions "\(^|.\|\s*\)web_custom\s*("he=e-1
syntax match zaWebFunctions "\(^|.\|\s*\)web_max_clients\s*("he=e-1
syntax match zaWebFunctions "\(^|.\|\s*\)web_get\s*("he=e-1
syntax match zaWebFunctions "\(^|.\|\s*\)web_head\s*("he=e-1
syntax match zaWebFunctions "\(^|.\|\s*\)web_post\s*("he=e-1
syntax match zaWebFunctions "\(^|.\|\s*\)web_display\s*("he=e-1
syntax match zaWebFunctions "\(^|.\|\s*\)web_serve_start\s*("he=e-1
syntax match zaWebFunctions "\(^|.\|\s*\)web_serve_stop\s*("he=e-1
syntax match zaWebFunctions "\(^|.\|\s*\)web_serve_up\s*("he=e-1
syntax match zaWebFunctions "\(^|.\|\s*\)web_serve_path\s*("he=e-1
syntax match zaWebFunctions "\(^|.\|\s*\)web_serve_log\s*("he=e-1
syntax match zaWebFunctions "\(^|.\|\s*\)web_serve_log_throttle\s*("he=e-1
syntax match zaWebFunctions "\(^|.\|\s*\)web_serve_decode\s*("he=e-1
syntax match zaWebFunctions "\(^|.\|\s*\)net_interfaces\s*("he=e-1
syntax match zaWebFunctions "\(^|.\|\s*\)html_escape\s*("he=e-1
syntax match zaWebFunctions "\(^|.\|\s*\)html_unescape\s*("he=e-1

" Database functions
syntax match zaDbFunctions "\(^|.\|\s\*\)db_init\s*("he=e-1
syntax match zaDbFunctions "\(^|.\|\s\*\)db_query\s*("he=e-1
syntax match zaDbFunctions "\(^|.\|\s\*\)db_close\s*("he=e-1

" String functions
syntax match zaStringFunctions "\(^|.\|\s*\)clean\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)next_match\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)stripquotes\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)stripcc\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)addansi\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)stripansi\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)pad\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)len\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)field\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)fields\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)get_value\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)has_start\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)has_end\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)reg_match\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)reg_filter\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)reg_replace\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)match\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)filter\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)line_match\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)line_filter\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)rvalid\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)grep\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)split\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)join\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)collapse\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)substr\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)gsub\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)replace\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)trim\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)lines\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)count\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)line_head\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)line_tail\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)line_add\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)line_delete\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)line_replace\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)line_add_before\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)line_add_after\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)reverse\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)tr\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)lower\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)upper\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)format\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)ccformat\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)strpos\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)pos\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)inset\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)bg256\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)fg256\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)bgrgb\s*("he=e-1
syntax match zaStringFunctions "\(^|.\|\s*\)fgrgb\s*("he=e-1


" OS functions
syntax match zaOsFunctions "\(^|.\|\s*\)env\s*("he=e-1
syntax match zaOsFunctions "\(^|.\|\s*\)get_env\s*("he=e-1
syntax match zaOsFunctions "\(^|.\|\s*\)set_env\s*("he=e-1
syntax match zaOsFunctions "\(^|.\|\s*\)cd\s*("he=e-1
syntax match zaOsFunctions "\(^|.\|\s*\)cwd\s*("he=e-1
syntax match zaOsFunctions "\(^|.\|\s*\)dir\s*("he=e-1
syntax match zaOsFunctions "\(^|.\|\s*\)umask\s*("he=e-1
syntax match zaOsFunctions "\(^|.\|\s*\)chroot\s*("he=e-1
syntax match zaOsFunctions "\(^|.\|\s*\)delete\s*("he=e-1
syntax match zaOsFunctions "\(^|.\|\s*\)rename\s*("he=e-1
syntax match zaOsFunctions "\(^|.\|\s*\)copy\s*("he=e-1
syntax match zaOsFunctions "\(^|.\|\s*\)can_read\s*("he=e-1
syntax match zaOsFunctions "\(^|.\|\s*\)can_write\s*("he=e-1
syntax match zaOsFunctions "\(^|.\|\s*\)parent\s*("he=e-1
syntax match zaOsFunctions "\(^|.\|\s*\)is_symlink\s*("he=e-1
syntax match zaOsFunctions "\(^|.\|\s*\)is_device\s*("he=e-1
syntax match zaOsFunctions "\(^|.\|\s*\)is_pipe\s*("he=e-1
syntax match zaOsFunctions "\(^|.\|\s*\)is_socket\s*("he=e-1
syntax match zaOsFunctions "\(^|.\|\s*\)is_sticky\s*("he=e-1
syntax match zaOsFunctions "\(^|.\|\s*\)is_setuid\s*("he=e-1
syntax match zaOsFunctions "\(^|.\|\s*\)is_setgid\s*("he=e-1
syntax match zaOsFunctions "\(^|.\|\s*\)groupname\s*("he=e-1
syntax match zaOsFunctions "\(^|.\|\s*\)username\s*("he=e-1

" HTML functions
syntax match zaHtmlFunctions "\(^|.\|\s*\)wpage\s*("he=e-1
syntax match zaHtmlFunctions "\(^|.\|\s*\)wbody\s*("he=e-1
syntax match zaHtmlFunctions "\(^|.\|\s*\)wdiv\s*("he=e-1
syntax match zaHtmlFunctions "\(^|.\|\s*\)wa\s*("he=e-1
syntax match zaHtmlFunctions "\(^|.\|\s*\)wimg\s*("he=e-1
syntax match zaHtmlFunctions "\(^|.\|\s*\)whead\s*("he=e-1
syntax match zaHtmlFunctions "\(^|.\|\s*\)wlink\s*("he=e-1
syntax match zaHtmlFunctions "\(^|.\|\s*\)wp\s*("he=e-1
syntax match zaHtmlFunctions "\(^|.\|\s*\)wtable\s*("he=e-1
syntax match zaHtmlFunctions "\(^|.\|\s*\)wthead\s*("he=e-1
syntax match zaHtmlFunctions "\(^|.\|\s*\)wtbody\s*("he=e-1
syntax match zaHtmlFunctions "\(^|.\|\s*\)wtr\s*("he=e-1
syntax match zaHtmlFunctions "\(^|.\|\s*\)wth\s*("he=e-1
syntax match zaHtmlFunctions "\(^|.\|\s*\)wtd\s*("he=e-1
syntax match zaHtmlFunctions "\(^|.\|\s*\)wul\s*("he=e-1
syntax match zaHtmlFunctions "\(^|.\|\s*\)wol\s*("he=e-1
syntax match zaHtmlFunctions "\(^|.\|\s*\)wli\s*("he=e-1
syntax match zaHtmlFunctions "\(^|.\|\s*\)wh1\s*("he=e-1
syntax match zaHtmlFunctions "\(^|.\|\s*\)wh2\s*("he=e-1
syntax match zaHtmlFunctions "\(^|.\|\s*\)wh3\s*("he=e-1
syntax match zaHtmlFunctions "\(^|.\|\s*\)wh4\s*("he=e-1
syntax match zaHtmlFunctions "\(^|.\|\s*\)wh5\s*("he=e-1

" Notify functions
syntax match zaNotifyFunctions "\(^|.\|\s*\)ev_watch\s*("he=e-1
syntax match zaNotifyFunctions "\(^|.\|\s*\)ev_watch_close\s*("he=e-1
syntax match zaNotifyFunctions "\(^|.\|\s*\)ev_watch_add\s*("he=e-1
syntax match zaNotifyFunctions "\(^|.\|\s*\)ev_watch_remove\s*("he=e-1
syntax match zaNotifyFunctions "\(^|.\|\s*\)ev_exists\s*("he=e-1
syntax match zaNotifyFunctions "\(^|.\|\s*\)ev_event\s*("he=e-1
syntax match zaNotifyFunctions "\(^|.\|\s*\)ev_mask\s*("he=e-1

" Sum functions
syntax match zaSumFunctions "\(^|.\|\s*\)md5sum\s*("he=e-1
syntax match zaSumFunctions "\(^|.\|\s*\)sha1sum\s*("he=e-1
syntax match zaSumFunctions "\(^|.\|\s*\)sha224sum\s*("he=e-1
syntax match zaSumFunctions "\(^|.\|\s*\)sha256sum\s*("he=e-1
syntax match zaSumFunctions "\(^|.\|\s*\)s3sum\s*("he=e-1


" YAML functions
syntax match zaYamlFunctions "\(^|.\|\s*\)yaml_delete\s*("he=e-1
syntax match zaYamlFunctions "\(^|.\|\s*\)yaml_get\s*("he=e-1
syntax match zaYamlFunctions "\(^|.\|\s*\)yaml_marshal\s*("he=e-1
syntax match zaYamlFunctions "\(^|.\|\s*\)yaml_parse\s*("he=e-1
syntax match zaYamlFunctions "\(^|.\|\s*\)yaml_set\s*("he=e-1

" Zip functions
syntax match zaZipFunctions "\(^|.\|\s*\)zip_add\s*("he=e-1
syntax match zaZipFunctions "\(^|.\|\s*\)zip_create\s*("he=e-1
syntax match zaZipFunctions "\(^|.\|\s*\)zip_create_from_dir\s*("he=e-1
syntax match zaZipFunctions "\(^|.\|\s*\)zip_extract\s*("he=e-1
syntax match zaZipFunctions "\(^|.\|\s*\)zip_extract_file\s*("he=e-1
syntax match zaZipFunctions "\(^|.\|\s*\)zip_list\s*("he=e-1
syntax match zaZipFunctions "\(^|.\|\s*\)zip_remove\s*("he=e-1

" Za Keywords: {{{1
" ==============

syntax match zaTestStatements "\(^\|\s\+\)\(doc\|test\|et\|endtest\|assert\)\($\|\s\+\)"
syntax match zaStatements '\(^\|\s\+\)|\($\|\s\+\)'
syntax match zaStatements "\( do \| to \| as \| in \| is \)"
syntax match zaStatements "\(^\|\s\+\)\(on\|or\|if\|at\|ef\|et\|ec\|ei\|ew\|es\)\($\|\s\+\)"
syntax match zaStatements "\(^\|\s\+\)\(end\|def\|fix\|has\|and\|not\|nop\|var\|log\|cls\|web\)\($\|\s\+\)"
syntax match zaStatements "\(^\|\s\+\)\(else\|step\|pane\|enum\|init\|help\|with\|case\|hist\|exit\)\($\|\s\+\)"
syntax match zaStatements "\(^\|\s\+\)\(struct\|pause\|debug\|async\|print\|break\|endif\|while\|quiet\)\($\|\s\+\)"
syntax match zaStatements "\(^\|\s\+\)\(resume\|module\|prompt\|return\|define\|enddef\|enable\)\($\|\s\+\)"
syntax match zaStatements "\(^\|\s\+\)\(version\|require\|println\|showdef\|endwith\|endcase\|logging\|subject\|disable\)\($\|\s\+\)"
syntax match zaStatements "\(^\|\s\+\)\(contains\|endwhile\|continue\)\($\|\s\+\)"
syntax match zaStatements "\(^\|\s\+\)\(endstruct\)\($\|\s\+\)"
syntax match zaStatements "\(^\|\s\+\)\(accessfile\|showstruct\)\($\|\s\+\)"

syntax match zaForStatements "\(^\|\s\+\)\(foreach\|for\|ef\|endfor\)\($\|\s\+\)"

syntax match zaTypes "\sany\(\s\|$\)"
syntax match zaTypes "\sint\(\s\|$\)"
syntax match zaTypes "\suint\(\s\|$\)"
syntax match zaTypes "\sbool\(\s\|$\)"
syntax match zaTypes "\sfloat\(\s\|$\)"
syntax match zaTypes "\sstring\(\s\|$\)"
syntax match zaTypes "\smap\(\s\|$\)"
syntax match zaTypes "\sarray\(\s\|$\)"


" Color Matching {{{1
" ===============
syntax match zaColourB0 "\[#b0\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote
syntax match zaColourB1 "\[#b1\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote
syntax match zaColourB2 "\[#b2\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote
syntax match zaColourB3 "\[#b3\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote
syntax match zaColourB4 "\[#b4\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote
syntax match zaColourB5 "\[#b5\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote
syntax match zaColourB6 "\[#b6\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote
syntax match zaColourB7 "\[#b7\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote
syntax match zaColourB0 "\[#bblack\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote
syntax match zaColourB1 "\[#bblue\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote
syntax match zaColourB2 "\[#bred\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote
syntax match zaColourB3 "\[#bmagenta\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote
syntax match zaColourB4 "\[#bgreen\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote
syntax match zaColourB5 "\[#bcyan\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote
syntax match zaColourB6 "\[#byellow\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote
syntax match zaColourB7 "\[#bwhite\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote

syntax match zaColourF0 "\[#0\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote
syntax match zaColourF1 "\[#1\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote
syntax match zaColourF2 "\[#2\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote
syntax match zaColourF3 "\[#3\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote
syntax match zaColourF4 "\[#4\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote
syntax match zaColourF5 "\[#5\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote
syntax match zaColourF6 "\[#6\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote
syntax match zaColourF7 "\[#7\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote
syntax match zaColourF0 "\[#fblack\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote
syntax match zaColourF1 "\[#fblue\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote
syntax match zaColourF2 "\[#fred\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote
syntax match zaColourF3 "\[#fmagenta\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote
syntax match zaColourF4 "\[#fgreen\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote
syntax match zaColourF5 "\[#fcyan\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote
syntax match zaColourF6 "\[#fyellow\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote
syntax match zaColourF7 "\[#fwhite\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote

syntax match zaColourNormal "\[##\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote
syntax match zaColourNormal "\[#-\]"hs=s+1,he=e-1 containedin=zaDoubleQuote,zaBacktkQuote

" Quoting: {{{1
" ========
syn match   zaSpecial    display contained "\\\(x\x\+\|\o\{1,3}\|.\|$\)" containedin=zaDoubleQuote,zaBacktkQuote
syn region  zaDoubleQuote   start=+L\="+ skip=+\\\\\|\\"+ end=+"+ extend
syn region  zaBacktkQuote   start=+L\=`+ skip=+\\\\\|\\`+ end=+`+ extend


" Clusters: contains=@... clusters: {{{1
"==================================
syn cluster zaFunctions       contains=zaListFunctions,zaTimeFunctions,zaConversionFunctions,zaInternalFunctions,zaOsFunctions,zaPackageFunctions,zaMathFunctions,zaFileFunctions,zaWebFunctions,zaDbFunctions,zaStringFunctions,zaImageFunctions,zaHtmlFunctions,zaNotifyFunctions,zaSumFunctions,zaYamlFunctions,zaZipFunctions,zaTuiFunctions
syn cluster zaArithParenList  contains=zaFloat,zaInteger,zaOperator,zaSpecial,zaIdentifiers,@zaFunctions

" Arithmetic Parenthesized Expressions: {{{1
" =====================================
syn region zaParen start='[^$]\zs(\%(\ze[^(]\|$\)' end=')' contains=@zaArithParenList

" Synchronization: {{{1
" ================
if !exists("g:za_minlines")
  let g:za_minlines = 400
endif
if !exists("g:za_maxlines")
  let g:za_maxlines = 2 * g:za_minlines
endif
exec "syn sync minlines=" . g:za_minlines . " maxlines=" . g:za_maxlines


" Default Highlighting: {{{1
" =====================

" Define the links
hi def link zaIdentifiers       Identifier
hi def link zaTypes        Type

hi def link zaColon   Comment
hi def link zaDoubleQuote String
hi def link zaBacktkQuote String
hi def link zaWrapLineOperator    Operator
hi def link zaNotifyFunctions Function
hi def link zaTimeFunctions Function
hi def link zaListFunctions Function
hi def link zaConversionFunctions Function
hi def link zaInternalFunctions Function
hi def link zaOsFunctions Function
hi def link zaPackageFunctions Function
hi def link zaMathFunctions Function
hi def link zaFileFunctions Function
hi def link zaWebFunctions Function
hi def link zaDbFunctions Function
hi def link zaStringFunctions Function
hi def link zaHtmlFunctions Function
hi def link zaImageFunctions Function
hi def link zaSumFunctions Function
hi def link zaYamlFunctions Function
hi def link zaZipFunctions Function
hi def link zaTuiFunctions Function

if !exists("g:za_no_error")
 hi def link CondError      Error
 hi def link CaseError      Error
 hi def link IfError        Error
 hi def link InError        Error
endif

hi def link zaOperator            Operator
hi def link zaAssignStatements    zaStatements
hi def link zaFloat               Number
hi def link zaInteger             Number

" Neovim-compatible color definitions
" Support both GUI and terminal colors

" Basic text
hi Normal       ctermfg=blue ctermbg=NONE guifg=#0000ff guibg=NONE

" Comments
hi zaComment      ctermfg=red guifg=#ff0000

" Constants and types
hi Constant     ctermfg=darkGreen cterm=bold guifg=#006400 guibg=NONE gui=bold
hi zaTypes        ctermfg=magenta guifg=#ff00ff

" Statements and keywords
hi zaStatements   ctermfg=darkBlue guifg=#00008b
hi zaTestStatements  ctermfg=magenta guifg=#ff00ff
hi zaForStatements  ctermfg=lightBlue guifg=#add8e6

" Identifiers
hi zaIdentifiers  ctermfg=darkYellow guifg=#b8860b

" Error and warning messages
hi zaErrorMsg     ctermfg=black ctermbg=red guifg=#000000 guibg=#ff0000
hi zaWarningMsg   ctermfg=black ctermbg=green guifg=#000000 guibg=#00ff00

" Matching and UI elements
hi MatchParen   ctermbg=blue ctermfg=yellow guibg=#0000ff guifg=#ffff00
hi zaError        ctermbg=red guibg=#ff0000
hi Function      ctermfg=Gray cterm=italic guifg=#808080 gui=italic

" Search and navigation
hi Search       ctermbg=darkGray ctermfg=blue guibg=#a9a9a9 guifg=#0000ff
hi LineNr       ctermfg=blue guifg=#0000ff
hi title        ctermfg=darkGray guifg=#a9a9a9

" Status line
hi StatusLineNC ctermfg=lightBlue ctermbg=darkBlue guifg=#add8e6 guibg=#00008b
hi StatusLine   cterm=bold ctermfg=cyan ctermbg=blue guifg=#00ffff guibg=#0000ff gui=bold

" Visual selection
hi clear Visual
hi Visual       term=reverse cterm=reverse guibg=#808080

" Diff highlighting
hi DiffChange   ctermbg=darkGreen guibg=#006400
hi diffOnly ctermfg=red cterm=bold guifg=#ff0000 gui=bold

" Background colors for za color codes
hi zaColourB0    ctermbg=black ctermfg=white guibg=#000000 guifg=#ffffff
hi zaColourB1    ctermbg=blue ctermfg=white guibg=#0000ff guifg=#ffffff
hi zaColourB2    ctermbg=red ctermfg=white guibg=#ff0000 guifg=#ffffff
hi zaColourB3    ctermbg=magenta ctermfg=white guibg=#ff00ff guifg=#ffffff
hi zaColourB4    ctermbg=green ctermfg=white guibg=#00ff00 guifg=#ffffff
hi zaColourB5    ctermbg=cyan ctermfg=black guibg=#00ffff guifg=#000000
hi zaColourB6    ctermbg=yellow ctermfg=black guibg=#ffff00 guifg=#000000
hi zaColourB7    ctermbg=gray ctermfg=black guibg=#808080 guifg=#000000

" Foreground colors for za color codes
hi zaColourF0    ctermfg=darkGray ctermbg=black guifg=#a9a9a9 guibg=#000000
hi zaColourF1    ctermfg=blue ctermbg=black guifg=#0000ff guibg=#000000
hi zaColourF2    ctermfg=red ctermbg=black guifg=#ff0000 guibg=#000000
hi zaColourF3    ctermfg=magenta ctermbg=black guifg=#ff00ff guibg=#000000
hi zaColourF4    ctermfg=green ctermbg=black guifg=#00ff00 guibg=#000000
hi zaColourF5    ctermfg=cyan ctermbg=black guifg=#00ffff guibg=#000000
hi zaColourF6    ctermfg=yellow ctermbg=black guifg=#ffff00 guibg=#000000
hi zaColourF7    ctermfg=white ctermbg=black guifg=#ffffff guibg=#000000

hi zaColourNormal ctermfg=white ctermbg=darkGreen guifg=#ffffff guibg=#006400

" Numbers and strings
hi Number      ctermfg=lightBlue guifg=#add8e6
hi String      ctermfg=green guifg=#00ff00

" Language detection
let b:current_syntax = "za"

" Autocommand for shebang detection
if !exists("g:za_no_autodetect")
  augroup ZaShebangDetection
    autocmd! BufRead * call s:DetectZaShebang()
    autocmd! BufNewFile * call s:DetectZaShebang()
  augroup END
endif

