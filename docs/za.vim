" za syntax file
" Language:		za (za)
" Maintainer:	Daniel Horsley  <dhorsley@gmail.com>
" Last Change:	August 10, 2019
" Version:		5

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
syn match   Operator	"<<\|>>"		contained
syn match   Operator	"[!&;|]"		contained
syn match   Operator	"\[[[^:]\|\]]"		contained


" Misc: {{{1
"======
syn match   WrapLineOperator "\\$"
syn match   Escape	contained	'\%(^\)\@!\%(\\\\\)*\\.'

syn match   Source	"^\.\s*"
syn match   Source	"\s\.\s"
syn match   Colon	'^\s*\zs:'


" String And Character Constants: {{{1
"================================
syn match   Number	"\<\d\+\>#\="
syn match   Number	"-\=\.\=\d\+\>#\="
syn match   CtrlSeq	"\\\d\d\d\|\\[abcfnrtv0]"		contained
syn match   StringSpecial	"[^[:print:] \t]"		contained

" Comments: {{{1
"==========
syn match	Comment		"^\s*\zs#.*$"	contains=@CommentGroup
syn match	Comment		"\s\zs#.*$"	    contains=@CommentGroup
syn match	Comment		"^\s*\zs//.*$"	contains=@CommentGroup
syn match	Comment		"\s\zs//.*$"	contains=@CommentGroup

" Identifiers: {{{1
"=============
syn match   folVarRHS           "{[\~#\&]\=[[:alnum:]_]\{-}}"hs=s+1,he=e-1
syn keyword assignStatements    input zero inc dec nextgroup=folVarLHS skipwhite
syn match   folVarLHS           '\i\+' contained

" Functions: {{{1
" ==========

syntax match udf_functions "\s*[[:alnum:]_]\{-}\s*("he=e-1

syntax match time_functions "\s*date\s*("he=e-1
syntax match time_functions "\s*epoch_time\s*("he=e-1
syntax match time_functions "\s*epoch_nano_time\s*("he=e-1
syntax match time_functions "\s*time_diff\s*("he=e-1

syntax match list_functions "\s*empty\s*("he=e-1
syntax match list_functions "\s*similar\s*("he=e-1
syntax match list_functions "\s*col\s*("he=e-1
syntax match list_functions "\s*head\s*("he=e-1
syntax match list_functions "\s*tail\s*("he=e-1
syntax match list_functions "\s*sum\s*("he=e-1
syntax match list_functions "\s*uniq\s*("he=e-1
syntax match list_functions "\s*append\s*("he=e-1
syntax match list_functions "\s*insert\s*("he=e-1
syntax match list_functions "\s*remove\s*("he=e-1
syntax match list_functions "\s*push\s*("he=e-1
syntax match list_functions "\s*sort\s*("he=e-1
syntax match list_functions "\s*deq\s*("he=e-1
syntax match list_functions "\s*any\s*("he=e-1
syntax match list_functions "\s*all\s*("he=e-1
syntax match list_functions "\s*concat\s*("he=e-1
syntax match list_functions "\s*esplit\s*("he=e-1
syntax match list_functions "\s*min\s*("he=e-1
syntax match list_functions "\s*max\s*("he=e-1
syntax match list_functions "\s*avg\s*("he=e-1
syntax match list_functions "\s*fieldsort\s*("he=e-1
syntax match list_functions "\s*numcomp\s*("he=e-1

syntax match conversion_functions "\s*int\s*("he=e-1
syntax match conversion_functions "\s*float\s*("he=e-1
syntax match conversion_functions "\s*string\s*("he=e-1
syntax match conversion_functions "\s*kind\s*("he=e-1
syntax match conversion_functions "\s*is_number\s*("he=e-1 
syntax match conversion_functions "\s*chr\s*("he=e-1
syntax match conversion_functions "\s*ascii\s*("he=e-1
syntax match conversion_functions "\s*list_float\s*("he=e-1
syntax match conversion_functions "\s*list_string\s*("he=e-1
syntax match conversion_functions "\s*list_int\s*("he=e-1
syntax match conversion_functions "\s*local\s*("he=e-1
syntax match conversion_functions "\s*base64e\s*("he=e-1
syntax match conversion_functions "\s*base64d\s*("he=e-1
syntax match conversion_functions "\s*json_decode\s*("he=e-1
syntax match conversion_functions "\s*json_format\s*("he=e-1

