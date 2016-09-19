package log

import (
	"io"
)

// First some interfaces to test on what can be done with Handlers

// CloneableHandler is a Handler which can clone itself for modification with
// the purpose of being swapped in to replace the current handler - thus utilizing the
// atomiciy of the swapper to change Handler behaviour during flight.
// Handlers not being Cloneable must be manually manipulated by the application
// and replaced by Logger.SetHandler()
//
// Making a Handler Cloneable makes it possible for the framework to support the
// standard top-level operations on it like StdFormatter and AutoColorer even
// for Handlers which cannot atomically modify their behaviour.
//
// The framework promises not to modify the Handler after it's in use.
// Once a Handler has been swapped in, it cannot be changed.

// ClonableHandler allows you to call ApplyHandlerOptions() on a Logger to swap
// in a new Handler modified by the provided HandlerOptions
type CloneableHandler interface {
	Handler
	Clone(options ...HandlerOption) CloneableHandler
}

// HandlerOption is an option-function provided by the package of a Handler
type HandlerOption func(CloneableHandler)

// Some special cloneable handler with support for stdlib tweaks
type hasFlagsOption interface {
	CloneableHandler
	SetFlags(flags int) HandlerOption
}

type hasPrefixOption interface {
	CloneableHandler
	SetPrefix(prefix string) HandlerOption
}

type hasOutputOption interface {
	CloneableHandler
	SetOutput(w io.Writer) HandlerOption
}

type hasAutoColoringOption interface {
	CloneableHandler
	AutoColoring() HandlerOption
}

/*****************************************************************************/
// Functions for manipulating the stored handler in std lib compatible ways
// These functions are a no-op for handlers not supporting the concepts
// though the swapper goes out of its way to let as many handlers as possible support
// these operations by implementing the below interfaces

// Flags return the Handler flags. Since Handlers are not modfied after being swapped in
// (unless they are StdMutables) this is safe for all.
func (h *swapper) Flags() (flag int) {
	if handler, ok := h.handler().(StdFormatter); ok {
		flag = handler.Flags()
	}
	return
}

// Prefix - same as for flags
func (h *swapper) Prefix() (prefix string) {
	if handler, ok := h.handler().(StdFormatter); ok {
		prefix = handler.Prefix()
	}
	return
}

func (h *swapper) SetFlags(flag int) {
	old := h.handler()
	// we have to atomically replace the handler with one with the new flag,
	// since locking can only be assumed to be done in 2 places:
	// swapper and stdformatter.out (when it's a syncwriter or equivalent),
	// nothing protects the flags of the formatter except replacing it entirely
	// Note that this is not a "compare-and-swap". A bad application might
	// end up swapping out another handler than the one it got the original
	// flags from. That's your own fault.
	// This operation only protects against outputting log-lines which
	// are not well defined for "some" handler.
	if clo, ok := old.(hasFlagsOption); ok {
		opt := clo.SetFlags(flag)
		new := clo.Clone(opt)
		h.SwapHandler(new)
	}
}

func (h *swapper) SetPrefix(prefix string) {
	old := h.handler()
	if clo, ok := old.(hasPrefixOption); ok {
		opt := clo.SetPrefix(prefix)
		new := clo.Clone(opt)
		h.SwapHandler(new)
	}
}

func (h *swapper) SetOutput(w io.Writer) {
	old := h.handler()
	if clo, ok := old.(hasOutputOption); ok {
		opt := clo.SetOutput(w)
		new := clo.Clone(opt)
		h.SwapHandler(new)
	}
}

// AutoColoring swaps in a equivalent handler doing AutoColoring if possible
func (h *swapper) AutoColoring() {
	old := h.handler()

	if clo, ok := old.(hasAutoColoringOption); ok {
		opt := clo.AutoColoring()
		new := clo.Clone(opt)
		h.SwapHandler(new)
	}
}

func (h *swapper) ApplyHandlerOptions(opt ...HandlerOption) {
	old := h.handler()
	if clo, ok := old.(CloneableHandler); ok {
		new := clo.Clone(opt...)
		h.SwapHandler(new)
	}
}
