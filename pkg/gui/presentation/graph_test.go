package presentation

import (
	"strings"
	"testing"

	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/stretchr/testify/assert"
)

func TestRenderLine(t *testing.T) {
	tests := []struct {
		commit        *models.Commit
		paths         []Path
		expectedLine  string
		expectedPaths []Path
	}{
		{
			commit: &models.Commit{Sha: "b", Parents: []string{"c"}},
			paths: []Path{
				{from: "a", to: "b", prevPos: 0},
			},
			expectedLine: "o ",
			expectedPaths: []Path{
				{from: "b", to: "c", prevPos: 0},
			},
		},
		{
			commit: &models.Commit{Sha: "b", Parents: []string{"c"}},
			paths: []Path{
				{from: "a", to: "b", prevPos: 0},
				{from: "1", to: "2", prevPos: 1},
			},
			expectedLine: "o │ ",
			expectedPaths: []Path{
				{from: "b", to: "c", prevPos: 0},
				{from: "1", to: "2", prevPos: 1},
			},
		},
		{
			commit: &models.Commit{Sha: "b", Parents: []string{"c"}},
			paths: []Path{
				{from: "a", to: "b", prevPos: 0},
				{from: "1", to: "b", prevPos: 1},
			},
			expectedLine: "o─┘ ",
			expectedPaths: []Path{
				{from: "b", to: "c", prevPos: 0},
			},
		},
		{
			commit: &models.Commit{Sha: "b", Parents: []string{"c"}},
			paths: []Path{
				{from: "a", to: "b", prevPos: 0},
				{from: "1", to: "b", prevPos: 1},
				{from: "2", to: "b", prevPos: 2},
			},
			expectedLine: "o─┴─┘ ",
			expectedPaths: []Path{
				{from: "b", to: "c", prevPos: 0},
			},
		},
		{
			commit: &models.Commit{Sha: "b", Parents: []string{"c", "d"}},
			paths: []Path{
				{from: "a", to: "b", prevPos: 0},
			},
			expectedLine: "M─┐ ",
			expectedPaths: []Path{
				{from: "b", to: "c", prevPos: 0},
				{from: "b", to: "d", prevPos: 1},
			},
		},
		{
			commit: &models.Commit{Sha: "b", Parents: []string{"c", "d"}},
			paths: []Path{
				{from: "a", to: "b", prevPos: 0},
				{from: "1", to: "2", prevPos: 1},
			},
			expectedLine: "M─┼─┐ ",
			expectedPaths: []Path{
				{from: "b", to: "c", prevPos: 0},
				{from: "1", to: "2", prevPos: 1},
				{from: "b", to: "d", prevPos: 2},
			},
		},
		{
			commit: &models.Commit{Sha: "b", Parents: []string{"c"}},
			paths: []Path{
				{from: "a", to: "b", prevPos: 0},
				{from: "1", to: "b", prevPos: 1},
				{from: "2", to: "3", prevPos: 2},
			},
			expectedLine: "o─┘ │ ",
			expectedPaths: []Path{
				{from: "b", to: "c", prevPos: 0},
				{from: "2", to: "3", prevPos: 2},
			},
		},
		{
			commit: &models.Commit{Sha: "c", Parents: []string{"d"}},
			paths: []Path{
				{from: "b", to: "c", prevPos: 0},
				{from: "2", to: "3", prevPos: 2},
			},
			expectedLine: "o ┌─┘ ",
			expectedPaths: []Path{
				{from: "c", to: "d", prevPos: 0},
				{from: "2", to: "3", prevPos: 1},
			},
		},
		{
			commit: &models.Commit{Sha: "c", Parents: []string{"d"}},
			paths: []Path{
				{from: "b", to: "c", prevPos: 0},
				{from: "2", to: "3", prevPos: 3},
			},
			expectedLine: "o ┌───┘ ",
			expectedPaths: []Path{
				{from: "c", to: "d", prevPos: 0},
				{from: "2", to: "3", prevPos: 1},
			},
		},
		{
			commit: &models.Commit{Sha: "c", Parents: []string{"d"}},
			paths: []Path{
				{from: "b", to: "c", prevPos: 0},
				{from: "2", to: "3", prevPos: 2},
				{from: "4", to: "5", prevPos: 3},
			},
			expectedLine: "o ┌─┼─┘ ",
			expectedPaths: []Path{
				{from: "c", to: "d", prevPos: 0},
				{from: "2", to: "3", prevPos: 1},
				{from: "4", to: "5", prevPos: 2},
			},
		},
		{
			commit: &models.Commit{Sha: "c", Parents: []string{"d", "e"}},
			paths: []Path{
				{from: "b", to: "c", prevPos: 0},
				{from: "2", to: "c", prevPos: 1},
				{from: "4", to: "c", prevPos: 2},
			},
			expectedLine: "M─┼─┘ ",
			expectedPaths: []Path{
				{from: "c", to: "d", prevPos: 0},
				{from: "c", to: "e", prevPos: 1},
			},
		},
	}

	for _, test := range tests {
		line, paths := renderLine(test.commit, test.paths)
		if line != test.expectedLine {
			t.Errorf("expected line to be %s, got %s", test.expectedLine, line)
		}
		if len(paths) != len(test.expectedPaths) {
			t.Errorf("expected paths to be %v, got %v", test.expectedPaths, paths)
			continue
		}
		for i, path := range paths {
			if path.from != test.expectedPaths[i].from {
				t.Errorf("expected from to be %s, got %s", test.expectedPaths[i].from, path.from)
			}
			if path.to != test.expectedPaths[i].to {
				t.Errorf("expected to to be %s, got %s", test.expectedPaths[i].to, path.to)
			}
		}
	}
}

func TestRenderGraph(t *testing.T) {
	commits := []*models.Commit{
		{Sha: "1", Parents: []string{"2"}},
		{Sha: "2", Parents: []string{"3"}},
		{Sha: "3", Parents: []string{"4"}},
		{Sha: "4", Parents: []string{"5", "7"}},
		{Sha: "7", Parents: []string{"5"}},
		{Sha: "5", Parents: []string{"8"}},
		{Sha: "8", Parents: []string{"9"}},
		{Sha: "9", Parents: []string{"10", "11"}},
		{Sha: "11", Parents: []string{"12"}},
		{Sha: "12", Parents: []string{"15"}},
		{Sha: "10", Parents: []string{"13"}},
		{Sha: "13", Parents: []string{"14"}},
		{Sha: "14", Parents: []string{"15"}},
		{Sha: "15", Parents: []string{"16"}},
	}

	t.Log("\n" + strings.Join(renderCommitGraph(commits), "\n"))

	assert.Equal(t,
		`o 
o 
o 
M─┐ 
│ o 
o─┘ 
o 
M─┐ 
│ o 
│ o 
o │ 
o │ 
o │ 
o─┘ `,
		strings.Join(renderCommitGraph(commits), "\n"))
}
