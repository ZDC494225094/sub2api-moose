package iqmonitor

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// PostgreSQL types the outputs of INSERT ... SELECT only when they meet the
// target columns, after the WHERE clause has already typed every parameter
// it uses. A parameter shared by both places is deduced twice (text vs
// varchar) and the statement fails; SQLite in the unit tests does not check.
func TestSubmitTopicSQLDoesNotShareSelectParamsWithWhere(t *testing.T) {
	parts := strings.SplitN(submitTopicSQL, "\nWHERE", 2)
	require.Len(t, parts, 2)
	selectList := parts[0][strings.Index(parts[0], "SELECT"):]
	param := regexp.MustCompile(`\$\d+`)
	inWhere := map[string]bool{}
	for _, p := range param.FindAllString(parts[1], -1) {
		inWhere[p] = true
	}
	for _, p := range param.FindAllString(selectList, -1) {
		require.False(t, inWhere[p], "%s is used in the SELECT list and the WHERE clause", p)
	}
}
