package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/common-nighthawk/go-figure"
	"golang.org/x/term"
)

var (
	// Styles with white, black, and gray colors
	headerStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")) // White
	infoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#808080"))            // Gray
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))            // White
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#808080"))            // Gray
	listStyle    = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).Padding(0, 1)
	nameStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")) // White
	userStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#808080")) // Gray
	hostStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#808080")) // Gray

	// Config
	homeDir, _  = os.UserHomeDir()
	configFile  = filepath.Join(homeDir, ".ssh", "connections.conf")
	sshKey      = filepath.Join(homeDir, ".ssh", "id_rsa")
	backupDir   = filepath.Join(homeDir, "ssh_backup")
	connections []string
)

func initDirs() {
	// Create necessary directories with appropriate permissions
	os.MkdirAll(filepath.Join(homeDir, ".ssh"), 0700)
	os.MkdirAll(backupDir, 0700)
	// Create config file if it doesn't exist
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		os.Create(configFile)
	}
}

func drawHeader() string {
	fig := figure.NewFigure("Hmpssh", "", true)
	text := fig.String()
	text += "https://github.com/thxrhmn/hmpssh\n"
	// No additional color styling applied to each line
	return text
}

func backupConfig() {
	// Generate timestamp for backup file name
	timestamp := time.Now().Format("20060102_150405")
	backupFile := filepath.Join(backupDir, fmt.Sprintf("ssh_config_backup_%s.tar.gz", timestamp))

	fmt.Println(infoStyle.Render("Creating configuration backup..."))
	fmt.Println(infoStyle.Render("Location: " + backupFile))
	time.Sleep(time.Second)

	// Create a tarball of the config files
	cmd := exec.Command("tar", "-czf", backupFile, "-C", filepath.Join(homeDir, ".ssh"), "connections.conf", "id_rsa", "id_rsa.pub")
	err := cmd.Run()
	if err != nil {
		fmt.Println(errorStyle.Render("✗ Failed to create backup"))
		return
	}
	fmt.Println(successStyle.Render("✓ Backup successful"))
	fmt.Println("Please move the file to another device for restore")
}

func addConnection() {
	fmt.Println(infoStyle.Render("Add a new SSH connection:"))

	scanner := bufio.NewScanner(os.Stdin)

	// Input connection name
	fmt.Print("Connection Name: ")
	scanner.Scan()
	name := strings.TrimSpace(scanner.Text())
	if name == "" {
		fmt.Println(errorStyle.Render("✗ Name cannot be empty"))
		return
	}

	// Input username
	fmt.Print("Username: ")
	scanner.Scan()
	user := strings.TrimSpace(scanner.Text())
	if user == "" {
		fmt.Println(errorStyle.Render("✗ Username cannot be empty"))
		return
	}

	// Input host
	fmt.Print("Host (IP or domain): ")
	scanner.Scan()
	host := strings.TrimSpace(scanner.Text())
	if host == "" {
		fmt.Println(errorStyle.Render("✗ Host cannot be empty"))
		return
	}

	// Input port with default value 22
	fmt.Print("Port (default 22): ")
	scanner.Scan()
	portInput := strings.TrimSpace(scanner.Text())
	port := "22"
	if portInput != "" {
		if _, err := strconv.Atoi(portInput); err != nil || len(portInput) > 5 {
			fmt.Println(errorStyle.Render("✗ Invalid port number"))
			return
		}
		port = portInput
	}

	// Format connection: name|user|host|port
	newConnection := fmt.Sprintf("%s|%s|%s|%s", name, user, host, port)

	// Write to the configuration file
	file, err := os.OpenFile(configFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		fmt.Println(errorStyle.Render("✗ Failed to open config file"))
		return
	}
	defer file.Close()

	if _, err := file.WriteString(newConnection + "\n"); err != nil {
		fmt.Println(errorStyle.Render("✗ Failed to save connection"))
		return
	}

	fmt.Println(successStyle.Render("✓ Connection added successfully"))

	// Ask if the user wants to set up an SSH key
	fmt.Print("Setup SSH key now? (y/n): ")
	scanner.Scan()
	setupKey := strings.ToLower(strings.TrimSpace(scanner.Text()))

	if setupKey == "y" {
		// Check if SSH key exists; if not, generate a new one
		if _, err := os.Stat(sshKey); os.IsNotExist(err) {
			fmt.Println(infoStyle.Render("Generating new SSH key..."))
			cmd := exec.Command("ssh-keygen", "-t", "rsa", "-b", "4096", "-f", sshKey, "-N", "")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Println(errorStyle.Render("✗ Failed to generate SSH key"))
				return
			}
		}

		// Copy the key to the host
		fmt.Println(infoStyle.Render(fmt.Sprintf("Copying key to %s...", host)))
		cmd := exec.Command("ssh-copy-id", "-i", sshKey, "-p", port, fmt.Sprintf("%s@%s", user, host))
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			fmt.Println(errorStyle.Render("✗ Failed to copy SSH key to host"))
			return
		}
		fmt.Println(successStyle.Render("✓ SSH key setup completed"))
	}
}

