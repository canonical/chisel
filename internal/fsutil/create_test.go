package fsutil_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/fsutil"
	"github.com/canonical/chisel/internal/testutil"
)

type createTest struct {
	summary string
	options fsutil.CreateOptions
	hackopt func(c *C, dir string, opts *fsutil.CreateOptions)
	result  map[string]string
	error   string
}

var createTests = []createTest{{
	summary: "Create a file and its parent directory",
	options: fsutil.CreateOptions{
		Path:        "foo/bar",
		Data:        bytes.NewBufferString("data1"),
		Mode:        0444,
		MakeParents: true,
	},
	result: map[string]string{
		"/foo/":    "dir 0755",
		"/foo/bar": "file 0444 5b41362b",
	},
}, {
	summary: "Create a symlink",
	options: fsutil.CreateOptions{
		Path:        "foo/bar",
		Link:        "../baz",
		Mode:        fs.ModeSymlink,
		MakeParents: true,
	},
	result: map[string]string{
		"/foo/":    "dir 0755",
		"/foo/bar": "symlink ../baz",
	},
}, {
	summary: "Create a directory",
	options: fsutil.CreateOptions{
		Path:        "foo/bar",
		Mode:        fs.ModeDir | 0444,
		MakeParents: true,
	},
	result: map[string]string{
		"/foo/":     "dir 0755",
		"/foo/bar/": "dir 0444",
	},
}, {
	summary: "Create a directory with sticky bit",
	options: fsutil.CreateOptions{
		Path: "tmp",
		Mode: fs.ModeDir | fs.ModeSticky | 0775,
	},
	result: map[string]string{
		"/tmp/": "dir 01775",
	},
}, {
	summary: "Cannot create a parent directory without MakeParents set",
	options: fsutil.CreateOptions{
		Path: "foo/bar",
		Mode: fs.ModeDir | 0775,
	},
	error: `mkdir /[^ ]*/foo/bar: no such file or directory`,
}, {
	summary: "Re-creating an existing directory keeps the original mode",
	options: fsutil.CreateOptions{
		Path: "foo",
		Mode: fs.ModeDir | 0775,
	},
	hackopt: func(c *C, dir string, opts *fsutil.CreateOptions) {
		c.Assert(os.Mkdir(filepath.Join(dir, "foo/"), fs.ModeDir|0765), IsNil)
	},
	result: map[string]string{
		// mode is not updated.
		"/foo/": "dir 0765",
	},
}, {
	summary: "Re-creating an existing file keeps the original mode",
	options: fsutil.CreateOptions{
		Path: "foo",
		// Mode should be ignored for existing entry.
		Mode: 0644,
		Data: bytes.NewBufferString("changed"),
	},
	hackopt: func(c *C, dir string, opts *fsutil.CreateOptions) {
		c.Assert(os.WriteFile(filepath.Join(dir, "foo"), []byte("data"), 0666), IsNil)
	},
	result: map[string]string{
		// mode is not updated.
		"/foo": "file 0666 d67e2e94",
	},
}, {
	summary: "Create a hard link",
	options: fsutil.CreateOptions{
		Path:        "hardlink",
		Link:        "file",
		Mode:        0644,
		MakeParents: true,
	},
	hackopt: func(c *C, dir string, opts *fsutil.CreateOptions) {
		c.Assert(os.WriteFile(filepath.Join(dir, "file"), []byte("data"), 0644), IsNil)
		// An absolute path is required to create a hard link.
		opts.Link = filepath.Join(dir, opts.Link)
	},
	result: map[string]string{
		"/file":     "file 0644 3a6eb079 <1>",
		"/hardlink": "file 0644 3a6eb079 <1>",
	},
}, {
	summary: "Cannot create a hard link if the link target does not exist",
	options: fsutil.CreateOptions{
		Path:        "hardlink",
		Link:        "missing-file",
		Mode:        0644,
		MakeParents: true,
	},
	hackopt: func(c *C, dir string, opts *fsutil.CreateOptions) {
		opts.Link = filepath.Join(dir, opts.Link)
	},
	error: `link /[^ ]*/missing-file /[^ ]*/hardlink: no such file or directory`,
}, {
	summary: "No error if hard link already exists",
	options: fsutil.CreateOptions{
		Path:        "hardlink",
		Link:        "file",
		Mode:        0644,
		MakeParents: true,
	},
	hackopt: func(c *C, dir string, opts *fsutil.CreateOptions) {
		c.Assert(os.WriteFile(filepath.Join(dir, "file"), []byte("data"), 0644), IsNil)
		c.Assert(os.Link(filepath.Join(dir, "file"), filepath.Join(dir, "hardlink")), IsNil)
		opts.Link = filepath.Join(dir, opts.Link)
	},
	result: map[string]string{
		"/file":     "file 0644 3a6eb079 <1>",
		"/hardlink": "file 0644 3a6eb079 <1>",
	},
}, {
	summary: "Cannot create a hard link if file exists but differs",
	options: fsutil.CreateOptions{
		Path:        "hardlink",
		Link:        "file",
		Mode:        0644,
		MakeParents: true,
	},
	hackopt: func(c *C, dir string, opts *fsutil.CreateOptions) {
		c.Assert(os.WriteFile(filepath.Join(dir, "file"), []byte("data"), 0644), IsNil)
		c.Assert(os.WriteFile(filepath.Join(dir, "hardlink"), []byte("data"), 0644), IsNil)
		opts.Link = filepath.Join(dir, opts.Link)
	},
	error: `link /[^ ]*/file /[^ ]*/hardlink: file exists`,
}, {
	summary: "Hard link to symlink",
	options: fsutil.CreateOptions{
		Path:        "hardlink",
		Link:        "bar",
		Mode:        0644,
		MakeParents: true,
	},
	hackopt: func(c *C, dir string, opts *fsutil.CreateOptions) {
		err := os.Symlink("foo", filepath.Join(dir, "bar"))
		c.Assert(err, IsNil)
		opts.Link = filepath.Join(dir, opts.Link)
	},
	result: map[string]string{
		"/bar":      "symlink foo <1>",
		"/hardlink": "symlink foo <1>",
	},
}, {
	summary: "The mode of a dir can be overridden",
	options: fsutil.CreateOptions{
		Path:         "foo",
		Mode:         fs.ModeDir | 0775,
		OverrideMode: true,
	},
	hackopt: func(c *C, dir string, opts *fsutil.CreateOptions) {
		c.Assert(os.Mkdir(filepath.Join(dir, "foo/"), fs.ModeDir|0765), IsNil)
	},
	result: map[string]string{
		// mode is updated.
		"/foo/": "dir 0775",
	},
}, {
	summary: "The mode of a file can be overridden",
	options: fsutil.CreateOptions{
		Path:         "foo",
		Mode:         0775,
		Data:         bytes.NewBufferString("whatever"),
		OverrideMode: true,
	},
	hackopt: func(c *C, dir string, opts *fsutil.CreateOptions) {
		err := os.WriteFile(filepath.Join(dir, "foo"), []byte("data"), 0666)
		c.Assert(err, IsNil)
	},
	result: map[string]string{
		// mode is updated.
		"/foo": "file 0775 85738f8f",
	},
}, {
	summary: "The mode of a symlink cannot be overridden",
	options: fsutil.CreateOptions{
		Path: "foo",
		Link: "./bar",
		Mode: 0666 | fs.ModeSymlink,
	},
	hackopt: func(c *C, dir string, opts *fsutil.CreateOptions) {
		err := os.WriteFile(filepath.Join(dir, "foo"), []byte("data"), 0666)
		c.Assert(err, IsNil)
	},
	result: map[string]string{
		"/foo": "symlink ./bar",
	},
}, {
	summary: "OverrideMode does not follow symlink",
	options: fsutil.CreateOptions{
		Path:         "foo",
		Link:         "./bar",
		Mode:         0776 | fs.ModeSymlink,
		OverrideMode: true,
	},
	hackopt: func(c *C, dir string, opts *fsutil.CreateOptions) {
		err := os.WriteFile(filepath.Join(dir, "bar"), []byte("data"), 0666)
		c.Assert(err, IsNil)
		err = os.WriteFile(filepath.Join(dir, "foo"), []byte("data"), 0666)
		c.Assert(err, IsNil)
	},
	result: map[string]string{
		"/foo": "symlink ./bar",
		// mode is not updated.
		"/bar": "file 0666 3a6eb079",
	},
}, {
	summary: "The target of an existing symlink can be overridden",
	options: fsutil.CreateOptions{
		Path: "bar",
		// Existing link with different target.
		Link: "other",
		Mode: 0666 | fs.ModeSymlink,
	},
	hackopt: func(c *C, dir string, opts *fsutil.CreateOptions) {
		err := os.Symlink("foo", filepath.Join(dir, "bar"))
		c.Assert(err, IsNil)
	},
	result: map[string]string{
		"/bar": "symlink other",
	},
}, {
	summary: "No error if symlink already exists",
	options: fsutil.CreateOptions{
		Path: "bar",
		// Existing link with same target.
		Link: "foo",
		Mode: 0666 | fs.ModeSymlink,
	},
	hackopt: func(c *C, dir string, opts *fsutil.CreateOptions) {
		err := os.Symlink("foo", filepath.Join(dir, "bar"))
		c.Assert(err, IsNil)
	},
	result: map[string]string{
		"/bar": "symlink foo",
	},
}, {
	summary: "Cannot create file outside of Root",
	options: fsutil.CreateOptions{
		Root: "/root",
		Path: "../file",
		Mode: 0666,
		Data: bytes.NewBufferString("hijacking system file"),
	},
	error: `cannot create path /file outside of root /root/`,
}, {
	summary: "Hardlink cannot escape Root",
	options: fsutil.CreateOptions{
		Root: "/root",
		Path: "/system-file",
		Link: "/etc/group",
		Mode: 0666,
	},
	error: `invalid hardlink /root/system-file target: /etc/group is outside of root /root/`,
}, {
	summary: "Root is correctly used as prefix",
	options: fsutil.CreateOptions{
		Root: "/foo",
		Path: "/system-file",
		Link: "/foobar",
		Mode: 0666,
	},
	error: `invalid hardlink /foo/system-file target: /foobar is outside of root /foo/`,
}}

