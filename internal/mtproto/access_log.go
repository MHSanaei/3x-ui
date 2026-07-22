package mtproto

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// AppendAccessLog persists attributed MTProto events beside normal Xray access
// records, using Xray's line format so existing viewers and IP tracking can
// consume them without a second log pipeline.
func AppendAccessLog(path string, events []AccessEvent) error {
	if len(events) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	for _, event := range events {
		if _, err := fmt.Fprintln(w, formatAccessLogLine(event)); err != nil {
			return err
		}
	}

	return w.Flush()
}

func formatAccessLogLine(event AccessEvent) string {
	clean := func(value string) string {
		return strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) || unicode.IsControl(r) {
				return '_'
			}

			return r
		}, value)
	}

	return fmt.Sprintf(
		"%s from %s accepted tcp:%s [%s >> %s] email: %s",
		event.Timestamp.Local().Format("2006/01/02 15:04:05.000000"),
		clean(event.FromAddress),
		clean(event.ToAddress),
		clean(event.Inbound),
		clean(event.Outbound),
		clean(event.Email),
	)
}
