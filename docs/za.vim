" za syntax file
" Language:     za (za)
" Maintainer:   Daniel Horsley  <dhorsley@gmail.com>
" Last Change:  Jan 20, 2021
" Version:      9

" Version control

syntax clear
if exists("b:current_syntax")
  finish
endif

let s:shell = "za"

" set up the syntax-highlighting iskeyword
if has("patch-7.4.1141")
 exe "syn iskeyword ".&iskeyword.",-"
endif

set bg&

" Error Codes: {{{1
" ============


" Operators: {{{1
" ==========
syn match   Operator    "<<\|>>"        contained
syn match   Operator    "[!&;|]"        contained
syn match   Operator    "\[[[^:]\|\]]"  contained


" Misc: {{{1
"======
syn match   WrapLineOperator "\\$"

syn match   Colon   '^\s*\zs:'


" String And Character Constants: {{{1
"================================
syn match Integer   "\<[-+]\=\d\+\([Ee]\=\d*\)\>"
syn match Float     "\<[-+]\=\d\+[\.]\=\d*\([Ee][-+]\=\d\+\)\=[f]\=\>"


" Comments: {{{1
"==========
syn match   Comment     "^\s*\zs#.*$"   contains=@CommentGroup
syn match   Comment     "\s\zs#.*$"     contains=@CommentGroup
syn match   Comment     "^\s*\zs//.*$"  contains=@CommentGroup
syn match   Comment     "\s\zs//.*$"    contains=@CommentGroup

" Identifiers: {{{1
"=============
syn match   idents           "\<[[:alpha:]_][[:alnum:]_\.]*\>"
syn keyword assignStatements    var setglob input nextgroup=folVarLHS skipwhite

" Functions: {{{1
" ==========

syntax match time_functions "\(^|.\|\s*\)date\s*("he=e-1
syntax match time_functions "\(^|.\|\s*\)epoch_time\s*("he=e-1
syntax match time_functions "\(^|.\|\s*\)epoch_nano_time\s*("he=e-1
syntax match time_functions "\(^|.\|\s*\)time_diff\s*("he=e-1
syntax match time_functions "\(^|.\|\s*\)date_human\s*("he=e-1
syntax match time_functions "\(^|.\|\s*\)time_year\s*("he=e-1
syntax match time_functions "\(^|.\|\s*\)time_month\s*("he=e-1
syntax match time_functions "\(^|.\|\s*\)time_dom\s*("he=e-1
syntax match time_functions "\(^|.\|\s*\)time_dow\s*("he=e-1
syntax match time_functions "\(^|.\|\s*\)time_hours\s*("he=e-1
syntax match time_functions "\(^|.\|\s*\)time_minutes\s*("he=e-1
syntax match time_functions "\(^|.\|\s*\)time_seconds\s*("he=e-1
syntax match time_functions "\(^|.\|\s*\)time_nanos\s*("he=e-1

syntax match list_functions "\(^|.\|\s*\)empty\s*("he=e-1
syntax match list_functions "\(^|.\|\s*\)similar\s*("he=e-1
syntax match list_functions "\(^|.\|\s*\)col\s*("he=e-1
syntax match list_functions "\(^|.\|\s*\)fieldsort\s*("he=e-1
syntax match list_functions "\(^|.\|\s*\)numcomp\s*("he=e-1
syntax match list_functions "\(^|.\|\s*\)head\s*("he=e-1
syntax match list_functions "\(^|.\|\s*\)tail\s*("he=e-1
syntax match list_functions "\(^|.\|\s*\)uniq\s*("he=e-1
syntax match list_functions "\(^|.\|\s*\)append\s*("he=e-1
syntax match list_functions "\(^|.\|\s*\)append_to\s*("he=e-1
syntax match list_functions "\(^|.\|\s*\)insert\s*("he=e-1
syntax match list_functions "\(^|.\|\s*\)remove\s*("he=e-1
syntax match list_functions "\(^|.\|\s*\)push_front\s*("he=e-1
syntax match list_functions "\(^|.\|\s*\)pop\s*("he=e-1
syntax match list_functions "\(^|.\|\s*\)sort\s*("he=e-1
syntax match list_functions "\(^|.\|\s*\)peek\s*("he=e-1
syntax match list_functions "\(^|.\|\s*\)any\s*("he=e-1
syntax match list_functions "\(^|.\|\s*\)all\s*("he=e-1
syntax match list_functions "\(^|.\|\s*\)concat\s*("he=e-1
syntax match list_functions "\(^|.\|\s*\)esplit\s*("he=e-1
syntax match list_functions "\(^|.\|\s*\)sum\s*("he=e-1
syntax match list_functions "\(^|.\|\s*\)min\s*("he=e-1
syntax match list_functions "\(^|.\|\s*\)max\s*("he=e-1
syntax match list_functions "\(^|.\|\s*\)avg\s*("he=e-1
syntax match list_functions "\(^|.\|\s*\)zip\s*("he=e-1
syntax match list_functions "\(^|.\|\s*\)scan_left\s*("he=e-1
syntax match list_functions "\(^|.\|\s*\)eqlen\s*("he=e-1