func (s *S) TestCreate(c *C) {
	oldUmask := syscall.Umask(0)
	defer func() {
		syscall.Umask(oldUmask)
	}()

	for _, test := range createTests {
		c.Logf("Test: %s", test.summary)
		if test.result == nil {
			// Empty map for no files created.
			test.result = make(map[string]string)
		}
		c.Logf("Options: %v", test.options)
		dir := c.MkDir()
		options := test.options
		if options.Root == "" {
			options.Root = dir
		}
		if test.hackopt != nil {
			test.hackopt(c, dir, &options)
		}
		entry, err := fsutil.Create(&options)

		if test.error != "" {
			c.Assert(err, ErrorMatches, test.error)
			continue
		}

		c.Assert(err, IsNil)
		c.Assert(testutil.TreeDump(dir), DeepEquals, test.result)

		if options.Mode.IsRegular() && options.Link != "" {
			// We should test hard link entries differently because
			// fsutil.Create does not return hash or size when it creates hard
			// links.
			c.Assert(entry.Link, Equals, options.Link)
			c.Assert(entry.Mode.IsRegular(), Equals, true)
			pathInfo, err := os.Lstat(entry.Path)
			c.Assert(err, IsNil)
			linkInfo, err := os.Lstat(entry.Link)
			c.Assert(err, IsNil)
			os.SameFile(pathInfo, linkInfo)
		} else {
			// [fsutil.Create] does not return information about parent directories
			// created implicitly. We only check for the requested path.
			entry.Path = strings.TrimPrefix(entry.Path, dir)
			// Add the slashes that TreeDump adds to the path.
			slashPath := "/" + test.options.Path
			if test.options.Mode.IsDir() {
				slashPath = slashPath + "/"
			}
			c.Assert(testutil.TreeDumpEntry(entry), DeepEquals, test.result[slashPath])
		}
	}
}

