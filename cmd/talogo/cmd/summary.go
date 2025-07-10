package cmd

import (
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	summaryCmdLogFile string
)

// TaskNode represents a node in the task hierarchy
type TaskNode struct {
	Name      string
	Duration  time.Duration
	Children  map[string]*TaskNode
	TotalTime time.Duration // Includes children
}

// summaryCmd defines the summary subcommand
var summaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Generate a report of total hours spent per task and subtasks per day",
	Run: func(cmd *cobra.Command, args []string) {
		if err := generateSummary(summaryCmdLogFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating summary: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	summaryCmd.Flags().StringVarP(&summaryCmdLogFile, "file", "f", "./talogo.csv", "Log file to read")
	rootCmd.AddCommand(summaryCmd)
}

// generateSummary reads the CSV and prints the daily task summary
func generateSummary(logFile string) error {
	file, err := os.Open(logFile)
	if err != nil {
		return fmt.Errorf("failed to open CSV file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.LazyQuotes = true       // Allow relaxed quoting
	reader.FieldsPerRecord = -1    // Allow variable number of fields
	reader.TrimLeadingSpace = true // Trim leading spaces

	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read CSV: %v", err)
	}

	if len(records) <= 1 {
		fmt.Println("No data in CSV file (only header or empty)")
		return nil
	}

	// Group records by day
	dailyTasks := make(map[string]map[string]*TaskNode) // date -> root task -> hierarchy
	for i, record := range records {
		if i == 0 {
			continue // Skip header row
		}

		// Ensure record has at least start_time, end_time, duration_seconds
		if len(record) < 3 {
			fmt.Fprintf(os.Stderr, "Skipping malformed record on line %d: too few fields (%d)\n", i+1, len(record))
			continue
		}

		// Parse start time
		startTime, err := time.Parse(time.RFC3339, record[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Skipping record on line %d: invalid start time (%s)\n", i+1, record[0])
			continue
		}

		// Parse duration
		durationSeconds, err := strconv.Atoi(record[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Skipping record on line %d: invalid duration (%s)\n", i+1, record[2])
			continue
		}
		duration := time.Duration(durationSeconds) * time.Second

		// Get date in YYYY-MM-DD format
		dateStr := startTime.Format("2006-01-02")

		// Initialize daily task map
		if _, exists := dailyTasks[dateStr]; !exists {
			dailyTasks[dateStr] = make(map[string]*TaskNode)
		}

		// Build task hierarchy
		current := dailyTasks[dateStr]
		for j := 3; j < len(record); j++ {
			if record[j] == "" {
				break // No more titles
			}
			taskName := record[j]
			if taskName == "" {
				continue // Skip empty task names
			}
			if _, exists := current[taskName]; !exists {
				current[taskName] = &TaskNode{
					Name:     taskName,
					Children: make(map[string]*TaskNode),
				}
			}
			current[taskName].Duration += duration
			current[taskName].TotalTime += duration
			current = current[taskName].Children
		}
	}

	// Sort dates
	var dates []string
	for date := range dailyTasks {
		dates = append(dates, date)
	}
	sort.Strings(dates)

	// Print report
	for _, date := range dates {
		fmt.Printf("Date: %s\n", date)
		tasks := dailyTasks[date]
		var taskNames []string
		for name := range tasks {
			taskNames = append(taskNames, name)
		}
		sort.Strings(taskNames)

		// Calculate total hours for the day
		var totalDayHours float64
		for _, taskName := range taskNames {
			totalDayHours += tasks[taskName].TotalTime.Hours()
		}
		fmt.Printf("Total: %.2f hs\n", totalDayHours)

		for _, taskName := range taskNames {
			task := tasks[taskName]
			fmt.Printf("  %s: %.2f hs\n", taskName, task.TotalTime.Hours())
			printSubtasks(task.Children, 4)
		}
		fmt.Println()
	}

	return nil
}

// printSubtasks recursively prints subtasks with indentation
func printSubtasks(tasks map[string]*TaskNode, indent int) {
	if len(tasks) == 0 {
		return
	}
	var taskNames []string
	for name := range tasks {
		taskNames = append(taskNames, name)
	}
	sort.Strings(taskNames)

	for _, taskName := range taskNames {
		task := tasks[taskName]
		fmt.Printf("%s%s: %.2f hs\n", strings.Repeat(" ", indent), taskName, task.Duration.Hours())
		printSubtasks(task.Children, indent+2)
	}
}
