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
	now := time.Now()

	if prev == nil {
		return mouseClick{x, y, now, false}
	}

	if prev.doubleClick {
		return mouseClick{x, y, now, false}
	}

	if x != prev.x || y != prev.y {
		return mouseClick{x, y, now, false}
	}

	if now.Sub(prev.time) > 500*time.Millisecond {
		return mouseClick{x, y, now, false}
	}

	return mouseClick{x, y, now, true}
}