syntax match conversion_functions "\(^|.\|\s*\)byte\s*("he=e-1
syntax match conversion_functions "\(^|.\|\s*\)as_int64\s*("he=e-1
syntax match conversion_functions "\(^|.\|\s*\)as_bigi\s*("he=e-1
syntax match conversion_functions "\(^|.\|\s*\)as_bigf\s*("he=e-1
syntax match conversion_functions "\(^|.\|\s*\)as_int\s*("he=e-1
syntax match conversion_functions "\(^|.\|\s*\)as_uint\s*("he=e-1
syntax match conversion_functions "\(^|.\|\s*\)as_bool\s*("he=e-1
syntax match conversion_functions "\(^|.\|\s*\)as_float\s*("he=e-1
syntax match conversion_functions "\(^|.\|\s*\)as_string\s*("he=e-1
syntax match conversion_functions "\(^|.\|\s*\)kind\s*("he=e-1
syntax match conversion_functions "\(^|.\|\s*\)is_number\s*("he=e-1 
syntax match conversion_functions "\(^|.\|\s*\)char\s*("he=e-1
syntax match conversion_functions "\(^|.\|\s*\)asc\s*("he=e-1
syntax match conversion_functions "\(^|.\|\s*\)list_float\s*("he=e-1
syntax match conversion_functions "\(^|.\|\s*\)list_string\s*("he=e-1
syntax match conversion_functions "\(^|.\|\s*\)list_int\s*("he=e-1
syntax match conversion_functions "\(^|.\|\s*\)list_bool\s*("he=e-1
syntax match conversion_functions "\(^|.\|\s*\)list_bigi\s*("he=e-1
syntax match conversion_functions "\(^|.\|\s*\)list_bigf\s*("he=e-1
syntax match conversion_functions "\(^|.\|\s*\)local\s*("he=e-1
syntax match conversion_functions "\(^|.\|\s*\)base64e\s*("he=e-1
syntax match conversion_functions "\(^|.\|\s*\)base64d\s*("he=e-1
syntax match conversion_functions "\(^|.\|\s*\)json_decode\s*("he=e-1
syntax match conversion_functions "\(^|.\|\s*\)json_format\s*("he=e-1
syntax match conversion_functions "\(^|.\|\s*\)json_query\s*("he=e-1
syntax match conversion_functions "\(^|.\|\s*\)write_struct\s*("he=e-1
syntax match conversion_functions "\(^|.\|\s*\)read_struct\s*("he=e-1
syntax match conversion_functions "\(^|.\|\s*\)btoi\s*("he=e-1
syntax match conversion_functions "\(^|.\|\s*\)itob\s*("he=e-1
syntax match conversion_functions "\(^|.\|\s*\)dtoo\s*("he=e-1
syntax match conversion_functions "\(^|.\|\s*\)otod\s*("he=e-1

