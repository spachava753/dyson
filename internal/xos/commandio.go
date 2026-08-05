package xos

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"
)

type commandCapture struct {
	reader *os.File
	writer *os.File
	buffer bytes.Buffer
}

type commandInput struct {
	writer *os.File
	data   []byte
}

type commandIO struct {
	stdin          *commandInput
	stdout         *commandCapture
	stderr         *commandCapture
	captures       []*commandCapture
	childFiles     []*os.File
	captureResults chan error
	stdinResult    chan error
	discard        *os.File
}

func prepareCommandIO(cmd *exec.Cmd, command Command) (*commandIO, error) {
	state := &commandIO{}
	capture := func() (*commandCapture, error) {
		reader, writer, err := os.Pipe()
		if err != nil {
			return nil, err
		}
		stream := &commandCapture{reader: reader, writer: writer}
		state.captures = append(state.captures, stream)
		state.childFiles = append(state.childFiles, writer)
		return stream, nil
	}
	discard := func() (*os.File, error) {
		if state.discard != nil {
			return state.discard, nil
		}
		file, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if err != nil {
			return nil, err
		}
		state.discard = file
		state.childFiles = append(state.childFiles, file)
		return file, nil
	}
	fail := func(err error) (*commandIO, error) {
		state.closeAll()
		return nil, err
	}

	if command.Input != nil {
		reader, writer, err := os.Pipe()
		if err != nil {
			return fail(err)
		}
		state.stdin = &commandInput{writer: writer, data: command.Input}
		state.childFiles = append(state.childFiles, reader)
		cmd.Stdin = reader
	} else {
		switch command.Stdin {
		case StreamPipe:
			reader, writer, err := os.Pipe()
			if err != nil {
				return fail(err)
			}
			state.childFiles = append(state.childFiles, reader, writer)
			cmd.Stdin = reader
		case StreamDiscard:
			file, err := os.Open(os.DevNull)
			if err != nil {
				return fail(err)
			}
			state.childFiles = append(state.childFiles, file)
			cmd.Stdin = file
		default:
			cmd.Stdin = os.Stdin
		}
	}

	switch command.Stdout {
	case StreamPipe:
		stream, err := capture()
		if err != nil {
			return fail(err)
		}
		state.stdout = stream
		cmd.Stdout = stream.writer
	case StreamDiscard:
		file, err := discard()
		if err != nil {
			return fail(err)
		}
		cmd.Stdout = file
	default:
		cmd.Stdout = os.Stdout
	}

	switch command.Stderr {
	case StreamPipe:
		stream, err := capture()
		if err != nil {
			return fail(err)
		}
		state.stderr = stream
		cmd.Stderr = stream.writer
	case StreamDiscard:
		file, err := discard()
		if err != nil {
			return fail(err)
		}
		cmd.Stderr = file
	case StreamStdout:
		cmd.Stderr = cmd.Stdout
	default:
		cmd.Stderr = os.Stderr
	}
	return state, nil
}

func (s *commandIO) start() {
	s.captureResults = make(chan error, len(s.captures))
	if s.stdin != nil {
		s.stdinResult = make(chan error, 1)
		go func() {
			_, err := io.Copy(s.stdin.writer, bytes.NewReader(s.stdin.data))
			_ = s.stdin.writer.Close()
			s.stdinResult <- err
		}()
	}
	for _, stream := range s.captures {
		go func() {
			_, err := io.Copy(&stream.buffer, stream.reader)
			_ = stream.reader.Close()
			s.captureResults <- err
		}()
	}
	for _, file := range s.childFiles {
		_ = file.Close()
	}
	s.childFiles = nil
}

func (s *commandIO) wait(ctx context.Context) (error, bool) {
	var ioErrors []error
	ctxDone := ctx.Done()
	canceled := false
	remainingCaptures := len(s.captures)
	stdinResult := s.stdinResult
	for remainingCaptures > 0 || stdinResult != nil {
		select {
		case err := <-s.captureResults:
			remainingCaptures--
			if err != nil {
				ioErrors = append(ioErrors, err)
			}
		case err := <-stdinResult:
			stdinResult = nil
			if err != nil && !errors.Is(err, syscall.EPIPE) {
				ioErrors = append(ioErrors, err)
			}
		case <-ctxDone:
			canceled = true
			if s.stdin != nil {
				_ = s.stdin.writer.Close()
			}
			for _, stream := range s.captures {
				_ = stream.reader.Close()
			}
			ctxDone = nil
		}
	}
	return errors.Join(ioErrors...), canceled
}

func (s *commandIO) closeAll() {
	if s.stdin != nil {
		_ = s.stdin.writer.Close()
	}
	for _, stream := range s.captures {
		_ = stream.reader.Close()
	}
	for _, file := range s.childFiles {
		_ = file.Close()
	}
	s.childFiles = nil
}
