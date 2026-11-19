package setup

import (
	"errors"
	"fmt"
	"slices"
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
	if c == (Channel{}) {
		return ""
	}
	channel := c.Risk
	if c.Track != "" {
		channel = c.Track + "/" + channel
	}
	if c.Branch != "" {
		channel += "/" + c.Branch
	}
	return channel
}

const DefaultRisk = "stable"

// The form a channel pattern must take, as reported to the user.
const channelPatternForm = "<track>/<risk-pattern>"

// splitChannel splits a channel or a channel pattern on "/", rejecting values
// holding spaces or an empty segment.
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
// stable. The set is defined by the store and does not depend on its content.
var knownRisks = []string{"stable", "candidate", "beta", "edge"}

func validateRisk(risk string) error {
	if !slices.Contains(knownRisks, risk) {
		return fmt.Errorf("unknown risk %q, must be one of %s", risk, strings.Join(knownRisks, ", "))
	}
	return nil
}

// validateChannelPatterns validates a list of patterns. A track may appear
// at most once across the patterns so that the resulting set of channels is
// unambiguous.
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
// pattern holds no branch, hence exactly one track and one risk pattern.
func validateChannelPattern(pattern string) (track string, err error) {
	segments, err := splitChannel(pattern, channelPatternForm)
	if err != nil {
		return "", err
	}
	if len(segments) != 2 {
		return "", fmt.Errorf("must be %s", channelPatternForm)
	}
	track, riskPattern := segments[0], segments[1]
	if strings.ContainsAny(track, "*!,") {
		return "", errors.New("only the risk accepts '*', '!' and ','")
	}
	if riskPattern != "*" && strings.Contains(riskPattern, "*") {
		// Checked before the risk pattern takes one of the forms below, as a
		// wildcard is not allowed within any of them.
		return "", errors.New("'*' must be the whole risk")
	}

	// The risk pattern takes one of the three forms of the grammar.
	switch {
	case riskPattern == "*":
		// Every risk of the track, nothing more to validate.
	case strings.HasPrefix(riskPattern, "!"):
		// Every risk of the track but the excluded one.
		except := strings.TrimPrefix(riskPattern, "!")
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
		risks := strings.Split(riskPattern, ",")
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

// MatchChannelPatterns returns whether the concrete "<track>/<risk>" channel
// matches any of the patterns. An empty list matches every channel, which
// means the patterns carry no restriction.
//
// Channel patterns follow the "<track>/<risk-pattern>" form, where the
// track is a literal and only the risk pattern accepts operators:
//
//   - "*": any risk of that track
//   - "!<risk>": any risk of that track but that one
//   - "<risk>[,...]": only those risks of that track
//
// Patterns are kept as written and interpreted on each match.
//
// A branch, as in "<track>/<risk>/<branch>", is ignored. Branches are ephemeral
// and thus never part of a pattern, so an entry applies to every branch of the
// risk it matches.
func MatchChannelPatterns(patterns []string, channel Channel) bool {
	if len(patterns) == 0 {
		return true
	}
	if channel.Track == "" || channel.Risk == "" {
		// An empty track or risk means the channel is not fully specified,
		// so it matches no pattern. In particular, an empty risk must not
		// match an exclusion such as "!stable", which it otherwise would.
		return false
	}
	for _, pattern := range patterns {
		if matchChannelPattern(pattern, channel) {
			return true
		}
	}
	return false
}

// matchChannelPattern returns whether the pattern matches the given channel.
// Note that the exclusion form is scoped to its own track, so "1.0/!stable"
// does not match any risk of the "2.0" track.
//
// The pattern is expected to be valid.
func matchChannelPattern(pattern string, channel Channel) bool {
	patternTrack, riskPattern, _ := strings.Cut(pattern, "/")
	if channel.Track != patternTrack {
		return false
	}
	if riskPattern == "*" {
		return true
	}
	if except, ok := strings.CutPrefix(riskPattern, "!"); ok {
		return channel.Risk != except
	}
	return slices.Contains(strings.Split(riskPattern, ","), channel.Risk)
}
