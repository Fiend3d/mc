package main

import (
	"fmt"
	"github.com/go-pkgz/fileutils"
)

type command interface {
	execute()
	undo()
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

func (cm *commandManager) execute(cmd command) {
	cmd.execute()
	cm.history = append(cm.history, cmd)
	cm.redoStack = cm.redoStack[:0]
}

func (cm *commandManager) undo() {
	if len(cm.history) == 0 {
		return
	}

	lastCmd := cm.history[len(cm.history)-1]
	lastCmd.undo()

	cm.history = cm.history[:len(cm.history)-1]
	cm.redoStack = append(cm.redoStack, lastCmd)
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
	src   string
	dst   string
	isDir bool
}

func (c *copyCommand) String() string {
	return fmt.Sprintf("copy src:%s dst:%s isDir:%v", c.src, c.dst, c.isDir)
}

func (c *copyCommand) execute() {
	if c.isDir {
		fileutils.CopyDir(c.src, c.dst)
	} else {
		fileutils.CopyFile(c.src, c.dst)
	}
}

func (c *copyCommand) undo() {

}
