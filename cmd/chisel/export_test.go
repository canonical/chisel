package main

import "github.com/canonical/chisel/internal/archive"

var RunMain = run

func FakeIsStdoutTTY(t bool) (restore func()) {
	oldIsStdoutTTY := isStdoutTTY
	isStdoutTTY = t
	return func() {
		isStdoutTTY = oldIsStdoutTTY
	}
}

func FakeIsStdinTTY(t bool) (restore func()) {
	oldIsStdinTTY := isStdinTTY
	isStdinTTY = t
	return func() {
		isStdinTTY = oldIsStdinTTY
	}
}

var FindSlices = findSlices

func FakeArchiveOpen(f func(_ *archive.Options) (archive.Archive, error)) (restore func()) {
	oldArchiveOpen := archiveOpen
	archiveOpen = f
	return func() {
		archiveOpen = oldArchiveOpen
	}
}
