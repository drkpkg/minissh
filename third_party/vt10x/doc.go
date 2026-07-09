/*
Package terminal is a vt10x terminal emulation backend, influenced
largely by st, rxvt, xterm, and iTerm as reference. Use it for terminal
muxing, a terminal emulation frontend, or wherever else you need
terminal emulation.

In development, but very usable.

This is a local fork of github.com/hinshun/vt10x@v0.0.0-20220301184237-5011da428d02
(no tagged releases upstream), vendored via a go.mod replace directive so
scrollback history support could be added — upstream discards scrolled-off
lines outright (see the history field and its use in scrollUp in state.go).
*/
package vt10x
