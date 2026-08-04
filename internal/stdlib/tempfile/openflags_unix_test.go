//go:build aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris

package tempfile

import (
	"testing"

	"golang.org/x/sys/unix"
)

func TestTemporaryFileOpenFlagsDoNotFollowLinks(t *testing.T) {
	if temporaryFileOpenFlags&unix.O_NOFOLLOW == 0 {
		t.Fatalf("temporary file flags %#x do not include O_NOFOLLOW", temporaryFileOpenFlags)
	}
}