syntax match internal_functions "\(^|.\|\s*\)execpath\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)last\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)last_out\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)zsh_version\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)bash_version\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)bash_versinfo\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)user\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)os\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)home\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)lang\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)release_name\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)release_version\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)release_id\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)has_shell\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)has_colour\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)has_term\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)shell_pid\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)winterm\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)wininfo\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)hostname\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)argv\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)argc\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)dump\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)exec\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)eval\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)keypress\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)tokens\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)glob_key\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)clear_line\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)key\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)clktck\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)glob_len\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)getglob\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)funcref\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)thisfunc\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)thisref\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)pid\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)ppid\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)commands\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)cursoron\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)cursoroff\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)cursorx\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)term_h\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)term_w\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)pane_h\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)pane_w\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)pane_r\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)pane_c\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)utf8supported\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)system\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)locks\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)echo\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)ansi\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)interpol\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)echo\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)get_row\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)get_col\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)unmap\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)coproc\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)capture_shell\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)await\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)mem_summary\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)get_mem\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)get_cores\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)funcs\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)func_inputs\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)func_outputs\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)func_descriptions\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)func_categories\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)zainfo\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)wrap\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)permit\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)enum_names\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)enum_all\s*("he=e-1
syntax match internal_functions "\(^|.\|\s*\)dup\s*("he=e-1

syntax match image_functions "\(^|.\|\s*\)svg_start\s*("he=e-1
syntax match image_functions "\(^|.\|\s*\)svg_end\s*("he=e-1
syntax match image_functions "\(^|.\|\s*\)svg_title\s*("he=e-1
syntax match image_functions "\(^|.\|\s*\)svg_desc\s*("he=e-1
syntax match image_functions "\(^|.\|\s*\)svg_plot\s*("he=e-1
syntax match image_functions "\(^|.\|\s*\)svg_circle\s*("he=e-1
syntax match image_functions "\(^|.\|\s*\)svg_ellipse\s*("he=e-1
syntax match image_functions "\(^|.\|\s*\)svg_rect\s*("he=e-1
syntax match image_functions "\(^|.\|\s*\)svg_square\s*("he=e-1
syntax match image_functions "\(^|.\|\s*\)svg_roundrect\s*("he=e-1
syntax match image_functions "\(^|.\|\s*\)svg_grid\s*("he=e-1
syntax match image_functions "\(^|.\|\s*\)svg_line\s*("he=e-1
syntax match image_functions "\(^|.\|\s*\)svg_polyline\s*("he=e-1
syntax match image_functions "\(^|.\|\s*\)svg_polygon\s*("he=e-1
syntax match image_functions "\(^|.\|\s*\)svg_text\s*("he=e-1
syntax match image_functions "\(^|.\|\s*\)svg_image\s*("he=e-1
syntax match image_functions "\(^|.\|\s*\)svg_def\s*("he=e-1
syntax match image_functions "\(^|.\|\s*\)svg_def_end\s*("he=e-1
syntax match image_functions "\(^|.\|\s*\)svg_link\s*("he=e-1
syntax match image_functions "\(^|.\|\s*\)svg_link_end\s*("he=e-1
syntax match image_functions "\(^|.\|\s*\)svg_group\s*("he=e-1
syntax match image_functions "\(^|.\|\s*\)svg_group_end\s*("he=e-1

syntax match package_functions "\(^|.\|\s*\)uninstall\s*("he=e-1
syntax match package_functions "\(^|.\|\s*\)is_installed\s*("he=e-1
syntax match package_functions "\(^|.\|\s*\)install\s*("he=e-1
syntax match package_functions "\(^|.\|\s*\)service\s*("he=e-1
syntax match package_functions "\(^|.\|\s*\)vcmp\s*("he=e-1

syntax match math_functions "\(^|.\|\s*\)seed\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)rand\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)randf\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)sqr\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)sqrt\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)pow\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)sin\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)cos\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)tan\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)asin\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)acos\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)atan\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)sinh\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)cosh\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)tanh\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)asinh\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)acosh\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)atanh\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)ln\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)logn\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)log2\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)log10\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)round\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)rad2deg\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)deg2rad\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)pi\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)phi\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)e\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)ln2\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)ln10\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)numcomma*\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)ubin8*\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)uhex32*\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)abs*\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)floor*\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)ibase*\s*("he=e-1
syntax match math_functions "\(^|.\|\s*\)prec*\s*("he=e-1

