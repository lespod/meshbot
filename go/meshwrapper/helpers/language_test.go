package helpers

import (
	"testing"
)

func TestPluralize(t *testing.T) {
	Assert(Pluralize("it", 0) == "them", "Pluralize(it, 0) == them")
	Assert(Pluralize("it", 1) == "it", "Pluralize(it, 1) == it")
	Assert(Pluralize("it", 2) == "them", "Pluralize(it, 2) == them")
	Assert(Pluralize("it", 3) == "them", "Pluralize(it, 3) == them")
	Assert(Pluralize("it", 4) == "them", "Pluralize(it, 4) == them")

	Assert(Pluralize("message", 0) == "messages", "Pluralize(message, 0) == messages")
	Assert(Pluralize("message", 1) == "message", "Pluralize(message, 1) == message")
	Assert(Pluralize("message", 2) == "messages", "Pluralize(message, 2) == messages")
	Assert(Pluralize("message", 3) == "messages", "Pluralize(message, 3) == messages")
	Assert(Pluralize("message", 4) == "messages", "Pluralize(message, 4) == messages")
}

func TestBreakMessage(t *testing.T) {
	twoHundredChars := "Helloahelloahelloahelloahelloahelloahelloahelloahelloahelloahelloahelloahelloahelloahelloahelloahelloahelloahelloahelloahelloahelloahelloahelloahelloahelloahelloahelloahelloahelloahelloahelloahelloahe"
	twoHundredCharWords := "Hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello he"

	AssertBreaking(t,
		"",
		[]string{""},
	)

	AssertBreaking(t,
		"Hello",
		[]string{"Hello"},
	)

	AssertBreaking(t,
		"Hello\nHello",
		[]string{"Hello\nHello"},
	)

	AssertBreaking(t,
		twoHundredChars,
		[]string{twoHundredChars},
	)

	AssertBreaking(t,
		twoHundredCharWords,
		[]string{twoHundredCharWords},
	)

	AssertBreaking(t,
		twoHundredChars+"a",
		[]string{
			twoHundredChars,
			"a",
		},
	)

	AssertBreaking(t,
		twoHundredChars+"📟!",
		[]string{
			twoHundredChars,
			"📟!",
		},
	)

	AssertBreaking(t,
		twoHundredChars[:len(twoHundredChars)-4]+"📟!",
		[]string{
			twoHundredChars[:len(twoHundredChars)-4] + "📟",
			"!",
		},
	)

	AssertBreaking(t,
		twoHundredChars[:len(twoHundredChars)-2]+"📟!",
		[]string{
			twoHundredChars[:len(twoHundredChars)-2],
			"📟!",
		},
	)

	AssertBreaking(t,
		twoHundredCharWords+"y",
		[]string{
			twoHundredCharWords[:len(twoHundredCharWords)-3],
			"hey",
		},
	)

	AssertBreaking(t,
		twoHundredCharWords+` Testing:
   - A thing here
   - And another one`,
		[]string{
			twoHundredCharWords,
			`Testing:
   - A thing here
   - And another one`,
		},
	)

	AssertBreaking(t,
		twoHundredCharWords+`
Testing:
   - A thing here
   - And another one`,
		[]string{
			twoHundredCharWords,
			`Testing:
   - A thing here
   - And another one`,
		},
	)

	AssertBreaking(t,
		twoHundredCharWords+"\n"+twoHundredCharWords+"\n"+"Working!\n",
		[]string{
			twoHundredCharWords,
			twoHundredCharWords,
			"Working!",
		},
	)

	AssertBreaking(t,
		`🤖👋 Hey there! I understand these commands:

✉️ Message box - An answering machine for Meshtastic
- INBOX: Check your inbox
- NEW: Get new messages
- OLD: Get old messages
- CLEAR: Clear old messages
- SEND: Leave a message (SEND <id> <message>)

📶 Signal reporting - Know what I'm seeing
- /SIGNAL: Get signal report (/SIGNAL [<id>])`,
		[]string{
			`🤖👋 Hey there! I understand these commands:

✉️ Message box - An answering machine for Meshtastic
- INBOX: Check your inbox
- NEW: Get new messages
- OLD: Get old messages`,
			`- CLEAR: Clear old messages
- SEND: Leave a message (SEND <id> <message>)

📶 Signal reporting - Know what I'm seeing
- /SIGNAL: Get signal report (/SIGNAL [<id>])`,
		},
	)

	AssertBreaking(t,
		`lsddjfksdjfhskjfhakfjhakfashflkshv fshdis uh sdkjvh aichua ssklvjhsd ivuhsv kjsdhvd iasvha vjhvl kajvh iusv sivhkjfh aklvh siuvh svhakjhslfgslkvh sdich ivhajkfhs kjvgsliv iuhv skjvhslhvljshlksjhvisudv svhlsiuvhsvjhslkcavshvluishv hslivhslkjvhskjchsldkvhjd kshv kjshv skjhv slhvkjshvlks hvlkshvlskjvh skvhsv kjshv sdfjsl fkslfj sdlfj slfj ksldfj ljdljskf lksdflkjsf lkj sdkfj lskjf sdkfjls sdgkjlk sf klj fslkj lkjdflsdkjglsdfjk lsjf slkfj lshf slkj klsjflshflksj sfkhslfh`,
		[]string{
			`lsddjfksdjfhskjfhakfjhakfashflkshv fshdis uh sdkjvh aichua ssklvjhsd ivuhsv kjsdhvd iasvha vjhvl kajvh iusv sivhkjfh aklvh siuvh svhakjhslfgslkvh sdich ivhajkfhs kjvgsliv iuhv skjvhslhvljshlksjhvisudv`,
			`svhlsiuvhsvjhslkcavshvluishv hslivhslkjvhskjchsldkvhjd kshv kjshv skjhv slhvkjshvlks hvlkshvlskjvh skvhsv kjshv sdfjsl fkslfj sdlfj slfj ksldfj ljdljskf lksdflkjsf lkj sdkfj lskjf sdkfjls sdgkjlk sf`,
			`klj fslkj lkjdflsdkjglsdfjk lsjf slkfj lshf slkj klsjflshflksj sfkhslfh`,
		},
	)

	AssertBreaking(t, `
Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed volutpat dolor rhoncus, fringilla mauris sed, tristique elit. Nulla facilisi. Phasellus orci tortor, finibus sed eleifend eget, malesuada at nisl. Nullam viverra libero sit amet metus fermentum, et vestibulum nulla fermentum. Aenean rutrum sed urna in efficitur. Curabitur ac nulla ut ante accumsan facilisis a quis arcu. Ut dapibus dolor lectus, eget semper lectus venenatis finibus. Etiam dapibus pulvinar ex, a dictum sem gravida quis. Praesent suscipit sem orci, a ultricies ligula luctus a. Fusce porta sem sed nibh feugiat condimentum.

Donec id tortor in ligula scelerisque imperdiet nec eu libero. Morbi congue hendrerit arcu, id rhoncus elit placerat ac. Vivamus efficitur quis nisi a aliquam. Quisque auctor aliquam interdum. Suspendisse facilisis lacus non efficitur ultrices. Cras a lacus dui. Maecenas neque risus, molestie ultrices velit eget, iaculis egestas erat. Praesent pharetra congue justo, id condimentum lectus pharetra sit amet. Proin a sagittis dolor, a interdum sem. Aenean at erat id augue hendrerit efficitur sed ac odio. Phasellus quis malesuada nulla. Sed id consequat erat. Curabitur sagittis eros nec sem facilisis, sed cursus purus pretium. Proin fermentum ut purus nec ultricies.
		`,
		[]string{
			`Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed volutpat dolor rhoncus, fringilla mauris sed, tristique elit. Nulla facilisi. Phasellus orci tortor, finibus sed eleifend eget, malesuada`,
			`at nisl. Nullam viverra libero sit amet metus fermentum, et vestibulum nulla fermentum. Aenean rutrum sed urna in efficitur. Curabitur ac nulla ut ante accumsan facilisis a quis arcu. Ut dapibus dolor`,
			`lectus, eget semper lectus venenatis finibus. Etiam dapibus pulvinar ex, a dictum sem gravida quis. Praesent suscipit sem orci, a ultricies ligula luctus a. Fusce porta sem sed nibh feugiat`,
			`condimentum.

Donec id tortor in ligula scelerisque imperdiet nec eu libero. Morbi congue hendrerit arcu, id rhoncus elit placerat ac. Vivamus efficitur quis nisi a aliquam. Quisque auctor aliquam`,
			`interdum. Suspendisse facilisis lacus non efficitur ultrices. Cras a lacus dui. Maecenas neque risus, molestie ultrices velit eget, iaculis egestas erat. Praesent pharetra congue justo, id condimentum`,
			`lectus pharetra sit amet. Proin a sagittis dolor, a interdum sem. Aenean at erat id augue hendrerit efficitur sed ac odio. Phasellus quis malesuada nulla. Sed id consequat erat. Curabitur sagittis`,
			`eros nec sem facilisis, sed cursus purus pretium. Proin fermentum ut purus nec ultricies.`,
		},
	)
}

func AssertBreaking(t *testing.T, message string, expected []string) {
	parts := BreakMessage(message)

	for i, part := range parts {
		if i >= len(expected) {
			t.Errorf(`Got more messages than I expected:
["%v"]`, part)
			break
		}
		if part != expected[i] {
			t.Errorf(`Expected message %d to be:
["%v"]
But got:
["%v"]`, i+1, expected[i], part)
		}
	}
}