syntax match internal_functions "\s*execpath\s*("he=e-1
syntax match internal_functions "\s*last\s*("he=e-1
syntax match internal_functions "\s*last_out\s*("he=e-1
syntax match internal_functions "\s*zsh_version\s*("he=e-1
syntax match internal_functions "\s*bash_version\s*("he=e-1
syntax match internal_functions "\s*bash_versinfo\s*("he=e-1
syntax match internal_functions "\s*user\s*("he=e-1
syntax match internal_functions "\s*os\s*("he=e-1
syntax match internal_functions "\s*home\s*("he=e-1
syntax match internal_functions "\s*lang\s*("he=e-1
syntax match internal_functions "\s*release_name\s*("he=e-1
syntax match internal_functions "\s*release_version\s*("he=e-1
syntax match internal_functions "\s*release_id\s*("he=e-1
syntax match internal_functions "\s*has_shell\s*("he=e-1
syntax match internal_functions "\s*shellpid\s*("he=e-1
syntax match internal_functions "\s*winterm\s*("he=e-1
syntax match internal_functions "\s*hostname\s*("he=e-1
syntax match internal_functions "\s*argv\s*("he=e-1
syntax match internal_functions "\s*argc\s*("he=e-1
syntax match internal_functions "\s*dump\s*("he=e-1
syntax match internal_functions "\s*eval\s*("he=e-1
syntax match internal_functions "\s*keypress\s*("he=e-1
syntax match internal_functions "\s*tokens\s*("he=e-1
syntax match internal_functions "\s*globkey\s*("he=e-1
syntax match internal_functions "\s*clear_line\s*("he=e-1
syntax match internal_functions "\s*key\s*("he=e-1
syntax match internal_functions "\s*clktck\s*("he=e-1
syntax match internal_functions "\s*globlen\s*("he=e-1
syntax match internal_functions "\s*getglob\s*("he=e-1
syntax match internal_functions "\s*funcref\s*("he=e-1
syntax match internal_functions "\s*thisfunc\s*("he=e-1
syntax match internal_functions "\s*thisref\s*("he=e-1
syntax match internal_functions "\s*pid\s*("he=e-1
syntax match internal_functions "\s*ppid\s*("he=e-1
syntax match internal_functions "\s*commands\s*("he=e-1
syntax match internal_functions "\s*cursoron\s*("he=e-1
syntax match internal_functions "\s*cursoroff\s*("he=e-1
syntax match internal_functions "\s*cursorx\s*("he=e-1
syntax match internal_functions "\s*term_h\s*("he=e-1
syntax match internal_functions "\s*term_w\s*("he=e-1
syntax match internal_functions "\s*pane_h\s*("he=e-1
syntax match internal_functions "\s*pane_w\s*("he=e-1
syntax match internal_functions "\s*utf8supported\s*("he=e-1
syntax match internal_functions "\s*system\s*("he=e-1
syntax match internal_functions "\s*locks\s*("he=e-1
syntax match internal_functions "\s*echo\s*("he=e-1
syntax match internal_functions "\s*ansi\s*("he=e-1
syntax match internal_functions "\s*interpol\s*("he=e-1
syntax match internal_functions "\s*tco\s*("he=e-1
syntax match internal_functions "\s*echo\s*("he=e-1
syntax match internal_functions "\s*getrow\s*("he=e-1
syntax match internal_functions "\s*getcol\s*("he=e-1
syntax match internal_functions "\s*unmap\s*("he=e-1
syntax match internal_functions "\s*coproc\s*("he=e-1
syntax match internal_functions "\s*await\s*("he=e-1
syntax match internal_functions "\s*funcs\s*("he=e-1

