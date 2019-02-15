package main

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/davecgh/go-spew/spew"
)

var (
	// EmojiRegex to get emoji IDs
	// thanks ym
	EmojiRegex = regexp.MustCompile(`<(.*?):(.*?):(\d+)>`)
)

// returns map[ID][]{name, url}
func parseEmojis(content string) (formatted string, emojiMap map[string][]string) {
	emojiMap = make(map[string][]string)
	formatted = content

	emojiIDs := EmojiRegex.FindAllStringSubmatch(content, -1)
	for _, nameandID := range emojiIDs {
		if len(nameandID) < 4 {
			continue
		}

		log.Println(spew.Sdump(nameandID))

		if _, ok := emojiMap[nameandID[3]]; !ok {
			var format = "png"
			if nameandID[1] != "" {
				format = "gif"
			}

			formatted = strings.Replace(
				formatted,
				nameandID[0],
				"["+nameandID[2]+"]",
				-1,
			)

			emojiMap[nameandID[3]] = []string{
				nameandID[2],
				fmt.Sprintf(
					`https://cdn.discordapp.com/emojis/%s.%s`,
					nameandID[3], format,
				),
			}
		}
	}

	return
}