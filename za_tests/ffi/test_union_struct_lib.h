// Test library for union with struct members

typedef struct {
    int x;
    int y;
} Point;

typedef struct {
    float r;
    float g;
    float b;
} Color;

typedef union {
    int as_int;
    Point as_point;
    Color as_color;
} Value;

Value make_int_value(int val);
Value make_point_value(int x, int y);
Value make_color_value(float r, float g, float b);

// Debug function to test float parameters
int test_float_arg(float f);

// Test returning Color struct directly
Color make_color_direct(float r, float g, float b);

// Test returning Value via pointer
void make_color_value_ptr(Value *out, float r, float g, float b);
