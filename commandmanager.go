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

type pathPair struct {
	src string
	dst string
}

type copyCutCommand struct {
	copy      bool
	dir       string
	pairs     []pathPair
	collision bool
}

func newCopyCutCommand(copy bool, paths []string, dst string, override bool) *copyCutCommand {
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

	return &copyCutCommand{copy, dst, pairs, collision}
}

func (c *copyCutCommand) String() string {
	if c.copy {
		return fmt.Sprintf("copy paths:%d", len(c.pairs))
	} else {
		return fmt.Sprintf("cut paths:%d", len(c.pairs))
	}
}

func (c *copyCutCommand) getDir() string {
	return c.dir
}

func (c *copyCutCommand) execute() error {
	for i := range c.pairs {
		if c.pairs[i].src == c.pairs[i].dst {
			continue
		}
		if fileutils.IsDir(c.pairs[i].src) {
			err := fileutils.CopyDir(c.pairs[i].src, c.pairs[i].dst)
			if err != nil {
				return err
			}
			if !c.copy {
				err := os.RemoveAll(c.pairs[i].src)
				if err != nil {
					return err
				}
			}
		} else {
			if c.copy {
				err := fileutils.CopyFile(c.pairs[i].src, c.pairs[i].dst)
				if err != nil {
					return err
				}
			} else {
				err := fileutils.MoveFile(c.pairs[i].src, c.pairs[i].dst)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (c *copyCutCommand) undo() error {
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
		if c.copy {
			err := os.RemoveAll(c.pairs[i].dst)
			if err != nil {
				return err
			}
		} else {
			if fileutils.IsDir(c.pairs[i].dst) {
				err := fileutils.CopyDir(c.pairs[i].dst, c.pairs[i].src)
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
