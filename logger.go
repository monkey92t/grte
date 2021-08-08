package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	noneIcon    = ""
	errorIcon   = " \u274C  "
	successIcon = " \u2705  "
	dockerIcon  = " \U0001F433 "
)

type logger struct {
	out io.Writer
}

func (l *logger) Print(v ...interface{})   { _, _ = l.out.Write([]byte(fmt.Sprint(v...))) }
func (l *logger) Println(v ...interface{}) { _, _ = l.out.Write([]byte(fmt.Sprintln(v...))) }
func (l *logger) Printf(format string, v ...interface{}) {
	_, _ = l.out.Write([]byte(fmt.Sprintf(format, v...)))
}

var log = logger{out: os.Stdout}

// dockerMessages is the json data output by docker,
// we only take the required fields and omit part of it.
type dockerMessage struct {
	Stream         string      `json:"stream"`
	Status         string      `json:"status"`
	Progress       string      `json:"progress"`
	ProgressDetail interface{} `json:"progressDetail"`
	ID             string      `json:"id"`
	Error          string      `json:"error"`
	ErrorDetail    struct {
		Message string `json:"message"`
	} `json:"errorDetail"`
}

func readDockerOutput(body io.ReadCloser) error {
	if body == nil {
		return nil
	}
	defer body.Close()

	scanner := bufio.NewScanner(body)

	var (
		msg     dockerMessage
		lastErr error
	)
	posMap := make(map[string]int)
	next := func() {
		for k := range posMap {
			posMap[k]++
		}
	}
	for scanner.Scan() {
		line := scanner.Bytes()

		msg.ID = ""
		msg.Stream = ""
		msg.Error = ""
		msg.ErrorDetail.Message = ""
		msg.Status = ""
		msg.Progress = ""
		msg.ProgressDetail = nil

		if err := json.Unmarshal(line, &msg); err != nil {
			iconLogf(errorIcon, "unable to unmarshal line [%s] ==> %v", string(line), err)
			lastErr = err
			next()
			continue
		}
		if msg.Error != "" {
			iconLogln(errorIcon, msg.Error)
			lastErr = errors.New(msg.Error)
			break
		}

		if msg.ErrorDetail.Message != "" {
			iconLogln(errorIcon, msg.ErrorDetail.Message)
			lastErr = errors.New(msg.Error)
			break
		}
		if msg.Status != "" {
			msg.Status = strings.TrimSpace(msg.Status)
			if msg.Progress != "" {
				dockerLogProgress(posMap[msg.ID], "%s :: %s :: %s", msg.Status, msg.ID, msg.Progress)
			} else if msg.ProgressDetail != nil && msg.ID != "" {
				if idx, ok := posMap[msg.ID]; ok {
					dockerLogProgress(idx, "%s :: %s", msg.Status, msg.ID)
				} else {
					iconLogf(dockerIcon, "%s :: %s", msg.Status, msg.ID)
					posMap[msg.ID] = 0
					next()
				}
			} else if msg.ID != "" {
				iconLogf(dockerIcon, "%s :: %s", msg.Status, msg.ID)
				next()
			} else {
				iconLogln(dockerIcon, msg.Status)
				next()
			}
		} else if msg.Stream != "" {
			iconLogln(dockerIcon, msg.Stream)
			next()
		} else {
			lastErr = fmt.Errorf("unable to handle line: %s", string(line))
			iconLogln(errorIcon, lastErr)
			next()
		}
	}
	return lastErr
}

func logf(format string, args ...interface{}) {
	log.Println(logFormat(noneIcon, format, args...))
}

func iconLogln(icon string, v ...interface{}) {
	log.Println(logFormat(icon, fmt.Sprint(v...)))
}

func iconLogf(icon, format string, args ...interface{}) {
	log.Println(logFormat(icon, format, args...))
}

func iconFail(icon string, v ...interface{}) {
	log.Println(logFormat(icon, fmt.Sprint(v...)))
	os.Exit(1)
}

func logProgress(pos int, format string, args ...interface{}) {
	log.Printf("\033[%dA\r\033[K%s\033[%dB\r", pos, logFormat(noneIcon, format, args...), pos)
}

func dockerLogProgress(pos int, format string, args ...interface{}) {
	log.Printf("\033[%dA\r\033[K%s\033[%dB\r", pos, logFormat(dockerIcon, format, args...), pos)
}

func logFormat(icon, format string, args ...interface{}) string {
	var s string
	if icon != noneIcon {
		s = fmt.Sprintf(fmt.Sprintf("%s%s", icon, format), args...)
	} else {
		s = fmt.Sprintf(format, args...)
	}

	return fmt.Sprintf("\x1b[%dm[%s] \x1b[0m%s", 36, "go-redis", s)
}
