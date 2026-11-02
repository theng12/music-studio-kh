//go:build windows

package library

import "golang.org/x/sys/windows"

// volumeSpace reports capacity and free bytes of the volume holding path with
// one call — no directory walk.
func volumeSpace(path string) (total, free uint64, err error) {
	pointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, err
	}
	var available, totalBytes, freeBytes uint64
	if err = windows.GetDiskFreeSpaceEx(pointer, &available, &totalBytes, &freeBytes); err != nil {
		return 0, 0, err
	}
	return totalBytes, available, nil
}
