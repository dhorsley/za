#include "test_nested_struct_lib.h"

Rect make_rect(int x1, int y1, int x2, int y2) {
    Rect r;
    r.topLeft.x = x1;
    r.topLeft.y = y1;
    r.bottomRight.x = x2;
    r.bottomRight.y = y2;
    return r;
}

int rect_area(Rect r) {
    int width = r.bottomRight.x - r.topLeft.x;
    int height = r.bottomRight.y - r.topLeft.y;
    return width * height;
}

int rect_width(Rect r) {
    return r.bottomRight.x - r.topLeft.x;
}

int rect_height(Rect r) {
    return r.bottomRight.y - r.topLeft.y;
}

Rect rect_translate(Rect r, Point delta) {
    r.topLeft.x += delta.x;
    r.topLeft.y += delta.y;
    r.bottomRight.x += delta.x;
    r.bottomRight.y += delta.y;
    return r;
}

Point make_point(int x, int y) {
    Point p;
    p.x = x;
    p.y = y;
    return p;
}
