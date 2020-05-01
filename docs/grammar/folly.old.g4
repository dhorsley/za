
/** ANTLR grammar for the za language. */

grammar za;


program : statement+ EOF ;

primaryExpression
    : Identifier
    | Constant
    | StringLiteral+
    | '(' expression ')'
    ;

postfixExpression
    : primaryExpression                                     // expression
    | postfixExpression '(' argumentExpressionList? ')'     // function calls
    | postfixExpression '.' Identifier                      // dotted fields
    ;

argumentExpressionList
    : assignmentExpression
    | argumentExpressionList ',' assignmentExpression
    ;

unaryExpression
    : postfixExpression
    | unaryOperator castExpression
    ;

unaryOperator: '+' | '-' | '!' ;

castExpression
    : unaryExpression
    | DigitSequence
    ;

multiplicativeExpression
    : castExpression
    |   multiplicativeExpression '*' castExpression
    |   multiplicativeExpression '/' castExpression
    |   multiplicativeExpression '%' castExpression
    ;

additiveExpression
    :   multiplicativeExpression
    |   additiveExpression '+' multiplicativeExpression
    |   additiveExpression '-' multiplicativeExpression
    ;

shiftExpression
    :   additiveExpression
    |   shiftExpression '<<' additiveExpression
    |   shiftExpression '>>' additiveExpression
    ;

relationalExpression
    :   shiftExpression
    |   relationalExpression '<' shiftExpression
    |   relationalExpression '>' shiftExpression
    |   relationalExpression '<=' shiftExpression
    |   relationalExpression '>=' shiftExpression
    ;

equalityExpression
    :   relationalExpression
    |   equalityExpression '==' relationalExpression
    |   equalityExpression '!=' relationalExpression
    ;


andExpression
    :   equalityExpression
    |   andExpression '&' equalityExpression
    ;

exclusiveOrExpression
    :   andExpression
    |   exclusiveOrExpression '^' andExpression
    ;

inclusiveOrExpression
    :   exclusiveOrExpression
    |   inclusiveOrExpression '|' exclusiveOrExpression
    ;

logicalAndExpression
    :   inclusiveOrExpression
    |   logicalAndExpression '&&' inclusiveOrExpression
    ;

logicalOrExpression
    :   logicalAndExpression
    |   logicalOrExpression '||' logicalAndExpression
    ;

conditionalExpression
    :   logicalOrExpression ('?' expression? ':' conditionalExpression)?
    ;

assignmentExpression
    : conditionalExpression
    | unaryExpression assignmentOperator assignmentExpression
    | DigitSequence
    ;

assignmentOperator
    : '='
    | '=|'
    ;

expression
    : assignmentExpression
    | expression ',' assignmentExpression
    | expression '..' expression
    ;

constantExpression
    : conditionalExpression
    ;


rangeExpression
    :  expression ('..' expression)?
    ;


/* String Handling **************************/

fragment
EscapeSequence
    : SimpleEscapeSequence
    ;