syntax match file_functions "\(^|.\|\s*\)file_mode\s*("he=e-1
syntax match file_functions "\(^|.\|\s*\)file_size\s*("he=e-1
syntax match file_functions "\(^|.\|\s*\)read_file\s*("he=e-1
syntax match file_functions "\(^|.\|\s*\)write_file\s*("he=e-1
syntax match file_functions "\(^|.\|\s*\)is_file\s*("he=e-1
syntax match file_functions "\(^|.\|\s*\)is_dir\s*("he=e-1
syntax match file_functions "\(^|.\|\s*\)perms\s*("he=e-1
syntax match file_functions "\(^|.\|\s*\)fopen\s*("he=e-1
syntax match file_functions "\(^|.\|\s*\)fclose\s*("he=e-1
syntax match file_functions "\(^|.\|\s*\)fseek\s*("he=e-1
syntax match file_functions "\(^|.\|\s*\)fread\s*("he=e-1
syntax match file_functions "\(^|.\|\s*\)fwrite\s*("he=e-1
syntax match file_functions "\(^|.\|\s*\)feof\s*("he=e-1
syntax match file_functions "\(^|.\|\s*\)ftell\s*("he=e-1
syntax match file_functions "\(^|.\|\s*\)fflush\s*("he=e-1
syntax match file_functions "\(^|.\|\s*\)stat\s*("he=e-1
syntax match file_functions "\(^|.\|\s*\)flock\s*("he=e-1

syntax match web_functions "\(^|.\|\s*\)download\s*("he=e-1
syntax match web_functions "\(^|.\|\s*\)web_download\s*("he=e-1
syntax match web_functions "\(^|.\|\s*\)web_custom\s*("he=e-1
syntax match web_functions "\(^|.\|\s*\)web_max_clients\s*("he=e-1
syntax match web_functions "\(^|.\|\s*\)web_get\s*("he=e-1
syntax match web_functions "\(^|.\|\s*\)web_head\s*("he=e-1
syntax match web_functions "\(^|.\|\s*\)web_post\s*("he=e-1
syntax match web_functions "\(^|.\|\s*\)web_serve_start\s*("he=e-1
syntax match web_functions "\(^|.\|\s*\)web_serve_stop\s*("he=e-1
syntax match web_functions "\(^|.\|\s*\)web_serve_up\s*("he=e-1
syntax match web_functions "\(^|.\|\s*\)web_serve_path\s*("he=e-1
syntax match web_functions "\(^|.\|\s*\)web_serve_log_throttle\s*("he=e-1
syntax match web_functions "\(^|.\|\s*\)web_serve_decode\s*("he=e-1
syntax match web_functions "\(^|.\|\s*\)net_interfaces\s*("he=e-1
syntax match web_functions "\(^|.\|\s*\)html_escape\s*("he=e-1
syntax match web_functions "\(^|.\|\s*\)html_unescape\s*("he=e-1

syntax match db_functions "\(^|.\|\s\*\)*db_init\s*("he=e-1
syntax match db_functions "\(^|.\|\s\*\)*db_query\s*("he=e-1
syntax match db_functions "\(^|.\|\s\*\)*db_close\s*("he=e-1

syntax match string_functions "\(^|.\|\s*\)next_match\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)stripquotes\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)stripcc\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)addansi\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)stripansi\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)pad\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)len\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)field\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)fields\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)pipesep\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)get_value\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)has_start\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)has_end\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)reg_match\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)reg_filter\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)reg_replace\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)match\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)filter\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)line_match\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)line_filter\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)grep\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)split\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)join\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)collapse\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)substr\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)gsub\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)replace\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)trim\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)lines\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)count\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)line_head\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)line_tail\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)line_add\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)line_delete\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)line_replace\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)line_add_before\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)line_add_after\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)reverse\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)tr\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)lower\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)upper\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)format\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)ccformat\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)strpos\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)pos\s*("he=e-1
syntax match string_functions "\(^|.\|\s*\)inset\s*("he=e-1

