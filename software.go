package main

import (
	"html/template"
	"net/http"
	"strings"
)

// Software represents a software package to be installed
type Software struct {
	Name        string
	Description string
	Command     string
}

// Common software packages for Ubuntu
var commonSoftware = []Software{
	{Name: "nginx", Description: "Web server", Command: "apt install -y nginx"},
	{Name: "python3", Description: "Python programming language", Command: "apt install -y python3"},
	{Name: "nodejs", Description: "JavaScript runtime", Command: "apt install -y nodejs npm"},
	{Name: "git", Description: "Version control system", Command: "apt install -y git"},
	{Name: "docker", Description: "Container platform", Command: "apt install -y docker.io"},
	{Name: "postgresql", Description: "SQL database", Command: "apt install -y postgresql postgresql-contrib"},
	{Name: "mysql", Description: "MySQL database", Command: "apt install -y mysql-server mysql-client"},
	{Name: "vim", Description: "Text editor", Command: "apt install -y vim"},
	{Name: "curl", Description: "Command line tool for transferring data", Command: "apt install -y curl"},
	{Name: "wget", Description: "Command line tool for retrieving files", Command: "apt install -y wget"},
}

// softwareHandler displays the software installation page
func softwareHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/software.html"))

	data := map[string]interface{}{
		"Servers":  ipMap,
		"Software": commonSoftware,
	}

	tmpl.Execute(w, data)
}

// installSoftwareHandler installs software on the selected server
func installSoftwareHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse form data
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Error parsing form: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Get server IP
	serverIP := r.FormValue("server_ip")
	if serverIP == "" {
		http.Error(w, "Server IP is required", http.StatusBadRequest)
		return
	}

	// Get server info
	server, ok := ipMap[serverIP]
	if !ok {
		http.Error(w, "Server not found", http.StatusNotFound)
		return
	}

	// Get software selection or custom command
	softwareType := r.FormValue("software_type")
	var installCommand string

	if softwareType == "common" {
		// Get selected common software
		softwareName := r.FormValue("common_software")
		found := false
		for _, s := range commonSoftware {
			if s.Name == softwareName {
				installCommand = s.Command
				found = true
				break
			}
		}

		if !found {
			http.Error(w, "Selected software not found", http.StatusBadRequest)
			return
		}
	} else if softwareType == "custom" {
		// Get custom software name
		customSoftware := strings.TrimSpace(r.FormValue("custom_software"))
		if customSoftware == "" {
			http.Error(w, "Custom software name is required", http.StatusBadRequest)
			return
		}

		// Sanitize input to prevent command injection
		customSoftware = sanitizePackageName(customSoftware)
		installCommand = "apt install -y " + customSoftware
	} else {
		http.Error(w, "Invalid software type", http.StatusBadRequest)
		return
	}

	// Build the full installation script
	var script strings.Builder
	if server.RootUsername == "root" {
		// Running as root, no need for sudo - use apk for Alpine
		script.WriteString("apk update && ")
		// Convert apt commands to apk commands for Alpine
		installCommand = strings.ReplaceAll(installCommand, "apt install -y", "apk add")
		script.WriteString(installCommand)
	} else {
		// Not running as root, use sudo with apt for Ubuntu
		script.WriteString("echo '")
		script.WriteString(server.RootPassword)
		script.WriteString("' | sudo -S apt update && echo '")
		script.WriteString(server.RootPassword)
		script.WriteString("' | sudo -S ")
		// Convert apk commands to apt commands
		installCommand = strings.ReplaceAll(installCommand, "apk add", "apt install -y")
		script.WriteString(installCommand)
	}

	// Execute the command on the remote server
	output, err := runRemoteCommand(serverIP, server.RootUsername, server.RootPassword, script.String())

	// Prepare log output
	var logBuilder strings.Builder
	logBuilder.WriteString("📦 Software Installation Log\n\n")
	logBuilder.WriteString("Server: " + serverIP + "\n")
	logBuilder.WriteString("Command: " + installCommand + "\n\n")

	if err != nil {
		logBuilder.WriteString("❌ Installation failed: " + err.Error() + "\n\n")
	} else {
		logBuilder.WriteString("✅ Installation command executed successfully\n\n")
	}

	logBuilder.WriteString("Output:\n" + output)

	// Display the results
	tmpl := template.Must(template.ParseFiles("templates/logs.html"))
	tmpl.Execute(w, logBuilder.String())
}

// sanitizePackageName removes potentially dangerous characters from package names
func sanitizePackageName(name string) string {
	// Remove any characters that could be used for command injection
	disallowed := []string{";", "&&", "||", "|", ">", "<", "$", "`", "\"", "'", "(", ")", "{", "}", "[", "]", "\n", "\r"}
	result := name

	for _, char := range disallowed {
		result = strings.ReplaceAll(result, char, "")
	}

	// Split by spaces and take only the first part to ensure it's a single package name
	parts := strings.Fields(result)
	if len(parts) > 0 {
		return parts[0]
	}

	return ""
}
