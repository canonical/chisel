package setup

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"
)

// The "channel" field of a slice definition holds patterns selecting which
// concrete "<track>/<risk>" channels an entry applies to. The track is a
// literal and only the risk part accepts operators:
//
//	*            - Any risk of that track
//	!<risk>      - Any risk of that track but that one
//	<risk>[,...] - Only those risks of that track
//
// Patterns are kept as written and interpreted on each match, as done for
// globs in the strdist package. They are validated when the release is read so
// that a malformed value is reported early, and rendered back verbatim.

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

// The form a channel pattern must take, as reported to the user.
const channelPatternForm = "<track>/<risk>"

// splitChannel splits a channel or a channel pattern on "/", checking what the
// two have in common: no spaces and no empty segment. How many segments are
// expected and what each one means is left to the caller, as that is where the
// two differ. form is the shape reported on error.
func splitChannel(value, form string) ([]string, error) {
	if strings.ContainsFunc(value, unicode.IsSpace) {
		return nil, errors.New("must not contain spaces")
	}
	segments := strings.Split(value, "/")
	if slices.Contains(segments, "") {
		return nil, fmt.Errorf("must be %s", form)
	}
	return segments, nil
}

// knownRisks holds every risk a channel may hold, from the most to the least
// stable. The set is defined by the store and does not depend on its content,
// hence risks are validated as architectures are.
var knownRisks = []string{"stable", "candidate", "beta", "edge"}

// validateRisk validates a single risk of a channel or of a channel pattern.
func validateRisk(risk string) error {
	if !slices.Contains(knownRisks, risk) {
		return fmt.Errorf("unknown risk %q, must be one of %s", risk, strings.Join(knownRisks, ", "))
	}
	return nil
}

// validateChannelPatterns validates the values of a "channel" field. A track
// may appear at most once across the values so that the resulting set of
// channels is unambiguous.
func validateChannelPatterns(patterns []string) error {
	seen := make(map[string]bool, len(patterns))
	for _, pattern := range patterns {
		track, err := validateChannelPattern(pattern)
		if err != nil {
			return fmt.Errorf("%q: %s", pattern, err)
		}
		if seen[track] {
			return fmt.Errorf("track %q is repeated", track)
		}
		seen[track] = true
	}
	return nil
}

// validateChannelPattern validates a single pattern and returns its track. A
// pattern holds no branch, hence exactly one track and one risk part.
func validateChannelPattern(pattern string) (track string, err error) {
	segments, err := splitChannel(pattern, channelPatternForm)
	if err != nil {
		return "", err
	}
	if len(segments) != 2 {
		return "", fmt.Errorf("must be %s", channelPatternForm)
	}
	track, riskPart := segments[0], segments[1]
	if strings.ContainsAny(track, "*!,") {
		return "", errors.New("only the risk accepts '*', '!' and ','")
	}
	if riskPart != "*" && strings.Contains(riskPart, "*") {
		// Checked before the risk part takes one of the forms below, as a
		// wildcard is not allowed within any of them.
		return "", errors.New("'*' must be the whole risk")
	}

	// The risk part takes one of the three forms of the grammar.
	switch {
	case riskPart == "*":
		// Every risk of the track, nothing more to validate.
	case strings.HasPrefix(riskPart, "!"):
		// Every risk of the track but the excluded one.
		except := strings.TrimPrefix(riskPart, "!")
		if strings.Contains(except, ",") {
			return "", errors.New("'!' cannot be combined with other risks")
		}
		if except == "" {
			return "", fmt.Errorf("must be %s", channelPatternForm)
		}
		if err := validateRisk(except); err != nil {
			return "", err
		}
	default:
		// Only the listed risks of the track.
		risks := strings.Split(riskPart, ",")
		for i, risk := range risks {
			if risk == "" {
				return "", fmt.Errorf("must be %s", channelPatternForm)
			}
			if strings.Contains(risk, "!") {
				return "", errors.New("'!' must prefix the whole risk")
			}
			if slices.Contains(risks[:i], risk) {
				return "", fmt.Errorf("risk %q is repeated", risk)
			}
			if err := validateRisk(risk); err != nil {
				return "", err
			}
		}
	}
	return track, nil
}

// MatchChannelPatterns reports whether the concrete "<track>/<risk>" channel
// matches any of the patterns. An empty list matches every channel, which means
// the entry is not channel specific.
//
// A branch, as in "<track>/<risk>/<branch>", is ignored. Branches are ephemeral
// and thus never part of a pattern, so an entry applies to every branch of the
// risk it matches.
func MatchChannelPatterns(patterns []string, channel Channel) bool {
	if len(patterns) == 0 {
		return true
	}
	if channel.Track == "" || channel.Risk == "" {
		// A channel without a risk is not a channel. Never match it, rather
		// than treat the missing risk as one that differs from an excluded one.
		return false
	}
	for _, pattern := range patterns {
		if matchChannel(pattern, channel.Track, channel.Risk) {
			return true
		}
	}
	return false
}

// matchChannel reports whether the pattern matches the track and the risk of a
// concrete channel. Note that the exclusion form is scoped to its own track, so
// "1.0/!stable" does not match any risk of the "2.0" track.
//
// The pattern is expected to be valid, as ensured when the release is read.
func matchChannel(pattern, track, risk string) bool {
	patternTrack, riskPart, _ := strings.Cut(pattern, "/")
	if track != patternTrack {
		return false
	}
	if riskPart == "*" {
		return true
	}
	if except, ok := strings.CutPrefix(riskPart, "!"); ok {
		return risk != except
	}
	return slices.Contains(strings.Split(riskPart, ","), risk)
}