syntax match os_functions "\(^|.\|\s*\)env\s*("he=e-1
syntax match os_functions "\(^|.\|\s*\)get_env\s*("he=e-1
syntax match os_functions "\(^|.\|\s*\)set_env\s*("he=e-1
syntax match os_functions "\(^|.\|\s*\)cd\s*("he=e-1
syntax match os_functions "\(^|.\|\s*\)cwd\s*("he=e-1
syntax match os_functions "\(^|.\|\s*\)dir\s*("he=e-1
syntax match os_functions "\(^|.\|\s*\)umask\s*("he=e-1
syntax match os_functions "\(^|.\|\s*\)chroot\s*("he=e-1
syntax match os_functions "\(^|.\|\s*\)delete\s*("he=e-1
syntax match os_functions "\(^|.\|\s*\)rename\s*("he=e-1
syntax match os_functions "\(^|.\|\s*\)copy\s*("he=e-1
syntax match os_functions "\(^|.\|\s*\)can_read\s*("he=e-1
syntax match os_functions "\(^|.\|\s*\)can_write\s*("he=e-1
syntax match os_functions "\(^|.\|\s*\)parent\s*("he=e-1
syntax match os_functions "\(^|.\|\s*\)fileabs\s*("he=e-1
syntax match os_functions "\(^|.\|\s*\)filebase\s*("he=e-1
syntax match os_functions "\(^|.\|\s*\)is_symlink\s*("he=e-1
syntax match os_functions "\(^|.\|\s*\)is_device\s*("he=e-1
syntax match os_functions "\(^|.\|\s*\)is_pipe\s*("he=e-1
syntax match os_functions "\(^|.\|\s*\)is_socket\s*("he=e-1
syntax match os_functions "\(^|.\|\s*\)is_sticky\s*("he=e-1
syntax match os_functions "\(^|.\|\s*\)is_setuid\s*("he=e-1
syntax match os_functions "\(^|.\|\s*\)is_setgid\s*("he=e-1

syntax match html_functions "\(^|.\|\s*\)wpage\s*("he=e-1
syntax match html_functions "\(^|.\|\s*\)wbody\s*("he=e-1
syntax match html_functions "\(^|.\|\s*\)wdiv\s*("he=e-1
syntax match html_functions "\(^|.\|\s*\)wa\s*("he=e-1
syntax match html_functions "\(^|.\|\s*\)wimg\s*("he=e-1
syntax match html_functions "\(^|.\|\s*\)whead\s*("he=e-1
syntax match html_functions "\(^|.\|\s*\)wlink\s*("he=e-1
syntax match html_functions "\(^|.\|\s*\)wp\s*("he=e-1
syntax match html_functions "\(^|.\|\s*\)wtable\s*("he=e-1
syntax match html_functions "\(^|.\|\s*\)wthead\s*("he=e-1
syntax match html_functions "\(^|.\|\s*\)wtbody\s*("he=e-1
syntax match html_functions "\(^|.\|\s*\)wtr\s*("he=e-1
syntax match html_functions "\(^|.\|\s*\)wth\s*("he=e-1
syntax match html_functions "\(^|.\|\s*\)wtd\s*("he=e-1
syntax match html_functions "\(^|.\|\s*\)wul\s*("he=e-1
syntax match html_functions "\(^|.\|\s*\)wol\s*("he=e-1
syntax match html_functions "\(^|.\|\s*\)wli\s*("he=e-1
syntax match html_functions "\(^|.\|\s*\)wh1\s*("he=e-1
syntax match html_functions "\(^|.\|\s*\)wh2\s*("he=e-1
syntax match html_functions "\(^|.\|\s*\)wh3\s*("he=e-1
syntax match html_functions "\(^|.\|\s*\)wh4\s*("he=e-1
syntax match html_functions "\(^|.\|\s*\)wh5\s*("he=e-1

syntax match notify_functions "\(^|.\|\s*\)ev_watch\s*("he=e-1
syntax match notify_functions "\(^|.\|\s*\)ev_watch_close\s*("he=e-1
syntax match notify_functions "\(^|.\|\s*\)ev_watch_add\s*("he=e-1
syntax match notify_functions "\(^|.\|\s*\)ev_exists\s*("he=e-1
syntax match notify_functions "\(^|.\|\s*\)ev_event\s*("he=e-1
syntax match notify_functions "\(^|.\|\s*\)ev_mask\s*("he=e-1

" Za Keywords: {{{1
" ==============

