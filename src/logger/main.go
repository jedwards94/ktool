package logger

import (
	"fmt"
)

type Color int

const (
	green   Color = iota
	yellow  Color = iota
	red     Color = iota
	magenta Color = iota
	blue    Color = iota
	cyan    Color = iota
)

func (c Color) String() string {
	switch c {
	case green:
		return "\033[32m%s\033[0m"
	case yellow:
		return "\033[33m%s\033[0m"
	case red:
		return "\033[31m%s\033[0m"
	case magenta:
		return "\033[35m%s\033[0m"
	case blue:
		return "\033[34m%s\033[0m"
	case cyan:
		return "\033[36m%s\033[0m"
	}
	return "unknown"
}

type LogLevel int

const (
	Debug   LogLevel = iota
	Info    LogLevel = iota
	Warning LogLevel = iota
	Error   LogLevel = iota
	Fatal   LogLevel = iota
)

var (
	logLevelMap = map[string]LogLevel{
		"debug":   Debug,
		"info":    Info,
		"warning": Warning,
		"error":   Error,
		"fatal":   Fatal,
	}
	logColorMap = map[LogLevel]Color{
		Debug:   cyan,
		Info:    green,
		Warning: yellow,
		Error:   red,
		Fatal:   magenta,
	}
)

type LogOptionsParams struct {
	NoColor bool
	Level   string
}

type LogOptions struct {
	NoColor bool
	Level   LogLevel
}

type Log struct {
	Name    string
	Options LogOptions
}

func (l *Log) WithGlobal(options LogOptionsParams) Log {
	return Log{
		Options: LogOptions{
			NoColor: options.NoColor,
			Level:   logLevelMap[options.Level],
		},
	}
}
func (l *Log) New(name string) Log {
	return Log{
		Name:    name,
		Options: l.Options,
	}
}

func (l *Log) Color(msg string, color Color) string {
	format := "%s"
	if !l.Options.NoColor {
		format = color.String()
	}
	return fmt.Sprintf(format, msg)
}

func (l *Log) log(logLevel LogLevel, abreviation string, msg string, a ...any) {
	if l.Options.Level > logLevel {
		return
	}
	prefix := l.Color(fmt.Sprintf("[%s][%s]", l.Name, abreviation), logColorMap[logLevel])
	fmt.Printf("%s  %s\n", prefix, fmt.Sprintf(msg, a...))
}
func (l *Log) Debug(msg string, a ...any) {
	l.log(Debug, "dbg", msg, a...)
}

func (l *Log) Info(msg string, a ...any) {
	l.log(Info, "inf", msg, a...)
}

func (l *Log) Warning(msg string, a ...any) {
	l.log(Warning, "wrn", msg, a...)
}

func (l *Log) Error(msg string, a ...any) {
	l.log(Error, "err", msg, a...)
}

func (l *Log) Fatal(msg string, a ...any) {
	l.log(Fatal, "ftl", msg, a...)
}

func (l *Log) Log(msg string, a ...any) {
	prefix := l.Color(fmt.Sprintf("[%s][log]", l.Name), blue)
	fmt.Printf("%s  %s\n", prefix, fmt.Sprintf(msg, a...))
}

var Logger = &Log{}
