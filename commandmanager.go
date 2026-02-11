package main

import (
	"fmt"
	"time"

	"github.com/go-pkgz/fileutils"
)

type command interface {
	execute() error
	undo() error
	String() string
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

func (cm *commandManager) undo() error {
	if len(cm.history) == 0 {
		return nil
	}

	lastCmd := cm.history[len(cm.history)-1]
	err := lastCmd.undo()

	cm.history = cm.history[:len(cm.history)-1]
	cm.redoStack = append(cm.redoStack, lastCmd)
	return err
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

type copyCommand struct {
	paths []string
	dst   string
}

func (c *copyCommand) String() string {
	return fmt.Sprintf("copy paths:%d dst:%s", len(c.paths), c.dst)
}

func (c *copyCommand) execute() error {
	time.Sleep(5 * time.Second)
	for i := range c.paths {
		_ = fileutils.IsDir(c.paths[i])
	}
	return nil
}

func (c *copyCommand) undo() error {
	return nil
}