syntax match tstatements "\(^\|\s\+\)\(doc\|test\|endtest\|assert\)\($\|\s\+\)"
syntax match statements '\(^\|\s\+\)|\($\|\s\+\)'
syntax match statements "\( do \| to \| as \| in \| is \)"
syntax match statements "\(^\|\s\+\)\(on\|or\|if\|at\)\($\|\s\+\)"
syntax match statements "\(^\|\s\+\)\(end\|and\|not\|for\|nop\|var\|log\|cls\|web\)\($\|\s\+\)"
syntax match statements "\(^\|\s\+\)\(else\|step\|pane\|enum\|init\|help\|with\|when\|hist\|exit\)\($\|\s\+\)"
syntax match statements "\(^\|\s\+\)\(struct\|pause\|debug\|async\|print\|break\|endif\|while\|quiet\)\($\|\s\+\)"
syntax match statements "\(^\|\s\+\)\(module\|prompt\|return\|define\|endfor\|enddef\|enable\)\($\|\s\+\)"
syntax match statements "\(^\|\s\+\)\(foreach\|version\|require\|println\|showdef\|endwith\|endwhen\|logging\|subject\|disable\)\($\|\s\+\)"
syntax match statements "\(^\|\s\+\)\(contains\|endwhile\|continue\)\($\|\s\+\)"
syntax match statements "\(^\|\s\+\)\(endstruct\)\($\|\s\+\)"
syntax match statements "\(^\|\s\+\)\(accessfile\|showstruct\)\($\|\s\+\)"

syntax match types "\sany\(\s\|$\)"
syntax match types "\sint\(\s\|$\)"
syntax match types "\suint\(\s\|$\)"
syntax match types "\sbool\(\s\|$\)"
syntax match types "\sfloat\(\s\|$\)"
syntax match types "\sstring\(\s\|$\)"
syntax match types "\smap\(\s\|$\)"
syntax match types "\sarray\(\s\|$\)"

" Color Matching {{{1
" ===============
syntax match colour_b0 "\[#b0\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote
syntax match colour_b1 "\[#b1\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote
syntax match colour_b2 "\[#b2\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote
syntax match colour_b3 "\[#b3\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote
syntax match colour_b4 "\[#b4\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote
syntax match colour_b5 "\[#b5\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote
syntax match colour_b6 "\[#b6\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote
syntax match colour_b7 "\[#b7\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote
syntax match colour_b0 "\[#bblack\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote
syntax match colour_b1 "\[#bblue\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote
syntax match colour_b2 "\[#bred\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote
syntax match colour_b3 "\[#bmagenta\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote
syntax match colour_b4 "\[#bgreen\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote
syntax match colour_b5 "\[#bcyan\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote
syntax match colour_b6 "\[#byellow\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote
syntax match colour_b7 "\[#bwhite\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote

syntax match colour_f0 "\[#0\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote
syntax match colour_f1 "\[#1\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote
syntax match colour_f2 "\[#2\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote
syntax match colour_f3 "\[#3\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote
syntax match colour_f4 "\[#4\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote
syntax match colour_f5 "\[#5\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote
syntax match colour_f6 "\[#6\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote
syntax match colour_f7 "\[#7\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote
syntax match colour_f0 "\[#fblack\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote
syntax match colour_f1 "\[#fblue\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote
syntax match colour_f2 "\[#fred\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote
syntax match colour_f3 "\[#fmagenta\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote
syntax match colour_f4 "\[#fgreen\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote
syntax match colour_f5 "\[#fcyan\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote
syntax match colour_f6 "\[#fyellow\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote
syntax match colour_f7 "\[#fwhite\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote

syntax match colour_normal "\[##\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote
syntax match colour_normal "\[#-\]"hs=s+1,he=e-1 containedin=DoubleQuote,BacktkQuote