type createWriterTest struct {
	options fsutil.CreateOptions
	data    []byte
	hackdir func(c *C, dir string)
	result  map[string]string
	error   string
}

var createWriterTests = []createWriterTest{{
	options: fsutil.CreateOptions{
		Path: "foo",
		Mode: 0644,
	},
	data: []byte("foo"),
	result: map[string]string{
		"/foo": "file 0644 2c26b46b",
	},
}, {
	options: fsutil.CreateOptions{
		Path: "foo",
		Mode: 0644 | fs.ModeDir,
	},
	error: `unsupported file type: \/[a-z0-9\-\/]*foo`,
}, {
	options: fsutil.CreateOptions{
		Path: "foo",
		Mode: 0644 | fs.ModeSymlink,
	},
	error: `unsupported file type: /[a-z0-9\-\/]*/foo`,
}, {
	options: fsutil.CreateOptions{
		Path:        "foo/bar",
		Mode:        0644,
		MakeParents: true,
	},
	data: []byte("foo"),
	result: map[string]string{
		"/foo/":    "dir 0755",
		"/foo/bar": "file 0644 2c26b46b",
	},
}, {
	options: fsutil.CreateOptions{
		Path:        "foo/bar",
		Mode:        0644,
		MakeParents: false,
	},
	error: `open /[a-z0-9\-\/]*/foo/bar: no such file or directory`,
}, {
	options: fsutil.CreateOptions{
		Root:        "/root",
		Path:        "../file",
		Mode:        0644,
		MakeParents: true,
	},
	error: `cannot create path /file outside of root /root/`,
}}

