package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type RemoveOptions struct {
	Root string
	// Path is relative to Root.
	Path string
}

func Remove(options *RemoveOptions) error {
	options, err := getValidRemoveOptions(options)
	if err != nil {
		return err
	}
	path, err := absPath(options.Root, options.Path)
	if err != nil {
		return err
	}
	if strings.HasSuffix(path, "/") {
		err = syscall.Rmdir(path)
		if err != nil && err != syscall.ENOTEMPTY {
			return err
		}
	} else {
		err = os.Remove(path)
		if err != nil {
			return err
		}
	}
	return nil
}

func getValidRemoveOptions(options *RemoveOptions) (*RemoveOptions, error) {
	optsCopy := *options
	o := &optsCopy
	if o.Root == "" {
		return nil, fmt.Errorf("internal error: RemoveOptions.Root is unset")
	}
	if o.Root != "/" {
		o.Root = filepath.Clean(o.Root) + "/"
	}
	return o, nil
}
