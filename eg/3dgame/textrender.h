#ifndef TEXTRENDER_H
#define TEXTRENDER_H

/* Text rendering library for HUD and text display */

/* Render a UTF-8 string using GD library
 * Returns a pointer to an RGBA buffer (width * height * 4 bytes)
 * width and height pointers will be filled with the rendered dimensions
 */
uint8_t *render_string_gdlib(const char *text, const char *font_path,
                           int *out_width, int *out_height);

/* Colored text renderer — foreground RGB is passed in */
uint8_t *render_string_gdlib_colored(const char *text, const char *font_path,
                                    int *out_width, int *out_height,
                                    uint8_t r, uint8_t g, uint8_t b);

/* Markup text renderer — supports {r,g,b} color tags and {} reset */
uint8_t *render_string_gdlib_markup(const char *text, const char *font_path,
                                   int *out_width, int *out_height,
                                   uint8_t default_r, uint8_t default_g, uint8_t default_b);

/* Free a bitmap buffer previously allocated by render_string_gdlib */
void free_gd_bitmap(uint8_t *bitmap);

#endif /* TEXTRENDER_H */
