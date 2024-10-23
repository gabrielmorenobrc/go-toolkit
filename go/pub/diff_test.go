package pub_test

import (
	"os"
	"sparrowhawktech/toolkit/util"
	"testing"

	"toolkit/pub"
)

type ChildObject1 struct {
	SomeOtherField *string `pub:"oldSomeOtherField"`
	OldChildOnly   string  `pub:"oldChildOnly"`
}

type ChildObject2 struct {
	SomeOtherField *string `pub:"newSomeOtherField"`
	NewChildOnly   string  `pub:"newChildOnly"`
}
type Parent1 struct {
	SomeParentField *string       `pub:"oldSomeParentField"`
	Child           *ChildObject1 `pub:"oldChild"`
	NotChanged      string
}

type Parent2 struct {
	SomeParentField *string       `pub:"newSomeParentField"`
	Child           *ChildObject2 `pub:"newChild"`
	NotChanged      string
}

func TestStructDiff(t *testing.T) {
	root := pub.StructDiff(Parent1{}, Parent2{})
	util.JsonPretty(root, os.Stdout)

	f, err := os.Create("diff.html")
	util.CheckErr(err)
	defer os.Remove("diff.html")
	defer f.Close()
	pub.PrintRootNode(root, f)
}

func TestDescribeDiff(t *testing.T) {
	node1 := pub.Describe(Parent1{})
	util.JsonPretty(node1, os.Stdout)

	node2 := pub.Describe(Parent2{})
	util.JsonPretty(node2, os.Stdout)

	diff := pub.MergeDiffNodes(node1, node2)
	util.JsonPretty(diff, os.Stdout)

	f, err := os.Create("combine.html")
	util.CheckErr(err)
	defer os.Remove("combine.html")
	defer f.Close()
	pub.PrintRootNode(diff, f)
}
