package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/admin/iCode/iWC/claude"
	"github.com/admin/iCode/iWC/config"
	"github.com/admin/iCode/iWC/engine"
	"github.com/admin/iCode/iWC/weixin"
)

const version = "1.0.6"

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	if len(os.Args) < 2 {
		printUsage()
		return
	}

	switch os.Args[1] {
	case "start":
		run()
	case "stop":
		runStop()
	case "restart":
		runRestart()
	case "wechat", "weixin":
		runWechatCmd(os.Args[2:])
	case "autostart":
		runAutostartCmd(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Printf("iWC v%s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n", os.Args[1])
		fmt.Fprintln(os.Stderr, "运行 iwc help 查看用法")
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`iWC - 微信个人号 ↔ Claude Code 桥接工具

用法:
  iwc start             启动服务
  iwc stop              停止服务
  iwc restart           重启服务
  iwc wechat setup      扫码登录微信
  iwc autostart on      设置开机自启
  iwc autostart off     取消开机自启
  iwc version           显示版本号

首次使用:
  1. iwc wechat setup     # 扫码登录
  2. iwc autostart on     # 设置开机自启（可选）
  3. iwc start            # 启动服务`)
}

// --- start ---

func run() {
	cfg, err := config.Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}

	if cfg.WeChat.Token == "" {
		fmt.Fprintln(os.Stderr, "❌ 未配置微信 token，请先运行: iwc wechat setup")
		os.Exit(1)
	}

	wx := weixin.NewPlatform(
		cfg.WeChat.Token,
		cfg.WeChat.BaseURL,
		cfg.WeChat.AllowFrom,
		cfg.WeChat.LongPollMS,
		cfg.System.DataDir,
	)

	agent := claude.NewAgent(cfg.Claude.WorkDir, cfg.Claude.CLIPath)

	eng := engine.New(wx, agent)
	if cfg.System.DataDir != "" {
		wechatDir := filepath.Join(cfg.System.DataDir, "wechat")
		os.MkdirAll(wechatDir, 0755)
		eng.SetSessionsPath(filepath.Join(wechatDir, "sessions.json"))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 写 PID 文件（供 stop 命令使用）
	pidPath := pidFilePath()
	os.MkdirAll(filepath.Dir(pidPath), 0755)
	os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0644)
	defer os.Remove(pidPath)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("收到信号 %v，正在关闭...", sig)
		eng.Stop()
		cancel()
		os.Exit(0)
	}()

	if err := eng.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "❌ 启动失败: %v\n", err)
		os.Exit(1)
	}

	<-ctx.Done()
}

func pidFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".iwc", "iwc.pid")
}

// --- stop ---

func runStop() {
	stopped := false

	// 尝试通过 launchctl 停止（autostart 模式）
	plist := plistPath()
	if _, err := os.Stat(plist); err == nil {
		out, _ := exec.Command("launchctl", "stop", "com.user.iwc").CombinedOutput()
		if len(out) > 0 {
			fmt.Print(string(out))
		}
		stopped = true
	}

	// 通过 PID 文件停止手动启动的进程
	pidPath := pidFilePath()
	if data, err := os.ReadFile(pidPath); err == nil {
		pid := strings.TrimSpace(string(data))
		if pid != "" {
			exec.Command("kill", pid).Run()
			os.Remove(pidPath)
			stopped = true
		}
	}

	if stopped {
		fmt.Println("✅ iWC 已停止")
	} else {
		fmt.Println("ℹ️  iWC 未在运行")
	}
}

// --- restart ---

func runRestart() {
	runStop()
	time.Sleep(500 * time.Millisecond)
	run()
}

// --- 开机自启 ---

const plistName = "com.user.iwc.plist"

func plistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", plistName)
}

