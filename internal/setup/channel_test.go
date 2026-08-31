package setup_test

import (
	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/setup"
)

var channelStringTests = []struct {
	summary  string
	channel  setup.Channel
	expected string
}{{
	summary:  "An unset channel renders as empty",
	channel:  setup.Channel{},
	expected: "",
}, {
	summary:  "A track and a risk",
	channel:  setup.Channel{Track: "3.0", Risk: "stable"},
	expected: "3.0/stable",
}, {
	summary:  "A branch is appended",
	channel:  setup.Channel{Track: "3.0", Risk: "edge", Branch: "mybranch"},
	expected: "3.0/edge/mybranch",
}, {
	// A channel is never built without a risk, but rendering the risk as
	// optional would turn the branch into one, that is a different channel.
	summary:  "A missing risk is not skipped over",
	channel:  setup.Channel{Track: "3.0", Branch: "mybranch"},
	expected: "3.0//mybranch",
}, {
	summary:  "A missing track is visible",
	channel:  setup.Channel{Risk: "edge"},
	expected: "/edge",
}}

func (s *S) TestChannelString(c *C) {
	for _, test := range channelStringTests {
		c.Logf("Summary: %s", test.summary)
		c.Assert(test.channel.String(), Equals, test.expected)
	}
}