fragment
SimpleEscapeSequence
    : '\\' ['"?abfnrtv\\]
    ;

fragment
OctalEscapeSequence
    :   '\\' OctalDigit
    |   '\\' OctalDigit OctalDigit
    |   '\\' OctalDigit OctalDigit OctalDigit
    ;
fragment
HexadecimalEscapeSequence
    :   '\\x' HexDigit+
    ;


StringLiteral
    :   ( '"' ( SCharSequenceDQ | Lendings )? '"' )
        | ( '\'' ( SCharSequenceSQ | Lendings )? '\'' ) 
    ;

fragment
SCharSequenceDQ    : SCharDQ+    ;

fragment
SCharDQ
    : ~["\\\r\n]
    | EscapeSequence
    ;

fragment
Lendings
    :
    | '\\\n'
    | '\\\r\n'
    ;

fragment
SCharSequenceSQ    : SCharSQ+    ;

fragment
SCharSQ 
    : ~['\\\r\n]
    | EscapeSequence
    ;


fragment
OctalDigit
    :   [0-7]
    ;

/****************************************/


/*
literalRegex
    : '"' RegexBrace1 .*? RegexBrace1 '"'  // #regex#
    | '"' RegexBrace2 .*? RegexBrace2 '"'  // /regex/
    | '"' Sb .*? Eb '"'                    // {regex}
    ;
*/

nestedParenthesesBlock
    :   (   ~('(' | ')')
        |   '(' nestedParenthesesBlock ')'
        )*
    ;

/*
RegexBrace1: '#';
RegexBrace2: '/';
*/


/** END OF EXPRESSIONS */


/** STATEMENTS */

/** Assignment */
C_Set: 'set';
C_Zero: 'zero';
C_Inc: 'inc';
C_Dec: 'dec';
C_LetcRemote: 'letc@';
C_Letc: 'letc';

assignment_control
    : assign_rule expression END
    | ( zero_rule | inc_rule | dec_rule ) Identifier END
    | remote_assign_rule Identifier '='? userhostExpression expression END
    ;

assign_rule
    : C_Set Identifier '='? expression
    ;

zero_rule: C_Zero ;
inc_rule: C_Inc ;
dec_rule: C_Dec ;
remote_assign_rule : C_LetcRemote ;

userhostExpression
    : expression '@'? expression;


/** CLI Interfacing */
C_LocalCommand: '|';
C_RemoteCommand: '|@';

cli_control
    : local_command_rule expression END
    | remote_command_rule userhostExpression expression END
    ;

local_command_rule: C_LocalCommand ;
remote_command_rule: C_RemoteCommand ;


/** Miscellaneous */
C_Install: 'install';
C_Push: 'push';             // pushes metric and value to graphite
C_Trigger: 'trigger';
C_Download: 'download';
C_Pause: 'pause';
C_Help: 'help';

miscellaneous_control
    : install_rule expression END
    | push_rule ( 'graphite' ) expression expression END
    | trigger_rule ( 'script' ) expression C_If expression C_Is expression END
    | download_rule expression expression END //url+dest_location
    | pause_rule expression END
    | help_rule expression? END
    ;

install_rule : C_Install ;
push_rule : C_Push ;
trigger_rule : C_Trigger ;
download_rule : C_Download ;
pause_rule : C_Pause ;
help_rule : C_Help ;

/* Option set for C_Trigger 'script' C_if:
trigger_option_set
    : ( 'ACCESS'
        | 'ATTRIB'
        | 'CLOSE_WRITE'
        | 'CLOSE_NOWRITE'
        | 'CREATE'
        | 'DELETE'
        | 'DELETE_SELF'
        | 'MODIFY'
        | 'IN_MOVE_SELF'
        | 'MOVED_FROM'
        | 'MOVED_TO'
        | 'OPEN'
        | 'DONT_FOLLOW'
        | 'ONESHOT'
        | 'ONLYDIR'
        | 'ALL_EVENTS'
        | 'MOVE'
        | 'CLOSE'
    );
*/


/** Program Control */
C_Nop: 'nop';
C_Debug: 'debug';
C_Require: 'require';
C_Depends: 'depends';
C_Version: 'version';
C_Exit: 'exit';

//     | C_Require Version_Format

program_control
    : nop_rule END
    | debug_rule ( 'off' | 'on' ) END
    | depends_rule 
        ( 
            ( 'yum' | 'zypper' | 'apt' ) expression packageConstants // expr = package_name
            | 'jira' expression ticketConstants // expr=ticket_id
        ) END
    | version_rule versionFormat? END
    | exit_rule ( expression )? END
    ;

nop_rule : C_Nop ;
debug_rule : C_Debug ;
depends_rule : C_Depends ;
exit_rule : C_Exit ;
version_rule : C_Version ;
versionFormat: ( DigitSequence '.' DigitSequence '.' DigitSequence ) ;

packageConstants : ( 'Installed' | 'Available' ) ;
ticketConstants  : ( 'Closed' | 'Open' ) ;


/** I/O */
C_Quiet: 'quiet';
C_Loud: 'loud';
C_Input: 'input';
C_Prompt: 'prompt';
C_Indent: 'indent';
C_Print: 'print';
C_Log: 'log';
C_Logging: 'logging';
C_Cls: 'cls';
C_At: 'at';

vector_2
    : expression ',' expression
    ;

W_Off: 'off';
W_On: 'on';

io_control
    : quiet_rule END
    | loud_rule END
    | input_rule Identifier ( 'param' | 'optarg' | 'key' ) expression END
    | prompt_rule Identifier expression expression expression? END   // exp1=prompt string, exp2=timeout, exp3=regex input loop filter
    | indent_rule expression END
    | (log_rule | print_rule ) expression END
    | logging_rule ( W_Off | W_On expression? ) END
    | cls_rule END
    | at_rule vector_2 END
    ;

quiet_rule : C_Quiet ;
loud_rule : C_Loud ;
input_rule : C_Input ;
prompt_rule : C_Prompt ;
indent_rule : C_Indent ;
log_rule : C_Log ;
print_rule : C_Print ;
logging_rule : C_Logging ;
cls_rule : C_Cls ;
at_rule : C_At ;


/** Procedures */
C_Define: 'define';
C_Enddef: 'enddef';
C_Showdef: 'showdef';
C_Return: 'return';

procedure_control
    : define_rule Identifier statement+ enddef_rule
    | showdef_rule expression END
    | return_rule expression? END
    ;

define_rule : C_Define ;
showdef_rule : C_Showdef ;
return_rule : C_Return ;
enddef_rule : C_Enddef ;


/** Modules */
C_Lib: 'lib';
C_Module: 'module';
C_Uses: 'uses';

module_control
    : ( lib_rule expression
    | module_rule expression
    | uses_rule expression ) END
    ;

lib_rule : C_Lib ;
module_rule : C_Module ;
uses_rule : C_Uses ;


/** Control Flow */

C_While: 'while';
C_Endwhile: 'endwhile';

C_For: 'for';
C_To: 'to';
C_Step: 'step';
C_Foreach: 'foreach';
C_Endfor: 'endfor';

iterationStatement
    : while_rule expression statement+ endwhile_rule
    | for_rule Identifier '='? expression C_To? expression (C_Step expression)? statement+ endfor_rule
    | foreach_rule Identifier C_In ('file' | 'var' expression) statement+ endfor_rule
    ; 

while_rule : C_While ;
endwhile_rule : C_Endwhile ;
for_rule : C_For ;
foreach_rule : C_Foreach ;
endfor_rule : C_Endfor ; 


jumpStatement
    :   ( continue_rule
    |   break_rule ) END
    ;

continue_rule : C_Continue ;
break_rule : C_Break ;

C_Break: 'break';
C_Continue: 'continue';

C_If: 'if';
C_Else: 'else';
C_Endif: 'endif';

selectionStatement
    : if_rule expression statement+ (else_rule statement+)? endif_rule
    | when_rule expression when_List endwhen_rule
    ;

if_rule : C_If ;
else_rule : C_Else ;
endif_rule : C_Endif ;
when_rule : C_When ;
endwhen_rule : C_Endwhen ;

C_When: 'when';
C_Is: 'is';
C_Contains: 'contains';
C_In: 'in';
C_Or: 'or';
C_Endwhen: 'endwhen';

when_Term
    : when_Is_Clause
    | when_Contains_Clause
    | when_In_Clause
    | when_Or_Clause
    ;

when_Is_Clause
    : C_Is expression statement+
    ;

when_Contains_Clause
    : C_Contains expression statement+   // expression = regex
    ;

when_In_Clause
    : C_In rangeExpression statement+
    ;
    
when_Or_Clause
    : C_Or statement+
    ;
    
when_List
    : when_Term
    | when_List when_Term
    ;


statement
    : cli_control
    | miscellaneous_control
    | program_control 
    | io_control
    | procedure_control
    | module_control
    | iterationStatement
    | selectionStatement
    | expressionStatement
    | jumpStatement
    ;

END : ';' ;

expressionStatement
    : expression END
    ;


/** BASIC DEFINITIONS AND FRAGMENTS */

Identifier 
    : IdentifierNondigit 
        ( IdentifierNondigit 
        | Digit 
        )*
    ;

    fragment
    IdentifierNondigit
        : Nondigit
        | UniversalCharacterName
        ;

    fragment
    Nondigit
        : [a-zA-Z_]
        ;

    fragment
    Digit
        : [0-9] 
        ;

    fragment
    UniversalCharacterName
        : '\\u' HexQuad
        | '\\U' HexQuad HexQuad
        ;

    fragment
    HexQuad
        : HexDigit HexDigit HexDigit HexDigit 
        ;


Constant
    : IntegerConstant
    | FloatingConstant
    ;

    fragment
    IntegerConstant
        : DecimalConstant
        ;

    fragment
    DecimalConstant
        : NonzeroDigit Digit*
        ;

    fragment
    NonzeroDigit
        : [1-9]
        ;

    fragment
    FloatingConstant
        : DecimalFloatingConstant
        ;

    fragment
    DecimalFloatingConstant
        : FractionalConstant ExponentPart?
        | DigitSequence ExponentPart?
        ;

    fragment
    FractionalConstant
        :   DigitSequence? '.' DigitSequence
        |   DigitSequence '.'
        ;

    fragment
    ExponentPart
        :   'e' Sign? DigitSequence
        |   'E' Sign? DigitSequence
        ;

    fragment
    Sign
        :   '+' | '-'
        ;

DigitSequence
   : Digit+
   ;    

fragment
HexDigit
    : [0-9a-fA-F]
    ;

Whitespace
    :   [ \t]+
        -> skip
    ;

Newline
    :   (   '\r' '\n'?
        |   '\n'
        )
        -> skip
    ;

BlockComment
    :   '/*' .*? '*/'
        -> skip
    ;

LineComment
    :   (   '//' ~[\r\n]*
        |   '#' ~[\r\n]*
        )
        -> skip
    ;

Sb: '{';
Eb: '}';

