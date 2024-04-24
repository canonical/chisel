package main

import "github.com/canonical/chisel/internal/archive"

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

func FakeOpenArchive(f func(opts *archive.Options) (archive.Archive, error)) (restore func()) {
	oldOpenArchive := openArchive
	openArchive = f
	return func() {
		openArchive = oldOpenArchive
	}
}