syntax match image_functions "\s*svg_start\s*("he=e-1
syntax match image_functions "\s*svg_end\s*("he=e-1
syntax match image_functions "\s*svg_title\s*("he=e-1
syntax match image_functions "\s*svg_desc\s*("he=e-1
syntax match image_functions "\s*svg_plot\s*("he=e-1
syntax match image_functions "\s*svg_circle\s*("he=e-1
syntax match image_functions "\s*svg_ellipse\s*("he=e-1
syntax match image_functions "\s*svg_rect\s*("he=e-1
syntax match image_functions "\s*svg_square\s*("he=e-1
syntax match image_functions "\s*svg_roundrect\s*("he=e-1
syntax match image_functions "\s*svg_grid\s*("he=e-1
syntax match image_functions "\s*svg_line\s*("he=e-1
syntax match image_functions "\s*svg_polyline\s*("he=e-1
syntax match image_functions "\s*svg_polygon\s*("he=e-1
syntax match image_functions "\s*svg_text\s*("he=e-1
syntax match image_functions "\s*svg_image\s*("he=e-1
syntax match image_functions "\s*svg_def\s*("he=e-1
syntax match image_functions "\s*svg_def_end\s*("he=e-1
syntax match image_functions "\s*svg_link\s*("he=e-1
syntax match image_functions "\s*svg_link_end\s*("he=e-1
syntax match image_functions "\s*svg_group\s*("he=e-1
syntax match image_functions "\s*svg_group_end\s*("he=e-1

syntax match package_functions "\s*install\s*("he=e-1
syntax match package_functions "\s*service\s*("he=e-1
syntax match package_functions "\s*vcmp\s*("he=e-1

syntax match math_functions "\s*seed\s*("he=e-1
syntax match math_functions "\s*rand\s*("he=e-1
syntax match math_functions "\s*sqr\s*("he=e-1
syntax match math_functions "\s*sqrt\s*("he=e-1
syntax match math_functions "\s*pow\s*("he=e-1
syntax match math_functions "\s*sin\s*("he=e-1
syntax match math_functions "\s*cos\s*("he=e-1
syntax match math_functions "\s*tan\s*("he=e-1
syntax match math_functions "\s*asin\s*("he=e-1
syntax match math_functions "\s*acos\s*("he=e-1
syntax match math_functions "\s*atan\s*("he=e-1
syntax match math_functions "\s*ln\s*("he=e-1
syntax match math_functions "\s*log\s*("he=e-1
syntax match math_functions "\s*log2\s*("he=e-1
syntax match math_functions "\s*log10\s*("he=e-1
syntax match math_functions "\s*round\s*("he=e-1
syntax match math_functions "\s*rad2deg\s*("he=e-1
syntax match math_functions "\s*deg2rad\s*("he=e-1
syntax match math_functions "\s*pi\s*("he=e-1
syntax match math_functions "\s*phi\s*("he=e-1
syntax match math_functions "\s*e\s*("he=e-1
syntax match math_functions "\s*ln2\s*("he=e-1
syntax match math_functions "\s*ln10\s*("he=e-1
syntax match math_functions "\s*numcomma*\s*("he=e-1
syntax match math_functions "\s*ubin8*\s*("he=e-1
syntax match math_functions "\s*uhex32*\s*("he=e-1
syntax match math_functions "\s*abs*\s*("he=e-1

syntax match file_functions "\s*file_mode\s*("he=e-1
syntax match file_functions "\s*file_size\s*("he=e-1
syntax match file_functions "\s*read_file\s*("he=e-1
syntax match file_functions "\s*write_file\s*("he=e-1
syntax match file_functions "\s*is_file\s*("he=e-1
syntax match file_functions "\s*is_dir\s*("he=e-1
syntax match file_functions "\s*is_soft\s*("he=e-1
syntax match file_functions "\s*is_pipe\s*("he=e-1
syntax match file_functions "\s*perms\s*("he=e-1
syntax match file_functions "\s*file_create\s*("he=e-1
syntax match file_functions "\s*file_close\s*("he=e-1

