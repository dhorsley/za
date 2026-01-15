// Test header for typedef parsing in AUTO clause
// Tests simple typedefs, typedef chains, struct typedefs, and pointer typedefs

// ===== SIMPLE TYPEDEFS =====

// Integer typedefs (common in headers)
typedef unsigned int uint32_t;
typedef unsigned long size_t;
typedef int ssize_t;
typedef unsigned char uint8_t;

// ===== TYPEDEF CHAINS =====

// Test recursive typedef resolution
typedef int MyInt;
typedef MyInt MyInt2;
typedef MyInt2 MyInt3;

// ===== STRUCT TYPEDEFS =====

// Named struct typedef
typedef struct Point {
    int x;
    int y;
} Point;

// Anonymous struct typedef
typedef struct {
    float r, g, b;
} Color;

// ===== POINTER TYPEDEFS =====

// Pointer to struct
typedef Color* ColorPtr;
typedef Point* PointPtr;

// Generic pointers
typedef void* VoidPtr;
typedef char* CharPtr;

// ===== TEST FUNCTIONS USING TYPEDEFS =====

// Functions using simple typedefs
uint32_t get_count(void);
size_t get_size(void);
ssize_t read_data(int fd);

// Functions using typedef chains
MyInt3 compute(MyInt input);
MyInt2 transform(MyInt2 value);

// Functions using struct typedefs
Point create_point(int x, int y);
Color get_default_color(void);

// Functions using pointer typedefs
ColorPtr allocate_color(void);
PointPtr create_point_ptr(int x, int y);
VoidPtr get_handle(void);

// Functions with typedef parameters
void set_size(size_t sz);
void process_point(PointPtr pt);
void set_color(ColorPtr color);
