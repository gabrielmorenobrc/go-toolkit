package coverage

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"sort"
	"sparrowhawktech/toolkit/util"
	"strconv"
	"strings"
)

type Node struct {
	Name         string  `json:"name"`
	Lines        int     `json:"-"`
	CoverLine    int     `json:"-"`
	CoverPercent float64 `json:"coverage"`
	Nodes        []Node  `json:"nodes"`
}

type preprocessAcumulator struct {
	Lines       int
	CovertLines int
}

func getDirectoriesMap(array []string) map[string][]string {
	result := make(map[string][]string, 0)
	for j := 0; j < len(array); j++ {
		directories := strings.Split(array[j], "/")
		if len(directories) > 1 {
			if _, exist := result[directories[0]]; !exist {
				result[directories[0]] = make([]string, 1)
			}
			result[directories[0]] = append(result[directories[0]], array[j])
		}
	}
	return result
}

func preProcessAcumulation(files []string) map[string]preprocessAcumulator {
	accumulator := make(map[string]preprocessAcumulator)
	for i := 0; i < len(files); i++ {
		record := strings.Split(files[i], ":")
		fileName := strings.TrimSpace(record[0])
		if len(fileName) > 0 {
			temp := strings.Split(record[1], " ")
			codeLine := temp[0]
			lines, _ := strconv.Atoi(temp[1])
			manyTimeExcuted, _ := strconv.Atoi(temp[2])
			if _, exist := accumulator[fileName+":"+codeLine]; !exist {
				u := new(preprocessAcumulator)
				u.Lines = lines
				accumulator[fileName+":"+codeLine] = *u
			}

			if manyTimeExcuted > 0 {
				var u = accumulator[fileName+":"+codeLine]
				u.CovertLines = lines
				accumulator[fileName+":"+codeLine] = u
			}
		}
	}
	return accumulator
}

func processFiles(array []string) map[string]Node {
	result := make(map[string]Node, 0)
	files := make([]string, 0)
	for j := 0; j < len(array); j++ {
		if strings.Contains(array[j], "/") == false {
			files = append(files, array[j])
		}
	}

	sort.Strings(files)
	accumulator := preProcessAcumulation(files)
	for k, v := range accumulator {
		record := strings.Split(k, ":")
		fileName := strings.TrimSpace(record[0])
		if _, exist := result[fileName]; !exist {
			node := new(Node)
			node.Name = fileName
			result[fileName] = *node
		}

		var n1 = result[fileName]
		n1.Lines += v.Lines
		n1.CoverLine += v.CovertLines
		n1.CoverPercent = round(float64(n1.CoverLine)/float64(n1.Lines)*100, 2)
		result[fileName] = n1
	}
	return result
}

func removeDirectoryNameFromPath(array []string, directory string) []string {
	for i := 0; i < len(array); i++ {
		array[i] = strings.ReplaceAll(array[i], directory+"/", "")
	}
	return array
}

func round(num float64, nbDigits float64) float64 {
	pow := math.Pow(10., nbDigits)
	rounded := float64(int(num*pow)) / pow
	return rounded
}

func parseRecursive(array []string, currentDirectory string) *Node {
	node := new(Node)
	node.Name = currentDirectory
	node.Nodes = make([]Node, 0)
	array = removeDirectoryNameFromPath(array, currentDirectory)
	dirs := getDirectoriesMap(array)
	for k, v := range dirs {
		childnode := parseRecursive(v, k)
		node.Nodes = append(node.Nodes, *childnode)
		node.Lines += childnode.Lines
		node.CoverLine += childnode.CoverLine
	}
	files := processFiles(array)
	for _, v := range files {
		node.Nodes = append(node.Nodes, v)
		node.Lines += v.Lines
		node.CoverLine += v.CoverLine
	}
	node.CoverPercent = round(float64(node.CoverLine)/float64(node.Lines)*100, 2)

	return node
}

func ParseOutput(databinary []byte, startFrom string, w io.Writer, json bool) {
	data := strings.ReplaceAll(string(databinary), "mode: atomic\n", "")
	idx := strings.Index(data, startFrom+"/")
	removestr := data[:idx]
	data = strings.ReplaceAll(data, removestr, "")
	node := parseRecursive(strings.Split(data, "\n"), "go")
	if json {
		tree := make(map[string]interface{})
		tree = printMap(tree, *node, 0)
		util.JsonPretty(tree, w)
	} else {
		printOut(bufio.NewWriter(w), *node, 0)
	}
}

func printMap(tree map[string]interface{}, node Node, level int) map[string]interface{} {
	if len(node.Nodes) > 0 {
		children := make([]map[string]interface{}, 0)
		for _, v := range node.Nodes {
			child := make(map[string]interface{})
			child = printMap(child, v, level+1)
			children = append(children, child)
		}
		tab := level * 4
		space := strings.Repeat(" ", 100-tab-len(node.Name))
		name := fmt.Sprintf("%s%s%00.2f", node.Name, space, node.CoverPercent)
		tree[name] = children
	} else {
		tree[node.Name] = fmt.Sprintf("%00.2f", node.CoverPercent)
	}

	return tree
}

func printOut(w *bufio.Writer, node Node, level int) {
	prefix := strings.Repeat("│   ", level)
	prefix += "├── "
	space := strings.Repeat(" ", 100-level*4-len(node.Name))
	w.WriteString(prefix)
	w.WriteString(node.Name)
	w.WriteString(space)
	w.WriteString(fmt.Sprintf("%00.2f", node.CoverPercent))
	w.WriteString("\n")
	sort.Slice(node.Nodes, func(i, j int) bool {
		return strings.Compare(node.Nodes[i].Name, node.Nodes[j].Name) < 0
	})
	for _, v := range node.Nodes {
		printOut(w, v, level+1)
	}
}
