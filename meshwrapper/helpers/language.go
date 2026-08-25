package helpers

import (
	"fmt"
	"math"
	"strconv"
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
		return "sekundę"
	}
	if seconds < 60 {
		return fmt.Sprintf("%d %s", seconds, polishSecondWord(seconds))
	}

	minutes := int(math.Floor(float64(seconds) / 60))
	if minutes == 1 {
		return "minutę"
	}
	if minutes < 60 {
		return fmt.Sprintf("%d %s", minutes, polishMinuteWord(minutes))
	}

	hours := int(math.Floor(float64(minutes) / 60))
	if hours == 1 {
		return "godzinę"
	}
	if hours < 24 {
		return fmt.Sprintf("%d %s", hours, polishHourWord(hours))
	}

	days := int(math.Floor(float64(hours) / 24))
	if days == 1 {
		return "dzień"
	}
	return fmt.Sprintf("%d %s", days, polishDayWord(days))
}

func PolishHopWord(count int) string {
	if count == 1 {
		return "hop"
	}
	if count%10 >= 2 && count%10 <= 4 && (count%100 < 12 || count%100 > 14) {
		return "hopy"
	}
	return "hopów"
}

func PolishJumpWord(count int) string {
	if count == 1 {
		return "przeskok"
	}
	if count%10 >= 2 && count%10 <= 4 && (count%100 < 12 || count%100 > 14) {
		return "przeskoki"
	}
	return "przeskoków"
}

func polishSecondWord(count int) string {
	if count%10 >= 2 && count%10 <= 4 && (count%100 < 12 || count%100 > 14) {
		return "sekundy"
	}
	return "sekund"
}

func polishMinuteWord(count int) string {
	if count%10 >= 2 && count%10 <= 4 && (count%100 < 12 || count%100 > 14) {
		return "minuty"
	}
	return "minut"
}

func polishHourWord(count int) string {
	if count%10 >= 2 && count%10 <= 4 && (count%100 < 12 || count%100 > 14) {
		return "godziny"
	}
	return "godzin"
}

func polishDayWord(count int) string {
	if count%10 >= 2 && count%10 <= 4 && (count%100 < 12 || count%100 > 14) {
		return "dni"
	}
	return "dni"
}

func BreakMessage(input string) []string {
	const MAX_MESSAGE_LENGTH = 200
	const MAX_LENGTH_WITH_PAGINATION = 200 - len(" [1/2]")
	input = strings.TrimSpace(input)
	messages := make([]string, 0)
	for _, message := range strings.Split(input, "<BREAK-MESSAGE>") {
		message = strings.TrimSpace(message)

		// Nie dziel wiadomości, które mieszczą się w limicie.
		if len(message) <= MAX_MESSAGE_LENGTH {
			messages = append(messages, message)
			continue
		}

		// Podziel wiadomość i dodaj numerację do każdej części.
		messageParts := BreakMessageAt(message, MAX_LENGTH_WITH_PAGINATION)
		for i := range messageParts {
			if len(messageParts) > 9 {
				messageParts[i] += " [" + strconv.Itoa(i+1) + "]"
			} else {
				messageParts[i] += " [" + strconv.Itoa(i+1) + "/" + strconv.Itoa(len(messageParts)) + "]"
			}
		}

		messages = append(messages, messageParts...)
	}
	Assert(len(messages) < 1000, "Tworzysz zbyt wiele wiadomości naraz")
	return messages
}

func BreakMessageAt(input string, maxlength int) []string {
	input = strings.TrimSpace(input)
	messages := make([]string, 0)
	startPtr := 0
	endPtr := 0
	resumePtr := 0

	for startPtr < len(input) {
		// Znajdź przybliżone miejsce cięcia, żeby część zmieściła się w wiadomości.
		charEnd := startPtr + maxlength

		if charEnd >= len(input) {
			// Reszta tekstu mieści się w jednej wiadomości, więc kończymy.
			messages = append(messages, input[startPtr:])
			break
		}

		// Skoryguj koniec tak, żeby nie przeciąć znaku UTF-8.
		for !utf8.ValidString(input[startPtr:charEnd]) {
			charEnd--
		}

		// Najpierw tnij po nowej linii, potem po spacji, a ostatecznie po znaku.
		wordEnd := strings.LastIndex(input[startPtr:charEnd+1], " ")
		lineEnd := strings.LastIndex(input[startPtr:charEnd+1], "\n")

		nextLineEnd := strings.Index(input[charEnd:], "\n")
		if nextLineEnd == -1 {
			nextLineEnd = len(input)
		}
		nextLineLength := (nextLineEnd + charEnd) - (lineEnd + startPtr + 1)

		if lineEnd != -1 && nextLineLength <= maxlength {
			endPtr = lineEnd + startPtr
			resumePtr = endPtr + 1 // Pomiń znak nowej linii.
		} else if wordEnd != -1 {
			endPtr = wordEnd + startPtr
			resumePtr = endPtr + 1 // Pomiń spację.
		} else {
			endPtr = charEnd
			resumePtr = endPtr
		}

		messages = append(messages, input[startPtr:endPtr])
		startPtr = resumePtr
	}

	return messages
}

func Indent(s, prefix string) string {
	lines := strings.SplitAfter(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "")
}