syntax match web_functions "\s*web_download\s*("he=e-1
syntax match web_functions "\s*web_custom\s*("he=e-1
syntax match web_functions "\s*web_max_clients\s*("he=e-1
syntax match web_functions "\s*web_get\s*("he=e-1
syntax match web_functions "\s*web_head\s*("he=e-1
syntax match web_functions "\s*web_post\s*("he=e-1
syntax match web_functions "\s*web_serve_start\s*("he=e-1
syntax match web_functions "\s*web_serve_stop\s*("he=e-1
syntax match web_functions "\s*web_serve_up\s*("he=e-1
syntax match web_functions "\s*web_serve_path\s*("he=e-1
syntax match web_functions "\s*web_serve_log_throttle\s*("he=e-1
syntax match web_functions "\s*web_serve_decode\s*("he=e-1
syntax match web_functions "\s*net_interfaces\s*("he=e-1
syntax match web_functions "\s*html_escape\s*("he=e-1
syntax match web_functions "\s*html_unescape\s*("he=e-1

syntax match db_functions "\s*db_init\s*("he=e-1
syntax match db_functions "\s*db_query\s*("he=e-1
syntax match db_functions "\s*db_fields\s*("he=e-1

syntax match string_functions "\s*stripansi\s*("he=e-1
syntax match string_functions "\s*pad\s*("he=e-1
syntax match string_functions "\s*len\s*("he=e-1
syntax match string_functions "\s*length\s*("he=e-1
syntax match string_functions "\s*field\s*("he=e-1
syntax match string_functions "\s*fields\s*("he=e-1
syntax match string_functions "\s*pipesep\s*("he=e-1
syntax match string_functions "\s*get_value\s*("he=e-1
syntax match string_functions "\s*start\s*("he=e-1
syntax match string_functions "\s*end\s*("he=e-1
syntax match string_functions "\s*match\s*("he=e-1
syntax match string_functions "\s*filter\s*("he=e-1
syntax match string_functions "\s*line_match\s*("he=e-1
syntax match string_functions "\s*line_filter\s*("he=e-1
syntax match string_functions "\s*split\s*("he=e-1
syntax match string_functions "\s*join\s*("he=e-1
syntax match string_functions "\s*collapse\s*("he=e-1
syntax match string_functions "\s*substr\s*("he=e-1
syntax match string_functions "\s*gsub\s*("he=e-1
syntax match string_functions "\s*replace\s*("he=e-1
syntax match string_functions "\s*trim\s*("he=e-1
syntax match string_functions "\s*lines\s*("he=e-1
syntax match string_functions "\s*count\s*("he=e-1
syntax match string_functions "\s*line_head\s*("he=e-1
syntax match string_functions "\s*line_tail\s*("he=e-1
syntax match string_functions "\s*line_add\s*("he=e-1
syntax match string_functions "\s*line_delete\s*("he=e-1
syntax match string_functions "\s*line_replace\s*("he=e-1
syntax match string_functions "\s*line_add_before\s*("he=e-1
syntax match string_functions "\s*line_add_after\s*("he=e-1
syntax match string_functions "\s*reverse\s*("he=e-1
syntax match string_functions "\s*tr\s*("he=e-1
syntax match string_functions "\s*lower\s*("he=e-1
syntax match string_functions "\s*upper\s*("he=e-1
syntax match string_functions "\s*format\s*("he=e-1
syntax match string_functions "\s*strpos\s*("he=e-1

syntax match env_functions "\s*env\s*("he=e-1
syntax match env_functions "\s*get_env\s*("he=e-1
syntax match env_functions "\s*set_env\s*("he=e-1
syntax match env_functions "\s*cd\s*("he=e-1
syntax match env_functions "\s*cwd\s*("he=e-1
syntax match env_functions "\s*dir\s*("he=e-1
syntax match env_functions "\s*umask\s*("he=e-1
syntax match env_functions "\s*chroot\s*("he=e-1
syntax match env_functions "\s*remove\s*("he=e-1

