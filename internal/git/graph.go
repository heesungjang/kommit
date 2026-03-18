package git

// GraphRow holds the visual graph data for a single commit row.
type GraphRow struct {
	// Cells contains the graph characters for each column.
	// Each cell is a string like "●", "│", "├", "─", "╮", "╰", " ", etc.
	Cells []GraphCell

	// NodeColumn is the column index where this commit's node sits.
	NodeColumn int

	// MaxColumns is the total number of active columns at this row.
	MaxColumns int
}

// GraphCell represents one cell in the graph grid.
type GraphCell struct {
	Char   string
	Column int // which logical branch track this belongs to (for coloring)
}

// GraphState holds the lane-tracking state between incremental graph passes.
// Pass it to ComputeGraphIncremental to resume from a previous computation.
type GraphState struct {
	Active []string // commit hashes expected in each column
}

// ComputeGraph computes an ASCII commit graph for the given commits.
// Commits must be in topological order (newest first), which is the
// default git log order.
func ComputeGraph(commits []CommitInfo) []GraphRow {
	rows, _ := ComputeGraphIncremental(commits, nil)
	return rows
}

// ComputeGraphIncremental computes graph rows and returns the lane state so
// additional commits can be appended without reprocessing earlier rows.
// If prev is nil the computation starts fresh.
func ComputeGraphIncremental(commits []CommitInfo, prev *GraphState) ([]GraphRow, *GraphState) {
	if len(commits) == 0 {
		return nil, prev
	}

	rows := make([]GraphRow, len(commits))

	// active tracks which commit hashes are expected in each column.
	// A column is "free" when its value is empty.
	var active []string
	if prev != nil {
		active = make([]string, len(prev.Active))
		copy(active, prev.Active)
	}

	for i, c := range commits {
		if c.Hash == "" {
			// Synthetic entry (WIP) — render as a standalone node in column 0
			row := GraphRow{NodeColumn: 0, MaxColumns: len(active)}
			if len(active) == 0 {
				row.Cells = []GraphCell{{Char: "●", Column: 0}}
				row.MaxColumns = 1
			} else {
				row.Cells = make([]GraphCell, len(active))
				row.Cells[0] = GraphCell{Char: "●", Column: 0}
				for j := 1; j < len(active); j++ {
					row.Cells[j] = GraphCell{Char: "│", Column: j}
				}
			}
			rows[i] = row
			continue
		}

		// Find which column this commit occupies.
		col := -1
		for j, hash := range active {
			if hash == c.Hash {
				col = j
				break
			}
		}

		if col == -1 {
			// This commit wasn't expected — allocate a new column.
			col = findFreeColumn(active)
			if col == len(active) {
				active = append(active, c.Hash)
			} else {
				active[col] = c.Hash
			}
		}

		// Build the row cells.
		cells := make([]GraphCell, len(active))
		for j := range cells {
			if j == col {
				cells[j] = GraphCell{Char: "●", Column: col}
			} else if active[j] != "" {
				cells[j] = GraphCell{Char: "│", Column: j}
			} else {
				cells[j] = GraphCell{Char: " ", Column: j}
			}
		}

		// Process parents.
		parents := c.Parents
		if len(parents) == 0 {
			// Root commit — this column becomes free.
			active[col] = ""
		} else {
			// First parent continues in the same column.
			active[col] = parents[0]

			// Additional parents (merge commits) — find or allocate columns.
			for p := 1; p < len(parents); p++ {
				parentHash := parents[p]
				// Check if this parent is already tracked in another column.
				existingCol := -1
				for j, hash := range active {
					if hash == parentHash {
						existingCol = j
						break
					}
				}
				if existingCol == -1 {
					// Allocate a new column for this parent.
					newCol := findFreeColumn(active)
					if newCol == len(active) {
						active = append(active, parentHash)
						// Extend cells to match
						cells = append(cells, GraphCell{Char: "╮", Column: newCol})
					} else {
						active[newCol] = parentHash
						// Add merge indicator
						for len(cells) <= newCol {
							cells = append(cells, GraphCell{Char: " ", Column: len(cells)})
						}
						cells[newCol] = GraphCell{Char: "╮", Column: newCol}
					}
					// Draw horizontal connectors between the node and the merge column
					for k := col + 1; k < newCol; k++ {
						switch cells[k].Char {
						case " ":
							cells[k] = GraphCell{Char: "─", Column: newCol}
						case "│":
							cells[k] = GraphCell{Char: "┼", Column: cells[k].Column}
						}
					}
				} else if existingCol != col {
					// Parent already tracked — draw merge line
					minC, maxC := col, existingCol
					if minC > maxC {
						minC, maxC = maxC, minC
					}
					for k := minC + 1; k < maxC; k++ {
						switch cells[k].Char {
						case " ":
							cells[k] = GraphCell{Char: "─", Column: existingCol}
						case "│":
							cells[k] = GraphCell{Char: "┼", Column: cells[k].Column}
						}
					}
					if existingCol > col {
						cells[existingCol] = GraphCell{Char: "╮", Column: existingCol}
					} else {
						cells[existingCol] = GraphCell{Char: "╯", Column: existingCol}
					}
				}
			}
		}

		// Trim trailing empty columns for display.
		maxUsed := 0
		for j := range cells {
			if cells[j].Char != " " {
				maxUsed = j + 1
			}
		}
		// But keep at least up to the active columns that have content.
		for j, hash := range active {
			if hash != "" && j+1 > maxUsed {
				maxUsed = j + 1
			}
		}

		rows[i] = GraphRow{
			Cells:      cells[:maxUsed],
			NodeColumn: col,
			MaxColumns: len(active),
		}

		// Clean up: compact trailing empty columns from active.
		for len(active) > 0 && active[len(active)-1] == "" {
			active = active[:len(active)-1]
		}
	}

	return rows, &GraphState{Active: active}
}

// findFreeColumn returns the first free (empty) column index, or len(active)
// if no free column exists.
func findFreeColumn(active []string) int {
	for i, hash := range active {
		if hash == "" {
			return i
		}
	}
	return len(active)
}
