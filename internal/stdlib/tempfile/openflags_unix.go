//go:build aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris

package tempfile

import "golang.org/x/sys/unix"

const noFollowFlag = unix.O_NOFOLLOW
