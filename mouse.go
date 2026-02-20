package main

import (
	"fmt"
	"time"
)

type mouseClick struct {
	x           int
	y           int
	time        time.Time
	doubleClick bool
}

func (c *mouseClick) String() string {
	if c.doubleClick {
		return fmt.Sprintf("%d %d double click", c.x, c.y)
	} else {
		return fmt.Sprintf("%d %d click", c.x, c.y)
	}
}

func newClick(x, y int, prev *mouseClick) mouseClick {
	t := time.Now()

	doubleClick := prev != nil &&
		!prev.doubleClick &&
		x == prev.x &&
		y == prev.y &&
		t.Sub(prev.time) <= 500*time.Millisecond

	return mouseClick{x, y, t, doubleClick}
}
