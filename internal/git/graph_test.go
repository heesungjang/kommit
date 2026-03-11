package git

import (
	"testing"
)

func TestComputeGraph(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		rows := ComputeGraph(nil)
		if rows != nil {
			t.Errorf("expected nil, got %v", rows)
		}
	})

	t.Run("single commit", func(t *testing.T) {
		commits := []CommitInfo{
			{Hash: "aaa", Parents: nil},
		}
		rows := ComputeGraph(commits)
		if len(rows) != 1 {
			t.Fatalf("expected 1 row, got %d", len(rows))
		}
		if rows[0].NodeColumn != 0 {
			t.Errorf("NodeColumn = %d, want 0", rows[0].NodeColumn)
		}
		// The node should be in the cells
		found := false
		for _, cell := range rows[0].Cells {
			if cell.Char == "●" {
				found = true
			}
		}
		if !found {
			t.Error("expected to find ● in cells")
		}
	})

	t.Run("linear history", func(t *testing.T) {
		commits := []CommitInfo{
			{Hash: "ccc", Parents: []string{"bbb"}},
			{Hash: "bbb", Parents: []string{"aaa"}},
			{Hash: "aaa", Parents: nil},
		}
		rows := ComputeGraph(commits)
		if len(rows) != 3 {
			t.Fatalf("expected 3 rows, got %d", len(rows))
		}
		for i, row := range rows {
			if row.NodeColumn != 0 {
				t.Errorf("row %d: NodeColumn = %d, want 0", i, row.NodeColumn)
			}
			// Each row should have the node indicator
			if len(row.Cells) == 0 {
				t.Errorf("row %d: no cells", i)
				continue
			}
			if row.Cells[0].Char != "●" {
				t.Errorf("row %d: cell[0] = %q, want ●", i, row.Cells[0].Char)
			}
		}
	})

	t.Run("root commit no parents", func(t *testing.T) {
		commits := []CommitInfo{
			{Hash: "root", Parents: nil},
		}
		rows := ComputeGraph(commits)
		if len(rows) != 1 {
			t.Fatalf("expected 1 row, got %d", len(rows))
		}
		if rows[0].NodeColumn != 0 {
			t.Errorf("NodeColumn = %d, want 0", rows[0].NodeColumn)
		}
	})

	t.Run("merge commit", func(t *testing.T) {
		// commit C merges A and B
		commits := []CommitInfo{
			{Hash: "C", Parents: []string{"A", "B"}},
			{Hash: "A", Parents: nil},
			{Hash: "B", Parents: nil},
		}
		rows := ComputeGraph(commits)
		if len(rows) != 3 {
			t.Fatalf("expected 3 rows, got %d", len(rows))
		}
		// The merge commit should be in column 0
		if rows[0].NodeColumn != 0 {
			t.Errorf("merge commit NodeColumn = %d, want 0", rows[0].NodeColumn)
		}
		// The merge should spawn a second column for the second parent
		if rows[0].MaxColumns < 2 {
			t.Errorf("merge commit MaxColumns = %d, want at least 2", rows[0].MaxColumns)
		}
	})
}

func TestFindFreeColumn(t *testing.T) {
	t.Run("empty slice", func(t *testing.T) {
		got := findFreeColumn(nil)
		if got != 0 {
			t.Errorf("findFreeColumn(nil) = %d, want 0", got)
		}
	})

	t.Run("first slot free", func(t *testing.T) {
		active := []string{"", "hash1", "hash2"}
		got := findFreeColumn(active)
		if got != 0 {
			t.Errorf("findFreeColumn = %d, want 0", got)
		}
	})

	t.Run("middle slot free", func(t *testing.T) {
		active := []string{"hash1", "", "hash2"}
		got := findFreeColumn(active)
		if got != 1 {
			t.Errorf("findFreeColumn = %d, want 1", got)
		}
	})

	t.Run("all occupied", func(t *testing.T) {
		active := []string{"hash1", "hash2", "hash3"}
		got := findFreeColumn(active)
		if got != 3 {
			t.Errorf("findFreeColumn = %d, want 3 (len)", got)
		}
	})
}