syntax match html_functions "\s*wpage\s*("he=e-1
syntax match html_functions "\s*wbody\s*("he=e-1
syntax match html_functions "\s*wdiv\s*("he=e-1
syntax match html_functions "\s*wa\s*("he=e-1
syntax match html_functions "\s*wimg\s*("he=e-1
syntax match html_functions "\s*whead\s*("he=e-1
syntax match html_functions "\s*wlink\s*("he=e-1
syntax match html_functions "\s*wp\s*("he=e-1
syntax match html_functions "\s*wtable\s*("he=e-1
syntax match html_functions "\s*wthead\s*("he=e-1
syntax match html_functions "\s*wtbody\s*("he=e-1
syntax match html_functions "\s*wtr\s*("he=e-1
syntax match html_functions "\s*wth\s*("he=e-1
syntax match html_functions "\s*wtd\s*("he=e-1
syntax match html_functions "\s*wul\s*("he=e-1
syntax match html_functions "\s*wol\s*("he=e-1
syntax match html_functions "\s*wli\s*("he=e-1
syntax match html_functions "\s*wh1\s*("he=e-1
syntax match html_functions "\s*wh2\s*("he=e-1
syntax match html_functions "\s*wh3\s*("he=e-1
syntax match html_functions "\s*wh4\s*("he=e-1
syntax match html_functions "\s*wh5\s*("he=e-1


" Za Keywords: {{{1
" ==============

syntax match tstatements "\(^\|\s\+\)\(doc\|test\|endtest\|assert\)\($\|\s\+\)"
syntax match statements '\(^\|\s\+\)|\($\|\s\+\)'
syntax match statements "\( do \| to \| in \)"
syntax match statements "\(^\|\s\+\)\(on\|or\|if\|at\)\($\|\s\+\)"
syntax match statements "\(^\|\s\+\)\(for\|nop\|log\|cls\|web\)\($\|\s\+\)"
syntax match statements "\(^\|\s\+\)\(else\|step\|pane\|init\|loud\|help\|with\|when\|hist\|exit\)\($\|\s\+\)"
syntax match statements "\(^\|\s\+\)\(pause\|debug\|async\|print\|break\|endif\|unset\|while\|quiet\|pane\)\($\|\s\+\)"
syntax match statements "\(^\|\s\+\)\(module\|prompt\|return\|define\|endfor\|enddef\|enable\)\($\|\s\+\)"
syntax match statements "\(^\|\s\+\)\(version\|require\|println\|setglob\|showdef\|endwith\|endwhen\|logging\|subject\|disable\)\($\|\s\+\)"
syntax match statements "\(^\|\s\+\)\(contains\|endwhile\|foreach\|continue\)\($\|\s\+\)"
syntax match statements "\(^\|\s\+\)\(accessfile\)\($\|\s\+\)"

" Color Matching {{{1
" ===============
syntax match colour_b0 "\[#b0\]"hs=s+1,he=e-1 containedin=DoubleQuote
syntax match colour_b1 "\[#b1\]"hs=s+1,he=e-1 containedin=DoubleQuote
syntax match colour_b2 "\[#b2\]"hs=s+1,he=e-1 containedin=DoubleQuote
syntax match colour_b3 "\[#b3\]"hs=s+1,he=e-1 containedin=DoubleQuote
syntax match colour_b4 "\[#b4\]"hs=s+1,he=e-1 containedin=DoubleQuote
syntax match colour_b5 "\[#b5\]"hs=s+1,he=e-1 containedin=DoubleQuote
syntax match colour_b6 "\[#b6\]"hs=s+1,he=e-1 containedin=DoubleQuote
syntax match colour_b7 "\[#b7\]"hs=s+1,he=e-1 containedin=DoubleQuote
syntax match colour_b0 "\[#bblack\]"hs=s+1,he=e-1 containedin=DoubleQuote
syntax match colour_b1 "\[#bblue\]"hs=s+1,he=e-1 containedin=DoubleQuote
syntax match colour_b2 "\[#bred\]"hs=s+1,he=e-1 containedin=DoubleQuote
syntax match colour_b3 "\[#bmagenta\]"hs=s+1,he=e-1 containedin=DoubleQuote
syntax match colour_b4 "\[#bgreen\]"hs=s+1,he=e-1 containedin=DoubleQuote
syntax match colour_b5 "\[#bcyan\]"hs=s+1,he=e-1 containedin=DoubleQuote
syntax match colour_b6 "\[#byellow\]"hs=s+1,he=e-1 containedin=DoubleQuote
syntax match colour_b7 "\[#bwhite\]"hs=s+1,he=e-1 containedin=DoubleQuote