func (s *S) TestCreateWriter(c *C) {
	oldUmask := syscall.Umask(0)
	defer func() {
		syscall.Umask(oldUmask)
	}()

	for _, test := range createWriterTests {
		if test.result == nil {
			// Empty map for no files created.
			test.result = make(map[string]string)
		}
		c.Logf("Options: %v", test.options)
		dir := c.MkDir()
		if test.hackdir != nil {
			test.hackdir(c, dir)
		}
		options := test.options
		if test.options.Root == "" {
			options.Root = dir
		}
		writer, entry, err := fsutil.CreateWriter(&options)
		if test.error != "" {
			c.Assert(err, ErrorMatches, test.error)
			continue
		}
		c.Assert(err, IsNil)

		// Hash and Size are only set when the writer is closed.
		_, err = writer.Write(test.data)
		c.Assert(err, IsNil)
		c.Assert(entry.Path, Equals, filepath.Clean(filepath.Join(options.Root, options.Path)))
		c.Assert(entry.Mode, Equals, options.Mode)
		c.Assert(entry.SHA256, Equals, "")
		c.Assert(entry.Size, Equals, 0)
		err = writer.Close()
		c.Assert(err, IsNil)

		c.Assert(testutil.TreeDump(dir), DeepEquals, test.result)
		// [fsutil.CreateWriter] does not return information about parent
		// directories created implicitly. We only check for the requested path.
		entry.Path = strings.TrimPrefix(entry.Path, dir)
		// Add the slashes that TreeDump adds to the path.
		slashPath := "/" + test.options.Path
		if test.options.Mode.IsDir() {
			slashPath = slashPath + "/"
		}
		c.Assert(testutil.TreeDumpEntry(entry), DeepEquals, test.result[slashPath])
	}
}

func (s *S) TestCreateEmptyRoot(c *C) {
	options := &fsutil.CreateOptions{Root: ""}
	_, err := fsutil.Create(options)
	c.Assert(err, ErrorMatches, "internal error: CreateOptions.Root is unset")
	_, _, err = fsutil.CreateWriter(options)
	c.Assert(err, ErrorMatches, "internal error: CreateOptions.Root is unset")
}

