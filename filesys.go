package main

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"gioui.org/app"
	"github.com/reactivego/rx"
)

func DataDir(organization, application string) (string, error) {
	dir, err := app.DataDir()
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, organization, application)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return "", err
	}
	return dir, nil
}

func Directory(name string) rx.Observable[fs.DirEntry] {
	return rx.Defer(func() rx.Observable[fs.DirEntry] {
		entries, err := os.ReadDir(name)
		return rx.Create(func(index int) (fs.DirEntry, error, bool) {
			if index < len(entries) {
				return entries[index], nil, false
			} else {
				var zero os.DirEntry
				return zero, err, true
			}
		})
	})
}

func File(dir, name string) rx.Observable[[]byte] {
	return rx.Create(func(index int) ([]byte, error, bool) {
		if index == 0 {
			next, err := os.ReadFile(filepath.Join(dir, name))
			return next, err, err != nil
		}
		return nil, nil, true
	})
}

func FileChunks(dir, name string, bufsize int) rx.Observable[[]byte] {
	return rx.Defer(func() rx.Observable[[]byte] {
		file, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			return rx.Throw[[]byte](err)
		}
		buf := make([]byte, bufsize)
		return rx.Create(func(index int) ([]byte, error, bool) {
			n, err := file.Read(buf)
			if err != nil && err != io.EOF {
				return nil, err, true
			}
			if n == 0 || err == io.EOF {
				return nil, nil, true
			}
			return buf[:n:n], nil, false
		}).OnDone(func(err error) {
			file.Close()
		})
	})
}
