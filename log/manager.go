package log

import (
	"strings"
	"sync"
)

// A Logger manager to look up Loggers by name, blatantly borrowed from
// Python "logging" module (Copyright (C) 2001-2014 Vinay Sajip.), translated to Go.
// ... it's seems like the only straight forward way to do this.

type placeholder struct {
	loggers []*Logger
}

// must be called under manager mutex lock
func (p *placeholder) append(l *Logger) {
	for _, k := range p.loggers {
		if k == l {
			return
		}
	}
	p.loggers = append(p.loggers, l)
}

func newPlaceholder(l *Logger) (p *placeholder) {
	p = &placeholder{loggers: make([]*Logger, 0, 1)}
	p.append(l)
	return
}

type manager struct {
	mu       sync.Mutex
	root     *Logger
	registry map[string]interface{} // contains either *Logger or *placeholder
}

var man *manager

func newManager(l *Logger) *manager {
	m := &manager{root: l}
	m.registry = make(map[string]interface{})
	return m
}

// GetLogger creates a new Logger or returns an already existing with the given name.
func GetLogger(name string) (l *Logger) {
	return man.getLogger(name)
}

func (m *manager) getLogger(name string) (l *Logger) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if node, ok := m.registry[name]; ok {
		if p, ok := node.(*placeholder); ok {
			l = newLogger(name)
			m.registry[name] = l
			m.fixupChildren(p, l)
			m.fixupParents(l)
			return
		}
		l = node.(*Logger) // must be a Logger.
	} else {
		l = newLogger(name)
		m.registry[name] = l
		m.fixupParents(l)
	}
	return
}

// Ensure that there are either loggers or placeholders all the way
// from the specified logger to the root of the logger hierarchy.
func (m *manager) fixupParents(l *Logger) {

	var parent *Logger
	name := l.name
	i := strings.LastIndexByte(name, '/')

	for i > 0 && parent == nil {
		name = name[:i]
		if node, ok := m.registry[name]; ok {
			if p, ok := node.(*placeholder); ok {
				// The logger will be child of this placeholder,
				// somewhere in the subtree.
				// if anybody asks later.
				p.append(l)
			} else { // it' s a *Logger, make it the parent
				if n, ok := node.(*Logger); ok {
					parent = n
				} else {
					// unreachable
					panic("Bad logger in registry")
				}
			}
		} else {
			m.registry[name] = newPlaceholder(l)
		}
		i = strings.LastIndexByte(name, '/')
	}
	if parent == nil {
		// If the logger inherited the root as parent from a child which previous had
		// the root as parent, this will just set it again
		parent = m.root
	}
	l.h.SwapParent(parent)
}

// Ensure that children of the placeholder ph are connected to the
// specified logger.
func (m *manager) fixupChildren(p *placeholder, l *Logger) {
	name := l.name
	for _, c := range p.loggers {
		cp := c.h.parent()
		// If the child this placeholder records has a parent which the new logger is
		// a prefix for, then the parent relation ship is below is in the tree,
		// else the childs parent is a above us and we need to insert our self
		// as the childs new parent and take over it's old parent as our own.
		if len(cp.name) < len(name) || cp.name[:len(name)] != name {
			// The child of this placeholder has a real parent higher in the tree.
			// We need to insert the new Logger here, making it a parent of the child,
			// and taking over the original parent.
			// We need atomic here, since the child Logger might be in use while
			// we do this
			// l.parent = c.parent
			// c.parent = l
			grand_parent := c.h.SwapParent(l) // set new logger as parent for the existing logger
			l.h.SwapParent(grand_parent)      // inherit the existing loggers parent for our selves
		}
	}
}
