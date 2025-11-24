// @title goMobile
// @version 25.11.24
// @description SIP Simulation and Policy Engine for internal debugging.
// @BasePath /

// swag - swag init -g cmd/goMobile/main.go
package main

import (
	"fmt"

	"os"
	"strings"

	"gomobile/internal/service/db"
	"gomobile/internal/service/fnm"
	"gomobile/internal/service/logic"
	"gomobile/internal/service/policy"
	"gomobile/internal/types"

	"github.com/fatih/color"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"gomobile/internal/constants"
	"gomobile/internal/logger"
	webserver "gomobile/internal/transport/http"
	sipserver "gomobile/internal/transport/sip"
)

func main() {
	// configuration initialization
	if err := InitConfig(); err != nil {
		fmt.Printf("error initializing config: %s", err.Error())
	}

	// load env variables
	if err := godotenv.Load(); err != nil {
		fmt.Printf("error loading env variables: %s", err.Error())
	}

	// logger initialization
	logDir := viper.GetString("logging.directory")
	if logDir == "" {
		logDir = "logs"
	}
	retain := viper.GetInt("logging.retain_days")
	if retain <= 0 {
		retain = 7
	}
	if err := logger.Init(logDir, retain); err != nil {
		logrus.Fatalf("failed to initialize logger: %v", err)
	}
	defer logger.Close()

	startingMessage()

	policiesRepo := policy.NewPolicyRepository()
	loader := policy.NewPolicyLoader(policiesRepo)
	policyDir := viper.GetString("data.policy.policy_dir")
	if err := loader.LoadLatestFromDir(policyDir); err != nil {
		fmt.Printf("Failed to load policies: %v", err)
		logrus.Fatalf("Failed to load policies: %v", err)
	}

	fnmRepo := fnm.NewFnmRepository()
	fnmLoader := fnm.NewFnmLoader(fnmRepo)
	fnmDir := viper.GetString("data.fnm.fnm_dir")

	if err := fnmLoader.LoadLatestFromDir(fnmDir); err != nil {
		logrus.Fatalf("Failed to load FNM: %v", err)
	}

	mysql_host := viper.GetString("services.db.mysql.host")
	mysql_port := viper.GetInt("services.db.mysql.port")
	mysql_user := viper.GetString("services.db.mysql.user")
	mysql_db := viper.GetString("services.db.mysql.database")

	mysql_pass := os.Getenv("MYSQL_PASSWORD")
	if mysql_pass == "" {
		logrus.Fatal("MYSQL_PASSWORD is not set")
	}

	storage, err := db.NewStorage(mysql_host, mysql_port, mysql_user, mysql_pass, mysql_db)
	if err != nil {
		fmt.Sprintf("failed to init DB: %v", err)
		logrus.Fatalf("failed to init DB: %v", err)
	}

	bl := logic.NewBusinessLogic(policiesRepo, fnmRepo, storage)

	sipPort := viper.GetInt("sipserver.port")
	sipSrv := sipserver.New(sipPort, bl)
	if err := sipSrv.Start(); err != nil {
		fmt.Printf("Failed to start SIP server: %v", err)
		logrus.Fatalf("Failed to start SIP server: %v", err)
	}

	httpPort := viper.GetString("webserver.port")
	httpSrv := webserver.NewHTTPServer(bl)
	if err := httpSrv.Start(httpPort); err != nil {
		logrus.Fatalf("Failed to start HTTP server: %v", err)
		fmt.Printf("Failed to start HTTP server: %v", err)
	}

	select {}

}

func InitConfig() error {
	viper.AddConfigPath("configs")
	viper.SetConfigName("config")
	return viper.ReadInConfig()
}

func startingMessage() {
	logMsg := fmt.Sprintf("goMobile %s", getAppInfo())
	logrus.Info(logMsg)
	logrus.Info(getConfigInfo())
	fmt.Println(getAppBanner())
	fmt.Println(getAppInfo())
	fmt.Println(getConfigInfo())

}

