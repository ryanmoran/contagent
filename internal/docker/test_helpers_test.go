package docker_test

import (
	"bytes"
	"io"
)

type mockWriter struct {
	buf *bytes.Buffer
}

func newMockWriter() *mockWriter {
	return &mockWriter{buf: &bytes.Buffer{}}
}

func (m *mockWriter) Print(v ...interface{}) { m.buf.WriteString(sprint(v...)) }
func (m *mockWriter) Printf(format string, v ...interface{}) {
	m.buf.WriteString(sprintf(format, v...))
}
func (m *mockWriter) Println(v ...interface{}) { m.buf.WriteString(sprintln(v...)) }
func (m *mockWriter) Warning(v ...interface{}) { m.buf.WriteString("Warning: " + sprintln(v...)) }
func (m *mockWriter) Warningf(format string, v ...interface{}) {
	m.buf.WriteString("Warning: " + sprintf(format, v...) + "\n")
}
func (m *mockWriter) Fatal(v ...interface{}) { m.buf.WriteString("Fatal: " + sprintln(v...)) }
func (m *mockWriter) Fatalf(format string, v ...interface{}) {
	m.buf.WriteString("Fatal: " + sprintf(format, v...) + "\n")
}
func (m *mockWriter) GetWriter() io.Writer { return m.buf }
func (m *mockWriter) String() string       { return m.buf.String() }

func sprint(v ...interface{}) string {
	var buf bytes.Buffer
	for i, val := range v {
		if i > 0 {
			buf.WriteString(" ")
		}
		buf.WriteString(toString(val))
	}
	return buf.String()
}

func sprintf(format string, v ...interface{}) string {
	result := format
	for _, val := range v {
		result = replaceFirst(result, "%v", toString(val))
		result = replaceFirst(result, "%s", toString(val))
		result = replaceFirst(result, "%d", toString(val))
	}
	return result
}

func sprintln(v ...interface{}) string {
	return sprint(v...) + "\n"
}

func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case int:
		return itoa(val)
	case int64:
		return itoa64(val)
	case error:
		return val.Error()
	default:
		return ""
	}
}

func replaceFirst(s, old, new string) string {
	idx := 0
	for i := 0; i < len(s)-len(old)+1; i++ {
		if s[i:i+len(old)] == old {
			idx = i
			return s[:idx] + new + s[idx+len(old):]
		}
	}
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var buf [20]byte
	i := len(buf) - 1
	for n > 0 {
		buf[i] = byte('0' + n%10)
		n /= 10
		i--
	}
	if negative {
		buf[i] = '-'
		i--
	}
	return string(buf[i+1:])
}

func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var buf [20]byte
	i := len(buf) - 1
	for n > 0 {
		buf[i] = byte('0' + n%10)
		n /= 10
		i--
	}
	if negative {
		buf[i] = '-'
		i--
	}
	return string(buf[i+1:])
}
