/* Test header for hex literal parsing in macro evaluation */

// Simple hex values (like GL constants)
#define GL_POLYGON_SMOOTH 0x8B65
#define GL_POLYGON_SMOOTH_HINT 0x8B66
#define GL_LIGHT_MODEL_AMBIENT 0x0B53
#define GL_LIGHT_MODEL_LOCAL_VIEWER 0x0B54

// Binary literals
#define BIT_FLAG_1 0b0001
#define BIT_FLAG_2 0b0010
#define BIT_FLAG_3 0b0100

// Mixed case hex
#define HEX_UPPERCASE 0xABCD
#define HEX_MIXED 0xAbCd
#define HEX_LOWERCASE 0xabcd

// Hex with underscores (C99)
#define HEX_WITH_UNDERSCORE 0x1234_5678

// Hex in expressions
#define HEX_OR 0xFF00 | 0x00FF
#define HEX_AND 0xFFFF & 0x00FF

// Should not cause prefix corruption
#define NORMAL_IDENTIFIER myVariable
#define ANOTHER_ID someFunction
