package main

import (
	"fmt"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/gonutz/w32"
	"golang.design/x/clipboard"
)

var mutex sync.Mutex

func clipboardWrite(text string) error {
	err := clipboard.Init()
	if err != nil {
		return err
	}

	clipboard.Write(clipboard.FmtText, []byte(text))
	return nil
}

const (
	cfHDrop        = 15 // CF_HDROP
	dropEffectCopy = 1  // DROPEFFECT_COPY
	dropEffectMove = 2  // DROPEFFECT_MOVE
	gmemMoveable   = 0x0002
	gmemZeroInit   = 0x0040
	ghnd           = gmemMoveable | gmemZeroInit
)

// OpType represents clipboard operation type
type OpType int

const (
	OpCopy OpType = iota
	OpCut
)

var (
	user32                      = syscall.NewLazyDLL("user32.dll")
	shell32                     = syscall.NewLazyDLL("shell32.dll")
	procRegisterClipboardFormat = user32.NewProc("RegisterClipboardFormatW")
	procDragQueryFile           = shell32.NewProc("DragQueryFileW")
	cfPreferredDropEffect       = registerClipboardFormat("Preferred DropEffect")
)

func registerClipboardFormat(name string) uint {
	p, _ := syscall.UTF16PtrFromString(name)
	ret, _, _ := procRegisterClipboardFormat.Call(uintptr(unsafe.Pointer(p)))
	return uint(ret)
}

// DROPFILES structure for CF_HDROP
type dropFiles struct {
	pFiles uint32
	ptX    int32
	ptY    int32
	fNC    uint32
	fWide  uint32
}

var (
	clipboardCacheMutex sync.Mutex
	clipboardCachePaths []string
	clipboardCacheOp    OpType
	clipboardCacheTime  time.Time
)

const clipboardCacheTTL = 300 * time.Millisecond

// invalidateClipboardCache must be called whenever we change the clipboard
// ourselves, so the copy/cut markers show up on the next read.
func invalidateClipboardCache() {
	clipboardCacheMutex.Lock()
	defer clipboardCacheMutex.Unlock()
	clipboardCacheTime = time.Time{}
	clipboardCachePaths = nil
}

// getClipboardFilesCached serves the copy/cut markers. Every directory read
// asks for them - and a single refresh reads once per tab - so a short TTL
// keeps us from opening the clipboard (which is a process-wide, exclusive
// resource) over and over.
func getClipboardFilesCached() ([]string, OpType, error) {
	clipboardCacheMutex.Lock()
	if !clipboardCacheTime.IsZero() &&
		time.Since(clipboardCacheTime) < clipboardCacheTTL {
		paths, op := clipboardCachePaths, clipboardCacheOp
		clipboardCacheMutex.Unlock()
		return paths, op, nil
	}
	clipboardCacheMutex.Unlock()

	// Deliberately not holding the cache lock here: getClipboardFiles takes
	// its own lock, and setClipboardFiles walks them in the other order.
	paths, op, err := getClipboardFiles()
	if err != nil {
		return nil, op, err
	}

	clipboardCacheMutex.Lock()
	clipboardCachePaths, clipboardCacheOp = paths, op
	clipboardCacheTime = time.Now()
	clipboardCacheMutex.Unlock()

	return paths, op, nil
}

