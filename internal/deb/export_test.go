package deb

func FakePlatformGoArch(goArch string) (restore func()) {
	saved := platformGoArch
	platformGoArch = goArch
	return func() { platformGoArch = saved }
}
