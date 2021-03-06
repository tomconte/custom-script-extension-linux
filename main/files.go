package main

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/Azure/custom-script-extension-linux/pkg/blobutil"
	"github.com/Azure/custom-script-extension-linux/pkg/download"
	"github.com/Azure/custom-script-extension-linux/pkg/preprocess"
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
)

// downloadAndProcessURL downloads using the specified downloader and saves it to the
// specified existing directory, which must be the path to the saved file. Then
// it post-processes file based on heuristics.
func downloadAndProcessURL(ctx *log.Context, url, downloadDir, storageAccountName, storageAccountKey string) error {
	fn, err := urlToFileName(url)
	if err != nil {
		return err
	}

	dl, err := getDownloader(url, storageAccountName, storageAccountKey)
	if err != nil {
		return err
	}

	fp := filepath.Join(downloadDir, fn)
	const mode = 0500 // we assume users download scripts to execute
	if _, err := download.SaveTo(ctx, dl, fp, mode); err != nil {
		return err
	}

	err = postProcessFile(fp)
	return errors.Wrapf(err, "failed to post-process '%s'", fn)
}

// getDownloader returns a downloader for the given URL based on whether the
// storage credentials are empty or not.
func getDownloader(fileURL string, storageAccountName, storageAccountKey string) (
	download.Downloader, error) {
	if storageAccountName == "" || storageAccountKey == "" {
		return download.NewURLDownload(fileURL), nil
	}

	blob, err := blobutil.ParseBlobURL(fileURL)
	if err != nil {
		return nil, err
	}
	return download.NewBlobDownload(
		storageAccountName, storageAccountKey,
		blob), nil
}

// urlToFileName parses given URL and returns the section after the last slash
// character of the path segment to be used as a file name. If a value is not
// found, an error is returned.
func urlToFileName(fileURL string) (string, error) {
	u, err := url.Parse(fileURL)
	if err != nil {
		return "", errors.Wrapf(err, "unable to parse URL: %q", fileURL)
	}

	s := strings.Split(u.Path, "/")
	if len(s) > 0 {
		fn := s[len(s)-1]
		if fn != "" {
			return fn, nil
		}
	}
	return "", fmt.Errorf("cannot extract file name from URL: %q", fileURL)
}

// postProcessFile determines if path is a script file based on heuristics
// and makes in-place changes to the file with some post-processing such as BOM
// and DOS-line endings fixes to make the script POSIX-friendly.
func postProcessFile(path string) error {
	ok, err := preprocess.IsTextFile(path)
	if err != nil {
		return errors.Wrapf(err, "error determining if script file")
	}
	if !ok {
		return nil
	}

	b, err := ioutil.ReadFile(path) // read the file into memory for processing
	if err != nil {
		return errors.Wrapf(err, "error reading file")
	}
	b = preprocess.RemoveBOM(b)
	b = preprocess.Dos2Unix(b)
	err = ioutil.WriteFile(path, b, 0) // mode is ignored
	return errors.Wrapf(err, "failed to write to file")
}
