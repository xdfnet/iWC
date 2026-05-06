package claude

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Agent 管理 Claude Code 进程
type Agent struct {
	workDir    string
	cliPath    string
	sessions   map[string]*session
	sessionsMu sync.RWMutex
}

// session 管理一个 Claude Code 会话
type session struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdinMu   sync.Mutex
	stdout    io.ReadCloser
	stderrBuf *bytes.Buffer
	sessionID atomic.Value // stores string
	ctx       context.Context
	cancel    context.CancelFunc
	events    chan event
	done      chan struct{}
	alive     atomic.Bool
}

type event struct {
	Type      string
	Content   string
	SessionID string
	Error     error
}

// NewAgent 创建 Claude Code Agent
func NewAgent(workDir, cliPath string) *Agent {
	if cliPath == "" {
		cliPath = "claude"
	}
	log.Printf("Agent 配置: cli=%s, workDir=%s", cliPath, workDir)
	return &Agent{
		workDir:  workDir,
		cliPath:  cliPath,
		sessions: make(map[string]*session),
	}
}

// SendWithSession 发送消息，指定 session（空则创建新 session）
func (a *Agent) SendWithSession(ctx context.Context, text, sessionID string, imageData []byte, imageMime string) (string, string, error) {
	sessID := strings.TrimSpace(sessionID)
	if sessID == "" {
		sessID, _ = newUUID()
	}

	sess, err := a.getOrCreateSession(ctx, sessID)
	if err != nil {
		return "", "", err
	}

	// 构建消息内容
	var content []map[string]any
	var savedPaths []string

	// 如果有图片，添加图片部分（保存到磁盘 + base64 发送）
	if len(imageData) > 0 {
		mime := imageMime
		if mime == "" {
			mime = "image/jpeg"
		}
		// 保存图片到磁盘
		ext := extFromMime(mime)
		attachDir := filepath.Join(a.workDir, ".config", "iwc", "temp")
		if err := os.MkdirAll(attachDir, 0o755); err == nil {
			fname := fmt.Sprintf("img_%d%s", time.Now().UnixMilli(), ext)
			fpath := filepath.Join(attachDir, fname)
			if err := os.WriteFile(fpath, imageData, 0o644); err == nil {
				savedPaths = append(savedPaths, fpath)
			}
		}
		// base64 编码发送
		content = append(content, map[string]any{
			"type": "image",
			"source": map[string]any{
				"type":       "base64",
				"media_type": mime,
				"data":       base64.StdEncoding.EncodeToString(imageData),
			},
		})
	}

	// 添加文本部分
	textPart := text
	if textPart == "" && len(imageData) > 0 {
		textPart = "Please analyze the attached image(s)."
	}
	if len(savedPaths) > 0 {
		textPart += "\n\n(Images saved locally: " + strings.Join(savedPaths, ", ") + ")"
	}
	if textPart != "" {
		content = append(content, map[string]any{
			"type": "text",
			"text": textPart,
		})
	}

	if len(content) == 0 {
		return "", sess.sessionID.Load().(string), fmt.Errorf("空消息")
	}

	err = sess.writeJSON(map[string]any{
		"type":    "user",
		"message": map[string]any{"role": "user", "content": content},
	})
	if err != nil {
		return "", "", fmt.Errorf("发送消息失败: %w", err)
	}

	// 读取响应直到收到 result 事件
	var resultText string
	var resultSessionID string
	for {
		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		case evt, ok := <-sess.events:
			if !ok {
				return "", "", fmt.Errorf("session 关闭")
			}
			switch evt.Type {
			case "system":
				resultSessionID = evt.SessionID
			case "result":
				resultText = evt.Content
				resultSessionID = evt.SessionID
				goto done
			case "error":
				return "", "", fmt.Errorf("Claude Code 错误: %s", evt.Content)
			}
		}
	}

done:
	resultText = strings.TrimSpace(resultText)
	if resultText == "" {
		return "", resultSessionID, fmt.Errorf("Claude Code 返回空")
	}

	return resultText, resultSessionID, nil
}

func extFromMime(mime string) string {
	switch mime {
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		return ".png"
	}
}

func (sess *session) writeJSON(obj map[string]any) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	sess.stdinMu.Lock()
	defer sess.stdinMu.Unlock()
	_, err = fmt.Fprintf(sess.stdin, "%s\n", data)
	return err
}

// getOrCreateSession 获取或创建一个会话（会话会被复用）
func (a *Agent) getOrCreateSession(ctx context.Context, sessID string) (*session, error) {
	a.sessionsMu.RLock()
	if sess, ok := a.sessions[sessID]; ok {
		if sess.alive.Load() {
			a.sessionsMu.RUnlock()
			return sess, nil
		}
	}
	a.sessionsMu.RUnlock()

	sess, err := a.startSession(ctx, sessID)
	if err != nil {
		return nil, err
	}

	a.sessionsMu.Lock()
	a.sessions[sessID] = sess
	a.sessionsMu.Unlock()

	return sess, nil
}

