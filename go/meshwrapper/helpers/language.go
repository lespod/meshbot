package helpers

import (
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"
)

func Pluralize(word string, count int) string {
	if count == 1 {
		return word
	}
	if word == "it" {
		return "them"
	}
	return word + "s"
}

func TimeAgo(timestamp time.Time) string {
	seconds := int(time.Since(timestamp).Seconds())

	if seconds == 1 {
		return "one second"
	}
	if seconds < 60 {
		return fmt.Sprintf("%d seconds", seconds)
	}

	minutes := int(math.Floor(float64(seconds) / 60))
	if minutes == 1 {
		return "one minute"
	}
	if minutes < 60 {
		return fmt.Sprintf("%d minutes", minutes)
	}

	hours := int(math.Floor(float64(minutes) / 60))
	if hours == 1 {
		return "one hour"
	}
	if hours < 24 {
		return fmt.Sprintf("%d hours", hours)
	}

	days := int(math.Floor(float64(hours) / 24))
	if days == 1 {
		return "one day"
	}
	return fmt.Sprintf("%d days", days)
}

func BreakMessage(input string) []string {
	const MAX_MESSAGE_LENGTH = 200
	input = strings.TrimSpace(input)
	messages := make([]string, 0)
	startPtr := 0
	endPtr := 0
	resumePtr := 0

	for startPtr < len(input) {
		// Find the next (rough) place where we need to cut the input to get it
		// to fit in a message
		charEnd := startPtr + MAX_MESSAGE_LENGTH

		if charEnd >= len(input) {
			// We can fit the whole rest of the input in the message, in other
			// words: we're done
			return append(messages, input[startPtr:])
		}

		// Find the "real" charEnd, that considers UTF-8 encoding
		// boundaries. This should walk back at most 4 bytes, and since
		// we're always considering 200 bytes at once, we should be fine.
		for !utf8.ValidString(input[startPtr:charEnd]) {
			charEnd--
		}

		// Break on the furthest newline that fits in the next message, if
		// the line after that can fit in a single message. Otherwise, break
		// on the furthest space. If neither is found, break on character.
		wordEnd := strings.LastIndex(input[startPtr:charEnd+1], " ")
		lineEnd := strings.LastIndex(input[startPtr:charEnd+1], "\n")

		nextLineEnd := strings.Index(input[charEnd:], "\n")
		if nextLineEnd == -1 {
			nextLineEnd = len(input)
		}
		nextLineLength := (nextLineEnd + charEnd) - (lineEnd + startPtr + 1)

		if lineEnd != -1 && nextLineLength <= MAX_MESSAGE_LENGTH {
			endPtr = lineEnd + startPtr
			resumePtr = endPtr + 1 // Skip the newline character
		} else if wordEnd != -1 {
			endPtr = wordEnd + startPtr
			resumePtr = endPtr + 1 // Skip the space character
		} else {
			endPtr = charEnd
			resumePtr = endPtr
		}

		messages = append(messages, input[startPtr:endPtr])
		startPtr = resumePtr
	}

	return messages
}