syntax match colour_f0 "\[#0\]"hs=s+1,he=e-1 containedin=DoubleQuote
syntax match colour_f1 "\[#1\]"hs=s+1,he=e-1 containedin=DoubleQuote
syntax match colour_f2 "\[#2\]"hs=s+1,he=e-1 containedin=DoubleQuote
syntax match colour_f3 "\[#3\]"hs=s+1,he=e-1 containedin=DoubleQuote
syntax match colour_f4 "\[#4\]"hs=s+1,he=e-1 containedin=DoubleQuote
syntax match colour_f5 "\[#5\]"hs=s+1,he=e-1 containedin=DoubleQuote
syntax match colour_f6 "\[#6\]"hs=s+1,he=e-1 containedin=DoubleQuote
syntax match colour_f7 "\[#7\]"hs=s+1,he=e-1 containedin=DoubleQuote
syntax match colour_f0 "\[#black\]"hs=s+1,he=e-1 containedin=DoubleQuote
syntax match colour_f1 "\[#blue\]"hs=s+1,he=e-1 containedin=DoubleQuote
syntax match colour_f2 "\[#red\]"hs=s+1,he=e-1 containedin=DoubleQuote
syntax match colour_f3 "\[#magenta\]"hs=s+1,he=e-1 containedin=DoubleQuote
syntax match colour_f4 "\[#green\]"hs=s+1,he=e-1 containedin=DoubleQuote
syntax match colour_f5 "\[#cyan\]"hs=s+1,he=e-1 containedin=DoubleQuote
syntax match colour_f6 "\[#yellow\]"hs=s+1,he=e-1 containedin=DoubleQuote
syntax match colour_f7 "\[#white\]"hs=s+1,he=e-1 containedin=DoubleQuote

syntax match colour_normal "\[##\]"hs=s+1,he=e-1 containedin=DoubleQuote
syntax match colour_normal "\[#-\]"hs=s+1,he=e-1 containedin=DoubleQuote

