//go:build !test

package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func buildZipLib() {

	features["zip"] = Feature{version: 1, category: "file"}
	categories["zip"] = []string{"zip_create", "zip_create_from_dir", "zip_extract", "zip_extract_file", "zip_list", "zip_add", "zip_remove"}

	slhelp["zip_create"] = LibHelp{in: "zip_filename, files", out: "bool", action: "Create ZIP archive from list of files."}
	stdlib["zip_create"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("zip_create", args, 1, "2", "string", "[]any"); !ok {
			return nil, err
		}

		zipFilename := args[0].(string)
		filesAny := args[1]

		// Convert []any to []string if needed
		var files []string
		switch v := filesAny.(type) {
		case []string:
			files = v
		case []any:
			files = make([]string, len(v))
			for i, item := range v {
				files[i] = fmt.Sprint(item)
			}
		default:
			return false, fmt.Errorf("zip_create error: files argument must be []string or []any, got %T", filesAny)
		}

		// Create the ZIP file
		zipFile, err := os.Create(zipFilename)
		if err != nil {
			return false, fmt.Errorf("zip_create error creating file: %v", err)
		}
		defer zipFile.Close()

		// Create ZIP writer
		zipWriter := zip.NewWriter(zipFile)
		defer zipWriter.Close()

		// Add each file to the ZIP
		for _, file := range files {
			err = addFileToZip(zipWriter, file, file)
			if err != nil {
				return false, fmt.Errorf("zip_create error adding file %s: %v", file, err)
			}
		}

		return true, nil
	}

	slhelp["zip_create_from_dir"] = LibHelp{in: "zip_filename, dirpath", out: "bool", action: "Create ZIP archive from directory contents."}
	stdlib["zip_create_from_dir"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("zip_create_from_dir", args, 1, "2", "string", "string"); !ok {
			return nil, err
		}

		zipFilename := args[0].(string)
		dirPath := args[1].(string)

		// Create the ZIP file
		zipFile, err := os.Create(zipFilename)
		if err != nil {
			return false, fmt.Errorf("zip_create_from_dir error creating file: %v", err)
		}
		defer zipFile.Close()

		// Create ZIP writer
		zipWriter := zip.NewWriter(zipFile)
		defer zipWriter.Close()

		// Walk the directory and add all files
		err = filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip the root directory itself
			if path == dirPath {
				return nil
			}

			// Calculate relative path
			relPath, err := filepath.Rel(dirPath, path)
			if err != nil {
				return err
			}

			if !info.IsDir() {
				return addFileToZip(zipWriter, path, relPath)
			}

			return nil
		})

		if err != nil {
			return false, fmt.Errorf("zip_create_from_dir error: %v", err)
		}

		return true, nil
	}

	slhelp["zip_extract"] = LibHelp{in: "zip_filename, dest_dir", out: "bool", action: "Extract ZIP archive to destination directory."}
	stdlib["zip_extract"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("zip_extract", args, 1, "2", "string", "string"); !ok {
			return nil, err
		}

		zipFilename := args[0].(string)
		destDir := args[1].(string)

		// Open the ZIP file
		reader, err := zip.OpenReader(zipFilename)
		if err != nil {
			return false, fmt.Errorf("zip_extract error opening file: %v", err)
		}
		defer reader.Close()

		// Create destination directory if it doesn't exist
		err = os.MkdirAll(destDir, 0755)
		if err != nil {
			return false, fmt.Errorf("zip_extract error creating directory: %v", err)
		}

		// Extract each file
		for _, file := range reader.File {
			err = extractFileFromZip(file, destDir)
			if err != nil {
				return false, fmt.Errorf("zip_extract error extracting %s: %v", file.Name, err)
			}
		}

		return true, nil
	}

	slhelp["zip_list"] = LibHelp{in: "zip_filename", out: "[]string", action: "List contents of ZIP archive."}
	stdlib["zip_list"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("zip_list", args, 1, "1", "string"); !ok {
			return nil, err
		}

		zipFilename := args[0].(string)

		// Open the ZIP file
		reader, err := zip.OpenReader(zipFilename)
		if err != nil {
			return nil, fmt.Errorf("zip_list error opening file: %v", err)
		}
		defer reader.Close()

		// Get list of files
		var files []string
		for _, file := range reader.File {
			files = append(files, file.Name)
		}

		return files, nil
	}

	slhelp["zip_add"] = LibHelp{in: "zip_filename, files", out: "bool", action: "Add files to existing ZIP archive."}
	stdlib["zip_add"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("zip_add", args, 1, "2", "string", "[]any"); !ok {
			return nil, err
		}

		zipFilename := args[0].(string)
		filesAny := args[1]

		// Convert []any to []string if needed
		var files []string
		switch v := filesAny.(type) {
		case []string:
			files = v
		case []any:
			files = make([]string, len(v))
			for i, item := range v {
				files[i] = fmt.Sprint(item)
			}
		default:
			return false, fmt.Errorf("zip_add error: files argument must be []string or []any, got %T", filesAny)
		}

		// Open existing ZIP file for reading
		reader, err := zip.OpenReader(zipFilename)
		if err != nil {
			return false, fmt.Errorf("zip_add error opening file: %v", err)
		}
		defer reader.Close()

		// Create temporary file for new ZIP
		tempFile, err := os.CreateTemp("", "za_zip_*")
		if err != nil {
			return false, fmt.Errorf("zip_add error creating temp file: %v", err)
		}
		tempFilename := tempFile.Name()
		tempFile.Close()

		// Create new ZIP with existing contents plus new files
		newZipFile, err := os.Create(tempFilename)
		if err != nil {
			return false, fmt.Errorf("zip_add error creating new zip: %v", err)
		}

		newZipWriter := zip.NewWriter(newZipFile)

		// Copy existing files
		for _, file := range reader.File {
			err = copyFileInZip(newZipWriter, file)
			if err != nil {
				newZipWriter.Close()
				newZipFile.Close()
				os.Remove(tempFilename)
				return false, fmt.Errorf("zip_add error copying existing file %s: %v", file.Name, err)
			}
		}

		// Add new files
		for _, file := range files {
			err = addFileToZip(newZipWriter, file, file)
			if err != nil {
				newZipWriter.Close()
				newZipFile.Close()
				os.Remove(tempFilename)
				return false, fmt.Errorf("zip_add error adding file %s: %v", file, err)
			}
		}

		newZipWriter.Close()
		newZipFile.Close()

		// Replace original with new file
		err = os.Rename(tempFilename, zipFilename)
		if err != nil {
			os.Remove(tempFilename)
			return false, fmt.Errorf("zip_add error replacing file: %v", err)
		}

		return true, nil
	}

	slhelp["zip_remove"] = LibHelp{in: "zip_filename, files", out: "bool", action: "Remove files from ZIP archive."}
	stdlib["zip_remove"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("zip_remove", args, 1, "2", "string", "[]any"); !ok {
			return nil, err
		}

		zipFilename := args[0].(string)
		filesAny := args[1]

		// Convert []any to []string if needed
		var filesToRemove []string
		switch v := filesAny.(type) {
		case []string:
			filesToRemove = v
		case []any:
			filesToRemove = make([]string, len(v))
			for i, item := range v {
				filesToRemove[i] = fmt.Sprint(item)
			}
		default:
			return false, fmt.Errorf("zip_remove error: files argument must be []string or []any, got %T", filesAny)
		}

		// Create set of files to remove for efficient lookup
		removeSet := make(map[string]bool)
		for _, file := range filesToRemove {
			removeSet[file] = true
		}

		// Open existing ZIP file
		reader, err := zip.OpenReader(zipFilename)
		if err != nil {
			return false, fmt.Errorf("zip_remove error opening file: %v", err)
		}
		defer reader.Close()

		// Create temporary file for new ZIP
		tempFile, err := os.CreateTemp("", "za_zip_*")
		if err != nil {
			return false, fmt.Errorf("zip_remove error creating temp file: %v", err)
		}
		tempFilename := tempFile.Name()
		tempFile.Close()

		// Create new ZIP without the specified files
		newZipFile, err := os.Create(tempFilename)
		if err != nil {
			return false, fmt.Errorf("zip_remove error creating new zip: %v", err)
		}

		newZipWriter := zip.NewWriter(newZipFile)

		// Copy files that are not in the remove list
		for _, file := range reader.File {
			if !removeSet[file.Name] {
				err = copyFileInZip(newZipWriter, file)
				if err != nil {
					newZipWriter.Close()
					newZipFile.Close()
					os.Remove(tempFilename)
					return false, fmt.Errorf("zip_remove error copying file %s: %v", file.Name, err)
				}
			}
		}

		newZipWriter.Close()
		newZipFile.Close()

		// Replace original with new file
		err = os.Rename(tempFilename, zipFilename)
		if err != nil {
			os.Remove(tempFilename)
			return false, fmt.Errorf("zip_remove error replacing file: %v", err)
		}

		return true, nil
	}

	slhelp["zip_extract_file"] = LibHelp{in: "zip_filename, files_to_extract, dest_dir", out: "bool", action: "Extract specific files from ZIP archive to destination directory."}
	stdlib["zip_extract_file"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("zip_extract_file", args, 1, "3", "string", "[]any", "string"); !ok {
			return nil, err
		}

		zipFilename := args[0].(string)
		filesAny := args[1]
		destDir := args[2].(string)

		// Convert []any to []string if needed
		var filesToExtract []string
		switch v := filesAny.(type) {
		case []string:
			filesToExtract = v
		case []any:
			filesToExtract = make([]string, len(v))
			for i, item := range v {
				filesToExtract[i] = fmt.Sprint(item)
			}
		default:
			return false, fmt.Errorf("zip_extract_file error: files argument must be []string or []any, got %T", filesAny)
		}

		// Open the ZIP file
		reader, err := zip.OpenReader(zipFilename)
		if err != nil {
			return false, fmt.Errorf("zip_extract_file error opening file: %v", err)
		}
		defer reader.Close()

		// Create destination directory if it doesn't exist
		err = os.MkdirAll(destDir, 0755)
		if err != nil {
			return false, fmt.Errorf("zip_extract_file error creating directory: %v", err)
		}

		// Create set of files to extract for efficient lookup
		extractSet := make(map[string]bool)
		for _, file := range filesToExtract {
			extractSet[file] = true
		}

		// Extract each requested file
		for _, file := range reader.File {
			if extractSet[file.Name] {
				err = extractFileFromZip(file, destDir)
				if err != nil {
					return false, fmt.Errorf("zip_extract_file error extracting %s: %v", file.Name, err)
				}
			}
		}

		return true, nil
	}

}

