#include <string.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <gd.h>
#include <gdfontl.h>

// Internal: render a string to an RGBA buffer with given foreground color.
// out_width and out_height are filled with the rendered dimensions.
// Returns allocated buffer on success, NULL on failure.
static uint8_t* render_string_internal(const char *text, const char *font_path,
                                       int *out_width, int *out_height,
                                       int fg_r, int fg_g, int fg_b) {
    if (!text || !out_width || !out_height) return NULL;

    double font_size = 48.0;

    // Two-call gdImageStringFT pattern: first call gets bounding box, second renders.
    int brect[8];
    char *error = gdImageStringFT(NULL, brect, 0, font_path, font_size, 0.0, 0, 0, (char *)text);
    if (error) return NULL;

    int min_x = brect[0], max_x = brect[2];
    int min_y = brect[7], max_y = brect[1];
    int rendered_width = max_x - min_x + 4;
    int rendered_height = max_y - min_y + 4;
    if (rendered_width <= 0) rendered_width = 10;
    if (rendered_height <= 0) rendered_height = 10;

    gdImagePtr img = gdImageCreateTrueColor(rendered_width, rendered_height);
    if (!img) return NULL;

    int transparent_black_bg = gdImageColorAllocateAlpha(img, 0, 0, 0, 127);
    int fg_color = gdImageColorAllocateAlpha(img, fg_r, fg_g, fg_b, 0);
    gdImageFill(img, 0, 0, transparent_black_bg);

    int text_x = 2 - min_x;
    int text_y = 2 - min_y;
    error = gdImageStringFT(img, brect, fg_color, font_path, font_size, 0.0, text_x, text_y, (char *)text);
    if (error) { gdImageDestroy(img); return NULL; }

    int buffer_size = rendered_width * rendered_height * 4;
    uint8_t *output_buffer = malloc(buffer_size);
    if (!output_buffer) { gdImageDestroy(img); return NULL; }

    for (int y = 0; y < rendered_height; y++) {
        memcpy(output_buffer + y * rendered_width * 4, img->tpixels[y], rendered_width * 4);
    }
    // Convert BGRA→RGBA and invert alpha in one pass
    for (int i = 0; i < buffer_size; i += 4) {
        uint8_t gd_alpha = output_buffer[i + 3] & 0x7F;
        output_buffer[i + 3] = (255 * (127 - gd_alpha)) / 127;
        uint8_t tmp = output_buffer[i];
        output_buffer[i] = output_buffer[i + 2];
        output_buffer[i + 2] = tmp;
    }

    gdImageDestroy(img);
    *out_width = rendered_width;
    *out_height = rendered_height;
    return output_buffer;
}

// White text renderer
uint8_t* render_string_gdlib(const char *text, const char *font_path,
                             int *out_width, int *out_height) {
    return render_string_internal(text, font_path, out_width, out_height, 255, 255, 255);
}

// Colored text renderer — foreground RGB is passed in
uint8_t* render_string_gdlib_colored(const char *text, const char *font_path,
                                     int *out_width, int *out_height,
                                     uint8_t r, uint8_t g, uint8_t b) {
    return render_string_internal(text, font_path, out_width, out_height, r, g, b);
}

typedef struct {
    char *text;
    uint8_t r, g, b;
    int line;
    int x;
    int w, h;
    uint8_t *buffer;
} MarkupSegment;

