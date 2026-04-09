package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-pkgz/fileutils"
)

type command interface {
	execute() error
	undo() error
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

func (cm *commandManager) execute(cmd command) error {
	err := cmd.execute()
	cm.history = append(cm.history, cmd)
	cm.redoStack = cm.redoStack[:0]
	return err
}

func (cm *commandManager) undo() (command, error) {
	if len(cm.history) == 0 {
		return nil, nil
	}

	lastCmd := cm.history[len(cm.history)-1]
	err := lastCmd.undo()

	cm.history = cm.history[:len(cm.history)-1]
	cm.redoStack = append(cm.redoStack, lastCmd)
	return lastCmd, err
}

func (cm *commandManager) redo() (command, error) {
	if len(cm.redoStack) == 0 {
		return nil, nil
	}

	lastCmd := cm.redoStack[len(cm.redoStack)-1]
	err := lastCmd.execute()

	cm.redoStack = cm.redoStack[:len(cm.redoStack)-1]
	cm.history = append(cm.history, lastCmd)
	return lastCmd, err
}

func (cm *commandManager) canUndo() bool {
	return len(cm.history) > 0
}

func (cm *commandManager) canRedo() bool {
	return len(cm.redoStack) > 0
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

type fileActionCommand struct {
	action    fileAction
	dir       string
	pairs     []pathPair
	collision bool
}

func newFileActionCommand(action fileAction, paths []string, dst string, override bool) *fileActionCommand {
	var pairs []pathPair
	collision := false
	var reserved []string
	for i := range paths {
		name := filepath.Base(paths[i])
		dstPath := filepath.Join(dst, name)
		if override {
			if pathExists(dstPath) {
				collision = true
			}
			pairs = append(pairs, pathPair{paths[i], dstPath})
		} else {
			path := uniquePath(reserved, paths, dstPath)
			reserved = append(reserved, path)
			pairs = append(pairs, pathPair{paths[i], path})
		}
	}

	return &fileActionCommand{action, dst, pairs, collision}
}

func (c *fileActionCommand) String() string {
	switch c.action {
	case copyFileAction:
		return fmt.Sprintf("copy paths:%d", len(c.pairs))
	case cutFileAction:
		return fmt.Sprintf("cut paths:%d", len(c.pairs))
	case renameFileAction:
		return fmt.Sprintf("rename paths:%d", len(c.pairs))
	}
	return "unknown command"
}

func (c *fileActionCommand) getDir() string {
	return c.dir
}

func (c *fileActionCommand) execute() error {
	for i := range c.pairs {
		if c.pairs[i].src == c.pairs[i].dst {
			continue
		}
		if fileutils.IsDir(c.pairs[i].src) {
			empty, err := isDirEmpty(c.pairs[i].src)
			if err != nil {
				return err
			}
			if empty {
				err := os.MkdirAll(c.pairs[i].dst, 0755)
				if err != nil {
					return err
				}
			} else {
				err := fileutils.CopyDir(c.pairs[i].src, c.pairs[i].dst)
				if err != nil {
					return err
				}
			}
			switch c.action {
			case cutFileAction, renameFileAction:
				err := os.RemoveAll(c.pairs[i].src)
				if err != nil {
					return err
				}
			}
		} else {
			switch c.action {
			case copyFileAction:
				err := fileutils.CopyFile(c.pairs[i].src, c.pairs[i].dst)
				if err != nil {
					return err
				}
			case cutFileAction, renameFileAction:
				err := fileutils.MoveFile(c.pairs[i].src, c.pairs[i].dst)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (c *fileActionCommand) undo() error {
	if c.collision {
		return fmt.Errorf("there's a collision")
	}
	for i := range c.pairs {
		if !pathExists(c.pairs[i].dst) {
			return fmt.Errorf("%s doesn't exist", c.pairs[i].dst)
		}
		if c.pairs[i].src == c.pairs[i].dst {
			continue
		}
		switch c.action {
		case copyFileAction:
			err := os.RemoveAll(c.pairs[i].dst)
			if err != nil {
				return err
			}
		case cutFileAction, renameFileAction:
			if fileutils.IsDir(c.pairs[i].dst) {
				err := fileutils.CopyDir(c.pairs[i].dst, c.pairs[i].src)
				if err != nil {
					return err
				}
				err = os.RemoveAll(c.pairs[i].dst)
				if err != nil {
					return err
				}
			} else {
				err := fileutils.MoveFile(c.pairs[i].dst, c.pairs[i].src)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (c *fileActionCommand) sel() *string {
	if len(c.pairs) > 0 {
		return &c.pairs[0].dst
	}
	return nil
}

type createCommand struct {
	path  string
	isDir bool
	dir   string
}

func newCreateCommand(name string, dir string) *createCommand {
	isDir := false
	runes := []rune(name)
	if runes[len(runes)-1] == '\\' || runes[len(runes)-1] == '/' {
		isDir = true
		runes = runes[:len(runes)-1]
	}
	path := uniquePath(nil, nil, filepath.Join(dir, string(runes)))
	return &createCommand{path, isDir, dir}
}

func (c *createCommand) execute() error {
	if c.isDir {
		err := os.MkdirAll(c.path, 0755)
		if err != nil {
			return err
		}
	} else {
		err := fileutils.TouchFile(c.path)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *createCommand) undo() error {
	return os.RemoveAll(c.path)
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