" Quoting: {{{1
" ========
syntax region DoubleQuote start=/\v"/ skip=+\\['"]+ end=/\v"/
syntax region DoubleQuote start=/\v`/ skip=+\\[`]+ end=/\v`/

" Clusters: contains=@... clusters: {{{1
"==================================
syn cluster Functions       contains=list_functions,conversion_functions,internal_functions,package_functions,math_functions,file_functions,web_functions,db_functions,string_functions,env_functions,image_functions,html_functions,udf_functions
syn cluster ArithParenList  contains=Arithmetic,Comment,Escape,Number,Operator,SingleQuote,Variable,CtrlSeq,Special,Paren,Functions

" Arithmetic Parenthesized Expressions: {{{1
" =====================================
syn region Paren start='[^$]\zs(\%(\ze[^(]\|$\)' end=')' contains=@ArithParenList


" Unused: {{{1
" =======
" syntax region beMatches matchgroup=beMatchFor    start="\<foreach\|for\>" end="\<endfor\>" contains=ALL
" syntax region beMatches matchgroup=beMatchWith   start="\<with\>" end="\<endwith\>" contains=ALL
" syntax region beMatches matchgroup=beMatchWhen   start="\<when\>" end="\<endwhen\>" contains=ALL
" syntax region beMatches matchgroup=beMatchIf     start="\<if\>" end="\<endif\>" contains=ALL
" syntax region beMatches matchgroup=beMatchDefine start="\<define\>" end="\<enddef\>" contains=ALL
" syntax region beMatches matchgroup=beMatchWhile  start="\<while\>" end="\<endwhile\>" contains=ALL
" syntax region beMatches matchgroup=beMatchTest   start="\<test\>" end="\<endtest\>" contains=ALL
" hi beMatchFor cterm=bold ctermfg=yellow ctermbg=black
" hi beMatchWhen cterm=bold ctermfg=yellow ctermbg=black
" hi beMatchIf cterm=bold ctermfg=yellow ctermbg=black
" hi beMatchDefine cterm=bold ctermfg=yellow ctermbg=black
" hi beMatchWhile cterm=bold ctermfg=yellow ctermbg=black
" hi beMatchTest cterm=bold ctermfg=yellow ctermbg=black

" Synchronization: {{{1
" ================
if !exists("sh_minlines")
  let sh_minlines = 400
endif
if !exists("sh_maxlines")
  let sh_maxlines = 2 * sh_minlines
endif
exec "syn sync minlines=" . sh_minlines . " maxlines=" . sh_maxlines


hi def link folVarLHS       colfolident
hi def link folVarRHS       colfolident
hi def link folVarGroup     colfolident

" Default Highlighting: {{{1
" =====================
hi def link CaseDoubleQuote	DoubleQuote
hi def link Quote	Operator
hi def link CaseSingleQuote	SingleQuote
hi def link Colon	Comment
hi def link DoubleQuote	String
hi def link Loop	statements
hi def link NoQuote	DoubleQuote
hi def link Pattern	String
hi def link Paren	Arithmetic
hi def link QuickComment	Comment
hi def link Range	Operator
hi def link SingleQuote	String
hi def link Source	Operator
hi def link SubShRegion	Operator
hi def link WrapLineOperator	Operator

hi def link time_functions functionlist
hi def link list_functions functionlist
hi def link conversion_functions functionlist
hi def link internal_functions functionlist
hi def link package_functions functionlist
hi def link math_functions functionlist
hi def link file_functions functionlist
hi def link web_functions functionlist
hi def link db_functions functionlist
hi def link string_functions functionlist
hi def link env_functions functionlist
hi def link html_functions functionlist
hi def link image_functions functionlist
hi def link udf_functions userfunctionlist

if !exists("g:sh_no_error")
 hi def link CondError		Error
 hi def link WhenError		Error
 hi def link IfError		Error
 hi def link InError		Error
endif

hi def link Arithmetic		    Special
hi def link SnglCase		    statements
hi def link Comment		        comment 
hi def link Conditional	    	Conditional
hi def link CtrlSeq		        Special
hi def link ExprRegion	    	Delimiter
hi def link Operator		    Operator
hi def link Set		            statements
hi def link assignStatements	statements
hi def link StringLiteral		String
hi def link folBash             colfolbash

hi Normal       ctermfg=white ctermbg=NONE
hi comment      ctermfg=Red
hi Constant     ctermfg=darkGreen cterm=bold
hi colfolident  ctermfg=Green cterm=bold
hi statements   ctermfg=Cyan
hi tstatements  ctermfg=Magenta
hi colfolbash   ctermfg=Red cterm=bold
hi colfolcc     ctermfg=lightBlue
hi colfolvar    ctermfg=darkYellow
hi ErrorMsg     ctermfg=black ctermbg=red
hi WarningMsg   ctermfg=black ctermbg=green
hi Error        ctermbg=Red
hi functionlist ctermfg=Blue cterm=italic
hi userfunctionlist ctermfg=darkYellow cterm=italic
hi Search       ctermbg=darkGray ctermfg=lightCyan
hi LineNr       ctermfg=blue
hi title        ctermfg=darkGray
hi ShowMarksHL  cterm=bold ctermfg=yellow ctermbg=black
hi StatusLineNC ctermfg=lightBlue ctermbg=darkBlue
hi StatusLine   cterm=bold    ctermfg=cyan  ctermbg=blue
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

hi Number       ctermfg=white
hi String ctermfg=Green

" Set Current Syntax: {{{1
" ===================
let b:current_syntax = "za"

