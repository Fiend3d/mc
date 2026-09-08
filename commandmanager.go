package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mc/shutil"
)

type command interface {
	execute() error
	undo() error
	undoable() bool // false commands never reach the history
	String() string
	getDir() string
	sel() *string // select is a keyword
}

type commandManager struct {
	history   []command
	redoStack []command
}

func newCommandManager() *commandManager {
	return &commandManager{
		history:   make([]command, 0),
		redoStack: make([]command, 0),
	}
}

func (cm *commandManager) pushHistory(cmd command) {
	cm.history = append(cm.history, cmd)
	cm.redoStack = cm.redoStack[:0]
}

func (cm *commandManager) canUndo() bool {
	return len(cm.history) > 0
}

func (cm *commandManager) canRedo() bool {
	return len(cm.redoStack) > 0
}

func (cm *commandManager) peekUndo() (command, error) {
	if len(cm.history) == 0 {
		return nil, fmt.Errorf("nothing to undo")
	}
	return cm.history[len(cm.history)-1], nil
}

func (cm *commandManager) commitUndo() {
	lastCmd := cm.history[len(cm.history)-1]
	cm.history = cm.history[:len(cm.history)-1]
	cm.redoStack = append(cm.redoStack, lastCmd)
}

func (cm *commandManager) peekRedo() (command, error) {
	if len(cm.redoStack) == 0 {
		return nil, fmt.Errorf("nothing to redo")
	}
	return cm.redoStack[len(cm.redoStack)-1], nil
}

func (cm *commandManager) commitRedo() {
	lastCmd := cm.redoStack[len(cm.redoStack)-1]
	cm.redoStack = cm.redoStack[:len(cm.redoStack)-1]
	cm.history = append(cm.history, lastCmd)
}

type deleteCommand struct {
	dir   string
	paths []string
}

func (c *deleteCommand) String() string {
	return fmt.Sprintf("delete %d paths", len(c.paths))
}

func (c *deleteCommand) getDir() string {
	return c.dir
}

func (c *deleteCommand) execute() error {
	for i := range c.paths {
		err := os.RemoveAll(c.paths[i])
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *deleteCommand) undo() error {
	return fmt.Errorf("can't be undone")
}

func (c *deleteCommand) undoable() bool {
	return false
}

func (c *deleteCommand) sel() *string {
	return nil
}

type pathPair struct {
	src string
	dst string
}

type fileAction int

const (
	copyFileAction fileAction = iota
	cutFileAction
	renameFileAction
)

// buildRenamePairs maps every new name onto a destination path.
//
// Names that weren't edited are left exactly as they are: such a path already
// exists - as itself - so putting it through UniquePath would turn "a.txt"
// into "a1.txt" and rename a file the user never touched. Windows paths are
// case-insensitive, so a case-only edit is compared the same way, which lets
// "foo.txt" -> "Foo.txt" through as a real rename instead of "Foo1.txt".
func buildRenamePairs(paths []string, names []string) []pathPair {
	pairs := make([]pathPair, 0, len(names))
	reserved := make([]string, 0, len(names))
	for i := range names {
		src := paths[i]
		dst := filepath.Join(filepath.Dir(src), names[i])
		if !strings.EqualFold(dst, src) {
			dst = shutil.UniquePath(reserved, paths, dst)
		}
		reserved = append(reserved, dst)
		pairs = append(pairs, pathPair{src, dst})
	}
	return pairs
}

type fileActionCommand struct {
	action            fileAction
	dir               string
	pairs             []pathPair
	collision         bool
	overwrite         bool
	overwriteExisting map[string]bool
	journal           []shutil.Record
	redoJournal       []shutil.Record
}

func newFileActionCommand(action fileAction, paths []string, dst string, override bool) *fileActionCommand {
	var pairs []pathPair
	collision := false
	existing := map[string]bool{}
	var reserved []string
	for i := range paths {
		name := filepath.Base(paths[i])
		dstPath := filepath.Join(dst, name)
		if override {
			if shutil.PathExists(dstPath) {
				collision = true
				existing[dstPath] = true
			}
			pairs = append(pairs, pathPair{paths[i], dstPath})
		} else {
			path := shutil.UniquePath(reserved, paths, dstPath)
			reserved = append(reserved, path)
			pairs = append(pairs, pathPair{paths[i], path})
		}
	}

	return &fileActionCommand{action: action, dir: dst, pairs: pairs, collision: collision, overwrite: override, overwriteExisting: existing}
}

func (c *fileActionCommand) String() string {
	switch c.action {
	case copyFileAction:
		return fmt.Sprintf("copy %d items", len(c.pairs))
	case cutFileAction:
		return fmt.Sprintf("move %d items", len(c.pairs))
	case renameFileAction:
		return fmt.Sprintf("rename %d items", len(c.pairs))
	}
	return "unknown command"
}

func (c *fileActionCommand) getDir() string {
	return c.dir
}

func (c *fileActionCommand) execute() error { return c.executeTask(context.Background(), nil) }
func (c *fileActionCommand) undo() error {
	return shutil.UndoJournal(context.Background(), &c.journal, &c.redoJournal, nil)
}

func (c *fileActionCommand) undoable() bool {
	return !c.collision
}

func (c *fileActionCommand) sel() *string {
	if len(c.pairs) > 0 {
		return &c.pairs[0].dst
	}
	return nil
}

type createCommand struct {
	snapshot map[string]shutil.Stamp
	path     string
	isDir    bool
	dir      string
}

func newCreateCommand(name string, dir string) *createCommand {
	isDir := false
	runes := []rune(name)
	if len(runes) > 0 &&
		(runes[len(runes)-1] == '\\' || runes[len(runes)-1] == '/') {
		isDir = true
		runes = runes[:len(runes)-1]
	}
	path := shutil.UniquePath(nil, nil, filepath.Join(dir, string(runes)))
	return &createCommand{path: path, isDir: isDir, dir: dir}
}

func (c *createCommand) execute() error {
	if shutil.PathExists(c.path) {
		return fmt.Errorf("path already exists: %s", c.path)
	}
	if err := shutil.CheckDestination(c.path); err != nil {
		return err
	}
	var err error
	if c.isDir {
		err = os.MkdirAll(c.path, 0755)
	} else {
		var f *os.File
		f, err = os.OpenFile(c.path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			err = f.Close()
		}
	}
	if err != nil {
		return err
	}
	c.snapshot, err = shutil.Snapshot(c.path)
	return err
}
func (c *createCommand) undo() error {
	if err := shutil.Unchanged(c.path, c.snapshot); err != nil {
		return err
	}
	return os.Remove(c.path)
}

func (c *createCommand) undoable() bool {
	return true
}

func (c *createCommand) getDir() string {
	return c.dir
}

func (c *createCommand) String() string {
	return fmt.Sprintf("create %s", c.path)
}

func (c *createCommand) sel() *string {
	return &c.path
}
