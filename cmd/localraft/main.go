package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/samber/lo"
	"github.com/shirou/gopsutil/v4/process"
)

const (
	defaultNode1Config = "examples/local-raft/node1.yaml"
	defaultNode2Config = "examples/local-raft/node2.yaml"
	defaultNode3Config = "examples/local-raft/node3.yaml"
	defaultWorkload    = "examples/local-raft/echo.yaml"
	defaultHostHeader  = "echo.warden.local"
)

type nodeSpec struct {
	Name   string
	API    int
	Config string
}

type clusterPaths struct {
	rootDir string
	runDir  string
	binPath string
	pidDir  string
	logDir  string
}

type responseEnvelope[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type deployResult struct {
	DeploymentID string `json:"deployment_id"`
	WorkloadName string `json:"workload_name"`
	Instances    int    `json:"instances"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "start":
		err = runStart(os.Args[2:])
	case "stop":
		err = runStop(os.Args[2:])
	case "status":
		err = runStatus(os.Args[2:])
	case "check":
		err = runCheck(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		err = fmt.Errorf("unsupported command: %s", os.Args[1])
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "[localraft] error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("localraft: local multi-process raft helper")
	fmt.Println("")
	fmt.Println("usage:")
	fmt.Println("  go run ./cmd/localraft start   [-rebuild]")
	fmt.Println("  go run ./cmd/localraft check   [-leader-api ... -follower-api ... -follower-ingress ...]")
	fmt.Println("  go run ./cmd/localraft status")
	fmt.Println("  go run ./cmd/localraft stop")
}

func runStart(args []string) error {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	rebuild := fs.Bool("rebuild", false, "rebuild server binary")
	node1Conf := fs.String("node1-conf", defaultNode1Config, "node1 config file")
	node2Conf := fs.String("node2-conf", defaultNode2Config, "node2 config file")
	node3Conf := fs.String("node3-conf", defaultNode3Config, "node3 config file")
	delay := fs.Duration("delay", 2*time.Second, "delay between node start operations")
	if err := fs.Parse(args); err != nil {
		return err
	}

	paths, err := resolvePaths()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(paths.runDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(paths.pidDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(paths.logDir, 0o755); err != nil {
		return err
	}

	if *rebuild || !fileExists(paths.binPath) {
		fmt.Println("[localraft] building server binary ...")
		if err := buildServerBinary(paths.rootDir, paths.binPath); err != nil {
			return err
		}
	}

	nodes := []nodeSpec{
		{Name: "node2", API: 7444, Config: *node2Conf},
		{Name: "node3", API: 7445, Config: *node3Conf},
		{Name: "node1", API: 7443, Config: *node1Conf},
	}

	for _, node := range nodes {
		if err := startNode(paths, node); err != nil {
			return err
		}
		time.Sleep(*delay)
	}

	fmt.Println("")
	fmt.Println("[localraft] cluster start requested.")
	apis := lo.Map([]nodeSpec{
		{Name: "node1", API: 7443},
		{Name: "node2", API: 7444},
		{Name: "node3", API: 7445},
	}, func(item nodeSpec, _ int) string {
		return fmt.Sprintf("  %s: http://127.0.0.1:%d", item.Name, item.API)
	})
	fmt.Println(strings.Join(apis, "\n"))
	fmt.Println("")
	fmt.Println("next:")
	fmt.Println("  1) go run ./cmd/localraft check")
	fmt.Println("  2) go run ./cmd/localraft stop")
	return nil
}

func runStop(args []string) error {
	fs := flag.NewFlagSet("stop", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}

	paths, err := resolvePaths()
	if err != nil {
		return err
	}
	if !fileExists(paths.pidDir) {
		fmt.Println("[localraft] no pid directory, nothing to stop")
		return nil
	}

	pidFiles, err := filepath.Glob(filepath.Join(paths.pidDir, "*.pid"))
	if err != nil {
		return err
	}
	if len(pidFiles) == 0 {
		fmt.Println("[localraft] no pid files, nothing to stop")
		return nil
	}

	for _, pidFile := range pidFiles {
		nodeName := strings.TrimSuffix(filepath.Base(pidFile), ".pid")
		pid, err := readPID(pidFile)
		if err != nil {
			_ = os.Remove(pidFile)
			fmt.Printf("[localraft] %s pid invalid, cleaned\n", nodeName)
			continue
		}
		alive, err := isProcessAlive(pid)
		if err != nil {
			return err
		}
		if alive {
			if err := killProcess(pid); err != nil {
				return fmt.Errorf("stop %s (pid=%d): %w", nodeName, pid, err)
			}
			fmt.Printf("[localraft] stopped %s (pid=%d)\n", nodeName, pid)
		} else {
			fmt.Printf("[localraft] %s not running (pid=%d)\n", nodeName, pid)
		}
		_ = os.Remove(pidFile)
	}

	fmt.Println("[localraft] stopped.")
	return nil
}

func runStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}

	paths, err := resolvePaths()
	if err != nil {
		return err
	}
	nodes := []nodeSpec{
		{Name: "node1", API: 7443},
		{Name: "node2", API: 7444},
		{Name: "node3", API: 7445},
	}

	fmt.Println("[localraft] status")
	for _, node := range nodes {
		pidPath := filepath.Join(paths.pidDir, node.Name+".pid")
		if !fileExists(pidPath) {
			fmt.Printf("  %s: stopped\n", node.Name)
			continue
		}
		pid, err := readPID(pidPath)
		if err != nil {
			fmt.Printf("  %s: pid file invalid\n", node.Name)
			continue
		}
		alive, err := isProcessAlive(pid)
		if err != nil {
			return err
		}
		state := "stopped"
		if alive {
			state = "running"
		}
		fmt.Printf("  %s: %s (pid=%d, api=http://127.0.0.1:%d)\n", node.Name, state, pid, node.API)
	}
	return nil
}

func runCheck(args []string) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	leaderAPI := fs.String("leader-api", "http://127.0.0.1:7443", "leader API base URL")
	followerAPI := fs.String("follower-api", "http://127.0.0.1:7444", "follower API base URL")
	followerIngress := fs.String("follower-ingress", "http://127.0.0.1:18082", "follower ingress base URL")
	workloadFile := fs.String("workload", defaultWorkload, "workload file path")
	hostHeader := fs.String("host", defaultHostHeader, "ingress host header")
	tokenFile := fs.String("token-file", filepath.Join(os.TempDir(), "warden.token"), "token file path")
	timeout := fs.Duration("timeout", 30*time.Second, "max ingress probe duration")
	if err := fs.Parse(args); err != nil {
		return err
	}

	paths, err := resolvePaths()
	if err != nil {
		return err
	}
	tokenRaw, err := os.ReadFile(*tokenFile)
	if err != nil {
		return fmt.Errorf("read token file: %w", err)
	}
	token := strings.TrimSpace(string(tokenRaw))
	if token == "" {
		return fmt.Errorf("token file is empty: %s", *tokenFile)
	}

	workloadPath, err := resolveFile(paths.rootDir, *workloadFile)
	if err != nil {
		return err
	}
	workloadRaw, err := os.ReadFile(workloadPath)
	if err != nil {
		return fmt.Errorf("read workload file: %w", err)
	}

	deployBody, err := json.Marshal(map[string]string{
		"filename": filepath.Base(workloadPath),
		"format":   "yaml",
		"content":  string(workloadRaw),
	})
	if err != nil {
		return err
	}

	fmt.Printf("[localraft] step1 deploy to leader %s ...\n", *leaderAPI)
	leaderRespRaw, err := doJSONRequest(http.MethodPost, strings.TrimRight(*leaderAPI, "/")+"/tasks/deploy", token, deployBody, nil)
	if err != nil {
		return err
	}

	var leaderResp responseEnvelope[deployResult]
	if err := json.Unmarshal(leaderRespRaw, &leaderResp); err != nil {
		return fmt.Errorf("decode leader response: %w", err)
	}
	if leaderResp.Code != 0 {
		return fmt.Errorf("leader deploy failed: %s", leaderResp.Message)
	}
	if strings.TrimSpace(leaderResp.Data.DeploymentID) == "" {
		return errors.New("leader deploy response missing deployment_id")
	}
	fmt.Printf("[localraft] deploy ok, deployment_id=%s\n", leaderResp.Data.DeploymentID)

	fmt.Printf("[localraft] step2 deploy to follower %s should fail with not leader ...\n", *followerAPI)
	statusCode, followerRaw, err := doRawJSONRequest(http.MethodPost, strings.TrimRight(*followerAPI, "/")+"/tasks/deploy", token, deployBody)
	if err != nil {
		return err
	}
	if statusCode >= 200 && statusCode < 300 {
		return errors.New("follower deploy unexpectedly succeeded")
	}
	followerText := strings.ToLower(string(followerRaw))
	if !strings.Contains(followerText, "not leader") {
		return fmt.Errorf("follower returned unexpected error: status=%d body=%s", statusCode, string(followerRaw))
	}
	fmt.Println("[localraft] follower reject confirmed: not leader")

	fmt.Printf("[localraft] step3 request follower ingress %s (host=%s) ...\n", *followerIngress, *hostHeader)
	if err := probeIngress(*followerIngress, *hostHeader, *timeout); err != nil {
		return err
	}
	fmt.Println("[localraft] ingress check ok: follower routes replicated data.")
	fmt.Println("[localraft] all checks passed.")
	return nil
}

func probeIngress(baseURL, host string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	target := strings.TrimRight(baseURL, "/") + "/"

	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodGet, target, nil)
		if err != nil {
			return err
		}
		req.Host = host
		resp, err := client.Do(req)
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK && strings.Contains(strings.ToLower(string(body)), "raft-ok") {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	return errors.New("ingress probe failed, raft replication may not be ready")
}

func startNode(paths clusterPaths, node nodeSpec) error {
	confPath, err := resolveFile(paths.rootDir, node.Config)
	if err != nil {
		return err
	}
	pidPath := filepath.Join(paths.pidDir, node.Name+".pid")
	if fileExists(pidPath) {
		pid, readErr := readPID(pidPath)
		if readErr == nil {
			alive, aliveErr := isProcessAlive(pid)
			if aliveErr != nil {
				return aliveErr
			}
			if alive {
				fmt.Printf("[localraft] %s already running (pid=%d), skip\n", node.Name, pid)
				return nil
			}
		}
		_ = os.Remove(pidPath)
	}

	stdoutPath := filepath.Join(paths.logDir, node.Name+".out.log")
	stderrPath := filepath.Join(paths.logDir, node.Name+".err.log")
	stdoutFile, err := os.OpenFile(stdoutPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer stdoutFile.Close()
	stderrFile, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer stderrFile.Close()

	fmt.Printf("[localraft] starting %s with %s\n", node.Name, node.Config)
	cmd := exec.Command(paths.binPath, "server", "--conf", confPath)
	cmd.Dir = paths.rootDir
	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile
	if err := cmd.Start(); err != nil {
		return err
	}
	if cmd.Process == nil {
		return fmt.Errorf("node %s has nil process", node.Name)
	}
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0o644); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func buildServerBinary(rootDir, output string) error {
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	cmd := exec.Command("go", "build", "-o", output, "./cmd/server")
	cmd.Dir = rootDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func resolvePaths() (clusterPaths, error) {
	rootDir, err := resolveRepoRoot()
	if err != nil {
		return clusterPaths{}, err
	}
	runDir := filepath.Join(rootDir, ".tmp", "local-raft")
	return clusterPaths{
		rootDir: rootDir,
		runDir:  runDir,
		binPath: filepath.Join(runDir, "bin", binaryName("warden-server")),
		pidDir:  filepath.Join(runDir, "pids"),
		logDir:  filepath.Join(runDir, "logs"),
	}, nil
}

func resolveRepoRoot() (string, error) {
	if raw := strings.TrimSpace(os.Getenv("WARDEN_ROOT")); raw != "" {
		return filepath.Clean(raw), nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if fileExists(filepath.Join(dir, "go.mod")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", errors.New("cannot find repo root (go.mod)")
}

func resolveFile(rootDir, pathOrAbs string) (string, error) {
	path := strings.TrimSpace(pathOrAbs)
	if path == "" {
		return "", errors.New("path is empty")
	}
	if filepath.IsAbs(path) {
		if !fileExists(path) {
			return "", fmt.Errorf("file not found: %s", path)
		}
		return path, nil
	}
	abs := filepath.Join(rootDir, path)
	if !fileExists(abs) {
		return "", fmt.Errorf("file not found: %s", abs)
	}
	return abs, nil
}

func binaryName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func readPID(path string) (int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		return 0, err
	}
	if pid <= 0 {
		return 0, fmt.Errorf("invalid pid: %d", pid)
	}
	return pid, nil
}

func isProcessAlive(pid int) (bool, error) {
	return process.PidExists(int32(pid))
}

func killProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := proc.Kill(); err != nil {
		return err
	}
	return nil
}

func doJSONRequest(method, url, token string, body []byte, headers map[string]string) ([]byte, error) {
	statusCode, raw, err := doRawJSONRequest(method, url, token, body, headers)
	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, fmt.Errorf("request failed: status=%d body=%s", statusCode, string(raw))
	}
	return raw, nil
}

func doRawJSONRequest(method, url, token string, body []byte, extraHeaders ...map[string]string) (int, []byte, error) {
	requestBody := bytes.NewReader(body)
	req, err := http.NewRequest(method, url, requestBody)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	}
	if len(extraHeaders) > 0 {
		for key, value := range extraHeaders[0] {
			req.Header.Set(key, value)
		}
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, raw, nil
}