func deleteConnection() {
	data, err := os.ReadFile(configFile)
	if err != nil {
		fmt.Println(errorStyle.Render("✗ Failed to read config file"))
		return
	}

	connections = strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(connections) == 0 || connections[0] == "" {
		fmt.Println(infoStyle.Render("No connections to delete"))
		return
	}

	fmt.Println(infoStyle.Render("Select a connection to delete (enter the number):"))
	for i, conn := range connections {
		parts := strings.Split(conn, "|")
		if len(parts) >= 3 {
			fmt.Printf("[%d] %s (%s@%s)\n", i, nameStyle.Render(parts[0]), userStyle.Render(parts[1]), hostStyle.Render(parts[2]))
		}
	}

	fmt.Print("Choice: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	choice := strings.TrimSpace(scanner.Text())

	index := -1
	fmt.Sscanf(choice, "%d", &index)
	if index < 0 || index >= len(connections) {
		fmt.Println(errorStyle.Render("✗ Invalid selection"))
		return
	}

	// Remove the selected connection
	connections = append(connections[:index], connections[index+1:]...)

	file, err := os.OpenFile(configFile, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0600)
	if err != nil {
		fmt.Println(errorStyle.Render("✗ Failed to open config file for writing"))
		return
	}
	defer file.Close()

	if len(connections) > 0 {
		_, err = file.WriteString(strings.Join(connections, "\n") + "\n")
	} else {
		err = file.Truncate(0)
	}
	if err != nil {
		fmt.Println(errorStyle.Render("✗ Failed to update config file"))
		return
	}

	fmt.Println(successStyle.Render("✓ Connection deleted successfully"))
}

func listConnections() {
	fmt.Println(drawHeader())

	data, err := os.ReadFile(configFile)
	if err != nil {
		fmt.Println(errorStyle.Render("✗ Failed to read config file"))
		return
	}

	connections = strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(connections) == 0 || connections[0] == "" {
		fmt.Println(infoStyle.Render("No connections yet"))
		return
	}

	// Get terminal width
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		// Fallback to default width if detection fails
		width = 80
		fmt.Println(errorStyle.Render("⚠ Using default terminal width due to size detection error"))
	}

	// Determine number of columns and set column width
	baseColWidth := 30
	borderPadding := 2
	colWidth := baseColWidth + borderPadding

	var numCols int
	switch {
	case width >= colWidth*3:
		numCols = 3
	case width >= colWidth*2:
		numCols = 2
	default:
		numCols = 1
	}

	var rows []string
	for i := 0; i < len(connections); i += numCols {
		var rowCols []string
		for j := 0; j < numCols && (i+j) < len(connections); j++ {
			parts := strings.Split(connections[i+j], "|")
			if len(parts) >= 4 {
				connInfo := []string{
					nameStyle.Render(fmt.Sprintf("Name: %s", parts[0])),
					userStyle.Render(fmt.Sprintf("User: %s", parts[1])),
					hostStyle.Render(fmt.Sprintf("Host: %s:%s", parts[2], parts[3])),
				}
				conn := strings.Join(connInfo, "\n")
				rowCols = append(rowCols, listStyle.Width(baseColWidth).Render(conn))
			}
		}
		if len(rowCols) > 0 {
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, rowCols...))
		}
	}

	if len(rows) > 0 {
		fmt.Println(lipgloss.JoinVertical(lipgloss.Left, rows...))
	}
}

func startSSHAgent() error {
	if os.Getenv("SSH_AUTH_SOCK") == "" {
		cmd := exec.Command("ssh-agent", "-s")
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("failed to start ssh-agent: %v", err)
		}

		// Parse ssh-agent output to set environment variables
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "SSH_AUTH_SOCK") {
				parts := strings.Split(line, ";")
				for _, part := range parts {
					if strings.Contains(part, "SSH_AUTH_SOCK") {
						kv := strings.Split(part, "=")
						if len(kv) == 2 {
							os.Setenv("SSH_AUTH_SOCK", strings.TrimSpace(kv[1]))
						}
					}
				}
			}
		}
	}

	// Add SSH key to the agent
	cmd := exec.Command("ssh-add", sshKey)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		if !strings.Contains(err.Error(), "already") {
			return fmt.Errorf("failed to add SSH key to agent: %v", err)
		}
	}

	return nil
}

