package pathutil

import "testing"

func TestRelVolume(t *testing.T) {
	_, _ = Rel(`C:\repo`, `D:\other\file.go`)
	_, _ = Rel(`C:\repo`, `\\server\share\file.go`)
}
