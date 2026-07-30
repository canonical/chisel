package setup

import (
	"errors"
	"strings"
	"unicode"
)

// Channel is a store channel, as in "<track>/<risk>[/<branch>]".
type Channel struct {
	Track  string
	Risk   string
	Branch string
}

func (c Channel) String() string {
	if c.Track == "" {
		return ""
	}
	channel := c.Track
	if c.Risk != "" {
		channel += "/" + c.Risk
	}
	if c.Branch != "" {
		channel += "/" + c.Branch
	}
	return channel
}

// parseChannel parses a "<track>[/<risk>[/<branch>]]" channel, as written.
// Validation is intentionally loose, the track, the risk and the branch are
// only checked for their presence so that their values are not rejected here.
func parseChannel(channel string) (Channel, error) {
	if channel == "" {
		return Channel{}, errors.New("missing channel")
	}
	if strings.ContainsFunc(channel, unicode.IsSpace) {
		return Channel{}, errors.New("channel must not contain spaces")
	}
	segments := strings.Split(channel, "/")
	if len(segments) > 3 {
		return Channel{}, errors.New("channel must be <track>[/<risk>[/<branch>]]")
	}
	for _, segment := range segments {
		if segment == "" {
			return Channel{}, errors.New("channel must be <track>[/<risk>[/<branch>]]")
		}
	}
	parsed := Channel{Track: segments[0]}
	if len(segments) > 1 {
		parsed.Risk = segments[1]
	}
	if len(segments) > 2 {
		parsed.Branch = segments[2]
	}
	return parsed, nil
}
