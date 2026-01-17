#ifndef TEST_NESTED_STRUCT_H
#define TEST_NESTED_STRUCT_H

// Simple 2D point
typedef struct {
    int x;
    int y;
} Point;

// Rectangle defined by two points
typedef struct {
    Point topLeft;
    Point bottomRight;
} Rect;

// Create a rectangle from coordinates
Rect make_rect(int x1, int y1, int x2, int y2);

// Calculate rectangle area
int rect_area(Rect r);

// Calculate width and height
int rect_width(Rect r);
int rect_height(Rect r);

// Translate a rectangle by a delta point
Rect rect_translate(Rect r, Point delta);

// Create a point from coordinates
Point make_point(int x, int y);

#endif
