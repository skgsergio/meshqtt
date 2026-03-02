package main

import "github.com/fatih/color"

// Small wrapper helpers around fatih/color so the rest of the code can stay
// simple and we can later centralize any behavior changes (e.g. disabling
// colors when not on a TTY).

var (
	boldStyle    = color.New(color.Bold)
	dimStyle     = color.New(color.Faint)
	cyanStyle    = color.New(color.FgCyan)
	greenStyle   = color.New(color.FgGreen)
	magentaStyle = color.New(color.FgMagenta)
	yellowStyle  = color.New(color.FgYellow)
)

func bold(s string) string {
	return boldStyle.Sprint(s)
}

func dim(s string) string {
	return dimStyle.Sprint(s)
}

func cyan(s string) string {
	return cyanStyle.Sprint(s)
}

func green(s string) string {
	return greenStyle.Sprint(s)
}

func magenta(s string) string {
	return magentaStyle.Sprint(s)
}

func yellow(s string) string {
	return yellowStyle.Sprint(s)
}


