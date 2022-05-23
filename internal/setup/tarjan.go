// This file was copied from mgo, MongoDB driver for Go.
//
// Copyright (c) 2010-2013 - Gustavo Niemeyer <gustavo@niemeyer.net>
//
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:
//
// 1. Redistributions of source code must retain the above copyright notice, this
//    list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright notice,
//    this list of conditions and the following disclaimer in the documentation
//    and/or other materials provided with the distribution.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
// ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
// WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
// DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER OR CONTRIBUTORS BE LIABLE FOR
// ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
// (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
// LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND
// ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
// SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package setup

import (
	"sort"
)

func tarjanSort(successors map[string][]string) [][]string {
	// http://en.wikipedia.org/wiki/Tarjan%27s_strongly_connected_components_algorithm
	data := &tarjanData{
		successors: successors,
		nodes:      make([]tarjanNode, 0, len(successors)),
		index:      make(map[string]int, len(successors)),
	}

	// Stabilize iteration through successors map to prevent
	// disjointed components producing unstable output due to
	// golang map randomized iteration.
	stableIDs := make([]string, 0, len(successors))
	for id := range successors {
		stableIDs = append(stableIDs, id)
	}
	sort.Strings(stableIDs)
	for _, id := range stableIDs {
		if _, seen := data.index[id]; !seen {
			data.strongConnect(id)
		}
	}

	// Sort connected components to stabilize the algorithm.
	for _, ids := range data.output {
		if len(ids) > 1 {
			sort.Sort(idList(ids))
		}
	}
	return data.output
}

type tarjanData struct {
	successors map[string][]string
	output     [][]string

	nodes []tarjanNode
	stack []string
	index map[string]int
}

type tarjanNode struct {
	lowlink int
	stacked bool
}

type idList []string

func (l idList) Len() int           { return len(l) }
func (l idList) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }
func (l idList) Less(i, j int) bool { return l[i] < l[j] }

func (data *tarjanData) strongConnect(id string) *tarjanNode {
	index := len(data.nodes)
	data.index[id] = index
	data.stack = append(data.stack, id)
	data.nodes = append(data.nodes, tarjanNode{index, true})
	node := &data.nodes[index]

	for _, succid := range data.successors[id] {
		succindex, seen := data.index[succid]
		if !seen {
			succnode := data.strongConnect(succid)
			if succnode.lowlink < node.lowlink {
				node.lowlink = succnode.lowlink
			}
		} else if data.nodes[succindex].stacked {
			// Part of the current strongly-connected component.
			if succindex < node.lowlink {
				node.lowlink = succindex
			}
		}
	}

	if node.lowlink == index {
		// Root node; pop stack and output new
		// strongly-connected component.
		var scc []string
		i := len(data.stack) - 1
		for {
			stackid := data.stack[i]
			stackindex := data.index[stackid]
			data.nodes[stackindex].stacked = false
			scc = append(scc, stackid)
			if stackindex == index {
				break
			}
			i--
		}
		data.stack = data.stack[:i]
		data.output = append(data.output, scc)
	}

	return node
}