func plistContent() string {
	home, _ := os.UserHomeDir()
	binary := filepath.Join(home, ".local", "bin", "iwc")
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.user.iwc</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>start</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/iwc_stdout.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/iwc_stderr.log</string>
</dict>
</plist>
`, binary)
}

func runAutostartCmd(args []string) {
	if len(args) == 0 {
		fmt.Println(`用法:
  iwc autostart on     设置开机自启
  iwc autostart off    取消开机自启`)
		return
	}

	switch args[0] {
	case "on", "enable":
		path := plistPath()
		dir := filepath.Dir(path)
		os.MkdirAll(dir, 0755)

		if err := os.WriteFile(path, []byte(plistContent()), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "❌ 写入 plist 失败: %v\n", err)
			os.Exit(1)
		}

		out, err := exec.Command("launchctl", "load", "-w", path).CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ launchctl load 失败: %v\n%s", err, string(out))
			os.Exit(1)
		}

		fmt.Println("✅ 开机自启已开启")
		fmt.Printf("   plist: %s\n", path)

	case "off", "disable":
		path := plistPath()

		out, err := exec.Command("launchctl", "unload", "-w", path).CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ launchctl unload 失败: %v\n%s", err, string(out))
			os.Exit(1)
		}

		os.Remove(path)
		fmt.Println("✅ 开机自启已关闭")

	default:
		fmt.Fprintf(os.Stderr, "未知参数: %s\n用法: iwc autostart on|off\n", args[0])
		os.Exit(1)
	}
}

// --- 微信设置 ---

func runWechatCmd(args []string) {
	if len(args) == 0 {
		fmt.Println(`微信命令:
  iwc wechat setup    扫码登录并生成配置
  iwc wechat help     显示帮助`)
		return
	}

	switch args[0] {
	case "setup", "new", "bind":
		fs := flag.NewFlagSet("wechat setup", flag.ExitOnError)
		token := fs.String("token", "", "已有 Bearer token（跳过扫码）")
		apiURL := fs.String("api-url", "https://ilinkai.weixin.qq.com", "ilink API 地址")
		timeout := fs.Int("timeout", 480, "扫码等待超时秒数")
		botType := fs.String("bot-type", "3", "bot_type")
		_ = fs.Parse(args[1:])
		doWechatSetup(*token, *apiURL, *timeout, *botType)
	case "help", "--help", "-h":
		fmt.Println(`用法:
  iwc wechat setup                 扫码登录
  iwc wechat setup --token <tok>  使用已有 token
  iwc wechat setup --api-url ...  自定义 API 地址`)
	default:
		fmt.Fprintf(os.Stderr, "未知子命令: %s\n", args[0])
	}
}

func doWechatSetup(tokenStr, apiURL string, timeout int, botType string) {
	cfg := config.DefaultConfig()
	cfgPath := config.ConfigPath()

	if existing, err := config.Load(""); err == nil {
		cfg = existing
	}

	if tokenStr != "" {
		fmt.Println("验证 token...")
		client := weixin.NewClient(apiURL, tokenStr)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		err := client.VerifyToken(ctx)
		cancel()
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Token 验证失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Token 验证通过")
		cfg.WeChat.Token = tokenStr
		cfg.WeChat.BaseURL = apiURL
	} else {
		fmt.Println("正在获取二维码...")
		fmt.Println("步骤 1/3: 获取二维码")
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		var openOnce sync.Once
		hooks := &weixin.QRLoginHooks{
			OnQRCode: func(url string) {
				fmt.Printf("步骤 2/3: 请扫码登录（二维码链接）\n%s\n", url)
				openOnce.Do(func() {
					if err := openBrowserURL(url); err == nil {
						fmt.Println("已自动尝试在浏览器打开二维码链接")
					}
				})
			},
			OnStatus: func(message string) {
				fmt.Printf("状态: %s\n", message)
			},
		}
		tok, baseURL, botID, userID, err := weixin.QRLoginWithHooks(ctx, apiURL, botType, time.Duration(timeout)*time.Second, hooks)
		cancel()
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ 扫码登录失败: %v\n", err)
			fmt.Fprintln(os.Stderr, "建议:")
			fmt.Fprintln(os.Stderr, "- 检查网络后重试: iwc wechat setup --timeout 600")
			fmt.Fprintln(os.Stderr, "- 如果你已有 token，可直接使用: iwc wechat setup --token <token>")
			os.Exit(1)
		}
		fmt.Println("步骤 3/3: 登录成功，正在写入配置")
		fmt.Printf("✅ 登录成功! bot_id: %s\n", botID)
		cfg.WeChat.Token = tok
		cfg.WeChat.BaseURL = baseURL
		if userID != "" && cfg.WeChat.AllowFrom == "" {
			cfg.WeChat.AllowFrom = userID
			fmt.Printf("📝 已设置 allow_from = %s\n", userID)
		}
	}

	if err := config.Save(cfg, cfgPath); err != nil {
		fmt.Fprintf(os.Stderr, "❌ 保存配置失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ 配置已保存: %s\n", cfgPath)
	fmt.Println()
	fmt.Println("现在可以运行 `iwc start` 启动服务了")
}

func openBrowserURL(url string) error {
	if strings.TrimSpace(url) == "" {
		return fmt.Errorf("empty url")
	}

	var cmd *exec.Cmd
	switch {
	case runtime.GOOS == "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case runtime.GOOS == "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
