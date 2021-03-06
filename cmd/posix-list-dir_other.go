// +build plan9 solaris

package cmd

import (
	"context"
	"io"
	"os"
	"path"
	"strings"

	"github.com/minio/radio/cmd/logger"
)

// Return all the entries at the directory dirPath.
func readDir(dirPath string) (entries []string, err error) {
	return readDirN(dirPath, -1)
}

// Return N entries at the directory dirPath. If count is -1, return all entries
func readDirN(dirPath string, count int) (entries []string, err error) {
	d, err := os.Open(dirPath)
	if err != nil {
		// File is really not found.
		if os.IsNotExist(err) {
			return nil, errFileNotFound
		}

		// File path cannot be verified since one of the parents is a file.
		if strings.Contains(err.Error(), "not a directory") {
			return nil, errFileNotFound
		}
		return nil, err
	}
	defer d.Close()

	maxEntries := 1000
	if count > 0 && count < maxEntries {
		maxEntries = count
	}

	done := false
	remaining := count

	for !done {
		// Read up to max number of entries.
		fis, err := d.Readdir(maxEntries)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if count > 0 {
			if remaining <= len(fis) {
				fis = fis[:remaining]
				done = true
			}
		}
		for _, fi := range fis {
			// Stat symbolic link and follow to get the final value.
			if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
				var st os.FileInfo
				st, err = os.Stat(path.Join(dirPath, fi.Name()))
				if err != nil {
					reqInfo := (&logger.ReqInfo{}).AppendTags("path", path.Join(dirPath, fi.Name()))
					ctx := logger.SetReqInfo(context.Background(), reqInfo)
					logger.LogIf(ctx, err)
					continue
				}
				// Append to entries if symbolic link exists and is valid.
				if st.IsDir() {
					entries = append(entries, fi.Name()+SlashSeparator)
				} else if st.Mode().IsRegular() {
					entries = append(entries, fi.Name())
				}
				if count > 0 {
					remaining--
				}
				continue
			}
			if fi.Mode().IsDir() {
				// Append SlashSeparator instead of "\" so that sorting is achieved as expected.
				entries = append(entries, fi.Name()+SlashSeparator)
			} else if fi.Mode().IsRegular() {
				entries = append(entries, fi.Name())
			}
			if count > 0 {
				remaining--
			}
		}
	}
	return entries, nil
}