func (a *Agent) startSession(ctx context.Context, sessID string) (*session, error) {
	sessionCtx, cancel := context.WithCancel(ctx)

	// 构建参数（按 cc-connect 的方式）
	args := []string{
		"--print",
		"--output-format", "stream-json",
		"--input-format", "stream-json",
		"--permission-prompt-tool", "stdio",
		"--verbose",
	}

	cmd := exec.CommandContext(sessionCtx, a.cliPath, args...)
	if a.workDir != "" {
		cmd.Dir = a.workDir
	}

	// 过滤 CLAUDECODE 环境变量（精确匹配 "CLAUDECODE=" 前缀）
	env := []string{}
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "CLAUDECODE=") {
			env = append(env, e)
		}
	}
	cmd.Env = env

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("创建 stdin pipe 失败: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("创建 stdout pipe 失败: %w", err)
	}

	stderrBuf := &bytes.Buffer{}
	cmd.Stderr = stderrBuf

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("启动 Claude Code 失败: %w", err)
	}

	sess := &session{
		cmd:       cmd,
		stdin:     stdin,
		stdout:    stdout,
		stderrBuf: stderrBuf,
		ctx:       sessionCtx,
		cancel:    cancel,
		events:    make(chan event, 64),
		done:      make(chan struct{}),
	}
	sess.sessionID.Store(sessID)
	sess.alive.Store(true)

	go sess.readLoop()

	return sess, nil
}

func (sess *session) readLoop() {
	defer close(sess.events)

	waitErrCh, waitDone := sess.startReadLoopWait()
	defer sess.finishReadLoop(waitErrCh)

	scanner := bufio.NewScanner(sess.stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var raw map[string]any
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		evtType, _ := raw["type"].(string)

		switch evtType {
		case "system":
			if systemSid, ok := raw["session_id"].(string); ok {
				sess.sessionID.Store(systemSid)
				sess.events <- event{Type: "system", SessionID: systemSid}
			}
		case "assistant":
			if msg, ok := raw["message"].(map[string]any); ok {
				if content, ok := msg["content"].([]any); ok {
					for _, c := range content {
						if cMap, ok := c.(map[string]any); ok {
							if cType, _ := cMap["type"].(string); cType == "text" {
								if text, ok := cMap["text"].(string); ok {
									sess.events <- event{Type: "text", Content: text}
								}
							}
						}
					}
				}
			}
		case "result":
			if result, ok := raw["result"].(string); ok {
				var resultSid string
				if s, ok := raw["session_id"].(string); ok {
					resultSid = s
				}
				sess.events <- event{Type: "result", Content: result, SessionID: resultSid}
			}
		case "error":
			if errMsg, ok := raw["error"].(string); ok {
				sess.events <- event{Type: "error", Content: errMsg}
			}
		}
	}

	sess.handleReadLoopScanErr(scanner.Err(), waitDone)
}

func (sess *session) startReadLoopWait() (<-chan error, <-chan struct{}) {
	waitErrCh := make(chan error, 1)
	waitDone := make(chan struct{})

	go func() {
		waitErrCh <- sess.cmd.Wait()
		close(waitDone)
	}()

	// 50ms 宽限期，让 scanner 有时间排空缓冲
	go func() {
		select {
		case <-sess.ctx.Done():
			_ = sess.stdout.Close()
			return
		case <-waitDone:
		}
		select {
		case <-sess.done:
			return
		case <-time.After(50 * time.Millisecond):
		}
		_ = sess.stdout.Close()
	}()

	return waitErrCh, waitDone
}

func (sess *session) finishReadLoop(waitErrCh <-chan error) {
	err := <-waitErrCh

	sess.alive.Store(false)
	stderrMsg := strings.TrimSpace(sess.stderrBuf.String())

	if stderrMsg != "" {
		log.Printf("Claude Code stderr: %s", stderrMsg)
	}

	if err != nil {
		errMsg := err.Error()
		if stderrMsg != "" {
			errMsg = fmt.Sprintf("%s (stderr: %s)", errMsg, stderrMsg)
		}
		select {
		case sess.events <- event{Type: "error", Content: errMsg}:
		case <-sess.ctx.Done():
		}
	}
	close(sess.done)
}

func (sess *session) handleReadLoopScanErr(err error, waitDone <-chan struct{}) {
	if err == nil {
		return
	}

	select {
	case <-sess.ctx.Done():
		return
	case <-waitDone:
		return
	default:
	}

	log.Printf("readLoop scanner error: %v", err)
	evt := event{Type: "error", Content: fmt.Errorf("read stdout: %w", err).Error()}
	select {
	case sess.events <- evt:
	case <-sess.ctx.Done():
	}
}

func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4],
		b[4:6],
		b[6:8],
		b[8:10],
		b[10:16],
	), nil
}

// Send 发送消息（无 session 复用，每次新建）
func (a *Agent) Send(ctx context.Context, text string) (string, error) {
	out, _, err := a.SendWithSession(ctx, text, "", nil, "")
	return out, err
}

// IsRunning 始终返回 true（无状态进程）
func (a *Agent) IsRunning() bool {
	return true
}
