package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/fclaude/dinoscript/internal/dino"
)

func main() {
	input := flag.String("input", "", "file with one line per readint() or pause()")
	flag.Usage = func() { fmt.Fprintln(os.Stderr, "Usage: dinoscript-run [-input answers.txt] program.ds") }
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(flag.Arg(0), *input, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(path, input string, output io.Writer) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	source, err := io.ReadAll(io.LimitReader(file, dino.MaxSourceBytes+1))
	if err != nil {
		return err
	}
	var outputErr error
	console := dino.Console{Output: func(line string) {
		if outputErr == nil {
			_, outputErr = io.WriteString(output, line)
		}
	}}
	if input != "" {
		file, err := os.Open(input)
		if err != nil {
			return err
		}
		defer file.Close()
		lines := bufio.NewScanner(file)
		lines.Buffer(make([]byte, dino.MaxInputBytes+1), dino.MaxInputBytes+3)
		console.Input = func(ctx context.Context) (string, error) {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			if lines.Scan() {
				return lines.Text(), nil
			}
			if err := lines.Err(); err != nil {
				return "", err
			}
			return "", io.EOF
		}
	}
	if err := dino.Run(context.Background(), string(source), console); err != nil {
		return err
	}
	return outputErr
}
