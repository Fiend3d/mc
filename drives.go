package main

import (
	"fmt"

	"golang.org/x/sys/windows"
)

type drive struct {
	letter    string
	name      string // Volume label
	driveType string // Fixed, Removable, etc.
	total     uint64 // Total bytes
	free      uint64 // Free bytes available to caller
	available uint64 // Total free bytes
}

func getDrives() ([]drive, error) {
	bitmask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil, err
	}

	var drives []drive

	for i := range 26 {
		if bitmask&(1<<i) != 0 {
			letter := fmt.Sprintf("%c:", 'A'+i)
			drive, err := getDriveInfo(letter)
			if err == nil {
				drives = append(drives, drive)
			}
		}
	}

	return drives, nil
}

func getDriveInfo(letter string) (drive, error) {
	path := letter + "\\"

	// Convert string safely
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return drive{}, err
	}

	// Get drive type
	var driveType string
	switch windows.GetDriveType(pathPtr) {
	case windows.DRIVE_FIXED:
		driveType = "Fixed"
	case windows.DRIVE_REMOVABLE:
		driveType = "Removable"
	case windows.DRIVE_REMOTE:
		driveType = "Network"
	case windows.DRIVE_CDROM:
		driveType = "CD-ROM"
	case windows.DRIVE_RAMDISK:
		driveType = "RAM Disk"
	default:
		driveType = "Unknown"
	}

	// Get volume name
	var volumeName [windows.MAX_PATH + 1]uint16
	var fileSystemName [16]uint16
	var serialNumber uint32
	var maxComponentLen uint32
	var fileSystemFlags uint32

	err = windows.GetVolumeInformation(
		pathPtr,
		&volumeName[0],
		windows.MAX_PATH+1,
		&serialNumber,
		&maxComponentLen,
		&fileSystemFlags,
		&fileSystemName[0],
		16,
	)
	if err != nil {
		return drive{}, err
	}

	volumeLabel := windows.UTF16ToString(volumeName[:])

	// Get disk space
	var freeBytesAvailable, totalBytes, totalFreeBytes uint64

	err = windows.GetDiskFreeSpaceEx(
		pathPtr,
		&freeBytesAvailable,
		&totalBytes,
		&totalFreeBytes,
	)
	if err != nil {
		return drive{}, err
	}

	return drive{
		letter:    letter,
		name:      volumeLabel,
		driveType: driveType,
		total:     totalBytes,
		free:      freeBytesAvailable,
		available: totalFreeBytes,
	}, nil
}
