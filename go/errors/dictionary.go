package errors

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

var GlobalDictionary = NewDictionary()

type Error struct {
	ErrorCode int
	Data      interface{}
}

func (o Error) String() string {
	d := GlobalDictionary.FindErrorDefinition(o.ErrorCode)
	return fmt.Sprintf("Error code: %v: %s\n%v", o.ErrorCode, *d.Message, o.Data)
}

type ErrorDefinition struct {
	Code    *int    `json:"code"`
	Message *string `json:"message"`
}

type Space struct {
	Name        *string                 `json:"name"`
	Description *string                 `json:"description"`
	Range       [2]int                  `json:"range"`
	Errors      map[int]ErrorDefinition `json:"errors"`
}

type SpaceInfo struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Range       [2]int            `json:"range"`
	Errors      []ErrorDefinition `json:"errors"`
}

type Dictionary struct {
	mux      *sync.Mutex
	spaceMap map[string]*Space
	errorMap map[int]ErrorDefinition
}

func (o *Dictionary) RegisterSpace(name string, description string, from int, to int) {
	o.mux.Lock()
	defer o.mux.Unlock()
	if _, ok := o.spaceMap[name]; ok {
		panic(fmt.Sprintf("space name already registered: %s", name))
	}
	for k, v := range o.spaceMap {
		if from >= v.Range[0] && from <= v.Range[1] {
			panic(fmt.Sprintf("Start of range overlaps with space %s [%d, %d]", k, v.Range[0], v.Range[1]))
		}
		if to >= v.Range[0] && to <= v.Range[1] {
			panic(fmt.Sprintf("End of range overlaps with space %s [%d, %d]", k, v.Range[0], v.Range[1]))
		}
	}
	o.spaceMap[name] = &Space{
		Name:        &name,
		Description: &description,
		Range:       [2]int{from, to},
		Errors:      make(map[int]ErrorDefinition),
	}
}

func (o *Dictionary) RegisterError(spaceName string, code int, message string) {
	o.mux.Lock()
	defer o.mux.Unlock()
	space, ok := o.spaceMap[spaceName]
	if !ok {
		panic(fmt.Sprintf("space name not registered: %s", spaceName))
	}
	if d, ok := space.Errors[code]; ok {
		panic(fmt.Sprintf("error code %d already registered. Current value is: %s", code, *d.Message))
	}
	if code < space.Range[0] || code > space.Range[1] {
		panic(fmt.Sprintf("Error code %d is out of range for space %s [%d, %d]", code, spaceName, space.Range[0], space.Range[1]))
	}
	errorDefinition := ErrorDefinition{Code: &code, Message: &message}
	space.Errors[code] = errorDefinition
	o.errorMap[code] = errorDefinition
}

func (o *Dictionary) Describe(prefix string) []SpaceInfo {
	o.mux.Lock()
	defer o.mux.Unlock()
	result := make([]SpaceInfo, 0)
	for k, v := range o.spaceMap {
		if prefix != "" && !strings.HasPrefix(k, prefix) {
			continue
		}
		spaceErrors := make([]ErrorDefinition, len(v.Errors))
		i := -1
		for _, e := range v.Errors {
			i++
			spaceErrors[i] = e
		}
		sort.Slice(spaceErrors, func(i, j int) bool {
			return *spaceErrors[i].Code < *spaceErrors[j].Code
		})
		info := SpaceInfo{
			Name:        *v.Name,
			Description: *v.Description,
			Range:       v.Range,
			Errors:      spaceErrors,
		}
		result = append(result, info)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Range[0] < result[j].Range[0]
	})
	return result
}

func (o *Dictionary) FindErrorDefinition(code int) *ErrorDefinition {
	e, ok := o.errorMap[code]
	if ok {
		return &e
	} else {
		return nil
	}
}

func NewDictionary() *Dictionary {
	return &Dictionary{
		mux:      &sync.Mutex{},
		spaceMap: make(map[string]*Space),
		errorMap: make(map[int]ErrorDefinition),
	}
}

func ThrowError(code int, data ...interface{}) {
	if len(data) > 0 {
		panic(Error{ErrorCode: code, Data: data[0]})
	} else {
		panic(Error{ErrorCode: code, Data: nil})
	}
}
