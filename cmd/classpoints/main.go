package main

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"classpoints/internal/app"

	"github.com/gin-gonic/gin"
)

//go:embed all:dist
var embeddedDist embed.FS

func main() {
	gin.SetMode(gin.ReleaseMode)

	dataDir := defaultDataDir()
	logFile, err := setupLogging(defaultLogDir())
	if err != nil {
		log.Fatalf("初始化日志失败: %v", err)
	}
	defer logFile.Close()
	if err := migrateLegacyData(dataDir); err != nil {
		log.Printf("迁移旧数据失败: %v", err)
	}

	port := getenv("CLASSPOINTS_PORT", "8787")

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("创建数据目录失败: %v", err)
	}

	server, err := app.NewServer(filepath.Join(dataDir, "classpoints.db"))
	if err != nil {
		log.Fatalf("启动服务失败: %v", err)
	}
	defer server.Close()

	server.StartAutoBackup(filepath.Join(dataDir, "backups"), 30*time.Minute)

	router := gin.New()
	router.Use(gin.LoggerWithWriter(logFile), gin.RecoveryWithWriter(logFile))
	router.Use(corsForLocalDev())
	server.RegisterRoutes(router.Group("/api"))
	mountStatic(router)

	addr := "127.0.0.1:" + port
	url := "http://" + addr
	if os.Getenv("CLASSPOINTS_NO_BROWSER") != "1" {
		go func() {
			time.Sleep(400 * time.Millisecond)
			_ = openBrowser(url)
		}()
	}

	log.Printf("班级积分系统已启动: %s", url)
	if err := router.Run(addr); err != nil {
		log.Fatalf("HTTP 服务退出: %v", err)
	}
}

func mountStatic(router *gin.Engine) {
	distFS, err := fs.Sub(embeddedDist, "dist")
	if err != nil {
		router.GET("/", func(c *gin.Context) {
			c.String(http.StatusOK, "前端尚未构建，请先运行 frontend 构建。")
		})
		return
	}

	if assets, err := fs.Sub(distFS, "assets"); err == nil {
		router.StaticFS("/assets", http.FS(assets))
	}
	serveIndex := func(c *gin.Context) {
		content, err := fs.ReadFile(distFS, "index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "前端文件读取失败")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", content)
	}
	router.GET("/", serveIndex)
	router.NoRoute(func(c *gin.Context) {
		if c.Request.URL.Path == "/api" || len(c.Request.URL.Path) >= 5 && c.Request.URL.Path[:5] == "/api/" {
			c.JSON(http.StatusNotFound, gin.H{"error": "接口不存在"})
			return
		}
		serveIndex(c)
	})
}

func corsForLocalDev() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin == "http://localhost:5173" || origin == "http://127.0.0.1:5173" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Headers", "Content-Type")
			c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return errors.New("unsupported platform")
	}
	return cmd.Start()
}

func getenv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func defaultDataDir() string {
	if value := os.Getenv("CLASSPOINTS_DATA_DIR"); value != "" {
		return value
	}
	return filepath.Join(defaultBaseDir(), "data")
}

func defaultLogDir() string {
	if value := os.Getenv("CLASSPOINTS_LOG_DIR"); value != "" {
		return value
	}
	return filepath.Join(defaultBaseDir(), "logs")
}

func defaultBaseDir() string {
	if runtime.GOOS == "darwin" {
		if configDir, err := os.UserConfigDir(); err == nil && configDir != "" {
			return filepath.Join(configDir, "ClassPoints")
		}
	}
	exe, err := os.Executable()
	if err == nil && exe != "" {
		return filepath.Dir(exe)
	}
	return "."
}

func setupLogging(logDir string) (*os.File, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}
	logPath := filepath.Join(logDir, "classpoints.log")
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	if os.Getenv("CLASSPOINTS_CONSOLE_LOG") == "1" {
		log.SetOutput(io.MultiWriter(file, os.Stdout))
	} else {
		log.SetOutput(file)
	}
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Printf("日志文件: %s", logPath)
	return file, nil
}

func migrateLegacyData(dataDir string) error {
	if os.Getenv("CLASSPOINTS_DATA_DIR") != "" {
		return nil
	}
	if runtime.GOOS != "windows" {
		return nil
	}
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return nil
	}
	legacyDataDir := filepath.Join(local, "ClassPoints", "data")
	legacyDB := filepath.Join(legacyDataDir, "classpoints.db")
	targetDB := filepath.Join(dataDir, "classpoints.db")
	if _, err := os.Stat(targetDB); err == nil {
		return nil
	}
	if _, err := os.Stat(legacyDB); err != nil {
		return nil
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}
	if err := copyFile(legacyDB, targetDB); err != nil {
		return err
	}
	legacyBackupDir := filepath.Join(legacyDataDir, "backups")
	targetBackupDir := filepath.Join(dataDir, "backups")
	if entries, err := os.ReadDir(legacyBackupDir); err == nil {
		if err := os.MkdirAll(targetBackupDir, 0755); err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			source := filepath.Join(legacyBackupDir, entry.Name())
			target := filepath.Join(targetBackupDir, entry.Name())
			if _, err := os.Stat(target); err == nil {
				continue
			}
			if err := copyFile(source, target); err != nil {
				return err
			}
		}
	}
	log.Printf("已从旧目录迁移数据: %s -> %s", legacyDataDir, dataDir)
	return nil
}

func copyFile(source string, target string) error {
	input, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(target, input, 0644)
}

func init() {
	if _, err := fs.Stat(embeddedDist, "dist/index.html"); err != nil {
		fmt.Println("提示：当前可直接运行 API；完整单机程序需要先构建 frontend。")
	}
}
