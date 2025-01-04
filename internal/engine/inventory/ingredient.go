package inventory

import (
	"regexp"
	"sort"
	"strconv"
)

const RegexLevels = `([0-9]*[.,]?[0-9]+)\(([0-9]*[.,]?[0-9]+)\)`

func (ing *Ingredient) fillLevels() {
	parts := regexp.MustCompile(RegexLevels).FindAllStringSubmatch(ing.Level, 50)
	ing.levelValue = make([]struct {
		lev int
		val int
	}, len(parts)+1)

	if len(parts) == 0 {
		return
	}

	for i, v := range parts {
		ing.levelValue[i+1].lev = stringToFixInt(v[1])
		ing.levelValue[i+1].val = stringToFixInt(v[2])
	}
	sort.Slice(ing.levelValue, func(i, j int) bool {
		return ing.levelValue[i].lev < ing.levelValue[j].lev
	})
}

func stringToFixInt(s string) int {
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return int(v * 100)
	}
	return 0
}
