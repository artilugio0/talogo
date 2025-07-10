package cmd

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var (
	logCmdLogFile string
)

type model struct {
	logFile   string
	titles    []string
	startTime time.Time
	elapsed   time.Duration
	running   bool
	quitting  bool
}

type tickMsg time.Time

var logCmd = &cobra.Command{
	Use:   "log TITLE {SUBTITLES}",
	Short: "Start tracking a task and log to file when finished",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		m := model{
			logFile:   logCmdLogFile,
			titles:    args, // Take all arguments as titles
			startTime: time.Now(),
			running:   true,
		}

		// Create program without AltScreen
		p := tea.NewProgram(m)
		if _, err := p.Run(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	logCmd.Flags().StringVarP(&logCmdLogFile, "file", "f", "./talogo.csv", "Log file to write")
	rootCmd.AddCommand(logCmd)
}

func (m model) Init() tea.Cmd {
	return tickCmd()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			m.running = false
			m.quitting = true
			// Save to CSV immediately on Ctrl+C
			if err := m.logToCSV(); err != nil {
				fmt.Printf("Error writing to CSV: %v\n", err)
			}
			return m, tea.Quit
		}
	case tickMsg:
		if m.running {
			m.elapsed = time.Since(m.startTime)
			return m, tickCmd()
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return "Timer stopped. Data saved to " + m.logFile + "\n"
	}
	if !m.running {
		return "Timer stopped.\n"
	}
	hours := int(m.elapsed.Hours())
	minutes := int(m.elapsed.Minutes()) % 60
	seconds := int(m.elapsed.Seconds()) % 60

	// Build title display with hierarchical numbering
	var titleLines []string
	for i, title := range m.titles {
		titleLines = append(titleLines, fmt.Sprintf("Title %d: %s", i+1, title))
	}
	return fmt.Sprintf("%s\nTimer: %02d:%02d:%02d\n", strings.Join(titleLines, "\n"), hours, minutes, seconds)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) logToCSV() error {
	startTime := m.startTime
	endTime := m.startTime.Add(m.elapsed)

	// Ensure file is created with proper permissions
	file, err := os.OpenFile(m.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open/create CSV file: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Check if file is empty to add header
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %v", err)
	}

	// Read existing CSV to determine max number of titles
	maxTitles := len(m.titles)
	if fileInfo.Size() > 0 {
		// Open file for reading to check existing headers
		readFile, err := os.Open(m.logFile)
		if err != nil {
			return fmt.Errorf("failed to read CSV file: %v", err)
		}
		defer readFile.Close()

		reader := csv.NewReader(readFile)
		headers, err := reader.Read()
		if err != nil {
			return fmt.Errorf("failed to read CSV headers: %v", err)
		}
		// Count title columns (headers after end_time)
		titleCount := len(headers) - 2 // start_time, end_time
		if titleCount > maxTitles {
			maxTitles = titleCount
		}
	}

	// Write header if file is empty
	if fileInfo.Size() == 0 {
		header := []string{"start_time", "end_time"}
		for i := 1; i <= maxTitles; i++ {
			header = append(header, fmt.Sprintf("title%d", i))
		}
		if err := writer.Write(header); err != nil {
			return fmt.Errorf("failed to write CSV header: %v", err)
		}
	}

	// Split into daily records if spanning multiple days
	currentStart := startTime
	for {
		year, month, day := currentStart.Date()
		nextDay := time.Date(year, month, day+1, 0, 0, 0, 0, currentStart.Location())
		endOfDay := nextDay.Add(-time.Nanosecond)

		currentEnd := endOfDay
		if endOfDay.After(endTime) {
			currentEnd = endTime
		}

		// Create record
		record := []string{
			currentStart.Format(time.RFC3339),
			currentEnd.Format(time.RFC3339),
		}
		// Add titles, padding with empty strings if fewer than maxTitles
		record = append(record, m.titles...)
		for len(record) < 2+maxTitles {
			record = append(record, "")
		}

		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write CSV record: %v", err)
		}

		if currentEnd.Equal(endTime) {
			break
		}

		// Move to next day
		currentStart = endOfDay.Add(time.Nanosecond)
	}

	// Ensure all data is written to disk
	if err := file.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %v", err)
	}

	return nil
}
