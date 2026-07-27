# Makefile for libtextrender.so

CC = gcc
CFLAGS = -Wall -Wextra -fPIC -O2
LDFLAGS = -shared
LIBS = -lgd

# Source files
SOURCES = textrender.c

# Object files
OBJECTS = $(SOURCES:.c=.o)

# Output library
TARGET = libtextrender.so

# Default target
all: $(TARGET)

# Build the shared library
$(TARGET): $(OBJECTS)
	$(CC) $(LDFLAGS) -o $@ $^ $(LIBS)

# Compile source files to object files
%.o: %.c
	$(CC) $(CFLAGS) -c $< -o $@

# Clean build artifacts
clean:
	rm -f $(OBJECTS) $(TARGET)

# Rebuild from scratch
rebuild: clean all

.PHONY: all clean rebuild
