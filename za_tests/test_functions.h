/* Test header for AUTO function signature parsing */

/* Simple function signatures - these are real libc functions */
size_t strlen(const char *s);
void *malloc(size_t size);
void free(void *ptr);

/* Functions with multiple pointer types */
char *strcpy(char *dest, const char *src);
int strcmp(const char *s1, const char *s2);

/* Functions that should be skipped */
static inline int internal_helper(int x) { return x * 2; }  /* Should skip: static inline */
typedef int (*callback_t)(void);  /* Should skip: typedef */
#define MAX(a,b) ((a)>(b)?(a):(b))  /* Should skip: macro */
int _private_function(int x);  /* Should skip: starts with underscore */

/* Multiline declaration */
extern size_t
    fread(void *ptr, size_t size,
          size_t nmemb, FILE *stream);

/* Variadic function */
int printf(const char *format, ...);