func connectToServer() {
	data, _ := os.ReadFile(configFile)
	connections = strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(connections) == 0 || connections[0] == "" {
		fmt.Println("No connections available")
		return
	}

	err := startSSHAgent()
	if err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("✗ Failed to initialize SSH agent: %v", err)))
		return
	}

	fmt.Println(infoStyle.Render("Select a server to connect (enter the number):"))
	for i, conn := range connections {
		parts := strings.Split(conn, "|")
		if len(parts) >= 3 {
			fmt.Printf("[%d] %s (%s@%s)\n", i, nameStyle.Render(parts[0]), userStyle.Render(parts[1]), hostStyle.Render(parts[2]))
		}
	}

	fmt.Print("Choice: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	choice := strings.TrimSpace(scanner.Text())

	index := -1
	fmt.Sscanf(choice, "%d", &index)
	if index < 0 || index >= len(connections) {
		fmt.Println(errorStyle.Render("✗ Invalid selection"))
		return
	}

	parts := strings.Split(connections[index], "|")
	if len(parts) >= 3 {
		user := parts[1]
		host := parts[2]
		port := "22"
		if len(parts) >= 4 {
			port = parts[3]
		}

		// Start SSH connection
		cmd := exec.Command("ssh", "-i", sshKey, "-p", port, fmt.Sprintf("%s@%s", user, host))
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Println(infoStyle.Render(fmt.Sprintf("Connecting to %s@%s:%s...", user, host, port)))
		err := cmd.Run()
		if err != nil {
			fmt.Println(errorStyle.Render("✗ Connection failed"))
		}
	}
}

func setupSSHKey() {
	fmt.Println(infoStyle.Render("Setting up SSH key..."))

	// Check if SSH key already exists
	if _, err := os.Stat(sshKey); !os.IsNotExist(err) {
		fmt.Println(infoStyle.Render("SSH key already exists. Do you want to generate a new one? (y/n)"))
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		if strings.ToLower(scanner.Text()) != "y" {
			return
		}
	}

	// Generate a new SSH key
	cmd := exec.Command("ssh-keygen", "-t", "rsa", "-b", "4096", "-f", sshKey)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println(errorStyle.Render("✗ Failed to generate SSH key"))
		return
	}
	fmt.Println(successStyle.Render("✓ SSH key generated successfully"))
}

func restoreConfig() {
	fmt.Println(infoStyle.Render("Enter the path to backup file:"))

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	backupFile := strings.TrimSpace(scanner.Text())

	// Check if backup file exists
	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		fmt.Println(errorStyle.Render("✗ Backup file not found"))
		return
	}

	// Restore the configuration from the backup
	cmd := exec.Command("tar", "-xzf", backupFile, "-C", filepath.Join(homeDir, ".ssh"))
	err := cmd.Run()
	if err != nil {
		fmt.Println(errorStyle.Render("✗ Failed to restore backup"))
		return
	}
	fmt.Println(successStyle.Render("✓ Configuration restored successfully"))
}

func showHelp() {
	helpOptions := []string{
		"1. Add Connection",
		"2. Connect to Server",
		"3. Delete Connection",
		"4. View List",
		"5. Setup SSH Key",
		"6. Backup Config",
		"7. Restore Config",
		"8. Exit",
	}
	fmt.Println(strings.Join(helpOptions, "\n"))
}

func clearTerminal() {
	// Get the operating system
	switch goos := runtime.GOOS; goos {
	case "darwin", "linux":
		cmd := exec.Command("clear")
		cmd.Stdout = os.Stdout
		cmd.Run()
	case "windows":
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		cmd.Run()
	default:
		// If OS is unrecognized, print some blank lines
		fmt.Print("\033[H\033[2J")
	}
}

func mainMenu() {
	listConnections()
	fmt.Println(infoStyle.Render("Type '?' or 'help' for options"))
}

func main() {
	// Initialize directories
	initDirs()
	scanner := bufio.NewScanner(os.Stdin)
	for {
		mainMenu()
		fmt.Print("Choice: ")
		scanner.Scan()
		choice := strings.TrimSpace(scanner.Text())
		fmt.Println()

		switch choice {
		case "?", "help":
			showHelp()
		case "1":
			addConnection()
		case "2":
			connectToServer()
		case "3":
			deleteConnection()
		case "4":
			listConnections()
		case "5":
			setupSSHKey()
		case "6":
			backupConfig()
		case "7":
			restoreConfig()
		case "8":
			fmt.Println(successStyle.Render("Thank you for using SSH Connection Manager"))
			return
		default:
			fmt.Println(errorStyle.Render("✗ Invalid choice, type '?' for help"))
		}
		fmt.Print("\nPress Enter to continue...")
		scanner.Scan()
		// Clear terminal before returning to the main menu
		clearTerminal()
		fmt.Println()
	}
}
