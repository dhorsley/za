//go:build !test

package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
)

func buildGzipLib() {

	features["gzip"] = Feature{version: 1, category: "file"}
	categories["gzip"] = []string{"gzip_compress", "gzip_decompress", "gzip_compress_bytes", "gzip_decompress_bytes"}

	// Helper to parse optional options map for overwrite and compression level.
	// Level defaults to gzip.DefaultCompression (-1) if not specified.
	parseGzipOptions := func(args []any) (overwrite bool, level int, err error) {
		overwrite = true
		level = gzip.DefaultCompression
		if len(args) == 3 {
			options, ok := args[2].(map[string]any)
			if !ok {
				return true, gzip.DefaultCompression, fmt.Errorf("gzip options must be a map")
			}
			if v, ok := options["overwrite"].(bool); ok {
				overwrite = v
			}
			if v, ok := options["level"]; ok {
				switch lv := v.(type) {
				case int:
					level = lv
				case int64:
					level = int(lv)
				case float64:
					level = int(lv)
				case uint:
					level = int(lv)
				case uint64:
					level = int(lv)
				case string:
					switch lv {
					case "default":
						level = gzip.DefaultCompression
					case "none", "no":
						level = gzip.NoCompression
					case "fast", "speed":
						level = gzip.BestSpeed
					case "best":
						level = gzip.BestCompression
					case "huffman":
						level = gzip.HuffmanOnly
					default:
						return true, gzip.DefaultCompression, fmt.Errorf("gzip unknown level alias: %q", lv)
					}
				default:
					return true, gzip.DefaultCompression, fmt.Errorf("gzip level must be an integer or string alias")
				}
				if level < gzip.HuffmanOnly || level > gzip.BestCompression {
					return true, gzip.DefaultCompression, fmt.Errorf("gzip level out of range: %d (valid range: -2 to 9)", level)
				}
			}
		}
		return overwrite, level, nil
	}

	// createGzipWriter returns a *gzip.Writer configured with the given level.
	createGzipWriter := func(w io.Writer, level int) (*gzip.Writer, error) {
		if level == gzip.DefaultCompression {
			return gzip.NewWriter(w), nil
		}
		return gzip.NewWriterLevel(w, level)
	}

	slhelp["gzip_compress"] = LibHelp{in: "source_file, dest_file[, options_map]", out: "bool", action: "Compress a file using gzip. Options: map(.overwrite true, .level int|string). Level aliases: \"default\", \"none\"/\"no\", \"fast\"/\"speed\", \"best\", \"huffman\"."}
	stdlib["gzip_compress"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("gzip_compress", args, 2,
			"2", "string", "string",
			"3", "string", "string", "map"); !ok {
			return nil, err
		}

		sourceFile := args[0].(string)
		destFile := args[1].(string)

		overwrite, level, err := parseGzipOptions(args)
		if err != nil {
			return false, err
		}

		if !overwrite {
			if _, err := os.Stat(destFile); err == nil {
				return false, fmt.Errorf("gzip_compress error: destination file '%s' already exists (overwrite is false)", destFile)
			}
		}

		sourceData, err := os.ReadFile(sourceFile)
		if err != nil {
			return false, fmt.Errorf("gzip_compress error reading source file: %v", err)
		}

		var buf bytes.Buffer
		gz, err := createGzipWriter(&buf, level)
		if err != nil {
			return false, fmt.Errorf("gzip_compress error creating compressor: %v", err)
		}
		_, err = gz.Write(sourceData)
		if err != nil {
			gz.Close()
			return false, fmt.Errorf("gzip_compress error compressing data: %v", err)
		}
		if err = gz.Close(); err != nil {
			return false, fmt.Errorf("gzip_compress error finalizing compression: %v", err)
		}

		err = os.WriteFile(destFile, buf.Bytes(), default_WriteMode)
		if err != nil {
			return false, fmt.Errorf("gzip_compress error writing destination file: %v", err)
		}

		return true, nil
	}

	slhelp["gzip_decompress"] = LibHelp{in: "source_file, dest_file[, options_map]", out: "bool", action: "Decompress a gzip file. Options: map(.overwrite true)."}
	stdlib["gzip_decompress"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("gzip_decompress", args, 2,
			"2", "string", "string",
			"3", "string", "string", "map"); !ok {
			return nil, err
		}

		sourceFile := args[0].(string)
		destFile := args[1].(string)

		overwrite, _, err := parseGzipOptions(args)
		if err != nil {
			return false, err
		}

		if !overwrite {
			if _, err := os.Stat(destFile); err == nil {
				return false, fmt.Errorf("gzip_decompress error: destination file '%s' already exists (overwrite is false)", destFile)
			}
		}

		sourceData, err := os.ReadFile(sourceFile)
		if err != nil {
			return false, fmt.Errorf("gzip_decompress error reading source file: %v", err)
		}

		gz, err := gzip.NewReader(bytes.NewReader(sourceData))
		if err != nil {
			return false, fmt.Errorf("gzip_decompress error initializing decompressor: %v", err)
		}
		defer gz.Close()

		decompressed, err := io.ReadAll(gz)
		if err != nil {
			return false, fmt.Errorf("gzip_decompress error decompressing data: %v", err)
		}

		err = os.WriteFile(destFile, decompressed, default_WriteMode)
		if err != nil {
			return false, fmt.Errorf("gzip_decompress error writing destination file: %v", err)
		}

		return true, nil
	}

	slhelp["gzip_compress_bytes"] = LibHelp{in: "data[, options_map]", out: "[]uint8", action: "Compress a byte array using gzip. Options: map(.level int|string). Level aliases: \"default\", \"none\"/\"no\", \"fast\"/\"speed\", \"best\", \"huffman\"."}
	stdlib["gzip_compress_bytes"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("gzip_compress_bytes", args, 2,
			"1", "[]uint8",
			"2", "[]uint8", "map"); !ok {
			return nil, err
		}

		data := args[0].([]uint8)

		var optionsArgs []any
		if len(args) == 1 {
			optionsArgs = []any{"", ""}
		} else {
			optionsArgs = []any{"", "", args[1]}
		}
		_, level, err := parseGzipOptions(optionsArgs)
		if err != nil {
			return nil, err
		}

		var buf bytes.Buffer
		gz, err := createGzipWriter(&buf, level)
		if err != nil {
			return nil, fmt.Errorf("gzip_compress_bytes error creating compressor: %v", err)
		}
		_, err = gz.Write(data)
		if err != nil {
			gz.Close()
			return nil, fmt.Errorf("gzip_compress_bytes error compressing data: %v", err)
		}
		if err = gz.Close(); err != nil {
			return nil, fmt.Errorf("gzip_compress_bytes error finalizing compression: %v", err)
		}

		return buf.Bytes(), nil
	}

	slhelp["gzip_decompress_bytes"] = LibHelp{in: "data", out: "[]uint8", action: "Decompress a gzip-compressed byte array."}
	stdlib["gzip_decompress_bytes"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("gzip_decompress_bytes", args, 1, "1", "[]uint8"); !ok {
			return nil, err
		}

		data := args[0].([]uint8)

		gz, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("gzip_decompress_bytes error initializing decompressor: %v", err)
		}
		defer gz.Close()

		decompressed, err := io.ReadAll(gz)
		if err != nil {
			return nil, fmt.Errorf("gzip_decompress_bytes error decompressing data: %v", err)
		}

		return decompressed, nil
	}

}