// setClipboardFiles copies file paths to clipboard with copy or cut operation.
func setClipboardFiles(paths []string, op OpType) error {
	mutex.Lock()
	defer mutex.Unlock()

	if len(paths) == 0 {
		return fmt.Errorf("no paths provided")
	}

	// Calculate memory size
	size := uint32(unsafe.Sizeof(dropFiles{}))
	for _, p := range paths {
		u, _ := syscall.UTF16FromString(p)
		size += uint32(len(u) * 2)
	}
	size += 2

	// Allocate and lock memory
	hMem := w32.GlobalAlloc(ghnd, size)
	if hMem == 0 {
		return fmt.Errorf("GlobalAlloc failed")
	}
	ptr := w32.GlobalLock(hMem)
	if ptr == nil {
		w32.GlobalFree(hMem)
		return fmt.Errorf("GlobalLock failed")
	}

	// Write DROPFILES header
	df := (*dropFiles)(unsafe.Pointer(ptr))
	df.pFiles = uint32(unsafe.Sizeof(dropFiles{}))
	df.fWide = 1

	// Write file paths
	offset := uintptr(df.pFiles)
	for _, p := range paths {
		u, _ := syscall.UTF16FromString(p)
		// Copy UTF-16 bytes directly
		for i := range u {
			*(*uint16)(unsafe.Pointer(uintptr(ptr) + offset + uintptr(i*2))) = u[i]
		}
		offset += uintptr(len(u) * 2)
	}

	w32.GlobalUnlock(hMem)

	// Open clipboard and set data
	if !w32.OpenClipboard(0) {
		w32.GlobalFree(hMem)
		return fmt.Errorf("OpenClipboard failed")
	}
	defer w32.CloseClipboard()

	w32.EmptyClipboard()

	if w32.SetClipboardData(cfHDrop, w32.HANDLE(hMem)) == 0 {
		w32.GlobalFree(hMem)
		return fmt.Errorf("SetClipboardData failed")
	}

	// Set drop effect (copy vs cut)
	effect := uint32(dropEffectCopy)
	if op == OpCut {
		effect = dropEffectMove
	}

	hEffect := w32.GlobalAlloc(ghnd, 4)
	if hEffect != 0 {
		if effectPtr := w32.GlobalLock(hEffect); effectPtr != nil {
			*(*uint32)(unsafe.Pointer(effectPtr)) = effect
			w32.GlobalUnlock(hEffect)
			w32.SetClipboardData(cfPreferredDropEffect, w32.HANDLE(hEffect))
		}
	}

	return nil
}

// dragQueryFileCount gets the number of files in HDROP
func dragQueryFileCount(hDrop w32.HDROP) uint32 {
	ret, _, _ := procDragQueryFile.Call(uintptr(hDrop), 0xFFFFFFFF, 0, 0)
	return uint32(ret)
}

// dragQueryFileSize gets the length, in characters, of the path at index,
// not counting the terminating null.
func dragQueryFileSize(hDrop w32.HDROP, index uint32) uint32 {
	ret, _, _ := procDragQueryFile.Call(uintptr(hDrop), uintptr(index), 0, 0)
	return uint32(ret)
}

// dragQueryFilePath gets the file path at index
func dragQueryFilePath(hDrop w32.HDROP, index uint32, buf []uint16) uint32 {
	ret, _, _ := procDragQueryFile.Call(uintptr(hDrop), uintptr(index), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return uint32(ret)
}

// getClipboardFiles retrieves file paths from clipboard and operation type.
func getClipboardFiles() ([]string, OpType, error) {
	mutex.Lock()
	defer mutex.Unlock()

	if !w32.IsClipboardFormatAvailable(cfHDrop) {
		return nil, OpCopy, fmt.Errorf("no files in clipboard")
	}

	if !w32.OpenClipboard(0) {
		return nil, OpCopy, fmt.Errorf("OpenClipboard failed")
	}

	hDrop := w32.HDROP(w32.GetClipboardData(cfHDrop))
	if hDrop == 0 {
		w32.CloseClipboard()
		return nil, OpCopy, fmt.Errorf("GetClipboardData failed")
	}

	// Get file count
	count := dragQueryFileCount(hDrop)
	if count == 0 {
		w32.CloseClipboard()
		return nil, OpCopy, fmt.Errorf("no files in drop data")
	}

	var paths []string

	for i := range count {
		// Ask for the length first - paths can be longer than MAX_PATH.
		size := dragQueryFileSize(hDrop, i)
		if size == 0 {
			continue
		}
		buf := make([]uint16, size+1) // room for the terminating null
		n := dragQueryFilePath(hDrop, i, buf)
		if n > 0 {
			paths = append(paths, syscall.UTF16ToString(buf[:n]))
		}
	}

	w32.CloseClipboard()

	if len(paths) == 0 {
		return nil, OpCopy, fmt.Errorf("no files in drop data")
	}

	// Check operation type
	op := OpCopy
	if w32.OpenClipboard(0) {
		if w32.IsClipboardFormatAvailable(cfPreferredDropEffect) {
			if h := w32.HANDLE(w32.GetClipboardData(cfPreferredDropEffect)); h != 0 {
				if ptr := w32.GlobalLock(w32.HGLOBAL(h)); ptr != nil {
					if *(*uint32)(unsafe.Pointer(ptr)) == dropEffectMove {
						op = OpCut
					}
					w32.GlobalUnlock(w32.HGLOBAL(h))
				}
			}
		}
		w32.CloseClipboard()
	}

	return paths, op, nil
}