// Render a string with embedded color markup into a single RGBA buffer.
// Markup syntax:
//   {r,g,b}   set foreground color (e.g. {0,255,0})
//   {}        reset to default color
//   \n        start a new line
// The default color is supplied by default_r/g/b.
uint8_t *render_string_gdlib_markup(const char *text, const char *font_path,
                                    int *out_width, int *out_height,
                                    uint8_t default_r, uint8_t default_g, uint8_t default_b) {
    if (!text || !out_width || !out_height) return NULL;

    int capacity = 16;
    int count = 0;
    MarkupSegment *segments = malloc(capacity * sizeof(MarkupSegment));
    if (!segments) return NULL;

    uint8_t cr = default_r, cg = default_g, cb = default_b;
    int line = 0;
    int text_len = strlen(text);
    int buf_cap = 256;
    char *buf = malloc(buf_cap);
    if (!buf) { free(segments); return NULL; }
    int buf_len = 0;
    int *line_widths = NULL;
    int *line_heights = NULL;

    for (int i = 0; i < text_len; i++) {
        char c = text[i];
        if (c == '{') {
            // Flush current buffer as a segment
            if (buf_len > 0) {
                if (count >= capacity) {
                    capacity *= 2;
                    MarkupSegment *new_seg = realloc(segments, capacity * sizeof(MarkupSegment));
                    if (!new_seg) { goto cleanup; }
                    segments = new_seg;
                }
                buf[buf_len] = '\0';
                segments[count].text = strdup(buf);
                segments[count].r = cr;
                segments[count].g = cg;
                segments[count].b = cb;
                segments[count].line = line;
                segments[count].x = 0;  // computed later
                segments[count].w = 0;
                segments[count].h = 0;
                segments[count].buffer = NULL;
                count++;
                buf_len = 0;
            }
            // Parse color tag
            i++;
            int j = i;
            while (j < text_len && text[j] != '}') j++;
            if (j < text_len && text[j] == '}') {
                if (j == i) {
                    // empty tag {} resets to default
                    cr = default_r; cg = default_g; cb = default_b;
                } else {
                    int r = 0, g = 0, b = 0;
                    sscanf(text + i, "%d,%d,%d", &r, &g, &b);
                    cr = (uint8_t)r; cg = (uint8_t)g; cb = (uint8_t)b;
                }
                i = j;
            }
        } else if (c == '\n') {
            if (buf_len > 0) {
                if (count >= capacity) {
                    capacity *= 2;
                    MarkupSegment *new_seg = realloc(segments, capacity * sizeof(MarkupSegment));
                    if (!new_seg) { goto cleanup; }
                    segments = new_seg;
                }
                buf[buf_len] = '\0';
                segments[count].text = strdup(buf);
                segments[count].r = cr;
                segments[count].g = cg;
                segments[count].b = cb;
                segments[count].line = line;
                segments[count].x = 0;
                segments[count].w = 0;
                segments[count].h = 0;
                segments[count].buffer = NULL;
                count++;
                buf_len = 0;
            }
            line++;
        } else {
            if (buf_len + 1 >= buf_cap) {
                buf_cap *= 2;
                char *new_buf = realloc(buf, buf_cap);
                if (!new_buf) { goto cleanup; }
                buf = new_buf;
            }
            buf[buf_len++] = c;
        }
    }

    // Flush remaining buffer
    if (buf_len > 0) {
        if (count >= capacity) {
            capacity *= 2;
            MarkupSegment *new_seg = realloc(segments, capacity * sizeof(MarkupSegment));
            if (!new_seg) { goto cleanup; }
            segments = new_seg;
        }
        buf[buf_len] = '\0';
        segments[count].text = strdup(buf);
        segments[count].r = cr;
        segments[count].g = cg;
        segments[count].b = cb;
        segments[count].line = line;
        segments[count].x = 0;
        segments[count].w = 0;
        segments[count].h = 0;
        segments[count].buffer = NULL;
        count++;
    }
    free(buf);
    buf = NULL;

    if (count == 0) {
        *out_width = 10;
        *out_height = 10;
        free(segments);
        uint8_t *empty = malloc(10 * 10 * 4);
        if (empty) memset(empty, 0, 10 * 10 * 4);
        return empty;
    }

    // Render each segment
    int num_lines = 1;
    for (int i = 0; i < count; i++) {
        int w = 0, h = 0;
        segments[i].buffer = render_string_gdlib_colored(segments[i].text, font_path, &w, &h,
                                                         segments[i].r, segments[i].g, segments[i].b);
        segments[i].w = w;
        segments[i].h = h;
        if (segments[i].line + 1 > num_lines) num_lines = segments[i].line + 1;
    }

    // Compute x offsets per line and line heights
    if (num_lines > 0) {
        line_widths = calloc(num_lines, sizeof(int));
        line_heights = calloc(num_lines, sizeof(int));
        if (!line_widths || !line_heights) { goto cleanup; }
    }

    for (int i = 0; i < count; i++) {
        segments[i].x = line_widths[segments[i].line];
        line_widths[segments[i].line] += segments[i].w;
        if (segments[i].h > line_heights[segments[i].line]) {
            line_heights[segments[i].line] = segments[i].h;
        }
    }

    int total_width = 0;
    int total_height = 0;
    for (int i = 0; i < num_lines; i++) {
        if (line_widths[i] > total_width) total_width = line_widths[i];
        total_height += line_heights[i];
    }

    // Allocate output buffer
    int buffer_size = total_width * total_height * 4;
    uint8_t *output = malloc(buffer_size);
    if (!output) { goto cleanup; }
    memset(output, 0, buffer_size);

    // Blit segments into output buffer
    for (int i = 0; i < count; i++) {
        if (!segments[i].buffer) continue;
        int y_offset = 0;
        for (int l = 0; l < segments[i].line; l++) {
            y_offset += line_heights[l];
        }
        int x_offset = segments[i].x;
        // Center vertically within the line if segment is shorter than line height
        int y_base = y_offset + (line_heights[segments[i].line] - segments[i].h) / 2;
        for (int y = 0; y < segments[i].h; y++) {
            for (int x = 0; x < segments[i].w; x++) {
                int src_idx = (y * segments[i].w + x) * 4;
                int dst_idx = ((y_base + y) * total_width + (x_offset + x)) * 4;
                // Simple alpha blend: if source alpha > 0, copy RGBA
                uint8_t a = segments[i].buffer[src_idx + 3];
                if (a > 0) {
                    output[dst_idx] = segments[i].buffer[src_idx];
                    output[dst_idx + 1] = segments[i].buffer[src_idx + 1];
                    output[dst_idx + 2] = segments[i].buffer[src_idx + 2];
                    output[dst_idx + 3] = a;
                }
            }
        }
    }

    *out_width = total_width;
    *out_height = total_height;

    // Cleanup
    for (int i = 0; i < count; i++) {
        free(segments[i].text);
        if (segments[i].buffer) free(segments[i].buffer);
    }
    free(segments);
    free(line_widths);
    free(line_heights);
    return output;

cleanup:
    if (buf) free(buf);
    for (int i = 0; i < count; i++) {
        free(segments[i].text);
        if (segments[i].buffer) free(segments[i].buffer);
    }
    free(segments);
    free(line_widths);
    free(line_heights);
    return NULL;
}

// Free bitmap allocated by render_string_gdlib
void free_gd_bitmap(uint8_t *bitmap) {
    if (bitmap) free(bitmap);
}

