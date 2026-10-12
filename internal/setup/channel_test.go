package setup_test

import (
	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/setup"
)

var channelPatternTests = []struct {
	summary string
	values  []string
	err     string
	// match maps a concrete channel to whether the patterns match it.
	match map[setup.Channel]bool
}{{
	summary: "No pattern matches every channel",
	values:  nil,
	match: map[setup.Channel]bool{
		{"3", "stable", ""}: true,
		{"2", "edge", ""}:   true,
	},
}, {
	summary: "Precise channel",
	values:  []string{"3/stable"},
	match: map[setup.Channel]bool{
		{"3", "stable", ""}: true,
		{"3", "edge", ""}:   false,
		{"2", "stable", ""}: false,
		// Branches are ephemeral, hence never part of a pattern.
		{"3", "stable", "mybranch"}: true,
		{"3", "edge", "mybranch"}:   false,
	},
}, {
	summary: "All risks of a track",
	values:  []string{"3/*"},
	match: map[setup.Channel]bool{
		{"3", "stable", ""}: true,
		{"3", "edge", ""}:   true,
		{"2", "stable", ""}: false,
	},
}, {
	summary: "Excluded risk",
	values:  []string{"2/!stable"},
	match: map[setup.Channel]bool{
		{"2", "stable", ""}: false,
		{"2", "edge", ""}:   true,
		{"2", "beta", ""}:   true,
		// The exclusion is scoped to its own track.
		{"3", "edge", ""}: false,
		// The branch is ignored, it is not part of the risk.
		{"2", "stable", "mybranch"}: false,
		{"2", "edge", "mybranch"}:   true,
	},
}, {
	summary: "Channel without a risk never matches",
	values:  []string{"3/*"},
	match: map[setup.Channel]bool{
		{Track: "3"}: false,
		{}:           false,
	},
}, {
	summary: "Several risks",
	values:  []string{"2/beta,edge"},
	match: map[setup.Channel]bool{
		{"2", "beta", ""}:   true,
		{"2", "edge", ""}:   true,
		{"2", "stable", ""}: false,
	},
}, {
	summary: "Union of several tracks",
	values:  []string{"2/!stable", "3/*"},
	match: map[setup.Channel]bool{
		{"2", "edge", ""}:   true,
		{"2", "stable", ""}: false,
		{"3", "stable", ""}: true,
	},
}, {
	summary: "Every known risk",
	values:  []string{"3/stable,candidate,beta,edge"},
	match: map[setup.Channel]bool{
		{"3", "stable", ""}:    true,
		{"3", "candidate", ""}: true,
		{"3", "beta", ""}:      true,
		{"3", "edge", ""}:      true,
	},
}, {
	summary: "Spaces",
	values:  []string{"3/not stable"},
	err:     `"3/not stable": must not contain spaces`,
}, {
	summary: "Missing risk",
	values:  []string{"3"},
	err:     `"3": must be <track>/<risk-pattern>`,
}, {
	summary: "Empty risk",
	values:  []string{"3/"},
	err:     `"3/": must be <track>/<risk-pattern>`,
}, {
	summary: "Empty track",
	values:  []string{"/stable"},
	err:     `"/stable": must be <track>/<risk-pattern>`,
}, {
	summary: "Pattern holding a branch",
	values:  []string{"3/stable/mybranch"},
	err:     `"3/stable/mybranch": must be <track>/<risk-pattern>`,
}, {
	summary: "Wildcard track",
	values:  []string{"*/stable"},
	err:     `"\*/stable": only the risk accepts '\*', '!' and ','`,
}, {
	summary: "Partial wildcard in track",
	values:  []string{"3-*/stable"},
	err:     `"3-\*/stable": only the risk accepts '\*', '!' and ','`,
}, {
	summary: "Wildcard is not a glob",
	values:  []string{"3/e*"},
	err:     `"3/e\*": '\*' must be the whole risk`,
}, {
	summary: "Unknown excluded risk",
	values:  []string{"3/!whatever"},
	err:     `"3/!whatever": unknown risk "whatever", must be one of stable, candidate, beta, edge`,
}, {
	summary: "Exclusion combined with other risks",
	values:  []string{"3/!stable,edge"},
	err:     `"3/!stable,edge": '!' cannot be combined with other risks`,
}, {
	summary: "Empty excluded risk",
	values:  []string{"3/!"},
	err:     `"3/!": must be <track>/<risk-pattern>`,
}, {
	summary: "Unknown risk",
	values:  []string{"3/whatever"},
	err:     `"3/whatever": unknown risk "whatever", must be one of stable, candidate, beta, edge`,
}, {
	summary: "Unknown risk in a list",
	values:  []string{"3/edge,whatever"},
	err:     `"3/edge,whatever": unknown risk "whatever", must be one of stable, candidate, beta, edge`,
}, {
	summary: "Risks are case sensitive",
	values:  []string{"3/Stable"},
	err:     `"3/Stable": unknown risk "Stable", must be one of stable, candidate, beta, edge`,
}, {
	summary: "Exclusion not prefixing the risk part",
	values:  []string{"3/edge,!stable"},
	err:     `"3/edge,!stable": '!' must prefix the whole risk`,
}, {
	summary: "Repeated risk",
	values:  []string{"3/edge,edge"},
	err:     `"3/edge,edge": risk "edge" is repeated`,
}, {
	summary: "Trailing comma in a list",
	values:  []string{"3/edge,"},
	err:     `"3/edge,": must be <track>/<risk-pattern>`,
}, {
	summary: "Leading comma in a list",
	values:  []string{"3/,edge"},
	err:     `"3/,edge": must be <track>/<risk-pattern>`,
}, {
	summary: "Repeated track",
	values:  []string{"3/*", "3/edge"},
	err:     `track "3" is repeated`,
}, {
	summary: "Repeated track with identical values",
	values:  []string{"3/edge", "3/edge"},
	err:     `track "3" is repeated`,
}}

func (s *S) TestChannelPatterns(c *C) {
	for _, test := range channelPatternTests {
		c.Logf("Summary: %s", test.summary)

		err := setup.ValidateChannelPatterns(test.values)
		if test.err != "" {
			c.Assert(err, ErrorMatches, test.err)
			continue
		}
		c.Assert(err, IsNil)

		for channel, expected := range test.match {
			c.Assert(setup.MatchChannelPatterns(test.values, channel), Equals, expected,
				Commentf("channel %q", channel.String()))
		}
	}
}
