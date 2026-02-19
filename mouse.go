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
	doubleClick := "single"
	if c.doubleClick {
		doubleClick = "double"
	}
	return fmt.Sprintf("%d %d %s click", c.x, c.y, doubleClick)
}

func newClick(x, y int, prev *mouseClick) mouseClick {
	doubleClick := false
	t := time.Now()
	if prev != nil {
		if prev.doubleClick {
			goto out
		}
		if x != prev.x || y != prev.y {
			goto out
		}
		if prev.time.Sub(t).Milliseconds() > 500 {
			goto out
		}
		doubleClick = true
	}
out:
	return mouseClick{x, y, t, doubleClick}
}
