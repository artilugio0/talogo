package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	title     string
	startTime time.Time
	elapsed   time.Duration
	running   bool
	quitting  bool
}

type tickMsg time.Time

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Please provide a title as a command-line argument")
		os.Exit(1)
	}
	title := strings.Join(os.Args[1:], " ")

	m := model{
		title:     title,
		startTime: time.Now(),
		running:   true,
	}

	// Create program without AltScreen
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
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
		return "Timer stopped. Data saved to talogo.csv\n"
	}
	if !m.running {
		return "Timer stopped.\n"
	}
	hours := int(m.elapsed.Hours())
	minutes := int(m.elapsed.Minutes()) % 60
	seconds := int(m.elapsed.Seconds()) % 60
	return fmt.Sprintf("Title: %s\nTimer: %02d:%02d:%02d\n", m.title, hours, minutes, seconds)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) logToCSV() error {
	endTime := m.startTime.Add(m.elapsed)
	duration := int(m.elapsed.Seconds())

	// Ensure file is created with proper permissions
	file, err := os.OpenFile("talogo.csv", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
	if fileInfo.Size() == 0 {
		if err := writer.Write([]string{"title", "start_time", "end_time", "duration_seconds"}); err != nil {
			return fmt.Errorf("failed to write CSV header: %v", err)
		}
	}

	record := []string{
		m.title,
		m.startTime.Format(time.RFC3339),
		endTime.Format(time.RFC3339),
		fmt.Sprintf("%d", duration),
	}

	if err := writer.Write(record); err != nil {
		return fmt.Errorf("failed to write CSV record: %v", err)
	}

	// Ensure all data is written to disk
	if err := file.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %v", err)
	}

	return nil
}
