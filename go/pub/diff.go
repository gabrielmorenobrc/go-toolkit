package pub

import (
	"fmt"
	"io"
	"reflect"
	"sparrowhawktech/toolkit/util"
)

type DiffNode struct {
	KeyName       *string     `json:"keyName"`
	PublicName    *string     `json:"publicName,omitempty"`
	NewPublicName *string     `json:"newPublicName,omitempty""`
	Children      []*DiffNode `json:"children,omitempty"`
}

func StructDiff(a any, b any) DiffNode {
	t1 := reflect.TypeOf(a)
	rootName1 := t1.Name()
	t2 := reflect.TypeOf(b)
	rootName2 := t2.Name()
	keyName := t1.Name()
	root := &DiffNode{
		KeyName:       &keyName,
		PublicName:    &rootName1,
		NewPublicName: &rootName2,
		Children:      nil,
	}
	nodeMap := make(map[string]*DiffNode)
	processOld(t1, "root", root, nodeMap)
	processNew(t2, "root", root, nodeMap)
	return *root
}

func processOld(t1 reflect.Type, parentKey string, thisNode *DiffNode, nodeMap map[string]*DiffNode) {
	for i := 0; i < t1.NumField(); i++ {
		ft := t1.Field(i)
		if !ft.IsExported() {
			continue
		}
		key := fmt.Sprintf("%s.%s", parentKey, ft.Name)
		t := ft.Type
		if t.Kind() == reflect.Map {
			t = t.Elem()
		} else if t.Kind() == reflect.Slice {
			t = t.Elem()
		} else if t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		label1 := ResolveLabel(ft)
		oldName := label1
		child := &DiffNode{
			KeyName:    &ft.Name,
			PublicName: &oldName,
		}
		if util.IsStruct(t) {
			processOld(t, key, child, nodeMap)
		}
		nodeMap[key] = child
		thisNode.Children = append(thisNode.Children, child)

	}
}

func processNew(t2 reflect.Type, parentKey string, thisNode *DiffNode, nodeMap map[string]*DiffNode) {
	println(t2.String())
	for i := 0; i < t2.NumField(); i++ {
		ft := t2.Field(i)
		if !ft.IsExported() {
			continue
		}
		key := fmt.Sprintf("%s.%s", parentKey, ft.Name)
		t := ft.Type
		if t.Kind() == reflect.Map {
			t = t.Elem()
		} else if t.Kind() == reflect.Slice {
			t = t.Elem()
		} else if t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		label2 := ResolveLabel(ft)
		newName := label2
		println(key, newName)

		child, ok := nodeMap[key]
		if !ok {
			child = &DiffNode{
				KeyName: &ft.Name,
			}
			thisNode.Children = append(thisNode.Children, child)
			nodeMap[key] = child
		}
		child.NewPublicName = &newName
		if util.IsStruct(t) {
			processNew(t, key, child, nodeMap)
		}

	}
}

func PrintRootNode(rootNode DiffNode, w io.Writer) {
	util.WriteString("<html>\r\n<body><table>\r\n", w)
	util.WriteString("<thead><tr><th style='width: 60%;'>Location</th><th style='width: 20%;'>Old name</th><th style='width: 20%;'>New name</th></tr></thead>\r\n", w)
	util.WriteString("<tbody>\r\n", w)
	for _, ch := range rootNode.Children {
		printNode(nil, *ch, w)
	}
	util.WriteString("</tbody></table></body>\r\n</html>", w)
	util.WriteString("</tbody>\r\n</table></body>\r\n</html>", w)

}
func printNode(parentPath *string, node DiffNode, w io.Writer) {

	path := *node.KeyName
	if node.PublicName != nil {
		path = *node.PublicName
	} else if node.NewPublicName != nil {
		path = *node.NewPublicName
	}
	if parentPath != nil {
		path = fmt.Sprintf("%s.%s", *parentPath, path)
	}
	util.WriteString("<tr>", w)
	util.WriteString("<td style='width: 60%;'>", w)
	util.WriteString(path, w)
	util.WriteString("</td>", w)

	color := "black"
	if node.NewPublicName == nil {
		color = "red"
	} else if node.NewPublicName == nil || (node.PublicName != nil && *node.PublicName != *node.NewPublicName) {
		color = "blue"
	}
	util.WriteString("<td style='width: 20%; color: ", w)
	util.WriteString(color, w)
	util.WriteString(";'>", w)

	if node.PublicName != nil {
		util.WriteString(*node.PublicName, w)
	}

	color = "black"
	if node.PublicName == nil {
		color = "green"
	} else if node.NewPublicName == nil || (node.PublicName != nil && *node.PublicName != *node.NewPublicName) {
		color = "red"
	}
	util.WriteString("</td>", w)
	util.WriteString("<td style='width: 20%; color: ", w)
	util.WriteString(color, w)
	util.WriteString(";'>", w)
	if node.NewPublicName != nil {
		util.WriteString(*node.NewPublicName, w)
	}
	util.WriteString("</td></tr>\r\n", w)

	for _, ch := range node.Children {
		printNode(&path, *ch, w)
	}

}

func MergeDiffNodes(node1 DiffNode, node2 DiffNode) DiffNode {
	m := make(map[string]*DiffNode)
	node2.KeyName = node1.KeyName
	putNode1(&node1, nil, m)
	return *mergeNode2(node2, nil, m)
}

func mergeNode2(node2 DiffNode, parentKey *string, m map[string]*DiffNode) *DiffNode {
	key := *node2.KeyName
	if parentKey != nil {
		key = fmt.Sprintf("%s.%s", *parentKey, key)
	}
	n, ok := m[key]
	if !ok {
		n = &node2
		n.NewPublicName = n.PublicName
		n.PublicName = nil
		m[key] = n
	} else {
		n.NewPublicName = node2.PublicName
	}
	for _, ch := range node2.Children {
		mergeNode2(*ch, &key, m)
	}
	return n
}

func putNode1(node1 *DiffNode, parentKey *string, m map[string]*DiffNode) {
	key := *node1.KeyName
	if parentKey != nil {
		key = fmt.Sprintf("%s.%s", *parentKey, key)
	}
	m[key] = node1
	for _, ch := range node1.Children {
		putNode1(ch, &key, m)
	}
}
