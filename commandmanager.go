package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-pkgz/fileutils"
)

type command interface {
	execute() error
	undo() (command, error)
	String() string
	getDir() string
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
	_, err := lastCmd.undo()

	cm.history = cm.history[:len(cm.history)-1]
	cm.redoStack = append(cm.redoStack, lastCmd)
	return lastCmd, err
}

func (cm *commandManager) redo() {
	if len(cm.redoStack) == 0 {
		return
	}

	lastCmd := cm.redoStack[len(cm.redoStack)-1]
	lastCmd.execute()

	cm.redoStack = cm.redoStack[:len(cm.redoStack)-1]
	cm.history = append(cm.history, lastCmd)
}

func (cm *commandManager) canUndo() bool {
	return len(cm.history) > 0
}

func (cm *commandManager) canRedo() bool {
	return len(cm.redoStack) > 0
}

type pathPair struct {
	src string
	dst string
}

type copyCommand struct {
	dir       string
	pairs     []pathPair
	collision bool
}

func newCopyCommand(paths []string, dst string, override bool) *copyCommand {
	var pairs []pathPair
	collision := false
	for i := range paths {
		name := filepath.Base(paths[i])
		dstPath := filepath.Join(dst, name)
		if override {
			if pathExists(dstPath) {
				collision = true
			}
			pairs = append(pairs, pathPair{paths[i], dstPath})
		} else {
			pairs = append(pairs, pathPair{paths[i], uniquePath(dstPath)})
		}
	}

	return &copyCommand{dst, pairs, collision}
}

func (c *copyCommand) String() string {
	return fmt.Sprintf("copy paths:%d", len(c.pairs))
}

func (c *copyCommand) getDir() string {
	return c.dir
}

func (c *copyCommand) execute() error {
	for i := range c.pairs {
		if fileutils.IsDir(c.pairs[i].src) {
			err := fileutils.CopyDir(c.pairs[i].src, c.pairs[i].dst)
			if err != nil {
				return err
			}
		} else {
			err := fileutils.CopyFile(c.pairs[i].src, c.pairs[i].dst)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *copyCommand) undo() (command, error) {
	if c.collision {
		return nil, fmt.Errorf("there's a collision")
	}
	for i := range c.pairs {
		err := os.RemoveAll(c.pairs[i].dst)
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}