// Helper function to add a file to a ZIP
func addFileToZip(zipWriter *zip.Writer, filePath, zipPath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Use provided zipPath or just the filename
	if zipPath == "" {
		zipPath = filepath.Base(filePath)
	}

	// Create ZIP entry
	zipEntry, err := zipWriter.Create(zipPath)
	if err != nil {
		return err
	}

	// Copy file contents
	_, err = io.Copy(zipEntry, file)
	return err
}

// Helper function to copy a file within a ZIP
func copyFileInZip(zipWriter *zip.Writer, zipFile *zip.File) error {
	// Create new entry
	entry, err := zipWriter.Create(zipFile.Name)
	if err != nil {
		return err
	}

	// Open the file in the ZIP
	fileReader, err := zipFile.Open()
	if err != nil {
		return err
	}
	defer fileReader.Close()

	// Copy contents
	_, err = io.Copy(entry, fileReader)
	return err
}

// Helper function to extract a file from a ZIP
func extractFileFromZip(zipFile *zip.File, destDir string) error {
	// Create the full path
	fullPath := filepath.Join(destDir, zipFile.Name)

	// Create directory if needed
	if zipFile.FileInfo().IsDir() {
		return os.MkdirAll(fullPath, zipFile.Mode())
	}

	// Create parent directories
	err := os.MkdirAll(filepath.Dir(fullPath), 0755)
	if err != nil {
		return err
	}

	// Create the file
	destFile, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, zipFile.Mode())
	if err != nil {
		return err
	}
	defer destFile.Close()

	// Open the file in the ZIP
	zipFileReader, err := zipFile.Open()
	if err != nil {
		return err
	}
	defer zipFileReader.Close()

	// Copy contents
	_, err = io.Copy(destFile, zipFileReader)
	return err
}
