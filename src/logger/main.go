package logger

import "fmt"

type Log struct {
	Name string
}

func (l Log) New(name string) Log {
	return Log{
		Name: name,
	}
}

func (l *Log) Error(msg string, a ...any) {
	fmt.Printf("\033[31m[%s][err]\033[0m  %s\n", l.Name, fmt.Sprintf(msg, a...))
}

func (l *Log) Info(msg string, a ...any) {
	fmt.Printf("\033[32m[%s][inf]\033[0m  %s\n", l.Name, fmt.Sprintf(msg, a...))
}

func (l *Log) Warning(msg string, a ...any) {
	fmt.Printf("\033[33m[%s][wrn]\033[0m  %s\n", l.Name, fmt.Sprintf(msg, a...))
}

func (l *Log) Log(msg string, a ...any) {
	fmt.Printf("\033[34m[%s][log]\033[0m  %s", l.Name, fmt.Sprintf(msg, a...))
}
