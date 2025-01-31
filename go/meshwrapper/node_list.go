package meshwrapper

import (
	"cmp"
	"regexp"
	"slices"
	"strconv"
)

type nodeList struct {
	nodes map[uint32]*Node
}

var Broadcast = Node{
	Id:        0xFFFFFFFF,
	ShortName: "CAST",
	LongName:  "Everyone",
}

var Unknown = Node{
	Id:        0x00000000,
	ShortName: "UNKN",
	LongName:  "Unknown",
}

func NewNodeList() nodeList {
	list := nodeList{
		nodes: make(map[uint32]*Node),
	}

	list.nodes[Broadcast.Id] = &Broadcast
	list.nodes[Unknown.Id] = &Unknown

	return list
}

func (n *nodeList) String() string {
	nodes := ""
	for _, node := range n.sortedNodes() {
		if node.Id != Broadcast.Id && node.Id != Unknown.Id {
			nodes += node.VerboseString() + "\n"
		}
	}
	return nodes
}

func (n *nodeList) sortedNodes() []Node {
	nodes := make([]Node, 0, len(n.nodes))
	for _, node := range n.nodes {
		nodes = append(nodes, *node)
	}
	slices.SortFunc(nodes, func(a, b Node) int {
		return cmp.Or(
			cmp.Compare(a.HopsAway, b.HopsAway),
			-cmp.Compare(a.LastHeard.Unix(), b.LastHeard.Unix()),
		)
	})
	return nodes
}

func (n *nodeList) findNode(needle string) *Node {
	needleBytes := []byte(needle)

	// Check if we have a specific, full hexadecimal id
	fullHexId, _ := regexp.Compile("![0-9a-fA-F]{8}")
	if fullHexId.Match(needleBytes) {
		id, _ := strconv.ParseUint(needle[1:], 16, 32)
		node, ok := n.nodes[uint32(id)]
		if ok {
			return node
		}
	}
	shortHexId, _ := regexp.Compile("[0-9a-fA-F]{8}")
	if shortHexId.Match(needleBytes) {
		id, _ := strconv.ParseUint(needle, 16, 32)
		node, ok := n.nodes[uint32(id)]
		if ok {
			return node
		}
	}

	// Check if we have a shortName
	for _, node := range n.nodes {
		if node.ShortName == needle {
			return node
		}
	}

	// Check if we have a decimal id
	numericId, _ := regexp.Compile("[0-9]+")
	if numericId.Match(needleBytes) {
		id, _ := strconv.ParseUint(needle, 10, 32)
		node, ok := n.nodes[uint32(id)]
		if ok {
			return node
		}
	}

	// Check if we have an abbreviated hexadecimal id
	abbreviatedHexId, _ := regexp.Compile("[0-9a-fA-F]{4}")
	if abbreviatedHexId.Match(needleBytes) {
		id, _ := strconv.ParseUint(needle, 16, 32)
		node, ok := n.nodes[uint32(id)]
		if ok {
			return node
		}
	}

	return nil
}
