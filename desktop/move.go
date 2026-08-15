package main

import (
	"errors"
	"io"
	"os"
)

// moveFile renames src to dst, falling back to a copy and delete when
// rename fails, for example across filesystems. On a failed copy the
// source is left in place.
func moveFile(src, dst string) error {
	if src == "" || dst == "" {
		return errors.New("move paths missing")
	}
	if renameErr := os.Rename(src, dst); renameErr == nil {
		return nil
	}
	if copyErr := copyFile(src, dst); copyErr != nil {
		return copyErr
	}
	return os.Remove(src)
}

// copyFile writes an exact copy of src at dst, removing a partial dst
// on failure.
func copyFile(src, dst string) error {
	// #nosec G304 -- both paths come from the user's own configuration
	in, openErr := os.Open(src)
	if openErr != nil {
		return openErr
	}
	defer func() {
		// The file was only read; a close error loses nothing.
		_ = in.Close()
	}()
	// #nosec G304 -- both paths come from the user's own configuration
	out, createErr := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if createErr != nil {
		return createErr
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		// Best effort; the copy error is what matters.
		_ = os.Remove(dst)
		return copyErr
	}
	return nil
}
