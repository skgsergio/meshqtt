package main

import (
	"os"

	"github.com/muesli/termenv"
)

// Small wrapper helpers around termenv so the rest of the code can stay
// simple and we can later centralize any behavior changes (e.g. disabling
// colors when not on a TTY).

var (
	output = termenv.NewOutput(os.Stdout)
	p      = output.ColorProfile()
)

func bold(s string) string {
	return output.String(s).Bold().String()
}

func dim(s string) string {
	return output.String(s).Faint().String()
}

func cyan(s string) string {
	return output.String(s).Foreground(p.Color("6")).String()
}

func green(s string) string {
	return output.String(s).Foreground(p.Color("2")).String()
}

func magenta(s string) string {
	return output.String(s).Foreground(p.Color("5")).String()
}

func yellow(s string) string {
	return output.String(s).Foreground(p.Color("3")).String()
}

func link(text, url string) string {
	return output.Hyperlink(url, text)
}