" Quoting: {{{1
" ========
syn match   cSpecial    display contained "\\\(x\x\+\|\o\{1,3}\|.\|$\)" containedin=DoubleQuote,BacktkQuote
syn region  DoubleQuote   start=+L\="+ skip=+\\\\\|\\"+ end=+"+ extend
syn region  BacktkQuote   start=+L\=`+ skip=+\\\\\|\\`+ end=+`+ extend

" Extra Bracing: {{{1
" ===================

" syntax region sqBrace transparent start=/\v\[/ skip=+\\[\]]+ end=/\v\]/

" Clusters: contains=@... clusters: {{{1
"==================================
syn cluster Functions       contains=list_functions,time_functions,conversion_functions,internal_functions,os_functions,package_functions,math_functions,file_functions,web_functions,db_functions,string_functions,image_functions,html_functions,notify_functions
syn cluster ArithParenList  contains=Float,Integer,Operator,SingleQuote,Variable,CtrlSeq,Paren,Functions

" Arithmetic Parenthesized Expressions: {{{1
" =====================================
syn region Paren start='[^$]\zs(\%(\ze[^(]\|$\)' end=')' contains=@ArithParenList


" Synchronization: {{{1
" ================
if !exists("sh_minlines")
  let sh_minlines = 400
endif
if !exists("sh_maxlines")
  let sh_maxlines = 2 * sh_minlines
endif
exec "syn sync minlines=" . sh_minlines . " maxlines=" . sh_maxlines


hi def link idents       Identifiers
hi def link types        Types

" Default Highlighting: {{{1
" =====================
hi def link Quote   Operator
hi def link Colon   Comment
hi def link DoubleQuote String
hi def link BacktkQuote String
hi def link Loop    statements
hi def link NoQuote DoubleQuote
hi def link Pattern String
hi def link Paren   Arithmetic
hi def link QuickComment    Comment
hi def link Range   Operator
hi def link SingleQuote String
hi def link SubShRegion Operator
hi def link WrapLineOperator    Operator
hi def link notify_functions functionlist
hi def link time_functions functionlist
hi def link list_functions functionlist
hi def link conversion_functions functionlist
hi def link internal_functions functionlist
hi def link os_functions functionlist
hi def link package_functions functionlist
hi def link math_functions functionlist
hi def link file_functions functionlist
hi def link web_functions functionlist
hi def link db_functions functionlist
hi def link string_functions functionlist
hi def link html_functions functionlist
hi def link image_functions functionlist

if !exists("g:sh_no_error")
 hi def link CondError      Error
 hi def link WhenError      Error
 hi def link IfError        Error
 hi def link InError        Error
endif

hi def link Arithmetic          Special
hi def link Comment             comment 
hi def link Conditional         Conditional
hi def link CtrlSeq             Special
hi def link ExprRegion          Delimiter
hi def link Operator            Operator
hi def link Set                 statements
hi def link assignStatements    statements
hi def link StringLiteral       String
hi def link Float               Numbers
hi def link Integer             Numbers
" hi def link sqBrace             MatchParen

hi Normal       ctermfg=blue ctermbg=NONE
hi comment      ctermfg=red
hi Constant     ctermfg=darkGreen cterm=bold
hi statements   ctermfg=darkBlue
hi tstatements  ctermfg=magenta
hi Identifiers  ctermfg=darkYellow
hi ErrorMsg     ctermfg=black ctermbg=red
hi WarningMsg   ctermfg=black ctermbg=green
hi MatchParen   ctermbg=blue ctermfg=yellow
hi InnerBrace   ctermbg=darkGray ctermfg=blue
hi Error        ctermbg=red
hi functionlist ctermfg=Gray cterm=italic
hi Search       ctermbg=darkGray ctermfg=blue
hi LineNr       ctermfg=blue
hi title        ctermfg=darkGray
hi ShowMarksHL  cterm=bold ctermfg=yellow ctermbg=black
hi StatusLineNC ctermfg=lightBlue ctermbg=darkBlue
hi StatusLine   cterm=bold ctermfg=cyan ctermbg=blue
hi clear Visual
hi Visual       term=reverse cterm=reverse cterm=reverse
hi DiffChange   ctermbg=darkGreen
hi diffOnly ctermfg=red cterm=bold

hi colour_b0    ctermbg=black ctermfg=white
hi colour_b1    ctermbg=blue ctermfg=white
hi colour_b2    ctermbg=red ctermfg=white
hi colour_b3    ctermbg=magenta ctermfg=white
hi colour_b4    ctermbg=green ctermfg=white
hi colour_b5    ctermbg=cyan ctermfg=black
hi colour_b6    ctermbg=yellow ctermfg=black
hi colour_b7    ctermbg=gray ctermfg=black

hi colour_f0    ctermfg=black ctermbg=black
hi colour_f1    ctermfg=blue ctermbg=black
hi colour_f2    ctermfg=red ctermbg=black
hi colour_f3    ctermfg=magenta ctermbg=black
hi colour_f4    ctermfg=green ctermbg=black
hi colour_f5    ctermfg=cyan ctermbg=black
hi colour_f6    ctermfg=yellow ctermbg=black
hi colour_f7    ctermfg=white ctermbg=black

hi colour_normal ctermfg=white ctermbg=darkGreen

hi Types        ctermfg=magenta
hi Numbers      ctermfg=lightBlue
hi String ctermfg=green

" Set Current Syntax: {{{1
" ===================
let b:current_syntax = "za"

