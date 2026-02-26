package main

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modNetapi32          = windows.NewLazySystemDLL("Netapi32.dll")
	procNetShareEnum     = modNetapi32.NewProc("NetShareEnum")
	procNetApiBufferFree = modNetapi32.NewProc("NetApiBufferFree")
)

const (
	MAX_PREFERRED_LENGTH = 0xFFFFFFFF
	NERR_Success         = 0
)

// SHARE_INFO_1 structure
type SHARE_INFO_1 struct {
	NetName *uint16
	Type    uint32
	Remark  *uint16
}

func netView(server string) ([]string, error) {
	var serverPtr *uint16
	var err error

	if server != "" {
		serverPtr, err = windows.UTF16PtrFromString(server)
		if err != nil {
			return nil, err
		}
	}

	var bufPtr *byte
	var entriesRead uint32
	var totalEntries uint32

	ret, _, _ := procNetShareEnum.Call(
		uintptr(unsafe.Pointer(serverPtr)), // servername (\\username)
		uintptr(1),                         // level 1
		uintptr(unsafe.Pointer(&bufPtr)),   // buffer
		uintptr(MAX_PREFERRED_LENGTH),
		uintptr(unsafe.Pointer(&entriesRead)),
		uintptr(unsafe.Pointer(&totalEntries)),
		uintptr(0), // resume handle
	)

	if ret != NERR_Success {
		return nil, fmt.Errorf("NetShareEnum failed: %d", ret)
	}

	defer procNetApiBufferFree.Call(uintptr(unsafe.Pointer(bufPtr)))

	shares := unsafe.Slice((*SHARE_INFO_1)(unsafe.Pointer(bufPtr)), entriesRead)

	var result []string
	for _, share := range shares {
		name := windows.UTF16PtrToString(share.NetName)
		if share.Type == 0 { // TODO: handle hidden shares
			result = append(result, name)
		}
	}

	return result, nil
}