func entryOwnership(path string) (uid, gid int) {
	info, err := os.Lstat(path)
	if err != nil {
		panic(err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		panic("unsupported system: not a syscall.Stat_t")
	}
	return int(stat.Uid), int(stat.Gid)
}

func (s *S) TestCreateOwnership(c *C) {
	// Ownership can always be set to the current process user and group,
	// even when running unprivileged. The unprivileged attempt to set
	// other ids fails with EPERM, which is logged and otherwise ignored.
	selfUid, selfGid := os.Getuid(), os.Getgid()
	dir := c.MkDir()

	uid, gid := selfUid, selfGid
	_, err := fsutil.Create(&fsutil.CreateOptions{
		Root: dir,
		Path: "file",
		Mode: 0644,
		Data: bytes.NewBufferString("data1"),
		Uid:  &uid,
		Gid:  &gid,
	})
	c.Assert(err, IsNil)
	gotUid, gotGid := entryOwnership(filepath.Join(dir, "file"))
	c.Assert(gotUid, Equals, selfUid)
	c.Assert(gotGid, Equals, selfGid)

	// Directories are owned too.
	_, err = fsutil.Create(&fsutil.CreateOptions{
		Root:        dir,
		Path:        "somedir",
		Mode:        fs.ModeDir | 0755,
		MakeParents: true,
		Uid:         &uid,
		Gid:         &gid,
	})
	c.Assert(err, IsNil)
	gotUid, gotGid = entryOwnership(filepath.Join(dir, "somedir"))
	c.Assert(gotUid, Equals, selfUid)
	c.Assert(gotGid, Equals, selfGid)

	// Ownership of pre-existing entries is only applied with
	// OverrideMode, mirroring the mode semantics.
	hack := func(root string) {
		err := os.WriteFile(filepath.Join(root, "file"), []byte("data"), 0644)
		c.Assert(err, IsNil)
	}
	root := c.MkDir()
	hack(root)
	rootUid, rootGid := 0, 0
	if selfUid == 0 {
		// Use an id different from the current ones for a meaningful
		// assertion when running as root.
		rootUid, rootGid = 0, 0
	}
	_, err = fsutil.Create(&fsutil.CreateOptions{
		Root: root,
		Path: "file",
		Mode: 0644,
		Data: bytes.NewBufferString("data"),
		Uid:  &rootUid,
		Gid:  &rootGid,
	})
	c.Assert(err, IsNil)
	gotUid, gotGid = entryOwnership(filepath.Join(root, "file"))
	if selfUid == 0 {
		c.Assert(gotUid, Equals, rootUid)
		c.Assert(gotGid, Equals, rootGid)
	} else {
		// Unchanged: the entry pre-existed and OverrideMode is unset.
		c.Assert(gotUid, Equals, selfUid)
		c.Assert(gotGid, Equals, selfGid)
	}

	root = c.MkDir()
	hack(root)
	_, err = fsutil.Create(&fsutil.CreateOptions{
		Root:         root,
		Path:         "file",
		Mode:         0644,
		Data:         bytes.NewBufferString("data"),
		OverrideMode: true,
		Uid:          &rootUid,
		Gid:          &rootGid,
	})
	c.Assert(err, IsNil)
	gotUid, gotGid = entryOwnership(filepath.Join(root, "file"))
	if selfUid == 0 {
		c.Assert(gotUid, Equals, rootUid)
		c.Assert(gotGid, Equals, rootGid)
	} else {
		// EPERM is logged once and otherwise ignored.
		c.Assert(gotUid, Equals, selfUid)
		c.Assert(gotGid, Equals, selfGid)
		c.Assert(c.GetTestLog(), Matches, "(?s).*WARNING: cannot preserve file ownership.*")
	}

	// Ownership of symlinks is never applied, not even via their target.
	root = c.MkDir()
	err = os.WriteFile(filepath.Join(root, "bar"), []byte("data"), 0644)
	c.Assert(err, IsNil)
	_, err = fsutil.Create(&fsutil.CreateOptions{
		Root: root,
		Path: "foo",
		Link: "bar",
		Mode: fs.ModeSymlink,
		Uid:  &rootUid,
		Gid:  &rootGid,
	})
	c.Assert(err, IsNil)
	gotUid, gotGid = entryOwnership(filepath.Join(root, "bar"))
	c.Assert(gotUid, Equals, selfUid)
	c.Assert(gotGid, Equals, selfGid)

	// CreateWriter applies ownership to newly created files too.
	root = c.MkDir()
	writer, _, err := fsutil.CreateWriter(&fsutil.CreateOptions{
		Root: root,
		Path: "file",
		Mode: 0644,
		Uid:  &uid,
		Gid:  &gid,
	})
	c.Assert(err, IsNil)
	_, err = writer.Write([]byte("data"))
	c.Assert(err, IsNil)
	c.Assert(writer.Close(), IsNil)
	gotUid, gotGid = entryOwnership(filepath.Join(root, "file"))
	c.Assert(gotUid, Equals, selfUid)
	c.Assert(gotGid, Equals, selfGid)
}

func (s *S) TestCreateOwnershipModeRestore(c *C) {
	// Chown strips the setuid and setgid mode bits of entries whose
	// ownership it changes, and open does not create files with those
	// bits in the first place, so applying ownership re-applies the
	// requested mode to preserve them.
	selfUid, selfGid := os.Getuid(), os.Getgid()
	uid, gid := selfUid, selfGid
	if os.Geteuid() == 0 {
		// Use a group different from the current one so that the
		// ownership change is an actual change.
		gid = 42
	}
	dir := c.MkDir()
	mode := fs.FileMode(0755) | fs.ModeSetgid
	_, err := fsutil.Create(&fsutil.CreateOptions{
		Root: dir,
		Path: "file",
		Mode: mode,
		Data: bytes.NewBufferString("data"),
		Uid:  &uid,
		Gid:  &gid,
	})
	c.Assert(err, IsNil)
	info, err := os.Lstat(filepath.Join(dir, "file"))
	c.Assert(err, IsNil)
	c.Assert(info.Mode(), Equals, mode)
	gotUid, gotGid := entryOwnership(filepath.Join(dir, "file"))
	c.Assert(gotUid, Equals, uid)
	c.Assert(gotGid, Equals, gid)
}
