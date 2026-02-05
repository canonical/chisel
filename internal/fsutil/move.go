package fsutil

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type MoveOptions struct {
	SrcRoot string
	DstRoot string
	// Path is relative to SrcRoot.
	Path string
	Mode fs.FileMode
	// If MakeParents is true, missing parent directories of Path are
	// created with permissions 0755 in DstRoot.
	MakeParents bool
}

// Move moves or creates a filesystem entry according to the provided options.
//
// Move can return errors from the os package.
func Move(options *MoveOptions) error {
	o, err := getValidMoveOptions(options)
	if err != nil {
		return err
	}

	srcPath, err := absPath(options.SrcRoot, o.Path)
	if err != nil {
		return err
	}
	dstPath, err := absPath(options.DstRoot, o.Path)
	if err != nil {
		return err
	}

	if o.MakeParents {
		if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
			return err
		}
	}

	switch o.Mode & fs.ModeType {
	case 0, fs.ModeSymlink:
		err = os.Rename(srcPath, dstPath)
	case fs.ModeDir:
		err = createDir(&CreateOptions{
			Root:         o.DstRoot,
			Path:         o.Path,
			Mode:         o.Mode,
			OverrideMode: true,
		})
	default:
		err = fmt.Errorf("unsupported file type: %s", o.Path)
	}
	if err != nil {
		return err
	}

	return nil
}

func getValidMoveOptions(options *MoveOptions) (*MoveOptions, error) {
	optsCopy := *options
	o := &optsCopy
	if o.SrcRoot == "" {
		return nil, fmt.Errorf("internal error: MoveOptions.SrcRoot is unset")
	}
	if o.DstRoot == "" {
		return nil, fmt.Errorf("internal error: MoveOptions.DstRoot is unset")
	}
	if o.SrcRoot != "/" {
		o.SrcRoot = filepath.Clean(o.SrcRoot) + "/"
	}
	if o.DstRoot != "/" {
		o.DstRoot = filepath.Clean(o.DstRoot) + "/"
	}
	return o, nil
}