func getAppBanner() string {
	cyan := color.New(color.FgCyan).SprintFunc()
	return cyan(`
╔══════════════════════════════════════════════════════╗
║                      goMobile                        ║
║           SIP Redirection Proxy Server               ║
║                 © Cocobri, LLC                       ║
╚══════════════════════════════════════════════════════╝
`)
}

func getAppInfo() string {
	cyan := color.New(color.FgCyan).SprintFunc()
	return cyan(fmt.Sprintf("Version: %s", constants.Version))
}

func getConfigInfo() string {
	colors := initColors()

	sections := []types.ConfigSection{
		{
			Name: "process",
			Fields: []types.ConfigField{
				{Key: "pid", Label: "pid", Color: colors.Yellow, Indent: 2, IsSpecial: true},
			},
		},
		{
			Name: "sipserver",
			Fields: []types.ConfigField{
				{Key: "sipserver.port", Label: "port", Color: colors.Green, Indent: 2},
				{Key: "sipserver.acl.ip", Label: "acl:", Color: colors.Blue, Indent: 2, IsNested: true},
			},
		},
		{
			Name: "webserver",
			Fields: []types.ConfigField{
				{Key: "webserver.port", Label: "port", Color: colors.Green, Indent: 2},
			},
		},
		{
			Name: "logging",
			Fields: []types.ConfigField{
				{Key: "logging.directory", Label: "directory", Color: colors.Yellow, Indent: 2},
				{Key: "logging.retain_days", Label: "retain_days", Color: colors.Magenta, Indent: 2},
			},
		},
		{
			Name: "flags",
			Fields: []types.ConfigField{
				{Key: "flags.debug", Label: "debug", Color: colors.Debug, Indent: 2},
			},
		},
	}

	return formatConfig(sections, &colors)
}

func formatConfig(sections []types.ConfigSection, colors *types.ColorSet) string {
	var result string
	for _, sec := range sections {
		result += colors.White(fmt.Sprintf("\n%s\n", strings.ToUpper(sec.Name)))

		// находим длину label для выравнивания
		maxLabel := 0
		for _, f := range sec.Fields {
			if len(f.Label) > maxLabel {
				maxLabel = len(f.Label)
			}
		}

		for _, f := range sec.Fields {
			indent := strings.Repeat(" ", f.Indent)
			label := fmt.Sprintf("%-*s", maxLabel, f.Label+":")
			value := getFieldValue(&f, colors, indent)
			result += fmt.Sprintf("%s%s %s\n", indent, f.Color(label), value)
		}
	}
	return result
}

func getFieldValue(f *types.ConfigField, colors *types.ColorSet, indent string) string {
	if f.IsSpecial && f.Key == "pid" {
		return colors.Yellow(fmt.Sprintf("%d", os.Getpid()))
	}

	if f.IsNested {
		ips := viper.GetStringSlice(f.Key)
		if len(ips) == 0 {
			return colors.Cyan("none")
		}
		lines := ""
		for _, ip := range ips {
			lines += fmt.Sprintf("\n%s  - %s", indent, colors.Cyan(ip))
		}
		return lines
	}

	raw := viper.Get(f.Key)
	return fmt.Sprintf("%v", raw)
}

func initColors() types.ColorSet {
	return types.ColorSet{
		White:   color.New(color.FgWhite).SprintFunc(),
		Green:   color.New(color.FgGreen).SprintFunc(),
		Yellow:  color.New(color.FgYellow).SprintFunc(),
		Blue:    color.New(color.FgBlue).SprintFunc(),
		Red:     color.New(color.FgRed).SprintFunc(),
		Cyan:    color.New(color.FgCyan).SprintFunc(),
		Magenta: color.New(color.FgMagenta).SprintFunc(),
		Debug: func(a ...interface{}) string {
			if viper.GetBool("flags.debug") {
				return color.New(color.FgGreen).SprintFunc()(a...)
			}
			return color.New(color.FgRed).SprintFunc()(a...)
		},
	}
}
