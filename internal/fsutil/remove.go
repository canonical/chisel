package fsutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type RemoveOptions struct {
	Root string
	// Path is relative to Root.
	Path string
}

// Remove removes a filesystem entry according to the provided options.
// Non-empty directories are not removed.
//
// Remove can return errors from the os and syscall packages.
func Remove(options *RemoveOptions) error {
	options, err := getValidRemoveOptions(options)
	if err != nil {
		return err
	}
	path, err := absPath(options.Root, options.Path)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if err != nil && !os.IsNotExist(err) && !errors.Is(err, syscall.ENOTEMPTY) {
		return err
	}
	return nil
}

func getValidRemoveOptions(options *RemoveOptions) (*RemoveOptions, error) {
	optsCopy := *options
	o := &optsCopy
	if o.Root == "" {
		return nil, fmt.Errorf("internal error: RemoveOptions.Root is unset")
	}
	if o.Path == "" {
		return nil, fmt.Errorf("internal error: RemoveOptions.Path is unset")
	}
	if o.Root != "/" {
		o.Root = filepath.Clean(o.Root) + "/"
	}
	return o, nil
}
